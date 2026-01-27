package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/logger"
	pkg_rabbitmq "UptimePingPlatform/pkg/rabbitmq"

	grpcHandler "UptimePingPlatform/services/incident-manager/internal/handler/grpc"
	"UptimePingPlatform/services/incident-manager/internal/service"
	incidentProducer "UptimePingPlatform/services/incident-manager/internal/producer/rabbitmq"

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
			log.Printf("Error syncing logger: %v", err)
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

	// Ожидание сигнала
	select {
	case sig := <-sigChan:
		appLogger.Info("Received shutdown signal", logger.String("signal", sig.String()))
	case <-ctx.Done():
		appLogger.Error("Context cancelled, shutting down")
		os.Exit(1)
	}

	// Graceful shutdown
	appLogger.Info("Shutting down gRPC server...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		appLogger.Info("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		appLogger.Warn("Shutdown timeout, forcing server stop")
		grpcServer.Stop()
	}

	appLogger.Info("Incident Manager service stopped")
}
