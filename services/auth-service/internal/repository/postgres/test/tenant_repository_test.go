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
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewTenantRepository(db)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-1",
		Name:      "Test Tenant",
		Slug:      "test-tenant",
		Settings:  map[string]interface{}{"theme": "dark", "notifications": true},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	// Создаем тенант
	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Проверяем, что тенант был создан
	createdTenant, err := repo.FindByID(context.Background(), tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, createdTenant.ID)
	assert.Equal(t, tenant.Name, createdTenant.Name)
	assert.Equal(t, tenant.Slug, createdTenant.Slug)
	assert.Equal(t, tenant.Settings["theme"], createdTenant.Settings["theme"])
	assert.Equal(t, tenant.Settings["notifications"], createdTenant.Settings["notifications"])
	assert.WithinDuration(t, tenant.CreatedAt, createdTenant.CreatedAt, time.Second)
	assert.WithinDuration(t, tenant.UpdatedAt, createdTenant.UpdatedAt, time.Second)
}

func TestTenantRepository_FindByID(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewTenantRepository(db)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-1",
		Name:      "Test Tenant",
		Slug:      "test-tenant",
		Settings:  map[string]interface{}{"theme": "dark", "notifications": true},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	// Создаем тенант
	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Ищем тенант по ID
	foundTenant, err := repo.FindByID(context.Background(), tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, foundTenant.ID)
	assert.Equal(t, tenant.Name, foundTenant.Name)
}

func TestTenantRepository_FindBySlug(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewTenantRepository(db)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-1",
		Name:      "Test Tenant",
		Slug:      "test-tenant",
		Settings:  map[string]interface{}{"theme": "dark", "notifications": true},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	// Создаем тенант
	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Ищем тенант по slug
	foundTenant, err := repo.FindBySlug(context.Background(), tenant.Slug)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, foundTenant.ID)
	assert.Equal(t, tenant.Slug, foundTenant.Slug)
}

func TestTenantRepository_Update(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewTenantRepository(db)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-1",
		Name:      "Test Tenant",
		Slug:      "test-tenant",
		Settings:  map[string]interface{}{"theme": "dark", "notifications": true},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	// Создаем тенант
	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Обновляем тенант
	tenant.Name = "Updated Tenant"
	tenant.Slug = "updated-tenant"
	tenant.Settings["theme"] = "light"
	tenant.Settings["notifications"] = false
	tenant.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)

	err = repo.Update(context.Background(), tenant)
	require.NoError(t, err)

	// Проверяем, что тенант был обновлен
	updatedTenant, err := repo.FindByID(context.Background(), tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Tenant", updatedTenant.Name)
	assert.Equal(t, "updated-tenant", updatedTenant.Slug)
	assert.Equal(t, "light", updatedTenant.Settings["theme"])
	assert.Equal(t, false, updatedTenant.Settings["notifications"])
	assert.WithinDuration(t, tenant.UpdatedAt, updatedTenant.UpdatedAt, time.Second)
}

func TestTenantRepository_Delete(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewTenantRepository(db)

	// Создаем тестовый тенант
	tenant := &domain.Tenant{
		ID:        "tenant-1",
		Name:      "Test Tenant",
		Slug:      "test-tenant",
		Settings:  map[string]interface{}{"theme": "dark", "notifications": true},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	// Создаем тенант
	err := repo.Create(context.Background(), tenant)
	require.NoError(t, err)

	// Удаляем тенант
	err = repo.Delete(context.Background(), tenant.ID)
	require.NoError(t, err)

	// Проверяем, что тенант был удален
	_, err = repo.FindByID(context.Background(), tenant.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenant not found")
}
