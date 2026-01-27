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
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	"UptimePingPlatform/services/metrics-service/internal/collector"
	httpHandler "UptimePingPlatform/services/metrics-service/internal/handler/http"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Создаем логгер
	logger, err := pkglogger.NewLogger(cfg.Environment, cfg.Logger.Level, "metrics-service", false)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Printf("Error syncing logger: %v", err)
		}
	}()

	logger.Info("Starting Metrics Service")

	// Создаем health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

	// Создаем Prometheus метрики для самого сервиса
	prometheusMetrics := metrics.NewMetrics("metrics-service")
	logger.Info("Prometheus metrics initialized")

	// Создаем коллектор метрик
	metricsCollector := collector.NewMetricsCollector(logger)
	logger.Info("Metrics collector initialized")

	// Добавляем сервисы из конфигурации или переменных окружения
	if err := loadServicesFromConfig(metricsCollector, cfg); err != nil {
		logger.Error("Failed to load services from config", pkglogger.Error(err))
	}

	// Создаем HTTP обработчики
	httpH := httpHandler.NewHTTPHandler(logger, metricsCollector)

	// Создаем mux для регистрации маршрутов
	mux := http.NewServeMux()

	// Регистрируем обработчики
	httpH.RegisterRoutes(mux)

	// Добавляем health check эндпоинты из pkg/health (только если не зарегистрированы)
	mux.HandleFunc("/health/pkg", health.Handler(healthChecker))
	mux.HandleFunc("/ready/pkg", health.ReadyHandler(healthChecker))
	mux.HandleFunc("/live/pkg", health.LiveHandler())

	// Добавляем metrics эндпоинт из pkg/metrics
	mux.Handle("/service-metrics", prometheusMetrics.GetHandler())

	// Применяем middleware
	handlerWithMetrics := prometheusMetrics.Middleware(mux)
	handlerWithLogging := httpH.LoggingMiddleware(handlerWithMetrics)
	handlerWithCORS := httpH.CORSMiddleware(handlerWithLogging)

	// Запускаем HTTP сервер
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      handlerWithCORS,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("Starting HTTP server",
		pkglogger.String("address", server.Addr),
		pkglogger.String("host", cfg.Server.Host),
		pkglogger.Int("port", cfg.Server.Port))

	// Запускаем сервер в горутине
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", pkglogger.Error(err))
		}
	}()

	logger.Info("Metrics Service started successfully",
		pkglogger.String("address", server.Addr),
		pkglogger.String("health", "http://"+server.Addr+"/health"),
		pkglogger.String("health_pkg", "http://"+server.Addr+"/health/pkg"),
		pkglogger.String("metrics", "http://"+server.Addr+"/metrics"),
		pkglogger.String("service_metrics", "http://"+server.Addr+"/service-metrics"))

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Metrics Service...")

	// Останавливаем HTTP сервер с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", pkglogger.Error(err))
	}

	// Закрываем коллектор метрик
	if err := metricsCollector.Shutdown(); err != nil {
		logger.Error("Failed to shutdown metrics collector", pkglogger.Error(err))
	}

	logger.Info("Metrics Service stopped successfully")
}

// loadServicesFromConfig загружает сервисы из конфигурации
func loadServicesFromConfig(collector *collector.MetricsCollector, cfg *config.Config) error {
	//todo Здесь можно добавить логику загрузки сервисов из конфигурации
	// Например, из конфигурационного файла или переменных окружения

	// Пример: добавляем сервисы из переменных окружения
	services := []struct {
		Name    string
		Address string
		EnvVar  string
	}{
		{"auth-service", "localhost:50051", "AUTH_SERVICE_ADDR"},
		{"core-service", "localhost:50052", "CORE_SERVICE_ADDR"},
		{"scheduler-service", "localhost:50053", "SCHEDULER_SERVICE_ADDR"},
		{"api-gateway", "localhost:8080", "API_GATEWAY_ADDR"},
	}

	for _, service := range services {
		address := os.Getenv(service.EnvVar)
		if address == "" {
			address = service.Address
		}

		if address != "" {
			if err := collector.AddService(service.Name, address); err != nil {
				return fmt.Errorf("failed to add service %s: %w", service.Name, err)
			}
		}
	}

	return nil
}
