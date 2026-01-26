package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUptimeMetrics(t *testing.T) {
	serviceName := "test-service"
	metrics := NewUptimeMetrics(serviceName)
	
	require.NotNil(t, metrics)
	require.NotNil(t, metrics.base)
	require.NotNil(t, metrics.checkDuration)
	require.NotNil(t, metrics.checkTotal)
	require.NotNil(t, metrics.checkErrors)
	require.NotNil(t, metrics.checkActive)
	require.NotNil(t, metrics.lastSuccessTimestamp)
	require.NotNil(t, metrics.responseSize)
}

func TestRecordCheckDuration(t *testing.T) {
	metrics := NewUptimeMetrics("test-service")
	
	checkType := "http"
	target := "https://example.com"
	status := "success"
	duration := 100 * time.Millisecond
	
	// Просто проверяем, что вызов не паникует
	assert.NotPanics(t, func() {
		metrics.RecordCheckDuration(checkType, target, status, duration)
	})
}

func TestIncrementCheckTotal(t *testing.T) {
	metrics := NewUptimeMetrics("test-service")
	
	checkType := "tcp"
	target := "example.com:80"
	status := "failure"
	
	// Просто проверяем, что вызов не паникует
	assert.NotPanics(t, func() {
		metrics.IncrementCheckTotal(checkType, target, status)
	})
}

func TestIncrementCheckErrors(t *testing.T) {
	metrics := NewUptimeMetrics("test-service")
	
	checkType := "grpc"
	target := "grpc.example.com:50051"
	errorType := "timeout"
	
	// Просто проверяем, что вызов не паникует
	assert.NotPanics(t, func() {
		metrics.IncrementCheckErrors(checkType, target, errorType)
	})
}

func TestActiveChecks(t *testing.T) {
	metrics := NewUptimeMetrics("test-service")
	
	// Просто проверяем, что вызовы не паникуют
	assert.NotPanics(t, func() {
		metrics.IncrementActiveChecks()
		metrics.DecrementActiveChecks()
	})
}

func TestRecordLastSuccessTimestamp(t *testing.T) {
	metrics := NewUptimeMetrics("test-service")
	
	checkType := "http"
	target := "https://example.com"
	timestamp := time.Now()
	
	// Просто проверяем, что вызов не паникует
	assert.NotPanics(t, func() {
		metrics.RecordLastSuccessTimestamp(checkType, target, timestamp)
	})
}

func TestRecordResponseSize(t *testing.T) {
	metrics := NewUptimeMetrics("test-service")
	
	checkType := "http"
	target := "https://example.com"
	status := "success"
	sizeBytes := int64(1024)
	
	// Просто проверяем, что вызов не паникует
	assert.NotPanics(t, func() {
		metrics.RecordResponseSize(checkType, target, status, sizeBytes)
	})
}

func TestRecordCheckResult_Success(t *testing.T) {
	metrics := NewUptimeMetrics("test-service")
	
	checkType := "http"
	target := "https://example.com"
	duration := 200 * time.Millisecond
	success := true
	responseSize := int64(2048)
	errorMsg := ""
	
	// Просто проверяем, что вызов не паникует
	assert.NotPanics(t, func() {
		metrics.RecordCheckResult(checkType, target, duration, success, responseSize, errorMsg)
	})
}

func TestRecordCheckResult_Failure(t *testing.T) {
	metrics := NewUptimeMetrics("test-service")
	
	checkType := "http"
	target := "https://example.com"
	duration := 100 * time.Millisecond
	success := false
	responseSize := int64(0)
	errorMsg := "connection timeout"
	
	// Просто проверяем, что вызов не паникует
	assert.NotPanics(t, func() {
		metrics.RecordCheckResult(checkType, target, duration, success, responseSize, errorMsg)
	})
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
		expected string
	}{
		{"timeout error", "connection timeout", "timeout"},
		{"deadline error", "context deadline exceeded", "timeout"},
		{"connection error", "network connection failed", "connection"},
		{"dns error", "dns resolution failed", "dns"},
		{"ssl error", "ssl certificate expired", "ssl"},
		{"tls error", "tls handshake failed", "ssl"},
		{"404 error", "404 not found", "not_found"},
		{"server error", "500 internal server error", "server_error"},
		{"client error", "400 bad request", "client_error"},
		{"unknown error", "something went wrong", "unknown"},
		{"empty error", "", "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeError(tt.errorMsg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"match lower", "hello world", "world", true},
		{"match upper", "HELLO WORLD", "WORLD", true},
		{"match mixed", "Hello World", "WORLD", true},
		{"no match", "hello world", "test", false},
		{"empty substring", "hello", "", true},
		{"substring longer", "hi", "hello", false},
		{"case insensitive", "Timeout", "timeout", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUptimeMetricsAdapter(t *testing.T) {
	um := NewUptimeMetrics("test-service")
	adapter := NewUptimeMetricsAdapter(um)
	
	checkType := "http"
	target := "https://example.com"
	
	// Просто проверяем, что вызовы не паникуют
	assert.NotPanics(t, func() {
		adapter.OnCheckStarted(checkType, target)
		
		duration := 150 * time.Millisecond
		success := true
		responseSize := int64(512)
		errorMsg := ""
		
		adapter.OnCheckCompleted(checkType, target, duration, success, responseSize, errorMsg)
		adapter.OnCheckError(checkType, target, "connection")
	})
}

func TestTraceCheck(t *testing.T) {
	um := NewUptimeMetrics("test-service")
	
	ctx := context.Background()
	checkType := "http"
	target := "https://example.com"
	
	// Тест успешной трассировки
	err := um.TraceCheck(ctx, checkType, target, func(ctx context.Context) error {
		return nil
	})
	
	assert.NoError(t, err)
	
	// Тест трассировки с ошибкой
	testErr := assert.AnError
	err = um.TraceCheck(ctx, checkType, target, func(ctx context.Context) error {
		return testErr
	})
	
	assert.Equal(t, testErr, err)
}

func TestGetBaseMetrics(t *testing.T) {
	um := NewUptimeMetrics("test-service")
	
	base := um.GetBaseMetrics()
	assert.NotNil(t, base)
	assert.Equal(t, um.base, base)
}

func TestGetHandler(t *testing.T) {
	um := NewUptimeMetrics("test-service")
	
	handler := um.GetHandler()
	assert.NotNil(t, handler)
}

// Бенчмарки
func BenchmarkRecordCheckResult(b *testing.B) {
	um := NewUptimeMetrics("benchmark-service")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		um.RecordCheckResult("http", "https://example.com", 100*time.Millisecond, true, 1024, "")
	}
}

func BenchmarkCategorizeError(b *testing.B) {
	errorMsg := "connection timeout occurred while processing request"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		categorizeError(errorMsg)
	}
}

func BenchmarkContainsIgnoreCase(b *testing.B) {
	s := "Hello World"
	substr := "WORLD"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		containsIgnoreCase(s, substr)
	}
}
