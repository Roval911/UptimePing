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
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	"UptimePingPlatform/pkg/ratelimit"
	pkg_redis "UptimePingPlatform/pkg/redis"
	httphandler "UptimePingPlatform/services/api-gateway/internal/handler/http"
	"UptimePingPlatform/services/api-gateway/internal/middleware"
)

// AuthServiceStub заглушка для AuthService
type AuthServiceStub struct{}

func (a *AuthServiceStub) Login(ctx context.Context, email, password string) (*httphandler.TokenPair, error) {
	return &httphandler.TokenPair{
		AccessToken:  "stub-access-token",
		RefreshToken: "stub-refresh-token",
	}, nil
}

func (a *AuthServiceStub) Register(ctx context.Context, email, password, tenantName string) (*httphandler.TokenPair, error) {
	return &httphandler.TokenPair{
		AccessToken:  "stub-access-token",
		RefreshToken: "stub-refresh-token",
	}, nil
}

func (a *AuthServiceStub) RefreshToken(ctx context.Context, refreshToken string) (*httphandler.TokenPair, error) {
	return &httphandler.TokenPair{
		AccessToken:  "new-stub-access-token",
		RefreshToken: "new-stub-refresh-token",
	}, nil
}

func (a *AuthServiceStub) Logout(ctx context.Context, userID, tokenID string) error {
	return nil
}

// HealthCheckerStub заглушка для HealthChecker
// Теперь реализует интерфейс health.HealthChecker из пакета health
type HealthCheckerStub struct{}

func (h *HealthCheckerStub) Check() *health.HealthStatus {
	return &health.HealthStatus{
		Status: "UP",
	}
}

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
		"api-gateway",
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

	// Инициализация Redis
	redisConfig := pkg_redis.NewConfig()
	redisConfig.Addr = "localhost:6379" // В реальном приложении брать из конфига

	redisCtx, redisCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer redisCancel()

	redisClient, err := pkg_redis.Connect(redisCtx, redisConfig)
	if err != nil {
		appLogger.Error("Failed to connect to redis", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer redisClient.Close()

	// Инициализация rate limiter
	rateLimiter := ratelimit.NewRedisRateLimiter(redisClient.Client)

	// Инициализация метрик
	metricCollector := metrics.NewMetrics("api_gateway")

	// Создание HealthChecker и HealthHandler
	healthChecker := &HealthCheckerStub{}
	healthHandler := httphandler.NewHealthHandler(healthChecker)

	// Создание AuthService
	authService := &AuthServiceStub{}

	// Настройка HTTP сервера
	baseHandler := httphandler.NewHandler(authService, healthHandler)

	// Обертываем хендлер в middleware
	var httpHandler http.Handler = baseHandler
	httpHandler = middleware.LoggingMiddleware(appLogger)(httpHandler)
	httpHandler = middleware.RecoveryMiddleware()(httpHandler)
	httpHandler = middleware.CORSMiddleware([]string{"*"})(httpHandler)
	httpHandler = middleware.RateLimitMiddleware(rateLimiter, 100, time.Minute)(httpHandler)
	// middleware.AuthMiddleware будет добавлено позже, когда будет реализован клиент auth-service

	// Добавляем эндпоинт для метрик
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metricCollector.GetHandler())
	metricsMux.Handle("/", httpHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: metricsMux,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		appLogger.Info("Starting API Gateway server", logger.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("Server failed", logger.String("error", err.Error()))
		}
	}()

	// Обработка сигналов для graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	appLogger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Error("Server shutdown failed", logger.String("error", err.Error()))
	}

	appLogger.Info("Server stopped")
}
