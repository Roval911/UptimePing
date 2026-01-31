package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/scheduler-service/internal/domain"
	"UptimePingPlatform/services/scheduler-service/internal/repository"
)

// CheckRepository реализация репозитория для проверок в PostgreSQL
type CheckRepository struct {
	pool   *pgxpool.Pool
	logger logger.Logger
}

// NewCheckRepository создает новый экземпляр CheckRepository
func NewCheckRepository(pool *pgxpool.Pool) repository.CheckRepository {
	return &CheckRepository{
		pool: pool,
	}
}

// Create создает новую проверку
func (r *CheckRepository) Create(ctx context.Context, check *domain.Check) error {
	query := `
		INSERT INTO checks (id, tenant_id, name, description, type, target, 
			interval_seconds, timeout_seconds, enabled, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.pool.Exec(ctx, query,
		check.ID,
		check.TenantID,
		check.Name,
		check.Description,
		check.Type,
		check.Target,
		check.Interval,
		check.Timeout,
		check.Enabled,
		check.Config,
		check.CreatedAt,
		check.UpdatedAt,
	)

	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to create check").
			WithDetails(fmt.Sprintf("tenant_id: %s, name: %s", check.TenantID, check.Name)).
			WithContext(ctx)
	}

	return nil
}

// GetByID возвращает проверку по ID
func (r *CheckRepository) GetByID(ctx context.Context, id string) (*domain.Check, error) {
	query := `
		SELECT id, tenant_id, name, description, type, target, 
			interval_seconds, timeout_seconds, enabled, config, created_at, updated_at
		FROM checks
		WHERE id = $1
	`

	var check domain.Check

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&check.ID,
		&check.TenantID,
		&check.Name,
		&check.Description,
		&check.Type,
		&check.Target,
		&check.Interval,
		&check.Timeout,
		&check.Enabled,
		&check.Config,
		&check.CreatedAt,
		&check.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrNotFound, "check not found").
				WithDetails(fmt.Sprintf("check_id: %s", id)).
				WithContext(ctx)
		}
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get check").
			WithDetails(fmt.Sprintf("check_id: %s", id)).
			WithContext(ctx)
	}

	return &check, nil
}

// GetByTenantID возвращает список проверок для tenant
func (r *CheckRepository) GetByTenantID(ctx context.Context, tenantID string) ([]*domain.Check, error) {
	query := `
		SELECT id, tenant_id, name, description, type, target, 
			interval_seconds, timeout_seconds, enabled, config, created_at, updated_at
		FROM checks
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get checks by tenant").
			WithDetails(fmt.Sprintf("tenant_id: %s", tenantID)).
			WithContext(ctx)
	}
	defer rows.Close()

	var checks []*domain.Check
	for rows.Next() {
		var check domain.Check

		err := rows.Scan(
			&check.ID,
			&check.TenantID,
			&check.Name,
			&check.Description,
			&check.Type,
			&check.Target,
			&check.Interval,
			&check.Timeout,
			&check.Enabled,
			&check.Config,
			&check.CreatedAt,
			&check.UpdatedAt,
		)

		if err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan check").
				WithDetails(fmt.Sprintf("tenant_id: %s", tenantID)).
				WithContext(ctx)
		}

		checks = append(checks, &check)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to iterate checks").
			WithDetails(fmt.Sprintf("tenant_id: %s", tenantID)).
			WithContext(ctx)
	}

	return checks, nil
}

// Update обновляет проверку
func (r *CheckRepository) Update(ctx context.Context, check *domain.Check) error {
	query := `
		UPDATE checks
		SET name = $2, description = $3, type = $4, target = $5, 
			interval_seconds = $6, timeout_seconds = $7, enabled = $8, 
			config = $9, updated_at = $10
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		check.ID,
		check.Name,
		check.Description,
		check.Type,
		check.Target,
		check.Interval,
		check.Timeout,
		check.Enabled,
		check.Config,
		check.UpdatedAt,
	)

	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to update check").
			WithDetails(fmt.Sprintf("check_id: %s, tenant_id: %s", check.ID, check.TenantID)).
			WithContext(ctx)
	}

	return nil
}

// Delete удаляет проверку
func (r *CheckRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM checks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to delete check").
			WithDetails(fmt.Sprintf("check_id: %s", id)).
			WithContext(ctx)
	}

	if result.RowsAffected() == 0 {
		return errors.New(errors.ErrNotFound, "check not found").
			WithDetails(fmt.Sprintf("check_id: %s", id)).
			WithContext(ctx)
	}

	return nil
}

// List возвращает список проверок с пагинацией
func (r *CheckRepository) List(ctx context.Context, tenantID string, pageSize int, pageToken string) ([]*domain.Check, error) {
	query := `
		SELECT id, tenant_id, name, description, type, target, 
			interval_seconds, timeout_seconds, enabled, config, created_at, updated_at
		FROM checks
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, tenantID, pageSize)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to list checks").
			WithDetails(fmt.Sprintf("tenant_id: %s, page_size: %d", tenantID, pageSize)).
			WithContext(ctx)
	}
	defer rows.Close()

	var checks []*domain.Check
	for rows.Next() {
		var check domain.Check

		err := rows.Scan(
			&check.ID,
			&check.TenantID,
			&check.Name,
			&check.Description,
			&check.Type,
			&check.Target,
			&check.Interval,
			&check.Timeout,
			&check.Enabled,
			&check.Config,
			&check.CreatedAt,
			&check.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan check").
				WithContext(ctx)
		}

		checks = append(checks, &check)
	}

	return checks, nil
}

// Count возвращает общее количество проверок для tenant
func (r *CheckRepository) Count(ctx context.Context, tenantID string) (int, error) {
	query := `SELECT COUNT(*) FROM checks WHERE tenant_id = $1`

	var count int
	err := r.pool.QueryRow(ctx, query, tenantID).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, errors.ErrInternal, "failed to count checks").
			WithDetails(fmt.Sprintf("tenant_id: %s", tenantID)).
			WithContext(ctx)
	}

	return count, nil
}

// GetActiveChecks возвращает список активных проверок
func (r *CheckRepository) GetActiveChecks(ctx context.Context) ([]*domain.Check, error) {
	query := `
		SELECT id, tenant_id, name, description, type, target, 
			interval_seconds, timeout_seconds, enabled, config, created_at, updated_at
		FROM checks
		WHERE enabled = true
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get active checks").
			WithContext(ctx)
	}
	defer rows.Close()

	var checks []*domain.Check
	for rows.Next() {
		var check domain.Check

		err := rows.Scan(
			&check.ID,
			&check.TenantID,
			&check.Name,
			&check.Description,
			&check.Type,
			&check.Target,
			&check.Interval,
			&check.Timeout,
			&check.Enabled,
			&check.Config,
			&check.CreatedAt,
			&check.UpdatedAt,
		)

		if err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan active check").
				WithContext(ctx)
		}

		checks = append(checks, &check)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to iterate active checks").
			WithContext(ctx)
	}

	return checks, nil
}

// GetActiveChecksByTenant возвращает список активных проверок для tenant
func (r *CheckRepository) GetActiveChecksByTenant(ctx context.Context, tenantID string) ([]*domain.Check, error) {
	query := `
		SELECT id, tenant_id, name, description, type, target, 
			interval_seconds, timeout_seconds, enabled, config, created_at, updated_at
		FROM checks
		WHERE tenant_id = $1 AND enabled = true
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get active checks by tenant").
			WithDetails(fmt.Sprintf("tenant_id: %s", tenantID)).
			WithContext(ctx)
	}
	defer rows.Close()

	var checks []*domain.Check
	for rows.Next() {
		var check domain.Check

		err := rows.Scan(
			&check.ID,
			&check.TenantID,
			&check.Name,
			&check.Description,
			&check.Type,
			&check.Target,
			&check.Interval,
			&check.Timeout,
			&check.Enabled,
			&check.Config,
			&check.CreatedAt,
			&check.UpdatedAt,
		)

		if err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan active check").
				WithDetails(fmt.Sprintf("tenant_id: %s", tenantID)).
				WithContext(ctx)
		}

		checks = append(checks, &check)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to iterate active checks").
			WithDetails(fmt.Sprintf("tenant_id: %s", tenantID)).
			WithContext(ctx)
	}

	return checks, nil
}

// Ping проверяет подключение к базе данных
func (r *CheckRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}
