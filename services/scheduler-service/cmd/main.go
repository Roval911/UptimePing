package main

import (
	pkg_database "UptimePingPlatform/pkg/database"
	pkg_rabbitmq "UptimePingPlatform/pkg/rabbitmq"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	"UptimePingPlatform/services/scheduler-service/internal/domain"

	schedulerv1 "UptimePingPlatform/proto/api/scheduler/v1"
	grpcHandler "UptimePingPlatform/services/scheduler-service/internal/handler/grpc"
	"UptimePingPlatform/services/scheduler-service/internal/repository"
	postgresRepo "UptimePingPlatform/services/scheduler-service/internal/repository/postgres"
	"UptimePingPlatform/services/scheduler-service/internal/usecase"
	"google.golang.org/grpc"
)

func main() {
	fmt.Println("=== ШАГ 1: Начало main функции ===")

	// Load configuration
	fmt.Println("=== ШАГ 2: Загрузка конфигурации ===")
	cfg, err := config.LoadConfigWithAutoPath("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	fmt.Println("=== ШАГ 3: Конфигурация загружена ===")

	// Initialize logger
	fmt.Println("=== ШАГ 4: Инициализация логгера ===")
	appLogger, err := logger.NewLogger(cfg.Environment, cfg.Logger.Level, "scheduler-service", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()
	fmt.Println("=== ШАГ 5: Логгер инициализирован ===")

	appLogger.Info("Starting Scheduler Service...")
	fmt.Println("=== ШАГ 6: Сообщение в лог отправлено ===")

	// Initialize metrics
	appMetrics := metrics.NewMetrics("scheduler-service")
	metricsHandler := appMetrics.GetHandler()

	// Initialize health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

	// Initialize context
	ctx := context.Background()

	// Initialize database connection
	db, err := pkg_database.Connect(ctx, &pkg_database.Config{
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
	})
	if err != nil {
		appLogger.Error("Failed to connect to database", logger.Error(err))
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	// Initialize Redis client
	redisClient, err := pkg_redis.Connect(ctx, &pkg_redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		appLogger.Error("Failed to connect to Redis", logger.Error(err))
	} else {
		defer redisClient.Close()
	}

	// Initialize repositories
	checkRepo := postgresRepo.NewCheckRepository(db.Pool)

	// Initialize scheduler repository with Redis client if available
	var schedulerRepo repository.SchedulerRepository
	if redisClient != nil && redisClient.Client != nil {
		schedulerRepo = postgresRepo.NewSchedulerRepository(db.Pool, redisClient.Client)
		appLogger.Info("Scheduler repository initialized with Redis")
	} else {
		schedulerRepo = postgresRepo.NewSchedulerRepository(db.Pool, nil)
		appLogger.Warn("Scheduler repository initialized without Redis")
	}

	// Initialize use case
	checkUseCase := usecase.NewCheckUseCase(checkRepo, schedulerRepo, appLogger)

	// Initialize RabbitMQ producer
	var rabbitProducer *RabbitMQProducer
	if cfg.RabbitMQ.URL != "" {
		producer, err := NewRabbitMQProducer(cfg, appLogger)
		if err != nil {
			appLogger.Error("Failed to initialize RabbitMQ producer", logger.Error(err))
		} else {
			rabbitProducer = producer
			defer rabbitProducer.Close()
			appLogger.Info("RabbitMQ producer initialized successfully")
		}
	} else {
		appLogger.Warn("RabbitMQ URL not configured, running without task publishing")
	}

	appLogger.Info("Starting gRPC server...")
	grpcPort := cfg.Server.Port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		appLogger.Error("Failed to listen", logger.Error(err))
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	appLogger.Info("Creating gRPC handler...")
	schedulerHandler := grpcHandler.NewHandlerFixed(checkUseCase, appLogger, rabbitProducer)
	appLogger.Info("gRPC handler created successfully")

	appLogger.Info("Registering gRPC service...")
	schedulerv1.RegisterSchedulerServiceServer(grpcServer, schedulerHandler)
	appLogger.Info("gRPC service registered successfully")

	// Start gRPC server
	go func() {
		appLogger.Info(fmt.Sprintf("Starting gRPC server on port %d", grpcPort))
		if err := grpcServer.Serve(lis); err != nil {
			appLogger.Error("gRPC server failed", logger.Error(err))
		}
	}()

	// Wait a moment for gRPC server to start
	time.Sleep(2 * time.Second)
	appLogger.Info("gRPC server started successfully and is ready to accept connections")

	// Start HTTP server for metrics and health
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port+1000), // Health check on port +1000
		Handler: setupHTTPHandler(metricsHandler, healthChecker, appLogger),
	}

	// Start server
	go func() {
		appLogger.Info(fmt.Sprintf("Starting HTTP server on port %d", cfg.Server.Port+1000))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("HTTP server failed", logger.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Graceful shutdown gRPC server
	grpcServer.GracefulStop()

	// Graceful shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		appLogger.Error("HTTP server shutdown failed", logger.Error(err))
	}

	appLogger.Info("Server exited properly")
}

func setupHTTPHandler(metricsHandler http.Handler, healthChecker health.HealthChecker, appLogger logger.Logger) http.Handler {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", metricsHandler)

	// Health endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"scheduler-service"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"scheduler-service"}`))
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"live","service":"scheduler-service"}`))
	})

	// Scheduler endpoints
	mux.HandleFunc("/api/v1/checks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Scheduler Service - Checks endpoint","status":"ok"}`))
	})

	mux.HandleFunc("/api/v1/schedules", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Scheduler Service - Schedules endpoint","status":"ok"}`))
	})

	mux.HandleFunc("/api/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Scheduler Service - Jobs endpoint","status":"ok"}`))
	})

	return mux
}

// RabbitMQ producer integration
type RabbitMQProducer struct {
	producer *pkg_rabbitmq.Producer
	logger   logger.Logger
	config   *config.Config
}

type TaskMessage struct {
	CheckID     string                 `json:"check_id"`
	ExecutionID string                 `json:"execution_id"`
	Target      string                 `json:"target"`
	Type        string                 `json:"type"`
	Config      map[string]interface{} `json:"config"`
	ScheduledAt string                 `json:"scheduled_at"`
	TenantID    string                 `json:"tenant_id"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func NewRabbitMQProducer(cfg *config.Config, logger logger.Logger) (*RabbitMQProducer, error) {
	conn, err := pkg_rabbitmq.Connect(context.Background(), &pkg_rabbitmq.Config{
		URL:        cfg.RabbitMQ.URL,
		Exchange:   cfg.RabbitMQ.Exchange,
		RoutingKey: cfg.RabbitMQ.RoutingKey,
		Queue:      cfg.RabbitMQ.Queue,
		DLX:        cfg.RabbitMQ.DLX,
		DLQ:        cfg.RabbitMQ.DLQ,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	producer := pkg_rabbitmq.NewProducer(conn, &pkg_rabbitmq.Config{
		URL:        cfg.RabbitMQ.URL,
		Exchange:   cfg.RabbitMQ.Exchange,
		RoutingKey: cfg.RabbitMQ.RoutingKey,
		Queue:      cfg.RabbitMQ.Queue,
		DLX:        cfg.RabbitMQ.DLX,
		DLQ:        cfg.RabbitMQ.DLQ,
	})

	return &RabbitMQProducer{
		producer: producer,
		logger:   logger,
		config:   cfg,
	}, nil
}

func (p *RabbitMQProducer) PublishTask(ctx context.Context, check *domain.Check, tenantID string) error {
	// Проверяем, что producer не nil
	if p.producer == nil {
		p.logger.Error("RabbitMQ producer is nil during PublishTask")
		return fmt.Errorf("rabbitmq producer is nil")
	}

	p.logger.Info("DEBUG: About to publish to RabbitMQ",
		logger.String("check_id", check.ID),
		logger.String("tenant_id", tenantID),
		logger.Bool("producer_is_nil", p.producer == nil))

	taskMessage := TaskMessage{
		CheckID:     check.ID,
		ExecutionID: fmt.Sprintf("exec-%d-%s", time.Now().Unix(), uuid.New().String()[:8]),
		Target:      check.Target,
		Type:        string(check.Type),
		Config:      check.Config,
		ScheduledAt: time.Now().UTC().Format(time.RFC3339),
		TenantID:    tenantID,
		Metadata: map[string]interface{}{
			"timeout":  check.Timeout,
			"interval": check.Interval,
		},
	}

	payload, err := json.Marshal(taskMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal task message: %w", err)
	}

	p.logger.Info("DEBUG: TaskMessage payload",
		logger.String("payload", string(payload)))

	p.logger.Info("DEBUG: Publishing to RabbitMQ",
		logger.String("exchange", p.config.RabbitMQ.Exchange),
		logger.String("routing_key", "check.task"))

	err = p.producer.Publish(ctx, payload,
		pkg_rabbitmq.WithExchange(p.config.RabbitMQ.Exchange),
		pkg_rabbitmq.WithRoutingKey("check.task"),
	)
	if err != nil {
		p.logger.Error("Failed to publish task to RabbitMQ",
			logger.String("check_id", check.ID),
			logger.String("tenant_id", tenantID),
			logger.Error(err),
		)
		return fmt.Errorf("failed to publish task: %w", err)
	}

	p.logger.Info("Task published to RabbitMQ successfully",
		logger.String("check_id", check.ID),
		logger.String("tenant_id", tenantID),
	)

	return nil
}

func (p *RabbitMQProducer) Close() error {
	// Producer doesn't have explicit Close method
	// Connection will be closed by the caller
	return nil
}
