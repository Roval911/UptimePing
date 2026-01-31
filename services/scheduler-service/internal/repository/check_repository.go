package repository

import (
	"context"

	"UptimePingPlatform/services/scheduler-service/internal/domain"
)

// CheckRepository определяет интерфейс для работы с проверками в БД
type CheckRepository interface {
	// Create создает новую проверку
	Create(ctx context.Context, check *domain.Check) error

	// GetByID возвращает проверку по ID
	GetByID(ctx context.Context, id string) (*domain.Check, error)

	// GetByTenantID возвращает список проверок для tenant
	GetByTenantID(ctx context.Context, tenantID string) ([]*domain.Check, error)

	// Update обновляет проверку
	Update(ctx context.Context, check *domain.Check) error

	// Delete удаляет проверку
	Delete(ctx context.Context, id string) error

	// GetActiveChecks возвращает список активных проверок
	GetActiveChecks(ctx context.Context) ([]*domain.Check, error)

	// GetActiveChecksByTenant возвращает список активных проверок для tenant
	GetActiveChecksByTenant(ctx context.Context, tenantID string) ([]*domain.Check, error)

	// List возвращает список проверок с пагинацией
	List(ctx context.Context, tenantID string, pageSize int, pageToken string) ([]*domain.Check, error)

	// Count возвращает общее количество проверок для tenant
	Count(ctx context.Context, tenantID string) (int, error)

	// Ping проверяет соединение с БД
	Ping(ctx context.Context) error
}
