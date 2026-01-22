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

	"golang.org/x/crypto/bcrypt"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/ratelimit"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/auth-service/internal/middleware"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/pkg/password"
	"UptimePingPlatform/services/auth-service/internal/repository/postgres"
	"UptimePingPlatform/services/auth-service/internal/repository/redis"
	"UptimePingPlatform/services/auth-service/internal/service"
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
		"auth-service",
		true,
	)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := appLogger.Sync(); err != nil {
			log.Printf("Error syncing logger: %v", err)
		}
	}()

	// Инициализация базы данных
	dbConfig := database.NewConfig()
	dbConfig.Host = cfg.Database.Host
	dbConfig.Port = cfg.Database.Port
	dbConfig.User = cfg.Database.User
	dbConfig.Password = cfg.Database.Password
	dbConfig.Database = cfg.Database.Name

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	postgresDB, err := database.Connect(ctx, dbConfig)
	if err != nil {
		appLogger.Error("Failed to connect to database", logger.String("error", err.Error()))
		os.Exit(1)
	}
	defer postgresDB.Close()

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

	// Инициализация зависимостей
	passwordHasher := password.NewBcryptHasher(bcrypt.DefaultCost)
	jwtManager := jwt.NewManager("secret-key", "refresh-secret-key", 24*time.Hour, 7*24*time.Hour)

	// Создание репозиториев
	userRepository := postgres.NewUserRepository(postgresDB.Pool)
	tenantRepository := postgres.NewTenantRepository(postgresDB.Pool)
	apiKeyRepository := postgres.NewAPIKeyRepository(postgresDB.Pool)
	sessionRepository := redis.NewSessionRepository(redisClient.Client)

	// Создание сервиса
	authService := service.NewAuthService(
		userRepository,
		tenantRepository,
		apiKeyRepository,
		sessionRepository,
		jwtManager,
		passwordHasher,
	)

	// Настройка HTTP сервера
	handler := setupHandler(authService, appLogger, rateLimiter, postgresDB, redisClient)
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: handler,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		appLogger.Info("Starting auth service server", logger.String("addr", server.Addr))
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
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Error("Server shutdown failed", logger.String("error", err.Error()))
	}

	appLogger.Info("Server stopped")
}

func setupHandler(authService service.AuthService, logger logger.Logger, rateLimiter ratelimit.RateLimiter, postgresDB *database.Postgres, redisClient *pkg_redis.Client) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return middleware.RateLimitMiddleware(rateLimiter, 100, time.Minute, false)(
		errors.Middleware(mux),
	)
}
