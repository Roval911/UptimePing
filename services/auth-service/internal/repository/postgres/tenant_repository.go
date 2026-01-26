package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantRepository реализация репозитория тенантов для PostgreSQL
type TenantRepository struct {
	pool *pgxpool.Pool
}

// NewTenantRepository создает новый экземпляр TenantRepository
func NewTenantRepository(pool *pgxpool.Pool) repository.TenantRepository {
	return &TenantRepository{pool: pool}
}

// Create сохраняет новый тенант в базе данных
func (r *TenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	// Преобразуем map[string]interface{} в JSON
	settingsJSON, err := json.Marshal(tenant.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings to JSON: %w", err)
	}

	query := `INSERT INTO tenants (id, name, slug, settings, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err = r.pool.Exec(ctx, query,
		tenant.ID,
		tenant.Name,
		tenant.Slug,
		settingsJSON,
		tenant.CreatedAt,
		tenant.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	return nil
}

// FindByID возвращает тенант по его ID
func (r *TenantRepository) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	query := `SELECT id, name, slug, settings, created_at, updated_at 
		FROM tenants WHERE id = $1`

	var tenant domain.Tenant
	var settingsJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.Slug,
		&settingsJSON,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tenant not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get tenant by id: %w", err)
	}

	// Декодируем JSON обратно в map
	if err = json.Unmarshal(settingsJSON, &tenant.Settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings from JSON: %w", err)
	}

	return &tenant, nil
}

// FindBySlug возвращает тенант по его slug
func (r *TenantRepository) FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	query := `SELECT id, name, slug, settings, created_at, updated_at 
		FROM tenants WHERE slug = $1`

	var tenant domain.Tenant
	var settingsJSON []byte

	err := r.pool.QueryRow(ctx, query, slug).Scan(
		&tenant.ID,
		&tenant.Name,
		&tenant.Slug,
		&settingsJSON,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tenant not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get tenant by slug: %w", err)
	}

	// Декодируем JSON обратно в map
	if err = json.Unmarshal(settingsJSON, &tenant.Settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings from JSON: %w", err)
	}

	return &tenant, nil
}

// Update обновляет существующий тенант
func (r *TenantRepository) Update(ctx context.Context, tenant *domain.Tenant) error {
	// Преобразуем map[string]interface{} в JSON
	settingsJSON, err := json.Marshal(tenant.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings to JSON: %w", err)
	}

	query := `UPDATE tenants SET 
		name = $2, 
		slug = $3, 
		settings = $4, 
		updated_at = $5 
	WHERE id = $1`

	result, err := r.pool.Exec(ctx, query,
		tenant.ID,
		tenant.Name,
		tenant.Slug,
		settingsJSON,
		tenant.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("tenant not found")
	}

	return nil
}

// Delete удаляет тенант по ID
func (r *TenantRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tenants WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("tenant not found")
	}

	return nil
}
