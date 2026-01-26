package postgres_test

import (
	"context"
	"testing"
	"time"

	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	// Для тестов используем in-memory PostgreSQL или тестовую базу
	// Пока пропускаем тесты если нет базы данных
	t.Skip("PostgreSQL test database not configured")
	return nil
}

func TestAPIKeyRepository_Create(t *testing.T) {
	// Тестовая база данных
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	// Создаем репозиторий
	repo := postgres.NewAPIKeyRepository(pool)

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

	// Сохраняем ключ
	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Проверяем, что ключ сохранен
	found, err := repo.FindByID(context.Background(), "key-1")
	require.NoError(t, err)
	assert.Equal(t, key.ID, found.ID)
	assert.Equal(t, key.TenantID, found.TenantID)
	assert.Equal(t, key.Name, found.Name)
	assert.Equal(t, key.IsActive, found.IsActive)
}

func TestAPIKeyRepository_FindByID(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewAPIKeyRepository(pool)

	// Создаем тестовый ключ
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key := &domain.APIKey{
		ID:         "key-2",
		TenantID:   "tenant-2",
		KeyHash:    "key-hash-2",
		SecretHash: "secret-hash-2",
		Name:       "Test API Key 2",
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
	}

	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Ищем по ID
	found, err := repo.FindByID(context.Background(), "key-2")
	require.NoError(t, err)
	assert.Equal(t, key.ID, found.ID)
	assert.Equal(t, key.Name, found.Name)

	// Ищем несуществующий ключ
	found, err = repo.FindByID(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Nil(t, found)
}

func TestAPIKeyRepository_FindByKeyHash(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewAPIKeyRepository(pool)

	// Создаем тестовый ключ
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key := &domain.APIKey{
		ID:         "key-3",
		TenantID:   "tenant-3",
		KeyHash:    "key-hash-3",
		SecretHash: "secret-hash-3",
		Name:       "Test API Key 3",
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
	}

	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Ищем по хэшу ключа
	found, err := repo.FindByKeyHash(context.Background(), "key-hash-3")
	require.NoError(t, err)
	assert.Equal(t, key.ID, found.ID)
	assert.Equal(t, key.KeyHash, found.KeyHash)

	// Ищем несуществующий хэш
	found, err = repo.FindByKeyHash(context.Background(), "non-existent-hash")
	assert.Error(t, err)
	assert.Nil(t, found)
}

func TestAPIKeyRepository_ListByTenant(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewAPIKeyRepository(pool)

	// Создаем несколько ключей для одного тенанта
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	keys := []*domain.APIKey{
		{
			ID:         "key-4-1",
			TenantID:   "tenant-4",
			KeyHash:    "key-hash-4-1",
			SecretHash: "secret-hash-4-1",
			Name:       "API Key 1",
			IsActive:   true,
			CreatedAt:  createdAt,
		},
		{
			ID:         "key-4-2",
			TenantID:   "tenant-4",
			KeyHash:    "key-hash-4-2",
			SecretHash: "secret-hash-4-2",
			Name:       "API Key 2",
			IsActive:   true,
			CreatedAt:  createdAt,
		},
	}

	for _, key := range keys {
		err := repo.Create(context.Background(), key)
		require.NoError(t, err)
	}

	// Получаем список ключей тенанта
	foundKeys, err := repo.ListByTenant(context.Background(), "tenant-4")
	require.NoError(t, err)
	assert.Len(t, foundKeys, 2)

	// Проверяем порядок (должны быть отсортированы по created_at DESC)
	assert.Equal(t, "API Key 2", foundKeys[0].Name)
	assert.Equal(t, "API Key 1", foundKeys[1].Name)
}

func TestAPIKeyRepository_Update(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewAPIKeyRepository(pool)

	// Создаем тестовый ключ
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key := &domain.APIKey{
		ID:         "key-5",
		TenantID:   "tenant-5",
		KeyHash:    "key-hash-5",
		SecretHash: "secret-hash-5",
		Name:       "Original Name",
		IsActive:   true,
		CreatedAt:  createdAt,
	}

	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Обновляем ключ
	key.Name = "Updated Name"
	key.IsActive = false

	err = repo.Update(context.Background(), key)
	require.NoError(t, err)

	// Проверяем обновление
	found, err := repo.FindByID(context.Background(), "key-5")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", found.Name)
	assert.Equal(t, false, found.IsActive)
}

func TestAPIKeyRepository_Delete(t *testing.T) {
	pool := setupTestDB(t)
	if pool == nil {
		t.Skip("Skipping test due to no database connection")
		return
	}
	defer pool.Close()

	repo := postgres.NewAPIKeyRepository(pool)

	// Создаем тестовый ключ
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	key := &domain.APIKey{
		ID:         "key-6",
		TenantID:   "tenant-6",
		KeyHash:    "key-hash-6",
		SecretHash: "secret-hash-6",
		Name:       "API Key to Delete",
		IsActive:   true,
		CreatedAt:  createdAt,
	}

	err := repo.Create(context.Background(), key)
	require.NoError(t, err)

	// Удаляем ключ
	err = repo.Delete(context.Background(), "key-6")
	require.NoError(t, err)

	// Проверяем, что ключ удален
	found, err := repo.FindByID(context.Background(), "key-6")
	assert.Error(t, err)
	assert.Nil(t, found)

	// Удаляем несуществующий ключ
	err = repo.Delete(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key not found")
}
