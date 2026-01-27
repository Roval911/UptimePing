package rabbitmq

import (
	"context"

	"UptimePingPlatform/pkg/errors"
	pkg_rabbitmq "UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/pkg/logger"
	"github.com/rabbitmq/amqp091-go"
)

// CheckServiceInterface определяет интерфейс для сервиса проверок
type CheckServiceInterface interface {
	ProcessTask(ctx context.Context, message []byte) error
}

// Consumer представляет RabbitMQ consumer для обработки задач
type Consumer struct {
	logger       logger.Logger
	checkService CheckServiceInterface
	queueName    string
	consumerTag  string
	rabbitConsumer *pkg_rabbitmq.Consumer
	rabbitConn     *pkg_rabbitmq.Connection
	done         chan bool
}

// ConsumerConfig конфигурация consumer'а
type ConsumerConfig struct {
	QueueName   string
	ConsumerTag string
}

// NewConsumer создает новый consumer
func NewConsumer(
	config ConsumerConfig,
	log logger.Logger,
	checkService CheckServiceInterface,
	rabbitConn *pkg_rabbitmq.Connection,
) (*Consumer, error) {
	if rabbitConn == nil {
		return nil, errors.New(errors.ErrValidation, "rabbitmq connection is required")
	}

	// Создаем конфигурацию для RabbitMQ consumer
	rabbitConfig := pkg_rabbitmq.NewConfig()
	rabbitConfig.Queue = config.QueueName
	rabbitConfig.PrefetchCount = 10 // Количество сообщений для предварительной загрузки

	// Создаем RabbitMQ consumer
	rabbitConsumer := pkg_rabbitmq.NewConsumer(rabbitConn, rabbitConfig)

	consumer := &Consumer{
		logger:          log,
		checkService:    checkService,
		queueName:       config.QueueName,
		consumerTag:     config.ConsumerTag,
		rabbitConsumer:  rabbitConsumer,
		rabbitConn:      rabbitConn,
		done:            make(chan bool),
	}

	// Регистрируем обработчик сообщений
	messageHandler := consumer.createMessageHandler()
	rabbitConsumer.RegisterHandler(config.QueueName, messageHandler)

	consumer.logger.Info("Consumer created",
		logger.String("queue", config.QueueName),
		logger.String("consumer_tag", config.ConsumerTag),
		logger.Int("prefetch_count", rabbitConfig.PrefetchCount),
	)

	return consumer, nil
}

// Start запускает обработку сообщений через RabbitMQ consumer
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("Starting RabbitMQ consumer",
		logger.String("queue", c.queueName),
		logger.String("consumer_tag", c.consumerTag),
	)

	// Запускаем RabbitMQ consumer
	if err := c.rabbitConsumer.Start(ctx); err != nil {
		c.logger.Error("Failed to start RabbitMQ consumer",
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to start rabbitmq consumer")
	}

	c.logger.Info("RabbitMQ consumer started successfully")
	return nil
}

// createMessageHandler создает обработчик сообщений для RabbitMQ
func (c *Consumer) createMessageHandler() pkg_rabbitmq.MessageHandler {
	return func(ctx context.Context, delivery amqp091.Delivery) error {
		c.logger.Debug("Received message from RabbitMQ",
			logger.String("message_id", delivery.MessageId),
			logger.String("routing_key", delivery.RoutingKey),
			logger.String("exchange", delivery.Exchange),
			logger.Int("body_size", len(delivery.Body)),
			logger.String("correlation_id", delivery.CorrelationId),
		)

		// Обрабатываем сообщение через CheckService
		err := c.checkService.ProcessTask(ctx, delivery.Body)
		if err != nil {
			c.logger.Error("Failed to process message",
				logger.String("message_id", delivery.MessageId),
				logger.Error(err),
			)
			
			// Отклоняем сообщение (NACK) с requeue для повторной обработки
			if delivery.Nack(false, true) != nil {
				c.logger.Error("Failed to NACK message",
					logger.String("message_id", delivery.MessageId),
					logger.Error(err),
				)
			}
			return errors.Wrap(err, errors.ErrInternal, "failed to process message")
		}

		// Подтверждаем успешную обработку (ACK)
		if err := delivery.Ack(false); err != nil {
			c.logger.Error("Failed to ACK message",
				logger.String("message_id", delivery.MessageId),
				logger.Error(err),
			)
			return errors.Wrap(err, errors.ErrInternal, "failed to acknowledge message")
		}

		c.logger.Debug("Message processed successfully",
			logger.String("message_id", delivery.MessageId),
		)

		return nil
	}
}

// ProcessMessage обрабатывает одно сообщение
func (c *Consumer) ProcessMessage(ctx context.Context, message []byte) error {
	c.logger.Info("Processing message",
		logger.Int("size", len(message)),
	)

	// Обработка сообщения через CheckService
	err := c.checkService.ProcessTask(ctx, message)
	if err != nil {
		c.logger.Error("Failed to process message",
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to process message")
	}

	c.logger.Info("Message processed successfully")
	return nil
}

// Close закрывает consumer
func (c *Consumer) Close() error {
	c.logger.Info("Closing RabbitMQ consumer",
		logger.String("queue", c.queueName),
		logger.String("consumer_tag", c.consumerTag),
	)

	// Сигнал о завершении
	close(c.done)

	// Закрываем RabbitMQ consumer (если есть метод Close)
	// В pkg/rabbitmq consumer закрывается через контекст
	
	c.logger.Info("RabbitMQ consumer closed")
	return nil
}

// GetStats возвращает статистику consumer'а
func (c *Consumer) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["queue_name"] = c.queueName
	stats["consumer_tag"] = c.consumerTag

	// Проверяем закрыт ли канал без блокировки
	select {
	case <-c.done:
		stats["closed"] = true
		stats["status"] = "closed"
	default:
		stats["closed"] = false
		stats["status"] = "running"
	}

	// Добавляем статистику из RabbitMQ consumer если доступно
	if c.rabbitConn != nil && c.rabbitConn.Channel() != nil {
		stats["rabbitmq_connected"] = true
		stats["rabbitmq_channel_open"] = true
	} else {
		stats["rabbitmq_connected"] = false
		stats["rabbitmq_channel_open"] = false
	}

	return stats
}

// HealthCheck проверяет состояние consumer'а
func (c *Consumer) HealthCheck(ctx context.Context) error {
	if c.rabbitConn == nil {
		return errors.New(errors.ErrInternal, "rabbitmq connection is nil")
	}

	// Проверяем что канал открыт
	if c.rabbitConn.Channel() == nil {
		return errors.New(errors.ErrInternal, "rabbitmq channel is nil")
	}

	// Проверяем что consumer не закрыт
	select {
	case <-c.done:
		return errors.New(errors.ErrInternal, "consumer is closed")
	default:
		// Consumer работает
	}

	return nil
}
