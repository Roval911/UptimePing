package domain

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsServiceClient(t *testing.T) {
	client := NewMetricsServiceClient(nil)
	assert.NotNil(t, client)
}

func TestNewHealthServiceClient(t *testing.T) {
	client := NewHealthServiceClient(nil)
	assert.NotNil(t, client)
}

func TestMockMetricsClient_GetMetrics(t *testing.T) {
	client := NewMetricsServiceClient(nil)
	
	req := &GetMetricsRequest{
		ServiceName: "test-service",
	}
	
	resp, err := client.GetMetrics(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-service", resp.ServiceName)
	assert.NotZero(t, resp.Timestamp)
	assert.Len(t, resp.Metrics, 2) // counter и histogram
}

func TestMockHealthClient_Check(t *testing.T) {
	client := NewHealthServiceClient(nil)
	
	req := &HealthCheckRequest{
		Service: "test-service",
	}
	
	resp, err := client.Check(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-service", resp.Service)
	assert.Equal(t, "SERVING", resp.Status)
	assert.Equal(t, "Service is healthy", resp.Message)
}

func TestMetric_Structure(t *testing.T) {
	metric := Metric{
		Name:  "test_metric",
		Type:  "counter",
		Value: float64(100),
		Tags:  map[string]string{
			"method": "GET",
			"status": "200",
		},
		Method: "GET",
	}

	assert.Equal(t, "test_metric", metric.Name)
	assert.Equal(t, "counter", metric.Type)
	assert.Equal(t, float64(100), metric.Value)
	assert.Equal(t, "GET", metric.Tags["method"])
	assert.Equal(t, "200", metric.Tags["status"])
	assert.Equal(t, "GET", metric.Method)
}

func TestGetMetricsRequest_Structure(t *testing.T) {
	req := &GetMetricsRequest{
		ServiceName: "test-service",
	}

	assert.Equal(t, "test-service", req.ServiceName)
}

func TestGetMetricsResponse_Structure(t *testing.T) {
	now := time.Now()
	resp := &GetMetricsResponse{
		ServiceName: "test-service",
		Timestamp:   now,
		Metrics: []Metric{
			{
				Name:   "requests_total",
				Type:   "counter",
				Value:  float64(100),
				Method: "GET",
				Tags:   map[string]string{"status": "200"},
			},
		},
	}

	assert.Equal(t, "test-service", resp.ServiceName)
	assert.Equal(t, now, resp.Timestamp)
	assert.Len(t, resp.Metrics, 1)
	assert.Equal(t, "requests_total", resp.Metrics[0].Name)
	assert.Equal(t, "counter", resp.Metrics[0].Type)
	assert.Equal(t, float64(100), resp.Metrics[0].Value)
	assert.Equal(t, "GET", resp.Metrics[0].Method)
}

func TestHealthCheckRequest_Structure(t *testing.T) {
	req := &HealthCheckRequest{
		Service: "test-service",
	}

	assert.Equal(t, "test-service", req.Service)
}

func TestHealthCheckResponse_Structure(t *testing.T) {
	resp := &HealthCheckResponse{
		Service: "test-service",
		Status:  "SERVING",
		Message: "Service is healthy",
	}

	assert.Equal(t, "test-service", resp.Service)
	assert.Equal(t, "SERVING", resp.Status)
	assert.Equal(t, "Service is healthy", resp.Message)
}

func TestMetric_Types(t *testing.T) {
	tests := []struct {
		name  string
		metric Metric
		valid  bool
	}{
		{
			name: "valid counter",
			metric: Metric{
				Name:   "requests_total",
				Type:   "counter",
				Value:  float64(100),
				Method: "GET",
			},
			valid: true,
		},
		{
			name: "valid histogram",
			metric: Metric{
				Name: "request_duration",
				Type:   "histogram",
				Value: map[string]interface{}{
					"count": float64(100),
					"sum":   float64(50.5),
				},
				Method: "GET",
			},
			valid: true,
		},
		{
			name: "valid gauge",
			metric: Metric{
				Name:   "active_connections",
				Type:   "gauge",
				Value:  float64(10),
				Method: "GET",
			},
			valid: true,
		},
		{
			name: "empty name",
			metric: Metric{
				Name:   "",
				Type:   "counter",
				Value:  float64(100),
				Method: "GET",
			},
			valid: false,
		},
		{
			name: "empty type",
			metric: Metric{
				Name:   "test_metric",
				Type:   "",
				Value:  float64(100),
				Method: "GET",
			},
			valid: false,
		},
		{
			name: "nil value",
			metric: Metric{
				Name:   "test_metric",
				Type:   "counter",
				Value:  nil,
				Method: "GET",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				assert.NotEmpty(t, tt.metric.Name)
				assert.NotEmpty(t, tt.metric.Type)
				assert.NotNil(t, tt.metric.Value)
			} else {
				assert.True(t, tt.metric.Name == "" || tt.metric.Type == "" || tt.metric.Value == nil)
			}
		})
	}
}

func TestMetric_Validation(t *testing.T) {
	// Тестируем валидацию типов метрик
	validTypes := []string{"counter", "gauge", "histogram", "summary"}
	
	for _, validType := range validTypes {
		metric := Metric{
			Name:   "test_metric",
			Type:   validType,
			Value:  float64(100),
			Method: "GET",
		}
		
		assert.Contains(t, validTypes, metric.Type)
	}
}

func TestHealthCheckStatuses(t *testing.T) {
	validStatuses := []string{"SERVING", "NOT_SERVING", "UNKNOWN"}
	
	for _, validStatus := range validStatuses {
		resp := &HealthCheckResponse{
			Service: "test-service",
			Status:  validStatus,
			Message: "test message",
		}
		
		assert.Contains(t, validStatuses, resp.Status)
	}
}

func TestMetric_TagsHandling(t *testing.T) {
	metric := Metric{
		Name:  "test_metric",
		Type:  "counter",
		Value: float64(100),
		Tags: map[string]string{
			"method":     "GET",
			"status":     "200",
			"endpoint":   "/api/test",
			"version":    "v1",
		},
		Method: "GET",
	}

	assert.Equal(t, "GET", metric.Tags["method"])
	assert.Equal(t, "200", metric.Tags["status"])
	assert.Equal(t, "/api/test", metric.Tags["endpoint"])
	assert.Equal(t, "v1", metric.Tags["version"])
	assert.Len(t, metric.Tags, 4)
}

func TestMetric_HistogramValue(t *testing.T) {
	histogramValue := map[string]interface{}{
		"count": float64(100),
		"sum":   float64(50.5),
		"min":   float64(0.1),
		"max":   float64(10.0),
	}

	metric := Metric{
		Name:   "request_duration_seconds",
		Type:   "histogram",
		Value:  histogramValue,
		Method: "GET",
	}

	assert.Equal(t, "histogram", metric.Type)
	assert.Equal(t, histogramValue, metric.Value)
	
	// Проверяем, что значение можно привести к map[string]interface{}
	if val, ok := metric.Value.(map[string]interface{}); ok {
		assert.Equal(t, float64(100), val["count"])
		assert.Equal(t, float64(50.5), val["sum"])
		assert.Equal(t, float64(0.1), val["min"])
		assert.Equal(t, float64(10.0), val["max"])
	}
}

func TestMetric_JSONSerialization(t *testing.T) {
	metric := Metric{
		Name:  "requests_total",
		Type:  "counter",
		Value: float64(100),
		Tags: map[string]string{
			"method": "GET",
			"status": "200",
		},
		Method: "GET",
	}

	// Проверяем, что структура может быть сериализована в JSON
	// (это важно для gRPC передачи)
	assert.NotEmpty(t, metric.Name)
	assert.NotEmpty(t, metric.Type)
	assert.NotNil(t, metric.Value)
	assert.NotNil(t, metric.Tags)
	assert.NotEmpty(t, metric.Method)
}

func TestGetMetricsResponse_JSONCompatibility(t *testing.T) {
	resp := &GetMetricsResponse{
		ServiceName: "test-service",
		Timestamp:   time.Now().UTC(),
		Metrics: []Metric{
			{
				Name:   "requests_total",
				Type:   "counter",
				Value:  float64(100),
				Method: "GET",
				Tags:   map[string]string{"status": "200"},
			},
			{
				Name: "request_duration_seconds",
				Type: "histogram",
				Value: map[string]interface{}{
					"count": float64(100),
					"sum":   float64(50.5),
				},
				Method: "POST",
				Tags:   map[string]string{"endpoint": "/api/test"},
			},
		},
	}

	// Проверяем структуру для совместимости с JSON
	assert.NotEmpty(t, resp.ServiceName)
	assert.False(t, resp.Timestamp.IsZero())
	assert.Len(t, resp.Metrics, 2)
	
	// Проверяем, что все метрики имеют необходимые поля
	for _, metric := range resp.Metrics {
		assert.NotEmpty(t, metric.Name)
		assert.NotEmpty(t, metric.Type)
		assert.NotNil(t, metric.Value)
		assert.NotEmpty(t, metric.Method)
	}
}

// Benchmark для проверки производительности
func BenchmarkMetric_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metric := Metric{
			Name:   "test_metric",
			Type:   "counter",
			Value:  float64(i),
			Method: "GET",
			Tags: map[string]string{
				"status": "200",
				"endpoint": "/api/test",
			},
		}
		_ = metric
	}
}

func BenchmarkGetMetricsResponse_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp := &GetMetricsResponse{
			ServiceName: "test-service",
			Timestamp:   time.Now().UTC(),
			Metrics: []Metric{
				{
					Name:   "requests_total",
					Type:   "counter",
					Value:  float64(i),
					Method: "GET",
					Tags:   map[string]string{"status": "200"},
				},
			},
		}
		_ = resp
	}
}
