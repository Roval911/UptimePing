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
}
