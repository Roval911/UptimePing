package http

import (
	"encoding/json"
	"net/http"
	"time"

	"UptimePingPlatform/services/metrics-service/internal/collector"
	pkglogger "UptimePingPlatform/pkg/logger"
)

// HTTPHandler обрабатывает HTTP запросы для Metrics Service
type HTTPHandler struct {
	logger   pkglogger.Logger
	collector *collector.MetricsCollector
}

// NewHTTPHandler создает новый HTTP обработчик
func NewHTTPHandler(logger pkglogger.Logger, collector *collector.MetricsCollector) *HTTPHandler {
	return &HTTPHandler{
		logger:   logger,
		collector: collector,
	}
}

// RegisterRoutes регистрирует HTTP маршруты
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	// Основные эндпоинты
	mux.HandleFunc("/metrics", h.handleMetrics)
	mux.HandleFunc("/api/services", h.handleServices)
	mux.HandleFunc("/api/services/add", h.addService)
	mux.HandleFunc("/api/services/remove/", h.handleRemoveService)
	mux.HandleFunc("/api/services/metrics/", h.handleServiceMetrics)
	mux.HandleFunc("/api/scrape", h.handleScrape)
	mux.HandleFunc("/api/status", h.handleStatus)
	
	// Health check эндпоинты
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/ready", h.handleReady)
	mux.HandleFunc("/live", h.handleLive)
}

// handleMetrics возвращает метрики в формате Prometheus
func (h *HTTPHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	h.logger.Debug("Metrics requested", 
		pkglogger.String("remote", r.RemoteAddr),
		pkglogger.String("user_agent", r.UserAgent()))
	
	// Устанавливаем заголовки для Prometheus
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	
	// Обрабатываем запрос
	handler := h.collector.GetHandler()
	handler.ServeHTTP(w, r)
}

// handleServices возвращает список всех сервисов
func (h *HTTPHandler) handleServices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getServices(w, r)
	case http.MethodPost:
		h.addService(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getServices возвращает список подключенных сервисов
func (h *HTTPHandler) getServices(w http.ResponseWriter, r *http.Request) {
	services := h.collector.GetServices()
	
	response := map[string]interface{}{
		"services": services,
		"count":    len(services),
		"timestamp": time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode services response", pkglogger.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// addService добавляет новый сервис для мониторинга
func (h *HTTPHandler) addService(w http.ResponseWriter, r *http.Request) {
	var req AddServiceRequest
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode add service request", pkglogger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// Валидация запроса
	if req.Name == "" {
		http.Error(w, "Service name is required", http.StatusBadRequest)
		return
	}
	
	if req.Address == "" {
		http.Error(w, "Service address is required", http.StatusBadRequest)
		return
	}
	
	// Добавляем сервис
	if err := h.collector.AddService(req.Name, req.Address); err != nil {
		h.logger.Error("Failed to add service", 
			pkglogger.String("service", req.Name),
			pkglogger.String("address", req.Address),
			pkglogger.Error(err))
		
		// Конвертируем ошибку в HTTP статус
		var statusCode int
		switch {
		case err != nil && err.Error() == "service already exists":
			statusCode = http.StatusConflict
		case err != nil && err.Error() == "validation failed":
			statusCode = http.StatusBadRequest
		default:
			statusCode = http.StatusInternalServerError
		}
		
		http.Error(w, err.Error(), statusCode)
		return
	}
	
	h.logger.Info("Service added successfully", 
		pkglogger.String("service", req.Name),
		pkglogger.String("address", req.Address))
	
	response := map[string]string{
		"status":  "success",
		"message": "Service added successfully",
		"service": req.Name,
		"address": req.Address,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRemoveService удаляет сервис из мониторинга
func (h *HTTPHandler) handleRemoveService(w http.ResponseWriter, r *http.Request) {
	// Извлекаем имя сервиса из URL
	serviceName := r.URL.Path[len("/api/services/"):]
	if serviceName == "" {
		http.Error(w, "Service name is required", http.StatusBadRequest)
		return
	}
	
	// Удаляем сервис
	if err := h.collector.RemoveService(serviceName); err != nil {
		h.logger.Error("Failed to remove service", 
			pkglogger.String("service", serviceName),
			pkglogger.Error(err))
		
		// Конвертируем ошибку в HTTP статус
		var statusCode int
		switch {
		case err != nil && err.Error() == "service not found":
			statusCode = http.StatusNotFound
		default:
			statusCode = http.StatusInternalServerError
		}
		
		http.Error(w, err.Error(), statusCode)
		return
	}
	
	h.logger.Info("Service removed successfully", 
		pkglogger.String("service", serviceName))
	
	response := map[string]string{
		"status":  "success",
		"message": "Service removed successfully",
		"service": serviceName,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleServiceMetrics возвращает метрики конкретного сервиса
func (h *HTTPHandler) handleServiceMetrics(w http.ResponseWriter, r *http.Request) {
	// Извлекаем имя сервиса из URL
	serviceName := r.URL.Path[len("/api/services/"):]
	if serviceName == "" {
		http.Error(w, "Service name is required", http.StatusBadRequest)
		return
	}
	
	// Получаем метрики сервиса
	serviceMetrics, err := h.collector.GetServiceMetrics(serviceName)
	if err != nil {
		h.logger.Error("Failed to get service metrics", 
			pkglogger.String("service", serviceName),
			pkglogger.Error(err))
		
		// Конвертируем ошибку в HTTP статус
		var statusCode int
		switch {
		case err != nil && err.Error() == "service not found":
			statusCode = http.StatusNotFound
		default:
			statusCode = http.StatusInternalServerError
		}
		
		http.Error(w, err.Error(), statusCode)
		return
	}
	
	// Формируем ответ
	response := map[string]interface{}{
		"service": serviceName,
		"address": serviceMetrics.Address,
		"metrics": map[string]interface{}{
			"request_count":    serviceMetrics.RequestCount,
			"request_duration": serviceMetrics.RequestDuration,
			"error_count":      serviceMetrics.ErrorCount,
			"active_connections": serviceMetrics.ActiveConnections,
		},
		"timestamp": time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode service metrics response", pkglogger.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleScrape выполняет принудительный сбор метрик
func (h *HTTPHandler) handleScrape(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Manual scrape triggered")
	
	// Выполняем сбор всех метрик
	if err := h.collector.ScrapeAll(); err != nil {
		h.logger.Error("Failed to scrape metrics", pkglogger.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"status":    "success",
		"message":   "Metrics scraped successfully",
		"timestamp": time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStatus возвращает статус сервиса
func (h *HTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	services := h.collector.GetServices()
	
	status := map[string]interface{}{
		"service":  "metrics-service",
		"version":  "1.0.0",
		"status":   "running",
		"timestamp": time.Now().UTC(),
		"services": map[string]interface{}{
			"total":   len(services),
			"monitored": services,
		},
		"endpoints": map[string]interface{}{
			"metrics":   "/metrics",
			"services":  "/api/services",
			"scrape":    "/api/scrape",
			"status":    "/api/status",
			"health":    "/health",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Health check обработчики
func (h *HTTPHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	services := h.collector.GetServices()
	
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"services": map[string]interface{}{
			"total": len(services),
			"list":  services,
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

func (h *HTTPHandler) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	response := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().UTC(),
	}
	
	json.NewEncoder(w).Encode(response)
}

func (h *HTTPHandler) handleLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().UTC(),
	}
	
	json.NewEncoder(w).Encode(response)
}

// AddServiceRequest запрос на добавление сервиса
type AddServiceRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Middleware для логирования запросов
func (h *HTTPHandler) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		h.logger.Info("HTTP request",
			pkglogger.String("method", r.Method),
			pkglogger.String("path", r.URL.Path),
			pkglogger.String("remote", r.RemoteAddr),
			pkglogger.String("user_agent", r.UserAgent()),
		)
		
		// Создаем response writer для захвата статуса
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		
		next.ServeHTTP(wrapped, r)
		
		duration := time.Since(start)
		h.logger.Info("HTTP response",
			pkglogger.String("method", r.Method),
			pkglogger.String("path", r.URL.Path),
			pkglogger.Int("status", wrapped.statusCode),
			pkglogger.String("duration", duration.String()),
		)
	})
}

// responseWriter обертка для захвата статуса ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CORSMiddleware для поддержки CORS
func (h *HTTPHandler) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
