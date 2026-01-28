package rabbitmq_test

import (
	"context"
	"testing"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/config"
	"UptimePingPlatform/services/notification-service/internal/domain"
	filter "UptimePingPlatform/services/notification-service/internal/filter"
	grouper "UptimePingPlatform/services/notification-service/internal/grouper"
	processor "UptimePingPlatform/services/notification-service/internal/processor"
	"UptimePingPlatform/services/notification-service/internal/sender"
	"UptimePingPlatform/services/notification-service/internal/template"
	rabbitmq "UptimePingPlatform/services/notification-service/internal/consumer/rabbitmq"
)

// MockLogger для тестов
type MockLogger struct{}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {}
func (m *MockLogger) Info(msg string, fields ...logger.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...logger.Field)  {}
func (m *MockLogger) Error(msg string, fields ...logger.Field) {}
func (m *MockLogger) With(fields ...logger.Field) logger.Logger { return m }
func (m *MockLogger) Sync() error                             { return nil }

// MockProviderManager для тестов
type MockProviderManager struct{}

func (m *MockProviderManager) SendNotification(ctx context.Context, notification *domain.Notification) error {
	// Mock implementation - всегда успешно
	return nil
}

func TestNewNotificationConsumer(t *testing.T) {
	// Создание mock компонентов
	mockLogger := &MockLogger{}
	mockFilter := filter.NewEventFilter(filter.DefaultFilterConfig(), mockLogger)
	
	// Создаем пустую конфигурацию получателей для теста
	recipientsConfig := config.DefaultProvidersConfig()
	mockGrouper := grouper.NewNotificationGrouper(grouper.DefaultGrouperConfig(), recipientsConfig, mockLogger)
	
	// Создание mock менеджера провайдеров
	mockProviderManager := &MockProviderManager{}
	
	// Создание mock менеджера шаблонов
	mockTemplateManager := template.NewMockTemplateManager()
	
	// Создание процессора
	mockProcessor := processor.NewNotificationProcessor(
		processor.DefaultProcessorConfig(),
		mockLogger,
		mockProviderManager,
		mockTemplateManager,
	)

	// Тест создания consumer
	consumer := rabbitmq.NewNotificationConsumer(
		nil, // RabbitMQ connection будет nil для теста
		mockFilter,
		mockGrouper,
		mockProcessor,
		mockLogger,
	)

	if consumer == nil {
		t.Fatal("Expected consumer to be created")
	}
}

func TestConfig(t *testing.T) {
	config := rabbitmq.NewConfig()

	if config.URL != "amqp://guest:guest@localhost:5672/" {
		t.Errorf("Expected default URL, got %s", config.URL)
	}

	if config.Exchange != "notifications" {
		t.Errorf("Expected default exchange, got %s", config.Exchange)
	}

	if config.Queue != "notification.events" {
		t.Errorf("Expected default queue, got %s", config.Queue)
	}

	if config.PrefetchCount != 10 {
		t.Errorf("Expected default prefetch count 10, got %d", config.PrefetchCount)
	}
}

func TestEventFilter(t *testing.T) {
	mockLogger := &MockLogger{}
	eventFilter := filter.NewEventFilter(filter.DefaultFilterConfig(), mockLogger)

	// Тест фильтрации включенных событий
	event := &domain.Event{
		ID:       "test-123",
		Type:     domain.NotificationTypeIncidentCreated,
		Severity: domain.SeverityHigh,
		TenantID: "test-tenant",
		Source:   "test-source",
		Title:    "Test Incident",
		Message:  "Test message",
		Data:     make(map[string]interface{}),
		Metadata: make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	shouldProcess := eventFilter.ShouldProcess(event)
	if !shouldProcess {
		t.Error("Expected event to be processed")
	}
}

func TestNotificationGrouper(t *testing.T) {
	mockLogger := &MockLogger{}
	grouperConfig := grouper.DefaultGrouperConfig()
	
	// Создаем пустую конфигурацию получателей для теста
	recipientsConfig := config.DefaultProvidersConfig()
	notificationGrouper := grouper.NewNotificationGrouper(grouperConfig, recipientsConfig, mockLogger)

	// Тест группировки уведомлений
	event := &domain.Event{
		ID:       "test-123",
		Type:     domain.NotificationTypeIncidentCreated,
		Severity: domain.SeverityHigh,
		TenantID: "test-tenant",
		Source:   "test-source",
		Title:    "Test Incident",
		Message:  "Test message",
		Data:     make(map[string]interface{}),
		Metadata: make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	groups, err := notificationGrouper.GroupNotifications(ctx, event)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(groups) == 0 {
		t.Error("Expected at least one group")
	}

	// Проверка статистики
	stats := notificationGrouper.GetGrouperStats()
	if stats["enabled"] != grouperConfig.Enabled {
		t.Error("Expected enabled config to match")
	}
}

func TestNotificationProcessor(t *testing.T) {
	mockLogger := &MockLogger{}
	
	// Создание mock менеджера провайдеров
	mockProviderManager := &MockProviderManager{}
	
	// Создание mock менеджера шаблонов
	mockTemplateManager := template.NewMockTemplateManager()
	
	// Создание процессора
	processorConfig := processor.DefaultProcessorConfig()
	notificationProcessor := processor.NewNotificationProcessor(
		processorConfig,
		mockLogger,
		mockProviderManager,
		mockTemplateManager,
	)

	// Создание тестового уведомления
	notification := &domain.Notification{
		ID:          "test-notification-123",
		EventID:     "test-event-123",
		Type:        domain.NotificationTypeIncidentCreated,
		Channel:     domain.ChannelEmail,
		Recipient:   "test@example.com",
		Subject:     "Test Incident",
		Body:        "Test message",
		TenantID:    "test-tenant",
		Severity:    domain.SeverityHigh,
		Status:      domain.NotificationStatusPending,
		Data:        make(map[string]interface{}),
		Metadata:    make(map[string]interface{}),
		CreatedAt:   time.Now(),
		RetryCount:  0,
		MaxRetries:  3,
	}

	// Тест обработки группы
	ctx := context.Background()
	groupID := "test-group"
	notifications := []*domain.Notification{notification}

	err := notificationProcessor.ProcessGroup(ctx, groupID, notifications)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Проверка статистики
	stats := notificationProcessor.GetProcessorStats()
	configStats := stats["config"].(map[string]interface{})
	if configStats["enabled"] != processorConfig.Enabled {
		t.Error("Expected enabled config to match")
	}
}

func TestTemplateManager(t *testing.T) {
	mockLogger := &MockLogger{}
	templateManager := template.NewDefaultTemplateManager(mockLogger)

	// Тест рендеринга шаблона
	data := map[string]interface{}{
		"notification": map[string]interface{}{
			"type":     "incident.created",
			"severity": "high",
			"title":    "Test Incident",
		},
	}

	result, err := templateManager.RenderTemplate("subject:incident.created", data)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Тест получения имен шаблонов
	subjectTemplate := templateManager.GetSubjectTemplate("incident.created")
	expected := "subject:incident.created"
	if subjectTemplate != expected {
		t.Errorf("Expected %s, got %s", expected, subjectTemplate)
	}

	bodyTemplate := templateManager.GetBodyTemplate("incident.created", "email")
	expected = "body:incident.created:email"
	if bodyTemplate != expected {
		t.Errorf("Expected %s, got %s", expected, bodyTemplate)
	}
}

func TestNotificationSenders(t *testing.T) {
	mockLogger := &MockLogger{}
	senderFactory := sender.NewSenderFactory(mockLogger)

	// Тест создания отправщиков
	senders := senderFactory.CreateSenders()

	expectedChannels := []string{
		domain.ChannelEmail,
		domain.ChannelSlack,
		domain.ChannelSMS,
		domain.ChannelWebhook,
	}

	for _, channel := range expectedChannels {
		sender, exists := senders[channel]
		if !exists {
			t.Errorf("Expected sender for channel %s", channel)
		}

		if sender.GetType() != channel {
			t.Errorf("Expected sender type %s, got %s", channel, sender.GetType())
		}

		// Тест здоровья отправщика
		ctx := context.Background()
		if !sender.IsHealthy(ctx) {
			t.Errorf("Expected sender %s to be healthy", channel)
		}
	}

	// Тест mock отправщиков
	mockSenders := senderFactory.CreateMockSenders()
	for _, channel := range expectedChannels {
		sender, exists := mockSenders[channel]
		if !exists {
			t.Errorf("Expected mock sender for channel %s", channel)
		}

		// Тест отправки
		notification := &domain.Notification{
			ID:        "test-123",
			Channel:   channel,
			Recipient: "test@example.com",
			Subject:   "Test",
			Body:      "Test message",
		}

		ctx := context.Background()
		err := sender.Send(ctx, notification)
		if err != nil {
			t.Errorf("Expected no error sending mock notification, got %v", err)
		}
	}
}
