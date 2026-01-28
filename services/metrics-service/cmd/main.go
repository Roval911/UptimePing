package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/health"
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	"UptimePingPlatform/services/metrics-service/internal/collector"
	grpcHandler "UptimePingPlatform/services/metrics-service/internal/handler/grpc"
	httpHandler "UptimePingPlatform/services/metrics-service/internal/handler/http"

	metricsv1 "UptimePingPlatform/gen/proto/api/metrics/v1"
)

func main() {
	// Загружаем конфигурацию - единая схема для всех сервисов
	cfg, err := config.LoadConfig("")
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

	// Создаем Prometheus метрики для самого сервиса с конфигурацией
	var prometheusMetrics *metrics.Metrics
	if cfg.Metrics.Enabled {
		prometheusMetrics = metrics.NewMetricsFromConfig("metrics-service", &cfg.Metrics)
	} else {
		prometheusMetrics = metrics.NewMetrics("metrics-service")
	}
	logger.Info("Prometheus metrics initialized")

	// Создаем коллектор метрик
	metricsCollector := collector.NewMetricsCollector(logger)
	logger.Info("Metrics collector initialized")

	// Добавляем сервисы из конфигурации или переменных окружения
	if err := loadServicesFromConfig(metricsCollector, cfg, logger); err != nil {
		logger.Error("Failed to load services from config", pkglogger.Error(err))
	}

	// Создаем gRPC сервер
	grpcPort := cfg.GRPC.Port
	if grpcPort == 0 {
		grpcPort = 50053 // По умолчанию для Metrics Service
	}

	grpcAddr := fmt.Sprintf(":%d", grpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Error("Failed to listen on gRPC port", 
			pkglogger.Int("port", grpcPort), 
			pkglogger.Error(err))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	metricsHandler := grpcHandler.NewMetricsHandler(metricsCollector, logger)
	metricsv1.RegisterMetricsServiceServer(grpcServer, metricsHandler)

	// Включаем reflection для разработки
	reflection.Register(grpcServer)

	logger.Info("gRPC server configured",
		pkglogger.String("address", grpcAddr),
		pkglogger.Int("port", grpcPort))

	// Запускаем gRPC сервер в горутине
	go func() {
		logger.Info("Starting gRPC server", pkglogger.String("address", grpcAddr))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server failed", pkglogger.Error(err))
		}
	}()

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
	metricsPath := prometheusMetrics.GetMetricsPath(&cfg.Metrics)
	mux.Handle("/service-metrics", prometheusMetrics.GetHandler())
	
	// Добавляем основные метрики если путь отличается от /metrics
	if metricsPath != "/metrics" {
		mux.Handle(metricsPath, prometheusMetrics.GetHandler())
	}

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

	// Останавливаем gRPC сервер
	logger.Info("Stopping gRPC server")
	grpcServer.GracefulStop()

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
func loadServicesFromConfig(collector *collector.MetricsCollector, cfg *config.Config, logger pkglogger.Logger) error {
	// Загружаем сервисы из конфигурации
	services := []struct {
		name    string
		config  config.ServiceConfig
	}{
		{"auth-service", cfg.Services.AuthService},
		{"core-service", cfg.Services.CoreService},
		{"scheduler-service", cfg.Services.SchedulerService},
		{"api-gateway", cfg.Services.APIGateway},
	}

	for _, service := range services {
		// Добавляем сервис только если он включен в конфигурации
		if service.config.Enabled && service.config.Address != "" {
			if err := collector.AddService(service.name, service.config.Address); err != nil {
				return fmt.Errorf("failed to add service %s: %w", service.name, err)
			}
			
			logger.Info("Service added from config",
				pkglogger.String("service", service.name),
				pkglogger.String("address", service.config.Address))
		} else {
			logger.Info("Service disabled in config",
				pkglogger.String("service", service.name),
				pkglogger.Bool("enabled", service.config.Enabled))
		}
	}

	return nil
}
