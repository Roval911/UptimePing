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
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_rabbitmq "UptimePingPlatform/pkg/rabbitmq"

	grpcHandler "UptimePingPlatform/services/incident-manager/internal/handler/grpc"
	incidentProducer "UptimePingPlatform/services/incident-manager/internal/producer/rabbitmq"
	"UptimePingPlatform/services/incident-manager/internal/service"

	pb "UptimePingPlatform/gen/go/proto/api/incident/v1"
)

func main() {
	// Инициализация конфигурации
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация логгера
	appLogger, err := logger.NewLogger(
		cfg.Environment,
		cfg.Logger.Level,
		"incident-manager",
		false, // временно отключил Loki
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := appLogger.Sync(); err != nil {
			appLogger.Error("Error syncing logger", logger.Error(err))
		}
	}()

	appLogger.Info("Starting Incident Manager service")

	// Инициализация RabbitMQ
	rabbitmqConfig := pkg_rabbitmq.NewConfig()
	rabbitmqConfig.URL = cfg.RabbitMQ.URL
	rabbitmqConfig.Exchange = cfg.RabbitMQ.Exchange
	rabbitmqConfig.RoutingKey = cfg.RabbitMQ.RoutingKey

	rabbitmqConn, err := pkg_rabbitmq.Connect(context.Background(), rabbitmqConfig)
	if err != nil {
		appLogger.Error("Failed to connect to RabbitMQ", logger.Error(err))
		os.Exit(1)
	}
	defer rabbitmqConn.Close()

	// Инициализация RabbitMQ producer для инцидентов
	incidentProducerConfig := incidentProducer.DefaultIncidentProducerConfig()
	incidentProducerConfig.URL = cfg.RabbitMQ.URL
	incidentProducerConfig.Exchange = cfg.RabbitMQ.Exchange

	incidentProducer, err := incidentProducer.NewIncidentProducer(rabbitmqConn, incidentProducerConfig, appLogger)
	if err != nil {
		appLogger.Error("Failed to create incident producer", logger.Error(err))
		os.Exit(1)
	}
	defer incidentProducer.Close()

	// Инициализация сервиса инцидентов
	incidentService := service.NewIncidentServiceWithProducer(nil, nil, appLogger, incidentProducer)

	// Инициализация gRPC handler
	incidentHandler := grpcHandler.NewIncidentHandler(incidentService, appLogger)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer()

	// Регистрация сервисов
	pb.RegisterIncidentServiceServer(grpcServer, incidentHandler)

	// Включаем reflection для разработки
	reflection.Register(grpcServer)

	// Запуск gRPC сервера
	listenAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		appLogger.Error("Failed to listen", logger.Error(err))
		os.Exit(1)
	}

	appLogger.Info("Starting gRPC server", logger.String("addr", listenAddr))

	// Создаем метрики
	metricsInstance := metrics.NewMetrics("incident-manager")
	appLogger.Info("Metrics initialized")

	// Создаем health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

	// Создаем HTTP сервер для health checks и метрик
	httpServer := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Server.Port+1000), // Используем порт+1000 для HTTP
	}

	// Регистрируем HTTP маршруты
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health.Handler(healthChecker))
	mux.HandleFunc("/ready", health.ReadyHandler(healthChecker))
	mux.HandleFunc("/live", health.LiveHandler())
	mux.Handle("/metrics", metricsInstance.GetHandler())

	httpServer.Handler = metricsInstance.Middleware(mux)

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Канал для сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запуск сервера в горутине
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			appLogger.Error("gRPC server failed", logger.Error(err))
			cancel()
		}
	}()

	// Запуск HTTP сервера для метрик в горутине
	go func() {
		appLogger.Info("Starting HTTP server for metrics",
			logger.String("address", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("HTTP server failed", logger.Error(err))
			cancel()
		}
	}()

	appLogger.Info("Incident Manager started successfully",
		logger.String("grpc_address", listenAddr),
		logger.String("http_address", httpServer.Addr),
		logger.String("metrics", "http://"+httpServer.Addr+"/metrics"),
		logger.String("health", "http://"+httpServer.Addr+"/health"))

	// Ожидание сигнала
	select {
	case sig := <-sigChan:
		appLogger.Info("Received shutdown signal", logger.String("signal", sig.String()))
	case <-ctx.Done():
		appLogger.Error("Context cancelled, shutting down")
		os.Exit(1)
	}

	// Graceful shutdown
	appLogger.Info("Shutting down servers...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	// Останавливаем HTTP сервер
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("HTTP server shutdown failed", logger.Error(err))
		httpServer.Close()
	}

	select {
	case <-done:
		appLogger.Info("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		appLogger.Warn("Shutdown timeout, forcing server stop")
		grpcServer.Stop()
	}

	appLogger.Info("Incident Manager service stopped")
}
