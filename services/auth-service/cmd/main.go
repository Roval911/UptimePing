package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/services/auth-service/internal/service"
	"UptimePingPlatform/services/auth-service/internal/repository/postgres"
	"UptimePingPlatform/services/auth-service/internal/repository/redis"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/pkg/password"
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
		if syncErr := appLogger.(*logger.LoggerImpl).zapLogger.Sync(); syncErr != nil {
			log.Printf("Error syncing logger: %v", syncErr)
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
		appLogger.Error("Failed to connect to database", logger.Error(err))
		os.Exit(1)
	}
	defer postgresDB.Close()

	// Инициализация Redis
	// В реальной реализации здесь будет инициализация Redis клиента
	redisClient := redis.NewClient("localhost:6379")
	defer redisClient.Close()

	// Инициализация зависимостей
	passwordHasher := password.NewBcryptHasher()
	jwtManager := jwt.NewJWTManager("secret-key", 24*time.Hour, 7*24*time.Hour)

	// Создание репозиториев
	userRepository := postgres.NewUserRepository(postgresDB.Pool)
	tenantRepository := postgres.NewTenantRepository(postgresDB.Pool)
	apiKeyRepository := postgres.NewAPIKeyRepository(postgresDB.Pool)
	sessionRepository := redis.NewSessionRepository(redisClient)

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
	handler := setupHandler(authService, appLogger)
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: handler,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		appLogger.Info("Starting auth service server", logger.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("Server failed", logger.Error(err))
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
		appLogger.Error("Server shutdown failed", logger.Error(err))
	}

	appLogger.Info("Server stopped")
}

func setupHandler(authService service.AuthService, logger logger.Logger) http.Handler {
	// В реальной реализации здесь будет настройка HTTP маршрутов
	// и middleware
	mux := http.NewServeMux()
	
	// Пример простого health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Проверка базы данных
		if err := postgresDB.HealthCheck(r.Context()); err != nil {
			logger.Error("Database health check failed", logger.Error(err))
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return errors.Middleware(mux)
}