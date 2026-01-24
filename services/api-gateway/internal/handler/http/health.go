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

	// Проверяем готовность основных зависимостей
	status := "ready"
	statusCode := http.StatusOK

	// Можно добавить проверку подключения к базе данных, Redis и другим зависимостям
	// В реальной реализации это будет делегировано checker.Check()
	if h.checker != nil {
		healthStatus := h.checker.Check()
		if healthStatus.Status != "healthy" {
			status = "not ready"
			statusCode = http.StatusServiceUnavailable
		}
	}

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Формируем и отправляем ответ
	response := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if h.checker != nil {
		response["details"] = h.checker.Check().Services
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeError(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// LiveCheck обрабатывает live check запросы
func (h *healthHandlerImpl) LiveCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем, что сервис жив
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Формируем и отправляем ответ
	response := map[string]string{
		"status":    "alive",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeError(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// writeError записывает ошибку в ответ
func (h *healthHandlerImpl) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
