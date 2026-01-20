package logger

import (
	"context"
	"testing"
)

// TestNewLogger_DevEnvironment проверяет создание логгера для dev окружения
func TestNewLogger_DevEnvironment(t *testing.T) {
	logger, err := NewLogger("dev", "debug", "test-service", false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Проверяем, что логгер создан
	if logger == nil {
		t.Fatal("Expected logger, got nil")
	}
	
	// Проверяем, что можно записывать логи
	logger.Info("Test message")
	logger.With(String("test", "value")).Info("Test message with field")
}

// TestNewLogger_ProdEnvironment проверяет создание логгера для prod окружения
func TestNewLogger_ProdEnvironment(t *testing.T) {
	logger, err := NewLogger("prod", "info", "test-service", true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Проверяем, что логгер создан
	if logger == nil {
		t.Fatal("Expected logger, got nil")
	}
	
	// Проверяем, что можно записывать логи
	logger.Info("Test message")
	logger.Error("Test error")
}

// TestLogger_Levels проверяет все уровни логирования
func TestLogger_Levels(t *testing.T) {
	logger, err := NewLogger("dev", "debug", "test-service", false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Проверяем все уровни логирования
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warn message")
	logger.Error("Error message")
}

// TestLogger_WithFields проверяет добавление полей к логгеру
func TestLogger_WithFields(t *testing.T) {
	logger, err := NewLogger("dev", "debug", "test-service", false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Создаем логгер с дополнительными полями
	logger = logger.With(
		String("component", "test"),
		Int("instance", 1),
	)
	
	// Проверяем, что можно записывать логи с полями
	logger.Info("Test message with component")
}

// TestLogger_CtxField проверяет создание поля с trace_id из контекста
func TestLogger_CtxField(t *testing.T) {
	// Создаем контекст с trace_id
	ctx := context.WithValue(context.Background(), "trace_id", "test-trace-123")
	
	// Проверяем, что CtxField возвращает правильное поле
	field := CtxField(ctx)
	
	// Поле должно иметь ключ trace_id
	if field.Field.Key != "trace_id" {
		t.Errorf("Expected field key to be 'trace_id', got %s", field.Field.Key)
	}
	
	// Значение должно соответствовать значению из контекста
	if field.Field.String == "" {
		t.Error("Expected field value, got empty")
	}
}

// TestLogger_Fields проверяет создание различных типов полей
func TestLogger_Fields(t *testing.T) {
	// Проверяем создание различных типов полей
	stringField := String("name", "test")
	if stringField.Field.Key != "name" {
		t.Errorf("Expected string field key to be 'name', got %s", stringField.Field.Key)
	}
	
	intField := Int("count", 42)
	if intField.Field.Key != "count" {
		t.Errorf("Expected int field key to be 'count', got %s", intField.Field.Key)
	}
	
	float64Field := Float64("value", 3.14)
	if float64Field.Field.Key != "value" {
		t.Errorf("Expected float64 field key to be 'value', got %s", float64Field.Field.Key)
	}
	
	boolField := Bool("active", true)
	if boolField.Field.Key != "active" {
		t.Errorf("Expected bool field key to be 'active', got %s", boolField.Field.Key)
	}
	
	errField := Error(nil)
	if errField.Field.Key != "error" {
		t.Errorf("Expected error field key to be 'error', got %s", errField.Field.Key)
	}
	
	anyField := Any("data", map[string]interface{}{"key": "value"})
	if anyField.Field.Key != "data" {
		t.Errorf("Expected any field key to be 'data', got %s", anyField.Field.Key)
	}
}

// TestNewLogger_InvalidLevel проверяет создание логгера с некорректным уровнем
func TestNewLogger_InvalidLevel(t *testing.T) {
	// При некорректном уровне должен использоваться info по умолчанию
	logger, err := NewLogger("dev", "invalid", "test-service", false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Проверяем, что логгер создан
	if logger == nil {
		t.Fatal("Expected logger, got nil")
	}
}