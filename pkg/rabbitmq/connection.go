package rabbitmq

import (
	"context"
	"fmt"
	"os"
	"strconv"
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
	// Retry settings
	MaxRetryAttempts int
	RetryDelay       time.Duration
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
		MaxRetryAttempts:  3,
		RetryDelay:        5 * time.Second,
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
func GetConfig() *Config {
	config := NewConfig()
	
	// Загружаем URL подключения
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		config.URL = url
	}
	
	// Загружаем exchange
	if exchange := os.Getenv("RABBITMQ_EXCHANGE"); exchange != "" {
		config.Exchange = exchange
	}
	
	// Загружаем routing key
	if routingKey := os.Getenv("RABBITMQ_ROUTING_KEY"); routingKey != "" {
		config.RoutingKey = routingKey
	}
	
	// Загружаем queue
	if queue := os.Getenv("RABBITMQ_QUEUE"); queue != "" {
		config.Queue = queue
	}
	
	// Загружаем DLX
	if dlx := os.Getenv("RABBITMQ_DLX"); dlx != "" {
		config.DLX = dlx
	}
	
	// Загружаем DLQ
	if dlq := os.Getenv("RABBITMQ_DLQ"); dlq != "" {
		config.DLQ = dlq
	}
	
	// Загружаем интервал переподключения
	if reconnectInterval := os.Getenv("RABBITMQ_RECONNECT_INTERVAL"); reconnectInterval != "" {
		if interval, err := time.ParseDuration(reconnectInterval); err == nil {
			config.ReconnectInterval = interval
		}
	}
	
	// Загружаем максимальное количество попыток
	if maxRetries := os.Getenv("RABBITMQ_MAX_RETRIES"); maxRetries != "" {
		if retries, err := strconv.Atoi(maxRetries); err == nil {
			config.MaxRetries = retries
		}
	}
	
	// Загружаем prefetch count
	if prefetchCount := os.Getenv("RABBITMQ_PREFETCH_COUNT"); prefetchCount != "" {
		if count, err := strconv.Atoi(prefetchCount); err == nil {
			config.PrefetchCount = count
		}
	}
	
	// Загружаем prefetch size
	if prefetchSize := os.Getenv("RABBITMQ_PREFETCH_SIZE"); prefetchSize != "" {
		if size, err := strconv.Atoi(prefetchSize); err == nil {
			config.PrefetchSize = size
		}
	}
	
	// Загружаем global prefetch
	if global := os.Getenv("RABBITMQ_GLOBAL"); global != "" {
		config.Global = global == "true" || global == "1"
	}
	
	// Загружаем retry настройки
	if maxRetryAttempts := os.Getenv("RABBITMQ_MAX_RETRY_ATTEMPTS"); maxRetryAttempts != "" {
		if attempts, err := strconv.Atoi(maxRetryAttempts); err == nil {
			config.MaxRetryAttempts = attempts
		}
	}
	
	if retryDelay := os.Getenv("RABBITMQ_RETRY_DELAY"); retryDelay != "" {
		if delay, err := time.ParseDuration(retryDelay); err == nil {
			config.RetryDelay = delay
		}
	}
	
	return config
}
