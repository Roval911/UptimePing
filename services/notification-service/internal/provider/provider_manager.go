package provider

import (
	"context"
	"fmt"
	"time"

	"UptimePingPlatform/pkg/logger"
	pkg_logger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/domain"
	"UptimePingPlatform/services/notification-service/internal/provider/email"
	"UptimePingPlatform/services/notification-service/internal/provider/retry"
	"UptimePingPlatform/services/notification-service/internal/provider/slack"
	"UptimePingPlatform/services/notification-service/internal/provider/telegram"
)

// NotificationProvider интерфейс для всех провайдеров уведомлений
type NotificationProvider interface {
	Send(ctx context.Context, notification *domain.Notification) error
	GetType() string
	IsHealthy(ctx context.Context) bool
	GetStats() map[string]interface{}
}

// ProviderManager управляет всеми провайдерами уведомлений
type ProviderManager struct {
	providers map[string]NotificationProvider
	logger    logger.Logger
	retryMgr  *retry.RetryManager
}

// ProviderConfig конфигурация провайдеров
type ProviderConfig struct {
	Telegram telegram.TelegramConfig `json:"telegram" yaml:"telegram"`
	Slack    slack.SlackConfig    `json:"slack" yaml:"slack"`
	Email    email.EmailConfig    `json:"email" yaml:"email"`
	Retry    retry.RetryConfig    `json:"retry" yaml:"retry"`
}

// NewProviderManager создает новый менеджер провайдеров
func NewProviderManager(config ProviderConfig, logger logger.Logger) *ProviderManager {
	retryMgr := retry.NewRetryManager(config.Retry, logger)
	
	manager := &ProviderManager{
		providers: make(map[string]NotificationProvider),
		logger:    logger,
		retryMgr:  retryMgr,
	}

	// Инициализация провайдеров
	if config.Telegram.BotToken != "" {
		manager.providers["telegram"] = telegram.NewTelegramProvider(config.Telegram, logger)
	}

	if config.Slack.BotToken != "" || config.Slack.WebhookURL != "" {
		manager.providers["slack"] = slack.NewSlackProvider(config.Slack, logger)
	}

	if config.Email.SMTPHost != "" && config.Email.Username != "" {
		manager.providers["email"] = email.NewEmailProvider(config.Email, logger)
	}

	manager.logger.Info("Provider manager initialized",
		pkg_logger.Int("providers_count", len(manager.providers)),
	)

	return manager
}

// SendNotification отправляет уведомление через все подходящие провайдеры
func (pm *ProviderManager) SendNotification(ctx context.Context, notification *domain.Notification) error {
	pm.logger.Info("Sending notification",
		logger.String("notification_id", notification.ID),
		logger.String("channel", notification.Channel),
		logger.String("recipient", notification.Recipient),
	)

	// Определение провайдера на основе канала
	provider, exists := pm.getProvider(notification.Channel)
	if !exists {
		return fmt.Errorf("no provider found for channel: %s", notification.Channel)
	}

	// Создание retry операции
	operation := retry.NewRetryOperation(
		fmt.Sprintf("send_%s_notification", notification.Channel),
		func(ctx context.Context) error {
			return provider.Send(ctx, notification)
		},
		func(err error) bool {
			// Проверяем, нужно ли повторять попытку для этого типа ошибки
			return pm.shouldRetryProvider(notification.Channel, err)
		},
	)

	// Выполнение с retry логикой
	err := pm.retryMgr.Execute(ctx, operation)
	if err != nil {
		pm.logger.Error("Failed to send notification after retries",
			logger.Error(err),
			logger.String("notification_id", notification.ID),
			logger.String("provider", provider.GetType()),
		)
		return fmt.Errorf("failed to send notification: %w", err)
	}

	pm.logger.Info("Notification sent successfully",
		logger.String("notification_id", notification.ID),
		logger.String("provider", provider.GetType()),
	)

	return nil
}

// getProvider возвращает провайдера для указанного канала
func (pm *ProviderManager) getProvider(channel string) (NotificationProvider, bool) {
	provider, exists := pm.providers[channel]
	return provider, exists
}

// GetAllProviders возвращает все доступные провайдеры
func (pm *ProviderManager) GetAllProviders() map[string]NotificationProvider {
	return pm.providers
}

// GetProviderStats возвращает статистику всех провайдеров
func (pm *ProviderManager) GetProviderStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	for name, provider := range pm.providers {
		stats[name] = provider.GetStats()
	}
	
	stats["retry_manager"] = pm.retryMgr.GetStats()
	stats["total_providers"] = len(pm.providers)
	
	return stats
}

// CheckHealth проверяет здоровье всех провайдеров
func (pm *ProviderManager) CheckHealth(ctx context.Context) map[string]bool {
	health := make(map[string]bool)
	
	for name, provider := range pm.providers {
		health[name] = provider.IsHealthy(ctx)
	}
	
	return health
}

// AddProvider добавляет новый провайдер
func (pm *ProviderManager) AddProvider(name string, provider NotificationProvider) {
	pm.providers[name] = provider
	pm.logger.Info("Provider added",
		logger.String("name", name),
		logger.String("type", provider.GetType()),
	)
}

// RemoveProvider удаляет провайдер
func (pm *ProviderManager) RemoveProvider(name string) {
	if _, exists := pm.providers[name]; exists {
		delete(pm.providers, name)
		pm.logger.Info("Provider removed",
			logger.String("name", name),
		)
	}
}

// IsHealthy проверяет здоровье менеджера провайдеров
func (pm *ProviderManager) IsHealthy(ctx context.Context) bool {
	health := pm.CheckHealth(ctx)
	
	// Менеджер считается здоровым, если хотя бы один провайдер здоров
	for _, isHealthy := range health {
		if isHealthy {
			return true
		}
	}
	
	// Или если нет провайдеров (система работает без них)
	return len(pm.providers) == 0
}

// GetStats возвращает статистику менеджера провайдеров
func (pm *ProviderManager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":             "provider_manager",
		"providers_count":  len(pm.providers),
		"healthy":          pm.IsHealthy(context.Background()),
		"providers":        pm.GetProviderStats(),
	}
}

// shouldRetryProvider проверяет, нужно ли повторять попытку для провайдера
func (pm *ProviderManager) shouldRetryProvider(providerType string, err error) bool {
	// Здесь можно добавить специфичную логику для разных типов провайдеров
	switch providerType {
	case "telegram":
		// Telegram ошибки
		errStr := err.Error()
		return !contains(errStr, "chat not found") &&
		       !contains(errStr, "bot token invalid") &&
		       !contains(errStr, "forbidden")
	case "slack":
		// Slack ошибки
		errStr := err.Error()
		return !contains(errStr, "channel_not_found") &&
		       !contains(errStr, "invalid_auth") &&
		       !contains(errStr, "not_in_channel")
	case "email":
		// Email ошибки
		errStr := err.Error()
		return !contains(errStr, "invalid_address") &&
		       !contains(errStr, "user_unknown") &&
		       !contains(errStr, "authentication failed")
	default:
		// По умолчанию используем общую логику
		return retry.IsRetryableError(err)
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

// DefaultProviderConfig возвращает конфигурацию по умолчанию
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		Telegram: telegram.TelegramConfig{
			APIURL:        "https://api.telegram.org",
			Timeout:       30 * time.Second,
			RetryAttempts: 3,
		},
		Slack: slack.SlackConfig{
			APIURL:        "https://slack.com/api",
			Timeout:       30 * time.Second,
			RetryAttempts: 3,
		},
		Email: email.EmailConfig{
			SMTPPort:     587,
			UseStartTLS:  true,
			Timeout:      30 * time.Second,
			RetryAttempts: 3,
		},
		Retry: retry.DefaultRetryConfig(),
	}
}

// DevelopmentProviderConfig возвращает конфигурацию для разработки
func DevelopmentProviderConfig() ProviderConfig {
	config := DefaultProviderConfig()
	
	// Для разработки используем mock провайдеры или более мягкие настройки
	config.Retry.MaxAttempts = 1 // Без retry для быстрой отладки
	config.Retry.Jitter = false
	
	return config
}

// ProductionProviderConfig возвращает конфигурацию для production
func ProductionProviderConfig() ProviderConfig {
	config := DefaultProviderConfig()
	
	// Более строгие настройки для production
	config.Retry.MaxAttempts = 5
	config.Retry.MaxDelay = 60 * time.Second
	config.Retry.Jitter = true
	config.Retry.JitterRange = 0.2
	
	// Email настройки для production
	config.Email.SMTPPort = 465
	config.Email.UseTLS = true
	config.Email.UseStartTLS = false
	config.Email.InsecureSkipVerify = false
	
	return config
}
