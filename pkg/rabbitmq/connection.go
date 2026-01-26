package rabbitmq

import (
	"context"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// Connection представляет подключение к RabbitMQ
type Connection struct {
	conn    *amqp091.Connection
	channel *amqp091.Channel
}

// Config представляет конфигурацию RabbitMQ
type Config struct {
	URL        string
	Exchange   string
	RoutingKey string
	Queue      string
	DLX        string // Dead Letter Exchange
	DLQ        string // Dead Letter Queue
	// Connection settings
	ReconnectInterval time.Duration
	MaxRetries        int
	// Consumer settings
	PrefetchCount int
	PrefetchSize  int
	Global        bool
}

// NewConfig создает конфигурацию по умолчанию
func NewConfig() *Config {
	return &Config{
		URL:               "amqp://guest:guest@localhost:5672/",
		Exchange:          "",
		RoutingKey:        "",
		Queue:             "",
		DLX:               "dlx",
		DLQ:               "dlq",
		ReconnectInterval: 5 * time.Second,
		MaxRetries:        3,
		PrefetchCount:     1,
		PrefetchSize:      0,
		Global:            false,
	}
}

// Connect устанавливает подключение к RabbitMQ с retry логикой
func Connect(ctx context.Context, config *Config) (*Connection, error) {
	var lastErr error

	// Пытаемся подключиться с retry
	for i := 0; i <= config.MaxRetries; i++ {
		// Создаем подключение
		conn, err := amqp091.Dial(config.URL)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to rabbitmq: %w", err)
			if i < config.MaxRetries {
				time.Sleep(config.ReconnectInterval)
			}
			continue
		}

		// Создаем канал
		channel, err := conn.Channel()
		if err != nil {
			conn.Close()
			lastErr = fmt.Errorf("failed to open channel: %w", err)
			if i < config.MaxRetries {
				time.Sleep(config.ReconnectInterval)
			}
			continue
		}

		// Настраиваем prefetch для consumer
		err = channel.Qos(
			config.PrefetchCount,
			config.PrefetchSize,
			config.Global,
		)
		if err != nil {
			channel.Close()
			conn.Close()
			lastErr = fmt.Errorf("failed to set QoS: %w", err)
			if i < config.MaxRetries {
				time.Sleep(config.ReconnectInterval)
			}
			continue
		}

		// Объявляем dead letter exchange, если задан
		if config.DLX != "" {
			err = channel.ExchangeDeclare(
				config.DLX,
				"direct",
				true,
				false,
				false,
				false,
				nil,
			)
			if err != nil {
				channel.Close()
				conn.Close()
				lastErr = fmt.Errorf("failed to declare DLX: %w", err)
				if i < config.MaxRetries {
					time.Sleep(config.ReconnectInterval)
				}
				continue
			}
		}

		// Объявляем dead letter queue, если задан
		if config.DLQ != "" {
			_, err = channel.QueueDeclare(
				config.DLQ,
				true,
				false,
				false,
				false,
				nil,
			)
			if err != nil {
				channel.Close()
				conn.Close()
				lastErr = fmt.Errorf("failed to declare DLQ: %w", err)
				if i < config.MaxRetries {
					time.Sleep(config.ReconnectInterval)
				}
				continue
			}
		}

		return &Connection{conn: conn, channel: channel}, nil
	}

	return nil, fmt.Errorf("failed to connect to rabbitmq after %d retries: %w", config.MaxRetries, lastErr)
}

// Close закрывает подключение к RabbitMQ
func (c *Connection) Close() error {
	var connErr, channelErr error
	if c.channel != nil {
		channelErr = c.channel.Close()
	}
	if c.conn != nil {
		connErr = c.conn.Close()
	}
	// Возвращаем первую ошибку, если есть
	if channelErr != nil {
		return channelErr
	}
	return connErr
}

// Channel возвращает канал для использования
func (c *Connection) Channel() *amqp091.Channel {
	return c.channel
}

// GetConfig возвращает конфигурацию из переменных окружения
// TODO В реальном приложении здесь будет интеграция с системой конфигурации
func GetConfig() *Config {
	// TODO: Реализовать загрузку из переменных окружения
	return NewConfig()
}
