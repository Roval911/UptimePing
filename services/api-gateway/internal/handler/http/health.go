package http

import (
	"encoding/json"
	"net/http"
	"time"

	"UptimePingPlatform/pkg/health"
)

// healthHandlerImpl реализует интерфейс HealthHandler
type healthHandlerImpl struct {
	checker health.HealthChecker
}

// NewHealthHandler создает новый экземпляр HealthHandler
func NewHealthHandler(checker health.HealthChecker) HealthHandler {
	return &healthHandlerImpl{checker: checker}
}

// HealthCheck обрабатывает health check запросы
func (h *healthHandlerImpl) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем статус здоровья
	status := h.checker.Check()

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Формируем и отправляем ответ
	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.writeError(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// ReadyCheck обрабатывает ready check запросы
func (h *healthHandlerImpl) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Пока возвращаем просто OK, в реальности может проверять готовность зависимостей
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "ready",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// LiveCheck обрабатывает live check запросы
func (h *healthHandlerImpl) LiveCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Пока возвращаем просто OK, в реальности может проверять живость сервиса
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "alive",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// writeError записывает ошибку в ответ
func (h *healthHandlerImpl) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
