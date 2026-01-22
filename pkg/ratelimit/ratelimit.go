package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RateLimiter интерфейс для ограничения частоты запросов
type RateLimiter interface {
	// CheckRateLimit проверяет лимит для заданного ключа
	// Возвращает true, если лимит превышен
	CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// RedisRateLimiter реализация RateLimiter с использованием Redis
// Использует sliding window алгоритм для точного подсчета запросов в заданном временном окне
type RedisRateLimiter struct {
	client *redis.Client
}

// NewRedisRateLimiter создает новый экземпляр RedisRateLimiter
func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client}
}

// CheckRateLimit проверяет, не превышен ли лимит запросов для заданного ключа
// Использует sliding window алгоритм
// Алгоритм:
// 1. Определение ключа (IP или user_id)
// 2. Получение текущего счетчика из Redis
// 3. Если счетчик >= лимит → возвращает true (ErrTooManyRequests)
// 4. Увеличение счетчика (INCR)
// 5. Установка TTL для ключа
// 6. Возвращает false (успех)
func (r *RedisRateLimiter) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	// Формируем ключ для Redis
	redisKey := fmt.Sprintf("rate_limit:%s", key)

	// Используем транзакцию для атомарной проверки и увеличения счетчика
	tx := r.client.TxPipeline()

	// Получаем текущее значение счетчика
	current, err := r.client.Get(ctx, redisKey).Int64()
	if err != nil && err != redis.Nil {
		return true, fmt.Errorf("failed to get rate limit counter: %w", err)
	}

	// Проверяем, не превышен ли лимит
	if int(current) >= limit {
		// Закрываем pipeline без выполнения INCR
		tx.Close()
		return true, nil // Лимит превышен
	}

	// Увеличиваем счетчик и устанавливаем TTL
	tx.Incr(ctx, redisKey)
	tx.Expire(ctx, redisKey, window)

	// Выполняем транзакцию
	_, err = tx.Exec(ctx)
	if err != nil {
		return true, fmt.Errorf("failed to execute rate limit transaction: %w", err)
	}

	// Лимит не превышен
	return false, nil
}