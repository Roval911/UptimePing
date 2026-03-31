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
	pkg_database "UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_rabbitmq "UptimePingPlatform/pkg/rabbitmq"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/notification-service/internal/handler"
	"UptimePingPlatform/services/notification-service/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfigWithAutoPath("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.NewLogger(cfg.Environment, cfg.Logger.Level, "notification-service", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	appLogger.Info("Starting Notification Service...")

	// Initialize metrics
	appMetrics := metrics.NewMetrics("notification-service")
	metricsHandler := appMetrics.GetHandler()

	// Initialize health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

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

	// Initialize PostgreSQL connection
	db, err := pkg_database.Connect(context.Background(), &pkg_database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Database: cfg.Database.Name,
		SSLMode:  "disable",
		MaxConns: 20,
		MinConns: 5,
	})
	if err != nil {
		appLogger.Error("Failed to connect to PostgreSQL", logger.Error(err))
	} else {
		defer db.Close()
	}

	// Initialize RabbitMQ connection
	rabbitConn, err := pkg_rabbitmq.Connect(context.Background(), &pkg_rabbitmq.Config{
		URL:        cfg.RabbitMQ.URL,
		Exchange:   cfg.RabbitMQ.Exchange,
		RoutingKey: cfg.RabbitMQ.RoutingKey,
		Queue:      cfg.RabbitMQ.Queue,
		DLX:        cfg.RabbitMQ.DLX,
		DLQ:        cfg.RabbitMQ.DLQ,
	})
	if err != nil {
		appLogger.Error("Failed to connect to RabbitMQ", logger.Error(err))
	} else {
		defer rabbitConn.Close()
		appLogger.Info("RabbitMQ connected successfully")
	}

	// Start HTTP server for metrics and health
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: setupHTTPHandler(metricsHandler, healthChecker, appLogger),
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

func setupHTTPHandler(metricsHandler http.Handler, healthChecker health.HealthChecker, appLogger logger.Logger) http.Handler {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", metricsHandler)

	// Health endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"notification-service"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"notification-service"}`))
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"live","service":"notification-service"}`))
	})

	// Initialize notification service
	notificationService := service.NewNotificationService(appLogger)
	httpHandler := handler.NewHTTPHandler(appLogger, notificationService)

	// Register notification routes
	httpHandler.RegisterRoutes(mux)

	return mux
}
