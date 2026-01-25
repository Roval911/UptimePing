package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/repository"
)

// APIKeyRepository реализация репозитория API ключей для PostgreSQL
type APIKeyRepository struct {
	db *sql.DB
}

// NewAPIKeyRepository создает новый экземпляр APIKeyRepository
func NewAPIKeyRepository(db *sql.DB) repository.APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create сохраняет новый API ключ в базе данных
func (r *APIKeyRepository) Create(ctx context.Context, key *domain.APIKey) error {
	query := `INSERT INTO api_keys (id, tenant_id, key_hash, secret_hash, name, is_active, expires_at, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.db.ExecContext(ctx, query,
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
	err := r.db.QueryRowContext(ctx, query, id).Scan(
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
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("API key not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get API key by id: %w", err)
	}

	return &key, nil
}

// FindByKeyHash возвращает API ключ по его хэшу публичной части
func (r *APIKeyRepository) FindByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	query := `SELECT id, tenant_id, key_hash, secret_hash, name, is_active, expires_at, created_at 
		FROM api_keys WHERE key_hash = $1`

	var key domain.APIKey
	err := r.db.QueryRowContext(ctx, query, keyHash).Scan(
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
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("API key not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get API key by key hash: %w", err)
	}

	return &key, nil
}

// ListByTenant возвращает все API ключи для указанного тенанта
func (r *APIKeyRepository) ListByTenant(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	query := `SELECT id, tenant_id, key_hash, secret_hash, name, is_active, expires_at, created_at 
		FROM api_keys WHERE tenant_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
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

	// Проверяем ошибку итерации
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

	result, err := r.db.ExecContext(ctx, query,
		key.ID,
		key.Name,
		key.IsActive,
		key.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// Delete удаляет API ключ по ID
func (r *APIKeyRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM api_keys WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}
