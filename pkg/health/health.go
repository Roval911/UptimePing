package health

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthChecker интерфейс для проверки здоровья сервиса
type HealthChecker interface {
	Check() *HealthStatus
}

// HealthStatus представляет статус здоровья сервиса
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]Status `json:"services,omitempty"`
	Version   string            `json:"version,omitempty"`
}

// Status представляет статус сервиса
type Status struct {
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

// SimpleHealthChecker простая реализация HealthChecker
type SimpleHealthChecker struct {
	version string
}

// NewSimpleHealthChecker создает новый SimpleHealthChecker
func NewSimpleHealthChecker(version string) *SimpleHealthChecker {
	return &SimpleHealthChecker{version: version}
}

// Check проверяет здоровье сервиса
func (s *SimpleHealthChecker) Check() *HealthStatus {
	return &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   s.version,
	}
}

// Handler создает HTTP обработчик для health check эндпоинта
func Handler(checker HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := checker.Check()
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		
		// Отправляем JSON ответ
		json.NewEncoder(w).Encode(status)
	}
}

// ReadyHandler создает HTTP обработчик для ready check эндпоинта
// Возвращает 200 если сервис готов принимать трафик
func ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		
		response := map[string]string{
			"status": "ready",
		}
		json.NewEncoder(w).Encode(response)
	}
}

// LiveHandler создает HTTP обработчик для live check эндпоинта
// Возвращает 200 если сервис жив
func LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		
		response := map[string]string{
			"status": "alive",
		}
		json.NewEncoder(w).Encode(response)
	}
}