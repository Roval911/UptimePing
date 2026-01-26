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
	"UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/logger"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/pkg/password"
	redis_repo "UptimePingPlatform/services/auth-service/internal/repository/redis"
	"UptimePingPlatform/services/auth-service/internal/repository/postgres"
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

	// Инициализация базы данных
	dbConfig := &database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Database: cfg.Database.Name,
		SSLMode:  "disable",
	}
	
	postgresDB, err := database.Connect(context.Background(), dbConfig)
	if err != nil {
		appLogger.Error("Failed to connect to database", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer postgresDB.Pool.Close()

	// Инициализация Redis
	redisConfig := pkg_redis.NewConfig()
	redisClient, err := pkg_redis.Connect(context.Background(), redisConfig)
	if err != nil {
		appLogger.Error("Failed to connect to Redis", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer redisClient.Client.Close()

	// Инициализация компонентов
	jwtManager := jwt.NewManager("your-access-secret", "your-refresh-secret", 24*time.Hour, 7*24*time.Hour)
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
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 50051))
		if err != nil {
			appLogger.Error("Failed to listen", logger.String("error", err.Error()))
			os.Exit(1)
		}

		appLogger.Info("gRPC server starting", logger.Int("port", 50051))
		if err := grpcServer.Serve(lis); err != nil {
			appLogger.Error("Failed to serve gRPC", logger.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Запуск HTTP сервера для health checks
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		appLogger.Info("HTTP server starting", logger.String("address", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)))
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port), mux); err != nil {
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
