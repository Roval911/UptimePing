package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"UptimePingPlatform/pkg/errors"
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/incident-manager/internal/domain"
)

// IncidentRepository реализация репозитория инцидентов для PostgreSQL
type IncidentRepository struct {
	pool   *pgxpool.Pool
	logger pkglogger.Logger
}

// NewIncidentRepository создает новый экземпляр IncidentRepository
func NewIncidentRepository(pool *pgxpool.Pool, logger pkglogger.Logger) *IncidentRepository {
	return &IncidentRepository{
		pool:   pool,
		logger: logger,
	}
}

// Create сохраняет новый инцидент в базе данных
func (r *IncidentRepository) Create(ctx context.Context, incident *domain.Incident) error {
	query := `
		INSERT INTO incidents (id, check_id, title, description, status, severity, started_at, resolved_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.pool.Exec(ctx, query,
		incident.ID,
		incident.CheckID,
		incident.Title,
		incident.Description,
		string(incident.Status),
		string(incident.Severity),
		incident.StartedAt,
		incident.ResolvedAt,
		incident.CreatedAt,
		incident.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create incident",
			pkglogger.String("incident_id", incident.ID),
			pkglogger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to create incident")
	}

	r.logger.Info("Incident created successfully",
		pkglogger.String("incident_id", incident.ID))

	return nil
}

// GetByID получает инцидент по ID
func (r *IncidentRepository) GetByID(ctx context.Context, id string) (*domain.Incident, error) {
	query := `
		SELECT id, check_id, title, description, status, severity, started_at, resolved_at, created_at, updated_at
		FROM incidents
		WHERE id = $1
	`

	var incident domain.Incident
	var status, severity string
	var resolvedAt sql.NullTime

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&incident.ID,
		&incident.CheckID,
		&incident.Title,
		&incident.Description,
		&status,
		&severity,
		&incident.StartedAt,
		&resolvedAt,
		&incident.CreatedAt,
		&incident.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrNotFound, fmt.Sprintf("incident %s not found", id))
		}
		r.logger.Error("Failed to get incident",
			pkglogger.String("incident_id", id),
			pkglogger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get incident")
	}

	incident.Status = domain.IncidentStatus(status)
	incident.Severity = domain.IncidentSeverity(severity)
	if resolvedAt.Valid {
		incident.ResolvedAt = &resolvedAt.Time
	}

	return &incident, nil
}

// Update обновляет инцидент в базе данных
func (r *IncidentRepository) Update(ctx context.Context, incident *domain.Incident) error {
	query := `
		UPDATE incidents
		SET title = $2, description = $3, status = $4, severity = $5, resolved_at = $6, updated_at = $7
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		incident.ID,
		incident.Title,
		incident.Description,
		string(incident.Status),
		string(incident.Severity),
		incident.ResolvedAt,
		time.Now(),
	)

	if err != nil {
		r.logger.Error("Failed to update incident",
			pkglogger.String("incident_id", incident.ID),
			pkglogger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to update incident")
	}

	r.logger.Info("Incident updated successfully",
		pkglogger.String("incident_id", incident.ID))

	return nil
}

// List получает список инцидентов с фильтрацией
func (r *IncidentRepository) List(ctx context.Context, filter *domain.IncidentFilter) ([]*domain.Incident, error) {
	query := `
		SELECT i.id, i.check_id, i.title, i.description, i.status, i.severity, i.started_at, i.resolved_at, i.created_at, i.updated_at
		FROM incidents i
		JOIN checks c ON i.check_id = c.id
		WHERE 1=1
	`

	args := []interface{}{}
	argIndex := 1

	// Добавляем фильтрацию по tenant_id
	if filter != nil && filter.TenantID != nil && *filter.TenantID != "" {
		query += fmt.Sprintf(" AND c.tenant_id = $%d", argIndex)
		args = append(args, *filter.TenantID)
		argIndex++
	}

	// Добавляем фильтрацию по статусу
	if filter != nil && filter.Status != nil {
		query += fmt.Sprintf(" AND i.status = $%d", argIndex)
		args = append(args, string(*filter.Status))
		argIndex++
	}

	// Добавляем фильтрацию по check_id
	if filter != nil && filter.CheckID != nil && *filter.CheckID != "" {
		query += fmt.Sprintf(" AND i.check_id = $%d", argIndex)
		args = append(args, *filter.CheckID)
		argIndex++
	}

	// Добавляем сортировку и лимит
	query += " ORDER BY i.created_at DESC"
	if filter != nil && filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}
	if filter != nil && filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to list incidents",
			pkglogger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to list incidents")
	}
	defer rows.Close()

	var incidents []*domain.Incident
	for rows.Next() {
		var incident domain.Incident
		var status, severity string
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&incident.ID,
			&incident.CheckID,
			&incident.Title,
			&incident.Description,
			&status,
			&severity,
			&incident.StartedAt,
			&resolvedAt,
			&incident.CreatedAt,
			&incident.UpdatedAt,
		)

		if err != nil {
			r.logger.Error("Failed to scan incident row",
				pkglogger.Error(err))
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan incident row")
		}

		incident.Status = domain.IncidentStatus(status)
		incident.Severity = domain.IncidentSeverity(severity)
		if resolvedAt.Valid {
			incident.ResolvedAt = &resolvedAt.Time
		}

		incidents = append(incidents, &incident)
	}

	return incidents, nil
}

// Delete удаляет инцидент по ID
func (r *IncidentRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM incidents WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete incident",
			pkglogger.String("incident_id", id),
			pkglogger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to delete incident")
	}

	r.logger.Info("Incident deleted successfully",
		pkglogger.String("incident_id", id))

	return nil
}

// GetStats получает статистику по инцидентам
func (r *IncidentRepository) GetStats(ctx context.Context, tenantID string) (*domain.IncidentStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN status = 'open' THEN 1 END) as open_count,
			COUNT(CASE WHEN status = 'acknowledged' THEN 1 END) as acknowledged_count,
			COUNT(CASE WHEN status = 'resolved' THEN 1 END) as resolved_count,
			COUNT(CASE WHEN severity = 'low' THEN 1 END) as low_count,
			COUNT(CASE WHEN severity = 'medium' THEN 1 END) as medium_count,
			COUNT(CASE WHEN severity = 'high' THEN 1 END) as high_count,
			COUNT(CASE WHEN severity = 'critical' THEN 1 END) as critical_count,
			COUNT(CASE WHEN created_at >= NOW() - INTERVAL '24 hours' THEN 1 END) as last_24h,
			COUNT(CASE WHEN created_at >= NOW() - INTERVAL '7 days' THEN 1 END) as last_7d,
			COUNT(CASE WHEN created_at >= NOW() - INTERVAL '30 days' THEN 1 END) as last_30d
		FROM incidents
	`

	var stats domain.IncidentStats
	var openCount, acknowledgedCount, resolvedCount int
	var lowCount, mediumCount, highCount, criticalCount int

	err := r.pool.QueryRow(ctx, query).Scan(
		&stats.Total,
		&openCount,
		&acknowledgedCount,
		&resolvedCount,
		&lowCount,
		&mediumCount,
		&highCount,
		&criticalCount,
		&stats.Last24h,
		&stats.Last7d,
		&stats.Last30d,
	)

	if err != nil {
		r.logger.Error("Failed to get incident stats",
			pkglogger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get incident stats")
	}

	// Заполняем мапы статусов и серьезности
	stats.ByStatus = map[domain.IncidentStatus]int{
		domain.IncidentStatusOpen:         openCount,
		domain.IncidentStatusAcknowledged: acknowledgedCount,
		domain.IncidentStatusResolved:     resolvedCount,
	}

	stats.BySeverity = map[domain.IncidentSeverity]int{
		domain.IncidentSeverityLow:      lowCount,
		domain.IncidentSeverityMedium:   mediumCount,
		domain.IncidentSeverityHigh:     highCount,
		domain.IncidentSeverityCritical: criticalCount,
	}

	return &stats, nil
}

// GetByTenantID получает инциденты по tenant_id с фильтрацией
func (r *IncidentRepository) GetByTenantID(ctx context.Context, tenantID string, filter *domain.IncidentFilter) ([]*domain.Incident, error) {
	query := `
		SELECT i.id, i.check_id, i.title, i.description, i.status, i.severity, i.started_at, i.resolved_at, i.created_at, i.updated_at
		FROM incidents i
		JOIN checks c ON i.check_id = c.id
		WHERE c.tenant_id = $1
	`

	args := []interface{}{tenantID}
	argIndex := 2

	// Добавляем фильтрацию по статусу
	if filter != nil && filter.Status != nil {
		query += fmt.Sprintf(" AND i.status = $%d", argIndex)
		args = append(args, string(*filter.Status))
		argIndex++
	}

	// Добавляем фильтрацию по check_id
	if filter != nil && filter.CheckID != nil && *filter.CheckID != "" {
		query += fmt.Sprintf(" AND i.check_id = $%d", argIndex)
		args = append(args, *filter.CheckID)
		argIndex++
	}

	// Добавляем сортировку и лимит
	query += " ORDER BY i.created_at DESC"
	if filter != nil && filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}
	if filter != nil && filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to list incidents by tenant_id",
			pkglogger.String("tenant_id", tenantID),
			pkglogger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to list incidents by tenant_id")
	}
	defer rows.Close()

	var incidents []*domain.Incident
	for rows.Next() {
		var incident domain.Incident
		var status, severity string
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&incident.ID,
			&incident.CheckID,
			&incident.Title,
			&incident.Description,
			&status,
			&severity,
			&incident.StartedAt,
			&resolvedAt,
			&incident.CreatedAt,
			&incident.UpdatedAt,
		)

		if err != nil {
			r.logger.Error("Failed to scan incident row",
				pkglogger.Error(err))
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan incident row")
		}

		incident.Status = domain.IncidentStatus(status)
		incident.Severity = domain.IncidentSeverity(severity)
		if resolvedAt.Valid {
			incident.ResolvedAt = &resolvedAt.Time
		}

		incidents = append(incidents, &incident)
	}

	return incidents, nil
}
