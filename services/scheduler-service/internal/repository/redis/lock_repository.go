package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/services/scheduler-service/internal/domain"
	"UptimePingPlatform/services/scheduler-service/internal/repository"
)

// RedisLockRepository реализация LockRepository с использованием Redis
type RedisLockRepository struct {
	client *redis.Client
}

// NewRedisLockRepository создает новый экземпляр RedisLockRepository
func NewRedisLockRepository(client *redis.Client) repository.LockRepository {
	return &RedisLockRepository{
		client: client,
	}
}

// TryLock пытается получить блокировку для проверки
func (r *RedisLockRepository) TryLock(ctx context.Context, checkID, workerID string, ttl time.Duration) (*domain.LockInfo, error) {
	lockKey := fmt.Sprintf("lock:check:%s", checkID)

	now := time.Now()
	expiresAt := now.Add(ttl)

	lockInfo := &domain.LockInfo{
		CheckID:   checkID,
		WorkerID:  workerID,
		LockedAt:  now,
		ExpiresAt: expiresAt,
	}

	lockData, err := json.Marshal(lockInfo)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to marshal lock info").
			WithContext(ctx)
	}

	// Используем SET с опцией NX (только если ключ не существует) и EX (время жизни)
	result, err := r.client.SetNX(ctx, lockKey, lockData, ttl).Result()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to acquire lock").
			WithDetails(fmt.Sprintf("check_id: %s, worker_id: %s", checkID, workerID)).
			WithContext(ctx)
	}

	if !result {
		// Блокировка уже занята
		return nil, errors.New(errors.ErrConflict, "lock already acquired").
			WithDetails(fmt.Sprintf("check_id: %s", checkID)).
			WithContext(ctx)
	}

	return lockInfo, nil
}

// ReleaseLock освобождает блокировку
func (r *RedisLockRepository) ReleaseLock(ctx context.Context, checkID, workerID string) error {
	lockKey := fmt.Sprintf("lock:check:%s", checkID)

	// Получаем текущую блокировку для проверки worker_id
	lockData, err := r.client.Get(ctx, lockKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Блокировка не существует
			return nil
		}
		return errors.Wrap(err, errors.ErrInternal, "failed to get lock").
			WithDetails(fmt.Sprintf("check_id: %s", checkID)).
			WithContext(ctx)
	}

	var lockInfo domain.LockInfo
	if err := json.Unmarshal([]byte(lockData), &lockInfo); err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to unmarshal lock info").
			WithContext(ctx)
	}

	// Проверяем, что блокировка принадлежит этому worker
	if lockInfo.WorkerID != workerID {
		return errors.New(errors.ErrUnauthorized, "lock belongs to different worker").
			WithDetails(fmt.Sprintf("check_id: %s, expected_worker: %s, actual_worker: %s",
				checkID, workerID, lockInfo.WorkerID)).
			WithContext(ctx)
	}

	// Удаляем блокировку
	if err := r.client.Del(ctx, lockKey).Err(); err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to release lock").
			WithDetails(fmt.Sprintf("check_id: %s", checkID)).
			WithContext(ctx)
	}

	return nil
}

// IsLocked проверяет, заблокирована ли проверка
func (r *RedisLockRepository) IsLocked(ctx context.Context, checkID string) (bool, error) {
	lockKey := fmt.Sprintf("lock:check:%s", checkID)

	exists, err := r.client.Exists(ctx, lockKey).Result()
	if err != nil {
		return false, errors.Wrap(err, errors.ErrInternal, "failed to check lock existence").
			WithDetails(fmt.Sprintf("check_id: %s", checkID)).
			WithContext(ctx)
	}

	return exists > 0, nil
}

// GetLockInfo получает информацию о блокировке
func (r *RedisLockRepository) GetLockInfo(ctx context.Context, checkID string) (*domain.LockInfo, error) {
	lockKey := fmt.Sprintf("lock:check:%s", checkID)

	lockData, err := r.client.Get(ctx, lockKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, errors.New(errors.ErrNotFound, "lock not found").
				WithDetails(fmt.Sprintf("check_id: %s", checkID)).
				WithContext(ctx)
		}
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get lock info").
			WithDetails(fmt.Sprintf("check_id: %s", checkID)).
			WithContext(ctx)
	}

	var lockInfo domain.LockInfo
	if err := json.Unmarshal([]byte(lockData), &lockInfo); err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to unmarshal lock info").
			WithContext(ctx)
	}

	return &lockInfo, nil
}
