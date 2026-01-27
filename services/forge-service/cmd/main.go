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
	"UptimePingPlatform/services/forge-service/internal/domain"
	"UptimePingPlatform/services/forge-service/internal/handler"
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

	// Конвертируем ServiceInfo в domain.Service
	domainServices := make([]domain.Service, 0, len(services))
	for _, svc := range services {
		methods := make([]domain.Method, 0, len(svc.Methods))
		for _, method := range svc.Methods {
			methods = append(methods, domain.Method{
				Name:    method.Name,
				Timeout: "30s", // Default timeout
				Enabled: true,
			})
		}

		domainServices = append(domainServices, domain.Service{
			Name:    svc.Name,
			Package: svc.Package,
			Host:    "localhost", // Default host
			Port:    50051,       // Default port
			Methods: methods,
		})
	}

	for _, svc := range domainServices {
		logger.Info("Service parsed",
			pkglogger.String("name", svc.Name),
			pkglogger.String("package", svc.Package),
			pkglogger.Int("methods", len(svc.Methods)))
	}

	// Создаем генератор кода
	outputDir := cfg.Forge.OutputDir
	if outputDir == "" {
		outputDir = "generated"
	}

	codeGenerator := service.NewCodeGenerator(logger, outputDir)

	// Генерируем YAML конфигурацию для UptimePing Core
	configPath := fmt.Sprintf("%s/uptime_config.yaml", outputDir)
	if err := codeGenerator.GenerateConfig(domainServices, configPath); err != nil {
		logger.Error("Failed to generate config", pkglogger.Error(err))
	} else {
		logger.Info("YAML config generated", pkglogger.String("path", configPath))
	}

	// Генерируем Go код для gRPC checker'ов
	checkersPath := fmt.Sprintf("%s/checkers", outputDir)
	if err := codeGenerator.GenerateGRPCCheckers(domainServices, checkersPath); err != nil {
		logger.Error("Failed to generate gRPC checkers", pkglogger.Error(err))
	} else {
		logger.Info("gRPC checkers generated", pkglogger.String("path", checkersPath))
	}

	// Создаем интерактивную конфигурацию по умолчанию
	interactiveConfig := domain.NewDefaultInteractiveConfig()
	interactiveConfigPath := fmt.Sprintf("%s/interactive", outputDir)
	if err := codeGenerator.GenerateInteractiveConfig(interactiveConfig); err != nil {
		logger.Error("Failed to generate interactive config", pkglogger.Error(err))
	} else {
		logger.Info("Interactive config generated", pkglogger.String("path", interactiveConfigPath))
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

	// Создаем HTTP обработчики
	httpHandler := handler.NewHTTPHandler(logger, codeGenerator, parser)

	// Создаем mux для регистрации маршрутов
	mux := http.NewServeMux()

	// Регистрируем обработчики
	mux.HandleFunc("/health", health.Handler(healthChecker))
	mux.HandleFunc("/ready", health.ReadyHandler(healthChecker))
	mux.HandleFunc("/live", health.LiveHandler())
	mux.Handle("/metrics", metricsInstance.GetHandler())

	// Статические файлы для веб-интерфейса
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "web/index.html")
			return
		}
		http.NotFound(w, r)
	})

	// Регистрируем API маршруты
	httpHandler.RegisterRoutes(mux)

	// Применяем middleware
	handlerWithMetrics := metricsInstance.Middleware(mux)
	handlerWithLogging := httpHandler.LoggingMiddleware(handlerWithMetrics)
	handlerWithCORS := httpHandler.CORSMiddleware(handlerWithLogging)

	server.Handler = handlerWithCORS

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
