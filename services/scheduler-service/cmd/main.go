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
	"UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_redis "UptimePingPlatform/pkg/redis"
	scheduler_http "UptimePingPlatform/services/scheduler-service/internal/handler/http"
	"UptimePingPlatform/services/scheduler-service/internal/repository/postgres"
	"UptimePingPlatform/services/scheduler-service/internal/repository/redis"
	"UptimePingPlatform/services/scheduler-service/internal/usecase"
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
		"scheduler-service",
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

	// Инициализация базы данных
	dbConfig := database.NewConfig()
	dbConfig.Host = cfg.Database.Host
	dbConfig.Port = cfg.Database.Port
	dbConfig.User = cfg.Database.User
	dbConfig.Password = cfg.Database.Password
	dbConfig.Database = cfg.Database.Name

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	postgresDB, err := database.Connect(ctx, dbConfig)
	if err != nil {
		appLogger.Error("Failed to connect to database", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer postgresDB.Close()

	// Инициализация Redis (для планировщика)
	redisConfig := pkg_redis.NewConfig()
	redisConfig.Addr = "localhost:6379" // В реальном приложении брать из конфига

	redisCtx, redisCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer redisCancel()

	redisClient, err := pkg_redis.Connect(redisCtx, redisConfig)
	if err != nil {
		appLogger.Error("Failed to connect to redis", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer redisClient.Close()

	// Инициализация репозиториев
	checkRepo := postgres.NewCheckRepository(postgresDB.Pool)
	schedulerRepo := postgres.NewSchedulerRepository(postgresDB.Pool, redisClient.Client)
	taskRepo := postgres.NewTaskRepository(postgresDB.Pool)
	lockRepo := redis.NewRedisLockRepository(redisClient.Client)

	// Инициализация use case с логгером
	checkUseCase := usecase.NewCheckUseCase(checkRepo, schedulerRepo, appLogger)
	schedulerUseCase := usecase.NewSchedulerUseCase(checkRepo, taskRepo, lockRepo, schedulerRepo, appLogger)

	// Инициализация метрик
	metricCollector := metrics.NewMetrics("scheduler_service")

	// Создание HealthChecker
	healthChecker := NewRealHealthChecker(appLogger, postgresDB, redisClient)

	// Настройка HTTP сервера
	handler := setupHandler(checkUseCase, schedulerUseCase, appLogger, healthChecker, metricCollector)
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: handler,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		appLogger.Info("Starting scheduler service server", logger.String("addr", server.Addr))
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
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Error("Server shutdown failed", logger.String("error", err.Error()))
	}

	appLogger.Info("Server stopped")
}

// RealHealthChecker реальная реализация HealthChecker
type RealHealthChecker struct {
	logger      logger.Logger
	database    *database.Postgres
	redisClient *pkg_redis.Client
}

// NewRealHealthChecker создает новый экземпляр RealHealthChecker
func NewRealHealthChecker(logger logger.Logger, database *database.Postgres, redisClient *pkg_redis.Client) *RealHealthChecker {
	return &RealHealthChecker{
		logger:      logger,
		database:    database,
		redisClient: redisClient,
	}
}

// Check проверяет здоровье сервиса
func (h *RealHealthChecker) Check() *health.HealthStatus {
	status := &health.HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Services:  make(map[string]health.Status),
	}

	// Проверка базы данных
	if h.database != nil {
		if err := h.database.HealthCheck(context.Background()); err != nil {
			status.Status = "degraded"
			status.Services["database"] = health.Status{
				Status:  "unhealthy",
				Details: err.Error(),
			}
		} else {
			status.Services["database"] = health.Status{
				Status: "healthy",
			}
		}
	}

	// Проверка Redis
	if h.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := h.redisClient.Client.Ping(ctx).Result(); err != nil {
			status.Status = "degraded"
			status.Services["redis"] = health.Status{
				Status:  "unhealthy",
				Details: err.Error(),
			}
		} else {
			status.Services["redis"] = health.Status{
				Status: "healthy",
			}
		}
	}

	// Если есть нездоровые сервисы, меняем общий статус
	for _, serviceStatus := range status.Services {
		if serviceStatus.Status != "healthy" {
			status.Status = "degraded"
			break
		}
	}

	return status
}

func setupHandler(checkUseCase *usecase.CheckUseCase, schedulerUseCase *usecase.SchedulerUseCase, logger logger.Logger, healthChecker health.HealthChecker, metricCollector *metrics.Metrics) http.Handler {
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("/health", health.Handler(healthChecker))
	mux.HandleFunc("/ready", health.ReadyHandler(healthChecker))
	mux.HandleFunc("/live", health.LiveHandler())

	// Metrics endpoint
	mux.Handle("/metrics", metricCollector.GetHandler())

	// Scheduler endpoints
	schedulerHandler := scheduler_http.NewSchedulerHandler(schedulerUseCase, logger)
	mux.HandleFunc("/api/v1/scheduler/start", schedulerHandler.Start)
	mux.HandleFunc("/api/v1/scheduler/stop", schedulerHandler.Stop)
	mux.HandleFunc("/api/v1/scheduler/stats", schedulerHandler.Stats)
	mux.HandleFunc("/api/v1/scheduler/execute", schedulerHandler.ExecuteTask)

	// Check management endpoints (будут добавлены позже)
	mux.HandleFunc("/api/v1/checks", func(w http.ResponseWriter, r *http.Request) {
		// Заглушка для API
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","message":"Scheduler Service API"}`))
	})

	// Оборачиваем в errors middleware для обработки ошибок
	return errors.Middleware(mux)
}
