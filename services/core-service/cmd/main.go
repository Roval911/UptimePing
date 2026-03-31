package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_rabbitmq "UptimePingPlatform/pkg/rabbitmq"
	pkg_redis "UptimePingPlatform/pkg/redis"
	corev1 "UptimePingPlatform/proto/api/core/v1"
	grpcHandler "UptimePingPlatform/services/core-service/internal/handler/grpc"
	"UptimePingPlatform/services/core-service/internal/repository"
	"UptimePingPlatform/services/core-service/internal/repository/postgres"
	coreservice "UptimePingPlatform/services/core-service/internal/service"
	"UptimePingPlatform/services/core-service/internal/service/checker"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfigWithAutoPath("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.NewLogger(cfg.Environment, cfg.Logger.Level, "core-service", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	appLogger.Info("Starting Core Service...")

	// Initialize database connection
	db, err := database.Connect(context.Background(), &database.Config{
		Host:          cfg.Database.Host,
		Port:          cfg.Database.Port,
		User:          cfg.Database.User,
		Password:      cfg.Database.Password,
		Database:      cfg.Database.Name,
		MaxConns:      20,
		MinConns:      5,
		MaxConnLife:   30 * time.Minute,
		MaxConnIdle:   5 * time.Minute,
		HealthCheck:   30 * time.Second,
		MaxRetries:    3,
		RetryInterval: time.Second,
	})
	if err != nil {
		appLogger.Error("Failed to connect to database", logger.Error(err))
	} else {
		defer db.Close()
	}

	// Initialize Redis client
	redisClient, err := pkg_redis.Connect(context.Background(), &pkg_redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		appLogger.Error("Failed to connect to Redis", logger.Error(err))
	} else {
		defer redisClient.Close()
	}

	// Initialize RabbitMQ connection and producer
	rabbitConn, err := pkg_rabbitmq.Connect(context.Background(), &pkg_rabbitmq.Config{
		URL:        cfg.RabbitMQ.URL,
		Exchange:   cfg.RabbitMQ.Exchange,
		RoutingKey: cfg.RabbitMQ.RoutingKey,
		Queue:      cfg.RabbitMQ.Queue,
	})
	if err != nil {
		appLogger.Error("Failed to connect to RabbitMQ", logger.Error(err))
	} else {
		defer rabbitConn.Close()
	}

	// Initialize checker factory and check service
	checkerFactory := checker.NewDefaultCheckerFactory(appLogger, checker.NewDefaultHTTPClient(30*time.Second))

	var repository repository.CheckResultRepository
	if db != nil {
		repository = postgres.NewCheckResultRepository(db.Pool, appLogger)
	}

	checkService := coreservice.NewCheckService(appLogger, checkerFactory, repository, redisClient, nil)

	// Initialize RabbitMQ consumer for tasks
	var taskConsumer *TaskConsumer
	if rabbitConn != nil && checkService != nil {
		consumer, err := NewTaskConsumer(rabbitConn, checkService, appLogger)
		if err != nil {
			appLogger.Error("Failed to create task consumer", logger.Error(err))
		} else {
			// Setup consumer
			if err := consumer.Setup(); err != nil {
				appLogger.Error("Failed to setup task consumer", logger.Error(err))
			} else {
				// Start consumer in background
				go func() {
					if err := consumer.Start(context.Background()); err != nil {
						appLogger.Error("Task consumer failed", logger.Error(err))
					}
				}()
				taskConsumer = consumer
				appLogger.Info("Task consumer started successfully")
			}
			defer taskConsumer.Close()
		}
	} else {
		appLogger.Warn("RabbitMQ connection or check service not available, running without task consumer")
	}

	// Initialize metrics
	appMetrics := metrics.NewMetrics("core-service")
	metricsHandler := appMetrics.GetHandler()

	// Initialize health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

	// Start HTTP server for metrics and health
	httpPort := 51054 // Default HTTP port
	if envPort := os.Getenv("HTTP_SERVER_PORT"); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil {
			httpPort = port
		}
	}

	appLogger.Info(fmt.Sprintf("HTTP_SERVER_PORT env var: %s, using port: %d", os.Getenv("HTTP_SERVER_PORT"), httpPort))

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: setupHTTPHandler(metricsHandler, healthChecker, appLogger),
	}

	// Start gRPC server for Core Service
	grpcPort := cfg.Server.Port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		appLogger.Error("Failed to listen for gRPC", logger.Error(err))
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	appLogger.Info(fmt.Sprintf("Registering Core gRPC handler on port %d", grpcPort))
	coreHandler := grpcHandler.NewCoreHandler(checkService, appLogger)
	corev1.RegisterCoreServiceServer(grpcServer, coreHandler)

	// Start gRPC server
	go func() {
		appLogger.Info(fmt.Sprintf("Starting gRPC server on port %d", grpcPort))
		if err := grpcServer.Serve(lis); err != nil {
			appLogger.Error("gRPC server failed", logger.Error(err))
		}
	}()
	// Wait briefly to ensure gRPC server starts
	time.Sleep(1 * time.Second)

	// Start server
	go func() {
		appLogger.Info(fmt.Sprintf("Starting HTTP server on port %d", httpPort))
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

	if err := httpServer.Shutdown(ctx); err != nil {
		appLogger.Error("Server shutdown failed", logger.Error(err))
	}

	appLogger.Info("Server stopped")
}

func setupHTTPHandler(metricsHandler http.Handler, healthChecker health.HealthChecker, appLogger logger.Logger) http.Handler {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", metricsHandler)

	// Health endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"core-service"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"core-service"}`))
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"live","service":"core-service"}`))
	})

	// Core service endpoints
	mux.HandleFunc("/api/v1/checks/execute", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Core Service - Execute Check endpoint","status":"ok"}`))
	})

	mux.HandleFunc("/api/v1/checks/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Core Service - Check Status endpoint","status":"ok"}`))
	})

	mux.HandleFunc("/api/v1/checks/history", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Core Service - Check History endpoint","status":"ok"}`))
	})

	return mux
}

// RabbitMQ consumer integration

// TaskMessage represents a task message from RabbitMQ
type TaskMessage struct {
	CheckID   string                 `json:"check_id"`
	TenantID  string                 `json:"tenant_id"`
	TaskType  string                 `json:"task_type"`
	Target    string                 `json:"target"`
	Timeout   int                    `json:"timeout"`
	Config    map[string]interface{} `json:"config,omitempty"`
	CreatedAt string                 `json:"created_at"`
}

type TaskConsumer struct {
	conn         *pkg_rabbitmq.Connection
	channel      *amqp.Channel
	queue        string
	exchange     string
	routingKey   string
	checkService CheckServiceInterface
	logger       logger.Logger
}

type CheckServiceInterface interface {
	ProcessTask(ctx context.Context, message []byte) error
}

func NewTaskConsumer(conn *pkg_rabbitmq.Connection, checkService CheckServiceInterface, logger logger.Logger) (*TaskConsumer, error) {
	channel := conn.Channel()
	if channel == nil {
		return nil, fmt.Errorf("failed to get RabbitMQ channel")
	}

	return &TaskConsumer{
		conn:         conn,
		channel:      channel,
		queue:        "checks.tasks",
		exchange:     "checks",
		routingKey:   "check.task",
		checkService: checkService,
		logger:       logger,
	}, nil
}

func (c *TaskConsumer) Setup() error {
	err := c.channel.ExchangeDeclare(
		c.exchange, "topic", true, false, false, false, nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	q, err := c.channel.QueueDeclare(
		c.queue, true, false, false, false, nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	err = c.channel.QueueBind(
		q.Name, c.routingKey, c.exchange, false, nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	return nil
}

func (c *TaskConsumer) Start(ctx context.Context) error {
	err := c.channel.Qos(1, 0, false)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := c.channel.Consume(
		c.queue, "", false, false, false, false, nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	c.logger.Info("Task consumer started",
		logger.String("queue", c.queue),
		logger.String("exchange", c.exchange),
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}

				if err := c.handleMessage(ctx, msg); err != nil {
					c.logger.Error("Failed to handle message", logger.Error(err))
					msg.Nack(false, true)
				} else {
					msg.Ack(false)
				}
			}
		}
	}()

	return nil
}

func (c *TaskConsumer) handleMessage(ctx context.Context, msg amqp.Delivery) error {
	c.logger.Debug("Processing task message",
		logger.String("message_id", msg.MessageId),
		logger.String("routing_key", msg.RoutingKey),
		logger.String("content_type", msg.ContentType),
	)

	// Process task through check service
	return c.checkService.ProcessTask(ctx, msg.Body)
}

func (c *TaskConsumer) Close() error {
	if c.channel != nil && !c.channel.IsClosed() {
		return c.channel.Close()
	}
	return nil
}
