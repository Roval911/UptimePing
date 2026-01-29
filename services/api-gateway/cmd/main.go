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
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_redis "UptimePingPlatform/pkg/redis"
	
	"UptimePingPlatform/services/api-gateway/internal/client"
	httpHandler "UptimePingPlatform/services/api-gateway/internal/handler/http"
)

// HealthHandlerAdapter адаптер для health.SimpleHealthChecker
type HealthHandlerAdapter struct {
	checker *health.SimpleHealthChecker
}

func (h *HealthHandlerAdapter) HealthCheck(w http.ResponseWriter, r *http.Request) {
	health.Handler(h.checker)(w, r)
}

func (h *HealthHandlerAdapter) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	health.ReadyHandler(h.checker)(w, r)
}

func (h *HealthHandlerAdapter) LiveCheck(w http.ResponseWriter, r *http.Request) {
	health.LiveHandler()(w, r)
}

func main() {
	// Load configuration with default environment
	cfg, err := config.LoadConfigWithAutoPath("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.NewLogger(cfg.Environment, cfg.Logger.Level, "api-gateway", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	appLogger.Info("Starting API Gateway...")

	// Initialize metrics
	appMetrics := metrics.NewMetrics("api-gateway")

	// Initialize health checker
	healthChecker := &HealthHandlerAdapter{
		checker: health.NewSimpleHealthChecker("1.0.0"),
	}

	// Initialize Redis client with context
	ctx := context.Background()
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

	// Create gRPC connections
	appLogger.Info("Connecting to Auth Service...")
	// Убираем прямое gRPC подключение, так как клиенты создают свои соединения

	// Create Auth Service gRPC client
	appLogger.Info("Creating Auth Service gRPC client...")
	authClient, err := client.NewGRPCAuthClient("localhost:50051", 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to create Auth Service client", logger.Error(err))
		log.Fatalf("Auth Service client creation failed: %v", err)
	}
	appLogger.Info("Auth Service gRPC client created successfully")

	// Create Config Service client
	configClient, err := client.NewConfigClient(5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to create Config Service client", logger.Error(err))
		log.Fatalf("Config Service client creation failed: %v", err)
	}

	// Create real gRPC clients for all services
	schedulerClient, err := client.NewSchedulerClient("localhost:50052", 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to create Scheduler Service client", logger.Error(err))
		log.Fatalf("Scheduler Service client creation failed: %v", err)
	}

	coreClient, err := client.NewCoreClient("localhost:50054", 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to create Core Service client", logger.Error(err))
		log.Fatalf("Core Service client creation failed: %v", err)
	}

	metricsClient, err := client.NewMetricsClient("localhost:50055", 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to create Metrics Service client", logger.Error(err))
		log.Fatalf("Metrics Service client creation failed: %v", err)
	}

	incidentClient, err := client.NewIncidentClient("localhost:50056", 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to create Incident Service client", logger.Error(err))
		log.Fatalf("Incident Service client creation failed: %v", err)
	}

	notificationClient, err := client.NewNotificationClient("localhost:50057", 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to create Notification Service client", logger.Error(err))
		log.Fatalf("Notification Service client creation failed: %v", err)
	}

	forgeClient, err := client.NewGRPCForgeClient("localhost:50053", 5*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to create Forge Service client", logger.Error(err))
		log.Fatalf("Forge Service client creation failed: %v", err)
	}

	// Create HTTP handler
	httpHandlerInstance := httpHandler.NewHandler(
		authClient,
		healthChecker,
		*schedulerClient,
		*coreClient,
		*metricsClient,
		*incidentClient,
		*notificationClient,
		*configClient,
		*forgeClient,
		appLogger,
	)

	// Start HTTP server with middleware
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: appMetrics.Middleware(httpHandlerInstance),
	}

	// Start server
	go func() {
		appLogger.Info(fmt.Sprintf("Starting HTTP server on port %d", cfg.Server.Port))
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
