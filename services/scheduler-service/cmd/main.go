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
)

func main() {
	// Инициализация конфигурации - единая схема для всех сервисов
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация логгера
	appLogger, err := logger.NewLogger(
		cfg.Environment,
		cfg.Logger.Level,
		"scheduler-service",
		false,
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := appLogger.Sync(); err != nil {
			appLogger.Error("Error syncing logger", logger.Error(err))
		}
	}()

	appLogger.Info("Starting scheduler service")

	// Показываем загруженную конфигурацию
	appLogger.Info("Configuration loaded successfully",
		logger.String("server", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
		logger.String("database", fmt.Sprintf("%s:%d/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)),
		logger.String("redis", cfg.Redis.Addr),
		logger.Int("scheduler_max_tasks", cfg.Scheduler.MaxConcurrentTasks),
		logger.String("scheduler_task_timeout", cfg.Scheduler.TaskTimeout.String()),
		logger.String("scheduler_cleanup_interval", cfg.Scheduler.CleanupInterval.String()),
		logger.String("scheduler_lock_timeout", cfg.Scheduler.LockTimeout.String()),
		logger.Int("rate_limit_requests_per_minute", cfg.RateLimiting.RequestsPerMinute),
		logger.Int("rate_limit_burst_size", cfg.RateLimiting.BurstSize),
	)

	// Инициализация метрик с конфигурацией
	var metricsInstance *metrics.Metrics
	if cfg.Metrics.Enabled {
		metricsInstance = metrics.NewMetricsFromConfig("scheduler-service", &cfg.Metrics)
	} else {
		metricsInstance = metrics.NewMetrics("scheduler-service")
	}

	// Инициализация health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

	// Запускаем HTTP сервер
	server := &http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
	}

	// Регистрируем обработчики
	http.HandleFunc("/health", health.Handler(healthChecker))
	http.HandleFunc("/ready", health.ReadyHandler(healthChecker))
	http.HandleFunc("/live", health.LiveHandler())
	
	metricsPath := metricsInstance.GetMetricsPath(&cfg.Metrics)
	http.Handle(metricsPath, metricsInstance.GetHandler())

	// API роуты для демонстрации
	http.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"running","service":"scheduler-service","version":"1.0.0"}`)
	})

	// Запускаем сервер в горутине
	go func() {
		appLogger.Info("Starting HTTP server",
			logger.String("address", server.Addr))

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("Failed to start server", logger.Error(err))
			os.Exit(1)
		}
	}()

	appLogger.Info("Scheduler service started successfully")

	// Ожидаем сигналы для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Error("Server forced to shutdown", logger.Error(err))
	}

	appLogger.Info("Scheduler service stopped")
}
