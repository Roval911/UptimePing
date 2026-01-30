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

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	pkg_redis "UptimePingPlatform/pkg/redis"

	"UptimePingPlatform/services/auth-service/internal/grpc/handlers"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/pkg/password"
	"UptimePingPlatform/services/auth-service/internal/repository"
	"UptimePingPlatform/services/auth-service/internal/repository/postgres"
	redisRepo "UptimePingPlatform/services/auth-service/internal/repository/redis"
	"UptimePingPlatform/services/auth-service/internal/service"

	grpc_auth "UptimePingPlatform/proto/api/auth/v1"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfigWithAutoPath("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.NewLogger(cfg.Environment, cfg.Logger.Level, "auth-service", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	appLogger.Info("Starting Auth Service...")

	// Initialize database connection
	db, err := database.Connect(context.Background(), &database.Config{
		Host:        cfg.Database.Host,
		Port:        cfg.Database.Port,
		User:        cfg.Database.User,
		Password:    cfg.Database.Password,
		Database:    cfg.Database.Name,
		SSLMode:     "disable",
		MaxConns:    25,
		MinConns:    5,
		MaxConnLife: time.Hour,
		MaxConnIdle: 5 * time.Minute,
		HealthCheck: 5 * time.Minute,
	})
	if err != nil {
		appLogger.Error("Failed to connect to database", logger.Error(err))
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	// Initialize Redis client
	redisClient, err := pkg_redis.Connect(context.Background(), &pkg_redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		appLogger.Error("Failed to connect to Redis", logger.Error(err))
	} else {
		defer redisClient.Close()
	}

	// Initialize repositories
	userRepo := postgres.NewUserRepository(db.Pool)
	tenantRepo := postgres.NewTenantRepository(db.Pool)
	apiKeyRepo := postgres.NewAPIKeyRepository(db.Pool)

	// Initialize session repository
	var sessionRepo repository.SessionRepository
	if redisClient != nil && redisClient.Client != nil {
		sessionRepo = redisRepo.NewSessionRepository(redisClient.Client)
		appLogger.Info("Session repository initialized successfully")
	} else {
		appLogger.Warn("Redis not available, session repository disabled")
	}

	// Initialize JWT manager
	jwtManager := jwt.NewManager(
		"your-access-secret-key",
		"your-refresh-secret-key",
		24*time.Hour,   // access token TTL
		7*24*time.Hour, // refresh token TTL
	)

	// Initialize password hasher
	passwordHasher := password.NewBcryptHasher(bcrypt.DefaultCost)

	// Initialize auth service
	authService := service.NewAuthService(
		userRepo,
		tenantRepo,
		apiKeyRepo,
		sessionRepo,
		jwtManager,
		passwordHasher,
		appLogger,
	)

	// Initialize gRPC handler
	authHandler := handlers.NewAuthHandler(authService, jwtManager, appLogger)

	// Initialize metrics
	appMetrics := metrics.NewMetrics("auth-service")
	metricsHandler := appMetrics.GetHandler()

	// Initialize health checker
	healthChecker := health.NewSimpleHealthChecker("1.0.0")

	// Start gRPC server
	grpcPort := cfg.Server.Port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		appLogger.Error("Failed to listen", logger.Error(err))
		log.Fatalf("Failed to listen: %v", err)
	}

	appLogger.Info(fmt.Sprintf("Successfully listening on port %d", grpcPort))

	grpcServer := grpc.NewServer()
	grpc_auth.RegisterAuthServiceServer(grpcServer, authHandler)

	appLogger.Info("gRPC server created, starting to serve...")

	// Graceful shutdown setup
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		appLogger.Info(fmt.Sprintf("Starting gRPC server on port %d", grpcPort))
		if err := grpcServer.Serve(lis); err != nil {
			appLogger.Error("gRPC server failed", logger.Error(err))
		}
	}()

	// Start HTTP server for health checks
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port+1000), // Health check on port +1000
		Handler: setupHTTPHandler(metricsHandler, healthChecker, appLogger),
	}

	go func() {
		appLogger.Info(fmt.Sprintf("Starting HTTP server on port %d", cfg.Server.Port+1000))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("HTTP server failed", logger.Error(err))
		}
	}()

	// Wait for interrupt signal
	<-quit
	appLogger.Info("Shutting down server...")

	// Graceful shutdown gRPC server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grpcServer.GracefulStop()

	// Graceful shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		appLogger.Error("HTTP server forced to stop", logger.Error(err))
	}

	// Close database connection
	if db != nil {
		db.Close()
	}

	// Close Redis connection
	if redisClient != nil {
		redisClient.Close()
	}

	appLogger.Info("Server exited properly")
}

func setupHTTPHandler(metricsHandler http.Handler, healthChecker health.HealthChecker, appLogger logger.Logger) http.Handler {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", metricsHandler)

	// Health endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"auth-service"}`))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","service":"auth-service"}`))
	})

	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"live","service":"auth-service"}`))
	})

	// Auth endpoints
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Auth Service - Login endpoint","status":"ok"}`))
	})

	mux.HandleFunc("/api/v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Auth Service - Register endpoint","status":"ok"}`))
	})

	mux.HandleFunc("/api/v1/auth/validate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Auth Service - Validate endpoint","status":"ok"}`))
	})

	return mux
}
