package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"UptimePingPlatform/services/core-service/internal/client"
	consumerRabbitMQ "UptimePingPlatform/services/core-service/internal/consumer/rabbitmq"
	"UptimePingPlatform/services/core-service/internal/health"
	"UptimePingPlatform/services/core-service/internal/logging"
	"UptimePingPlatform/services/core-service/internal/metrics"
	"UptimePingPlatform/services/core-service/internal/service/checker"
	"UptimePingPlatform/services/core-service/internal/worker"
	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/pkg/logger"
)

const (
	serviceName    = "core-service"
	serviceVersion = "v1.0.0"
)

// AppConfig конфигурация приложения
type AppConfig struct {
	// База данных
	DatabaseDSN string `json:"database_dsn"`
	
	// RabbitMQ
	RabbitMQURL string `json:"rabbitmq_url"`
	RabbitMQQueue string `json:"rabbitmq_queue"`
	
	// Worker pool
	WorkerPool *worker.Config `json:"worker_pool"`
	
	// Health checks
	Health *health.Config `json:"health"`
	
	// Incident Manager
	IncidentManagerAddress string `json:"incident_manager_address"`
}

// DefaultAppConfig возвращает конфигурацию по умолчанию
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		DatabaseDSN: "postgres://user:password@localhost/uptimedb?sslmode=disable",
		RabbitMQURL: "amqp://localhost:5672",
		RabbitMQQueue: "uptime_checks",
		WorkerPool: worker.DefaultConfig(),
		Health: health.DefaultConfig(),
		IncidentManagerAddress: "localhost:50052",
	}
}

func main() {
	ctx := context.Background()
	
	// Инициализируем логгер
	baseLogger, err := logger.NewLogger("development", "info", serviceName, false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	
	uptimeLogger := logging.NewUptimeLogger(baseLogger)
	
	// Устанавливаем глобальный логгер
	logging.InitGlobalUptimeLogger(baseLogger)
	
	baseLogger.Info("Starting core service",
		logger.String("version", serviceVersion),
		logger.String("service", serviceName))
	
	// Загружаем конфигурацию
	appConfig := DefaultAppConfig()
	
	// Инициализируем метрики
	metricsInstance := metrics.NewUptimeMetrics(serviceName)
	
	// Инициализируем checkers
	checkers := initCheckers(uptimeLogger)
	
	// Инициализируем компоненты
	db, rabbitProducer, incidentClient, err := initializeComponents(ctx, appConfig, uptimeLogger)
	if err != nil {
		baseLogger.Error("Failed to initialize components", logger.Error(err))
		os.Exit(1)
	}
	defer cleanupComponents(db, rabbitProducer, incidentClient)
	
	// Инициализируем health checker
	healthService, err := health.NewService(appConfig.Health, uptimeLogger)
	if err != nil {
		baseLogger.Error("Failed to initialize health service", logger.Error(err))
		os.Exit(1)
	}
	
	// Запускаем health checker
	if err := healthService.Start(ctx); err != nil {
		baseLogger.Error("Failed to start health service", logger.Error(err))
		os.Exit(1)
	}
	defer healthService.Stop(ctx)
	
	// Выполняем health checks при запуске
	if err := performStartupHealthChecks(ctx, healthService); err != nil {
		baseLogger.Error("Startup health checks failed", logger.Error(err))
		os.Exit(1)
	}
	
	// Инициализируем worker pool
	workerPool, err := worker.NewPool(appConfig.WorkerPool, uptimeLogger, metricsInstance, checkers)
	if err != nil {
		baseLogger.Error("Failed to create worker pool", logger.Error(err))
		os.Exit(1)
	}
	
	// Запускаем worker pool
	if err := workerPool.Start(ctx); err != nil {
		baseLogger.Error("Failed to start worker pool", logger.Error(err))
		os.Exit(1)
	}
	
	// Инициализируем consumer
	consumerConfig := consumerRabbitMQ.ConsumerConfig{
		QueueName:   appConfig.RabbitMQQueue,
		ConsumerTag: "core-service-consumer",
	}
	consumer, err := consumerRabbitMQ.NewConsumer(
		consumerConfig,
		baseLogger,
		nil, // TODO: implement CheckServiceInterface
	)
	if err != nil {
		baseLogger.Error("Failed to create consumer", logger.Error(err))
		os.Exit(1)
	}
	defer consumer.Close()
	
	// Запускаем consumer
	if err := consumer.Start(ctx); err != nil {
		baseLogger.Error("Failed to start consumer", logger.Error(err))
		os.Exit(1)
	}
	
	baseLogger.Info("Core service started successfully")
	
	// Ожидаем сигналы для graceful shutdown
	awaitGracefulShutdown(ctx, uptimeLogger, workerPool, healthService)
}

// initCheckers инициализирует checkers
func initCheckers(logger *logging.UptimeLogger) map[domain.TaskType]checker.Checker {
	checkers := make(map[domain.TaskType]checker.Checker)
	
	// HTTP checker
	httpChecker := checker.NewHTTPChecker(30000) // 30 секунд
	checkers[domain.TaskTypeHTTP] = httpChecker
	
	// TCP checker
	tcpChecker := checker.NewTCPChecker(10000, nil) // 10 секунд
	checkers[domain.TaskTypeTCP] = tcpChecker
	
	// gRPC checker
	grpcChecker := checker.NewgRPCChecker(15000, logger.GetBaseLogger()) // 15 секунд
	checkers[domain.TaskTypeGRPC] = grpcChecker
	
	// GraphQL checker
	graphqlChecker := checker.NewGraphQLChecker(30000, logger.GetBaseLogger()) // 30 секунд
	checkers[domain.TaskTypeGraphQL] = graphqlChecker
	
	return checkers
}

// initializeComponents инициализирует все компоненты
func initializeComponents(ctx context.Context, config *AppConfig, logger *logging.UptimeLogger) (*sql.DB, *rabbitmq.Producer, client.IncidentClient, error) {
	logger.GetBaseLogger().Info("Initializing components")
	
	// Инициализируем базу данных
	logger.GetBaseLogger().Debug("Initializing database connection")
	db, err := sql.Open("postgres", config.DatabaseDSN)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Проверяем соединение
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}
	logger.GetBaseLogger().Info("Database connection established")
	
	// Инициализируем RabbitMQ producer
	logger.GetBaseLogger().Debug("Initializing RabbitMQ producer")
	rabbitConfig := rabbitmq.NewConfig()
	rabbitConfig.URL = config.RabbitMQURL
	rabbitConfig.Queue = config.RabbitMQQueue
	
	conn, err := rabbitmq.Connect(ctx, rabbitConfig)
	if err != nil {
		db.Close()
		return nil, nil, nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	
	rabbitProducer := rabbitmq.NewProducer(conn, rabbitConfig)
	logger.GetBaseLogger().Info("RabbitMQ producer created")
	
	// Инициализируем Incident Manager client
	logger.GetBaseLogger().Debug("Initializing Incident Manager client")
	incidentClient, err := client.NewIncidentClient(&client.Config{
		Address: config.IncidentManagerAddress,
	})
	if err != nil {
		// RabbitMQ producer закрывается через connection
		conn.Close()
		db.Close()
		return nil, nil, nil, fmt.Errorf("failed to create Incident Manager client: %w", err)
	}
	logger.GetBaseLogger().Info("Incident Manager client created")
	
	return db, rabbitProducer, incidentClient, nil
}

// cleanupComponents очищает ресурсы
func cleanupComponents(db *sql.DB, rabbitProducer *rabbitmq.Producer, incidentClient client.IncidentClient) {
	if incidentClient != nil {
		incidentClient.Close()
	}
	if rabbitProducer != nil {
		// RabbitMQ producer закрывается через connection
	}
	if db != nil {
		db.Close()
	}
}

// performStartupHealthChecks выполняет health checks при запуске
func performStartupHealthChecks(ctx context.Context, healthService *health.Service) error {
	// Используем глобальный логгер
	globalLogger := logging.GetGlobalUptimeLogger()
	globalLogger.GetBaseLogger().Info("Performing startup health checks")
	
	results := healthService.CheckAll(ctx)
	
	// Проверяем результаты
	for name, result := range results {
		if result.Status == health.StatusUnhealthy {
			return fmt.Errorf("component %s is unhealthy: %s", name, result.Message)
		}
		
		if result.Status == health.StatusDegraded {
			globalLogger.GetBaseLogger().Warn("Component is degraded",
				logger.String("component", name),
				logger.String("message", result.Message))
		} else {
			globalLogger.GetBaseLogger().Info("Component is healthy",
				logger.String("component", name))
		}
	}
	
	// Проверяем общий статус
	overallStatus := healthService.GetStatus()
	if overallStatus != health.StatusHealthy {
		return fmt.Errorf("overall health status is %s", overallStatus)
	}
	
	globalLogger.GetBaseLogger().Info("All startup health checks passed")
	return nil
}

// awaitGracefulShutdown ожидает сигналы и выполняет graceful shutdown
func awaitGracefulShutdown(ctx context.Context, uptimeLogger *logging.UptimeLogger, workerPool *worker.Pool, healthService *health.Service) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Ожидаем сигнал
	sig := <-sigChan
	uptimeLogger.GetBaseLogger().Info("Received shutdown signal", logger.String("signal", sig.String()))
	
	// Создаем контекст для graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	uptimeLogger.GetBaseLogger().Info("Starting graceful shutdown")
	
	// Останавливаем прием новых задач
	uptimeLogger.GetBaseLogger().Info("Stopping task submission")
	
	// Останавливаем worker pool
	uptimeLogger.GetBaseLogger().Info("Stopping worker pool")
	if err := workerPool.Stop(shutdownCtx); err != nil {
		uptimeLogger.GetBaseLogger().Error("Error stopping worker pool", logger.Error(err))
	} else {
		uptimeLogger.GetBaseLogger().Info("Worker pool stopped successfully")
	}
	
	// Останавливаем health checker
	uptimeLogger.GetBaseLogger().Info("Stopping health service")
	if err := healthService.Stop(shutdownCtx); err != nil {
		uptimeLogger.GetBaseLogger().Error("Error stopping health service", logger.Error(err))
	} else {
		uptimeLogger.GetBaseLogger().Info("Health service stopped successfully")
	}
	
	// Получаем финальную статистику
	stats := workerPool.GetStats()
	uptimeLogger.GetBaseLogger().Info("Final statistics",
		logger.Int("tasks_received", int(stats.TasksReceived)),
		logger.Int("tasks_completed", int(stats.TasksCompleted)),
		logger.Int("tasks_failed", int(stats.TasksFailed)),
		logger.Int("tasks_retried", int(stats.TasksRetried)),
		logger.Float64("average_duration_ms", stats.AverageDuration))
	
	// Получаем статистику метрик
	uptimeLogger.GetBaseLogger().Info("Service shutdown completed")
	
	os.Exit(0)
}
