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
	"UptimePingPlatform/pkg/health"
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/services/incident-manager/internal/handler"
	postgresRepo "UptimePingPlatform/services/incident-manager/internal/repository/postgres"
	"UptimePingPlatform/services/incident-manager/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfigWithAutoPath("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := pkglogger.NewLogger(cfg.Environment, cfg.Logger.Level, "incident-manager", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	appLogger.Info("Starting Incident Manager...")

	// Initialize metrics
	appMetrics := metrics.NewMetrics("incident-manager")
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
		appLogger.Error("Failed to connect to Redis", pkglogger.Error(err))
	} else {
		defer redisClient.Close()
	}

	// Initialize RabbitMQ connection
	rabbitConn, err := rabbitmq.Connect(context.Background(), &rabbitmq.Config{
		URL:        "amqp://guest:guest@localhost:5672/",
		Exchange:   "incidents",
		RoutingKey: "incident.events",
		Queue:      "incident.events",
	})
	if err != nil {
		appLogger.Error("Failed to connect to RabbitMQ", pkglogger.Error(err))
	} else {
		defer rabbitConn.Close()
	}

	// Initialize PostgreSQL connection
	pgConfig := database.NewConfig()
	pgConfig.Host = cfg.Database.Host
	pgConfig.Port = cfg.Database.Port
	pgConfig.User = cfg.Database.User
	pgConfig.Password = cfg.Database.Password
	pgConfig.Database = cfg.Database.Name
	
	appLogger.Info("Attempting to connect to PostgreSQL",
		pkglogger.String("host", cfg.Database.Host),
		pkglogger.Int("port", cfg.Database.Port),
		pkglogger.String("database", cfg.Database.Name),
		pkglogger.String("user", cfg.Database.User),
	)
	
	var incidentService service.IncidentService

	appLogger.Info("Attempting PostgreSQL connection with retry logic")
	pgClient, err := database.Connect(context.Background(), pgConfig)
	if err != nil {
		appLogger.Error("Failed to connect to PostgreSQL after retries", 
			pkglogger.Error(err),
			pkglogger.Int("max_retries", pgConfig.MaxRetries),
			pkglogger.String("retry_interval", pgConfig.RetryInterval.String()),
		)
		// Fallback to in-memory service if PostgreSQL fails
		appLogger.Warn("Falling back to in-memory incident service")
		redisClient, _ := pkg_redis.Connect(context.Background(), &pkg_redis.Config{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		})
		incidentService = service.NewInMemoryIncidentService(appLogger, *redisClient)
	} else {
		defer pgClient.Close()
		
		appLogger.Info("Successfully connected to PostgreSQL")
		
		// Test database connection
		if err := pgClient.HealthCheck(context.Background()); err != nil {
			appLogger.Error("PostgreSQL health check failed", pkglogger.Error(err))
		} else {
			appLogger.Info("PostgreSQL health check passed")
		}
		
		// Initialize PostgreSQL repositories
		repos := postgresRepo.NewRepositoryContainer(pgClient, appLogger)

		// Initialize incident service with PostgreSQL
		incidentService = service.NewPostgreSQLIncidentService(
			appLogger,
			repos.IncidentRepo,
			repos.IncidentEventRepo,
		)
		appLogger.Info("Using PostgreSQL incident service")
	}

	// Initialize HTTP handler
	httpHandler := handler.NewHTTPHandler(appLogger, incidentService)

	// Start HTTP server for metrics and health
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: setupHTTPHandler(metricsHandler, healthChecker, appLogger, httpHandler),
	}

	// Start server
	go func() {
		appLogger.Info(fmt.Sprintf("Starting HTTP server on port %d", cfg.Server.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("HTTP server failed", pkglogger.Error(err))
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
		appLogger.Error("Server shutdown failed", pkglogger.Error(err))
	}

	appLogger.Info("Server stopped")
}

func setupHTTPHandler(metricsHandler http.Handler, healthChecker health.HealthChecker, appLogger pkglogger.Logger, httpHandler *handler.HTTPHandler) http.Handler {
	mux := http.NewServeMux()
	
	// Metrics endpoint
	mux.Handle("/metrics", metricsHandler)
	
	// Health endpoints using pkg/health
	mux.HandleFunc("/health", health.Handler(healthChecker))
	mux.HandleFunc("/ready", health.ReadyHandler(healthChecker))
	mux.HandleFunc("/live", health.LiveHandler())

	// Register incident manager routes
	httpHandler.RegisterRoutes(mux)
	
	return mux
}
