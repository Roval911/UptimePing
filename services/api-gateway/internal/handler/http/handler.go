package http

import (
	corev1 "UptimePingPlatform/proto/api/core/v1"
	forgev1 "UptimePingPlatform/proto/api/forge/v1"
	incidentv1 "UptimePingPlatform/proto/api/incident/v1"
	metricsv1 "UptimePingPlatform/proto/api/metrics/v1"
	notificationv1 "UptimePingPlatform/proto/api/notification/v1"
	schedulerv1 "UptimePingPlatform/proto/api/scheduler/v1"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

	h.setupRoutes()
	return h
}

// ServeHTTP реализует интерфейс http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// setupRoutes настраивает маршруты для приложения
func (h *Handler) setupRoutes() {
	// Scheduler роуты для всех операций с проверками

	// Роут для /api/v1/checks (без слэша) - список проверок
	checksHandler := middleware.AuthMiddleware(h.authService, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info("DEBUG: Route /api/v1/checks matched!",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("full_url", r.URL.String()))

		switch r.Method {
		case http.MethodGet:
			h.logger.Info("Handling GET /api/v1/checks (list) with checks:read permission")
			// GET /api/v1/checks - требует checks:read
			middleware.PermissionMiddleware([]string{"checks:read"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleSchedulerChecks(w, r)
			})).ServeHTTP(w, r)
		case http.MethodPost:
			h.logger.Info("Handling POST /api/v1/checks (create) with checks:write permission")
			// POST /api/v1/checks - требует checks:write
			middleware.PermissionMiddleware([]string{"checks:write"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleSchedulerChecks(w, r)
			})).ServeHTTP(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		}
	}))
	h.mux.Handle("/api/v1/checks", checksHandler).Methods(http.MethodGet, http.MethodPost)

	// Роут для /api/v1/checks/{id} - операции с конкретными проверками
	checkByIDHandler := middleware.AuthMiddleware(h.authService, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		checkID := vars["id"]

		h.logger.Info("DEBUG: Route /api/v1/checks/{id} matched!",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("check_id", checkID),
			logger.String("full_url", r.URL.String()))

		switch r.Method {
		case http.MethodGet:
			h.logger.Info("Handling GET /api/v1/checks/{id} with checks:read permission")
			// GET /api/v1/checks/{id} - требует checks:read
			middleware.PermissionMiddleware([]string{"checks:read"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleSchedulerCheckByID(w, r)
			})).ServeHTTP(w, r)
		case http.MethodPut:
			h.logger.Info("Handling PUT /api/v1/checks/{id} with checks:write permission")
			// PUT /api/v1/checks/{id} - требует checks:write
			middleware.PermissionMiddleware([]string{"checks:write"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleSchedulerChecks(w, r)
			})).ServeHTTP(w, r)
		case http.MethodDelete:
			h.logger.Info("Handling DELETE /api/v1/checks/{id} with checks:write permission")
			// DELETE /api/v1/checks/{id} - требует checks:write
			middleware.PermissionMiddleware([]string{"checks:write"}, h.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.handleSchedulerChecks(w, r)
			})).ServeHTTP(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		}
	}))
	h.mux.Handle("/api/v1/checks/{id}", checkByIDHandler).Methods(http.MethodGet, http.MethodPut, http.MethodDelete)

	// Публичные роуты
	h.mux.HandleFunc("/api/v1/auth/login", h.handleLogin)
	h.mux.HandleFunc("/api/v1/auth/register", h.handleRegister)
	h.mux.HandleFunc("/api/v1/auth/refresh", h.handleRefreshToken)
	h.mux.HandleFunc("/api/v1/auth/logout", h.handleLogout)
	h.mux.HandleFunc("/api/v1/auth/validate", h.handleValidateToken)

	// API ключи (потребуют аутентификацию)
	h.mux.HandleFunc("/api/v1/auth/api-keys", h.handleAPIKeys)

	// Config роуты (требуют прав доступа)
	configHandler := middleware.PermissionMiddleware([]string{"config:read"}, h.logger)(http.HandlerFunc(h.handleConfig))
	h.mux.HandleFunc("/api/v1/config", configHandler.ServeHTTP).Methods(http.MethodGet)

	// Health check роуты
	h.mux.HandleFunc("/health", h.healthHandler.HealthCheck)
	h.mux.HandleFunc("/ready", h.healthHandler.ReadyCheck)
	h.mux.HandleFunc("/live", h.healthHandler.LiveCheck)

	// Auth Service health endpoints (для тестирования)
	h.mux.HandleFunc("/api/v1/auth/health", h.handleAuthHealthProxy)
	h.mux.HandleFunc("/api/v1/scheduler/health", h.handleSchedulerHealthProxy)
	h.mux.HandleFunc("/api/v1/core/health", h.handleCoreHealthProxy)

	// Расписания проверок
	h.mux.HandleFunc("/api/v1/schedules", h.handleProtected(h.handleScheduleProxy))
	h.mux.HandleFunc("/api/v1/schedules/", h.handleProtected(h.handleScheduleProxy))

	// Core Service операции
	h.mux.HandleFunc("/api/v1/core", h.handleProtected(h.handleCoreProxy))
	h.mux.HandleFunc("/api/v1/core/", h.handleProtected(h.handleCoreProxy))

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
}

// handleProtected оборачивает обработчик, требующий аутентификации
func (h *Handler) handleProtected(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info("DEBUG: handleProtected called",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("full_url", r.URL.String()))

		// Проверяем аутентификацию
		userInfo, err := h.authService.ValidateToken(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			h.logger.Error("Authentication failed",
				logger.Error(err),
				logger.String("path", r.URL.Path))

			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"code":    "403",
				"error":   "true",
				"message": "insufficient permissions",
			})
			return
		}

		// Добавляем информацию о пользователе в контекст
		ctx := context.WithValue(r.Context(), "user", userInfo)
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

		//TODO В реальной реализации здесь будет проверка подписи и экспирации
		// Сейчас проверяем только базовый формат
		h.baseHandler.LogOperationStart(ctx, "authentication", map[string]interface{}{
			"success": true,
			"method":  "jwt_bearer",
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
	userInfo, ok := r.Context().Value("user_info").(*UserInfo)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	// Извлекаем ID проверки из URL пути для операций с конкретной проверкой
	checkID := extractCheckIDFromPath(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		if checkID != "" {
			h.handleGetCheck(w, r, userInfo.TenantID, checkID)
		} else {
			h.handleListChecks(w, r, userInfo.TenantID)
		}
	case http.MethodPost:
		h.handleCreateCheck(w, r, userInfo.TenantID)
	case http.MethodPut:
		if checkID != "" {
			h.handleUpdateCheck(w, r, userInfo.TenantID, checkID)
		} else {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "check ID required"), http.StatusBadRequest)
		}
	case http.MethodDelete:
		if checkID != "" {
			h.handleDeleteCheck(w, r, userInfo.TenantID, checkID)
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
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "checks" && i+1 < len(parts) {
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
	var createReq schedulerv1.CreateCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Устанавливаем tenant_id из контекста
	createReq.TenantId = tenantID

	check, err := h.schedulerClient.CreateCheck(r.Context(), &createReq)
	if err != nil {
		h.handleError(w, err)
		return
	}

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

	check, err := h.schedulerClient.UpdateCheck(r.Context(), &updateReq)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Проверяем, что проверка принадлежит тенанту
	if check.TenantId != tenantID {
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
	userInfo, ok := r.Context().Value("user_info").(*UserInfo)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	// Извлекаем ID проверки из URL пути
	checkID := extractCheckIDFromPath(r.URL.Path)

	switch r.Method {
	case http.MethodPost:
		if checkID != "" {
			h.handleScheduleCheck(w, r, userInfo.TenantID, checkID)
		} else {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "check ID required"), http.StatusBadRequest)
		}
	case http.MethodDelete:
		if checkID != "" {
			h.handleUnscheduleCheck(w, r, userInfo.TenantID, checkID)
		} else {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "check ID required"), http.StatusBadRequest)
		}
	case http.MethodGet:
		if checkID != "" {
			h.handleGetSchedule(w, r, userInfo.TenantID, checkID)
		} else {
			h.handleListSchedules(w, r, userInfo.TenantID)
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
	userInfo, ok := r.Context().Value("user_info").(*UserInfo)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
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
		h.handleExecuteCheck(w, r, userInfo.TenantID, checkID)
	case http.MethodGet:
		if strings.HasSuffix(r.URL.Path, "/status") {
			h.handleGetCheckStatus(w, r, userInfo.TenantID, checkID)
		} else if strings.HasSuffix(r.URL.Path, "/history") {
			h.handleGetCheckHistory(w, r, userInfo.TenantID, checkID)
		} else {
			h.handleGetCheckStatus(w, r, userInfo.TenantID, checkID)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      result.Success,
		"execution_id": result.ExecutionId,
		"duration_ms":  result.DurationMs,
		"status_code":  result.StatusCode,
		"error":        result.Error,
		"checked_at":   result.CheckedAt,
	})
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
		"is_healthy":       status.IsHealthy,
		"response_time_ms": status.ResponseTimeMs,
		"last_checked_at":  status.LastCheckedAt,
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
	userInfo, ok := r.Context().Value("user_info").(*UserInfo)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetMetrics(w, r, userInfo.TenantID)
	case http.MethodPost:
		if strings.HasSuffix(r.URL.Path, "/collect") {
			h.handleCollectMetrics(w, r, userInfo.TenantID)
		} else {
			h.handleGetMetrics(w, r, userInfo.TenantID)
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

	req.TenantId = tenantID

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
	userInfo, ok := r.Context().Value("user_info").(*UserInfo)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	// Извлекаем ID инцидента из URL пути
	incidentID := extractIDFromPath(r.URL.Path, "incidents")

	switch r.Method {
	case http.MethodGet:
		if incidentID != "" {
			h.handleGetIncident(w, r, userInfo.TenantID, incidentID)
		} else {
			h.handleListIncidents(w, r, userInfo.TenantID)
		}
	case http.MethodPost:
		h.handleCreateIncident(w, r, userInfo.TenantID)
	case http.MethodPut:
		if incidentID != "" {
			h.handleResolveIncident(w, r, userInfo.TenantID, incidentID)
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

	req.TenantId = tenantID

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
	if incident.TenantId != tenantID {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrForbidden, "access denied"), http.StatusForbidden)
		return
	}

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
		TenantId: tenantID,
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
	userInfo, ok := r.Context().Value("user_info").(*UserInfo)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "user info not found"), http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if strings.HasSuffix(r.URL.Path, "/channels") {
			h.handleGetNotificationChannels(w, r, userInfo.TenantID)
		} else {
			h.handleSendNotification(w, r, userInfo.TenantID)
		}
	case http.MethodPost:
		if strings.HasSuffix(r.URL.Path, "/channels") {
			h.handleCreateNotificationChannel(w, r, userInfo.TenantID)
		} else {
			h.handleSendNotification(w, r, userInfo.TenantID)
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

	req.TenantId = tenantID

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

	req.TenantId = tenantID

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
	// Auth Service работает на gRPC, но имеет HTTP health endpoint на том же порту
	authURL := "http://auth-service:50051/health"

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

	// Формируем URL для Scheduler Service
	schedulerURL := "http://scheduler-service:50052/health"

	// Создаем новый запрос
	req, err := http.NewRequestWithContext(r.Context(), "GET", schedulerURL, nil)
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

// handleCoreHealthProxy проксирует health запрос к Core Service
func (h *Handler) handleCoreHealthProxy(w http.ResponseWriter, r *http.Request) {
	// Создаем HTTP клиент
	client := &http.Client{Timeout: 5 * time.Second}

	// Формируем URL для Core Service HTTP health endpoint
	// Core Service работает на gRPC, но имеет HTTP health endpoint на том же порту
	coreURL := "http://core-service:50054/health"

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
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodPost {
		// Mock ответ для создания API ключа
		response := map[string]interface{}{
			"id":     "test-api-key-id",
			"key":    "test-api-key",
			"secret": "test-api-secret",
			"name":   "Test API Key",
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Для других методов
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
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
		req := &schedulerv1.UpdateCheckRequest{
			CheckId: checkID,
		} // TODO: parse request body
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
