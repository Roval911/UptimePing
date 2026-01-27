package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActiveConnectionsMetrics(t *testing.T) {
	metrics := NewMetrics("test-service")
	
	// Test that methods don't panic
	assert.NotPanics(t, func() {
		metrics.SetActiveConnections("http", 10.0)
		metrics.IncrementActiveConnections("http")
		metrics.DecrementActiveConnections("http")
	})
}

func TestQueueSizeMetrics(t *testing.T) {
	metrics := NewMetrics("test-service")
	
	// Test that methods don't panic
	assert.NotPanics(t, func() {
		metrics.SetQueueSize("task_queue", 100.0)
		metrics.IncrementQueueSize("task_queue")
		metrics.DecrementQueueSize("task_queue")
	})
}

func TestMetricsMiddlewareWithAdditionalMetrics(t *testing.T) {
	metrics := NewMetrics("test-service")
	
	// Test that additional metrics don't panic
	assert.NotPanics(t, func() {
		metrics.SetActiveConnections("http", 5.0)
		metrics.SetQueueSize("task_queue", 25.0)
	})
	
	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// Wrap with middleware
	wrapped := metrics.Middleware(handler)
	
	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	// Serve the request
	wrapped.ServeHTTP(w, req)
	
	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestInitializeOpenTelemetry(t *testing.T) {
	err := InitializeOpenTelemetry("test-service")
	assert.NoError(t, err)
	
	// Verify tracer is available by creating metrics instance
	metrics := NewMetrics("test-service")
	assert.NotNil(t, metrics.Tracer)
}
