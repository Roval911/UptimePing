package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/services/notification-service/internal/domain"
	filter "UptimePingPlatform/services/notification-service/internal/filter"
	grouper "UptimePingPlatform/services/notification-service/internal/grouper"
	processor "UptimePingPlatform/services/notification-service/internal/processor"
)

const (
	// Exchange и очереди для уведомлений
	NotificationsExchange = "notifications"
	NotificationsQueue    = "notification.events"
	NotificationsDLX      = "notifications.dlx"
	NotificationsDLQ      = "notification.events.dlq"

	// Ключи маршрутизации
	RoutingKeyIncidentCreated = "incident.created"
	RoutingKeyIncidentUpdated = "incident.updated"
	RoutingKeyIncidentResolved = "incident.resolved"
	RoutingKeyCheckFailed     = "check.failed"
	RoutingKeyCheckRecovered  = "check.recovered"
)

// Consumer обрабатывает события из RabbitMQ
type Consumer struct {
	conn         *rabbitmq.Connection
	logger       logger.Logger
	filter       filter.EventFilterInterface
	grouper      grouper.NotificationGrouperInterface
	processor    processor.NotificationProcessorInterface
	prefetchCount int
}

// Config конфигурация consumer
type Config struct {
	URL             string        `json:"url" yaml:"url"`
	Exchange        string        `json:"exchange" yaml:"exchange"`
	Queue           string        `json:"queue" yaml:"queue"`
	DLX             string        `json:"dlx" yaml:"dlx"`
	DLQ             string        `json:"dlq" yaml:"dlq"`
	PrefetchCount   int           `json:"prefetch_count" yaml:"prefetch_count"`
	ReconnectDelay  time.Duration `json:"reconnect_delay" yaml:"reconnect_delay"`
	MaxRetries      int           `json:"max_retries" yaml:"max_retries"`
	ProcessTimeout  time.Duration `json:"process_timeout" yaml:"process_timeout"`
}

// NewConfig создает конфигурацию по умолчанию
func NewConfig() *Config {
	rabbitmqURL := "amqp://guest:guest@localhost:5672/"
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		rabbitmqURL = url
	}
	
	return &Config{
		URL:            rabbitmqURL,
		Exchange:       NotificationsExchange,
		Queue:          NotificationsQueue,
		DLX:            NotificationsDLX,
		DLQ:            NotificationsDLQ,
		PrefetchCount:  10,
		ReconnectDelay: 5 * time.Second,
		MaxRetries:     3,
		ProcessTimeout: 30 * time.Second,
	}
}

// NewNotificationConsumer создает новый consumer
func NewNotificationConsumer(
	conn *rabbitmq.Connection,
	filter filter.EventFilterInterface,
	grouper grouper.NotificationGrouperInterface,
	processor processor.NotificationProcessorInterface,
	logger logger.Logger,
) *Consumer {
	return &Consumer{
		conn:          conn,
		logger:        logger,
		filter:        filter,
		grouper:       grouper,
		processor:     processor,
		prefetchCount: 10,
	}
}

// Start запускает consumer
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("Starting notification consumer",
		logger.String("exchange", NotificationsExchange),
		logger.String("queue", NotificationsQueue),
		logger.Int("prefetch_count", c.prefetchCount),
	)

	// Настройка канала
	ch := c.conn.Channel()
	defer ch.Close()

	var err error
	// Установка QoS
	err = ch.Qos(
		c.prefetchCount, // prefetch count
		0,               // prefetch size
		false,           // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Объявление exchange
	err = ch.ExchangeDeclare(
		NotificationsExchange, // name
		"topic",               // type
		true,                  // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Объявление DLX
	err = ch.ExchangeDeclare(
		NotificationsDLX, // name
		"direct",        // type
		true,            // durable
		false,           // auto-deleted
		false,           // internal
		false,           // no-wait
		nil,             // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLX: %w", err)
	}

	// Объявление очереди с DLX
	args := amqp.Table{
		"x-dead-letter-exchange":    NotificationsDLX,
		"x-dead-letter-routing-key": NotificationsDLQ,
		"x-message-ttl":             int64((24 * time.Hour).Seconds()), // 24 часа TTL
	}

	q, err := ch.QueueDeclare(
		NotificationsQueue, // name
		true,              // durable
		false,             // auto-deleted
		false,             // exclusive
		false,             // no-wait
		args,              // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Объявление DLQ
	_, err = ch.QueueDeclare(
		NotificationsDLQ, // name
		true,            // durable
		false,           // auto-deleted
		false,           // exclusive
		false,           // no-wait
		nil,             // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// Биндинг очередей к exchange
	routingKeys := []string{
		RoutingKeyIncidentCreated,
		RoutingKeyIncidentUpdated,
		RoutingKeyIncidentResolved,
		RoutingKeyCheckFailed,
		RoutingKeyCheckRecovered,
	}

	for _, routingKey := range routingKeys {
		err = ch.QueueBind(
			q.Name,               // queue name
			routingKey,           // routing key
			NotificationsExchange, // exchange
			false,                // no-wait
			nil,                  // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange: %w", routingKey, err)
		}
	}

	// Биндинг DLQ к DLX
	err = ch.QueueBind(
		NotificationsDLQ, // queue name
		NotificationsDLQ, // routing key
		NotificationsDLX, // exchange
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind DLQ to DLX: %w", err)
	}

	// Подписка на сообщения
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack (ручное подтверждение)
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	c.logger.Info("Notification consumer started successfully")

	// Обработка сообщений
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Context cancelled, stopping consumer")
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				c.logger.Warn("Consumer channel closed")
				return fmt.Errorf("consumer channel closed")
			}

			err := c.processMessage(ctx, msg)
			if err != nil {
				c.logger.Error("Failed to process message",
					logger.Error(err),
					logger.String("message_id", msg.MessageId),
					logger.String("routing_key", msg.RoutingKey),
				)

				// Отправка в DLQ при ошибке
				if err := c.sendToDLQ(ch, msg, err); err != nil {
					c.logger.Error("Failed to send message to DLQ",
						logger.Error(err),
						logger.String("message_id", msg.MessageId),
					)
				}

				// Отклонение сообщения
				if err := msg.Nack(false, false); err != nil {
					c.logger.Error("Failed to nack message",
						logger.Error(err),
						logger.String("message_id", msg.MessageId),
					)
				}
				continue
			}

			// Подтверждение успешной обработки
			if err := msg.Ack(false); err != nil {
				c.logger.Error("Failed to ack message",
					logger.Error(err),
					logger.String("message_id", msg.MessageId),
				)
			}
		}
	}
}

// processMessage обрабатывает одно сообщение
func (c *Consumer) processMessage(ctx context.Context, msg amqp.Delivery) error {
	startTime := time.Now()

	c.logger.Debug("Processing message",
		logger.String("message_id", msg.MessageId),
		logger.String("routing_key", msg.RoutingKey),
		logger.String("content_type", msg.ContentType),
	)

	// Парсинг события
	event, err := c.parseEvent(msg)
	if err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	// Фильтрация события
	if !c.filter.ShouldProcess(event) {
		c.logger.Debug("Event filtered out",
			logger.String("event_id", event.ID),
			logger.String("event_type", event.Type),
			logger.String("severity", event.Severity),
		)
		return nil
	}

	// Группировка уведомлений
	groups, err := c.grouper.GroupNotifications(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to group notifications: %w", err)
	}

	// Обработка каждой группы
	for groupID, notifications := range groups {
		c.logger.Debug("Processing notification group",
			logger.String("group_id", groupID),
			logger.Int("notification_count", len(notifications)),
		)

		// Обработка группы уведомлений
		err := c.processor.ProcessGroup(ctx, groupID, notifications)
		if err != nil {
			return fmt.Errorf("failed to process notification group %s: %w", groupID, err)
		}
	}

	c.logger.Info("Message processed successfully",
		logger.String("message_id", msg.MessageId),
		logger.String("event_id", event.ID),
		logger.Duration("processing_time", time.Since(startTime)),
	)

	return nil
}

// parseEvent парсит событие из сообщения
func (c *Consumer) parseEvent(msg amqp.Delivery) (*domain.Event, error) {
	var event domain.Event

	// Определение типа события по routing key
	eventType := c.getEventTypeFromRoutingKey(msg.RoutingKey)
	if eventType == "" {
		return nil, fmt.Errorf("unknown routing key: %s", msg.RoutingKey)
	}

	event.Type = eventType

	// Парсинг тела сообщения
	switch msg.ContentType {
	case "application/json":
		if err := json.Unmarshal(msg.Body, &event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported content type: %s", msg.ContentType)
	}

	// Установка метаданных
	event.Metadata = make(map[string]interface{})
	for key, value := range msg.Headers {
		if str, ok := value.(string); ok {
			event.Metadata[key] = str
		} else {
			event.Metadata[key] = fmt.Sprintf("%v", value)
		}
	}

	// Установка временных меток
	event.Timestamp = time.Now()
	if timestamp, ok := msg.Headers["timestamp"].(string); ok {
		if parsedTime, err := time.Parse(time.RFC3339, timestamp); err == nil {
			event.Timestamp = parsedTime
		}
	}

	return &event, nil
}

// getEventTypeFromRoutingKey определяет тип события по routing key
func (c *Consumer) getEventTypeFromRoutingKey(routingKey string) string {
	switch routingKey {
	case RoutingKeyIncidentCreated:
		return "incident.created"
	case RoutingKeyIncidentUpdated:
		return "incident.updated"
	case RoutingKeyIncidentResolved:
		return "incident.resolved"
	case RoutingKeyCheckFailed:
		return "check.failed"
	case RoutingKeyCheckRecovered:
		return "check.recovered"
	default:
		return ""
	}
}

// sendToDLQ отправляет сообщение в Dead Letter Queue
func (c *Consumer) sendToDLQ(ch *amqp.Channel, msg amqp.Delivery, processErr error) error {
	// Добавление информации об ошибке в заголовки
	headers := amqp.Table{}
	for k, v := range msg.Headers {
		headers[k] = v
	}
	headers["x-death-reason"] = processErr.Error()
	headers["x-death-timestamp"] = time.Now().Format(time.RFC3339)
	headers["original-routing-key"] = msg.RoutingKey
	headers["original-message-id"] = msg.MessageId

	// Публикация в DLQ
	err := ch.Publish(
		NotificationsDLX, // exchange
		NotificationsDLQ, // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType:   msg.ContentType,
			Body:          msg.Body,
			Headers:       headers,
			Timestamp:     time.Now(),
			MessageId:     msg.MessageId + "-dlq",
			CorrelationId: msg.CorrelationId,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish to DLQ: %w", err)
	}

	c.logger.Info("Message sent to DLQ",
		logger.String("original_message_id", msg.MessageId),
		logger.String("error", processErr.Error()),
	)

	return nil
}

// Stop останавливает consumer
func (c *Consumer) Stop() error {
	c.logger.Info("Stopping notification consumer")
	return nil
}

// GetStats возвращает статистику работы consumer
func (c *Consumer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"prefetch_count": c.prefetchCount,
		"exchange":       NotificationsExchange,
		"queue":          NotificationsQueue,
		"status":         "running",
	}
}
