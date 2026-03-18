package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/database"
	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/pkg/validation"

	grpc_auth "UptimePingPlatform/proto/api/auth/v1"
	grpcHandlers "UptimePingPlatform/services/auth-service/internal/grpc/handlers"
	authJWT "UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/pkg/password"
	"UptimePingPlatform/services/auth-service/internal/repository/postgres"
	authRedis "UptimePingPlatform/services/auth-service/internal/repository/redis"
	"UptimePingPlatform/services/auth-service/internal/service"

	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.LoadConfigWithAutoPath("dev")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	appLogger, err := logger.NewLogger(cfg.Environment, cfg.Logger.Level, "auth-service", false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	ctx := context.Background()

	pgCfg := database.NewConfig()
	pgCfg.Host = cfg.Database.Host
	pgCfg.Port = cfg.Database.Port
	pgCfg.User = cfg.Database.User
	pgCfg.Password = cfg.Database.Password
	pgCfg.Database = cfg.Database.Name

	pg, err := database.Connect(ctx, pgCfg)
	if err != nil {
		appLogger.Error("Failed to connect to Postgres", logger.Error(err))
		log.Fatalf("Postgres connect failed: %v", err)
	}
	defer pg.Close()

	redisClient, err := pkg_redis.Connect(ctx, &pkg_redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})
	if err != nil {
		appLogger.Error("Failed to connect to Redis", logger.Error(err))
		log.Fatalf("Redis connect failed: %v", err)
	}
	defer redisClient.Close()

	if cfg.JWT.AccessSecret == "" || cfg.JWT.RefreshSecret == "" {
		log.Fatalf("JWT secrets are required (JWT_ACCESS_SECRET, JWT_REFRESH_SECRET)")
	}

	accessTTL, err := time.ParseDuration(cfg.JWT.AccessTokenDuration)
	if err != nil {
		log.Fatalf("Invalid JWT access token duration: %v", err)
	}
	refreshTTL, err := time.ParseDuration(cfg.JWT.RefreshTokenDuration)
	if err != nil {
		log.Fatalf("Invalid JWT refresh token duration: %v", err)
	}

	jwtManager := authJWT.NewManager(cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret, accessTTL, refreshTTL)
	passwordHasher := password.NewBcryptHasher(0)

	userRepo := postgres.NewUserRepository(pg.Pool)
	tenantRepo := postgres.NewTenantRepository(pg.Pool)
	apiKeyRepo := postgres.NewAPIKeyRepository(pg.Pool)
	sessionRepo := authRedis.NewSessionRepository(redisClient.Client)

	authService := service.NewAuthService(userRepo, tenantRepo, apiKeyRepo, sessionRepo, jwtManager, passwordHasher, *redisClient, appLogger)

	healthChecker := health.NewSimpleHealthChecker("1.0.0")
	validator := validation.NewValidator()

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      newHTTPHandler(authService, jwtManager, healthChecker, validator, appLogger),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		appLogger.Error("Failed to listen for gRPC", logger.Error(err))
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	grpcAuthHandler := grpcHandlers.NewAuthHandler(authService, jwtManager, appLogger)
	grpc_auth.RegisterAuthServiceServer(grpcServer, grpcAuthHandler)

	go func() {
		appLogger.Info(fmt.Sprintf("Starting Auth gRPC server on port %d", cfg.GRPC.Port))
		if err := grpcServer.Serve(grpcLis); err != nil {
			appLogger.Error("gRPC server failed", logger.Error(err))
		}
	}()

	go func() {
		appLogger.Info(fmt.Sprintf("Starting Auth HTTP server on port %d", cfg.Server.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("HTTP server failed", logger.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grpcServer.GracefulStop()
	_ = httpServer.Shutdown(shutdownCtx)
}

type httpAuthHandler struct {
	authService   service.AuthService
	jwtManager    authJWT.JWTManager
	healthChecker health.HealthChecker
	validator     *validation.Validator
	log           logger.Logger
}

func newHTTPHandler(
	authService service.AuthService,
	jwtManager authJWT.JWTManager,
	healthChecker health.HealthChecker,
	validator *validation.Validator,
	log logger.Logger,
) http.Handler {
	h := &httpAuthHandler{
		authService:   authService,
		jwtManager:    jwtManager,
		healthChecker: healthChecker,
		validator:     validator,
		log:           log,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/ready", h.handleReady)
	mux.HandleFunc("/live", h.handleLive)

	mux.HandleFunc("/api/v1/auth/register", h.handleRegister)
	mux.HandleFunc("/api/v1/auth/login", h.handleLogin)
	mux.HandleFunc("/api/v1/auth/refresh", h.handleRefresh)
	mux.HandleFunc("/api/v1/auth/logout", h.handleLogout)
	mux.HandleFunc("/api/v1/auth/validate", h.handleValidate)
	mux.HandleFunc("/api/v1/auth/api-keys", h.handleAPIKeys)
	mux.HandleFunc("/api/v1/auth/validate-api-key", h.handleValidateAPIKey)

	return mux
}

func (h *httpAuthHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy","service":"auth-service"}`))
}

func (h *httpAuthHandler) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready","service":"auth-service"}`))
}

func (h *httpAuthHandler) handleLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"live","service":"auth-service"}`))
}

func (h *httpAuthHandler) decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return fmt.Errorf("empty request body")
	}
	return json.Unmarshal(body, dst)
}

func (h *httpAuthHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *httpAuthHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		TenantName string `json:"tenant_name"`
	}
	if err := h.decodeJSON(r, &req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validator.ValidateRequiredFields(map[string]interface{}{
		"email":       req.Email,
		"password":    req.Password,
		"tenant_name": req.TenantName,
	}, map[string]string{
		"email":       "Email",
		"password":    "Password",
		"tenant_name": "Tenant name",
	}); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	tokenPair, err := h.authService.Register(r.Context(), req.Email, req.Password, req.TenantName)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, tokenPair)
}

func (h *httpAuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := h.decodeJSON(r, &req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validator.ValidateRequiredFields(map[string]interface{}{
		"email":    req.Email,
		"password": req.Password,
	}, map[string]string{
		"email":    "Email",
		"password": "Password",
	}); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	tokenPair, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, tokenPair)
}

func (h *httpAuthHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := h.decodeJSON(r, &req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validator.ValidateRequiredFields(map[string]interface{}{
		"refresh_token": req.RefreshToken,
	}, map[string]string{
		"refresh_token": "Refresh token",
	}); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	tokenPair, err := h.authService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		h.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, tokenPair)
}

func (h *httpAuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		AccessToken string `json:"access_token"`
	}
	if err := h.decodeJSON(r, &req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.AccessToken == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "access_token is required"})
		return
	}

	// Stateless logout: validate token; session revocation is refresh-token based.
	if _, err := h.jwtManager.ValidateAccessToken(req.AccessToken); err != nil {
		h.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (h *httpAuthHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		AccessToken string `json:"access_token"`
	}
	if err := h.decodeJSON(r, &req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.AccessToken == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "access_token is required"})
		return
	}

	claims, err := h.jwtManager.ValidateAccessToken(req.AccessToken)
	if err != nil {
		h.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	user, err := h.authService.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		h.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	roles, err := h.authService.GetUserRoles(r.Context(), claims.UserID)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get user roles"})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":     claims.UserID,
		"tenant_id":   claims.TenantID,
		"email":       user.Email,
		"is_admin":    claims.IsAdmin,
		"expires_at":  claims.ExpiresAt.Unix(),
		"roles":       roles,
		"permissions": claims.Permissions,
	})
}

func (h *httpAuthHandler) handleValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Key    string `json:"key"`
		Secret string `json:"secret"`
	}
	if err := h.decodeJSON(r, &req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Key == "" || req.Secret == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key and secret are required"})
		return
	}

	claims, err := h.authService.ValidateAPIKey(r.Context(), req.Key, req.Secret)
	if err != nil {
		h.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid api key"})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id": claims.TenantID,
		"key_id":    claims.KeyID,
		"is_valid":  true,
	})
}

func (h *httpAuthHandler) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	// В этом проекте API Gateway ходит в Auth Service по HTTP, поэтому делаем HTTP endpoint.
	// Поддерживаем создание ключа. Листинг можно будет добавить позже при необходимости.

	if r.Method != http.MethodPost {
		h.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		h.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := h.jwtManager.ValidateAccessToken(accessToken)
	if err != nil {
		h.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := h.decodeJSON(r, &req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	apiKeyPair, err := h.authService.CreateAPIKey(r.Context(), claims.TenantID, req.Name)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"key":       apiKeyPair.Key,
		"secret":    apiKeyPair.Secret,
		"name":      req.Name,
		"tenant_id": claims.TenantID,
	})
}
