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
	// Инициализация конфигурации - единая схема для всех сервисов
	cfg, err := config.LoadConfig("")
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

	// Инициализация метрик с конфигурацией
	var metricCollector *metrics.Metrics
	if cfg.Metrics.Enabled {
		metricCollector = metrics.NewMetricsFromConfig("api-gateway", &cfg.Metrics)
	} else {
		metricCollector = metrics.NewMetrics("api-gateway")
	}

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

	// Создаем gRPC клиент для core-service
	coreServiceAddr := os.Getenv("CORE_SERVICE_ADDR")
	if coreServiceAddr == "" {
		coreServiceAddr = "localhost:50054"
	}
	coreClient, err := client.NewCoreClient(coreServiceAddr, 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to core service", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer coreClient.Close()

	// Создаем gRPC клиент для metrics-service
	metricsServiceAddr := os.Getenv("METRICS_SERVICE_ADDR")
	if metricsServiceAddr == "" {
		metricsServiceAddr = "localhost:50055"
	}
	metricsClient, err := client.NewMetricsClient(metricsServiceAddr, 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to metrics service", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer metricsClient.Close()

	// Создаем gRPC клиент для notification-service
	notificationServiceAddr := os.Getenv("NOTIFICATION_SERVICE_ADDR")
	if notificationServiceAddr == "" {
		notificationServiceAddr = "localhost:50057"
	}
	notificationClient, err := client.NewNotificationClient(notificationServiceAddr, 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to notification service", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer notificationClient.Close()

	// Создаем gRPC клиент для incident-manager
	incidentServiceAddr := os.Getenv("INCIDENT_SERVICE_ADDR")
	if incidentServiceAddr == "" {
		incidentServiceAddr = "localhost:50056"
	}
	incidentClient, err := client.NewIncidentClient(incidentServiceAddr, 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to incident service", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer incidentClient.Close()

	// Создаем клиент для конфигурации (использует pkg/config)
	configClient, err := client.NewConfigClient(5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to initialize config client", logger.String("error", err.Error()))
		os.Exit(1)
	}

	// Настройка HTTP сервера
	// Создаем реальные сервисы
	authAdapter := service.NewAuthAdapter(authClient)
	healthChecker := health.NewSimpleHealthChecker("1.0.0")
	healthHandler := httphandler.NewHealthHandler(healthChecker, appLogger)

	baseHandler := httphandler.NewHandler(authAdapter, healthHandler, schedulerClient, coreClient, metricsClient, incidentClient, notificationClient, configClient, forgeClient, appLogger)

	// Обертываем хендлер в middleware
	var httpHandler http.Handler = baseHandler
	httpHandler = middleware.LoggingMiddleware(appLogger)(httpHandler)
	httpHandler = middleware.RecoveryMiddleware(appLogger)(httpHandler)
	httpHandler = middleware.CORSMiddleware([]string{"*"}, appLogger)(httpHandler)
	httpHandler = middleware.RateLimitMiddleware(rateLimiter, 100, time.Minute, appLogger)(httpHandler)
	httpHandler = middleware.AuthMiddleware(authClient, appLogger)(httpHandler)

	// Добавляем эндпоинт для метрик
	metricsMux := http.NewServeMux()
	metricsPath := metricCollector.GetMetricsPath(&cfg.Metrics)
	metricsMux.Handle(metricsPath, metricCollector.GetHandler())
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
