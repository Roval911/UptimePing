package postgres

import (
	"context"
	"fmt"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIKeyRepository реализация репозитория API ключей для PostgreSQL
type APIKeyRepository struct {
	*BaseRepository
}

// NewAPIKeyRepository создает новый экземпляр APIKeyRepository
func NewAPIKeyRepository(pool *pgxpool.Pool) repository.APIKeyRepository {
	return &APIKeyRepository{BaseRepository: NewBaseRepository(pool)}
}

// Create сохраняет новый API ключ в базе данных
func (r *APIKeyRepository) Create(ctx context.Context, key *domain.APIKey) error {
	query := `INSERT INTO api_keys (id, tenant_id, key_hash, secret_hash, name, is_active, expires_at, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.Pool.Exec(ctx, query,
		key.ID,
		key.TenantID,
		key.KeyHash,
		key.SecretHash,
		key.Name,
		key.IsActive,
		key.ExpiresAt,
		key.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	return nil
}

// FindByID возвращает API ключ по его ID
func (r *APIKeyRepository) FindByID(ctx context.Context, id string) (*domain.APIKey, error) {
	query := `SELECT id, tenant_id, key_hash, secret_hash, name, is_active, expires_at, created_at 
		FROM api_keys WHERE id = $1`

	var key domain.APIKey
	err := r.Pool.QueryRow(ctx, query, id).Scan(
		&key.ID,
		&key.TenantID,
		&key.KeyHash,
		&key.SecretHash,
		&key.Name,
		&key.IsActive,
		&key.ExpiresAt,
		&key.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.ErrNotFound, "API key not found")
		}
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get API key by id")
	}

	return &key, nil
}

// FindByKeyHash возвращает API ключ по его хэшу публичной части
func (r *APIKeyRepository) FindByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	query := `SELECT id, tenant_id, key_hash, secret_hash, name, is_active, expires_at, created_at 
		FROM api_keys WHERE key_hash = $1`

	var key domain.APIKey
	err := r.Pool.QueryRow(ctx, query, keyHash).Scan(
		&key.ID,
		&key.TenantID,
		&key.KeyHash,
		&key.SecretHash,
		&key.Name,
		&key.IsActive,
		&key.ExpiresAt,
		&key.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.ErrNotFound, "API key not found")
		}
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get API key by key hash")
	}

	return &key, nil
}

// ListByTenant возвращает все API ключи для указанного тенанта
func (r *APIKeyRepository) ListByTenant(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	query := `SELECT id, tenant_id, key_hash, secret_hash, name, is_active, expires_at, created_at 
		FROM api_keys WHERE tenant_id = $1 ORDER BY created_at DESC`

	rows, err := r.Pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		var key domain.APIKey
		err := rows.Scan(
			&key.ID,
			&key.TenantID,
			&key.KeyHash,
			&key.SecretHash,
			&key.Name,
			&key.IsActive,
			&key.ExpiresAt,
			&key.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, &key)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate API keys: %w", err)
	}

	return keys, nil
}

// Update обновляет существующий API ключ
func (r *APIKeyRepository) Update(ctx context.Context, key *domain.APIKey) error {
	query := `UPDATE api_keys SET 
		name = $2, 
		is_active = $3, 
		expires_at = $4 
	WHERE id = $1`

	tag, err := r.Pool.Exec(ctx, query,
		key.ID,
		key.Name,
		key.IsActive,
		key.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// Delete удаляет API ключ по ID
func (r *APIKeyRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM api_keys WHERE id = $1`

	tag, err := r.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}
