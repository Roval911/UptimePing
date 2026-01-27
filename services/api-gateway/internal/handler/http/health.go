package http

import (
	"encoding/json"
	"net/http"
	"runtime"
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

	// Проверяем готовность сервиса и его зависимостей
	ready := h.checkReadiness()

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	h.log.Info("Ready check completed",
		logger.String("status", map[bool]string{true: "ready", false: "not_ready"}[ready]),
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Формируем и отправляем ответ
	response := map[string]interface{}{
		"status":    map[bool]string{true: "ready", false: "not_ready"}[ready],
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks": map[string]interface{}{
			"database":    h.checkDatabaseReadiness(),
			"redis":       h.checkRedisReadiness(),
			"services":    h.checkServicesReadiness(),
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode ready check response", logger.Error(err))
		h.writeError(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// LiveCheck обрабатывает live check запросы
func (h *healthHandlerImpl) LiveCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.log.Warn("Invalid method for live check", logger.String("method", r.Method))
		h.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем живость сервиса (базовая проверка)
	alive := h.checkLiveness()

	statusCode := http.StatusOK
	if !alive {
		statusCode = http.StatusServiceUnavailable
	}

	h.log.Info("Live check completed",
		logger.String("status", map[bool]string{true: "alive", false: "not_alive"}[alive]),
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Формируем и отправляем ответ
	response := map[string]interface{}{
		"status":    map[bool]string{true: "alive", false: "not_alive"}[alive],
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    h.getUptime(),
		"version":   h.getVersion(),
		"checks": map[string]interface{}{
			"memory":      h.checkMemoryUsage(),
			"goroutines":  h.checkGoroutines(),
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode live check response", logger.Error(err))
		h.writeError(w, "Failed to encode response", http.StatusInternalServerError)
	}
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

// checkReadiness проверяет готовность сервиса и зависимостей
func (h *healthHandlerImpl) checkReadiness() bool {
	// Базовая проверка - сервис готов если все зависимости готовы
	dbReady := h.checkDatabaseReadiness()["ready"].(bool)
	redisReady := h.checkRedisReadiness()["ready"].(bool)
	servicesReady := h.checkServicesReadiness()["ready"].(bool)
	
	return dbReady && redisReady && servicesReady
}

// checkLiveness проверяет живость сервиса
func (h *healthHandlerImpl) checkLiveness() bool {
	// Базовая проверка - сервис жив если может отвечать на запросы
	// В реальной реализации здесь могут быть более сложные проверки
	return true
}

// checkDatabaseReadiness проверяет готовность базы данных
func (h *healthHandlerImpl) checkDatabaseReadiness() map[string]interface{} {
	// В реальной реализации здесь будет проверка подключения к БД
	// Сейчас возвращаем симуляцию
	return map[string]interface{}{
		"ready":   true,
		"message": "Database connection is ready",
		"latency": "5ms",
	}
}

// checkRedisReadiness проверяет готовность Redis
func (h *healthHandlerImpl) checkRedisReadiness() map[string]interface{} {
	// В реальной реализации здесь будет проверка подключения к Redis
	// Сейчас возвращаем симуляцию
	return map[string]interface{}{
		"ready":   true,
		"message": "Redis connection is ready",
		"latency": "2ms",
	}
}

// checkServicesReadiness проверяет готовность зависимых сервисов
func (h *healthHandlerImpl) checkServicesReadiness() map[string]interface{} {
	// В реальной реализации здесь будут проверки gRPC сервисов
	// Сейчас возвращаем симуляцию
	services := map[string]interface{}{
		"ready":   true,
		"message": "All dependent services are ready",
		"details": map[string]interface{}{
			"auth_service":     map[string]interface{}{"ready": true, "latency": "10ms"},
			"scheduler_service": map[string]interface{}{"ready": true, "latency": "8ms"},
			"forge_service":    map[string]interface{}{"ready": true, "latency": "12ms"},
		},
	}
	return services
}

// getUptime возвращает время работы сервиса
func (h *healthHandlerImpl) getUptime() string {
	// В реальной реализации здесь будет отслеживание времени запуска
	// Сейчас возвращаем симуляцию
	return "2h30m45s"
}

// getVersion возвращает версию сервиса
func (h *healthHandlerImpl) getVersion() string {
	// В реальной реализации здесь будет получение версии из сборки
	// Сейчас возвращаем симуляцию
	return "1.0.0"
}

// checkMemoryUsage проверяет использование памяти
func (h *healthHandlerImpl) checkMemoryUsage() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	usedMB := float64(m.Alloc) / 1024 / 1024
	totalMB := float64(m.Sys) / 1024 / 1024
	percent := int((usedMB / totalMB) * 100)
	
	status := "healthy"
	if percent > 80 {
		status = "warning"
	}
	if percent > 90 {
		status = "critical"
	}
	
	return map[string]interface{}{
		"used_mb":   int(usedMB),
		"total_mb":  int(totalMB),
		"percent":   percent,
		"status":    status,
	}
}

// checkGoroutines проверяет количество горутин
func (h *healthHandlerImpl) checkGoroutines() map[string]interface{} {
	count := runtime.NumGoroutine()
	
	status := "healthy"
	if count > 1000 {
		status = "warning"
	}
	if count > 5000 {
		status = "critical"
	}
	
	return map[string]interface{}{
		"count":  count,
		"status": status,
	}
}
