package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	"UptimePingPlatform/pkg/logger"
	pkgRabbitmq "UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/services/incident-manager/internal/service"
	amqp "github.com/rabbitmq/amqp091-go"
)

// IncidentConsumer обрабатывает результаты проверок из RabbitMQ
type IncidentConsumer struct {
	conn            *pkgRabbitmq.Connection
	channel         *amqp.Channel
	queue           string
	exchange        string
	routingKey      string
	incidentService service.IncidentService
	logger          logger.Logger
}

// NewIncidentConsumer создает новый consumer для инцидентов
func NewIncidentConsumer(conn *pkgRabbitmq.Connection, incidentService service.IncidentService, logger logger.Logger) (*IncidentConsumer, error) {
	channel := conn.Channel()
	if channel == nil {
		return nil, fmt.Errorf("failed to get RabbitMQ channel")
	}

	return &IncidentConsumer{
		conn:            conn,
		channel:         channel,
		queue:           "check.results",
		exchange:        "checks",
		routingKey:      "check.result",
		incidentService: incidentService,
		logger:          logger,
	}, nil
}

// Setup настраивает consumer
func (c *IncidentConsumer) Setup() error {
	// Объявляем exchange
	err := c.channel.ExchangeDeclare(
		c.exchange, // name
		"topic",    // type
		true,       // durable
		false,      // auto-deleted
		false,      // internal
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Объявляем queue
	q, err := c.channel.QueueDeclare(
		c.queue, // name
		true,    // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue к exchange
	err = c.channel.QueueBind(
		q.Name,       // queue name
		c.routingKey, // routing key
		c.exchange,   // exchange
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	return nil
}

// Start начинает потребление сообщений
func (c *IncidentConsumer) Start(ctx context.Context) error {
	// Устанавливаем QoS
	err := c.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Начинаем потребление
	msgs, err := c.channel.Consume(
		c.queue, // queue
		"",      // consumer
		false,   // auto-ack
		false,   // exclusive
		false,   // no-local
		false,   // no-wait
		nil,     // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	c.logger.Info("Incident consumer started",
		logger.String("queue", c.queue),
		logger.String("exchange", c.exchange),
		logger.String("routing_key", c.routingKey))

	// Обрабатываем сообщения в горутине
	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("Incident consumer stopping")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.logger.Info("Consumer channel closed")
					return
				}

				if err := c.handleMessage(msg); err != nil {
					c.logger.Error("Failed to handle message", logger.Error(err))
					// В зависимости от стратегии можно отклонить или ACK сообщение
					msg.Nack(false, false) // reject, don't requeue
				} else {
					msg.Ack(false) // acknowledge
				}
			}
		}
	}()

	return nil
}

// handleMessage обрабатывает одно сообщение
func (c *IncidentConsumer) handleMessage(msg amqp.Delivery) error {
	var checkResult service.CheckResult
	if err := json.Unmarshal(msg.Body, &checkResult); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	c.logger.Info("Processing check result",
		logger.String("check_id", checkResult.CheckID),
		logger.String("tenant_id", checkResult.TenantID),
		logger.Bool("is_success", checkResult.IsSuccess))

	// Обрабатываем результат проверки через сервис инцидентов
	err := c.incidentService.ProcessCheckResultEvent(context.Background(), &checkResult)
	if err != nil {
		return fmt.Errorf("failed to process check result: %w", err)
	}

	c.logger.Info("Check result processed successfully",
		logger.String("check_id", checkResult.CheckID))

	return nil
}

// Close закрывает consumer
func (c *IncidentConsumer) Close() error {
	if c.channel != nil && !c.channel.IsClosed() {
		if err := c.channel.Close(); err != nil {
			c.logger.Error("Failed to close RabbitMQ channel", logger.Error(err))
			return err
		}
	}
	return nil
}
