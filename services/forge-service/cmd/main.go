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
	"UptimePingPlatform/services/forge-service/internal/service"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Создаем логгер
	logger, err := pkglogger.NewLogger(cfg.Environment, cfg.Logger.Level, "forge-service", false)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	logger.Info("Starting Forge Service")

	// Создаем health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

	// Создаем метрики (если включены)
	var metricsInstance *metrics.Metrics
	if cfg.Metrics.Enabled {
		metricsInstance = metrics.NewMetrics("forge-service")
		logger.Info("Metrics enabled", pkglogger.String("port", fmt.Sprintf("%d", cfg.Metrics.Port)))
	}

	// Создаем парсер proto файлов
	protoDir := cfg.Forge.ProtoDir
	if len(os.Args) > 1 {
		protoDir = os.Args[1]
	}

	parser := service.NewProtoParser(protoDir)

	// Загружаем и валидируем proto файлы
	logger.Info("Loading proto files", pkglogger.String("directory", protoDir))
	if err := parser.LoadAndValidateProtoFiles(); err != nil {
		log.Fatalf("Failed to load proto files: %v", err)
	}

	// Валидируем извлеченные данные
	if err := parser.Validate(); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	// Выводим сводную информацию
	parser.PrintSummary()

	// Анализ сервисов
	services := parser.GetServices()
	logger.Info("Found services", pkglogger.Int("count", len(services)))
	for _, svc := range services {
		logger.Info("Service parsed",
			pkglogger.String("name", svc.Name),
			pkglogger.String("package", svc.Package),
			pkglogger.Int("methods", len(svc.Methods)))
	}

	// Анализ сообщений
	messages := parser.GetMessages()
	logger.Info("Found messages", pkglogger.Int("count", len(messages)))

	// Анализ enums
	enums := parser.GetEnums()
	logger.Info("Found enums", pkglogger.Int("count", len(enums)))

	// Запускаем HTTP сервер для health checks и метрик
	server := &http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
	}

	logger.Info("Server configuration",
		pkglogger.String("host", cfg.Server.Host),
		pkglogger.Int("port", cfg.Server.Port))

	// Регистрируем обработчики
	http.HandleFunc("/health", health.Handler(healthChecker))
	http.HandleFunc("/ready", health.ReadyHandler(healthChecker))
	http.HandleFunc("/live", health.LiveHandler())

	if metricsInstance != nil {
		http.Handle("/metrics", metricsInstance.GetHandler())
	}

	logger.Info("HTTP handlers registered")

	// Запускаем сервер в горутине
	go func() {
		logger.Info("Starting HTTP server",
			pkglogger.String("address", server.Addr),
			pkglogger.Bool("health_enabled", cfg.Health.Enabled),
			pkglogger.Bool("metrics_enabled", cfg.Metrics.Enabled))

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	logger.Info("HTTP server started, waiting for shutdown signal")

	// Ожидаем сигналы для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	logger.Info("Waiting for shutdown signal...")
	<-quit

	logger.Info("Shutdown signal received")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", pkglogger.Error(err))
	}

	logger.Info("Forge Service stopped")
}
