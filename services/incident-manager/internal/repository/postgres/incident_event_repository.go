package postgres

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"

	"UptimePingPlatform/pkg/errors"
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/incident-manager/internal/domain"
)

// IncidentEventRepository реализация репозитория событий инцидентов для PostgreSQL
type IncidentEventRepository struct {
	pool   *pgxpool.Pool
	logger pkglogger.Logger
}

// NewIncidentEventRepository создает новый экземпляр IncidentEventRepository
func NewIncidentEventRepository(pool *pgxpool.Pool, logger pkglogger.Logger) *IncidentEventRepository {
	return &IncidentEventRepository{
		pool:   pool,
		logger: logger,
	}
}

// Create сохраняет новое событие инцидента в базе данных
func (r *IncidentEventRepository) Create(ctx context.Context, event *domain.IncidentEvent) error {
	query := `
		INSERT INTO incident_events (id, incident_id, event_type, old_status, new_status, old_severity, new_severity, message, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.pool.Exec(ctx, query,
		event.ID,
		event.IncidentID,
		event.EventType,
		string(event.OldStatus),
		string(event.NewStatus),
		string(event.OldSeverity),
		string(event.NewSeverity),
		event.Message,
		event.Metadata,
		event.CreatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create incident event",
			pkglogger.String("event_id", event.ID),
			pkglogger.String("incident_id", event.IncidentID),
			pkglogger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to create incident event")
	}

	r.logger.Info("Incident event created successfully",
		pkglogger.String("event_id", event.ID),
		pkglogger.String("incident_id", event.IncidentID))

	return nil
}

// GetByIncidentID получает события инцидента по ID инцидента
func (r *IncidentEventRepository) GetByIncidentID(ctx context.Context, incidentID string) ([]*domain.IncidentEvent, error) {
	query := `
		SELECT id, incident_id, event_type, old_status, new_status, old_severity, new_severity, message, metadata, created_at
		FROM incident_events
		WHERE incident_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, incidentID)
	if err != nil {
		r.logger.Error("Failed to get incident events",
			pkglogger.String("incident_id", incidentID),
			pkglogger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get incident events")
	}
	defer rows.Close()

	var events []*domain.IncidentEvent
	for rows.Next() {
		var event domain.IncidentEvent
		var oldStatus, newStatus, oldSeverity, newSeverity string
		var metadata sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.IncidentID,
			&event.EventType,
			&oldStatus,
			&newStatus,
			&oldSeverity,
			&newSeverity,
			&event.Message,
			&metadata,
			&event.CreatedAt,
		)

		if err != nil {
			r.logger.Error("Failed to scan incident event row",
				pkglogger.Error(err))
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan incident event row")
		}

		event.OldStatus = domain.IncidentStatus(oldStatus)
		event.NewStatus = domain.IncidentStatus(newStatus)
		event.OldSeverity = domain.IncidentSeverity(oldSeverity)
		event.NewSeverity = domain.IncidentSeverity(newSeverity)

		if metadata.Valid {
			// Здесь можно добавить парсинг JSON для метаданных
			event.Metadata = make(map[string]interface{})
		} else {
			event.Metadata = nil
		}

		events = append(events, &event)
	}

	return events, nil
}

// DeleteByIncidentID удаляет все события инцидента по ID инцидента
func (r *IncidentEventRepository) DeleteByIncidentID(ctx context.Context, incidentID string) error {
	query := `DELETE FROM incident_events WHERE incident_id = $1`

	_, err := r.pool.Exec(ctx, query, incidentID)
	if err != nil {
		r.logger.Error("Failed to delete incident events",
			pkglogger.String("incident_id", incidentID),
			pkglogger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to delete incident events")
	}

	r.logger.Info("Incident events deleted successfully",
		pkglogger.String("incident_id", incidentID))

	return nil
}
