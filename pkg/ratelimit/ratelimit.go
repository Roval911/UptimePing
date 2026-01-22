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
type RedisRateLimiter struct {
	client *redis.Client
}

// NewRedisRateLimiter создает новый экземпляр RedisRateLimiter
func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client}
}

// CheckRateLimit проверяет лимит запросов для заданного ключа
func (r *RedisRateLimiter) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	redisKey := fmt.Sprintf("rate_limit:%s", key)

	// Lua-скрипт для атомарной проверки и увеличения счетчика
	script := `
		local current = redis.call('GET', KEYS[1])
		if current and tonumber(current) >= tonumber(ARGV[1]) then
			return 1
		end
		redis.call('INCR', KEYS[1])
		redis.call('EXPIRE', KEYS[1], ARGV[2])
		return 0
	`

	result, err := r.client.Eval(ctx, script, []string{redisKey}, limit, window.Seconds()).Int64()
	if err != nil {
		return true, fmt.Errorf("failed to execute rate limit script: %w", err)
	}

	return result == 1, nil
}
