package processor

import (
	"context"
	"fmt"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/domain"
)

// NotificationProcessorInterface интерфейс процессора уведомлений
type NotificationProcessorInterface interface {
	ProcessGroup(ctx context.Context, groupID string, notifications []*domain.Notification) error
	GetProcessorStats() map[string]interface{}
}

// NotificationProcessor обрабатывает группы уведомлений
type NotificationProcessor struct {
	config    ProcessorConfig
	logger    logger.Logger
	senders   map[string]NotificationSender
	templates TemplateManager
}

// ProcessorConfig конфигурация процессора
type ProcessorConfig struct {
	// Таймаут обработки группы
	GroupTimeout time.Duration `json:"group_timeout" yaml:"group_timeout"`
	
	// Параллельная обработка
	ParallelProcessing bool `json:"parallel_processing" yaml:"parallel_processing"`
	
	// Максимальное количество воркеров
	MaxWorkers int `json:"max_workers" yaml:"max_workers"`
	
	// Интервал между retry попытками
	RetryInterval time.Duration `json:"retry_interval" yaml:"retry_interval"`
	
	// Включить обработку
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// NotificationSender интерфейс для отправки уведомлений
type NotificationSender interface {
	Send(ctx context.Context, notification *domain.Notification) error
	GetType() string
	IsHealthy(ctx context.Context) bool
}

// TemplateManager интерфейс для управления шаблонами
type TemplateManager interface {
	RenderTemplate(templateName string, data map[string]interface{}) (string, error)
	GetSubjectTemplate(eventType string) string
	GetBodyTemplate(eventType, channel string) string
}

// NewNotificationProcessor создает новый процессор
func NewNotificationProcessor(
	config ProcessorConfig,
	logger logger.Logger,
	providerManager interface {
		SendNotification(ctx context.Context, notification *domain.Notification) error
	},
	templates TemplateManager,
) *NotificationProcessor {
	// Создаем адаптер для провайдеров
	senders := make(map[string]NotificationSender)
	
	// Создаем адаптер для providerManager
	senders["provider_manager"] = &ProviderManagerAdapter{
		providerManager: providerManager,
	}
	
	return &NotificationProcessor{
		config:    config,
		logger:    logger,
		senders:   senders,
		templates: templates,
	}
}

// ProcessGroup обрабатывает группу уведомлений
func (p *NotificationProcessor) ProcessGroup(ctx context.Context, groupID string, notifications []*domain.Notification) error {
	p.logger.Info("Processing notification group",
		logger.String("group_id", groupID),
		logger.Int("notifications_count", len(notifications)),
	)

	// Проверяем, включен ли процессор
	if !p.config.Enabled {
		p.logger.Warn("Processor is disabled, skipping group")
		return nil
	}

	// Создаем контекст с таймаутом
	groupCtx, cancel := context.WithTimeout(ctx, p.config.GroupTimeout)
	defer cancel()

	// Обработка уведомлений
	if p.config.ParallelProcessing {
		return p.processGroupParallel(groupCtx, notifications)
	} else {
		return p.processGroupSequential(groupCtx, notifications)
	}
}

// processGroupSequential обрабатывает группу последовательно
func (p *NotificationProcessor) processGroupSequential(ctx context.Context, notifications []*domain.Notification) error {
	for _, notification := range notifications {
		if err := p.processNotification(ctx, notification); err != nil {
			p.logger.Error("Failed to process notification",
				logger.Error(err),
				logger.String("notification_id", notification.ID),
			)
			return err
		}
	}
	return nil
}

// processGroupParallel обрабатывает группу параллельно
func (p *NotificationProcessor) processGroupParallel(ctx context.Context, notifications []*domain.Notification) error {
	// Создаем воркеры
	workerPool := make(chan struct {
		notification *domain.Notification
		err         error
	}, len(notifications))

	// Запускаем воркеры
	for i := 0; i < p.config.MaxWorkers && i < len(notifications); i++ {
		go p.worker(ctx, workerPool)
	}

	// Отправляем уведомления в воркеры
	for _, notification := range notifications {
		select {
		case workerPool <- struct {
			notification *domain.Notification
			err         error
		}{notification: notification, err: nil}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Закрываем воркеры
	close(workerPool)

	// Собираем результаты
	var errors []error
	for i := 0; i < len(notifications); i++ {
		result := <-workerPool
		if result.err != nil {
			errors = append(errors, result.err)
		}
	}

	// Если есть ошибки, возвращаем первую
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// worker обрабатывает уведомления в воркере
func (p *NotificationProcessor) worker(ctx context.Context, workerPool chan struct {
	notification *domain.Notification
	err         error
}) {
	for result := range workerPool {
		result.err = p.processNotification(ctx, result.notification)
		workerPool <- result
	}
}

// processNotification обрабатывает одно уведомление
func (p *NotificationProcessor) processNotification(ctx context.Context, notification *domain.Notification) error {
	p.logger.Debug("Processing notification",
		logger.String("notification_id", notification.ID),
		logger.String("channel", notification.Channel),
		logger.String("recipient", notification.Recipient),
	)

	// Проверяем срок действия уведомления
	if notification.IsExpired() {
		p.logger.Warn("Notification expired, skipping",
			logger.String("notification_id", notification.ID),
			logger.String("created_at", notification.CreatedAt.Format(time.RFC3339)),
		)
		return nil
	}

	// Получаем отправщика для канала
	sender, exists := p.senders["provider_manager"]
	if !exists {
		p.logger.Warn("No sender found for provider manager",
			logger.String("channel", notification.Channel),
			logger.String("notification_id", notification.ID),
		)
		return fmt.Errorf("no sender found for provider manager")
	}

	// Отправляем уведомление
	if err := sender.Send(ctx, notification); err != nil {
		p.logger.Error("Failed to send notification",
			logger.Error(err),
			logger.String("notification_id", notification.ID),
			logger.String("sender_type", sender.GetType()),
		)
		return err
	}

	p.logger.Info("Notification processed successfully",
		logger.String("notification_id", notification.ID),
		logger.String("sender_type", sender.GetType()),
	)

	return nil
}

// GetProcessorStats возвращает статистику процессора
func (p *NotificationProcessor) GetProcessorStats() map[string]interface{} {
	// Статистика отправщиков
	senderStats := make(map[string]interface{})
	for name, sender := range p.senders {
		senderStats[name] = map[string]interface{}{
			"type":    sender.GetType(),
			"healthy": sender.IsHealthy(context.Background()),
		}
	}

	return map[string]interface{}{
		"config": map[string]interface{}{
			"group_timeout":        p.config.GroupTimeout.String(),
			"parallel_processing":   p.config.ParallelProcessing,
			"max_workers":          p.config.MaxWorkers,
			"retry_interval":       p.config.RetryInterval.String(),
			"enabled":             p.config.Enabled,
		},
		"senders":              senderStats,
		"retry_interval":       p.config.RetryInterval.String(),
	}
}

// DefaultProcessorConfig возвращает конфигурацию по умолчанию
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		GroupTimeout:        30 * time.Second,
		ParallelProcessing: false,
		MaxWorkers:          5,
		RetryInterval:       5 * time.Second,
		Enabled:             true,
	}
}

// ProductionProcessorConfig возвращает конфигурацию для production
func ProductionProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		GroupTimeout:        60 * time.Second,
		ParallelProcessing: true,
		MaxWorkers:          10,
		RetryInterval:       10 * time.Second,
		Enabled:             true,
	}
}

// ProviderManagerAdapter адаптер для интеграции ProviderManager с NotificationProcessor
type ProviderManagerAdapter struct {
	providerManager interface {
		SendNotification(ctx context.Context, notification *domain.Notification) error
	}
}

// Send отправляет уведомление через менеджер провайдеров
func (p *ProviderManagerAdapter) Send(ctx context.Context, notification *domain.Notification) error {
	return p.providerManager.SendNotification(ctx, notification)
}

// GetType возвращает тип адаптера
func (p *ProviderManagerAdapter) GetType() string {
	return "provider_manager"
}

// IsHealthy проверяет здоровье менеджера провайдеров
func (p *ProviderManagerAdapter) IsHealthy(ctx context.Context) bool {
	// Проверяем здоровье через интерфейс, если он поддерживается
	if healthyChecker, ok := p.providerManager.(interface {
		IsHealthy(ctx context.Context) bool
	}); ok {
		return healthyChecker.IsHealthy(ctx)
	}
	return true // По умолчанию считаем здоровым
}
