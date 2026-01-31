package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository реализация репозитория пользователей для PostgreSQL
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository создает новый экземпляр UserRepository
func NewUserRepository(pool *pgxpool.Pool) repository.UserRepository {
	return &UserRepository{pool: pool}
}

// Create сохраняет нового пользователя в базе данных
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, email, password_hash, tenant_id, first_name, last_name, is_active, is_admin, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.TenantID,
		"", // first_name
		"", // last_name
		user.IsActive,
		user.IsAdmin,
		user.CreatedAt,
		user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// FindByID возвращает пользователя по его ID
func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	query := `SELECT id, email, password_hash, tenant_id, first_name, last_name, is_active, is_admin, created_at, updated_at 
		FROM users WHERE id = $1`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.TenantID,
		&user.FirstName, // first_name
		&user.LastName,  // last_name
		&user.IsActive,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return &user, nil
}

// FindByEmail возвращает пользователя по его email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, email, password_hash, tenant_id, first_name, last_name, is_active, is_admin, created_at, updated_at 
		FROM users WHERE email = $1`

	// Debug log
	fmt.Printf("DEBUG: Looking for user with email: %s\n", email)

	var user domain.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.TenantID,
		&user.FirstName, // first_name
		&user.LastName,  // last_name
		&user.IsActive,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

// Update обновляет существующего пользователя
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `UPDATE users SET 
		email = $2, 
		password_hash = $3, 
		tenant_id = $4, 
		is_active = $5, 
		is_admin = $6, 
		updated_at = $7 
	WHERE id = $1`

	result, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.TenantID,
		"", // first_name
		"", // last_name
		user.IsActive,
		user.IsAdmin,
		user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// Delete удаляет пользователя по ID
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
