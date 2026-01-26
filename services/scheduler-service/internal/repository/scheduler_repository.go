package repository

import (
	"context"

	"UptimePingPlatform/services/scheduler-service/internal/domain"
)

// SchedulerRepository определяет интерфейс для работы с планировщиком
type SchedulerRepository interface {
	// AddCheck добавляет проверку в планировщик
	AddCheck(ctx context.Context, check *domain.Check) error

	// RemoveCheck удаляет проверку из планировщика
	RemoveCheck(ctx context.Context, checkID string) error

	// UpdateCheck обновляет проверку в планировщике
	UpdateCheck(ctx context.Context, check *domain.Check) error

	// GetScheduledChecks возвращает список запланированных проверок
	GetScheduledChecks(ctx context.Context) ([]*domain.Check, error)

	// Методы для работы с расписаниями
	Create(ctx context.Context, schedule *domain.Schedule) (*domain.Schedule, error)
	DeleteByCheckID(ctx context.Context, checkID string) error
	GetByCheckID(ctx context.Context, checkID string) (*domain.Schedule, error)
	List(ctx context.Context, pageSize int, pageToken string, filter string) ([]*domain.Schedule, error)
	Count(ctx context.Context, filter string) (int, error)

	// Health check метод
	Ping(ctx context.Context) (interface{}, error)
}
