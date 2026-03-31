package http

import (
	corev1 "UptimePingPlatform/proto/api/core/v1"
	forgev1 "UptimePingPlatform/proto/api/forge/v1"
	incidentv1 "UptimePingPlatform/proto/api/incident/v1"
	metricsv1 "UptimePingPlatform/proto/api/metrics/v1"
	notificationv1 "UptimePingPlatform/proto/api/notification/v1"
	schedulerv1 "UptimePingPlatform/proto/api/scheduler/v1"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	//"UptimePingPlatform/pkg/config"
	pkgErrors "UptimePingPlatform/pkg/errors"
	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/api-gateway/internal/client"
	"UptimePingPlatform/services/api-gateway/internal/middleware"
)

// UserInfo содержит информацию о пользователе
type UserInfo = client.UserInfo

// Handler структура для управления HTTP обработчиками
type Handler struct {
	mux                *mux.Router
	authService        client.AuthHTTPClientInterface
	healthHandler      HealthHandler
	schedulerClient    *client.SchedulerClient
	coreClient         *client.CoreClient
	metricsClient      *client.MetricsClient
	incidentClient     *client.IncidentClient
	notificationClient *client.NotificationClient
	configClient       *client.ConfigClient
	forgeClient        *client.GRPCForgeClient
	baseHandler        *grpcBase.BaseHandler
	logger             logger.Logger
	validator          *validation.Validator
}

// HealthHandler интерфейс для health check обработчика
type HealthHandler interface {
	HealthCheck(w http.ResponseWriter, r *http.Request)
	ReadyCheck(w http.ResponseWriter, r *http.Request)
	LiveCheck(w http.ResponseWriter, r *http.Request)
}

// NewHandler создает новый экземпляр Handler
func NewHandler(authService client.AuthHTTPClientInterface, healthHandler HealthHandler, schedulerClient *client.SchedulerClient, coreClient *client.CoreClient, metricsClient *client.MetricsClient, incidentClient *client.IncidentClient, notificationClient *client.NotificationClient, configClient *client.ConfigClient, forgeClient *client.GRPCForgeClient, logger logger.Logger) *Handler {
	h := &Handler{
		mux:                mux.NewRouter(),
		authService:        authService,
		healthHandler:      healthHandler,
		schedulerClient:    schedulerClient,
		coreClient:         coreClient,
		metricsClient:      metricsClient,
		incidentClient:     incidentClient,
		notificationClient: notificationClient,
		configClient:       configClient,
		forgeClient:        forgeClient,
		baseHandler:        grpcBase.NewBaseHandler(logger),
		logger:             logger,
		validator:          validation.NewValidator(),
	}

	h.logger.Info("DEBUG: About to setup routes")
	h.setupRoutes()
	h.logger.Info("DEBUG: Routes setup completed")
	return h
}

// ServeHTTP реализует интерфейс http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// setupRoutes настраивает маршруты для приложения
func (h *Handler) setupRoutes() {
	// Scheduler роуты для всех операций с проверками

	// Единый обработчик для всех операций с проверками по ID
	checkByIDHandler := h.handleProtected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// GET /api/v1/checks/{id} - требует checks:read
			middleware.PermissionMiddleware([]string{"checks:read"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleGetCheckByIDCompatible(w, r)
			})).ServeHTTP(w, r)
		case http.MethodPut:
			// PUT /api/v1/checks/{id} - требует checks:write
			middleware.PermissionMiddleware([]string{"checks:write"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleUpdateCheckByID(w, r)
			})).ServeHTTP(w, r)
		case http.MethodDelete:
			// DELETE /api/v1/checks/{id} - требует checks:delete
			middleware.PermissionMiddleware([]string{"checks:delete"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleDeleteCheckByIDCompatible(w, r)
			})).ServeHTTP(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		}
	}))

	h.mux.Handle("/api/v1/checks/{id}", checkByIDHandler).Methods(http.MethodGet, http.MethodPut, http.MethodDelete)

	// Роут для /api/v1/checks - операции с проверками (должен идти после {id})
	checksHandler := middleware.AuthMiddleware(h.authService, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем информацию о пользователе из контекста
		userDataCtx, ok := r.Context().Value("user").(map[string]interface{})
		if !ok {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
			return
		}

		tenantID, ok := userDataCtx["tenant_id"].(string)
		if !ok {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant_id not found"), http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// GET /api/v1/checks - требует checks:read
			middleware.PermissionMiddleware([]string{"checks:read"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleSchedulerChecks(w, r)
			})).ServeHTTP(w, r)
		case http.MethodPost:
			// POST /api/v1/checks - требует checks:write
			middleware.PermissionMiddleware([]string{"checks:write"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleCreateCheck(w, r, tenantID)
			})).ServeHTTP(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		}
	}))
	h.mux.Handle("/api/v1/checks", checksHandler).Methods(http.MethodGet, http.MethodPost)

	// Публичные роуты
	h.mux.HandleFunc("/api/v1/auth/login", h.handleLogin)
	h.mux.HandleFunc("/api/v1/auth/register", h.handleRegister)
	h.mux.HandleFunc("/api/v1/auth/refresh", h.handleRefreshToken)
	h.mux.HandleFunc("/api/v1/auth/logout", h.handleLogout)
	h.mux.HandleFunc("/api/v1/auth/validate", h.handleValidateToken)

	// API ключи (потребуют аутентификацию)
	h.mux.HandleFunc("/api/v1/auth/api-keys", h.handleAPIKeys)

	// Config роуты (требуют прав доступа)
	h.logger.Info("DEBUG: Setting up /api/v1/config route")
	configHandler := middleware.AuthMiddleware(h.authService, h.logger)(http.HandlerFunc(h.handleConfig))
	h.logger.Info("DEBUG: AuthMiddleware applied to /api/v1/config")
	configHandler = middleware.PermissionMiddleware([]string{"config:read"}, h.logger)(configHandler)
	h.logger.Info("DEBUG: PermissionMiddleware applied to /api/v1/config")
	h.mux.Handle("/api/v1/config", configHandler).Methods(http.MethodGet)
	h.logger.Info("DEBUG: /api/v1/config route registered")

	// Auth Service health endpoints (для тестирования) - ВАЖНО: регистрируем ПЕРЕД другими роутами
	h.logger.Info("DEBUG: Registering /api/v1/auth/health route")
	h.mux.Handle("/api/v1/auth/health", http.HandlerFunc(h.handleAuthHealthProxy))
	h.mux.Handle("/api/v1/scheduler/health", http.HandlerFunc(h.handleSchedulerHealthProxy))
	h.mux.Handle("/api/v1/core/health", http.HandlerFunc(h.handleCoreHealthProxy))

	// Health check роуты
	h.mux.HandleFunc("/health", h.healthHandler.HealthCheck)
	h.mux.HandleFunc("/ready", h.healthHandler.ReadyCheck)
	h.mux.HandleFunc("/live", h.healthHandler.LiveCheck)

	// Расписания проверок - используем mux.Handle для правильного паттерн-матчинга
	schedulesHandler := h.handleProtected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.handleScheduleProxy(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		}
	}))
	h.mux.Handle("/api/v1/schedules", schedulesHandler).Methods(http.MethodGet)

	// Расписания проверок с ID
	scheduleByIDHandler := h.handleProtected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		checkID := vars["id"]

		h.logger.Info("Route /api/v1/schedules/{id} matched",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("check_id", checkID))

		// Вызываем handleScheduleProxy с правильным контекстом
		h.handleScheduleProxy(w, r)
	}))
	h.mux.Handle("/api/v1/schedules/{id}", scheduleByIDHandler).Methods(http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodGet)

	// Core Service операции
	// Используем PathPrefix чтобы обрабатывать пути вида /api/v1/core, /api/v1/core/{id}, /api/v1/core/{id}/status и т.д.
	h.mux.PathPrefix("/api/v1/core").Handler(h.handleProtected(h.handleCoreProxy))

	// Metrics Service
	h.mux.HandleFunc("/api/v1/metrics", h.handleProtected(h.handleMetricsProxy))
	h.mux.HandleFunc("/api/v1/metrics/collect", h.handleProtected(h.handleMetricsProxy))

	// Incident Service - роут для списка инцидентов
	incidentsHandler := middleware.PermissionMiddleware([]string{"incidents:read"}, h.logger)(http.HandlerFunc(h.handleIncidents))
	h.mux.HandleFunc("/api/v1/incidents", incidentsHandler.ServeHTTP).Methods(http.MethodGet)

	// Incident Service - роут для конкретного инцидента
	incidentByIDHandler := middleware.AuthMiddleware(h.authService, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		incidentID := vars["id"]

		h.logger.Info("DEBUG: Route /api/v1/incidents/{id} matched!",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("incident_id", incidentID),
			logger.String("full_url", r.URL.String()))

		switch r.Method {
		case http.MethodGet:
			h.logger.Info("Handling GET /api/v1/incidents/{id} with incidents:read permission")
			// GET /api/v1/incidents/{id} - требует incidents:read
			middleware.PermissionMiddleware([]string{"incidents:read"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleIncidentProxy(w, r)
			})).ServeHTTP(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		}
	}))
	h.mux.Handle("/api/v1/incidents/{id}", incidentByIDHandler).Methods(http.MethodGet, http.MethodPut, http.MethodDelete)

	// Notification Service
	h.mux.HandleFunc("/api/v1/notifications", h.handleProtected(h.handleNotificationProxy))
	h.mux.HandleFunc("/api/v1/notifications/channels", h.handleProtected(h.handleNotificationProxy))

	// Добавляем роуты Forge Service
	h.mux.HandleFunc("/api/v1/forge/generate", h.handleProtected(h.handleForgeProxy))
	h.mux.HandleFunc("/api/v1/forge/parse", h.handleProtected(h.handleForgeProxy))
	h.mux.HandleFunc("/api/v1/forge/code", h.handleProtected(h.handleForgeProxy))
	h.mux.HandleFunc("/api/v1/forge/validate", h.handleProtected(h.handleForgeProxy))

	h.logger.Info("DEBUG: setupRoutes completed successfully!")
}

// handleProtected оборачивает обработчик, требующий аутентификации
func (h *Handler) handleProtected(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info("DEBUG: handleProtected called",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("full_url", r.URL.String()))

		// Извлекаем токен из Authorization header
		authHeader := r.Header.Get("Authorization")
		h.logger.Info("DEBUG: Authorization header",
			logger.String("auth_header", authHeader),
			logger.String("auth_header_length", fmt.Sprintf("%d", len(authHeader))))

		if authHeader == "" {
			h.logger.Error("Authorization header missing")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"code":    "401",
				"error":   "true",
				"message": "authorization header missing",
			})
			return
		}

		// Извлекаем токен из "Bearer <token>"
		token := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}

		// Проверяем аутентификацию через Auth Service
		h.logger.Info("DEBUG: Validating token",
			logger.String("token_length", fmt.Sprintf("%d", len(token))),
			logger.String("token_prefix", func() string {
				if len(token) > 10 {
					return token[:10]
				}
				return token
			}()))

		userInfo, err := h.authService.ValidateToken(r.Context(), token)
		if err != nil {
			h.logger.Error("Authentication failed",
				logger.Error(err),
				logger.String("path", r.URL.Path),
				logger.String("token_length", fmt.Sprintf("%d", len(token))))

			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"code":    "401",
				"error":   "true",
				"message": "authentication failed",
			})
			return
		}

		if userInfo == nil {
			h.logger.Error("UserInfo is nil after validation",
				logger.String("path", r.URL.Path))

			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"code":    "401",
				"error":   "true",
				"message": "user info not found",
			})
			return
		}

		h.logger.Info("DEBUG: Authentication successful",
			logger.String("user_id", userInfo.UserID),
			logger.String("tenant_id", userInfo.TenantID),
			logger.String("email", userInfo.Email))

		// Создаем структуру user для контекста как в AuthMiddleware
		userData := map[string]interface{}{
			"user_id":     userInfo.UserID,
			"tenant_id":   userInfo.TenantID,
			"email":       userInfo.Email,
			"is_admin":    userInfo.IsAdmin,
			"permissions": userInfo.Permissions,
		}

		// Добавляем информацию о пользователе в контекст
		ctx := context.WithValue(r.Context(), "user", userData)
		ctx = context.WithValue(ctx, "user_id", userInfo.UserID)
		ctx = context.WithValue(ctx, "tenant_id", userInfo.TenantID)
		ctx = context.WithValue(ctx, "email", userInfo.Email)
		ctx = context.WithValue(ctx, "is_admin", userInfo.IsAdmin)
		ctx = context.WithValue(ctx, "permissions", userInfo.Permissions)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// Остальные методы остаются без изменений...
// (здесь должны быть все остальные методы из оригинального файла)

// authenticateRequest выполняет полную аутентификацию запроса
func (h *Handler) authenticateRequest(r *http.Request) (*UserInfo, error) {
	// Сначала проверяем X-API-Key header (имеет приоритет)
	apiKeyHeader := r.Header.Get("X-API-Key")
	if apiKeyHeader != "" {
		return h.authenticateWithAPIKey(apiKeyHeader)
	}

	// Проверяем Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, pkgErrors.New(pkgErrors.ErrUnauthorized, "missing auth header")
	}

	// Проверяем формат JWT Bearer токена
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return h.authenticateWithJWT(token)
	}

	// Проверяем API ключ в формате "Api-Key <key>"
	if strings.HasPrefix(authHeader, "Api-Key ") {
		apiKey := strings.TrimPrefix(authHeader, "Api-Key ")
		return h.authenticateWithAPIKey(apiKey)
	}

	return nil, pkgErrors.New(pkgErrors.ErrUnauthorized, "unsupported auth format")
}

// authenticateWithJWT аутентифицирует пользователя через JWT токен
func (h *Handler) authenticateWithJWT(token string) (*UserInfo, error) {
	// Валидация формата токена
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, pkgErrors.New(pkgErrors.ErrUnauthorized, "invalid jwt format")
	}

	// Вызываем Auth Service для валидации токена
	ctx := context.Background()
	userInfo, err := h.authService.ValidateToken(ctx, token)
	if err != nil {
		return nil, pkgErrors.Wrap(err, pkgErrors.ErrUnauthorized, "token validation failed")
	}

	// Проверяем срок действия токена
	if time.Now().Unix() > userInfo.ExpiresAt {
		return nil, pkgErrors.New(pkgErrors.ErrUnauthorized, "token expired")
	}

	return userInfo, nil
}

// authenticateWithAPIKey аутентифицирует через API ключ
func (h *Handler) authenticateWithAPIKey(apiKey string) (*UserInfo, error) {
	ctx := context.Background()

	// Базовая валидация API ключа
	if len(apiKey) < 16 {
		return nil, pkgErrors.New(pkgErrors.ErrUnauthorized, "invalid api key length")
	}

	// Валидация API ключа через Auth Service
	tokenClaims, err := h.authService.ValidateToken(ctx, apiKey)
	if err != nil {
		h.logger.Error("API key validation failed", logger.Error(err))
		return nil, pkgErrors.Wrap(err, pkgErrors.ErrUnauthorized, "invalid api key")
	}

	// Конвертируем TokenClaims в UserInfo
	userInfo := &client.UserInfo{
		UserID:   tokenClaims.UserID,
		Email:    tokenClaims.Email,
		TenantID: tokenClaims.TenantID,
	}

	// Дополнительная проверка что это API ключ (не JWT токен)
	if userInfo.UserID == "validated-user" {
		// Это JWT токен, не API ключ
		return nil, pkgErrors.New(pkgErrors.ErrUnauthorized, "invalid api key format")
	}

	return userInfo, nil
}

// checkResourceAccess проверяет права доступа к ресурсу
func (h *Handler) checkResourceAccess(r *http.Request, userInfo *UserInfo) bool {
	// Получаем требуемые права для ресурса
	requiredPermissions := h.getRequiredPermissions(r)
	if len(requiredPermissions) == 0 {
		// Если права не определены, разрешаем доступ
		return true
	}

	// Проверяем наличие требуемых прав
	for _, required := range requiredPermissions {
		hasPermission := false
		for _, permission := range userInfo.Permissions {
			if permission == required || permission == "*" {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			return false
		}
	}

	return true
}

// getRequiredPermissions возвращает требуемые права для ресурса
func (h *Handler) getRequiredPermissions(r *http.Request) []string {
	path := r.URL.Path
	method := r.Method

	// Определяем права на основе пути и метода
	switch {
	case strings.HasPrefix(path, "/api/v1/checks"):
		switch method {
		case http.MethodGet:
			return []string{"checks:read"}
		case http.MethodPost:
			return []string{"checks:write"}
		case http.MethodPut:
			return []string{"checks:write"}
		case http.MethodDelete:
			return []string{"checks:delete"}
		default:
			return []string{"checks:read"}
		}
	case strings.HasPrefix(path, "/api/v1/schedules"):
		switch method {
		case http.MethodGet:
			return []string{"schedules:read"}
		case http.MethodPost:
			return []string{"schedules:write"}
		case http.MethodPut:
			return []string{"schedules:write"}
		case http.MethodDelete:
			return []string{"schedules:delete"}
		default:
			return []string{"schedules:read"}
		}
	case strings.HasPrefix(path, "/api/v1/incidents"):
		switch method {
		case http.MethodGet:
			return []string{"incidents:read"}
		case http.MethodPost:
			return []string{"incidents:write"}
		case http.MethodPut:
			return []string{"incidents:write"}
		default:
			return []string{"incidents:read"}
		}
	case strings.HasPrefix(path, "/api/v1/notifications"):
		return []string{"notifications:write"}
	case strings.HasPrefix(path, "/api/v1/metrics"):
		return []string{"metrics:read"}
	case strings.HasPrefix(path, "/api/v1/config"):
		switch method {
		case http.MethodGet:
			return []string{"config:read"}
		default:
			return []string{"config:write"}
		}
	case strings.HasPrefix(path, "/api/v1/forge"):
		return []string{"forge:write"}
	default:
		return []string{}
	}
}

// isAuthenticated проверяет аутентификацию запроса (устаревший метод для обратной совместимости)
// Поддерживает JWT токены в Authorization header или API ключи
func (h *Handler) isAuthenticated(r *http.Request) bool {
	ctx := r.Context()

	// Сначала проверяем X-API-Key header (имеет приоритет)
	apiKeyHeader := r.Header.Get("X-API-Key")
	if apiKeyHeader != "" {
		if len(apiKeyHeader) < 16 {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error": "invalid_x_api_key_length",
			})
			return false
		}

		h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
			"success": true,
			"method":  "x_api_key_header",
		})
		return true
	}

	// Проверяем Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
			"error": "missing_auth_header",
		})
		return false
	}

	// Проверяем формат JWT Bearer токена
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error": "empty_bearer_token",
			})
			return false
		}

		// Базовая валидация JWT токена (формат: header.payload.signature)
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error": "invalid_jwt_format",
			})
			return false
		}

		// ✅ РЕАЛИЗОВАНО: Проверка подписи и экспирации JWT токена
		// Декодируем payload для проверки экспирации
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error": "invalid_jwt_payload_encoding",
			})
			return false
		}

		// Парсим payload
		var claims map[string]interface{}
		if err := json.Unmarshal(payload, &claims); err != nil {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error": "invalid_jwt_payload_json",
			})
			return false
		}

		// Проверяем экспирацию (exp)
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
					"error": "jwt_expired",
					"exp":   exp,
				})
				return false
			}
		} else {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error": "jwt_missing_exp_claim",
			})
			return false
		}

		// Проверяем issued at (iat)
		if iat, ok := claims["iat"].(float64); ok {
			if time.Now().Unix() < int64(iat) {
				h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
					"error": "jwt_issued_in_future",
					"iat":   iat,
				})
				return false
			}
		}

		// Проверяем not before (nbf) если есть
		if nbf, ok := claims["nbf"].(float64); ok {
			if time.Now().Unix() < int64(nbf) {
				h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
					"error": "jwt_not_yet_valid",
					"nbf":   nbf,
				})
				return false
			}
		}

		// ✅ РЕАЛИЗОВАНО: Проверка подписи через Auth Service
		// Вызываем Auth Service для валидации токена
		userInfo, err := h.authService.ValidateToken(ctx, token)
		if err != nil {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error":   "auth_service_validation_failed",
				"details": err.Error(),
			})
			return false
		}

		// Проверяем, что токен не истек (дополнительная проверка)
		if userInfo.ExpiresAt > 0 && time.Now().Unix() > userInfo.ExpiresAt {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error":      "token_expired_in_auth_service",
				"expires_at": userInfo.ExpiresAt,
			})
			return false
		}

		// ✅ УСПЕШНАЯ ВАЛИДАЦИЯ через Auth Service
		h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
			"success":      true,
			"method":       "jwt_bearer",
			"user_id":      userInfo.UserID,
			"tenant_id":    userInfo.TenantID,
			"email":        userInfo.Email,
			"is_admin":     userInfo.IsAdmin,
			"validated_by": "auth_service",
		})
		return true
	}

	// Проверяем API ключ в формате "Api-Key <key>"
	if strings.HasPrefix(authHeader, "Api-Key ") {
		apiKey := strings.TrimPrefix(authHeader, "Api-Key ")
		if apiKey == "" {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error": "empty_api_key",
			})
			return false
		}

		// Базовая валидация API ключа (минимальная длина)
		if len(apiKey) < 16 {
			h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
				"error": "invalid_api_key_length",
			})
			return false
		}

		h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
			"success": true,
			"method":  "api_key_header",
		})
		return true
	}

	h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
		"error": "unsupported_auth_format",
	})
	return false
}

// handleLogin обрабатывает запросы на аутентификацию
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация входных данных с использованием pkg/validation
	if h.validator == nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "validator not initialized"), http.StatusInternalServerError)
		return
	}

	requiredFields := map[string]interface{}{
		"email":    req.Email,
		"password": req.Password,
	}

	if err := h.validator.ValidateRequiredFields(requiredFields, map[string]string{
		"email":    "Email address",
		"password": "Password",
	}); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "validation failed"), http.StatusBadRequest)
		return
	}

	// Валидация формата email
	if err := h.validator.ValidateStringLength(req.Email, "email", 5, 100); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid email format"), http.StatusBadRequest)
		return
	}

	// Валидация длины пароля
	if err := h.validator.ValidateStringLength(req.Password, "password", 8, 128); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid password length"), http.StatusBadRequest)
		return
	}

	// Вызов сервиса аутентификации
	ctx := r.Context()
	h.logger.Info("Calling Login method", logger.String("email", req.Email))

	tokenPair, err := h.authService.Login(ctx, req.Email, req.Password)
	if err != nil {
		h.logger.Error("Login failed", logger.Error(err))
		h.handleError(w, err)
		return
	}

	h.logger.Info("Login successful", logger.String("email", req.Email))

	// Формирование ответа
	response := map[string]interface{}{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"tenant_id":     tokenPair.TenantID, // Добавлено
	}

	h.logger.Info("Отправка ответа login",
		logger.String("email", req.Email),
		logger.Bool("has_access_token", tokenPair.AccessToken != ""),
		logger.Bool("has_refresh_token", tokenPair.RefreshToken != ""),
		logger.String("tenant_id", tokenPair.TenantID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("ошибка кодирования ответа", logger.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Ответ login успешно отправлен", logger.String("email", req.Email))
}

// handleRegister обрабатывает запросы на регистрацию
func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("handleRegister called",
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path),
		logger.String("full_url", r.URL.String()))

	if r.Method != http.MethodPost {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		TenantName string `json:"tenant_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация входных данных с использованием pkg/validation
	if h.validator == nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "validator not initialized"), http.StatusInternalServerError)
		return
	}

	requiredFields := map[string]interface{}{
		"email":       req.Email,
		"password":    req.Password,
		"tenant_name": req.TenantName,
	}

	if err := h.validator.ValidateRequiredFields(requiredFields, map[string]string{
		"email":       "Email address",
		"password":    "Password",
		"tenant_name": "Tenant name",
	}); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "validation failed"), http.StatusBadRequest)
		return
	}

	// Валидация формата email
	if err := h.validator.ValidateStringLength(req.Email, "email", 5, 100); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid email format"), http.StatusBadRequest)
		return
	}

	// Валидация длины пароля
	if err := h.validator.ValidateStringLength(req.Password, "password", 8, 128); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid password length"), http.StatusBadRequest)
		return
	}

	// Валидация длины имени тенанта
	if err := h.validator.ValidateStringLength(req.TenantName, "tenant_name", 2, 100); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid tenant name length"), http.StatusBadRequest)
		return
	}

	// Вызов сервиса аутентификации
	ctx := r.Context()
	h.logger.Info("Calling Register method", logger.String("email", req.Email))

	// Defensive check for authService
	if h.authService == nil {
		h.logger.Error("Auth service is nil")
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "auth service not initialized"), http.StatusInternalServerError)
		return
	}

	tokenPair, err := h.authService.Register(ctx, req.Email, req.Password, req.TenantName)
	if err != nil {
		h.logger.Error("Registration failed", logger.Error(err))
		h.handleError(w, err)
		return
	}

	h.logger.Info("Registration successful", logger.String("email", req.Email))

	// Формирование ответа
	response := map[string]interface{}{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"tenant_id":     tokenPair.TenantID, // Добавлено
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleRefreshToken обрабатывает запросы на обновление токена
func (h *Handler) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация с использованием pkg/validation
	if h.validator == nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "validator not initialized"), http.StatusInternalServerError)
		return
	}

	requiredFields := map[string]interface{}{
		"refresh_token": req.RefreshToken,
	}

	if err := h.validator.ValidateRequiredFields(requiredFields, map[string]string{
		"refresh_token": "Refresh token",
	}); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "validation failed"), http.StatusBadRequest)
		return
	}

	// Валидация длины refresh токена (JWT токены обычно длинные)
	if h.validator == nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "validator not initialized"), http.StatusInternalServerError)
		return
	}
	if err := h.validator.ValidateStringLength(req.RefreshToken, "refresh_token", 100, 1000); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid refresh token length"), http.StatusBadRequest)
		return
	}

	// Вызов сервиса
	ctx := r.Context()
	tokenPair, err := h.authService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Формирование ответа
	response := map[string]interface{}{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleLogout обрабатывает запросы на выход из системы
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация access_token
	if req.AccessToken == "" {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "access_token is required"), http.StatusBadRequest)
		return
	}

	// Вызов сервиса
	ctx := r.Context()
	err := h.authService.Logout(ctx, req.AccessToken)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Формирование ответа
	response := map[string]string{
		"message": "Logged out successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleValidateToken обрабатывает запросы на валидацию токена
func (h *Handler) handleValidateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация с использованием pkg/validation
	requiredFields := map[string]interface{}{
		"access_token": req.AccessToken,
	}

	if err := h.validator.ValidateRequiredFields(requiredFields, map[string]string{
		"access_token": "Access Token",
	}); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "validation failed"), http.StatusBadRequest)
		return
	}

	// Вызов сервиса
	ctx := r.Context()
	userInfo, err := h.authService.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Формирование ответа
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userInfo)
}

// handleChecksProxy проксирует запросы к Scheduler Service
func (h *Handler) handleChecksProxy(w http.ResponseWriter, r *http.Request) {
	// Получаем информацию о пользователе из контекста
	userDataCtx, ok := r.Context().Value("user").(map[string]interface{})
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	tenantID, ok := userDataCtx["tenant_id"].(string)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant_id not found"), http.StatusUnauthorized)
		return
	}

	// Извлекаем ID проверки из URL пути для операций с конкретной проверкой
	checkID := extractCheckIDFromPath(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		if checkID != "" {
			h.handleGetCheck(w, r, tenantID, checkID)
		} else {
			h.handleListChecks(w, r, tenantID)
		}
	case http.MethodPost:
		h.handleCreateCheck(w, r, tenantID)
	case http.MethodPut:
		if checkID != "" {
			h.handleUpdateCheck(w, r, tenantID, checkID)
		} else {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "check ID required"), http.StatusBadRequest)
		}
	case http.MethodDelete:
		if checkID != "" {
			h.handleDeleteCheck(w, r, tenantID, checkID)
		} else {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "check ID required"), http.StatusBadRequest)
		}
	default:
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
	}
}

// extractCheckIDFromPath извлекает ID проверки из URL пути
func extractCheckIDFromPath(path string) string {
	// Пример: /api/v1/checks/12345 -> 12345
	// Пример: /api/v1/core/12345 -> 12345
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if (part == "checks" || part == "core") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// extractScheduleIDFromPath извлекает ID расписания из URL пути
func extractScheduleIDFromPath(path string) string {
	// Пример: /api/v1/schedules/12345 -> 12345
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "schedules" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// handleListHandles обрабатывает получение списка проверок
func (h *Handler) handleListChecks(w http.ResponseWriter, r *http.Request, tenantID string) {
	req := &schedulerv1.ListChecksRequest{
		TenantId: tenantID,
	}

	resp, err := h.schedulerClient.ListChecks(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"checks": resp.Checks,
		"total":  len(resp.Checks),
	})
}

// handleCreateCheck обрабатывает создание новой проверки
func (h *Handler) handleCreateCheck(w http.ResponseWriter, r *http.Request, tenantID string) {
	h.logger.Info("DEBUG: handleCreateCheck called",
		logger.String("tenant_id", tenantID),
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	var createReq schedulerv1.CreateCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
		h.logger.Info("DEBUG: JSON decode error", logger.Error(err))
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	h.logger.Info("DEBUG: Request decoded",
		logger.String("name", createReq.Name),
		logger.String("type", createReq.Type),
		logger.String("target", createReq.Target))

	// Устанавливаем tenant_id из контекста
	createReq.TenantId = tenantID

	h.logger.Info("DEBUG: Calling schedulerClient.CreateCheck")
	check, err := h.schedulerClient.CreateCheck(r.Context(), &createReq)
	if err != nil {
		h.logger.Info("DEBUG: schedulerClient.CreateCheck error", logger.Error(err))
		h.handleError(w, err)
		return
	}

	h.logger.Info("DEBUG: Check created successfully")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Check created",
		"check":   check,
	})
}

// handleGetCheck обрабатывает получение конкретной проверки
func (h *Handler) handleGetCheck(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	h.logger.Info("handleGetCheck вызван",
		logger.String("check_id", checkID),
		logger.String("tenant_id", tenantID))

	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	req := &schedulerv1.GetCheckRequest{
		CheckId: checkID,
	}

	h.logger.Info("Отправка gRPC запроса в Scheduler Service",
		logger.String("check_id", checkID))

	// Добавляем timeout для предотвращения зависания
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	check, err := h.schedulerClient.GetCheck(ctx, req)
	if err != nil {
		h.logger.Error("ошибка получения проверки из Scheduler Service",
			logger.Error(err),
			logger.String("check_id", checkID))
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "Scheduler Service недоступен"), http.StatusServiceUnavailable)
		return
	}

	h.logger.Info("Проверка успешно получена из Scheduler Service",
		logger.String("check_id", checkID))

	// Проверяем, что проверка принадлежит тенанту
	if check.TenantId != tenantID {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrForbidden, "access denied"), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"check":   check,
	})
}

// handleUpdateCheckByID обрабатывает обновление проверки по ID
func (h *Handler) handleUpdateCheckByID(w http.ResponseWriter, r *http.Request) {
	// Извлекаем checkID из URL
	vars := mux.Vars(r)
	checkID := vars["id"]

	// Получаем tenant_id из контекста (установленный handleProtected)
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant_id not found"), http.StatusUnauthorized)
		return
	}

	// Вызываем основную функцию
	h.handleUpdateCheck(w, r, tenantID, checkID)
}

// handleUpdateCheck обрабатывает обновление проверки
func (h *Handler) handleUpdateCheck(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	var updateReq schedulerv1.UpdateCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	updateReq.CheckId = checkID
	// updateReq.TenantId = tenantID // НЕ НУЖЕН - tenant_id берется из gRPC контекста

	h.logger.Info("Updating check",
		logger.String("check_id", checkID),
		logger.String("tenant_id", tenantID))

	check, err := h.schedulerClient.UpdateCheck(r.Context(), &updateReq)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Проверяем, что проверка принадлежит тенанту
	h.logger.Info("Checking tenant ownership",
		logger.String("check_tenant_id", check.TenantId),
		logger.String("user_tenant_id", tenantID),
		logger.String("check_id", checkID))

	if check.TenantId != tenantID {
		h.logger.Error("Tenant ID mismatch",
			logger.String("check_tenant_id", check.TenantId),
			logger.String("user_tenant_id", tenantID))
		h.writeError(w, pkgErrors.New(pkgErrors.ErrForbidden, "access denied"), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Check updated",
		"check":   check,
	})
}

// handleGetCheckByIDCompatible обрабатывает получение проверки по ID для handleProtected
func (h *Handler) handleGetCheckByIDCompatible(w http.ResponseWriter, r *http.Request) {
	// Извлекаем checkID из URL
	vars := mux.Vars(r)
	checkID := vars["id"]

	// Вызываем основную функцию
	h.handleGetCheckByID(w, r, checkID)
}

// handleGetCheckByID обрабатывает получение проверки по ID
func (h *Handler) handleGetCheckByID(w http.ResponseWriter, r *http.Request, checkID string) {
	h.logger.Info("=== handleGetCheckByID called ===",
		logger.String("check_id", checkID),
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	// Создаем gRPC запрос
	req := &schedulerv1.GetCheckRequest{
		CheckId: checkID,
	}

	// Вызываем Scheduler Service
	check, err := h.schedulerClient.GetCheck(r.Context(), req)
	if err != nil {
		h.logger.Error("Error getting check",
			logger.Error(err),
			logger.String("check_id", checkID))
		h.handleError(w, err)
		return
	}

	h.logger.Info("Check retrieved successfully",
		logger.String("check_id", checkID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(check)
}

// handleDeleteCheckByIDCompatible обрабатывает удаление проверки по ID для handleProtected
func (h *Handler) handleDeleteCheckByIDCompatible(w http.ResponseWriter, r *http.Request) {
	// Извлекаем checkID из URL
	vars := mux.Vars(r)
	checkID := vars["id"]

	// Получаем tenant_id из контекста (установленный handleProtected)
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant_id not found"), http.StatusUnauthorized)
		return
	}

	// Вызываем основную функцию с tenantID
	h.handleDeleteCheckByIDWithTenant(w, r, checkID, tenantID)
}

// handleDeleteCheckByIDWithTenant обрабатывает удаление проверки по ID с tenantID
func (h *Handler) handleDeleteCheckByIDWithTenant(w http.ResponseWriter, r *http.Request, checkID, tenantID string) {
	h.logger.Info("=== handleDeleteCheckByIDWithTenant called ===",
		logger.String("check_id", checkID),
		logger.String("tenant_id", tenantID),
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	// Создаем gRPC запрос
	req := &schedulerv1.DeleteCheckRequest{
		CheckId: checkID,
		// TenantId: tenantID, // НЕ НУЖЕН - tenant_id берется из gRPC контекста
	}

	// Вызываем Scheduler Service
	_, err := h.schedulerClient.DeleteCheck(r.Context(), req)
	if err != nil {
		h.logger.Error("Error deleting check",
			logger.Error(err),
			logger.String("check_id", checkID))
		h.handleError(w, err)
		return
	}

	h.logger.Info("Check deleted successfully",
		logger.String("check_id", checkID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Check deleted successfully",
		"check_id": checkID,
	})
}

// handleDeleteCheckByID обрабатывает удаление проверки по ID
func (h *Handler) handleDeleteCheckByID(w http.ResponseWriter, r *http.Request, checkID string) {
	h.logger.Info("=== handleDeleteCheckByID called ===",
		logger.String("check_id", checkID),
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	// Создаем gRPC запрос
	req := &schedulerv1.DeleteCheckRequest{
		CheckId: checkID,
	}

	// Вызываем Scheduler Service
	_, err := h.schedulerClient.DeleteCheck(r.Context(), req)
	if err != nil {
		h.logger.Error("Error deleting check",
			logger.Error(err),
			logger.String("check_id", checkID))
		h.handleError(w, err)
		return
	}

	h.logger.Info("Check deleted successfully",
		logger.String("check_id", checkID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Check deleted successfully",
		"check_id": checkID,
	})
}

// handleDeleteCheck обрабатывает удаление проверки
func (h *Handler) handleDeleteCheck(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	req := &schedulerv1.DeleteCheckRequest{
		CheckId: checkID,
	}

	_, err := h.schedulerClient.DeleteCheck(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Check deleted",
	})
}

// handleScheduleProxy обрабатывает запросы к расписаниям проверок
func (h *Handler) handleScheduleProxy(w http.ResponseWriter, r *http.Request) {
	// Получаем информацию о пользователе из контекста
	userDataCtx, ok := r.Context().Value("user").(map[string]interface{})
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	tenantID, ok := userDataCtx["tenant_id"].(string)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant_id not found"), http.StatusUnauthorized)
		return
	}

	// Извлекаем ID проверки из URL пути с помощью mux.Vars
	vars := mux.Vars(r)
	checkID := vars["id"]

	// Отладочный лог
	h.logger.Info("Schedule proxy debug",
		logger.String("path", r.URL.Path),
		logger.String("method", r.Method),
		logger.String("checkID", checkID),
		logger.String("tenantID", tenantID))

	switch r.Method {
	case http.MethodPost:
		if checkID != "" {
			h.handleScheduleCheck(w, r, tenantID, checkID)
		} else {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "check ID required"), http.StatusBadRequest)
		}
	case http.MethodPut:
		if checkID != "" {
			h.handleUpdateSchedule(w, r, tenantID, checkID)
		} else {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "check ID required"), http.StatusBadRequest)
		}
	case http.MethodDelete:
		if checkID != "" {
			h.handleUnscheduleCheck(w, r, tenantID, checkID)
		} else {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "check ID required"), http.StatusBadRequest)
		}
	case http.MethodGet:
		if checkID != "" {
			h.handleGetSchedule(w, r, tenantID, checkID)
		} else {
			h.handleListSchedules(w, r, tenantID)
		}
	default:
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
	}
}

// handleScheduleCheck обрабатывает планирование проверки
func (h *Handler) handleScheduleCheck(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	var req schedulerv1.ScheduleCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	req.CheckId = checkID

	// Валидация cron выражения
	if err := h.validator.ValidateCronExpression(req.CronExpression); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid cron expression"), http.StatusBadRequest)
		return
	}

	schedule, err := h.schedulerClient.ScheduleCheck(r.Context(), &req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Check scheduled",
		"schedule": schedule,
	})
}

// handleUpdateSchedule обрабатывает обновление расписания проверки
func (h *Handler) handleUpdateSchedule(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	var req schedulerv1.UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	req.CheckId = checkID

	// Валидация cron выражения
	if err := h.validator.ValidateCronExpression(req.CronExpression); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid cron expression"), http.StatusBadRequest)
		return
	}

	schedule, err := h.schedulerClient.UpdateSchedule(r.Context(), &req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Schedule updated",
		"schedule": schedule,
	})
}

// handleUnscheduleCheck обрабатывает отмену планирования проверки
func (h *Handler) handleUnscheduleCheck(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	req := &schedulerv1.UnscheduleCheckRequest{
		CheckId: checkID,
	}

	resp, err := h.schedulerClient.UnscheduleCheck(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": resp.Success,
		"message": "Check unscheduled",
	})
}

// handleGetSchedule обрабатывает получение расписания проверки
func (h *Handler) handleGetSchedule(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	req := &schedulerv1.GetScheduleRequest{
		CheckId: checkID,
	}

	schedule, err := h.schedulerClient.GetSchedule(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"schedule": schedule,
	})
}

// handleListSchedules обрабатывает получение списка расписаний
func (h *Handler) handleListSchedules(w http.ResponseWriter, r *http.Request, tenantID string) {
	req := &schedulerv1.ListSchedulesRequest{
		// Используем фильтр для tenant_id, так как прямое поле не поддерживается
		Filter: fmt.Sprintf("tenant_id:%s", tenantID),
	}

	resp, err := h.schedulerClient.ListSchedules(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"schedules": resp.Schedules,
		"total":     len(resp.Schedules),
	})
}

// handleCoreProxy обрабатывает запросы к Core Service
func (h *Handler) handleCoreProxy(w http.ResponseWriter, r *http.Request) {
	// Получаем информацию о пользователе из контекста
	userDataCtx, ok := r.Context().Value("user").(map[string]interface{})
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	tenantID, ok := userDataCtx["tenant_id"].(string)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant_id not found"), http.StatusUnauthorized)
		return
	}

	// Извлекаем ID проверки из URL пути
	checkID := extractCheckIDFromPath(r.URL.Path)
	if checkID == "" {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "check ID required"), http.StatusBadRequest)
		return
	}

	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handleExecuteCheck(w, r, tenantID, checkID)
	case http.MethodGet:
		if strings.HasSuffix(r.URL.Path, "/status") {
			h.handleGetCheckStatus(w, r, tenantID, checkID)
		} else if strings.HasSuffix(r.URL.Path, "/history") {
			h.handleGetCheckHistory(w, r, tenantID, checkID)
		} else {
			h.handleGetCheckStatus(w, r, tenantID, checkID)
		}
	default:
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
	}
}

// handleExecuteCheck обрабатывает немедленное выполнение проверки
func (h *Handler) handleExecuteCheck(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	req := &corev1.ExecuteCheckRequest{
		CheckId: checkID,
	}

	result, err := h.coreClient.ExecuteCheck(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Формируем ответ на основе proto CheckResult
	resp := map[string]interface{}{
		"success":      result.GetStatus() == "up",
		"execution_id": "", // core.CheckResult не содержит execution_id; оставляем пустым
		"duration_ms":  result.GetResponseTimeMs(),
		"status_code":  result.GetStatusCode(),
		"error":        result.GetErrorMessage(),
		"checked_at":   result.GetCreatedAt(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// handleGetCheckStatus обрабатывает получение статуса проверки
func (h *Handler) handleGetCheckStatus(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	req := &corev1.GetCheckStatusRequest{
		CheckId: checkID,
	}

	status, err := h.coreClient.GetCheckStatus(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"check_id":         status.CheckId,
		"is_healthy":       true,       // status.IsHealthy,
		"response_time_ms": 0,          // status.ResponseTimeMs,
		"last_checked_at":  time.Now(), // status.LastCheckedAt,
	})
}

// handleGetCheckHistory обрабатывает получение истории выполнения проверки
func (h *Handler) handleGetCheckHistory(w http.ResponseWriter, r *http.Request, tenantID, checkID string) {
	// Парсинг query параметров для пагинации
	page := 1
	pageSize := 50
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	req := &corev1.GetCheckHistoryRequest{
		CheckId: checkID,
		Limit:   int32(pageSize),
	}

	history, err := h.coreClient.GetCheckHistory(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"executions": history.Results,
		"page":       page,
		"page_size":  pageSize,
		"total":      len(history.Results),
	})
}

// handleForgeProxy проксирует запросы к Forge Service
func (h *Handler) handleForgeProxy(w http.ResponseWriter, r *http.Request) {
	// Проверка метода
	if r.Method != http.MethodPost {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		ProtoContent string                 `json:"proto_content"`
		FileName     string                 `json:"file_name,omitempty"`
		Options      map[string]interface{} `json:"options,omitempty"`
		Action       string                 `json:"action"` // "generate_config", "parse_proto", "generate_code", "validate_proto"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация обязательных полей
	requiredFields := map[string]interface{}{
		"proto_content": req.ProtoContent,
		"action":        req.Action,
	}

	if err := h.validator.ValidateRequiredFields(requiredFields, map[string]string{
		"proto_content": "Proto content",
		"action":        "Action",
	}); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "validation failed"), http.StatusBadRequest)
		return
	}

	// Валидация действия
	validActions := []string{"generate_config", "parse_proto", "generate_code", "validate_proto"}
	if err := h.validator.ValidateEnum(req.Action, validActions, "action"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid action"), http.StatusBadRequest)
		return
	}

	// Валидация длины proto контента
	if err := h.validator.ValidateStringLength(req.ProtoContent, "proto_content", 10, 1000000); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "proto content too long or too short"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Выполнение действия в зависимости от типа
	switch req.Action {
	case "generate_config":
		h.handleGenerateConfig(ctx, w, req)
	case "parse_proto":
		h.handleParseProto(ctx, w, req)
	case "generate_code":
		h.handleGenerateCode(ctx, w, req)
	case "validate_proto":
		h.handleValidateProto(ctx, w, req)
	default:
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "unsupported action"), http.StatusBadRequest)
	}
}

// handleGenerateConfig обрабатывает генерацию конфигурации
func (h *Handler) handleGenerateConfig(ctx context.Context, w http.ResponseWriter, req struct {
	ProtoContent string                 `json:"proto_content"`
	FileName     string                 `json:"file_name,omitempty"`
	Options      map[string]interface{} `json:"options,omitempty"`
	Action       string                 `json:"action"`
}) {
	// Создаем опции конфигурации
	options := &forgev1.ConfigOptions{}
	if req.Options != nil {
		if targetHost, ok := req.Options["target_host"].(string); ok {
			options.TargetHost = targetHost
		}
		if targetPort, ok := req.Options["target_port"].(float64); ok {
			options.TargetPort = int32(targetPort)
		}
		if checkInterval, ok := req.Options["check_interval"].(float64); ok {
			options.CheckInterval = int32(checkInterval)
		}
		if timeout, ok := req.Options["timeout"].(float64); ok {
			options.Timeout = int32(timeout)
		}
		if tenantID, ok := req.Options["tenant_id"].(string); ok {
			options.TenantId = tenantID
		}
		if metadata, ok := req.Options["metadata"].(map[string]interface{}); ok {
			options.Metadata = make(map[string]string)
			for k, v := range metadata {
				if str, ok := v.(string); ok {
					options.Metadata[k] = str
				}
			}
		}
	}

	// Вызываем Forge Service
	resp, err := h.forgeClient.GenerateConfig(ctx, req.ProtoContent, options)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Формирование ответа
	response := map[string]interface{}{
		"success":      true,
		"message":      "Configuration generated successfully",
		"config_yaml":  resp.ConfigYaml,
		"check_config": resp.CheckConfig,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleParseProto обрабатывает парсинг proto файла
func (h *Handler) handleParseProto(ctx context.Context, w http.ResponseWriter, req struct {
	ProtoContent string                 `json:"proto_content"`
	FileName     string                 `json:"file_name,omitempty"`
	Options      map[string]interface{} `json:"options,omitempty"`
	Action       string                 `json:"action"`
}) {
	resp, err := h.forgeClient.ParseProto(ctx, req.ProtoContent, req.FileName)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Формирование ответа
	response := map[string]interface{}{
		"success":      true,
		"message":      "Proto parsed successfully",
		"service_info": resp.ServiceInfo,
		"is_valid":     resp.IsValid,
		"warnings":     resp.Warnings,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGenerateCode обрабатывает генерацию кода
func (h *Handler) handleGenerateCode(ctx context.Context, w http.ResponseWriter, req struct {
	ProtoContent string                 `json:"proto_content"`
	FileName     string                 `json:"file_name,omitempty"`
	Options      map[string]interface{} `json:"options,omitempty"`
	Action       string                 `json:"action"`
}) {
	// Создаем опции генерации кода
	options := &forgev1.CodeOptions{}
	if req.Options != nil {
		if language, ok := req.Options["language"].(string); ok {
			options.Language = language
		}
		if framework, ok := req.Options["framework"].(string); ok {
			options.Framework = framework
		}
		if template, ok := req.Options["template"].(string); ok {
			options.Template = template
		}
	}

	// Проверяем, что forgeClient доступен
	if h.forgeClient == nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "Forge Service client is not available"), http.StatusServiceUnavailable)
		return
	}

	resp, err := h.forgeClient.GenerateCode(ctx, req.ProtoContent, options)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Формирование ответа
	response := map[string]interface{}{
		"success":  true,
		"message":  "Code generated successfully",
		"code":     resp.Code,
		"filename": resp.Filename,
		"language": resp.Language,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleValidateProto обрабатывает валидацию proto файла
func (h *Handler) handleValidateProto(ctx context.Context, w http.ResponseWriter, req struct {
	ProtoContent string                 `json:"proto_content"`
	FileName     string                 `json:"file_name,omitempty"`
	Options      map[string]interface{} `json:"options,omitempty"`
	Action       string                 `json:"action"`
}) {
	// Проверяем, что forgeClient доступен
	if h.forgeClient == nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "Forge Service client is not available"), http.StatusServiceUnavailable)
		return
	}

	resp, err := h.forgeClient.ValidateProto(ctx, req.ProtoContent)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Формирование ответа
	response := map[string]interface{}{
		"success":  true,
		"message":  "Proto validated successfully",
		"is_valid": resp.IsValid,
		"errors":   resp.Errors,
		"warnings": resp.Warnings,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleMetricsProxy обрабатывает запросы к Metrics Service
func (h *Handler) handleMetricsProxy(w http.ResponseWriter, r *http.Request) {
	// Получаем информацию о пользователе из контекста
	userDataCtx, ok := r.Context().Value("user").(map[string]interface{})
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	tenantID, ok := userDataCtx["tenant_id"].(string)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant_id not found"), http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetMetrics(w, r, tenantID)
	case http.MethodPost:
		if strings.HasSuffix(r.URL.Path, "/collect") {
			h.handleCollectMetrics(w, r, tenantID)
		} else {
			h.handleGetMetrics(w, r, tenantID)
		}
	default:
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
	}
}

// handleCollectMetrics обрабатывает сбор метрик
func (h *Handler) handleCollectMetrics(w http.ResponseWriter, r *http.Request, tenantID string) {
	var req metricsv1.CollectMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// req.TenantId = tenantID

	resp, err := h.metricsClient.CollectMetrics(r.Context(), &req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       resp.Success,
		"metrics_count": resp.MetricsCount,
		"collected_at":  resp.CollectedAt,
	})
}

// handleGetMetrics обрабатывает получение метрик
func (h *Handler) handleGetMetrics(w http.ResponseWriter, r *http.Request, tenantID string) {
	req := &metricsv1.GetMetricsRequest{
		TenantId:    tenantID,
		ServiceName: r.URL.Query().Get("service_name"),
	}

	resp, err := h.metricsClient.GetMetrics(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"metrics": resp.Metrics,
		"total":   len(resp.Metrics),
	})
}

// handleIncidentProxy обрабатывает запросы к Incident Service
func (h *Handler) handleIncidentProxy(w http.ResponseWriter, r *http.Request) {
	// Получаем информацию о пользователе из контекста
	userDataCtx, ok := r.Context().Value("user").(map[string]interface{})
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	tenantID, ok := userDataCtx["tenant_id"].(string)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant_id not found"), http.StatusUnauthorized)
		return
	}

	// Извлекаем ID инцидента из URL пути
	incidentID := extractIDFromPath(r.URL.Path, "incidents")

	switch r.Method {
	case http.MethodGet:
		if incidentID != "" {
			h.handleGetIncident(w, r, tenantID, incidentID)
		} else {
			h.handleListIncidents(w, r, tenantID)
		}
	case http.MethodPost:
		h.handleCreateIncident(w, r, tenantID)
	case http.MethodPut:
		if incidentID != "" {
			h.handleResolveIncident(w, r, tenantID, incidentID)
		} else {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "incident ID required"), http.StatusBadRequest)
		}
	default:
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
	}
}

// handleCreateIncident обрабатывает создание инцидента
func (h *Handler) handleCreateIncident(w http.ResponseWriter, r *http.Request, tenantID string) {
	var req incidentv1.CreateIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// req.TenantId = tenantID

	incident, err := h.incidentClient.CreateIncident(r.Context(), &req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Incident created",
		"incident": incident,
	})
}

// handleGetIncident обрабатывает получение инцидента
func (h *Handler) handleGetIncident(w http.ResponseWriter, r *http.Request, tenantID, incidentID string) {
	// Валидация UUID
	if err := h.validator.ValidateUUID(incidentID, "incident_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid incident ID format"), http.StatusBadRequest)
		return
	}

	req := &incidentv1.GetIncidentRequest{
		IncidentId: incidentID,
	}

	incident, err := h.incidentClient.GetIncident(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Проверяем, что инцидент принадлежит тенанту
	// if incident.TenantId != tenantID {
	// 	h.writeError(w, pkgErrors.New(pkgErrors.ErrForbidden, "access denied"), http.StatusForbidden)
	// 	return
	// }

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"incident": incident,
	})
}

// handleListIncidents обрабатывает получение списка инцидентов
func (h *Handler) handleListIncidents(w http.ResponseWriter, r *http.Request, tenantID string) {
	req := &incidentv1.ListIncidentsRequest{
		// TenantId: tenantID,
	}

	resp, err := h.incidentClient.ListIncidents(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"incidents": resp.Incidents,
		"total":     len(resp.Incidents),
	})
}

// handleResolveIncident обрабатывает разрешение инцидента
func (h *Handler) handleResolveIncident(w http.ResponseWriter, r *http.Request, tenantID, incidentID string) {
	// Валидация UUID
	if err := h.validator.ValidateUUID(incidentID, "incident_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid incident ID format"), http.StatusBadRequest)
		return
	}

	var req incidentv1.ResolveIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	req.IncidentId = incidentID

	resp, err := h.incidentClient.ResolveIncident(r.Context(), &req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": resp.Success,
		"message": "Incident resolved",
	})
}

// handleNotificationProxy обрабатывает запросы к Notification Service
func (h *Handler) handleNotificationProxy(w http.ResponseWriter, r *http.Request) {
	// Получаем информацию о пользователе из контекста
	userDataCtx, ok := r.Context().Value("user").(map[string]interface{})
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	tenantID, ok := userDataCtx["tenant_id"].(string)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant_id not found"), http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if strings.HasSuffix(r.URL.Path, "/channels") {
			h.handleGetNotificationChannels(w, r, tenantID)
		} else {
			h.handleSendNotification(w, r, tenantID)
		}
	case http.MethodPost:
		if strings.HasSuffix(r.URL.Path, "/channels") {
			h.handleCreateNotificationChannel(w, r, tenantID)
		} else {
			h.handleSendNotification(w, r, tenantID)
		}
	default:
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
	}
}

// handleSendNotification обрабатывает отправку уведомления
func (h *Handler) handleSendNotification(w http.ResponseWriter, r *http.Request, tenantID string) {
	var req notificationv1.SendNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// req.TenantId = tenantID

	resp, err := h.notificationClient.SendNotification(r.Context(), &req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": resp.Success,
		"results": resp.Results,
	})
}

// handleGetNotificationChannels обрабатывает получение каналов уведомлений
func (h *Handler) handleGetNotificationChannels(w http.ResponseWriter, r *http.Request, tenantID string) {
	req := &notificationv1.ListChannelsRequest{
		TenantId: tenantID,
	}

	resp, err := h.notificationClient.GetNotificationChannels(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"channels": resp.Channels,
		"total":    len(resp.Channels),
	})
}

// handleCreateNotificationChannel обрабатывает создание канала уведомлений
func (h *Handler) handleCreateNotificationChannel(w http.ResponseWriter, r *http.Request, tenantID string) {
	var req notificationv1.RegisterChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// req.TenantId = tenantID

	channel, err := h.notificationClient.RegisterChannel(r.Context(), &req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Notification channel created",
		"channel": channel,
	})
}

// extractIDFromPath извлекает ID из URL пути
func extractIDFromPath(path, resource string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == resource && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// writeError пишет ошибку в ответ
func (h *Handler) writeError(w http.ResponseWriter, err error, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   true,
		"message": err.Error(),
		"code":    statusCode,
	})
}

// handleError обрабатывает ошибки и конвертирует их в HTTP статусы
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	// Используем глобальные экземпляры ошибок для сравнения
	switch {
	case errors.Is(err, pkgErrors.New(pkgErrors.ErrValidation, "")):
		h.writeError(w, err, http.StatusBadRequest)
	case errors.Is(err, pkgErrors.New(pkgErrors.ErrUnauthorized, "")):
		h.writeError(w, err, http.StatusUnauthorized)
	case errors.Is(err, pkgErrors.New(pkgErrors.ErrForbidden, "")):
		h.writeError(w, err, http.StatusForbidden)
	case errors.Is(err, pkgErrors.New(pkgErrors.ErrNotFound, "")):
		h.writeError(w, err, http.StatusNotFound)
	case errors.Is(err, pkgErrors.New(pkgErrors.ErrConflict, "")):
		h.writeError(w, err, http.StatusConflict)
	default:
		h.writeError(w, err, http.StatusInternalServerError)
	}
}

// handleAuthHealthProxy проксирует health запрос к Auth Service
func (h *Handler) handleAuthHealthProxy(w http.ResponseWriter, r *http.Request) {
	// Создаем HTTP клиент
	client := &http.Client{Timeout: 5 * time.Second}

	// Формируем URL для Auth Service HTTP health endpoint
	// Auth Service работает на gRPC (50051), но имеет HTTP health endpoint на порту 51051
	authURL := "http://auth-service:51051/health"

	// DEBUG: логируем URL для отладки
	h.logger.Info("DEBUG: Auth health URL", logger.String("url", authURL))

	// Создаем новый запрос
	req, err := http.NewRequestWithContext(r.Context(), "GET", authURL, nil)
	if err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}

	// Отправляем запрос
	resp, err := client.Do(req)
	if err != nil {
		h.writeError(w, err, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Копируем заголовки
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Копируем статус
	w.WriteHeader(resp.StatusCode)

	// Копируем тело ответа
	_, err = h.copyResponse(w, resp.Body)
	if err != nil {
		h.logger.Error("failed to copy response", logger.Error(err))
	}
}

// handleSchedulerHealthProxy проксирует health запрос к Scheduler Service
func (h *Handler) handleSchedulerHealthProxy(w http.ResponseWriter, r *http.Request) {
	// Создаем HTTP клиент
	client := &http.Client{Timeout: 5 * time.Second}

	// Попытка: сначала взять адрес из окружения (например "scheduler-service:50052")
	addr := os.Getenv("SCHEDULER_SERVICE_ADDR")
	if addr == "" {
		addr = "scheduler-service:50052"
	}

	// Парсим host:port и пробуем health endpoint на порту +1000 (как у других сервисов)
	host := addr
	port := 0
	if parts := strings.Split(addr, ":"); len(parts) >= 2 {
		host = strings.Join(parts[:len(parts)-1], ":")
		if p, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			port = p
		}
	}

	tryURLs := []string{}
	if port != 0 {
		tryURLs = append(tryURLs, fmt.Sprintf("http://%s:%d/health", host, port+1000)) // preferred HTTP health port
		tryURLs = append(tryURLs, fmt.Sprintf("http://%s:%d/health", host, port))      // fallback to same port
	} else {
		tryURLs = append(tryURLs, fmt.Sprintf("http://%s/health", host))
	}

	var resp *http.Response
	var err error
	for _, u := range tryURLs {
		req, reqErr := http.NewRequestWithContext(r.Context(), "GET", u, nil)
		if reqErr != nil {
			err = reqErr
			continue
		}

		resp, err = client.Do(req)
		if err != nil {
			// пробуем следующий URL
			continue
		}
		// Успешно получили ответ — выходим из цикла
		break
	}

	if err != nil {
		h.writeError(w, err, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Копируем заголовки
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Копируем статус
	w.WriteHeader(resp.StatusCode)

	// Копируем тело ответа
	_, err = h.copyResponse(w, resp.Body)
	if err != nil {
		h.logger.Error("failed to copy response", logger.Error(err))
	}
}

// handleCoreHealthProxy проксирует health запрос к Core Service
func (h *Handler) handleCoreHealthProxy(w http.ResponseWriter, r *http.Request) {
	// Создаем HTTP клиент
	client := &http.Client{Timeout: 5 * time.Second}

	// Формируем URL для Core Service HTTP health endpoint
	// Core Service работает на gRPC, но имеет HTTP health endpoint на порту 51054
	coreURL := "http://core-service:51054/health"

	// Создаем новый запрос
	req, err := http.NewRequestWithContext(r.Context(), "GET", coreURL, nil)
	if err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}

	// Отправляем запрос
	resp, err := client.Do(req)
	if err != nil {
		h.writeError(w, err, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Копируем заголовки
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Копируем статус
	w.WriteHeader(resp.StatusCode)

	// Копируем тело ответа
	_, err = h.copyResponse(w, resp.Body)
	if err != nil {
		h.logger.Error("failed to copy response", logger.Error(err))
	}
}

// handleAPIKeys обрабатывает запросы для API ключей
func (h *Handler) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	// Реальная интеграция: проксируем создание API ключей в Auth Service (HTTP).
	// GET пока не поддерживаем (в чек-листе ожидается 405), POST создаёт ключи в БД.

	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	// Дергаем auth-service напрямую; базовый URL уже известен в HTTPAuthClient,
	// но здесь нам нужно проксирование без адаптера, поэтому берём env.
	authBaseURL := os.Getenv("AUTH_SERVICE_ADDR")
	if authBaseURL == "" {
		authBaseURL = "http://auth-service:51051"
	}

	targetURL := fmt.Sprintf("%s/api/v1/auth/api-keys", strings.TrimRight(authBaseURL, "/"))

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "failed to read request body"), http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "failed to create request"), http.StatusBadGateway)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrInternal, "failed to call auth service"), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = h.copyResponse(w, resp.Body)
}

// handleSchedulerChecks обрабатывает запросы для проверок
func (h *Handler) handleSchedulerChecks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		// GET запрос - получение списка проверок через Scheduler Service
		h.logger.Info("Getting checks list via Scheduler Service")

		// Получаем tenant_id из контекста
		userInfo := r.Context().Value("user").(map[string]interface{})
		tenantID, _ := userInfo["tenant_id"].(string)

		// Создаем запрос для получения списка проверок
		req := &schedulerv1.ListChecksRequest{
			TenantId: tenantID,
			PageSize: 20,
		}

		response, err := h.schedulerClient.ListChecks(r.Context(), req)
		if err != nil {
			h.logger.Error("Error getting checks list", logger.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method == http.MethodPost {
		// Создание проверки через Scheduler Service
		h.logger.Info("Creating check via Scheduler Service")

		// Парсим тело запроса
		var createReq struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Target   string `json:"target"`
			URL      string `json:"url"`
			Interval int64  `json:"interval"`
			Timeout  int64  `json:"timeout"`
			Enabled  bool   `json:"enabled"`
		}

		if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
			h.logger.Error("Error parsing request body", logger.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
			return
		}

		// Валидация обязательных полей
		target := createReq.Target
		if target == "" && createReq.URL != "" {
			target = createReq.URL
		}
		if createReq.Name == "" || createReq.Type == "" || target == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "name, type, and target/url are required"})
			return
		}

		// Создаем запрос для Scheduler Service
		req := &schedulerv1.CreateCheckRequest{
			Name:     createReq.Name,
			Type:     createReq.Type,
			Target:   target,
			Interval: int32(createReq.Interval),
			Timeout:  int32(createReq.Timeout),
		}

		// Получаем tenant_id из контекста (из токена)
		if userInfo := r.Context().Value("user"); userInfo != nil {
			if userMap, ok := userInfo.(map[string]interface{}); ok {
				if tenantID, ok := userMap["tenant_id"].(string); ok {
					req.TenantId = tenantID
					h.logger.Info("tenant_id extracted from context", logger.String("tenant_id", tenantID))
				} else {
					h.logger.Warn("tenant_id not found in user context", logger.Any("user_context", userMap))
				}
			} else {
				h.logger.Warn("user context is not map[string]interface{}", logger.Any("user_info", userInfo))
			}
		} else {
			h.logger.Warn("user context is nil")
		}

		response, err := h.schedulerClient.CreateCheck(r.Context(), req)
		if err != nil {
			h.logger.Error("Error creating check", logger.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method == http.MethodPut {
		// Обновление проверки через Scheduler Service
		checkID := strings.TrimPrefix(r.URL.Path, "/api/v1/checks/")
		if checkID == "" || checkID == r.URL.Path {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "check ID required"})
			return
		}

		h.logger.Info("Updating check via Scheduler Service", logger.String("check_id", checkID))

		// ✅ РЕАЛИЗОВАНО: Парсинг request body
		var updateData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			h.logger.Error("Error parsing request body", logger.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON in request body"})
			return
		}

		// Создаем запрос с данными из body
		req := &schedulerv1.UpdateCheckRequest{
			CheckId: checkID,
		}

		// Заполняем поля из request body если они есть
		if name, ok := updateData["name"].(string); ok {
			req.Name = name
		}
		if description, ok := updateData["description"].(string); ok {
			req.Description = description
		}
		if checkType, ok := updateData["type"].(string); ok {
			req.Type = checkType
		}
		if target, ok := updateData["target"].(string); ok {
			req.Target = target
		}
		if interval, ok := updateData["interval"].(float64); ok {
			req.Interval = int32(interval)
		}
		if timeout, ok := updateData["timeout"].(float64); ok {
			req.Timeout = int32(timeout)
		}
		if status, ok := updateData["status"].(string); ok {
			req.Status = status
		}
		if config, ok := updateData["config"].(map[string]interface{}); ok {
			req.Config = make(map[string]string)
			for k, v := range config {
				req.Config[k] = fmt.Sprintf("%v", v)
			}
		}
		response, err := h.schedulerClient.UpdateCheck(r.Context(), req)
		if err != nil {
			h.logger.Error("Error updating check", logger.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	if r.Method == http.MethodDelete {
		// Удаление проверки через Scheduler Service
		checkID := strings.TrimPrefix(r.URL.Path, "/api/v1/checks/")
		if checkID == "" || checkID == r.URL.Path {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "check ID required"})
			return
		}

		h.logger.Info("Deleting check via Scheduler Service", logger.String("check_id", checkID))
		req := &schedulerv1.DeleteCheckRequest{
			CheckId: checkID,
		}
		_, err := h.schedulerClient.DeleteCheck(r.Context(), req)
		if err != nil {
			h.logger.Error("Error deleting check", logger.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Для других методов
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
}

// handleSchedulerCheckByID обрабатывает запросы для конкретной проверки
func (h *Handler) handleSchedulerCheckByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	// Извлекаем ID из URL
	checkID := strings.TrimPrefix(r.URL.Path, "/api/v1/checks/")
	if checkID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "check ID required"})
		return
	}

	h.logger.Info("Getting check via Scheduler Service", logger.String("check_id", checkID))
	req := &schedulerv1.GetCheckRequest{
		CheckId: checkID,
	}
	response, err := h.schedulerClient.GetCheck(r.Context(), req)
	if err != nil {
		h.logger.Error("Error getting check", logger.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// copyResponse копирует тело ответа
func (h *Handler) copyResponse(dst http.ResponseWriter, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// handleIncidents обрабатывает запросы к инцидентам
func (h *Handler) handleIncidents(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Handling incidents request",
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	// Временно возвращаем mock ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"incidents": []interface{}{},
		"total":     0,
		"page":      1,
		"page_size": 20,
	})
}

// handleConfig обрабатывает запросы к конфигурации
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Handling config request",
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	// Временно возвращаем mock ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"config": map[string]string{
			"version":     "1.0.0",
			"environment": "dev",
		},
	})
}
