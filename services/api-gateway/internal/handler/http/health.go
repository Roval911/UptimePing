package http

import (
	"encoding/json"
	"net/http"
	"time"

	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
)

// healthHandlerImpl реализует интерфейс HealthHandler
type healthHandlerImpl struct {
	checker health.HealthChecker
	log     logger.Logger
}

// NewHealthHandler создает новый экземпляр HealthHandler
func NewHealthHandler(checker health.HealthChecker, log logger.Logger) HealthHandler {
	return &healthHandlerImpl{
		checker: checker,
		log:     log,
	}
}

// HealthCheck обрабатывает health check запросы
func (h *healthHandlerImpl) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.log.Warn("Invalid method for health check", logger.String("method", r.Method))
		h.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем статус здоровья
	status := h.checker.Check()

	h.log.Info("Health check completed",
		logger.String("status", status.Status),
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Формируем и отправляем ответ
	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.log.Error("Failed to encode health check response", logger.Error(err))
		h.writeError(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// ReadyCheck обрабатывает ready check запросы
func (h *healthHandlerImpl) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.log.Warn("Invalid method for ready check", logger.String("method", r.Method))
		h.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.log.Info("Ready check completed",
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	//TODO Пока возвращаем просто OK, в реальности может проверять готовность зависимостей
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
		h.log.Warn("Invalid method for live check", logger.String("method", r.Method))
		h.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.log.Info("Live check completed",
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	//TODO Пока возвращаем просто OK, в реальности может проверять живость сервиса
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "alive",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// writeError записывает ошибку в ответ
func (h *healthHandlerImpl) writeError(w http.ResponseWriter, message string, status int) {
	h.log.Error("Health check error",
		logger.String("message", message),
		logger.Int("status", status))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
