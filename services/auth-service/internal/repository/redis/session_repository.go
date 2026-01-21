package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/repository"
	"github.com/go-redis/redis/v8"
)

// SessionRepository реализация репозитория сессий для Redis
type SessionRepository struct {
	client *redis.Client
}

// NewSessionRepository создает новый экземпляр SessionRepository
func NewSessionRepository(client *redis.Client) repository.SessionRepository {
	return &SessionRepository{client: client}
}

// Create сохраняет сессию в Redis
func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	// Преобразуем сессию в JSON
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Устанавливаем TTL как разницу между ExpiresAt и текущим временем
	ttl := time.Until(session.ExpiresAt)

	// Используем хэш публичного токена как ключ
	key := fmt.Sprintf("session:access:%s", session.AccessTokenHash)

	// Сохраняем сессию в Redis
	err = r.client.Set(ctx, key, sessionData, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set session in Redis: %w", err)
	}

	// Также сохраняем по хэшу refresh токена для возможности отзыва
	refreshKey := fmt.Sprintf("session:refresh:%s", session.RefreshTokenHash)
	err = r.client.Set(ctx, refreshKey, sessionData, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set refresh session in Redis: %w", err)
	}

	return nil
}

// FindByID возвращает сессию по ее ID
func (r *SessionRepository) FindByID(ctx context.Context, id string) (*domain.Session, error) {
	// Получаем все ключи, соответствующие сессии по ID
	keys, err := r.client.Keys(ctx, fmt.Sprintf("session:*:%s", id)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session keys: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("session not found")
	}

	// Получаем данные первой найденной сессии
	data, err := r.client.Get(ctx, keys[0]).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Декодируем JSON обратно в структуру
	var session domain.Session
	err = json.Unmarshal([]byte(data), &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// FindByAccessTokenHash возвращает сессию по хэшу access токена
func (r *SessionRepository) FindByAccessTokenHash(ctx context.Context, accessTokenHash string) (*domain.Session, error) {
	key := fmt.Sprintf("session:access:%s", accessTokenHash)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get session by access token hash: %w", err)
	}

	// Декодируем JSON обратно в структуру
	var session domain.Session
	err = json.Unmarshal([]byte(data), &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// FindByRefreshTokenHash возвращает сессию по хэшу refresh токена
func (r *SessionRepository) FindByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (*domain.Session, error) {
	key := fmt.Sprintf("session:refresh:%s", refreshTokenHash)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get session by refresh token hash: %w", err)
	}

	// Декодируем JSON обратно в структуру
	var session domain.Session
	err = json.Unmarshal([]byte(data), &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// Delete удаляет сессию по ID
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	// Находим все ключи, связанные с сессией
	keys, err := r.client.Keys(ctx, fmt.Sprintf("session:*:%s", id)).Result()
	if err != nil {
		return fmt.Errorf("failed to get session keys: %w", err)
	}

	if len(keys) == 0 {
		return fmt.Errorf("session not found")
	}

	// Удаляем все найденные ключи
	_, err = r.client.Del(ctx, keys...).Result()
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// DeleteByUserID удаляет все сессии пользователя
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	// Находим все ключи, связанные с пользователем
	keys, err := r.client.Keys(ctx, fmt.Sprintf("session:*:*:%s*", userID)).Result()
	if err != nil {
		return fmt.Errorf("failed to get session keys: %w", err)
	}

	if len(keys) == 0 {
		return nil // Нет сессий для удаления
	}

	// Удаляем все найденные ключи
	_, err = r.client.Del(ctx, keys...).Result()
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return nil
}

// CleanupExpired удаляет просроченные сессии
func (r *SessionRepository) CleanupExpired(ctx context.Context, before time.Time) error {
	// В Redis мы полагаемся на TTL для автоматического удаления
	// Этот метод можно использовать для ручной очистки, если нужно
	// Но обычно достаточно TTL

	// Получаем все ключи сессий
	keys, err := r.client.Keys(ctx, "session:*:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get session keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Проверяем TTL каждого ключа и удаляем просроченные
	var expiredKeys []string
	for _, key := range keys {
		ttl, err := r.client.TTL(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("failed to get TTL for key %s: %w", key, err)
		}
		if ttl <= 0 {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Удаляем просроченные ключи
	if len(expiredKeys) > 0 {
		_, err = r.client.Del(ctx, expiredKeys...).Result()
		if err != nil {
			return fmt.Errorf("failed to delete expired sessions: %w", err)
		}
	}

	return nil
}
