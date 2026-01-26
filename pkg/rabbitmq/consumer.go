package rabbitmq

import (
	"context"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// Consumer представляет консьюмера сообщений
type Consumer struct {
	conn     *Connection
	config   *Config
	handlers map[string]MessageHandler
}

// MessageHandler функция для обработки сообщения
type MessageHandler func(context.Context, amqp091.Delivery) error

// NewConsumer создает нового консьюмера
func NewConsumer(conn *Connection, config *Config) *Consumer {
	return &Consumer{
		conn:     conn,
		config:   config,
		handlers: make(map[string]MessageHandler),
	}
}

// RegisterHandler регистрирует обработчик для конкретной очереди
func (c *Consumer) RegisterHandler(queueName string, handler MessageHandler) {
	c.handlers[queueName] = handler
}

// Start запускает консьюмера для всех зарегистрированных очередей
func (c *Consumer) Start(ctx context.Context) error {
	for queueName, handler := range c.handlers {
		// Запускаем обработку для каждой очереди в отдельной горутине
		go func(queue string, h MessageHandler) {
			// Пытаемся запустить обработку с reconnect логикой
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if err := c.consume(ctx, queue, h); err != nil {
						fmt.Printf("Error consuming from queue %s: %v. Reconnecting in %s...\n", queue, err, c.config.ReconnectInterval)
						time.Sleep(c.config.ReconnectInterval)
					}
				}
			}
		}(queueName, handler)
	}

	// Ждем завершения контекста
	<-ctx.Done()
	return ctx.Err()
}

// consume обрабатывает сообщения из очереди
func (c *Consumer) consume(ctx context.Context, queueName string, handler MessageHandler) error {
	// Проверяем, что канал инициализирован
	if c.conn.Channel() == nil {
		return fmt.Errorf("rabbitmq channel is not initialized")
	}

	// Объявляем очередь
	_, err := c.conn.Channel().QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}

	// Привязываем очередь к exchange, если задан
	if c.config.Exchange != "" {
		// Проверяем, что канал инициализирован
		if c.conn.Channel() == nil {
			return fmt.Errorf("rabbitmq channel is not initialized")
		}
		err = c.conn.Channel().QueueBind(
			queueName,
			c.config.RoutingKey,
			c.config.Exchange,
			false,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w", queueName, c.config.Exchange, err)
		}
	}

	// Проверяем, что канал инициализирован
	if c.conn.Channel() == nil {
		return fmt.Errorf("rabbitmq channel is not initialized")
	}

	// Получаем сообщения
	msgs, err := c.conn.Channel().Consume(
		queueName,
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	// Обрабатываем сообщения
	for msg := range msgs {
		// Создаем контекст для обработки сообщения
		msgCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		// Обрабатываем сообщение
		err := handler(msgCtx, msg)

		// Отправляем ack/nack в зависимости от результата
		if err == nil {
			// Успешная обработка - отправляем ack
			if err := msg.Ack(false); err != nil {
				fmt.Printf("Error sending ack for delivery %d: %v\n", msg.DeliveryTag, err)
			}
		} else {
			// Ошибка при обработке - отправляем nack с requeue
			//TODO В реальном приложении здесь может быть логика retry с задержкой
			// или отправка в DLQ после определенного количества попыток

			// Проверяем количество попыток
			retryCount := 0
			if xDeath, ok := msg.Headers["x-death"]; ok {
				if deaths, ok := xDeath.([]interface{}); ok {
					retryCount = len(deaths)
				}
			}

			// Если попыток меньше 3, пробуем снова
			if retryCount < 3 {
				if err := msg.Nack(false, true); err != nil {
					fmt.Printf("Error sending nack with requeue for delivery %d: %v\n", msg.DeliveryTag, err)
				}
			} else {
				// Иначе отправляем в DLQ
				if err := msg.Nack(false, false); err != nil {
					fmt.Printf("Error sending nack without requeue for delivery %d: %v\n", msg.DeliveryTag, err)
				}
			}
		}

		// Завершаем контекст
		cancel()
	}

	// Если канал закрыт, возвращаем ошибку
	select {
	case _, ok := <-msgs:
		if !ok {
			return fmt.Errorf("consumer channel closed")
		}
	default:
		// Канал все еще открыт
	}

	return nil
}

// HealthCheck проверяет состояние подключения к RabbitMQ
func (c *Consumer) HealthCheck(ctx context.Context) error {
	if c.conn == nil || c.conn.conn == nil {
		return fmt.Errorf("rabbitmq connection is not initialized")
	}

	// Пытаемся выполнить простой запрос
	channel, err := c.conn.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}
	return channel.Close()
}
