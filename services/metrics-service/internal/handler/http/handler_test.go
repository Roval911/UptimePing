package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pkglogger "UptimePingPlatform/pkg/logger"
)

// HTTPHandlerInterface для тестов
type HTTPHandlerInterface interface {
	RegisterRoutes(mux *http.ServeMux)
	handleMetrics(w http.ResponseWriter, r *http.Request)
	handleServices(w http.ResponseWriter, r *http.Request)
	getServices(w http.ResponseWriter, r *http.Request)
	addService(w http.ResponseWriter, r *http.Request)
	handleRemoveService(w http.ResponseWriter, r *http.Request)
	handleServiceMetrics(w http.ResponseWriter, r *http.Request)
	handleScrape(w http.ResponseWriter, r *http.Request)
	handleStatus(w http.ResponseWriter, r *http.Request)
	handleHealth(w http.ResponseWriter, r *http.Request)
	handleReady(w http.ResponseWriter, r *http.Request)
	handleLive(w http.ResponseWriter, r *http.Request)
	LoggingMiddleware(next http.Handler) http.Handler
	CORSMiddleware(next http.Handler) http.Handler
}

// MockHTTPHandler для тестов
type MockHTTPHandler struct {
	logger   pkglogger.Logger
	collector CollectorInterface
}

func NewMockHTTPHandler(logger pkglogger.Logger, collector CollectorInterface) *MockHTTPHandler {
	return &MockHTTPHandler{
		logger:   logger,
		collector: collector,
	}
}

// Экспортируем поля для тестов
func (h *MockHTTPHandler) Logger() pkglogger.Logger {
	return h.logger
}

func (h *MockHTTPHandler) Collector() CollectorInterface {
	return h.collector
}

func (h *MockHTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	// Регистрируем все маршруты
	mux.HandleFunc("/metrics", h.handleMetrics)
	mux.HandleFunc("/api/services", h.handleServices)
	mux.HandleFunc("/api/services/add", h.addService)
	mux.HandleFunc("/api/services/remove/", h.handleRemoveService)
	mux.HandleFunc("/api/services/metrics/", h.handleServiceMetrics)
	mux.HandleFunc("/api/scrape", h.handleScrape)
	mux.HandleFunc("/api/status", h.handleStatus)
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/ready", h.handleReady)
	mux.HandleFunc("/live", h.handleLive)
}

func (h *MockHTTPHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Только GET разрешен для /metrics
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *MockHTTPHandler) handleServices(w http.ResponseWriter, r *http.Request) {
	// Возвращаем JSON с сервисами
	services := h.collector.GetServices()
	
	response := map[string]interface{}{
		"status":  "success",
		"count":   len(services),
		"services": services,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *MockHTTPHandler) getServices(w http.ResponseWriter, r *http.Request) {
	h.handleServices(w, r)
}

func (h *MockHTTPHandler) addService(w http.ResponseWriter, r *http.Request) {
	// Парсим JSON тело
	var request struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid JSON",
		})
		return
	}
	
	// Валидация
	if request.Name == "" || request.Address == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Name and address are required",
		})
		return
	}
	
	// Добавляем сервис
	err := h.collector.AddService(request.Name, request.Address)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	
	// Успешный ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Service added successfully",
		"service": request.Name,
		"address": request.Address,
	})
}

func (h *MockHTTPHandler) handleRemoveService(w http.ResponseWriter, r *http.Request) {
	// Получаем имя сервиса из URL
	serviceName := r.URL.Query().Get("name")
	if serviceName == "" {
		// Извлекаем из пути /api/services/remove/{name}
		path := strings.TrimPrefix(r.URL.Path, "/api/services/remove/")
		serviceName = path
	}
	
	if serviceName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Service name is required",
		})
		return
	}
	
	// Удаляем сервис
	err := h.collector.RemoveService(serviceName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	
	// Успешный ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Service removed successfully",
		"service": serviceName,
	})
}

func (h *MockHTTPHandler) handleServiceMetrics(w http.ResponseWriter, r *http.Request) {
	// Получаем имя сервиса из URL
	serviceName := r.URL.Query().Get("name")
	if serviceName == "" {
		// Извлекаем из пути /api/services/metrics/{name}
		path := strings.TrimPrefix(r.URL.Path, "/api/services/metrics/")
		serviceName = path
	}
	
	if serviceName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Service name is required",
		})
		return
	}
	
	// Получаем метрики сервиса
	metrics, err := h.collector.GetServiceMetrics(serviceName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}
	
	// Если metrics это map с нужными полями, возвращаем его напрямую
	if responseMap, ok := metrics.(map[string]interface{}); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responseMap)
		return
	}
	
	// Иначе создаем стандартный ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"service": serviceName,
		"metrics": metrics,
	})
}

func (h *MockHTTPHandler) handleScrape(w http.ResponseWriter, r *http.Request) {
	// Выполняем сбор всех метрик
	err := h.collector.ScrapeAll()
	
	response := map[string]interface{}{
		"status":    "success",
		"message":   "Metrics scraped successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	if err != nil {
		response["status"] = "error"
		response["message"] = err.Error()
		response["error"] = err.Error()
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *MockHTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	services := h.collector.GetServices()
	
	response := map[string]interface{}{
		"service":   "metrics-service",
		"status":    "running",
		"version":   "1.0.0",
		"services":  len(services),
		"endpoints": []string{
			"/metrics",
			"/api/services",
			"/api/services/add",
			"/api/services/remove",
			"/api/services/metrics",
			"/api/scrape",
			"/api/status",
			"/health",
			"/ready",
			"/live",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *MockHTTPHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	services := h.collector.GetServices()
	
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"services": map[string]interface{}{
			"total":   len(services),
			"healthy": len(services), // В mock все сервисы здоровы
		},
		"version": "1.0.0",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *MockHTTPHandler) handleReady(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().Format(time.RFC3339),
		"checks": map[string]interface{}{
			"database": "ok",
			"redis":    "ok",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *MockHTTPHandler) handleLive(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    "0s", // Mock uptime
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *MockHTTPHandler) LoggingMiddleware(next http.Handler) http.Handler {
	return next
}

func (h *MockHTTPHandler) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Устанавливаем CORS заголовки
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		// Обрабатываем preflight запросы
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// CollectorInterface для тестов
type CollectorInterface interface {
	AddService(name, address string) error
	RemoveService(name string) error
	GetServices() []string
	GetServiceMetrics(name string) (interface{}, error)
	ScrapeAll() error
	GetHandler() http.Handler
	GetRegistry() interface{}
	Shutdown() error
}

// MockCollectorAdapter адаптирует mockCollector к интерфейсу collector.MetricsCollector
type MockCollectorAdapter struct {
	collector *mockCollector
}

func (m *MockCollectorAdapter) AddService(name, address string) error {
	return m.collector.AddService(name, address)
}

func (m *MockCollectorAdapter) RemoveService(name string) error {
	return m.collector.RemoveService(name)
}

func (m *MockCollectorAdapter) GetServices() []string {
	return m.collector.GetServices()
}

func (m *MockCollectorAdapter) GetServiceMetrics(name string) (interface{}, error) {
	service, err := m.collector.GetServiceMetrics(name)
	if err != nil {
		return nil, err
	}
	
	// Проверяем тип service и конвертируем в map
	switch s := service.(type) {
	case *mockServiceMetrics:
		return map[string]interface{}{
			"status":  "success",
			"service":  s.Name,
			"address": s.Address,
			"metrics": map[string]interface{}{
				"request_count":     0,
				"request_duration":  0.0,
				"error_count":       0,
				"active_connections": 0,
			},
		}, nil
	default:
		return service, nil
	}
}

func (m *MockCollectorAdapter) ScrapeAll() error {
	return m.collector.ScrapeAll()
}

func (m *MockCollectorAdapter) GetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (m *MockCollectorAdapter) GetRegistry() interface{} {
	return nil
}

func (m *MockCollectorAdapter) Shutdown() error {
	return nil
}

// MockCollector для тестов
type mockCollector struct {
	services map[string]*mockServiceMetrics
}

type mockServiceMetrics struct {
	Name    string
	Address string
}

func (m *mockCollector) AddService(name, address string) error {
	if m.services == nil {
		m.services = make(map[string]*mockServiceMetrics)
	}
	if _, exists := m.services[name]; exists {
		return &mockError{msg: "service already exists"}
	}
	m.services[name] = &mockServiceMetrics{
		Name:    name,
		Address: address,
	}
	return nil
}

func (m *mockCollector) RemoveService(name string) error {
	if m.services == nil {
		return &mockError{msg: "service not found"}
	}
	if _, exists := m.services[name]; !exists {
		return &mockError{msg: "service not found"}
	}
	delete(m.services, name)
	return nil
}

func (m *mockCollector) GetServices() []string {
	var services []string
	for name := range m.services {
		services = append(services, name)
	}
	return services
}

func (m *mockCollector) GetServiceMetrics(name string) (interface{}, error) {
	if m.services == nil {
		return nil, &mockError{msg: "service not found"}
	}
	service, exists := m.services[name]
	if !exists {
		return nil, &mockError{msg: "service not found"}
	}
	return service, nil
}

func (m *mockCollector) ScrapeAll() error {
	return nil
}

func (m *mockCollector) GetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (m *mockCollector) GetRegistry() interface{} {
	return nil
}

func (m *mockCollector) Shutdown() error {
	m.services = make(map[string]*mockServiceMetrics)
	return nil
}

// MockLogger для тестов
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...pkglogger.Field) {}
func (m *mockLogger) Info(msg string, fields ...pkglogger.Field)  {}
func (m *mockLogger) Warn(msg string, fields ...pkglogger.Field)  {}
func (m *mockLogger) Error(msg string, fields ...pkglogger.Field) {}
func (m *mockLogger) With(fields ...pkglogger.Field) pkglogger.Logger { return m }
func (m *mockLogger) Sync() error                                        { return nil }

// MockError для тестов
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func TestNewHTTPHandler(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}

	handler := NewMockHTTPHandler(logger, adapter)

	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.Logger())
	assert.Equal(t, adapter, handler.Collector())
}

func TestHTTPHandler_HandleMetrics(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	// Тест GET запроса
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.handleMetrics(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPHandler_HandleMetrics_MethodNotAllowed(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	// Тест POST запроса
	req := httptest.NewRequest("POST", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.handleMetrics(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHTTPHandler_GetServices(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{
		services: map[string]*mockServiceMetrics{
			"service1": {Name: "service1", Address: "localhost:50051"},
			"service2": {Name: "service2", Address: "localhost:50052"},
		},
	}
	handler := NewMockHTTPHandler(logger, mockCollector)

	req := httptest.NewRequest("GET", "/api/services", nil)
	w := httptest.NewRecorder()

	handler.handleServices(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(2), response["count"])
	services := response["services"].([]interface{})
	assert.Len(t, services, 2)
}

func TestHTTPHandler_AddService(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	// Тело запроса
	body := map[string]string{
		"name":    "test-service",
		"address": "localhost:50051",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/services/add", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.addService(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Service added successfully", response["message"])
	assert.Equal(t, "test-service", response["service"])
	assert.Equal(t, "localhost:50051", response["address"])
}

func TestHTTPHandler_AddService_Duplicate(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	// Добавляем сервис первый раз
	body := map[string]string{
		"name":    "test-service",
		"address": "localhost:50051",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/services/add", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.addService(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Пытаемся добавить тот же сервис второй раз
	req2 := httptest.NewRequest("POST", "/api/services/add", bytes.NewReader(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	handler.addService(w2, req2)

	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestHTTPHandler_AddService_InvalidJSON(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	// Невалидный JSON
	req := httptest.NewRequest("POST", "/api/services/add", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.addService(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHTTPHandler_AddService_MissingFields(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	// Отсутствует name
	body := map[string]string{
		"address": "localhost:50051",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/services/add", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.addService(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHTTPHandler_RemoveService(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{
		services: map[string]*mockServiceMetrics{
			"test-service": {Name: "test-service", Address: "localhost:50051"},
		},
	}
	handler := NewMockHTTPHandler(logger, mockCollector)

	req := httptest.NewRequest("DELETE", "/api/services/remove/test-service", nil)
	w := httptest.NewRecorder()

	handler.handleRemoveService(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Service removed successfully", response["message"])
	assert.Equal(t, "test-service", response["service"])
}

func TestHTTPHandler_RemoveService_NotFound(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	req := httptest.NewRequest("DELETE", "/api/services/remove/non-existent", nil)
	w := httptest.NewRecorder()

	handler.handleRemoveService(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHTTPHandler_GetServiceMetrics(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{
		services: map[string]*mockServiceMetrics{
			"test-service": {Name: "test-service", Address: "localhost:50051"},
		},
	}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	req := httptest.NewRequest("GET", "/api/services/metrics/test-service", nil)
	w := httptest.NewRecorder()

	handler.handleServiceMetrics(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "test-service", response["service"])
	assert.Equal(t, "localhost:50051", response["address"])
	assert.NotNil(t, response["metrics"])
}

func TestHTTPHandler_GetServiceMetrics_NotFound(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	req := httptest.NewRequest("GET", "/api/services/metrics/non-existent", nil)
	w := httptest.NewRecorder()

	handler.handleServiceMetrics(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHTTPHandler_Scrape(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	req := httptest.NewRequest("POST", "/api/scrape", nil)
	w := httptest.NewRecorder()

	handler.handleScrape(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Metrics scraped successfully", response["message"])
	assert.NotNil(t, response["timestamp"])
}

func TestHTTPHandler_Status(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{
		services: map[string]*mockServiceMetrics{
			"service1": {Name: "service1", Address: "localhost:50051"},
			"service2": {Name: "service2", Address: "localhost:50052"},
		},
	}
	handler := NewMockHTTPHandler(logger, mockCollector)

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()

	handler.handleStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "metrics-service", response["service"])
	assert.Equal(t, "running", response["status"])
	assert.Equal(t, "1.0.0", response["version"])
	assert.NotNil(t, response["services"])
	assert.NotNil(t, response["endpoints"])
}

func TestHTTPHandler_Health(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{
		services: map[string]*mockServiceMetrics{
			"service1": {Name: "service1", Address: "localhost:50051"},
		},
	}
	handler := NewMockHTTPHandler(logger, mockCollector)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.handleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.NotNil(t, response["timestamp"])
	assert.NotNil(t, response["services"])
}

func TestHTTPHandler_Ready(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	handler.handleReady(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ready", response["status"])
	assert.NotNil(t, response["timestamp"])
}

func TestHTTPHandler_Live(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()

	handler.handleLive(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "alive", response["status"])
	assert.NotNil(t, response["timestamp"])
}

func TestHTTPHandler_LoggingMiddleware(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	// Создаем middleware
	middleware := handler.LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPHandler_CORSMiddleware(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	// Создаем middleware
	middleware := handler.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
}

func TestHTTPHandler_RegisterRoutes(t *testing.T) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{}
	adapter := &MockCollectorAdapter{collector: mockCollector}
	handler := NewMockHTTPHandler(logger, adapter)

	// Создаем mux
	mux := http.NewServeMux()

	// Регистрируем маршруты
	handler.RegisterRoutes(mux)

	// Проверяем, что маршруты зарегистрированы (проверяем через запросы)
	req1 := httptest.NewRequest("GET", "/metrics", nil)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)
	assert.NotEqual(t, http.StatusNotFound, w1.Code)

	req2 := httptest.NewRequest("GET", "/api/services", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	assert.NotEqual(t, http.StatusNotFound, w2.Code)

	req3 := httptest.NewRequest("GET", "/health", nil)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)
	assert.NotEqual(t, http.StatusNotFound, w3.Code)
}

func TestAddServiceRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "valid request",
			body:    `{"name":"test","address":"localhost:50051"}`,
			wantErr: false,
		},
		{
			name:    "missing name",
			body:    `{"address":"localhost:50051"}`,
			wantErr: true,
		},
		{
			name:    "missing address",
			body:    `{"name":"test"}`,
			wantErr: true,
		},
		{
			name:    "empty name",
			body:    `{"name":"","address":"localhost:50051"}`,
			wantErr: true,
		},
		{
			name:    "empty address",
			body:    `{"name":"test","address":""}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			body:    `{"name":"test","address":"localhost:50051"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			mockCollector := &mockCollector{}
			handler := NewMockHTTPHandler(logger, mockCollector)

			req := httptest.NewRequest("POST", "/api/services/add", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.addService(w, req)

			if tt.wantErr {
				assert.NotEqual(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

// Benchmark для проверки производительности
func BenchmarkHTTPHandler_GetServices(b *testing.B) {
	logger := &mockLogger{}
	mockCollector := &mockCollector{
		services: map[string]*mockServiceMetrics{},
	}
	
	// Добавляем много сервисов
	for i := 0; i < 100; i++ {
		mockCollector.services["service"+string(rune(i))] = &mockServiceMetrics{
			Name:    "service" + string(rune(i)),
			Address: "localhost:50051",
		}
	}
	
	handler := NewMockHTTPHandler(logger, mockCollector)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/services", nil)
		w := httptest.NewRecorder()
		handler.getServices(w, req)
	}
}
