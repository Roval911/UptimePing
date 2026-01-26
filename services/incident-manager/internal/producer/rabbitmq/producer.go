package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/services/incident-manager/internal/domain"
)

// IncidentEvent представляет событие инцидента
type IncidentEvent struct {
	EventType    string                 `json:"event_type"`    // incident.opened, incident.updated, incident.resolved, incident.grouped
	Timestamp   time.Time              `json:"timestamp"`    // Время события
	Service     string                 `json:"service"`      // incident-manager
	IncidentID  string                 `json:"incident_id"`  // ID инцидента
	CheckID     string                 `json:"check_id"`     // ID проверки
	TenantID    string                 `json:"tenant_id"`    // ID тенанта
	Status      domain.IncidentStatus  `json:"status"`       // Статус инцидента
	Severity    domain.IncidentSeverity `json:"severity"`     // Уровень серьезности
	Count       int                    `json:"count"`        // Количество повторений
	Duration    int64                  `json:"duration"`     // Длительность в миллисекундах
	ErrorMessage string                `json:"error_message,omitempty"` // Сообщение об ошибке
	ErrorHash   string                 `json:"error_hash,omitempty"`   // Хеш ошибки
	FirstSeen   time.Time              `json:"first_seen"`   // Время первого появления
	LastSeen    time.Time              `json:"last_seen"`    // Время последнего появления
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // Дополнительные метаданные
}

// IncidentProducer публикует события инцидентов в RabbitMQ
type IncidentProducer struct {
	conn    *rabbitmq.Connection
	channel *amqp.Channel
	config  *IncidentProducerConfig
	logger  logger.Logger
}

// NewIncidentProducer создает новый producer для событий инцидентов
func NewIncidentProducer(conn *rabbitmq.Connection, config *IncidentProducerConfig, logger logger.Logger) (*IncidentProducer, error) {
	producer := &IncidentProducer{
		conn:   conn,
		config: config,
		logger: logger,
	}

	err := producer.setupChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to setup channel: %w", err)
	}

	err = producer.setupExchange()
	if err != nil {
		return nil, fmt.Errorf("failed to setup exchange: %w", err)
	}

	return producer, nil
}

// setupChannel создает и настраивает канал RabbitMQ
func (p *IncidentProducer) setupChannel() error {
	p.channel = p.conn.Channel()

	// Устанавливаем QoS для гарантированной доставки
	err := p.channel.Qos(
		p.config.PrefetchCount,
		p.config.PrefetchSize,
		p.config.Global,
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	return nil
}

// setupExchange создает exchange для событий инцидентов
func (p *IncidentProducer) setupExchange() error {
	err := p.channel.ExchangeDeclare(
		p.config.Exchange,   // имя exchange
		"topic",             // тип exchange
		true,                // durable
		false,               // auto-delete
		false,               // internal
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	return nil
}

// PublishIncidentEvent публикует событие инцидента
func (p *IncidentProducer) PublishIncidentEvent(ctx context.Context, eventType string, incident *domain.Incident, result *CheckResult) error {
	if incident == nil {
		return fmt.Errorf("incident cannot be nil")
	}

	// Создаем событие
	event := &IncidentEvent{
		EventType:    eventType,
		Timestamp:   time.Now(),
		Service:     "incident-manager",
		IncidentID:  incident.ID,
		CheckID:     incident.CheckID,
		TenantID:    incident.TenantID,
		Status:      incident.Status,
		Severity:    incident.Severity,
		Count:       incident.Count,
		Duration:    calculateDuration(result),
		ErrorMessage: incident.ErrorMessage,
		ErrorHash:   incident.ErrorHash,
		FirstSeen:   incident.FirstSeen,
		LastSeen:    incident.LastSeen,
		Metadata:    incident.Metadata,
	}

	// Добавляем специфичные для типа события поля
	switch eventType {
	case "incident.opened":
		// Для открытия инцидента все поля уже заполнены
	case "incident.updated":
		// Для обновления добавляем информацию об изменениях
		if incident.Metadata != nil {
			if escalationHistory, ok := incident.Metadata["escalation_history"]; ok {
				event.Metadata["escalation_history"] = escalationHistory
			}
		}
	case "incident.resolved":
		// Для закрытия добавляем длительность инцидента
		event.Metadata["incident_duration"] = incident.GetDuration().String()
	case "incident.grouped":
		// Для группировки добавляем информацию о сгруппированных ошибках
		if incident.Metadata != nil {
			if groupedErrors, ok := incident.Metadata["grouped_errors"]; ok {
				event.Metadata["grouped_errors"] = groupedErrors
			}
		}
	}

	// Сериализуем событие в JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("Failed to marshal incident event",
			logger.String("event_type", eventType),
			logger.String("incident_id", incident.ID),
			logger.Error(err))
		return fmt.Errorf("failed to marshal incident event: %w", err)
	}

	// Определяем routing key
	routingKey := fmt.Sprintf("incident.%s.%s.%s", 
		eventType, 
		incident.TenantID, 
		incident.Severity)

	// Публикуем событие
	err = p.channel.Publish(
		p.config.Exchange, // exchange
		routingKey,       // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Headers: amqp.Table{
				"event_type":    eventType,
				"incident_id":   incident.ID,
				"check_id":       incident.CheckID,
				"tenant_id":      incident.TenantID,
				"severity":       string(incident.Severity),
				"status":         string(incident.Status),
				"service":        "incident-manager",
				"timestamp":      time.Now().Unix(),
			},
			Timestamp: time.Now(),
		},
	)
	if err != nil {
		p.logger.Error("Failed to publish incident event",
			logger.String("event_type", eventType),
			logger.String("incident_id", incident.ID),
			logger.String("routing_key", routingKey),
			logger.Error(err))
		return fmt.Errorf("failed to publish incident event: %w", err)
	}

	p.logger.Info("Incident event published successfully",
		logger.String("event_type", eventType),
		logger.String("incident_id", incident.ID),
		logger.String("routing_key", routingKey),
		logger.Int("event_size", len(eventData)))

	return nil
}

// PublishIncidentEventWithRetry публикует событие с retry логикой
func (p *IncidentProducer) PublishIncidentEventWithRetry(ctx context.Context, eventType string, incident *domain.Incident, result *CheckResult) error {
	maxRetries := 3
	initialDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := p.PublishIncidentEvent(ctx, eventType, incident, result)
		if err == nil {
			return nil
		}

		if attempt < maxRetries-1 {
			delay := time.Duration(attempt+1) * initialDelay
			p.logger.Warn("Failed to publish incident event, retrying",
				logger.String("event_type", eventType),
				logger.String("incident_id", incident.ID),
				logger.Int("attempt", attempt+1),
				logger.Int("max_retries", maxRetries),
				logger.Duration("delay", delay),
				logger.Error(err))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Продолжаем retry
			}
		}
	}

	return fmt.Errorf("failed to publish incident event after %d attempts", maxRetries)
}

// calculateDuration вычисляет длительность в миллисекундах
func calculateDuration(result *CheckResult) int64 {
	if result == nil {
		return 0
	}
	return result.Duration.Milliseconds()
}

// Close закрывает producer
func (p *IncidentProducer) Close() error {
	if p.channel != nil && !p.channel.IsClosed() {
		err := p.channel.Close()
		if err != nil {
			p.logger.Error("Failed to close channel", logger.Error(err))
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}
	return nil
}

// IsConnected проверяет состояние подключения
func (p *IncidentProducer) IsConnected() bool {
	return p.conn != nil && p.conn.Channel() != nil && !p.conn.Channel().IsClosed() && p.channel != nil && !p.channel.IsClosed()
}
