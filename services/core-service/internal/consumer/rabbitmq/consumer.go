package rabbitmq

import (
	"context"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
)

// CheckServiceInterface определяет интерфейс для сервиса проверок
type CheckServiceInterface interface {
	ProcessTask(ctx context.Context, message []byte) error
}

// Consumer представляет упрощенный consumer для обработки задач
type Consumer struct {
	logger       logger.Logger
	checkService CheckServiceInterface
	queueName    string
	consumerTag  string
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
) (*Consumer, error) {
	consumer := &Consumer{
		logger:       log,
		checkService: checkService,
		queueName:    config.QueueName,
		consumerTag:  config.ConsumerTag,
		done:         make(chan bool),
	}

	consumer.logger.Info("Consumer created",
		logger.String("queue", config.QueueName),
		logger.String("consumer_tag", config.ConsumerTag),
	)

	return consumer, nil
}

// Start запускает обработку сообщений (в реальной реализации это будет RabbitMQ)
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("Starting consumer",
		logger.String("queue", c.queueName),
	)

	//todo В реальной реализации здесь будет подключение к RabbitMQ
	// и запуск обработки сообщений
	// Для демонстрации просто возвращаем успех
	return nil
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
	c.logger.Info("Closing consumer")

	// Сигнал о завершении
	close(c.done)

	c.logger.Info("Consumer closed")
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

	return stats
}
