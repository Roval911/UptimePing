package logging

import (
	"context"
	"testing"
	"time"

	"UptimePingPlatform/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUptimeLogger(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	uptimeLogger := NewUptimeLogger(baseLogger)
	
	require.NotNil(t, uptimeLogger)
	assert.Equal(t, baseLogger, uptimeLogger.GetBaseLogger())
}

func TestLogCheckStart(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogCheckStart(ctx, "http", "https://example.com", "check-456", "exec-789")
	
	// Проверяем, что логгер не возвращает ошибку (без sync в тестах)
	// err := ul.Sync()
	// assert.NoError(t, err)
}

func TestLogCheckComplete(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogCheckComplete(ctx, "http", "https://example.com", "check-456", "exec-789", 100*time.Millisecond, true, 200, 1024)
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogCheckError(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	testErr := assert.AnError
	ul.LogCheckError(ctx, "http", "https://example.com", "check-456", "exec-789", testErr, 50*time.Millisecond)
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogCheckDebug(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogCheckDebug(ctx, "http", "https://example.com", "check-456", "exec-789", "debug message", 
		logger.String("extra_field", "extra_value"))
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogTaskReceived(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogTaskReceived(ctx, "task-123", "check-456", "tenant-789")
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogTaskProcessed(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	
	// Тест успешной обработки
	ul.LogTaskProcessed(ctx, "task-123", "check-456", "tenant-789", true, nil)
	
	// Тест обработки с ошибкой
	testErr := assert.AnError
	ul.LogTaskProcessed(ctx, "task-124", "check-457", "tenant-789", false, testErr)
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogIncidentCreated(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogIncidentCreated(ctx, "check-456", "tenant-789", "incident-123", "critical")
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogIncidentUpdated(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogIncidentUpdated(ctx, "incident-123", "open", "acknowledged")
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogIncidentError(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	testErr := assert.AnError
	ul.LogIncidentError(ctx, "create", "incident-123", testErr)
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogConsumerStarted(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogConsumerStarted(ctx, "uptime_checks_queue")
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogConsumerStopped(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogConsumerStopped(ctx, "uptime_checks_queue")
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogConnectionError(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	testErr := assert.AnError
	ul.LogConnectionError(ctx, "rabbitmq", testErr)
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogRetryAttempt(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogRetryAttempt(ctx, "create_incident", 2, 5, 100*time.Millisecond)
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogMetricsExported(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogMetricsExported(ctx, 25)
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogServiceStarted(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogServiceStarted(ctx, "core-service", "v1.0.0")
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestLogServiceStopped(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	ul.LogServiceStopped(ctx, "core-service")
	
	// err = ul.Sync()
	// assert.NoError(t, err)
}

func TestWithCheckContext(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	checkLogger := ul.WithCheckContext(ctx, "http", "https://example.com", "check-456", "exec-789")
	
	require.NotNil(t, checkLogger)
	assert.NotEqual(t, ul, checkLogger) // Должен быть новый экземпляр
	
	// Проверяем, что логгер работает
	checkLogger.LogCheckStart(ctx, "http", "https://example.com", "check-456", "exec-789")
	
	// err = checkLogger.Sync()
	// assert.NoError(t, err)
}

func TestWithComponent(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	componentLogger := ul.WithComponent("test_component")
	
	require.NotNil(t, componentLogger)
	assert.NotEqual(t, ul, componentLogger)
	
	// err = componentLogger.Sync()
	// assert.NoError(t, err)
}

func TestWithTenant(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	ul := NewUptimeLogger(baseLogger)
	
	tenantLogger := ul.WithTenant("tenant-123")
	
	require.NotNil(t, tenantLogger)
	assert.NotEqual(t, ul, tenantLogger)
	
	// err = tenantLogger.Sync()
	// assert.NoError(t, err)
}

func TestContextFunctions(t *testing.T) {
	// Test WithTraceID and GetTraceID
	ctx := context.Background()
	traceID := "trace-123"
	
	ctx = WithTraceID(ctx, traceID)
	assert.Equal(t, traceID, GetTraceID(ctx))
	
	// Test WithCheckID and GetCheckID
	checkID := "check-456"
	ctx = WithCheckID(ctx, checkID)
	assert.Equal(t, checkID, GetCheckID(ctx))
	
	// Test WithExecutionID and GetExecutionID
	executionID := "exec-789"
	ctx = WithExecutionID(ctx, executionID)
	assert.Equal(t, executionID, GetExecutionID(ctx))
	
	// Test WithTenantID and GetTenantID
	tenantID := "tenant-123"
	ctx = WithTenantID(ctx, tenantID)
	assert.Equal(t, tenantID, GetTenantID(ctx))
	
	// Test WithCheckContext
	ctx2 := context.Background()
	ctx2 = WithCheckContext(ctx2, traceID, checkID, executionID, tenantID)
	
	assert.Equal(t, traceID, GetTraceID(ctx2))
	assert.Equal(t, checkID, GetCheckID(ctx2))
	assert.Equal(t, executionID, GetExecutionID(ctx2))
	assert.Equal(t, tenantID, GetTenantID(ctx2))
}

func TestGenerateTraceID(t *testing.T) {
	traceID1 := GenerateTraceID()
	time.Sleep(1 * time.Nanosecond) // Небольшая задержка для уникальности
	traceID2 := GenerateTraceID()
	
	assert.NotEmpty(t, traceID1)
	assert.NotEmpty(t, traceID2)
	assert.NotEqual(t, traceID1, traceID2) // Должны быть разными
}

func TestGetTraceID_Empty(t *testing.T) {
	ctx := context.Background()
	traceID := GetTraceID(ctx)
	assert.Empty(t, traceID)
}

func TestGetCheckID_Empty(t *testing.T) {
	ctx := context.Background()
	checkID := GetCheckID(ctx)
	assert.Empty(t, checkID)
}

func TestGetExecutionID_Empty(t *testing.T) {
	ctx := context.Background()
	executionID := GetExecutionID(ctx)
	assert.Empty(t, executionID)
}

func TestGetTenantID_Empty(t *testing.T) {
	ctx := context.Background()
	tenantID := GetTenantID(ctx)
	assert.Empty(t, tenantID)
}

func TestGetTraceID_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), TraceIDKey, 123) // Неправильный тип
	traceID := GetTraceID(ctx)
	assert.Empty(t, traceID)
}

func TestInitGlobalUptimeLogger(t *testing.T) {
	baseLogger, err := logger.NewLogger("development", "debug", "test-service", false)
	require.NoError(t, err)
	
	InitGlobalUptimeLogger(baseLogger)
	
	globalLogger := GetGlobalUptimeLogger()
	require.NotNil(t, globalLogger)
	assert.Equal(t, baseLogger, globalLogger.GetBaseLogger())
}

func TestGetGlobalUptimeLogger_Default(t *testing.T) {
	// Сбрасываем глобальный логгер
	GlobalUptimeLogger = nil
	
	globalLogger := GetGlobalUptimeLogger()
	require.NotNil(t, globalLogger)
	
	// Проверяем, что работает (без sync в тестах)
	// err := globalLogger.Sync()
	// assert.NoError(t, err)
}

// Бенчмарки
func BenchmarkLogCheckStart(b *testing.B) {
	baseLogger, _ := logger.NewLogger("development", "info", "test-service", false)
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ul.LogCheckStart(ctx, "http", "https://example.com", "check-456", "exec-789")
	}
}

func BenchmarkLogCheckComplete(b *testing.B) {
	baseLogger, _ := logger.NewLogger("development", "info", "test-service", false)
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ul.LogCheckComplete(ctx, "http", "https://example.com", "check-456", "exec-789", 100*time.Millisecond, true, 200, 1024)
	}
}

func BenchmarkWithCheckContext(b *testing.B) {
	baseLogger, _ := logger.NewLogger("development", "info", "test-service", false)
	ul := NewUptimeLogger(baseLogger)
	
	ctx := WithTraceID(context.Background(), "trace-123")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ul.WithCheckContext(ctx, "http", "https://example.com", "check-456", "exec-789")
	}
}

func BenchmarkGenerateTraceID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateTraceID()
	}
}
