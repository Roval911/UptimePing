package http

import (
	"encoding/json"
	"net/http"
)

// Handler структура для управления HTTP обработчиками
type Handler struct {
	mux *http.ServeMux
}

// NewHandler создает новый экземпляр Handler
func NewHandler() *Handler {
	h := &Handler{
		mux: http.NewServeMux(),
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

	// Health check
	h.mux.HandleFunc("/health", h.handleHealth)

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
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// isAuthenticated проверяет аутентификацию запроса
// В реальной реализации будет проверять JWT или API ключ
func (h *Handler) isAuthenticated(r *http.Request) bool {
	// Пока возвращаем true для тестирования
	return true
}

// handleLogin обрабатывает запросы на аутентификацию
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Здесь будет реализация прокси к Auth Service
	// Пока возвращаем заглушку
	response := map[string]interface{}{
		"success": true,
		"message": "Login successful",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleRegister обрабатывает запросы на регистрацию
func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Здесь будет реализация прокси к Auth Service
	// Пока возвращаем заглушку
	response := map[string]interface{}{
		"success": true,
		"message": "Registration successful",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleHealth обрабатывает health check запросы
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleChecksProxy проксирует запросы к Scheduler Service
func (h *Handler) handleChecksProxy(w http.ResponseWriter, r *http.Request) {
	// Проверка метода
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Здесь будет реализация аутентификации
	// Пока возвращаем заглушку
	if r.Method == http.MethodGet {
		// GET /api/v1/checks - получение списка проверок
		response := map[string]interface{}{
			"checks": []interface{}{},
			"total": 0,
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверка аутентификации
	if !h.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Прокси запроса к Forge Service
	// Пока возвращаем заглушку
	response := map[string]interface{}{
		"success": true,
		"message": "Configuration generated",
		"config": "dummy config content",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}