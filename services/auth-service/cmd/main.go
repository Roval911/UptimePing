package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/connection"
	"UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	"UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/pkg/ratelimit"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/pkg/password"
	"UptimePingPlatform/services/auth-service/internal/repository/postgres"
	redis_repo "UptimePingPlatform/services/auth-service/internal/repository/redis"
	"UptimePingPlatform/services/auth-service/internal/service"

	grpc_auth "UptimePingPlatform/gen/go/proto/api/auth/v1"
	"UptimePingPlatform/services/auth-service/internal/grpc/handlers"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Инициализация конфигурации
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация логгера
	appLogger, err := logger.NewLogger(cfg.Environment, cfg.Logger.Level, "auth-service", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Инициализация метрик
	metricsInstance := metrics.NewMetrics("auth-service")

	// Инициализация retry конфигурации
	retryConfig := connection.DefaultRetryConfig()

	// Инициализация базы данных с retry логикой
	dbConfig := &database.Config{
		Host:          cfg.Database.Host,
		Port:          cfg.Database.Port,
		User:          cfg.Database.User,
		Password:      cfg.Database.Password,
		Database:      cfg.Database.Name,
		SSLMode:       "disable",
		MaxConns:      20,
		MinConns:      5,
		MaxConnLife:   30 * time.Minute,
		MaxConnIdle:   5 * time.Minute,
		HealthCheck:   30 * time.Second,
		MaxRetries:    3,
		RetryInterval: 1 * time.Second,
	}

	var postgresDB *database.Postgres
	err = connection.WithRetry(context.Background(), retryConfig, func(ctx context.Context) error {
		var err error
		postgresDB, err = database.Connect(ctx, dbConfig)
		if err != nil {
			appLogger.Error("Failed to connect to database, retrying...", logger.String("error", err.Error()))
			return err
		}
		return nil
	})
	if err != nil {
		appLogger.Error("Failed to connect to database after retries", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer postgresDB.Pool.Close()

	// Инициализация Redis с retry логикой
	redisConfig := pkg_redis.NewConfig()
	var redisClient *pkg_redis.Client
	err = connection.WithRetry(context.Background(), retryConfig, func(ctx context.Context) error {
		var err error
		redisClient, err = pkg_redis.Connect(ctx, redisConfig)
		if err != nil {
			appLogger.Error("Failed to connect to Redis, retrying...", logger.String("error", err.Error()))
			return err
		}
		return nil
	})
	if err != nil {
		appLogger.Error("Failed to connect to Redis after retries", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer redisClient.Client.Close()

	// Инициализация RabbitMQ для уведомлений
	rabbitConfig := rabbitmq.NewConfig()
	rabbitConfig.URL = cfg.RabbitMQ.URL
	rabbitConfig.Queue = cfg.RabbitMQ.Queue

	rabbitConn, err := rabbitmq.Connect(context.Background(), rabbitConfig)
	if err != nil {
		appLogger.Warn("Failed to connect to RabbitMQ (notifications disabled)", logger.String("error", err.Error()))
		// Не выходим, так как это не критично для работы сервиса
	} else {
		defer rabbitConn.Close()
		appLogger.Info("RabbitMQ connected for notifications")
	}

	// Инициализация компонентов
	jwtManager := jwt.NewManager(cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret, 24*time.Hour, 7*24*time.Hour)
	passwordHasher := password.NewBcryptHasher(12)

	// Инициализация репозиториев
	userRepo := postgres.NewUserRepository(postgresDB.Pool)
	tenantRepo := postgres.NewTenantRepository(postgresDB.Pool)
	apiKeyRepo := postgres.NewAPIKeyRepository(postgresDB.Pool)
	sessionRepo := redis_repo.NewSessionRepository(redisClient.Client)

	// Инициализация сервиса
	authService := service.NewAuthService(
		userRepo,
		tenantRepo,
		apiKeyRepo,
		sessionRepo,
		jwtManager,
		passwordHasher,
	)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer()

	// Регистрация обработчиков
	authHandler := handlers.NewAuthHandler(authService, appLogger)
	grpc_auth.RegisterAuthServiceServer(grpcServer, authHandler)

	// Включаем reflection для разработки
	reflection.Register(grpcServer)

	// Запуск gRPC сервера
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
		if err != nil {
			appLogger.Error("Failed to listen", logger.String("error", err.Error()))
			os.Exit(1)
		}

		appLogger.Info("gRPC server starting", logger.Int("port", cfg.GRPC.Port))
		if err := grpcServer.Serve(lis); err != nil {
			appLogger.Error("Failed to serve gRPC", logger.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Запуск HTTP сервера для health checks
	go func() {
		mux := http.NewServeMux()

		// Инициализация health checker
		healthChecker := health.NewSimpleHealthChecker("1.0.0")

		// Инициализация rate limiter
		rateLimiter := ratelimit.NewRedisRateLimiter(redisClient.Client)

		// Rate limiting middleware
		rateLimitMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				clientIP := getClientIP(r)
				allowed, err := rateLimiter.CheckRateLimit(r.Context(), clientIP, cfg.RateLimiting.RequestsPerMinute, time.Minute)
				if err != nil {
					appLogger.Error("Rate limit check failed", logger.String("error", err.Error()))
					http.Error(w, "Rate limit check failed", http.StatusInternalServerError)
					return
				}

				if !allowed {
					appLogger.Warn("Rate limit exceeded", logger.String("client_ip", clientIP))
					http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
					return
				}

				next.ServeHTTP(w, r)
			})
		}

		// Health check эндпоинты
		mux.HandleFunc("/health", health.Handler(healthChecker))
		mux.HandleFunc("/ready", health.ReadyHandler(healthChecker))
		mux.HandleFunc("/live", health.LiveHandler())

		// Metrics эндпоинт
		mux.Handle("/metrics", metricsInstance.GetHandler())

		// Применяем middleware
		handler := metricsInstance.Middleware(rateLimitMiddleware(mux))

		appLogger.Info("HTTP server starting", logger.String("address", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)))
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port), handler); err != nil {
			appLogger.Error("Failed to serve HTTP", logger.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Остановка gRPC сервера
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		appLogger.Info("gRPC server stopped gracefully")
	case <-ctx.Done():
		appLogger.Info("gRPC server stopped forcefully")
		grpcServer.Stop()
	}

	appLogger.Info("Server shutdown complete")
}

// getClientIP получает IP адрес клиента из запроса
func getClientIP(r *http.Request) string {
	// Проверяем X-Forwarded-For header (если за прокси)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// Проверяем X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Возвращаем RemoteAddr
	return r.RemoteAddr
}
