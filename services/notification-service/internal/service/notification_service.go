package service

import (
	"context"
	"fmt"
	"time"

	"UptimePingPlatform/pkg/logger"
)

// NotificationService предоставляет бизнес-логику для работы с уведомлениями
type NotificationService interface {
	// SendNotification отправляет уведомление через указанные каналы
	SendNotification(ctx context.Context, notification *Notification) ([]*SendResult, error)
	
	// RegisterChannel регистрирует новый канал уведомлений
	RegisterChannel(ctx context.Context, channel *Channel) (*Channel, error)
	
	// UnregisterChannel удаляет канал уведомлений
	UnregisterChannel(ctx context.Context, channelID string) error
	
	// ListChannels возвращает список каналов уведомлений
	ListChannels(ctx context.Context, tenantID string, channelType ChannelType) ([]*Channel, error)
}

// Notification представляет уведомление
type Notification struct {
	TenantID   string            `json:"tenant_id"`
	IncidentID string            `json:"incident_id"`
	Severity   NotificationSeverity `json:"severity"`
	Title      string            `json:"title"`
	Message    string            `json:"message"`
	ChannelIDs []string          `json:"channel_ids"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// NotificationSeverity определяет серьезность уведомления
type NotificationSeverity int32

const (
	NotificationSeverityInfo NotificationSeverity = iota
	NotificationSeverityWarning
	NotificationSeverityError
	NotificationSeverityCritical
)

// Channel представляет канал уведомлений
type Channel struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenant_id"`
	Type      ChannelType       `json:"type"`
	Name      string            `json:"name"`
	Config    map[string]string `json:"config"`
	IsActive  bool              `json:"is_active"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// ChannelType определяет тип канала уведомлений
type ChannelType int32

const (
	ChannelTypeUnspecified ChannelType = iota
	ChannelTypeTelegram
	ChannelTypeSlack
	ChannelTypeEmail
)

// SendResult содержит результат отправки в конкретный канал
type SendResult struct {
	ChannelID string `json:"channel_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// notificationService реализация NotificationService
type notificationService struct {
	logger logger.Logger
	// Здесь можно добавить зависимости: репозитории, клиенты для отправки и т.д.
}

// NewNotificationService создает новый экземпляр NotificationService
func NewNotificationService(logger logger.Logger) NotificationService {
	return &notificationService{
		logger: logger,
	}
}

// SendNotification отправляет уведомление через указанные каналы
func (s *notificationService) SendNotification(ctx context.Context, notification *Notification) ([]*SendResult, error) {
	s.logger.Info("Sending notification",
		logger.String("tenant_id", notification.TenantID),
		logger.String("incident_id", notification.IncidentID),
		logger.Int("severity", int(notification.Severity)),
		logger.String("title", notification.Title),
		logger.String("channel_ids", fmt.Sprintf("%v", notification.ChannelIDs)))

	results := make([]*SendResult, 0, len(notification.ChannelIDs))

	// Если каналы не указаны, отправляем во все активные каналы тенанта
	channelIDs := notification.ChannelIDs
	if len(channelIDs) == 0 {
		channels, err := s.ListChannels(ctx, notification.TenantID, ChannelTypeUnspecified)
		if err != nil {
			return nil, fmt.Errorf("failed to get channels: %w", err)
		}

		for _, channel := range channels {
			if channel.IsActive {
				channelIDs = append(channelIDs, channel.ID)
			}
		}
	}

	// Отправка в каждый канал
	for _, channelID := range channelIDs {
		result := &SendResult{
			ChannelID: channelID,
			Success:   true, // Для простоты всегда успешная отправка
		}

		// Здесь будет реальная логика отправки в зависимости от типа канала
		err := s.sendToChannel(ctx, channelID, notification)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		}

		results = append(results, result)
	}

	s.logger.Info("Notification sent",
		logger.String("tenant_id", notification.TenantID),
		logger.Int("channels_sent", len(results)))

	return results, nil
}

// RegisterChannel регистрирует новый канал уведомлений
func (s *notificationService) RegisterChannel(ctx context.Context, channel *Channel) (*Channel, error) {
	s.logger.Info("Registering channel",
		logger.String("tenant_id", channel.TenantID),
		logger.String("name", channel.Name),
		logger.Int("type", int(channel.Type)))

	// Генерируем ID для нового канала
	channel.ID = fmt.Sprintf("channel_%s_%d", channel.TenantID, time.Now().Unix())
	channel.CreatedAt = time.Now().Format(time.RFC3339)
	channel.UpdatedAt = time.Now().Format(time.RFC3339)

	// Здесь будет реальная логика сохранения в базу данных
	s.logger.Info("Channel registered successfully",
		logger.String("channel_id", channel.ID))

	return channel, nil
}

// UnregisterChannel удаляет канал уведомлений
func (s *notificationService) UnregisterChannel(ctx context.Context, channelID string) error {
	s.logger.Info("Unregistering channel",
		logger.String("channel_id", channelID))

	// Здесь будет реальная логика удаления из базы данных
	s.logger.Info("Channel unregistered successfully",
		logger.String("channel_id", channelID))

	return nil
}

// ListChannels возвращает список каналов уведомлений
func (s *notificationService) ListChannels(ctx context.Context, tenantID string, channelType ChannelType) ([]*Channel, error) {
	s.logger.Info("Listing channels",
		logger.String("tenant_id", tenantID),
		logger.Int("type", int(channelType)))

	// Здесь будет реальная логика получения из базы данных
	// Для примера вернем несколько тестовых каналов
	channels := []*Channel{
		{
			ID:        "channel_1",
			TenantID:  tenantID,
			Type:      ChannelTypeEmail,
			Name:      "Email Notifications",
			Config: map[string]string{
				"smtp_host": "smtp.example.com",
				"smtp_port": "587",
			},
			IsActive:  true,
			CreatedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			UpdatedAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		},
		{
			ID:        "channel_2",
			TenantID:  tenantID,
			Type:      ChannelTypeSlack,
			Name:      "Slack Webhook",
			Config: map[string]string{
				"webhook_url": "https://hooks.slack.com/services/...",
			},
			IsActive:  true,
			CreatedAt: time.Now().Add(-12 * time.Hour).Format(time.RFC3339),
			UpdatedAt: time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
		},
	}

	// Фильтрация по типу если указан
	if channelType != ChannelTypeUnspecified {
		filtered := make([]*Channel, 0)
		for _, channel := range channels {
			if channel.Type == channelType {
				filtered = append(filtered, channel)
			}
		}
		channels = filtered
	}

	s.logger.Info("Channels listed successfully",
		logger.String("tenant_id", tenantID),
		logger.Int("count", len(channels)))

	return channels, nil
}

// sendToChannel отправляет уведомление в конкретный канал
func (s *notificationService) sendToChannel(ctx context.Context, channelID string, notification *Notification) error {
	s.logger.Debug("Sending to channel",
		logger.String("channel_id", channelID),
		logger.String("title", notification.Title))

	// Здесь будет реальная логика отправки в зависимости от типа канала
	// Для примера просто логируем
	s.logger.Info("Notification sent to channel",
		logger.String("channel_id", channelID),
		logger.String("title", notification.Title),
		logger.String("message", notification.Message))

	return nil
}
