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

func TestAPIKeyRepository_Create(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewAPIKeyRepository(db)

	// Создаем тестовый API ключ
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key := &domain.APIKey{
		ID:         "key-1",
		TenantID:   "tenant-1",
		KeyHash:    "key-hash-1",
		SecretHash: "secret-hash-1",
		Name:       "Test API Key",
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
	}

	// Создаем API ключ
	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Проверяем, что API ключ был создан
	createdKey, err := repo.FindByID(context.Background(), key.ID)
	require.NoError(t, err)
	assert.Equal(t, key.ID, createdKey.ID)
	assert.Equal(t, key.TenantID, createdKey.TenantID)
	assert.Equal(t, key.KeyHash, createdKey.KeyHash)
	assert.Equal(t, key.SecretHash, createdKey.SecretHash)
	assert.Equal(t, key.Name, createdKey.Name)
	assert.Equal(t, key.IsActive, createdKey.IsActive)
	assert.WithinDuration(t, key.ExpiresAt, createdKey.ExpiresAt, time.Second)
	assert.WithinDuration(t, key.CreatedAt, createdKey.CreatedAt, time.Second)
}

func TestAPIKeyRepository_FindByID(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewAPIKeyRepository(db)

	// Создаем тестовый API ключ
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key := &domain.APIKey{
		ID:         "key-1",
		TenantID:   "tenant-1",
		KeyHash:    "key-hash-1",
		SecretHash: "secret-hash-1",
		Name:       "Test API Key",
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
	}

	// Создаем API ключ
	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Ищем API ключ по ID
	foundKey, err := repo.FindByID(context.Background(), key.ID)
	require.NoError(t, err)
	assert.Equal(t, key.ID, foundKey.ID)
	assert.Equal(t, key.Name, foundKey.Name)
}

func TestAPIKeyRepository_FindByKeyHash(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewAPIKeyRepository(db)

	// Создаем тестовый API ключ
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key := &domain.APIKey{
		ID:         "key-1",
		TenantID:   "tenant-1",
		KeyHash:    "key-hash-1",
		SecretHash: "secret-hash-1",
		Name:       "Test API Key",
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
	}

	// Создаем API ключ
	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Ищем API ключ по хэшу
	foundKey, err := repo.FindByKeyHash(context.Background(), key.KeyHash)
	require.NoError(t, err)
	assert.Equal(t, key.ID, foundKey.ID)
	assert.Equal(t, key.KeyHash, foundKey.KeyHash)
}

func TestAPIKeyRepository_ListByTenant(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewAPIKeyRepository(db)

	// Создаем тестовые API ключи
	tenantID := "tenant-1"
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key1 := &domain.APIKey{
		ID:         "key-1",
		TenantID:   tenantID,
		KeyHash:    "key-hash-1",
		SecretHash: "secret-hash-1",
		Name:       "Test API Key 1",
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
	}

	key2 := &domain.APIKey{
		ID:         "key-2",
		TenantID:   tenantID,
		KeyHash:    "key-hash-2",
		SecretHash: "secret-hash-2",
		Name:       "Test API Key 2",
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
	}

	// Создаем API ключи
	err := repo.Create(context.Background(), key1)
	require.NoError(t, err)

	err = repo.Create(context.Background(), key2)
	require.NoError(t, err)

	// Получаем список API ключей для тенанта
	keys, err := repo.ListByTenant(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Equal(t, key1.ID, keys[0].ID)
	assert.Equal(t, key2.ID, keys[1].ID)
}

func TestAPIKeyRepository_Update(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewAPIKeyRepository(db)

	// Создаем тестовый API ключ
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key := &domain.APIKey{
		ID:         "key-1",
		TenantID:   "tenant-1",
		KeyHash:    "key-hash-1",
		SecretHash: "secret-hash-1",
		Name:       "Test API Key",
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
	}

	// Создаем API ключ
	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Обновляем API ключ
	key.Name = "Updated API Key"
	key.IsActive = false
	key.ExpiresAt = time.Now().UTC().Add(48 * time.Hour).Truncate(time.Microsecond)

	err = repo.Update(context.Background(), key)
	require.NoError(t, err)

	// Проверяем, что API ключ был обновлен
	updatedKey, err := repo.FindByID(context.Background(), key.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated API Key", updatedKey.Name)
	assert.False(t, updatedKey.IsActive)
	assert.WithinDuration(t, key.ExpiresAt, updatedKey.ExpiresAt, time.Second)
}

func TestAPIKeyRepository_Delete(t *testing.T) {
	// Тестовая база данных
	_ = setupTestDB(t)

	// Создаем репозиторий
	db := setupTestDB(t)
	repo := postgres.NewAPIKeyRepository(db)

	// Создаем тестовый API ключ
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key := &domain.APIKey{
		ID:         "key-1",
		TenantID:   "tenant-1",
		KeyHash:    "key-hash-1",
		SecretHash: "secret-hash-1",
		Name:       "Test API Key",
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
	}

	// Создаем API ключ
	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Удаляем API ключ
	err = repo.Delete(context.Background(), key.ID)
	require.NoError(t, err)

	// Проверяем, что API ключ был удален
	_, err = repo.FindByID(context.Background(), key.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found")
}
