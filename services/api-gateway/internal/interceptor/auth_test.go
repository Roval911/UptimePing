package interceptor

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"UptimePingPlatform/pkg/logger"
)

// MockLogger мок для логгера
type MockLogger struct {
	debugMessages []string
	warnMessages  []string
	errorMessages []string
}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {
	m.debugMessages = append(m.debugMessages, msg)
}

func (m *MockLogger) Info(msg string, fields ...logger.Field) {
	// Не используется в тестах
}

func (m *MockLogger) Warn(msg string, fields ...logger.Field) {
	m.warnMessages = append(m.warnMessages, msg)
}

func (m *MockLogger) Error(msg string, fields ...logger.Field) {
	m.errorMessages = append(m.errorMessages, msg)
}

func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	return m
}

func (m *MockLogger) Sync() error {
	return nil
}

// MockInvoker мок для gRPC invoker
func MockInvoker(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
	return nil
}

func TestAuthInterceptor_TokenInContext(t *testing.T) {
	mockLogger := &MockLogger{}
	interceptor := AuthInterceptor(mockLogger)

	// Создаем контекст с токеном
	ctx := context.WithValue(context.Background(), "jwt_token", "test_token_123")
	ctx = context.WithValue(ctx, "trace_id", "test-trace")

	// Вызываем интерсептор
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, MockInvoker)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Проверяем, что было логирование
	if len(mockLogger.debugMessages) == 0 {
		t.Error("Expected debug messages")
	}

	// Проверяем, что токен был найден
	foundToken := false
	for _, msg := range mockLogger.debugMessages {
		if msg == "JWT token found in context" {
			foundToken = true
			break
		}
	}
	if !foundToken {
		t.Error("Expected 'JWT token found in context' message")
	}
}

func TestAuthInterceptor_TokenInHTTPHeaders(t *testing.T) {
	mockLogger := &MockLogger{}
	interceptor := AuthInterceptor(mockLogger)

	// Создаем контекст с HTTP заголовками
	httpHeaders := map[string][]string{
		"authorization": {"Bearer test_token_456"},
	}
	ctx := context.WithValue(context.Background(), "http_headers", httpHeaders)
	ctx = context.WithValue(ctx, "trace_id", "test-trace")

	// Вызываем интерсептор
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, MockInvoker)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Проверяем, что токен был найден в HTTP заголовках
	foundToken := false
	for _, msg := range mockLogger.debugMessages {
		if msg == "JWT token found in HTTP headers" {
			foundToken = true
			break
		}
	}
	if !foundToken {
		t.Error("Expected 'JWT token found in HTTP headers' message")
	}
}

func TestAuthInterceptor_TokenInMetadata(t *testing.T) {
	mockLogger := &MockLogger{}
	interceptor := AuthInterceptor(mockLogger)

	// Создаем контекст с metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer test_token_789",
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx = context.WithValue(ctx, "trace_id", "test-trace")

	// Вызываем интерсептор
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, MockInvoker)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Проверяем, что токен был найден в metadata
	foundToken := false
	for _, msg := range mockLogger.debugMessages {
		if msg == "JWT token found in outgoing metadata" {
			foundToken = true
			break
		}
	}
	if !foundToken {
		t.Error("Expected 'JWT token found in outgoing metadata' message")
	}
}

func TestAuthInterceptor_NoToken(t *testing.T) {
	mockLogger := &MockLogger{}
	interceptor := AuthInterceptor(mockLogger)

	// Создаем контекст без токена
	ctx := context.WithValue(context.Background(), "trace_id", "test-trace")

	// Вызываем интерсептор
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, MockInvoker)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Проверяем, что было предупреждение об отсутствии токена
	if len(mockLogger.warnMessages) == 0 {
		t.Error("Expected warn messages")
	}

	foundWarning := false
	for _, msg := range mockLogger.warnMessages {
		if msg == "JWT token not found in any context source" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Expected 'JWT token not found in any context source' warning")
	}
}

func TestAuthInterceptor_InvalidTokenFormat(t *testing.T) {
	mockLogger := &MockLogger{}
	interceptor := AuthInterceptor(mockLogger)

	// Создаем контекст с токеном неверного формата
	httpHeaders := map[string][]string{
		"authorization": {"InvalidFormat token"},
	}
	ctx := context.WithValue(context.Background(), "http_headers", httpHeaders)
	ctx = context.WithValue(ctx, "trace_id", "test-trace")

	// Вызываем интерсептор
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, MockInvoker)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Проверяем, что было предупреждение об отсутствии токена (т.к. формат неверный)
	foundWarning := false
	for _, msg := range mockLogger.warnMessages {
		if msg == "JWT token not found in any context source" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Expected warning about missing token")
	}
}

func TestAuthInterceptor_ShortToken(t *testing.T) {
	mockLogger := &MockLogger{}
	interceptor := AuthInterceptor(mockLogger)

	// Создаем контекст с коротким токеном
	ctx := context.WithValue(context.Background(), "jwt_token", "short")
	ctx = context.WithValue(ctx, "trace_id", "test-trace")

	// Вызываем интерсептор
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, MockInvoker)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Проверяем, что было предупреждение о коротком токене
	foundWarning := false
	for _, msg := range mockLogger.warnMessages {
		if msg == "JWT token too short" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Expected 'JWT token too short' warning")
	}
}

func TestAuthInterceptor_PriorityOrder(t *testing.T) {
	mockLogger := &MockLogger{}
	interceptor := AuthInterceptor(mockLogger)

	// Создаем контекст с токеном во всех источниках
	httpHeaders := map[string][]string{
		"authorization": {"Bearer http_token"},
	}
	md := metadata.New(map[string]string{
		"authorization": "Bearer metadata_token",
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx = context.WithValue(ctx, "http_headers", httpHeaders)
	ctx = context.WithValue(ctx, "jwt_token", "direct_token") // Этот должен иметь приоритет
	ctx = context.WithValue(ctx, "trace_id", "test-trace")

	// Вызываем интерсептор
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, MockInvoker)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Проверяем, что был использован прямой токен из контекста (наивысший приоритет)
	foundDirectToken := false
	for _, msg := range mockLogger.debugMessages {
		if msg == "JWT token found in context" {
			foundDirectToken = true
			break
		}
	}
	if !foundDirectToken {
		t.Error("Expected 'JWT token found in context' message (highest priority)")
	}
}
