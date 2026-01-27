package retry

import (
	"context"
	"math"
	"math/rand"
	"time"

	"UptimePingPlatform/pkg/logger"
)

// RetryConfig конфигурация retry логики
type RetryConfig struct {
	MaxAttempts    int           `json:"max_attempts" yaml:"max_attempts"`
	InitialDelay   time.Duration `json:"initial_delay" yaml:"initial_delay"`
	MaxDelay       time.Duration `json:"max_delay" yaml:"max_delay"`
	Multiplier     float64       `json:"multiplier" yaml:"multiplier"`
	Jitter         bool          `json:"jitter" yaml:"jitter"`
	JitterRange    float64       `json:"jitter_range" yaml:"jitter_range"`
}

// RetryableOperation интерфейс для retry операций
type RetryableOperation interface {
	Execute(ctx context.Context) error
	ShouldRetry(err error) bool
}

// RetryManager управляет retry логикой
type RetryManager struct {
	config RetryConfig
	logger logger.Logger
}

// NewRetryManager создает новый менеджер retry
func NewRetryManager(config RetryConfig, logger logger.Logger) *RetryManager {
	// Установка значений по умолчанию
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay == 0 {
		config.InitialDelay = 1 * time.Second
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.Multiplier == 0 {
		config.Multiplier = 2.0
	}
	if config.JitterRange == 0 {
		config.JitterRange = 0.1
	}

	return &RetryManager{
		config: config,
		logger: logger,
	}
}

// Execute выполняет операцию с retry логикой
func (r *RetryManager) Execute(ctx context.Context, operation RetryableOperation) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		if attempt > 1 {
			delay := r.calculateDelay(attempt - 1)
			
			r.logger.Debug("Retrying operation",
				logger.Int("attempt", attempt),
				logger.Duration("delay", delay),
				logger.Error(lastErr),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := operation.Execute(ctx)
		if err == nil {
			if attempt > 1 {
				r.logger.Info("Operation succeeded after retry",
					logger.Int("attempt", attempt),
				)
			}
			return nil
		}

		lastErr = err

		// Проверяем, нужно ли повторять попытку
		if !operation.ShouldRetry(err) {
			r.logger.Debug("Operation error is not retryable",
				logger.Error(err),
				logger.Int("attempt", attempt),
			)
			break
		}

		r.logger.Warn("Operation attempt failed",
			logger.Error(err),
			logger.Int("attempt", attempt),
			logger.Int("max_attempts", r.config.MaxAttempts),
		)
	}

	return lastErr
}

// calculateDelay вычисляет задержку для retry
func (r *RetryManager) calculateDelay(attempt int) time.Duration {
	// Базовая экспоненциальная задержка
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))
	
	// Ограничение максимальной задержки
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}
	
	// Добавление jitter для предотвращения thundering herd
	if r.config.Jitter {
		jitter := rand.Float64() * r.config.JitterRange * delay
		delay += jitter
	}
	
	return time.Duration(delay)
}

// GetDelay возвращает задержку для указанной попытки (без jitter)
func (r *RetryManager) GetDelay(attempt int) time.Duration {
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))
	
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}
	
	return time.Duration(delay)
}

// GetStats возвращает статистику retry менеджера
func (r *RetryManager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"max_attempts":  r.config.MaxAttempts,
		"initial_delay": r.config.InitialDelay.String(),
		"max_delay":     r.config.MaxDelay.String(),
		"multiplier":    r.config.Multiplier,
		"jitter":        r.config.Jitter,
		"jitter_range":  r.config.JitterRange,
	}
}

// DefaultRetryConfig возвращает конфигурацию по умолчанию
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		JitterRange:  0.1,
	}
}

// FastRetryConfig возвращает конфигурацию для быстрых retry
func FastRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   1.5,
		Jitter:       true,
		JitterRange:  0.05,
	}
}

// SlowRetryConfig возвращает конфигурацию для медленных retry
func SlowRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 5 * time.Second,
		MaxDelay:     300 * time.Second,
		Multiplier:   3.0,
		Jitter:       true,
		JitterRange:  0.2,
	}
}

// NoRetryConfig возвращает конфигурацию без retry
func NoRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  1,
		InitialDelay: 0,
		MaxDelay:     0,
		Multiplier:   1.0,
		Jitter:       false,
		JitterRange:  0,
	}
}

// RetryOperation базовая реализация RetryableOperation
type RetryOperation struct {
	Name        string
	ExecuteFunc func(ctx context.Context) error
	RetryFunc   func(err error) bool
}

// NewRetryOperation создает новую retry операцию
func NewRetryOperation(name string, executeFunc func(ctx context.Context) error, retryFunc func(err error) bool) *RetryOperation {
	return &RetryOperation{
		Name:        name,
		ExecuteFunc: executeFunc,
		RetryFunc:   retryFunc,
	}
}

// Execute выполняет операцию
func (op *RetryOperation) Execute(ctx context.Context) error {
	return op.ExecuteFunc(ctx)
}

// ShouldRetry определяет, нужно ли повторять попытку
func (op *RetryOperation) ShouldRetry(err error) bool {
	if op.RetryFunc != nil {
		return op.RetryFunc(err)
	}
	
	// Стандартная логика для retry
	return IsRetryableError(err)
}

// IsRetryableError проверяет, является ли ошибка retryable
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Context ошибки не retryable
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	
	errStr := err.Error()
	
	// Network ошибки обычно retryable
	if contains(errStr, "connection refused") ||
	   contains(errStr, "timeout") ||
	   contains(errStr, "network") ||
	   contains(errStr, "temporary") ||
	   contains(errStr, "connection reset") ||
	   contains(errStr, "no such host") ||
	   contains(errStr, "connection refused") {
		return true
	}
	
	// Rate limiting ошибки
	if contains(errStr, "rate limited") ||
	   contains(errStr, "too many requests") ||
	   contains(errStr, "rate limit") ||
	   contains(errStr, "quota exceeded") {
		return true
	}
	
	// Server ошибки
	if contains(errStr, "internal server error") ||
	   contains(errStr, "service unavailable") ||
	   contains(errStr, "bad gateway") ||
	   contains(errStr, "service temporarily unavailable") {
		return true
	}
	
	// Database ошибки
	if contains(errStr, "connection lost") ||
	   contains(errStr, "database locked") ||
	   contains(errStr, "deadlock") ||
	   contains(errStr, "connection timed out") {
		return true
	}
	
	// HTTP ошибки
	if contains(errStr, "502") ||
	   contains(errStr, "503") ||
	   contains(errStr, "504") ||
	   contains(errStr, "507") ||
	   contains(errStr, "509") ||
	   contains(errStr, "429") {
		return true
	}
	
	// Не retryable ошибки
	if contains(errStr, "unauthorized") ||
	   contains(errStr, "forbidden") ||
	   contains(errStr, "not found") ||
	   contains(errStr, "bad request") ||
	   contains(errStr, "validation") ||
	   contains(errStr, "invalid") ||
	   contains(errStr, "authentication") ||
	   contains(errStr, "permission") {
		return false
	}
	
	// По умолчанию считаем ошибку retryable
	return true
}

// contains проверяет наличие подстроки
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr ||
			 indexOf(s, substr) >= 0)))
}

// indexOf возвращает индекс подстроки
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
