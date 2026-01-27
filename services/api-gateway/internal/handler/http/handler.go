package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	forgev1 "UptimePingPlatform/gen/go/proto/api/forge/v1"
	schedulerv1 "UptimePingPlatform/gen/go/proto/api/scheduler/v1"
	pkgErrors "UptimePingPlatform/pkg/errors"
	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
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
	Close() error
}

// Handler структура для управления HTTP обработчиками
type Handler struct {
	mux             *http.ServeMux
	authService     AuthService
	healthHandler   HealthHandler
	schedulerClient SchedulerServiceClient
	forgeClient     ForgeServiceClient
	baseHandler     *grpcBase.BaseHandler
	validator       *validation.Validator
}

// AuthService интерфейс для сервиса аутентификации
type AuthService interface {
	Login(ctx context.Context, email, password string) (*TokenPair, error)
	Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, userID, tokenID string) error
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
func NewHandler(authService AuthService, healthHandler HealthHandler, schedulerClient SchedulerServiceClient, forgeClient ForgeServiceClient, logger logger.Logger) *Handler {
	h := &Handler{
		mux:             http.NewServeMux(),
		authService:     authService,
		healthHandler:   healthHandler,
		schedulerClient: schedulerClient,
		forgeClient:     forgeClient,
		baseHandler:     grpcBase.NewBaseHandler(logger),
		validator:       validation.NewValidator(),
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

	// Health check роуты
	h.mux.HandleFunc("/health", h.healthHandler.HealthCheck)
	h.mux.HandleFunc("/ready", h.healthHandler.ReadyCheck)
	h.mux.HandleFunc("/live", h.healthHandler.LiveCheck)

	// Защищенные роуты
	h.mux.HandleFunc("/api/v1/checks", h.handleProtected(h.handleChecksProxy))

	// Добавляем роуты Forge Service
	h.mux.HandleFunc("/api/v1/forge/generate", h.handleProtected(h.handleForgeProxy))
}

// handleProtected оборачивает обработчик, требующий аутентификации
func (h *Handler) handleProtected(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверка аутентификации
		if !h.isAuthenticated(r) {
			h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "unauthorized"), http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// TODO isAuthenticated проверяет аутентификацию запроса
// В реальной реализации будет проверять JWT или API ключ
func (h *Handler) isAuthenticated(r *http.Request) bool {
	// Пока возвращаем true для тестирования
	// В реальной реализации будет проверять JWT или API ключ
	return true
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
	tokenPair, err := h.authService.Login(ctx, req.Email, req.Password)
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

	// Валидация входных данных
	if req.Email == "" {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "email is required"), http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "password is required"), http.StatusBadRequest)
		return
	}

	if req.TenantName == "" {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "tenant name is required"), http.StatusBadRequest)
		return
	}

	// Вызов сервиса аутентификации
	ctx := r.Context()
	tokenPair, err := h.authService.Register(ctx, req.Email, req.Password, req.TenantName)
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

	// Валидация
	if req.RefreshToken == "" {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "refresh token is required"), http.StatusBadRequest)
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

	// Валидация
	if req.UserID == "" {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "user_id is required"), http.StatusBadRequest)
		return
	}

	if req.TokenID == "" {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "token_id is required"), http.StatusBadRequest)
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
	// Проверка метода
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Получаем tenant_id из контекста (установлен в AuthMiddleware)
	tenantID, ok := r.Context().Value("tenant_id").(string)
	if !ok {
		h.writeError(w, pkgErrors.New(pkgErrors.ErrUnauthorized, "tenant not found"), http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodGet {
		// GET /api/v1/checks - получение списка проверок
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
	} else {
		// POST /api/v1/checks - создание новой проверки
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

// writeError записывает ошибку в ответ
func (h *Handler) writeError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errorResponse := map[string]interface{}{
		"error":   err.Error(),
		"status":  status,
		"success": false,
	}

	json.NewEncoder(w).Encode(errorResponse)
}

// handleError обрабатывает ошибки сервиса
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
