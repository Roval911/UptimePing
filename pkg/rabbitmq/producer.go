package rabbitmq

import (
	"context"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// Producer представляет продюсера сообщений
type Producer struct {
	conn   *Connection
	config *Config
}

// NewProducer создает нового продюсера
func NewProducer(conn *Connection, config *Config) *Producer {
	return &Producer{conn: conn, config: config}
}

// Publish публикует сообщение в RabbitMQ с подтверждениями
func (p *Producer) Publish(ctx context.Context, body []byte, options ...PublishOption) error {
	// Устанавливаем опции по умолчанию
	opts := &PublishOptions{
		Exchange:   p.config.Exchange,
		RoutingKey: p.config.RoutingKey,
		Mandatory:  false,
		Immediate:  false,
	}

	// Применяем пользовательские опции
	for _, option := range options {
		option(opts)
	}

	// Проверяем, что канал инициализирован
	if p.conn.Channel() == nil {
		return fmt.Errorf("rabbitmq channel is not initialized")
	}

	// Включаем confirm mode для получения подтверждений
	if err := p.conn.Channel().Confirm(false); err != nil {
		return fmt.Errorf("failed to enable confirm mode: %w", err)
	}

	// Создаем канал для подтверждений
	confirms := p.conn.Channel().NotifyPublish(make(chan amqp091.Confirmation, 1))
	defer close(confirms)

	// Публикуем сообщение
	msg := amqp091.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp091.Persistent,
		Timestamp:    time.Now(),
	}

	// Устанавливаем заголовки, если есть
	if len(opts.Headers) > 0 {
		msg.Headers = opts.Headers
	}

	// Публикуем сообщение
	if err := p.conn.Channel().PublishWithContext(ctx,
		opts.Exchange,
		opts.RoutingKey,
		opts.Mandatory,
		opts.Immediate,
		msg,
	); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Ожидаем подтверждение
	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return fmt.Errorf("message rejected by broker")
		}
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for confirmation: %w", ctx.Err())
	case <-time.After(10 * time.Second): // Таймаут ожидания подтверждения
		return fmt.Errorf("timeout waiting for confirmation")
	}

	return nil
}

// PublishWithRetry публикует сообщение с retry логикой
func (p *Producer) PublishWithRetry(ctx context.Context, body []byte, maxRetries int, retryInterval time.Duration, options ...PublishOption) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		err := p.Publish(ctx, body, options...)
		if err == nil {
			return nil
		}

		lastErr = err
		if i < maxRetries {
			time.Sleep(retryInterval)
		}
	}

	return fmt.Errorf("failed to publish message after %d retries: %w", maxRetries, lastErr)
}

// PublishOptions представляет опции для публикации сообщения
type PublishOptions struct {
	Exchange   string
	RoutingKey string
	Mandatory  bool
	Immediate  bool
	Headers    amqp091.Table
}

// PublishOption функция для настройки опций публикации
type PublishOption func(*PublishOptions)

// WithExchange устанавливает exchange
func WithExchange(exchange string) PublishOption {
	return func(opts *PublishOptions) {
		opts.Exchange = exchange
	}
}

// WithRoutingKey устанавливает routing key
func WithRoutingKey(routingKey string) PublishOption {
	return func(opts *PublishOptions) {
		opts.RoutingKey = routingKey
	}
}

// WithMandatory устанавливает mandatory флаг
func WithMandatory(mandatory bool) PublishOption {
	return func(opts *PublishOptions) {
		opts.Mandatory = mandatory
	}
}

// WithImmediate устанавливает immediate флаг
func WithImmediate(immediate bool) PublishOption {
	return func(opts *PublishOptions) {
		opts.Immediate = immediate
	}
}

// WithHeaders устанавливает заголовки
func WithHeaders(headers amqp091.Table) PublishOption {
	return func(opts *PublishOptions) {
		opts.Headers = headers
	}
}
