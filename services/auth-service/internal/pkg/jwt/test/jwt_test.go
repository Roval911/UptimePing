package jwt_test

import (
	"testing"
	"time"

	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager_GenerateAndValidateToken(t *testing.T) {
	// Создаем менеджер JWT
	manager := jwt.NewManager(
		"test-access-secret-key-1234567890",
		"test-refresh-secret-key-1234567890",
		15*time.Minute,
		7*24*time.Hour,
	)

	// Генерируем токен
	accessToken, refreshToken, err := manager.GenerateToken("user-1", "tenant-1", false)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)

	// Валидируем access токен
	accessClaims, err := manager.ValidateAccessToken(accessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-1", accessClaims.UserID)
	assert.Equal(t, "tenant-1", accessClaims.TenantID)
	assert.False(t, accessClaims.IsAdmin)
	assert.Equal(t, "access", accessClaims.TokenType)
	assert.WithinDuration(t, time.Now().UTC(), accessClaims.RegisteredClaims.IssuedAt.Time, time.Second)
	assert.WithinDuration(t, time.Now().UTC().Add(15*time.Minute), accessClaims.RegisteredClaims.ExpiresAt.Time, time.Second)

	// Валидируем refresh токен
	refreshClaims, err := manager.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)
	assert.Equal(t, "user-1", refreshClaims.UserID)
	assert.Equal(t, "tenant-1", refreshClaims.TenantID)
	assert.False(t, refreshClaims.IsAdmin)
	assert.Equal(t, "refresh", refreshClaims.TokenType)
	assert.WithinDuration(t, time.Now().UTC(), refreshClaims.RegisteredClaims.IssuedAt.Time, time.Second)
	assert.WithinDuration(t, time.Now().UTC().Add(7*24*time.Hour), refreshClaims.RegisteredClaims.ExpiresAt.Time, time.Second)
}

func TestJWTManager_ValidateInvalidToken(t *testing.T) {
	// Создаем менеджер JWT
	manager := jwt.NewManager(
		"test-access-secret-key-1234567890",
		"test-refresh-secret-key-1234567890",
		15*time.Minute,
		7*24*time.Hour,
	)

	// Пытаемся валидировать невалидный токен
	claims, err := manager.ValidateAccessToken("invalid-token")
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "failed to parse token")

	claims, err = manager.ValidateRefreshToken("invalid-token")
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "failed to parse token")
}

func TestJWTManager_ValidateExpiredToken(t *testing.T) {
	// Создаем менеджер JWT с очень коротким TTL
	manager := jwt.NewManager(
		"test-access-secret-key-1234567890",
		"test-refresh-secret-key-1234567890",
		1*time.Millisecond, // Короткий TTL
		7*24*time.Hour,
	)

	// Генерируем токен
	accessToken, _, err := manager.GenerateToken("user-1", "tenant-1", false)
	require.NoError(t, err)

	// Ждем, пока токен истечет
	time.Sleep(2 * time.Millisecond)

	// Пытаемся валидировать истекший токен
	claims, err := manager.ValidateAccessToken(accessToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "token is expired")
}

func TestJWTManager_GenerateAccessToken(t *testing.T) {
	// Создаем менеджер JWT
	manager := jwt.NewManager(
		"test-access-secret-key-1234567890",
		"test-refresh-secret-key-1234567890",
		15*time.Minute,
		7*24*time.Hour,
	)

	// Генерируем только access токен
	accessToken, err := manager.GenerateAccessToken("user-1", "tenant-1", true)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)

	// Валидируем access токен
	claims, err := manager.ValidateAccessToken(accessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "tenant-1", claims.TenantID)
	assert.True(t, claims.IsAdmin)
	assert.Equal(t, "access", claims.TokenType)

	// Пытаемся валидировать access токен как refresh (должна быть ошибка)
	_, err = manager.ValidateRefreshToken(accessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate refresh token")
}

func TestJWTManager_GenerateRefreshToken(t *testing.T) {
	// Создаем менеджер JWT
	manager := jwt.NewManager(
		"test-access-secret-key-1234567890",
		"test-refresh-secret-key-1234567890",
		15*time.Minute,
		7*24*time.Hour,
	)

	// Генерируем только refresh токен
	refreshToken, err := manager.GenerateRefreshToken("user-1", "tenant-1", false)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshToken)

	// Валидируем refresh токен
	claims, err := manager.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "tenant-1", claims.TenantID)
	assert.False(t, claims.IsAdmin)
	assert.Equal(t, "refresh", claims.TokenType)
	assert.WithinDuration(t, time.Now().UTC(), claims.RegisteredClaims.IssuedAt.Time, time.Second)
	assert.WithinDuration(t, time.Now().UTC().Add(7*24*time.Hour), claims.RegisteredClaims.ExpiresAt.Time, time.Second)

	// Пытаемся валидировать refresh токен как access (должна быть ошибка)
	_, err = manager.ValidateAccessToken(refreshToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate access token")
}

func TestJWTManager_TokenTypeValidation(t *testing.T) {
	// Создаем менеджер JWT с разными ключами
	manager := jwt.NewManager(
		"access-secret-key",
		"refresh-secret-key",
		15*time.Minute,
		7*24*time.Hour,
	)

	// Генерируем пару токенов
	accessToken, refreshToken, err := manager.GenerateToken("user-1", "tenant-1", true)
	require.NoError(t, err)

	// Проверяем, что access токен нельзя валидировать как refresh
	accessAsRefresh, err := manager.ValidateRefreshToken(accessToken)
	assert.Error(t, err)
	assert.Nil(t, accessAsRefresh)
	assert.Contains(t, err.Error(), "failed to validate refresh token")

	// Проверяем, что refresh токен нельзя валидировать как access
	refreshAsAccess, err := manager.ValidateAccessToken(refreshToken)
	assert.Error(t, err)
	assert.Nil(t, refreshAsAccess)
	assert.Contains(t, err.Error(), "failed to validate access token")
}

func TestJWTManager_DifferentSecrets(t *testing.T) {
	// Создаем менеджер с разными секретными ключами
	manager1 := jwt.NewManager(
		"secret-key-1",
		"secret-key-2",
		15*time.Minute,
		7*24*time.Hour,
	)

	manager2 := jwt.NewManager(
		"different-secret-key-1",
		"different-secret-key-2",
		15*time.Minute,
		7*24*time.Hour,
	)

	// Генерируем токен с первым менеджером
	accessToken, refreshToken, err := manager1.GenerateToken("user-1", "tenant-1", false)
	require.NoError(t, err)

	// Пытаемся валидировать токен вторым менеджером (должна быть ошибка)
	_, err = manager2.ValidateAccessToken(accessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate access token")

	_, err = manager2.ValidateRefreshToken(refreshToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate refresh token")
}

func TestJWTManager_InvalidSigningMethod(t *testing.T) {
	// Создаем менеджер JWT
	manager := jwt.NewManager(
		"test-access-secret-key-1234567890",
		"test-refresh-secret-key-1234567890",
		15*time.Minute,
		7*24*time.Hour,
	)

	// Создаем токен с неправильным методом подписи (для теста)
	// В реальности такого не будет, но тестируем обработку ошибок
	token := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.TJVA95OrM7E2cBab30RMHrHDcEfxjoYZgeFONFh7HgQ"

	_, err := manager.ValidateAccessToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected signing method")
}
