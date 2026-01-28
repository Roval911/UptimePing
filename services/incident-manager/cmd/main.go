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
	httpHandler "UptimePingPlatform/services/incident-manager/internal/handler"
	incidentProducer "UptimePingPlatform/services/incident-manager/internal/producer/rabbitmq"
	"UptimePingPlatform/services/incident-manager/internal/service"

	incidentv1 "UptimePingPlatform/gen/proto/api/incident/v1"
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
	grpcPort := cfg.GRPC.Port
	if grpcPort == 0 {
		grpcPort = 50056 // По умолчанию для Incident Manager
	}

	grpcAddr := fmt.Sprintf(":%d", grpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		appLogger.Error("Failed to listen on gRPC port", 
			logger.Int("port", grpcPort), 
			logger.Error(err))
		os.Exit(1)
	}

	appLogger.Info("Starting gRPC server", logger.String("addr", grpcAddr))

	// Создаем gRPC сервер
	grpcServer := grpc.NewServer()

	// Регистрация сервисов
	incidentv1.RegisterIncidentServiceServer(grpcServer, incidentHandler)

	// Включаем reflection для разработки
	reflection.Register(grpcServer)

	// Запускаем gRPC сервер в горутине
	go func() {
		appLogger.Info("Starting gRPC server", logger.String("address", grpcAddr))
		if err := grpcServer.Serve(lis); err != nil {
			appLogger.Error("gRPC server failed", logger.Error(err))
		}
	}()

	// Создаем метрики с конфигурацией
	var metricsInstance *metrics.Metrics
	if cfg.Metrics.Enabled {
		metricsInstance = metrics.NewMetricsFromConfig("incident-manager", &cfg.Metrics)
	} else {
		metricsInstance = metrics.NewMetrics("incident-manager")
	}
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
	
	metricsPath := metricsInstance.GetMetricsPath(&cfg.Metrics)
	mux.Handle(metricsPath, metricsInstance.GetHandler())

	// Создаем и регистрируем HTTP обработчики для Incident API
	incidentHTTPHandler := httpHandler.NewHTTPHandler(appLogger, incidentService)
	incidentHTTPHandler.RegisterRoutes(mux)

	httpServer.Handler = metricsInstance.Middleware(mux)

	appLogger.Info("Incident Manager started successfully",
		logger.String("grpc_address", grpcAddr),
		logger.String("http_address", httpServer.Addr),
		logger.String("metrics", "http://"+httpServer.Addr+"/metrics"),
		logger.String("health", "http://"+httpServer.Addr+"/health"))
	appLogger.Info("Waiting for signals...")

	// Ожидание сигнала для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	appLogger.Info("Shutdown signal received")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defer shutdownCtx.Done()

	// Останавливаем gRPC сервер
	appLogger.Info("Stopping gRPC server")
	grpcServer.GracefulStop()

	// Останавливаем HTTP сервер
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("HTTP server shutdown failed", logger.Error(err))
		httpServer.Close()
	}

	appLogger.Info("Incident Manager stopped successfully")
}
