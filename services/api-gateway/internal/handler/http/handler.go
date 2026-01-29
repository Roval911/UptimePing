package http

import (
	forgev1 "UptimePingPlatform/proto/api/forge/v1"
	incidentv1 "UptimePingPlatform/proto/api/incident/v1"
	metricsv1 "UptimePingPlatform/proto/api/metrics/v1"
	notificationv1 "UptimePingPlatform/proto/api/notification/v1"
	schedulerv1 "UptimePingPlatform/proto/api/scheduler/v1"
	corev1 "UptimePingPlatform/proto/api/core/v1"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkgErrors "UptimePingPlatform/pkg/errors"
	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/validation"
	
	"UptimePingPlatform/services/api-gateway/internal/client"
)

// ForgeServiceClient интерфейс для Forge Service клиента
type ForgeServiceClient interface {
	GenerateConfig(ctx context.Context, protoContent string, options *forgev1.ConfigOptions) (*forgev1.GenerateConfigResponse, error)
	ParseProto(ctx context.Context, protoContent, fileName string) (*forgev1.ParseProtoResponse, error)
	GenerateCode(ctx context.Context, protoContent string, options *forgev1.CodeOptions) (*forgev1.GenerateCodeResponse, error)
	ValidateProto(ctx context.Context, protoContent string) (*forgev1.ValidateProtoResponse, error)
	Close() error
}

// SchedulerServiceClient интерфейс для Scheduler Service клиента
type SchedulerServiceClient interface {
	ListChecks(ctx context.Context, req *schedulerv1.ListChecksRequest) (*schedulerv1.ListChecksResponse, error)
	CreateCheck(ctx context.Context, req *schedulerv1.CreateCheckRequest) (*schedulerv1.Check, error)
	GetCheck(ctx context.Context, req *schedulerv1.GetCheckRequest) (*schedulerv1.Check, error)
	UpdateCheck(ctx context.Context, req *schedulerv1.UpdateCheckRequest) (*schedulerv1.Check, error)
	DeleteCheck(ctx context.Context, req *schedulerv1.DeleteCheckRequest) (*schedulerv1.DeleteCheckResponse, error)
	ScheduleCheck(ctx context.Context, req *schedulerv1.ScheduleCheckRequest) (*schedulerv1.Schedule, error)
	UnscheduleCheck(ctx context.Context, req *schedulerv1.UnscheduleCheckRequest) (*schedulerv1.UnscheduleCheckResponse, error)
	GetSchedule(ctx context.Context, req *schedulerv1.GetScheduleRequest) (*schedulerv1.Schedule, error)
	ListSchedules(ctx context.Context, req *schedulerv1.ListSchedulesRequest) (*schedulerv1.ListSchedulesResponse, error)
	Close() error
}

// MetricsServiceClient интерфейс для Metrics Service клиента
type MetricsServiceClient interface {
	CollectMetrics(ctx context.Context, req *metricsv1.CollectMetricsRequest) (*metricsv1.CollectMetricsResponse, error)
	GetMetrics(ctx context.Context, req *metricsv1.GetMetricsRequest) (*metricsv1.GetMetricsResponse, error)
	Close() error
}

// IncidentServiceClient интерфейс для Incident Service клиента
type IncidentServiceClient interface {
	CreateIncident(ctx context.Context, req *incidentv1.CreateIncidentRequest) (*incidentv1.Incident, error)
	GetIncident(ctx context.Context, req *incidentv1.GetIncidentRequest) (*incidentv1.Incident, error)
	ListIncidents(ctx context.Context, req *incidentv1.ListIncidentsRequest) (*incidentv1.ListIncidentsResponse, error)
	ResolveIncident(ctx context.Context, req *incidentv1.ResolveIncidentRequest) (*incidentv1.ResolveIncidentResponse, error)
	Close() error
}

// NotificationServiceClient интерфейс для Notification Service клиента
type NotificationServiceClient interface {
	SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error)
	GetNotificationChannels(ctx context.Context, req *notificationv1.ListChannelsRequest) (*notificationv1.ListChannelsResponse, error)
	RegisterChannel(ctx context.Context, req *notificationv1.RegisterChannelRequest) (*notificationv1.Channel, error)
	Close() error
}

// ConfigServiceClient интерфейс для Config Service клиента
type ConfigServiceClient interface {
	GetConfig(ctx context.Context) *config.Config
	UpdateConfig(ctx context.Context, newConfig *config.Config) error
	Close() error
}

// CoreServiceClient интерфейс для Core Service клиента
type CoreServiceClient interface {
	ExecuteCheck(ctx context.Context, req *corev1.ExecuteCheckRequest) (*corev1.CheckResult, error)
	GetCheckStatus(ctx context.Context, req *corev1.GetCheckStatusRequest) (*corev1.CheckStatusResponse, error)
	GetCheckHistory(ctx context.Context, req *corev1.GetCheckHistoryRequest) (*corev1.GetCheckHistoryResponse, error)
	Close() error
}

// Handler структура для управления HTTP обработчиками
type Handler struct {
	mux                   *http.ServeMux
	authService           *client.GRPCAuthClient
	healthHandler         HealthHandler
	schedulerClient       client.SchedulerClient
	coreClient            client.CoreClient
	metricsClient         client.MetricsClient
	incidentClient        client.IncidentClient
	notificationClient    client.NotificationClient
	configClient          client.ConfigClient
	forgeClient            client.GRPCForgeClient
	baseHandler           *grpcBase.BaseHandler
	validator             *validation.Validator
	logger                logger.Logger
}

// UserInfo содержит информацию о пользователе
type UserInfo struct {
	UserID       string   `json:"user_id"`
	TenantID     string   `json:"tenant_id"`
	Email        string   `json:"email"`
	Roles        []string `json:"roles"`
	Permissions  []string `json:"permissions"`
	ExpiresAt    int64    `json:"expires_at"`
}

// TokenPair структура для хранения пары токенов
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// HealthHandler интерфейс для health check обработчика
type HealthHandler interface {
	HealthCheck(w http.ResponseWriter, r *http.Request)
	ReadyCheck(w http.ResponseWriter, r *http.Request)
	LiveCheck(w http.ResponseWriter, r *http.Request)
}

// NewHandler создает новый экземпляр Handler
func NewHandler(authService *client.GRPCAuthClient, healthHandler HealthHandler, schedulerClient client.SchedulerClient, coreClient client.CoreClient, metricsClient client.MetricsClient, incidentClient client.IncidentClient, notificationClient client.NotificationClient, configClient client.ConfigClient, forgeClient client.GRPCForgeClient, logger logger.Logger) *Handler {
	h := &Handler{
		mux:                   http.NewServeMux(),
		authService:           authService,
		healthHandler:         healthHandler,
		schedulerClient:       schedulerClient,
		coreClient:            coreClient,
		metricsClient:         metricsClient,
		incidentClient:        incidentClient,
		notificationClient:    notificationClient,
		configClient:          configClient,
		forgeClient:            forgeClient,
		baseHandler:           grpcBase.NewBaseHandler(logger),
		validator:             validation.NewValidator(),
	}

	// Настройка роутинга
	h.setupRoutes()

	return h
}

// ServeHTTP реализует интерфейс http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// setupRoutes настраивает маршруты для приложения
func (h *Handler) setupRoutes() {
	// Публичные роуты
	h.mux.HandleFunc("/api/v1/auth/login", h.handleLogin)
	h.mux.HandleFunc("/api/v1/auth/register", h.handleRegister)
	h.mux.HandleFunc("/api/v1/auth/refresh", h.handleRefreshToken)
	h.mux.HandleFunc("/api/v1/auth/logout", h.handleLogout)
	
	// API ключи (потребуют аутентификацию)
	h.mux.HandleFunc("/api/v1/auth/api-keys", h.handleAPIKeys)
	
	// Scheduler роуты
	h.mux.HandleFunc("/api/v1/scheduler/checks", h.handleSchedulerChecks)

	// Health check роуты
	h.mux.HandleFunc("/health", h.healthHandler.HealthCheck)
	h.mux.HandleFunc("/ready", h.healthHandler.ReadyCheck)
	h.mux.HandleFunc("/live", h.healthHandler.LiveCheck)

	// Auth Service health endpoints (для тестирования)
	h.mux.HandleFunc("/api/v1/auth/health", h.handleAuthHealthProxy)
	h.mux.HandleFunc("/api/v1/scheduler/health", h.handleSchedulerHealthProxy)
	h.mux.HandleFunc("/api/v1/core/health", h.handleCoreHealthProxy)

	// Защищенные роуты
	h.mux.HandleFunc("/api/v1/checks", h.handleProtected(h.handleChecksProxy))
	h.mux.HandleFunc("/api/v1/checks/", h.handleProtected(h.handleChecksProxy))

	// Расписания проверок
	h.mux.HandleFunc("/api/v1/schedules", h.handleProtected(h.handleScheduleProxy))
	h.mux.HandleFunc("/api/v1/schedules/", h.handleProtected(h.handleScheduleProxy))

	// Core Service операции
	h.mux.HandleFunc("/api/v1/core", h.handleProtected(h.handleCoreProxy))
	h.mux.HandleFunc("/api/v1/core/", h.handleProtected(h.handleCoreProxy))

	// Metrics Service
	h.mux.HandleFunc("/api/v1/metrics", h.handleProtected(h.handleMetricsProxy))
	h.mux.HandleFunc("/api/v1/metrics/collect", h.handleProtected(h.handleMetricsProxy))

	// Incident Service
	h.mux.HandleFunc("/api/v1/incidents", h.handleProtected(h.handleIncidentProxy))
	h.mux.HandleFunc("/api/v1/incidents/", h.handleProtected(h.handleIncidentProxy))

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
		// Проверка аутентификации
		userInfo, err := h.authenticateRequest(r)
		if err != nil {
			h.writeError(w, err, http.StatusUnauthorized)
			return
		}

		// Добавляем информацию о пользователе в контекст
		ctx := context.WithValue(r.Context(), "user_info", userInfo)
		ctx = context.WithValue(ctx, "tenant_id", userInfo.TenantID)
		ctx = context.WithValue(ctx, "user_id", userInfo.UserID)
		ctx = context.WithValue(ctx, "user_roles", userInfo.Roles)
		ctx = context.WithValue(ctx, "user_permissions", userInfo.Permissions)

		// Проверяем права доступа к ресурсу
		if !h.checkResourceAccess(r, userInfo) {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrForbidden, "insufficient permissions"), http.StatusForbidden)
			return
		}

		next(w, r.WithContext(ctx))
	}
}

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
	tokenClaims, err := h.authService.ValidateToken(ctx, token)
	if err != nil {
		return nil, pkgErrors.Wrap(err, pkgErrors.ErrUnauthorized, "token validation failed")
	}

	// Конвертируем TokenClaims в UserInfo
	userInfo := &UserInfo{
		UserID:      tokenClaims.UserID,
		TenantID:    tokenClaims.TenantID,
		Email:       tokenClaims.Email,
		Roles:       tokenClaims.Roles,
		Permissions: tokenClaims.Permissions,
		ExpiresAt:   tokenClaims.ExpiresAt,
	}

	// Проверяем срок действия токена
	if time.Now().Unix() > userInfo.ExpiresAt {
		return nil, pkgErrors.New(pkgErrors.ErrUnauthorized, "token expired")
	}

	return userInfo, nil
}

// authenticateWithAPIKey аутентифицирует через API ключ
func (h *Handler) authenticateWithAPIKey(apiKey string) (*UserInfo, error) {
	// Базовая валидация API ключа
	if len(apiKey) < 16 {
		return nil, pkgErrors.New(pkgErrors.ErrUnauthorized, "invalid api key length")
	}

	// TODO: Реализовать валидацию API ключа через Auth Service
	// Сейчас возвращаем базового пользователя для API ключа
	return &UserInfo{
		UserID:      "api-key-user",
		TenantID:    "default-tenant",
		Email:       "api-key@system",
		Roles:       []string{"api"},
		Permissions: []string{"read", "write"},
		ExpiresAt:   time.Now().Add(24 * time.Hour).Unix(),
	}, nil
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

		// В реальной реализации здесь будет проверка подписи и экспирации
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
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
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
	requiredFields := map[string]interface{}{
		"email":      req.Email,
		"password":   req.Password,
		"tenant_name": req.TenantName,
	}

	if err := h.validator.ValidateRequiredFields(requiredFields, map[string]string{
		"email":      "Email address",
		"password":   "Password",
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
		UserID  string `json:"user_id"`
		TokenID string `json:"token_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация с использованием pkg/validation
	requiredFields := map[string]interface{}{
		"user_id":  req.UserID,
		"token_id": req.TokenID,
	}

	if err := h.validator.ValidateRequiredFields(requiredFields, map[string]string{
		"user_id":  "User ID",
		"token_id": "Token ID",
	}); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "validation failed"), http.StatusBadRequest)
		return
	}

	// Валидация формата UUID
	if err := h.validator.ValidateUUID(req.UserID, "user_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid user_id format"), http.StatusBadRequest)
		return
	}

	if err := h.validator.ValidateUUID(req.TokenID, "token_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid token_id format"), http.StatusBadRequest)
		return
	}

	// Вызов сервиса
	ctx := r.Context()
	err := h.authService.Logout(ctx, req.UserID, req.TokenID)
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
	// Валидация UUID
	if err := h.validator.ValidateUUID(checkID, "check_id"); err != nil {
		h.writeError(w, pkgErrors.Wrap(err, pkgErrors.ErrValidation, "invalid check ID format"), http.StatusBadRequest)
		return
	}

	req := &schedulerv1.GetCheckRequest{
		CheckId: checkID,
	}

	check, err := h.schedulerClient.GetCheck(r.Context(), req)
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
		"success": true,
		"message": "Check scheduled",
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
		"success": true,
		"schedule": schedule,
	})
}

// handleListSchedules обрабатывает получение списка расписаний
func (h *Handler) handleListSchedules(w http.ResponseWriter, r *http.Request, tenantID string) {
	req := &schedulerv1.ListSchedulesRequest{
		// TODO: Добавить фильтрацию по tenant_id когда будет поддерживаться в proto
	}

	resp, err := h.schedulerClient.ListSchedules(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
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
		"success":     result.Success,
		"execution_id": result.ExecutionId,
		"duration_ms": result.DurationMs,
		"status_code": result.StatusCode,
		"error":       result.Error,
		"checked_at":  result.CheckedAt,
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
		"check_id":           status.CheckId,
		"is_healthy":         status.IsHealthy,
		"response_time_ms":   status.ResponseTimeMs,
		"last_checked_at":   status.LastCheckedAt,
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
		TenantId: tenantID,
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
	
	if r.Method == http.MethodPost {
		// Mock ответ для создания проверки
		response := map[string]interface{}{
			"id":         "test-check-id",
			"name":       "Test Check",
			"url":        "https://httpbin.org/status/200",
			"check_type": "http",
			"interval":   60,
			"timeout":    30,
			"status":     "active",
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Для других методов
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
}

// copyResponse копирует тело ответа
func (h *Handler) copyResponse(dst http.ResponseWriter, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
