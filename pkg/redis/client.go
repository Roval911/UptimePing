package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Client представляет подключение к Redis
type Client struct {
	Client *redis.Client
}

// Config представляет конфигурацию Redis
type Config struct {
	Addr     string
	Password string
	DB       int
	// Connection pool settings
	PoolSize    int
	MinIdleConn int
	// Retry settings
	MaxRetries    int
	RetryInterval time.Duration
	// Health check
	HealthCheck time.Duration
}

// NewConfig создает конфигурацию по умолчанию
func NewConfig() *Config {
	return &Config{
		Addr:          "localhost:6379",
		Password:      "",
		DB:            0,
		PoolSize:      10,
		MinIdleConn:   2,
		MaxRetries:    3,
		RetryInterval: 1 * time.Second,
		HealthCheck:   30 * time.Second,
	}
}

// Connect устанавливает подключение к Redis с retry логикой
func Connect(ctx context.Context, config *Config) (*Client, error) {
	var lastErr error

	// Пытаемся подключиться с retry
	for i := 0; i <= config.MaxRetries; i++ {
		// Создаем клиент Redis
		client := redis.NewClient(&redis.Options{
			Addr:         config.Addr,
			Password:     config.Password,
			DB:           config.DB,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConn,
			// Таймауты
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			// Таймаут для получения соединения из пула
			PoolTimeout:        4 * time.Second,
			IdleCheckFrequency: config.HealthCheck,
		})

		// Проверяем подключение
		if err := client.Ping(ctx).Err(); err != nil {
			lastErr = fmt.Errorf("failed to ping redis: %w", err)
			client.Close()
			if i < config.MaxRetries {
				time.Sleep(config.RetryInterval)
			}
			continue
		}

		return &Client{Client: client}, nil
	}

	return nil, fmt.Errorf("failed to connect to redis after %d retries: %w", config.MaxRetries, lastErr)
}

// Close закрывает подключение к Redis
func (r *Client) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

// HealthCheck проверяет состояние подключения к Redis
func (r *Client) HealthCheck(ctx context.Context) error {
	if r.Client == nil {
		return fmt.Errorf("redis client is not initialized")
	}

	// Пытаемся выполнить простой запрос
	return r.Client.Ping(ctx).Err()
}

// GetConfig возвращает конфигурацию из переменных окружения
// В реальном приложении здесь будет интеграция с системой конфигурации
func GetConfig() *Config {
	// TODO: Реализовать загрузку из переменных окружения
	return NewConfig()
}
