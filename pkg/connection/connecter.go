package connection

import (
	"context"
	"fmt"
	"time"
)

// Connecter определяет интерфейс для подключения к внешним системам
type Connecter interface {
	Connect(ctx context.Context) error
	Close() error
	IsConnected() bool
}

// RetryConfig содержит конфигурацию повторных попыток
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       bool
}

// DefaultRetryConfig возвращает конфигурацию по умолчанию
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// RetryFunc представляет функцию для повторной попытки
type RetryFunc func(ctx context.Context) error

// WithRetry выполняет функцию с retry логикой
func WithRetry(ctx context.Context, config RetryConfig, operation RetryFunc) error {
	var lastErr error
	
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := operation(ctx)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		if attempt < config.MaxAttempts {
			delay := calculateDelay(attempt, config)
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Продолжаем следующую попытку
			}
			continue
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// ConnectWithRetry выполняет подключение с retry логикой
func ConnectWithRetry(ctx context.Context, connecter Connecter, config RetryConfig) error {
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := connecter.Connect(ctx); err != nil {
			lastErr = err

			if attempt < config.MaxAttempts {
				delay := calculateDelay(attempt, config)

				// Логирование можно добавить через dependency injection
				fmt.Printf("Connection attempt %d failed, retrying in %v: %v\n", attempt, delay, err)

				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			continue
		}

		// Успешное подключение
		return nil
	}

	return fmt.Errorf("failed to connect after %d attempts: %w", config.MaxAttempts, lastErr)
}

// calculateDelay вычисляет задержку для retry
func calculateDelay(attempt int, config RetryConfig) time.Duration {
	delay := time.Duration(float64(config.InitialDelay) *
		pow(config.Multiplier, float64(attempt-1)))

	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	// Добавляем jitter если включен
	if config.Jitter {
		delay = addJitter(delay)
	}

	return delay
}

// addJitter добавляет случайную вариацию к задержке
func addJitter(delay time.Duration) time.Duration {
	// Добавляем случайную вариацию ±25%
	jitter := time.Duration(float64(delay) * 0.25 * (2*float64(time.Now().UnixNano()%1000)/1000 - 0.5))
	return delay + jitter - time.Duration(float64(jitter)*0.5)
}

// pow - простая реализация возведения в степень
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// BaseConnecter предоставляет базовую реализацию Connecter
type BaseConnecter struct {
	connected bool
}

// NewBaseConnecter создает новый BaseConnecter
func NewBaseConnecter() *BaseConnecter {
	return &BaseConnecter{
		connected: false,
	}
}

// IsConnected возвращает статус подключения
func (b *BaseConnecter) IsConnected() bool {
	return b.connected
}

// setConnected устанавливает статус подключения
func (b *BaseConnecter) setConnected(connected bool) {
	b.connected = connected
}
