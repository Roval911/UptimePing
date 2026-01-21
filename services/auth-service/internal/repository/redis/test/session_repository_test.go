package redis_test

import (
	"context"
	"testing"
	"time"

	"UptimePingPlatform/services/auth-service/internal/domain"
	authRedis "UptimePingPlatform/services/auth-service/internal/repository/redis" // Алиас для вашего пакета репозитория
	redisClient "github.com/go-redis/redis/v8"                                     // Алиас для Redis клиента
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionRepository_Create(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий с использованием алиаса
	repo := authRedis.NewSessionRepository(client)

	// Создаем тестовую сессию
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	session := &domain.Session{
		ID:               "session-1",
		UserID:           "user-1",
		AccessTokenHash:  "access-token-hash-1",
		RefreshTokenHash: "refresh-token-hash-1",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	// Создаем сессию
	err := repo.Create(context.Background(), session)
	require.NoError(t, err)

	// Проверяем, что сессия была создана по access token hash
	createdSession, err := repo.FindByAccessTokenHash(context.Background(), session.AccessTokenHash)
	require.NoError(t, err)
	assert.Equal(t, session.ID, createdSession.ID)
	assert.Equal(t, session.UserID, createdSession.UserID)
	assert.Equal(t, session.AccessTokenHash, createdSession.AccessTokenHash)
	assert.Equal(t, session.RefreshTokenHash, createdSession.RefreshTokenHash)
	assert.WithinDuration(t, session.ExpiresAt, createdSession.ExpiresAt, time.Second)
	assert.WithinDuration(t, session.CreatedAt, createdSession.CreatedAt, time.Second)

	// Проверяем, что сессия была создана по refresh token hash
	createdSessionByRefresh, err := repo.FindByRefreshTokenHash(context.Background(), session.RefreshTokenHash)
	require.NoError(t, err)
	assert.Equal(t, session.ID, createdSessionByRefresh.ID)
}

func TestSessionRepository_FindByID(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Создаем тестовую сессию
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	session := &domain.Session{
		ID:               "session-1",
		UserID:           "user-1",
		AccessTokenHash:  "access-token-hash-1",
		RefreshTokenHash: "refresh-token-hash-1",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	// Создаем сессию
	err := repo.Create(context.Background(), session)
	require.NoError(t, err)

	// Ищем сессию по ID
	foundSession, err := repo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, foundSession.ID)
	assert.Equal(t, session.UserID, foundSession.UserID)
	assert.Equal(t, session.AccessTokenHash, foundSession.AccessTokenHash)
	assert.Equal(t, session.RefreshTokenHash, foundSession.RefreshTokenHash)
	assert.WithinDuration(t, session.ExpiresAt, foundSession.ExpiresAt, time.Second)
	assert.WithinDuration(t, session.CreatedAt, foundSession.CreatedAt, time.Second)
}

func TestSessionRepository_FindByAccessTokenHash(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Создаем тестовую сессию
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	session := &domain.Session{
		ID:               "session-1",
		UserID:           "user-1",
		AccessTokenHash:  "access-token-hash-1",
		RefreshTokenHash: "refresh-token-hash-1",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	// Создаем сессию
	err := repo.Create(context.Background(), session)
	require.NoError(t, err)

	// Ищем сессию по хэшу access токена
	foundSession, err := repo.FindByAccessTokenHash(context.Background(), session.AccessTokenHash)
	require.NoError(t, err)
	assert.Equal(t, session.ID, foundSession.ID)
	assert.Equal(t, session.AccessTokenHash, foundSession.AccessTokenHash)
	assert.Equal(t, session.UserID, foundSession.UserID)
	assert.Equal(t, session.RefreshTokenHash, foundSession.RefreshTokenHash)
	assert.WithinDuration(t, session.ExpiresAt, foundSession.ExpiresAt, time.Second)
	assert.WithinDuration(t, session.CreatedAt, foundSession.CreatedAt, time.Second)
}

func TestSessionRepository_FindByRefreshTokenHash(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Создаем тестовую сессию
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	session := &domain.Session{
		ID:               "session-1",
		UserID:           "user-1",
		AccessTokenHash:  "access-token-hash-1",
		RefreshTokenHash: "refresh-token-hash-1",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	// Создаем сессию
	err := repo.Create(context.Background(), session)
	require.NoError(t, err)

	// Ищем сессию по хэшу refresh токена
	foundSession, err := repo.FindByRefreshTokenHash(context.Background(), session.RefreshTokenHash)
	require.NoError(t, err)
	assert.Equal(t, session.ID, foundSession.ID)
	assert.Equal(t, session.RefreshTokenHash, foundSession.RefreshTokenHash)
	assert.Equal(t, session.UserID, foundSession.UserID)
	assert.Equal(t, session.AccessTokenHash, foundSession.AccessTokenHash)
	assert.WithinDuration(t, session.ExpiresAt, foundSession.ExpiresAt, time.Second)
	assert.WithinDuration(t, session.CreatedAt, foundSession.CreatedAt, time.Second)
}

func TestSessionRepository_FindByAccessTokenHash_NotFound(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Пытаемся найти несуществующую сессию
	_, err := repo.FindByAccessTokenHash(context.Background(), "non-existent-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSessionRepository_FindByRefreshTokenHash_NotFound(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Пытаемся найти несуществующую сессию
	_, err := repo.FindByRefreshTokenHash(context.Background(), "non-existent-refresh-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSessionRepository_Delete(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Создаем тестовую сессию
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	session := &domain.Session{
		ID:               "session-1",
		UserID:           "user-1",
		AccessTokenHash:  "access-token-hash-1",
		RefreshTokenHash: "refresh-token-hash-1",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	// Создаем сессию
	err := repo.Create(context.Background(), session)
	require.NoError(t, err)

	// Убеждаемся, что сессия существует
	foundSession, err := repo.FindByID(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, foundSession.ID)

	// Удаляем сессию
	err = repo.Delete(context.Background(), session.ID)
	require.NoError(t, err)

	// Проверяем, что сессия была удалена по ID
	_, err = repo.FindByID(context.Background(), session.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")

	// Проверяем, что сессия была удалена по access token hash
	_, err = repo.FindByAccessTokenHash(context.Background(), session.AccessTokenHash)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")

	// Проверяем, что сессия была удалена по refresh token hash
	_, err = repo.FindByRefreshTokenHash(context.Background(), session.RefreshTokenHash)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSessionRepository_DeleteByUserID(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Создаем тестовые сессии для одного пользователя
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	session1 := &domain.Session{
		ID:               "session-1",
		UserID:           "user-1",
		AccessTokenHash:  "access-token-hash-1",
		RefreshTokenHash: "refresh-token-hash-1",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	session2 := &domain.Session{
		ID:               "session-2",
		UserID:           "user-1",
		AccessTokenHash:  "access-token-hash-2",
		RefreshTokenHash: "refresh-token-hash-2",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	// Создаем сессию для другого пользователя
	session3 := &domain.Session{
		ID:               "session-3",
		UserID:           "user-2",
		AccessTokenHash:  "access-token-hash-3",
		RefreshTokenHash: "refresh-token-hash-3",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	// Создаем все сессии
	err := repo.Create(context.Background(), session1)
	require.NoError(t, err)

	err = repo.Create(context.Background(), session2)
	require.NoError(t, err)

	err = repo.Create(context.Background(), session3)
	require.NoError(t, err)

	// Удаляем все сессии пользователя user-1
	err = repo.DeleteByUserID(context.Background(), "user-1")
	require.NoError(t, err)

	// Проверяем, что сессии user-1 были удалены
	_, err = repo.FindByAccessTokenHash(context.Background(), session1.AccessTokenHash)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")

	_, err = repo.FindByAccessTokenHash(context.Background(), session2.AccessTokenHash)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")

	// Проверяем, что сессия user-2 осталась
	foundSession3, err := repo.FindByAccessTokenHash(context.Background(), session3.AccessTokenHash)
	require.NoError(t, err)
	assert.Equal(t, session3.ID, foundSession3.ID)
	assert.Equal(t, "user-2", foundSession3.UserID)
}

func TestSessionRepository_CleanupExpired(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Создаем тестовую сессию с истекшим сроком действия
	expiredAt := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Microsecond) // Просроченная сессия
	createdAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)

	expiredSession := &domain.Session{
		ID:               "expired-session",
		UserID:           "user-1",
		AccessTokenHash:  "expired-access-token",
		RefreshTokenHash: "expired-refresh-token",
		ExpiresAt:        expiredAt,
		CreatedAt:        createdAt,
	}

	// Создаем тестовую сессию с действующим сроком
	validExpiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)

	validSession := &domain.Session{
		ID:               "valid-session",
		UserID:           "user-1",
		AccessTokenHash:  "valid-access-token",
		RefreshTokenHash: "valid-refresh-token",
		ExpiresAt:        validExpiresAt,
		CreatedAt:        time.Now().UTC().Truncate(time.Microsecond),
	}

	// Создаем обе сессии
	err := repo.Create(context.Background(), expiredSession)
	require.NoError(t, err)

	err = repo.Create(context.Background(), validSession)
	require.NoError(t, err)

	// Выполняем очистку просроченных сессий
	before := time.Now().UTC()
	err = repo.CleanupExpired(context.Background(), before)
	require.NoError(t, err)

	// Проверяем, что просроченная сессия была удалена
	_, err = repo.FindByAccessTokenHash(context.Background(), expiredSession.AccessTokenHash)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")

	// Проверяем, что действующая сессия осталась
	foundValidSession, err := repo.FindByAccessTokenHash(context.Background(), validSession.AccessTokenHash)
	require.NoError(t, err)
	assert.Equal(t, validSession.ID, foundValidSession.ID)
	assert.Equal(t, validSession.UserID, foundValidSession.UserID)
	assert.Equal(t, validSession.AccessTokenHash, foundValidSession.AccessTokenHash)
}

func TestSessionRepository_CleanupExpired_NoExpiredSessions(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Создаем тестовую сессию с действующим сроком
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	session := &domain.Session{
		ID:               "valid-session",
		UserID:           "user-1",
		AccessTokenHash:  "valid-access-token",
		RefreshTokenHash: "valid-refresh-token",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	// Создаем сессию
	err := repo.Create(context.Background(), session)
	require.NoError(t, err)

	// Выполняем очистку просроченных сессий с временем в прошлом
	before := time.Now().UTC().Add(-48 * time.Hour) // Время, когда сессия еще не истекла
	err = repo.CleanupExpired(context.Background(), before)
	require.NoError(t, err)

	// Проверяем, что сессия осталась
	foundSession, err := repo.FindByAccessTokenHash(context.Background(), session.AccessTokenHash)
	require.NoError(t, err)
	assert.Equal(t, session.ID, foundSession.ID)
	assert.Equal(t, session.UserID, foundSession.UserID)
}

func TestSessionRepository_ConcurrentAccess(t *testing.T) {
	// Тестовый Redis клиент
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	// Создаем репозиторий
	repo := authRedis.NewSessionRepository(client)

	// Создаем тестовую сессию
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	session := &domain.Session{
		ID:               "session-1",
		UserID:           "user-1",
		AccessTokenHash:  "access-token-hash-1",
		RefreshTokenHash: "refresh-token-hash-1",
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
	}

	// Создаем сессию в горутине
	errCh := make(chan error, 1)
	go func() {
		errCh <- repo.Create(context.Background(), session)
	}()

	// Ждем завершения горутины
	err := <-errCh
	require.NoError(t, err)

	// Параллельно читаем сессию
	var foundSession *domain.Session
	var findErr error
	done := make(chan bool)

	go func() {
		foundSession, findErr = repo.FindByID(context.Background(), session.ID)
		done <- true
	}()

	<-done
	require.NoError(t, findErr)
	assert.Equal(t, session.ID, foundSession.ID)
}

// setupTestRedis создает клиент для тестового Redis
func setupTestRedis(t *testing.T) *redisClient.Client {
	// Проверяем, установлена ли переменная окружения для тестового Redis
	// Если нет, используем in-memory решение или пропускаем тест

	// Для реального Redis (закомментируйте эту часть, если используете мок):
	client := redisClient.NewClient(&redisClient.Options{
		Addr:     "localhost:6379",
		Password: "", // нет пароля
		DB:       1,  // Используем базу 1 для тестов, чтобы не затирать production данные
	})

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		t.Skipf("Skipping Redis tests because Redis is not available at localhost:6379: %v", err)
		return nil
	}

	// Очищаем тестовую базу перед тестом
	err = client.FlushDB(ctx).Err()
	require.NoError(t, err, "Failed to flush Redis database")

	return client
}

// cleanupTestRedis очищает Redis после теста
func cleanupTestRedis(t *testing.T, client *redisClient.Client) {
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Очищаем базу после теста
	err := client.FlushDB(ctx).Err()
	if err != nil {
		t.Logf("Warning: failed to flush Redis database after test: %v", err)
	}

	// Закрываем соединение
	err = client.Close()
	if err != nil {
		t.Logf("Warning: failed to close Redis client: %v", err)
	}
}
