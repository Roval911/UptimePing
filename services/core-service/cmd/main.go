package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/connection"
	"UptimePingPlatform/pkg/database"
	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	pkg_metrics "UptimePingPlatform/pkg/metrics"
	pkg_rabbitmq "UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/pkg/ratelimit"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/core-service/internal/client"
	consumerRabbitMQ "UptimePingPlatform/services/core-service/internal/consumer/rabbitmq"
	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/services/core-service/internal/logging"
	"UptimePingPlatform/services/core-service/internal/metrics"
	"UptimePingPlatform/services/core-service/internal/repository/postgres"
	"UptimePingPlatform/services/core-service/internal/service"
	"UptimePingPlatform/services/core-service/internal/service/checker"
	"UptimePingPlatform/services/core-service/internal/worker"
)

const (
	serviceName    = "core-service"
	serviceVersion = "v1.0.0"
)

func main() {
	// Вызываем основную функцию сервиса
	mainCmd()
}

// Экспортируемая функция для вызова из корневого main.go
func CmdServer() {
	mainCmd()
}

func mainCmd() {
	// Определяем путь к конфигурации
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	configPath := ""

	// Ищем config.yaml в services/core-service/config/
	testPath := filepath.Join(wd, "services", "core-service", "config", "config.yaml")
	if _, err := os.Stat(testPath); err == nil {
		configPath = testPath
	}

	// Если не нашли, пробуем другие варианты
	if configPath == "" {
		testPath = filepath.Join(wd, "config", "config.yaml")
		if _, err := os.Stat(testPath); err == nil {
			configPath = testPath
		}
	}

	// Если все еще не нашли, ищем в родительских директориях
	if configPath == "" {
		parentDir := wd
		for i := 0; i < 5; i++ {
			testPath = filepath.Join(parentDir, "config", "config.yaml")
			if _, err := os.Stat(testPath); err == nil {
				configPath = testPath
				break
			}
			parentDir = filepath.Dir(parentDir)
		}
	}

	if configPath == "" {
		log.Fatalf("Could not find config.yaml file")
	}

	// Инициализация конфигурации
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация логгера
	appLogger, err := logger.NewLogger(cfg.Environment, cfg.Logger.Level, serviceName, false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := appLogger.Sync(); err != nil {
			appLogger.Error("Error syncing logger", logger.Error(err))
		}
	}()

	appLogger.Info("Starting core service",
		logger.String("version", serviceVersion),
		logger.String("service", serviceName))

	// Инициализация BaseHandler
	baseHandler := grpcBase.NewBaseHandler(appLogger)

	// Инициализация retry конфигурации
	retryConfig := connection.DefaultRetryConfig()

	// Инициализируем метрики
	metricsInstance := pkg_metrics.NewMetrics(serviceName)

	ctx := context.Background()

	// Инициализируем базу данных
	dbConfig := database.NewConfig()
	dbConfig.Host = cfg.Database.Host
	dbConfig.Port = cfg.Database.Port
	dbConfig.User = cfg.Database.User
	dbConfig.Password = cfg.Database.Password
	dbConfig.Database = cfg.Database.Name

	// Подключение к базе данных с retry логикой
	var db *database.Postgres
	err = connection.WithRetry(ctx, retryConfig, func(ctx context.Context) error {
		var err error
		db, err = database.Connect(ctx, dbConfig)
		if err != nil {
			baseHandler.LogOperationStart(ctx, "database_connect", map[string]interface{}{
				"host":     dbConfig.Host,
				"port":     dbConfig.Port,
				"database": dbConfig.Database,
			})
			return err
		}
		return nil
	})
	if err != nil {
		appLogger.Error("Failed to connect to database", logger.Error(err))
		os.Exit(1)
	}
	defer db.Close()

	baseHandler.LogOperationSuccess(ctx, "database_connect", map[string]interface{}{
		"database": dbConfig.Database,
	})

	// Инициализируем RabbitMQ
	rabbitConfig := pkg_rabbitmq.NewConfig()
	rabbitConfig.URL = cfg.RabbitMQ.URL
	rabbitConfig.Queue = cfg.RabbitMQ.Queue

	rabbitConn, err := pkg_rabbitmq.Connect(ctx, rabbitConfig)
	if err != nil {
		appLogger.Error("Failed to connect to RabbitMQ", logger.Error(err))
		os.Exit(1)
	}
	defer rabbitConn.Close()

	// Инициализируем Redis
	redisConfig := pkg_redis.NewConfig()
	redisConfig.Addr = cfg.Redis.Addr
	redisConfig.Password = cfg.Redis.Password
	redisConfig.DB = cfg.Redis.DB
	redisConfig.PoolSize = cfg.Redis.PoolSize
	redisConfig.MinIdleConn = cfg.Redis.MinIdleConn
	redisConfig.MaxRetries = cfg.Redis.MaxRetries

	redisClient, err := pkg_redis.Connect(ctx, redisConfig)
	if err != nil {
		appLogger.Error("Failed to connect to Redis", logger.Error(err))
		os.Exit(1)
	}
	defer redisClient.Close()

	// Инициализируем health checker
	healthChecker := health.NewSimpleHealthChecker(serviceVersion)

	// Инициализируем rate limiter для проверок
	rateLimiter := ratelimit.NewRedisRateLimiter(redisClient.Client)

	// Создаем фабрику checker'ов
	httpClient := checker.NewDefaultHTTPClient(30 * time.Second)
	checkerFactory := checker.NewDefaultCheckerFactory(appLogger, httpClient)

	// Создаем checker'ы для всех поддерживаемых типов
	checkers := make(map[domain.TaskType]checker.Checker)
	for _, taskType := range checkerFactory.GetSupportedTypes() {
		checker, err := checkerFactory.CreateChecker(taskType)
		if err != nil {
			appLogger.Error("Failed to create checker",
				logger.String("task_type", string(taskType)),
				logger.Error(err))
			os.Exit(1)
		}
		checkers[taskType] = checker
		appLogger.Info("Created checker", logger.String("task_type", string(taskType)))
	}

	// Инициализируем worker pool
	uptimeLogger := logging.NewUptimeLogger(appLogger)
	uptimeMetrics := metrics.NewUptimeMetrics(serviceName)
	workerPool, err := worker.NewPool(worker.DefaultConfig(), uptimeLogger, uptimeMetrics, checkers)
	if err != nil {
		appLogger.Error("Failed to create worker pool", logger.Error(err))
		os.Exit(1)
	}

	// Запускаем worker pool
	if err := workerPool.Start(ctx); err != nil {
		appLogger.Error("Failed to start worker pool", logger.Error(err))
		os.Exit(1)
	}

	// Инициализируем consumer
	checkResultRepository := postgres.NewCheckResultRepository(db.Pool, appLogger)

	// Создаем Incident Manager клиент
	incidentClientConfig := &client.Config{
		Address:         cfg.IncidentManager.Address,
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        10 * time.Second,
		RetryMultiplier: 2.0,
		RetryJitter:     0.1,
	}

	incidentClient, err := client.NewIncidentClient(incidentClientConfig)
	if err != nil {
		appLogger.Warn("Failed to create incident client, using nil", logger.Error(err))
		incidentClient = nil
	}
	defer func() {
		if incidentClient != nil {
			incidentClient.Close()
		}
	}()

	incidentManager := service.NewGRPCIncidentManager(incidentClient, appLogger)

	checkService := service.NewCheckService(appLogger, checkerFactory, checkResultRepository, redisClient, incidentManager)
	consumer, err := consumerRabbitMQ.NewConsumer(
		consumerRabbitMQ.ConsumerConfig{
			QueueName:   rabbitConfig.Queue,
			ConsumerTag: "core-service-consumer",
		},
		appLogger,
		checkService,
		rabbitConn,
	)
	if err != nil {
		appLogger.Error("Failed to create consumer", logger.Error(err))
		os.Exit(1)
	}
	defer consumer.Close()

	// Запускаем consumer
	if err := consumer.Start(ctx); err != nil {
		appLogger.Error("Failed to start consumer", logger.Error(err))
		os.Exit(1)
	}

	appLogger.Info("Core service started successfully")

	// Запускаем HTTP сервер для health check и metrics
	go func() {
		mux := http.NewServeMux()

		// Rate limiting middleware
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Rate limiting для API запросов
			clientIP := getClientIP(r)
			allowed, err := rateLimiter.CheckRateLimit(r.Context(), clientIP, cfg.RateLimiting.RequestsPerMinute, time.Minute)
			if err != nil {
				http.Error(w, "Rate limit check failed", http.StatusInternalServerError)
				return
			}

			if !allowed {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Простой health check
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Core Service is running"))
		})

		// Health endpoints
		mux.HandleFunc("/health", health.Handler(healthChecker))
		mux.HandleFunc("/ready", health.ReadyHandler(healthChecker))
		mux.HandleFunc("/live", health.LiveHandler())

		// Metrics endpoint
		mux.Handle("/metrics", metricsInstance.GetHandler())

		server := &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Server.Port), // Используем порт из конфига
			Handler: mux,
		}

		appLogger.Info("HTTP server started", logger.Int("port", cfg.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("HTTP server failed", logger.Error(err))
		}
	}()

	// Ожидаем сигналы для graceful shutdown
	awaitGracefulShutdown(ctx, appLogger, workerPool)
}

// awaitGracefulShutdown ожидает сигналы для graceful shutdown
func awaitGracefulShutdown(ctx context.Context, logger logger.Logger, workerPool *worker.Pool) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Остановка worker pool
	if err := workerPool.Stop(ctx); err != nil {
		logger.Error("Failed to stop worker pool")
	}

	logger.Info("Server stopped gracefully")
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
