package postgres

import (
	"context"
	"database/sql"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/repository"
)

// UserRepository реализация репозитория пользователей для PostgreSQL
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository создает новый экземпляр UserRepository
func NewUserRepository(db *sql.DB) repository.UserRepository {
	return &UserRepository{db: db}
}

// Create сохраняет нового пользователя в базе данных
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, email, password_hash, tenant_id, is_active, is_admin, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.TenantID,
		user.IsActive,
		user.IsAdmin,
		user.CreatedAt,
		user.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to create user")
	}

	return nil
}

// FindByID возвращает пользователя по его ID
func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	query := `SELECT id, email, password_hash, tenant_id, is_active, is_admin, created_at, updated_at 
		FROM users WHERE id = $1`

	var user domain.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.TenantID,
		&user.IsActive,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrNotFound, "user not found")
		}
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get user by id")
	}

	return &user, nil
}

// FindByEmail возвращает пользователя по его email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, email, password_hash, tenant_id, is_active, is_admin, created_at, updated_at 
		FROM users WHERE email = $1`

	var user domain.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.TenantID,
		&user.IsActive,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrNotFound, "user not found")
		}
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get user by email")
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

	result, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.TenantID,
		user.IsActive,
		user.IsAdmin,
		user.UpdatedAt,
	)

	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to update user")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "user not found")
	}

	return nil
}

// Delete удаляет пользователя по ID
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to delete user")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "user not found")
	}

	return nil
}
