package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/connection"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	"UptimePingPlatform/pkg/ratelimit"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/api-gateway/internal/client"
	httphandler "UptimePingPlatform/services/api-gateway/internal/handler/http" // алиас для вашего пакета http
	"UptimePingPlatform/services/api-gateway/internal/middleware"
	"UptimePingPlatform/services/api-gateway/internal/service"
)

func main() {
	// Инициализация конфигурации
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация логгера
	appLogger, err := logger.NewLogger(
		cfg.Environment,
		cfg.Logger.Level,
		"api-gateway",
		false, // временно отключил Loki
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := appLogger.Sync(); err != nil {
			log.Printf("Error syncing logger: %v", err)
		}
	}()

	// Инициализация retry конфигурации
	retryConfig := connection.DefaultRetryConfig()

	// Инициализация Redis с retry логикой
	redisConfig := pkg_redis.NewConfig()
	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		redisConfig.Addr = redisAddr
	}

	redisCtx, redisCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer redisCancel()

	var redisClient *pkg_redis.Client
	err = connection.WithRetry(redisCtx, retryConfig, func(ctx context.Context) error {
		var err error
		redisClient, err = pkg_redis.Connect(ctx, redisConfig)
		if err != nil {
			appLogger.Error("Failed to connect to redis, retrying...", logger.String("error", err.Error()))
			return err
		}
		return nil
	})
	if err != nil {
		appLogger.Error("Failed to connect to redis after retries", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer redisClient.Close()

	// Инициализация rate limiter
	rateLimiter := ratelimit.NewRedisRateLimiter(redisClient.Client)

	// Инициализация метрик
	metricCollector := metrics.NewMetrics("api_gateway")

	// Создаем реальный gRPC клиент для auth-service
	authServiceAddr := os.Getenv("AUTH_SERVICE_ADDR")
	if authServiceAddr == "" {
		authServiceAddr = "localhost:50051"
	}
	authClient, err := client.NewGRPCAuthClient(authServiceAddr, 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to auth service", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer authClient.Close()

	// Создаем gRPC клиент для scheduler-service
	schedulerServiceAddr := os.Getenv("SCHEDULER_SERVICE_ADDR")
	if schedulerServiceAddr == "" {
		schedulerServiceAddr = "localhost:50052"
	}
	schedulerClient, err := client.NewSchedulerClient(schedulerServiceAddr, 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to scheduler service", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer schedulerClient.Close()

	// Создаем gRPC клиент для forge-service
	forgeServiceAddr := os.Getenv("FORGE_SERVICE_ADDR")
	if forgeServiceAddr == "" {
		forgeServiceAddr = "localhost:50053"
	}
	forgeClient, err := client.NewGRPCForgeClient(forgeServiceAddr, 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to forge service", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer forgeClient.Close()

	// Настройка HTTP сервера
	// Создаем реальные сервисы
	authAdapter := service.NewAuthAdapter(authClient)
	healthChecker := health.NewSimpleHealthChecker("1.0.0")
	healthHandler := httphandler.NewHealthHandler(healthChecker)
	
	baseHandler := httphandler.NewHandler(authAdapter, healthHandler, schedulerClient, forgeClient, appLogger)

	// Обертываем хендлер в middleware
	var httpHandler http.Handler = baseHandler
	httpHandler = middleware.LoggingMiddleware(appLogger)(httpHandler)
	httpHandler = middleware.RecoveryMiddleware()(httpHandler)
	httpHandler = middleware.CORSMiddleware([]string{"*"})(httpHandler)
	httpHandler = middleware.RateLimitMiddleware(rateLimiter, 100, time.Minute)(httpHandler)
	httpHandler = middleware.AuthMiddleware(authClient)(httpHandler)

	// Добавляем эндпоинт для метрик
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metricCollector.GetHandler())
	metricsMux.Handle("/", httpHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: metricsMux,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		appLogger.Info("Starting API Gateway server", logger.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("Server failed", logger.String("error", err.Error()))
		}
	}()

	// Обработка сигналов для graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	appLogger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Error("Server shutdown failed", logger.String("error", err.Error()))
	}

	appLogger.Info("Server stopped")
}
