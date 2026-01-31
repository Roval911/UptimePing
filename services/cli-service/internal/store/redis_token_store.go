package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisTokenStore хранит токены в Redis
type RedisTokenStore struct {
	client *redis.Client
	prefix string
}

// NewRedisTokenStore создает новое хранилище токенов в Redis
func NewRedisTokenStore() (*RedisTokenStore, error) {
	// Подключаемся к Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к Redis: %w", err)
	}

	return &RedisTokenStore{
		client: rdb,
		prefix: "uptimeping:cli:tokens:",
	}, nil
}

// SaveTokens сохраняет токены в Redis
func (rts *RedisTokenStore) SaveTokens(tokenInfo *TokenInfo) error {
	ctx := context.Background()

	// Сериализуем токены
	data, err := json.Marshal(tokenInfo)
	if err != nil {
		return fmt.Errorf("ошибка сериализации токенов: %w", err)
	}

	// Сохраняем в Redis с TTL = время истечения токена
	key := rts.prefix + "current"
	ttl := time.Until(tokenInfo.ExpiresAt)

	err = rts.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("ошибка сохранения токенов в Redis: %w", err)
	}

	return nil
}

// LoadTokens загружает токены из Redis
func (rts *RedisTokenStore) LoadTokens() (*TokenInfo, error) {
	ctx := context.Background()

	key := rts.prefix + "current"
	data, err := rts.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("токены не найдены")
		}
		return nil, fmt.Errorf("ошибка загрузки токенов из Redis: %w", err)
	}

	// Десериализуем токены
	var tokenInfo TokenInfo
	err = json.Unmarshal([]byte(data), &tokenInfo)
	if err != nil {
		return nil, fmt.Errorf("ошибка десериализации токенов: %w", err)
	}

	return &tokenInfo, nil
}

// HasTokens проверяет наличие токенов
func (rts *RedisTokenStore) HasTokens() bool {
	ctx := context.Background()

	key := rts.prefix + "current"
	_, err := rts.client.Get(ctx, key).Result()
	return err != redis.Nil
}

// ClearTokens удаляет токены из Redis
func (rts *RedisTokenStore) ClearTokens() error {
	ctx := context.Background()

	key := rts.prefix + "current"
	err := rts.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("ошибка удаления токенов из Redis: %w", err)
	}

	return nil
}

// GetAccessToken возвращает access токен
func (rts *RedisTokenStore) GetAccessToken() string {
	if tokenInfo, err := rts.LoadTokens(); err == nil {
		return tokenInfo.AccessToken
	}
	return ""
}

// Close закрывает подключение к Redis
func (rts *RedisTokenStore) Close() error {
	return rts.client.Close()
}
