package repository

import (
	"context"
	"time"

	"UptimePingPlatform/services/scheduler-service/internal/domain"
)

// LockRepository интерфейс для управления распределенными блокировками
type LockRepository interface {
	// TryLock пытается получить блокировку для проверки
	TryLock(ctx context.Context, checkID, workerID string, ttl time.Duration) (*domain.LockInfo, error)
	
	// ReleaseLock освобождает блокировку
	ReleaseLock(ctx context.Context, checkID, workerID string) error
	
	// IsLocked проверяет, заблокирована ли проверка
	IsLocked(ctx context.Context, checkID string) (bool, error)
	
	// GetLockInfo получает информацию о блокировке
	GetLockInfo(ctx context.Context, checkID string) (*domain.LockInfo, error)
}

// TaskRepository интерфейс для управления задачами
type TaskRepository interface {
	// CreateTask создает задачу
	CreateTask(ctx context.Context, task *domain.Task) error
	
	// GetTaskByID получает задачу по ID
	GetTaskByID(ctx context.Context, taskID string) (*domain.Task, error)
	
	// GetPendingTasks получает список ожидающих задач
	GetPendingTasks(ctx context.Context, limit int) ([]*domain.Task, error)
	
	// UpdateTaskStatus обновляет статус задачи
	UpdateTaskStatus(ctx context.Context, taskID string, status domain.TaskStatus) error
	
	// SaveTaskResult сохраняет результат выполнения задачи
	SaveTaskResult(ctx context.Context, result *domain.TaskResult) error
}
