package postgres_test

import (
	"context"
	"testing"
	"time"

	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_Create(t *testing.T) {
	// Тестовая база данных
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	// Создаем репозиторий
	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hashed-password-1",
		TenantID:     "tenant-1",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	// Сохраняем пользователя
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Проверяем, что пользователь сохранен
	found, err := repo.FindByID(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, user.Email, found.Email)
	assert.Equal(t, user.TenantID, found.TenantID)
	assert.Equal(t, user.IsActive, found.IsActive)
	assert.Equal(t, user.IsAdmin, found.IsAdmin)
}

func TestUserRepository_FindByID(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-2",
		Email:        "test2@example.com",
		PasswordHash: "hashed-password-2",
		TenantID:     "tenant-2",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Ищем по ID
	found, err := repo.FindByID(context.Background(), "user-2")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, user.Email, found.Email)

	// Ищем несуществующего пользователя
	found, err = repo.FindByID(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Nil(t, found)
}

func TestUserRepository_FindByEmail(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-3",
		Email:        "test3@example.com",
		PasswordHash: "hashed-password-3",
		TenantID:     "tenant-3",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Ищем по email
	found, err := repo.FindByEmail(context.Background(), "test3@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, user.Email, found.Email)

	// Ищем несуществующий email
	found, err = repo.FindByEmail(context.Background(), "nonexistent@example.com")
	assert.Error(t, err)
	assert.Nil(t, found)
}

func TestUserRepository_Update(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-4",
		Email:        "original@example.com",
		PasswordHash: "hashed-password-4",
		TenantID:     "tenant-4",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Обновляем пользователя
	user.Email = "updated@example.com"
	user.IsActive = false
	user.IsAdmin = true
	updatedAt := time.Now().UTC().Truncate(time.Microsecond)
	user.UpdatedAt = updatedAt

	err = repo.Update(context.Background(), user)
	require.NoError(t, err)

	// Проверяем обновление
	found, err := repo.FindByID(context.Background(), "user-4")
	require.NoError(t, err)
	assert.Equal(t, "updated@example.com", found.Email)
	assert.Equal(t, false, found.IsActive)
	assert.Equal(t, true, found.IsAdmin)
}

func TestUserRepository_Delete(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewUserRepository(pool)

	// Создаем тестового пользователя
	user := &domain.User{
		ID:           "user-5",
		Email:        "delete@example.com",
		PasswordHash: "hashed-password-5",
		TenantID:     "tenant-5",
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Удаляем пользователя
	err = repo.Delete(context.Background(), "user-5")
	require.NoError(t, err)

	// Проверяем, что пользователь удален
	found, err := repo.FindByID(context.Background(), "user-5")
	assert.Error(t, err)
	assert.Nil(t, found)

	// Удаляем несуществующего пользователя
	err = repo.Delete(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}
