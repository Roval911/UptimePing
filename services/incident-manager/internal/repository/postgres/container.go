package postgres

import (
	"context"

	"UptimePingPlatform/pkg/database"
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/incident-manager/internal/repository"
)

// RepositoryContainer содержит все репозитории для PostgreSQL
type RepositoryContainer struct {
	IncidentRepo     repository.IncidentRepository
	IncidentEventRepo repository.IncidentEventRepository
}

// NewRepositoryContainer создает новый контейнер репозиториев
func NewRepositoryContainer(db *database.Postgres, logger pkglogger.Logger) *RepositoryContainer {
	return &RepositoryContainer{
		IncidentRepo:     NewIncidentRepository(db.Pool, logger),
		IncidentEventRepo: NewIncidentEventRepository(db.Pool, logger),
	}
}

// HealthCheck проверяет доступность PostgreSQL
func (r *RepositoryContainer) HealthCheck(ctx context.Context) error {
	return r.IncidentRepo.(*IncidentRepository).pool.Ping(ctx)
}

// Close закрывает все соединения
func (r *RepositoryContainer) Close() error {
	if r.IncidentRepo.(*IncidentRepository).pool != nil {
		r.IncidentRepo.(*IncidentRepository).pool.Close()
	}
	return nil
}
