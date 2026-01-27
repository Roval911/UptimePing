package domain

import (
	"context"
	"time"
)

// MetricsServiceClient интерфейс для gRPC клиента метрик
type MetricsServiceClient interface {
	GetMetrics(ctx context.Context, req *GetMetricsRequest) (*GetMetricsResponse, error)
}

// HealthServiceClient интерфейс для gRPC клиента health check
type HealthServiceClient interface {
	Check(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error)
}

// GetMetricsRequest запрос на получение метрик
type GetMetricsRequest struct {
	ServiceName string `json:"service_name"`
}

// GetMetricsResponse ответ с метриками
type GetMetricsResponse struct {
	ServiceName string                 `json:"service_name"`
	Timestamp   time.Time              `json:"timestamp"`
	Metrics     []Metric               `json:"metrics"`
}

// Metric представляет метрику
type Metric struct {
	Name  string      `json:"name"`
	Type  string      `json:"type"` // counter, gauge, histogram
	Value interface{} `json:"value"`
	Tags  map[string]string `json:"tags"`
	Method string      `json:"method"`
}

// HealthCheckRequest запрос на проверку здоровья
type HealthCheckRequest struct {
	Service string `json:"service"`
}

// HealthCheckResponse ответ на проверку здоровья
type HealthCheckResponse struct {
	Service string    `json:"service"`
	Status  string    `json:"status"` // SERVING, NOT_SERVING, UNKNOWN
	Message string    `json:"message,omitempty"`
}

// NewMetricsServiceClient создает новый клиент метрик
func NewMetricsServiceClient(conn interface{}) MetricsServiceClient {
	// Здесь должна быть реализация создания gRPC клиента
	// Для примера возвращаем mock
	return &mockMetricsClient{}
}

// NewHealthServiceClient создает новый клиент health check
func NewHealthServiceClient(conn interface{}) HealthServiceClient {
	// Здесь должна быть реализация создания gRPC клиента
	// Для примера возвращаем mock
	return &mockHealthClient{}
}

// Mock реализации для примера
type mockMetricsClient struct{}

func (m *mockMetricsClient) GetMetrics(ctx context.Context, req *GetMetricsRequest) (*GetMetricsResponse, error) {
	return &GetMetricsResponse{
		ServiceName: req.ServiceName,
		Timestamp:   time.Now(),
		Metrics: []Metric{
			{
				Name: "requests_total",
				Type: "counter",
				Value: float64(100),
				Method: "GET",
				Tags: map[string]string{
					"status": "200",
				},
			},
			{
				Name: "request_duration_seconds",
				Type: "histogram",
				Value: map[string]interface{}{
					"count": 100,
					"sum":   50.5,
				},
				Method: "GET",
				Tags: map[string]string{},
			},
		},
	}, nil
}

type mockHealthClient struct{}

func (m *mockHealthClient) Check(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	return &HealthCheckResponse{
		Service: req.Service,
		Status:  "SERVING",
		Message: "Service is healthy",
	}, nil
}
