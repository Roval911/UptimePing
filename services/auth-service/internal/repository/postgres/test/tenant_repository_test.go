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

func TestTenantRepository_Create(t *testing.T) {
	// Тестовая база данных
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	// Создаем репозиторий
	repo := postgres.NewTenantRepository(pool)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-1",
		Name:      "Test Tenant",
		Slug:      "test-tenant",
		Settings:  map[string]interface{}{"theme": "dark", "notifications": true},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	// Сохраняем тенант
	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Проверяем, что тенант сохранен
	found, err := repo.FindByID(context.Background(), "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, found.ID)
	assert.Equal(t, tenant.Name, found.Name)
	assert.Equal(t, tenant.Slug, found.Slug)
	assert.Equal(t, tenant.Settings, found.Settings)
}

func TestTenantRepository_FindByID(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewTenantRepository(pool)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-2",
		Name:      "Test Tenant 2",
		Slug:      "test-tenant-2",
		Settings:  map[string]interface{}{"theme": "light"},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Ищем по ID
	found, err := repo.FindByID(context.Background(), "tenant-2")
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, found.ID)
	assert.Equal(t, tenant.Name, found.Name)

	// Ищем несуществующий тенант
	found, err = repo.FindByID(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Nil(t, found)
}

func TestTenantRepository_FindBySlug(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewTenantRepository(pool)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-3",
		Name:      "Test Tenant 3",
		Slug:      "test-tenant-3",
		Settings:  map[string]interface{}{"theme": "dark"},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Ищем по slug
	found, err := repo.FindBySlug(context.Background(), "test-tenant-3")
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, found.ID)
	assert.Equal(t, tenant.Slug, found.Slug)

	// Ищем несуществующий slug
	found, err = repo.FindBySlug(context.Background(), "non-existent-slug")
	assert.Error(t, err)
	assert.Nil(t, found)
}

func TestTenantRepository_Update(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewTenantRepository(pool)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-4",
		Name:      "Original Name",
		Slug:      "original-slug",
		Settings:  map[string]interface{}{"theme": "light"},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Обновляем тенант
	tenant.Name = "Updated Name"
	tenant.Settings = map[string]interface{}{"theme": "dark", "language": "en"}
	updatedAt := time.Now().UTC().Truncate(time.Microsecond)
	tenant.UpdatedAt = updatedAt

	err = repo.Update(context.Background(), tenant)
	require.NoError(t, err)

	// Проверяем обновление
	found, err := repo.FindByID(context.Background(), "tenant-4")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", found.Name)
	assert.Equal(t, tenant.Settings, found.Settings)
}

func TestTenantRepository_Delete(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewTenantRepository(pool)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-5",
		Name:      "Tenant to Delete",
		Slug:      "tenant-to-delete",
		Settings:  map[string]interface{}{"theme": "light"},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Удаляем тенант
	err = repo.Delete(context.Background(), "tenant-5")
	require.NoError(t, err)

	// Проверяем, что тенант удален
	found, err := repo.FindByID(context.Background(), "tenant-5")
	assert.Error(t, err)
	assert.Nil(t, found)

	// Удаляем несуществующий тенант
	err = repo.Delete(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant not found")
}
