package http

import (
	"context"
	"encoding/json"
	"net/http"

	"UptimePingPlatform/pkg/errors"
)

// Handler структура для управления HTTP обработчиками
type Handler struct {
	mux           *http.ServeMux
	authService   AuthService
	healthHandler HealthHandler
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
func NewHandler(authService AuthService, healthHandler HealthHandler) *Handler {
	h := &Handler{
		mux:           http.NewServeMux(),
		authService:   authService,
		healthHandler: healthHandler,
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
			h.writeError(w, errors.New(errors.ErrUnauthorized, "unauthorized"), http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// isAuthenticated проверяет аутентификацию запроса
// В реальной реализации будет проверять JWT или API ключ
func (h *Handler) isAuthenticated(r *http.Request) bool {
	// Пока возвращаем true для тестирования
	// В реальной реализации будет проверять JWT или API ключ
	return true
}

// handleLogin обрабатывает запросы на аутентификацию
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, errors.New(errors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, errors.New(errors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация входных данных
	if req.Email == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "email is required"), http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "password is required"), http.StatusBadRequest)
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
		h.writeError(w, errors.New(errors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		TenantName string `json:"tenant_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, errors.New(errors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация входных данных
	if req.Email == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "email is required"), http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "password is required"), http.StatusBadRequest)
		return
	}

	if req.TenantName == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "tenant name is required"), http.StatusBadRequest)
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
		h.writeError(w, errors.New(errors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, errors.New(errors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация
	if req.RefreshToken == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "refresh token is required"), http.StatusBadRequest)
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
		h.writeError(w, errors.New(errors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Декодирование запроса
	var req struct {
		UserID  string `json:"user_id"`
		TokenID string `json:"token_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, errors.New(errors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация
	if req.UserID == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "user_id is required"), http.StatusBadRequest)
		return
	}

	if req.TokenID == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "token_id is required"), http.StatusBadRequest)
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
		h.writeError(w, errors.New(errors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Здесь будет реализация прокси к Scheduler Service
	// Пока возвращаем заглушку
	if r.Method == http.MethodGet {
		// GET /api/v1/checks - получение списка проверок
		response := map[string]interface{}{
			"checks": []interface{}{},
			"total":  0,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	} else {
		// POST /api/v1/checks - создание новой проверки
		response := map[string]interface{}{
			"success": true,
			"message": "Check created",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

// handleForgeProxy проксирует запросы к Forge Service
func (h *Handler) handleForgeProxy(w http.ResponseWriter, r *http.Request) {
	// Проверка метода
	if r.Method != http.MethodPost {
		h.writeError(w, errors.New(errors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Проверка аутентификации
	if !h.isAuthenticated(r) {
		h.writeError(w, errors.New(errors.ErrUnauthorized, "unauthorized"), http.StatusUnauthorized)
		return
	}

	// Прокси запроса к Forge Service
	// Пока возвращаем заглушку
	response := map[string]interface{}{
		"success": true,
		"message": "Configuration generated",
		"config":  "dummy config content",
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
	case errors.Is(err, errors.ErrValidationInstance):
		h.writeError(w, err, http.StatusBadRequest)
	case errors.Is(err, errors.ErrUnauthorizedInstance):
		h.writeError(w, err, http.StatusUnauthorized)
	case errors.Is(err, errors.ErrForbiddenInstance):
		h.writeError(w, err, http.StatusForbidden)
	case errors.Is(err, errors.ErrNotFoundInstance):
		h.writeError(w, err, http.StatusNotFound)
	case errors.Is(err, errors.ErrConflictInstance):
		h.writeError(w, err, http.StatusConflict)
	default:
		h.writeError(w, err, http.StatusInternalServerError)
	}
}
