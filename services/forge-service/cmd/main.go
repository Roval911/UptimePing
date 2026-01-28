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
	"UptimePingPlatform/services/forge-service/internal/validation"
	"UptimePingPlatform/services/forge-service/internal/domain"
	grpcHandler "UptimePingPlatform/services/forge-service/internal/handler/grpc"
	"UptimePingPlatform/services/forge-service/internal/handler"
	"UptimePingPlatform/services/forge-service/internal/service"

	forgev1 "UptimePingPlatform/gen/proto/api/forge/v1"
)

func main() {
	// Загружаем конфигурацию - единая схема для всех сервисов
	cfg, err := config.LoadConfig("")
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

	// Создаем метрики (если включены) с конфигурацией
	var metricsInstance *metrics.Metrics
	if cfg.Metrics.Enabled {
		metricsInstance = metrics.NewMetricsFromConfig("forge-service", &cfg.Metrics)
		logger.Info("Metrics enabled", pkglogger.String("port", fmt.Sprintf("%d", cfg.Metrics.Port)))
	} else {
		metricsInstance = metrics.NewMetrics("forge-service")
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

	// Создаем сервис Forge
	validator := validation.NewForgeValidator()
	forgeService := service.NewForgeService(logger, parser, codeGenerator, validator)

	// Создаем HTTP обработчик с gRPC клиентом для Auth Service
	apiGatewayAddress := os.Getenv("API_GATEWAY_ADDRESS")
	if apiGatewayAddress == "" {
		apiGatewayAddress = "localhost:50051" // По умолчанию для API Gateway
	}
	
	httpHandler := handler.NewHTTPHandler(logger, codeGenerator, parser, forgeService, apiGatewayAddress)
	if httpHandler == nil {
		log.Fatalf("Failed to create HTTP handler")
	}

	// Создаем HTTP сервер
	httpPortStr := os.Getenv("FORGE_HTTP_PORT")
	if httpPortStr == "" {
		httpPortStr = "8080" // По умолчанию для Forge Service
	}
	
	httpPort := 8080
	if _, err := fmt.Sscanf(httpPortStr, "%d", &httpPort); err != nil {
		httpPort = 8080
	}

	httpAddr := fmt.Sprintf(":%d", httpPort)
	mux := http.NewServeMux()
	
	// Регистрируем HTTP маршруты
	httpHandler.RegisterRoutes(mux)
	
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	logger.Info("HTTP server configured",
		pkglogger.String("address", httpAddr),
		pkglogger.Int("port", httpPort),
		pkglogger.String("api_gateway", apiGatewayAddress))

	// Запускаем HTTP сервер в горутине
	go func() {
		logger.Info("Starting HTTP server", pkglogger.String("address", httpAddr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", pkglogger.Error(err))
		}
	}()

	// Создаем gRPC сервер
	grpcPortStr := os.Getenv("FORGE_GRPC_PORT")
	if grpcPortStr == "" {
		grpcPortStr = "50054" // По умолчанию для Forge Service
	}
	
	grpcPort := 50054
	if _, err := fmt.Sscanf(grpcPortStr, "%d", &grpcPort); err != nil {
		grpcPort = 50054
	}

	grpcAddr := fmt.Sprintf(":%d", grpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port %d: %v", grpcPort, err)
	}

	grpcServer := grpc.NewServer()
	forgeHandler := grpcHandler.NewForgeHandler(forgeService, logger)
	forgev1.RegisterForgeServiceServer(grpcServer, forgeHandler)

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

	// Запускаем HTTP сервер для health checks и метрик
	server := &http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
	}

	logger.Info("Server configuration",
		pkglogger.String("host", cfg.Server.Host),
		pkglogger.Int("port", cfg.Server.Port))

	// Создаем mux для регистрации маршрутов
	mux = http.NewServeMux()

	// Регистрируем обработчики
	mux.HandleFunc("/health", health.Handler(healthChecker))
	mux.HandleFunc("/ready", health.ReadyHandler(healthChecker))
	mux.HandleFunc("/live", health.LiveHandler())
	
	metricsPath := metricsInstance.GetMetricsPath(&cfg.Metrics)
	mux.Handle(metricsPath, metricsInstance.GetHandler())

	// Применяем middleware
	handlerWithMetrics := metricsInstance.Middleware(mux)
	handlerWithLogging := httpHandler.LoggingMiddleware(handlerWithMetrics)
	handlerWithCORS := httpHandler.CORSMiddleware(handlerWithLogging)

	// Регистрируем API маршруты
	httpHandler.RegisterRoutes(mux)

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

	// Останавливаем gRPC сервер
	logger.Info("Stopping gRPC server")
	grpcServer.GracefulStop()

	// Останавливаем HTTP сервер
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", pkglogger.Error(err))
	}

	logger.Info("Forge Service stopped")
}
