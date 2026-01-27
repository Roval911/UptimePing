package grouper

import (
	"context"
	"fmt"
	"strings"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/domain"
)

// NotificationGrouperInterface интерфейс группировщика уведомлений
type NotificationGrouperInterface interface {
	GroupNotifications(ctx context.Context, event *domain.Event) (map[string][]*domain.Notification, error)
	GetGrouperStats() map[string]interface{}
}

// NotificationGrouper группирует уведомления
type NotificationGrouper struct {
	config GrouperConfig
	logger logger.Logger
}

// GrouperConfig конфигурация группировщика
type GrouperConfig struct {
	// Временное окно для группировки (в минутах)
	GroupWindowMinutes int `json:"group_window_minutes" yaml:"group_window_minutes"`

	// Максимальный размер группы
	MaxGroupSize int `json:"max_group_size" yaml:"max_group_size"`

	// Включить группировку
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Стратегии группировки
	Strategies []string `json:"strategies" yaml:"strategies"`
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
func NewNotificationGrouper(config GrouperConfig, logger logger.Logger) *NotificationGrouper {
	return &NotificationGrouper{
		config: config,
		logger: logger,
	}
}

// GroupNotifications группирует уведомления из события
func (g *NotificationGrouper) GroupNotifications(ctx context.Context, event *domain.Event) (map[string][]*domain.Notification, error) {
	if !g.config.Enabled {
		// Если группировка отключена, создаем отдельное уведомление для каждого канала
		return g.createIndividualNotifications(ctx, event)
	}

	// Создаем базовые уведомления из события
	notifications := g.createNotificationsFromEvent(ctx, event)

	// Группируем уведомления
	groups := make(map[string][]*domain.Notification)

	for _, notification := range notifications {
		groupKey := g.getGroupKey(notification)

		// Добавляем в существующую группу или создаем новую
		if _, exists := groups[groupKey]; !exists {
			groups[groupKey] = []*domain.Notification{}
		}

		// Проверяем размер группы
		if len(groups[groupKey]) >= g.config.MaxGroupSize {
			// Если группа переполнена, создаем новую с суффиксом
			suffix := 1
			newGroupKey := fmt.Sprintf("%s_%d", groupKey, suffix)
			for groups[newGroupKey] != nil {
				suffix++
				newGroupKey = fmt.Sprintf("%s_%d", groupKey, suffix)
			}
			groupKey = newGroupKey
		}

		groups[groupKey] = append(groups[groupKey], notification)
	}

	// Логируем результат группировки
	g.logger.Debug("Notifications grouped",
		logger.String("event_id", event.ID),
		logger.Int("total_notifications", len(notifications)),
		logger.Int("groups_count", len(groups)),
	)

	return groups, nil
}

// createIndividualNotifications создает отдельные уведомления для каждого канала
func (g *NotificationGrouper) createIndividualNotifications(ctx context.Context, event *domain.Event) (map[string][]*domain.Notification, error) {
	notifications := g.createNotificationsFromEvent(ctx, event)
	groups := make(map[string][]*domain.Notification)

	for _, notification := range notifications {
		groupKey := fmt.Sprintf("%s:%s:%s",
			notification.TenantID,
			notification.Channel,
			notification.Recipient)
		groups[groupKey] = []*domain.Notification{notification}
	}

	return groups, nil
}

// createNotificationsFromEvent создает уведомления из события
func (g *NotificationGrouper) createNotificationsFromEvent(ctx context.Context, event *domain.Event) []*domain.Notification {
	// Определяем каналы для уведомлений
	channels := g.getChannelsForEvent(event)

	var notifications []*domain.Notification

	for _, channel := range channels {
		// Определяем получателей для канала
		recipients := g.getRecipientsForChannel(ctx, event, channel)

		for _, recipient := range recipients {
			notification := &domain.Notification{
				ID:         g.generateNotificationID(event.ID, channel, recipient),
				EventID:    event.ID,
				Type:       event.Type,
				Channel:    channel,
				Recipient:  recipient,
				Subject:    g.generateSubject(event),
				Body:       g.generateBody(event),
				TenantID:   event.TenantID,
				Severity:   event.Severity,
				Status:     domain.NotificationStatusPending,
				Data:       event.Data,
				Metadata:   event.Metadata,
				CreatedAt:  time.Now(),
				RetryCount: 0,
				MaxRetries: 3,
			}

			notifications = append(notifications, notification)
		}
	}

	return notifications
}

// getChannelsForEvent определяет каналы для события
func (g *NotificationGrouper) getChannelsForEvent(event *domain.Event) []string {
	var channels []string

	// Базовые каналы для всех событий
	channels = append(channels, domain.ChannelEmail)

	// Дополнительные каналы в зависимости от серьезности
	switch event.Severity {
	case domain.SeverityCritical:
		channels = append(channels, domain.ChannelSlack, domain.ChannelSMS)
	case domain.SeverityHigh:
		channels = append(channels, domain.ChannelSlack)
	case domain.SeverityMedium:
		// Только email для medium
	default:
		// Только email для low
	}

	// Webhook для определенных типов событий
	if event.Type == domain.NotificationTypeIncidentCreated ||
		event.Type == domain.NotificationTypeIncidentResolved {
		channels = append(channels, domain.ChannelWebhook)
	}

	return channels
}

// getRecipientsForChannel определяет получателей для канала
func (g *NotificationGrouper) getRecipientsForChannel(ctx context.Context, event *domain.Event, channel string) []string {
	//todo Здесь должна быть логика определения получателей из конфигурации или БД
	// Для примера используем базовую логику

	switch channel {
	case domain.ChannelEmail:
		return []string{
			fmt.Sprintf("admin@%s.com", event.TenantID),
			fmt.Sprintf("ops@%s.com", event.TenantID),
		}
	case domain.ChannelSlack:
		return []string{
			fmt.Sprintf("#alerts-%s", event.TenantID),
			fmt.Sprintf("#incidents-%s", event.TenantID),
		}
	case domain.ChannelSMS:
		return []string{
			"+1234567890", // Администратор
		}
	case domain.ChannelWebhook:
		return []string{
			fmt.Sprintf("https://webhook.%s.com/notifications", event.TenantID),
		}
	default:
		return []string{}
	}
}

// generateSubject генерирует тему уведомления
func (g *NotificationGrouper) generateSubject(event *domain.Event) string {
	severityIcon := g.getSeverityIcon(event.Severity)

	switch event.Type {
	case domain.NotificationTypeIncidentCreated:
		return fmt.Sprintf("%s [INCIDENT] %s", severityIcon, event.Title)
	case domain.NotificationTypeIncidentUpdated:
		return fmt.Sprintf("%s [INCIDENT UPDATE] %s", severityIcon, event.Title)
	case domain.NotificationTypeIncidentResolved:
		return fmt.Sprintf("%s [RESOLVED] %s", severityIcon, event.Title)
	case domain.NotificationTypeCheckFailed:
		return fmt.Sprintf("%s [CHECK FAILED] %s", severityIcon, event.Title)
	case domain.NotificationTypeCheckRecovered:
		return fmt.Sprintf("%s [RECOVERED] %s", severityIcon, event.Title)
	default:
		return fmt.Sprintf("%s [%s] %s", severityIcon, strings.ToUpper(event.Type), event.Title)
	}
}

// generateBody генерирует тело уведомления
func (g *NotificationGrouper) generateBody(event *domain.Event) string {
	return fmt.Sprintf(`
Event: %s
Severity: %s
Source: %s
Time: %s

Message:
%s

Additional Information:
Tenant ID: %s
Event ID: %s
`,
		event.Type,
		event.Severity,
		event.Source,
		event.Timestamp.Format(time.RFC3339),
		event.Message,
		event.TenantID,
		event.ID,
	)
}

// getSeverityIcon возвращает иконку для уровня серьезности
func (g *NotificationGrouper) getSeverityIcon(severity string) string {
	switch severity {
	case domain.SeverityCritical:
		return "🔴"
	case domain.SeverityHigh:
		return "🟠"
	case domain.SeverityMedium:
		return "🟡"
	case domain.SeverityLow:
		return "🟢"
	default:
		return "ℹ️"
	}
}

// getGroupKey возвращает ключ для группировки
func (g *NotificationGrouper) getGroupKey(notification *domain.Notification) string {
	var keyParts []string

	// Применяем стратегии группировки
	for _, strategy := range g.config.Strategies {
		switch GroupStrategy(strategy) {
		case StrategyByTenant:
			keyParts = append(keyParts, notification.TenantID)
		case StrategyBySeverity:
			keyParts = append(keyParts, notification.Severity)
		case StrategyByType:
			keyParts = append(keyParts, notification.Type)
		case StrategyByChannel:
			keyParts = append(keyParts, notification.Channel)
		case StrategyByRecipient:
			keyParts = append(keyParts, notification.Recipient)
		case StrategyByTime:
			// Группировка по временному окну
			timeWindow := time.Duration(g.config.GroupWindowMinutes) * time.Minute
			timeSlot := notification.CreatedAt.Truncate(timeWindow)
			keyParts = append(keyParts, timeSlot.Format("2006-01-02-15:04"))
		}
	}

	if len(keyParts) == 0 {
		// Стратегия по умолчанию
		keyParts = []string{notification.TenantID, notification.Channel, notification.Severity}
	}

	return strings.Join(keyParts, ":")
}

// generateNotificationID генерирует ID уведомления
func (g *NotificationGrouper) generateNotificationID(eventID, channel, recipient string) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%s-%s-%d", eventID, channel, recipient, timestamp)
}

// GetGrouperStats возвращает статистику группировщика
func (g *NotificationGrouper) GetGrouperStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":              g.config.Enabled,
		"group_window_minutes": g.config.GroupWindowMinutes,
		"max_group_size":       g.config.MaxGroupSize,
		"strategies":           g.config.Strategies,
	}
}

// DefaultGrouperConfig возвращает конфигурацию по умолчанию
func DefaultGrouperConfig() GrouperConfig {
	return GrouperConfig{
		GroupWindowMinutes: 5,  // 5 минут
		MaxGroupSize:       10, // Максимум 10 уведомлений в группе
		Enabled:            true,
		Strategies: []string{
			string(StrategyByTenant),
			string(StrategyByChannel),
			string(StrategyBySeverity),
		},
	}
}

// ProductionGrouperConfig возвращает конфигурацию для production
func ProductionGrouperConfig() GrouperConfig {
	return GrouperConfig{
		GroupWindowMinutes: 10, // 10 минут
		MaxGroupSize:       20, // Максимум 20 уведомлений в группе
		Enabled:            true,
		Strategies: []string{
			string(StrategyByTenant),
			string(StrategyByChannel),
			string(StrategyBySeverity),
			string(StrategyByTime),
		},
	}
}

// DevelopmentGrouperConfig возвращает конфигурацию для разработки
func DevelopmentGrouperConfig() GrouperConfig {
	return GrouperConfig{
		GroupWindowMinutes: 1,     // 1 минута
		MaxGroupSize:       5,     // Максимум 5 уведомлений в группе
		Enabled:            false, // Отключена для разработки
		Strategies: []string{
			string(StrategyByTenant),
			string(StrategyByChannel),
		},
	}
}
