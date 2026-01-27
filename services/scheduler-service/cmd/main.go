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
	"UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/pkg/ratelimit"
	scheduler_http "UptimePingPlatform/services/scheduler-service/internal/handler/http"
	"UptimePingPlatform/services/scheduler-service/internal/repository/postgres"
	"UptimePingPlatform/services/scheduler-service/internal/repository/redis"
	"UptimePingPlatform/services/scheduler-service/internal/usecase"
	"gopkg.in/yaml.v2"
)

func main() {
	// Инициализация конфигурации
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация логгера
	appLogger, err := logger.NewLogger(
		cfg.Logger.Level,
		cfg.Logger.Format,
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

	// Инициализация retry конфигурации
	retryConfig := cfg.Retry
	
	// Инициализация базы данных с retry логикой
	dbConfig := loadDatabaseConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var postgresDB *database.Postgres
	err = connection.WithRetry(ctx, retryConfig, func(ctx context.Context) error {
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
	defer postgresDB.Close()

	// Инициализация Redis с retry логикой
	redisConfig := loadRedisConfig(cfg)

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
	
	// Инициализация rate limiter
	rateLimiter := ratelimit.NewRedisRateLimiter(redisClient.Client)

	// Создание HealthChecker
	healthChecker := NewRealHealthChecker(appLogger, postgresDB, redisClient)

	// Настройка HTTP сервера
	handler := setupHandler(checkUseCase, schedulerUseCase, appLogger, healthChecker, metricCollector, rateLimiter)
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

func setupHandler(checkUseCase *usecase.CheckUseCase, schedulerUseCase *usecase.SchedulerUseCase, logger logger.Logger, healthChecker health.HealthChecker, metricCollector *metrics.Metrics, rateLimiter ratelimit.RateLimiter) http.Handler {
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

// AppConfig расширенная конфигурация для scheduler-service
type AppConfig struct {
	config.Config
	Redis     RedisConfig     `json:"redis" yaml:"redis"`
	Scheduler SchedulerConfig `json:"scheduler" yaml:"scheduler"`
	RateLimit RateLimitConfig `json:"rate_limit" yaml:"rate_limit"`
	Retry     connection.RetryConfig `json:"retry" yaml:"retry"`
	Production ProductionConfig `json:"production" yaml:"production"`
}

// RedisConfig конфигурация Redis
type RedisConfig struct {
	Addr           string        `json:"addr" yaml:"addr"`
	Password       string        `json:"password" yaml:"password"`
	DB             int           `json:"db" yaml:"db"`
	PoolSize       int           `json:"pool_size" yaml:"pool_size"`
	MinIdleConn    int           `json:"min_idle_conn" yaml:"min_idle_conn"`
	MaxRetries     int           `json:"max_retries" yaml:"max_retries"`
	RetryInterval  time.Duration `json:"retry_interval" yaml:"retry_interval"`
	HealthCheck    time.Duration `json:"health_check" yaml:"health_check"`
}

// SchedulerConfig конфигурация планировщика
type SchedulerConfig struct {
	MaxConcurrentTasks int           `json:"max_concurrent_tasks" yaml:"max_concurrent_tasks"`
	TaskTimeout        time.Duration `json:"task_timeout" yaml:"task_timeout"`
	CleanupInterval    time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
	LockTimeout        time.Duration `json:"lock_timeout" yaml:"lock_timeout"`
}

// RateLimitConfig конфигурация rate limiting
type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute" yaml:"requests_per_minute"`
	BurstSize         int `json:"burst_size" yaml:"burst_size"`
}

// ProductionConfig конфигурация для production
type ProductionConfig struct {
	Database DatabasePoolConfig `json:"database" yaml:"database"`
}

// DatabasePoolConfig конфигурация пула базы данных
type DatabasePoolConfig struct {
	MaxConns         int           `json:"max_conns" yaml:"max_conns"`
	MinConns         int           `json:"min_conns" yaml:"min_conns"`
	MaxConnLifetime  time.Duration `json:"max_conn_lifetime" yaml:"max_conn_lifetime"`
	MaxConnIdleTime  time.Duration `json:"max_conn_idle_time" yaml:"max_conn_idle_time"`
	HealthCheckPeriod time.Duration `json:"health_check_period" yaml:"health_check_period"`
}

// loadConfig загружает конфигурацию из YAML файла с подстановкой переменных окружения
func loadConfig() (*AppConfig, error) {
	// Загрузка базовой конфигурации
	cfg, err := config.LoadConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load base config: %w", err)
	}

	// Расширенная конфигурация
	appConfig := &AppConfig{
		Config: *cfg,
	}

	// Загрузка из файла config.yaml с подстановкой переменных окружения
	if data, err := os.ReadFile("config/config.yaml"); err == nil {
		// Простая подстановка переменных окружения вида ${VAR:default}
		configContent := string(data)
		configContent = os.ExpandEnv(configContent)
		
		if err := yaml.Unmarshal([]byte(configContent), appConfig); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else {
		// Использование значений по умолчанию, если файл не найден
		appConfig := &AppConfig{
			Redis: RedisConfig{
				Addr:           os.Getenv("REDIS_ADDR"),
				DB:             0,
				PoolSize:       10,
				MinIdleConn:    2,
				MaxRetries:     3,
				RetryInterval:  1 * time.Second,
				HealthCheck:    30 * time.Second,
			},
		}
		
		if appConfig.Redis.Addr == "" {
			appConfig.Redis.Addr = "localhost:6379"
		}
		appConfig.Scheduler = SchedulerConfig{
			MaxConcurrentTasks: 10,
			TaskTimeout:        30 * time.Second,
			CleanupInterval:    1 * time.Hour,
			LockTimeout:        5 * time.Minute,
		}
		appConfig.RateLimit = RateLimitConfig{
			RequestsPerMinute: 60,
			BurstSize:         10,
		}
		appConfig.Retry = connection.DefaultRetryConfig()
	}

	return appConfig, nil
}

// loadRedisConfig создает конфигурацию для pkg/redis
func loadRedisConfig(cfg *AppConfig) *pkg_redis.Config {
	redisConfig := pkg_redis.NewConfig()
	redisConfig.Addr = cfg.Redis.Addr
	redisConfig.Password = cfg.Redis.Password
	redisConfig.DB = cfg.Redis.DB
	redisConfig.PoolSize = cfg.Redis.PoolSize
	redisConfig.MinIdleConn = cfg.Redis.MinIdleConn
	redisConfig.MaxRetries = cfg.Redis.MaxRetries
	redisConfig.RetryInterval = cfg.Redis.RetryInterval
	redisConfig.HealthCheck = cfg.Redis.HealthCheck
	return redisConfig
}

// loadDatabaseConfig создает конфигурацию для pkg/database
func loadDatabaseConfig(cfg *AppConfig) *database.Config {
	dbConfig := database.NewConfig()
	dbConfig.Host = cfg.Database.Host
	dbConfig.Port = cfg.Database.Port
	dbConfig.User = cfg.Database.User
	dbConfig.Password = cfg.Database.Password
	dbConfig.Database = cfg.Database.Name
	
	// Production настройки, если доступны
	if cfg.Production.Database.MaxConns > 0 {
		dbConfig.MaxConns = cfg.Production.Database.MaxConns
	}
	if cfg.Production.Database.MinConns > 0 {
		dbConfig.MinConns = cfg.Production.Database.MinConns
	}
	if cfg.Production.Database.MaxConnLifetime > 0 {
		dbConfig.MaxConnLife = cfg.Production.Database.MaxConnLifetime
	}
	if cfg.Production.Database.MaxConnIdleTime > 0 {
		dbConfig.MaxConnIdle = cfg.Production.Database.MaxConnIdleTime
	}
	if cfg.Production.Database.HealthCheckPeriod > 0 {
		dbConfig.HealthCheck = cfg.Production.Database.HealthCheckPeriod
	}
	
	return dbConfig
}
