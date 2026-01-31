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
	"UptimePingPlatform/services/api-gateway/internal/middleware"
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

	// Create Auth Service HTTP client
	authServiceAddr := os.Getenv("AUTH_SERVICE_ADDR")
	if authServiceAddr == "" {
		authServiceAddr = "http://localhost:50051"
	}
	httpAuthClient, err := client.NewHTTPAuthClient(authServiceAddr, 10*time.Second, appLogger)
	if err != nil {
		appLogger.Error("Failed to create HTTP Auth client", logger.Error(err))
		log.Fatalf("HTTP Auth client creation failed: %v", err)
	}
	authAdapter := client.NewAuthHTTPAdapter(httpAuthClient)
	appLogger.Info("Auth Service HTTP client created successfully")

	// Create real gRPC clients for all services - make them optional for startup
	var schedulerClient *client.SchedulerClient
	var coreClient *client.CoreClient
	var metricsClient *client.MetricsClient
	var incidentClient *client.IncidentClient
	var notificationClient *client.NotificationClient
	var forgeClient *client.GRPCForgeClient

	// Try to connect to services, but don't fail if they're not available
	if schedulerClient, err = client.NewSchedulerClient("scheduler-service:50052", 10*time.Second, appLogger); err != nil {
		appLogger.Warn("Scheduler Service client creation failed, continuing without it", logger.Error(err))
	}
	if coreClient, err = client.NewCoreClient("core-service:50054", 2*time.Second, appLogger); err != nil {
		appLogger.Warn("Core Service client creation failed, continuing without it", logger.Error(err))
	}
	if metricsClient, err = client.NewMetricsClient("metrics-service:50055", 2*time.Second, appLogger); err != nil {
		appLogger.Warn("Metrics Service client creation failed, continuing without it", logger.Error(err))
	}
	if incidentClient, err = client.NewIncidentClient("incident-manager:50056", 2*time.Second, appLogger); err != nil {
		appLogger.Warn("Incident Service client creation failed, continuing without it", logger.Error(err))
	}
	if notificationClient, err = client.NewNotificationClient("notification-service:50057", 2*time.Second, appLogger); err != nil {
		appLogger.Warn("Notification Service client creation failed, continuing without it", logger.Error(err))
	}
	if forgeClient, err = client.NewGRPCForgeClient("forge-service:50053", 2*time.Second, appLogger); err != nil {
		appLogger.Warn("Forge Service client creation failed, continuing without it", logger.Error(err))
	}

	// Create Config Service client
	configClient, err := client.NewConfigClient(2*time.Second, appLogger)
	if err != nil {
		appLogger.Warn("Config Service client creation failed, continuing without it", logger.Error(err))
	}

	// Create HTTP handler
	httpHandlerInstance := httpHandler.NewHandler(
		authAdapter,
		healthChecker,
		schedulerClient,
		coreClient,
		metricsClient,
		incidentClient,
		notificationClient,
		configClient,
		forgeClient,
		appLogger,
	)

	// Start HTTP server with middleware
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: appMetrics.Middleware(middleware.AuthMiddleware(httpAuthClient, appLogger)(httpHandlerInstance)),
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
