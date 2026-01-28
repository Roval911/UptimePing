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
	"gopkg.in/yaml.v2"

	notificationConfig "UptimePingPlatform/services/notification-service/config"
	grpcHandler "UptimePingPlatform/services/notification-service/internal/handler/grpc"
	"UptimePingPlatform/services/notification-service/internal/consumer/rabbitmq"
	"UptimePingPlatform/services/notification-service/internal/filter"
	"UptimePingPlatform/services/notification-service/internal/grouper"
	"UptimePingPlatform/services/notification-service/internal/processor"
	"UptimePingPlatform/services/notification-service/internal/provider"
	"UptimePingPlatform/services/notification-service/internal/service"
	"UptimePingPlatform/services/notification-service/internal/template"

	notificationv1 "UptimePingPlatform/gen/proto/api/notification/v1"
)

func main() {
	// Загрузка конфигурации - единая схема для всех сервисов
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Инициализация логгера
	appLogger, err := logger.NewLogger(
		cfg.Environment,
		cfg.Logger.Level,
		"notification-service",
		false, // Loki disabled for now
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}

	// Загрузка конфигурации провайдеров из YAML файла
	providersConfig := notificationConfig.DefaultProvidersConfig()

	// Загрузка из файла config.yaml с подстановкой переменных окружения
	if data, err := os.ReadFile("config/config.yaml"); err == nil {
		// Простая подстановка переменных окружения вида ${VAR:default}
		configContent := string(data)
		configContent = os.ExpandEnv(configContent)

		if err := yaml.Unmarshal([]byte(configContent), &providersConfig); err != nil {
			appLogger.Warn("Failed to parse providers config file", logger.Error(err))
		} else {
			appLogger.Info("Loaded providers config from config/config.yaml")
		}
	} else {
		appLogger.Warn("No config file found, using defaults")
	}

	appLogger.Info("Starting Notification Service")

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

	// Инициализация компонентов
	eventFilter := filter.NewEventFilter(filter.DefaultFilterConfig(), appLogger)
	notificationGrouper := grouper.NewNotificationGrouper(grouper.DefaultGrouperConfig(), providersConfig, appLogger)

	// Создание менеджера провайдеров уведомлений
	providerManager := provider.NewProviderManager(provider.ProviderConfig{
		Telegram: providersConfig.Telegram,
		Slack:    providersConfig.Slack,
		Email:    providersConfig.Email,
		Retry:    providersConfig.Retry,
	}, appLogger)

	// Создание менеджера шаблонов
	templateManager := template.NewDefaultTemplateManager(appLogger)

	// Создание процессора с провайдерами
	notificationProcessor := processor.NewNotificationProcessor(
		processor.DefaultProcessorConfig(),
		appLogger,
		providerManager,
		templateManager,
	)

	// Создание consumer
	notificationConsumer := rabbitmq.NewNotificationConsumer(
		rabbitmqConn,
		eventFilter,
		notificationGrouper,
		notificationProcessor,
		appLogger,
	)

	// Создаем метрики с конфигурацией
	var metricsInstance *metrics.Metrics
	if cfg.Metrics.Enabled {
		metricsInstance = metrics.NewMetricsFromConfig("notification-service", &cfg.Metrics)
	} else {
		metricsInstance = metrics.NewMetrics("notification-service")
	}
	appLogger.Info("Metrics initialized")

	// Создаем health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

	// Создаем Notification Service
	notificationService := service.NewNotificationService(appLogger)
	appLogger.Info("Notification service initialized")

	// Создаем gRPC сервер
	grpcPort := cfg.GRPC.Port
	if grpcPort == 0 {
		grpcPort = 50055 // По умолчанию для Notification Service
	}

	grpcAddr := fmt.Sprintf(":%d", grpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		appLogger.Error("Failed to listen on gRPC port", 
			logger.Int("port", grpcPort), 
			logger.Error(err))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	notificationHandler := grpcHandler.NewNotificationHandler(notificationService, appLogger)
	notificationv1.RegisterNotificationServiceServer(grpcServer, notificationHandler)

	// Включаем reflection для разработки
	reflection.Register(grpcServer)

	appLogger.Info("gRPC server configured",
		logger.String("address", grpcAddr),
		logger.Int("port", grpcPort))

	// Запускаем gRPC сервер в горутине
	go func() {
		appLogger.Info("Starting gRPC server", logger.String("address", grpcAddr))
		if err := grpcServer.Serve(lis); err != nil {
			appLogger.Error("gRPC server failed", logger.Error(err))
		}
	}()

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

	httpServer.Handler = metricsInstance.Middleware(mux)

	// Запуск consumer в горутине
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		appLogger.Info("Starting notification consumer")
		if err := notificationConsumer.Start(ctx); err != nil {
			appLogger.Error("Notification consumer failed", logger.Error(err))
			os.Exit(1)
		}
	}()

	// Запуск HTTP сервера для метрик в горутине
	go func() {
		appLogger.Info("Starting HTTP server for metrics",
			logger.String("address", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("HTTP server failed", logger.Error(err))
		}
	}()

	appLogger.Info("Notification Service started successfully",
		logger.String("http_address", httpServer.Addr),
		logger.String("metrics", "http://"+httpServer.Addr+"/metrics"),
		logger.String("health", "http://"+httpServer.Addr+"/health"),
		logger.String("grpc_address", grpcAddr))
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

	appLogger.Info("Stopping notification consumer...")
	if err := notificationConsumer.Stop(); err != nil {
		appLogger.Error("Failed to stop notification consumer", logger.Error(err))
	}

	// Останавливаем HTTP сервер
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("HTTP server shutdown failed", logger.Error(err))
		httpServer.Close()
	}

	appLogger.Info("Notification Service stopped gracefully")
}
