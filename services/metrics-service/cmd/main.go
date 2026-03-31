package main

import (
	"context"
	"encoding/json"
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
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/metrics-service/internal/collector"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfigWithAutoPath("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := pkglogger.NewLogger(cfg.Environment, cfg.Logger.Level, "metrics-service", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	appLogger.Info("Starting Metrics Service...")

	// Initialize metrics
	appMetrics := metrics.NewMetrics("metrics-service")
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
		appLogger.Error("Failed to connect to PostgreSQL", pkglogger.Error(err))
	} else {
		defer db.Close()
	}

	// Initialize metrics collector
	metricsCollector := collector.NewMetricsCollector(appLogger)

	// Start collecting metrics in background
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := metricsCollector.CollectAllMetrics(context.Background()); err != nil {
					appLogger.Error("Failed to collect metrics", pkglogger.Error(err))
				}
			}
		}
	}()

	// Start HTTP server for metrics and health
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: setupHTTPHandler(metricsHandler, healthChecker, appLogger, metricsCollector),
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

func setupHTTPHandler(metricsHandler http.Handler, healthChecker health.HealthChecker, appLogger pkglogger.Logger, metricsCollector *collector.MetricsCollector) http.Handler {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle("/metrics", metricsHandler)

	// Health endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"metrics-service"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"metrics-service"}`))
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"live","service":"metrics-service"}`))
	})

	// Metrics API endpoints
	mux.HandleFunc("/api/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get metrics from collector
		allMetrics := metricsCollector.GetAllMetrics()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"metrics": allMetrics,
			"total":   len(allMetrics),
			"message": "Metrics retrieved successfully",
		})
	})

	mux.HandleFunc("/api/v1/metrics/collect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Force collection of metrics
		if err := metricsCollector.CollectAllMetrics(context.Background()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
				"message": "Failed to collect metrics",
			})
			return
		}

		allMetrics := metricsCollector.GetAllMetrics()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":       true,
			"metrics_count": len(allMetrics),
			"collected_at":  time.Now().Format(time.RFC3339),
			"metrics":       allMetrics,
			"message":       "Metrics collected successfully",
		})
	})

	mux.HandleFunc("/api/v1/metrics/export", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"export_url": "/metrics",
			"message":    "Metrics export ready",
		})
	})

	mux.HandleFunc("/api/v1/metrics/query", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse query parameters
		serviceName := r.URL.Query().Get("service_name")
		metricType := r.URL.Query().Get("metric_type")

		allMetrics := metricsCollector.GetAllMetrics()
		var filteredMetrics []interface{}

		// Filter metrics based on query parameters
		for _, metric := range allMetrics {
			metricMap, ok := metric.(map[string]interface{})
			if !ok {
				continue
			}

			if serviceName != "" && metricMap["service_name"] != serviceName {
				continue
			}
			if metricType != "" && metricMap["type"] != metricType {
				continue
			}
			filteredMetrics = append(filteredMetrics, metric)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"query_result": filteredMetrics,
			"total":        len(filteredMetrics),
			"filters": map[string]string{
				"service_name": serviceName,
				"metric_type":  metricType,
			},
			"message": "Query executed successfully",
		})
	})

	return mux
}
