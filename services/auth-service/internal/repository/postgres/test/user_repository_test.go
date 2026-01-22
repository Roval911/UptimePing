package postgres_test

import (
	"context"
	"testing"
	"time"

	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_Create(t *testing.T) {
	// Создаем тестовую базу данных
	pool := setupTestDB(t)
	defer pool.Close()

	// Создаем репозиторий
	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hashed-password",
		TenantID:     "tenant-1",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Создаем пользователя
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Проверяем, что пользователь был создан
	createdUser, err := repo.FindByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, createdUser.ID)
	assert.Equal(t, user.Email, createdUser.Email)
	assert.Equal(t, user.PasswordHash, createdUser.PasswordHash)
	assert.Equal(t, user.TenantID, createdUser.TenantID)
	assert.Equal(t, user.IsActive, createdUser.IsActive)
	assert.Equal(t, user.IsAdmin, createdUser.IsAdmin)
}

func TestUserRepository_FindByID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hashed-password",
		TenantID:     "tenant-1",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Создаем пользователя
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Ищем пользователя по ID
	foundUser, err := repo.FindByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, foundUser.ID)
	assert.Equal(t, user.Email, foundUser.Email)
}

func TestUserRepository_FindByEmail(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hashed-password",
		TenantID:     "tenant-1",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Создаем пользователя
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Ищем пользователя по email
	foundUser, err := repo.FindByEmail(context.Background(), user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, foundUser.ID)
	assert.Equal(t, user.Email, foundUser.Email)
}

func TestUserRepository_Update(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hashed-password",
		TenantID:     "tenant-1",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Создаем пользователя
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Обновляем пользователя
	user.Email = "updated@example.com"
	user.IsActive = false
	user.UpdatedAt = time.Now().UTC()

	err = repo.Update(context.Background(), user)
	require.NoError(t, err)

	// Проверяем, что пользователь был обновлен
	updatedUser, err := repo.FindByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated@example.com", updatedUser.Email)
	assert.False(t, updatedUser.IsActive)
}

func TestUserRepository_Delete(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hashed-password",
		TenantID:     "tenant-1",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Создаем пользователя
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Удаляем пользователя
	err = repo.Delete(context.Background(), user.ID)
	require.NoError(t, err)

	// Проверяем, что пользователь был удален
	_, err = repo.FindByID(context.Background(), user.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func setupTestDB(t *testing.T) *pgxpool.Pool {
	// В реальных тестах здесь должно быть подключение к тестовой базе данных
	// или использование Docker контейнера с PostgreSQL

	// Для примера создаем заглушку
	t.Skip("Test database setup not implemented")
	return nil
}
