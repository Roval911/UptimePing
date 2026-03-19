package repository

import (
	"context"

	"UptimePingPlatform/services/incident-manager/internal/domain"
)

// IncidentRepository интерфейс репозитория инцидентов
type IncidentRepository interface {
	Create(ctx context.Context, incident *domain.Incident) error
	GetByID(ctx context.Context, id string) (*domain.Incident, error)
	Update(ctx context.Context, incident *domain.Incident) error
	List(ctx context.Context, filter *domain.IncidentFilter) ([]*domain.Incident, error)
	Delete(ctx context.Context, id string) error
	GetStats(ctx context.Context, tenantID string) (*domain.IncidentStats, error)
}

// IncidentEventRepository интерфейс репозитория событий инцидентов
type IncidentEventRepository interface {
	Create(ctx context.Context, event *domain.IncidentEvent) error
	GetByIncidentID(ctx context.Context, incidentID string) ([]*domain.IncidentEvent, error)
	DeleteByIncidentID(ctx context.Context, incidentID string) error
}
