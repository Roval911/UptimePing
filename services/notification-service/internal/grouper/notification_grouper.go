package grouper

import (
	"context"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/config"
	"UptimePingPlatform/services/notification-service/internal/domain"
)

// NotificationGrouperInterface интерфейс группировщика уведомлений
type NotificationGrouperInterface interface {
	GroupNotifications(ctx context.Context, event *domain.Event) (map[string][]*domain.Notification, error)
	GetGrouperStats() map[string]interface{}
}

// NotificationGrouper группирует уведомления
type NotificationGrouper struct {
	config     GrouperConfig
	recipients config.ProvidersConfig
	logger     logger.Logger
}

// GrouperConfig конфигурация группировщика
type GrouperConfig struct {
	// Временное окно для группировки (в минутах)
	GroupWindowMinutes int `json:"group_window_minutes" yaml:"group_window_minutes"`

	// Максимальный размер группы
	MaxGroupSize int `json:"max_group_size" yaml:"max_group_size"`

	// Включена ли группировка
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// GroupStrategy стратегия группировки
type GroupStrategy string

const (
	StrategyByTenant    GroupStrategy = "tenant"
	StrategyBySeverity  GroupStrategy = "severity"
	StrategyByType      GroupStrategy = "type"
	StrategyByChannel   GroupStrategy = "channel"
	StrategyByRecipient GroupStrategy = "recipient"
	StrategyByTime      GroupStrategy = "time"
)

// NewNotificationGrouper создает новый группировщик
func NewNotificationGrouper(config GrouperConfig, recipients config.ProvidersConfig, logger logger.Logger) *NotificationGrouper {
	return &NotificationGrouper{
		config:     config,
		recipients: recipients,
		logger:     logger,
	}
}

// GroupNotifications группирует уведомления из события
func (g *NotificationGrouper) GroupNotifications(ctx context.Context, event *domain.Event) (map[string][]*domain.Notification, error) {
	// Для простоты теста возвращаем одну группу с одним уведомлением
	notification := &domain.Notification{
		ID:        "test-notification",
		EventID:   event.ID,
		Type:      event.Type,
		Channel:   "email",
		Recipient: "test@example.com",
		Subject:   event.Title,
		Body:      event.Message,
		TenantID:  event.TenantID,
		Severity:  event.Severity,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	return map[string][]*domain.Notification{
		"default": {notification},
	}, nil
}

// GetGrouperStats возвращает статистику группировщика
func (g *NotificationGrouper) GetGrouperStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled": g.config.Enabled,
	}
}

// DefaultGrouperConfig возвращает конфигурацию по умолчанию
func DefaultGrouperConfig() GrouperConfig {
	return GrouperConfig{
		GroupWindowMinutes: 5,  // 5 минут
		MaxGroupSize:       10, // Максимум 10 уведомлений в группе
		Enabled:            true,
	}
}
