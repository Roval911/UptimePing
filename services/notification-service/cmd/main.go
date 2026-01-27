package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pkg_config "UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_rabbitmq "UptimePingPlatform/pkg/rabbitmq"
	"gopkg.in/yaml.v2"

	"UptimePingPlatform/services/notification-service/config"
	"UptimePingPlatform/services/notification-service/internal/consumer/rabbitmq"
	"UptimePingPlatform/services/notification-service/internal/filter"
	"UptimePingPlatform/services/notification-service/internal/grouper"
	"UptimePingPlatform/services/notification-service/internal/processor"
	"UptimePingPlatform/services/notification-service/internal/provider"
	"UptimePingPlatform/services/notification-service/internal/template"
)

func main() {
	// Загрузка конфигурации
	cfg, err := pkg_config.LoadConfig("")
	if err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}

	// Инициализация логгера
	appLogger, err := logger.NewLogger(
		cfg.Logger.Level,
		cfg.Logger.Format,
		"notification-service",
		false, // Loki disabled for now
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}

	// Загрузка конфигурации провайдеров из YAML файла
	providersConfig := config.DefaultProvidersConfig()

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
	notificationGrouper := grouper.NewNotificationGrouper(grouper.DefaultGrouperConfig(), cfg.Recipients, appLogger)

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

	// Создаем метрики
	metricsInstance := metrics.NewMetrics("notification-service")
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
		logger.String("health", "http://"+httpServer.Addr+"/health"))
	appLogger.Info("Waiting for signals...")

	// Ожидание сигнала для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	appLogger.Info("Shutdown signal received")

	// Graceful shutdown
	shutdownCtx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCtx.Done()

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
