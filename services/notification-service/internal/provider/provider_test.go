package provider_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/domain"
	email "UptimePingPlatform/services/notification-service/internal/provider/email"
	provider "UptimePingPlatform/services/notification-service/internal/provider"
	"UptimePingPlatform/services/notification-service/internal/provider/retry"
	slack "UptimePingPlatform/services/notification-service/internal/provider/slack"
	telegram "UptimePingPlatform/services/notification-service/internal/provider/telegram"
)

// MockLogger для тестов
type MockLogger struct{}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {}
func (m *MockLogger) Info(msg string, fields ...logger.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...logger.Field)  {}
func (m *MockLogger) Error(msg string, fields ...logger.Field) {}
func (m *MockLogger) With(fields ...logger.Field) logger.Logger { return m }
func (m *MockLogger) Sync() error                             { return nil }

// MockProvider для тестов
type MockProvider struct {
	name      string
	sendError bool
	healthy   bool
}

func (m *MockProvider) Send(ctx context.Context, notification *domain.Notification) error {
	if m.sendError {
		return fmt.Errorf("mock send error")
	}
	return nil
}

func (m *MockProvider) GetType() string {
	return m.name
}

func (m *MockProvider) IsHealthy(ctx context.Context) bool {
	return m.healthy
}

func (m *MockProvider) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":    m.name,
		"healthy": m.healthy,
	}
}

func (m *MockProvider) ShouldRetry(err error) bool {
	return true
}

func TestProviderManager(t *testing.T) {
	// Создание mock компонентов
	mockLogger := &MockLogger{}
	
	// Тестовая конфигурация
	config := provider.DefaultProviderConfig()
	
	// Создание менеджера провайдеров
	manager := provider.NewProviderManager(config, mockLogger)
	
	// Тест добавления mock провайдера
	mockProvider := &MockProvider{
		name:    "mock",
		healthy: true,
	}
	manager.AddProvider("mock", mockProvider)
	
	// Проверка добавления
	providers := manager.GetAllProviders()
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}
	
	if _, exists := providers["mock"]; !exists {
		t.Error("Mock provider not found")
	}
	
	// Тест отправки уведомления
	notification := &domain.Notification{
		ID:        "test-123",
		Type:      domain.NotificationTypeIncidentCreated,
		Channel:   "mock",
		Recipient: "test@example.com",
		Subject:   "Test Notification",
		Body:      "Test message",
		TenantID:  "test-tenant",
		Severity: domain.SeverityHigh,
		Status:    domain.NotificationStatusPending,
		CreatedAt: time.Now(),
	}
	
	err := manager.SendNotification(context.Background(), notification)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	// Тест получения статистики
	stats := manager.GetStats()
	if stats["providers_count"] != 1 {
		t.Errorf("Expected providers_count 1, got %v", stats["providers_count"])
	}
	
	// Тест здоровья
	health := manager.CheckHealth(context.Background())
	if !health["mock"] {
		t.Error("Mock provider should be healthy")
	}
	
	if !manager.IsHealthy(context.Background()) {
		t.Error("Manager should be healthy")
	}
	
	// Тест удаления провайдера
	manager.RemoveProvider("mock")
	providers = manager.GetAllProviders()
	if len(providers) != 0 {
		t.Errorf("Expected 0 providers after removal, got %d", len(providers))
	}
}

func TestTelegramProvider(t *testing.T) {
	mockLogger := &MockLogger{}
	
	config := telegram.TelegramConfig{
		BotToken:      "test-token",
		APIURL:        "https://api.telegram.org",
		Timeout:       5 * time.Second,
		RetryAttempts: 2,
	}
	
	provider := telegram.NewTelegramProvider(config, mockLogger)
	
	// Тест базовых методов
	if provider.GetType() != "telegram" {
		t.Errorf("Expected type 'telegram', got '%s'", provider.GetType())
	}
	
	// Тест здоровья (без реального подключения)
	stats := provider.GetStats()
	if stats["type"] != "telegram" {
		t.Errorf("Expected type 'telegram', got %v", stats["type"])
	}
	
	if stats["timeout"] != "5s" {
		t.Errorf("Expected timeout '5s', got %v", stats["timeout"])
	}
	
	if stats["retry_attempts"] != 2 {
		t.Errorf("Expected retry_attempts 2, got %v", stats["retry_attempts"])
	}
}

func TestSlackProvider(t *testing.T) {
	mockLogger := &MockLogger{}
	
	config := slack.SlackConfig{
		BotToken:      "test-token",
		WebhookURL:    "https://hooks.slack.com/test",
		APIURL:        "https://slack.com/api",
		Timeout:       5 * time.Second,
		RetryAttempts: 2,
	}
	
	provider := slack.NewSlackProvider(config, mockLogger)
	
	// Тест базовых методов
	if provider.GetType() != "slack" {
		t.Errorf("Expected type 'slack', got '%s'", provider.GetType())
	}
	
	// Тест здоровья (без реального подключения)
	stats := provider.GetStats()
	if stats["type"] != "slack" {
		t.Errorf("Expected type 'slack', got %v", stats["type"])
	}
	
	if stats["timeout"] != "5s" {
		t.Errorf("Expected timeout '5s', got %v", stats["timeout"])
	}
	
	if stats["retry_attempts"] != 2 {
		t.Errorf("Expected retry_attempts 2, got %v", stats["retry_attempts"])
	}
}

func TestEmailProvider(t *testing.T) {
	mockLogger := &MockLogger{}
	
	config := email.EmailConfig{
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     587,
		Username:     "test@gmail.com",
		Password:     "password",
		FromAddress:  "noreply@example.com",
		FromName:     "Test Service",
		UseStartTLS:  true,
		Timeout:      10 * time.Second,
		RetryAttempts: 2,
	}
	
	provider := email.NewEmailProvider(config, mockLogger)
	
	// Тест базовых методов
	if provider.GetType() != "email" {
		t.Errorf("Expected type 'email', got '%s'", provider.GetType())
	}
	
	// Тест здоровья (без реального подключения)
	stats := provider.GetStats()
	if stats["type"] != "email" {
		t.Errorf("Expected type 'email', got %v", stats["type"])
	}
	
	if stats["timeout"] != "10s" {
		t.Errorf("Expected timeout '10s', got %v", stats["timeout"])
	}
	
	if stats["retry_attempts"] != 2 {
		t.Errorf("Expected retry_attempts 2, got %v", stats["retry_attempts"])
	}
}

func TestRetryManager(t *testing.T) {
	mockLogger := &MockLogger{}
	
	// Тест экспоненциальной backoff
	config := retry.RetryConfig{
		MaxAttempts: 5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}
	
	manager := retry.NewRetryManager(config, mockLogger)
	
	// Тест расчета задержек
	delay1 := manager.GetDelay(1)
	delay2 := manager.GetDelay(2)
	delay3 := manager.GetDelay(3)
	
	if delay1 != 100*time.Millisecond {
		t.Errorf("Expected delay1 100ms, got %v", delay1)
	}
	
	if delay2 != 200*time.Millisecond {
		t.Errorf("Expected delay2 200ms, got %v", delay2)
	}
	
	if delay3 != 400*time.Millisecond {
		t.Errorf("Expected delay3 400ms, got %v", delay3)
	}
	
	// Тест операции с успехом
	successOp := retry.NewRetryOperation("test", func(ctx context.Context) error {
		return nil
	}, nil)
	
	err := manager.Execute(context.Background(), successOp)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	// Тест операции с ошибкой (без retry)
	noRetryOp := retry.NewRetryOperation("test", func(ctx context.Context) error {
		return fmt.Errorf("non-retryable error")
	}, func(err error) bool {
		return false
	})
	
	err = manager.Execute(context.Background(), noRetryOp)
	if err == nil {
		t.Error("Expected error for non-retryable operation")
	}
	
	// Тест статистики
	stats := manager.GetStats()
	if stats["max_attempts"] != 5 {
		t.Errorf("Expected max_attempts 5, got %v", stats["max_attempts"])
	}
	
	if stats["initial_delay"] != "100ms" {
		t.Errorf("Expected initial_delay 100ms, got %v", stats["initial_delay"])
	}
}

func TestProviderIntegration(t *testing.T) {
	mockLogger := &MockLogger{}
	
	// Тестовая конфигурация
	config := provider.DefaultProviderConfig()
	config.Telegram.BotToken = "test-token"
	config.Slack.WebhookURL = "https://hooks.slack.com/test"
	config.Email.SMTPHost = "smtp.test.com"
	config.Email.Username = "test@test.com"
	config.Email.Password = "test"
	
	manager := provider.NewProviderManager(config, mockLogger)
	
	// Тест уведомления для разных каналов
	testCases := []struct {
		channel   string
		recipient string
		expected  bool
	}{
		{domain.ChannelEmail, "test@example.com", false}, // Реальный провайдер, но без SMTP
		{domain.ChannelSlack, "#general", false}, // Реальный провайдер, но без webhook
		{domain.ChannelWebhook, "https://webhook.example.com", false}, // Нет провайдера
		{domain.ChannelSMS, "+1234567890", false}, // Нет провайдера
	}
	
	for _, tc := range testCases {
		notification := &domain.Notification{
			ID:        fmt.Sprintf("test-%s", tc.channel),
			Type:      domain.NotificationTypeIncidentCreated,
			Channel:   tc.channel,
			Recipient: tc.recipient,
			Subject:   "Test",
			Body:      "Test message",
			TenantID:  "test-tenant",
			Severity: domain.SeverityMedium,
			Status:   domain.NotificationStatusPending,
			CreatedAt: time.Now(),
		}
		
		err := manager.SendNotification(context.Background(), notification)
		
		if tc.expected && err == nil {
			t.Errorf("Expected error for channel %s, but got success", tc.channel)
		}
		
		if !tc.expected && err == nil {
			t.Errorf("Expected error for channel %s, but got success", tc.channel)
		}
	}
	
	// Тест статистики
	stats := manager.GetStats()
	providersCount := stats["providers_count"].(int)
	
	if providersCount != 3 {
		t.Errorf("Expected 3 providers, got %d", providersCount)
	}
}

// contains проверяет наличие подстроки
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr ||
			 indexOf(s, substr) >= 0)))
}

// indexOf возвращает индекс подстроки
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
	}
	}
	return -1
}
