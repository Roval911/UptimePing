package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims структура для хранения пользовательских данных в JWT токене
type TokenClaims struct {
	UserID    string `json:"user_id"`
	TenantID  string `json:"tenant_id"`
	IsAdmin   bool   `json:"is_admin"`
	TokenType string `json:"token_type"` // Добавляем поле для различения типов токенов
	jwt.RegisteredClaims
}

// JWTManager интерфейс для работы с JWT токенами
type JWTManager interface {
	GenerateToken(userID, tenantID string, isAdmin bool) (string, string, error)
	ValidateAccessToken(token string) (*TokenClaims, error)
	ValidateRefreshToken(token string) (*TokenClaims, error)
	GenerateAccessToken(userID, tenantID string, isAdmin bool) (string, error)
	GenerateRefreshToken(userID, tenantID string, isAdmin bool) (string, error)
}

// Manager реализация JWTManager
type Manager struct {
	accessSecretKey  string
	refreshSecretKey string
	accessTokenTTL   time.Duration
	refreshTokenTTL  time.Duration
}

// NewManager создает новый экземпляр JWT менеджера
func NewManager(accessSecretKey, refreshSecretKey string, accessTokenTTL, refreshTokenTTL time.Duration) *Manager {
	return &Manager{
		accessSecretKey:  accessSecretKey,
		refreshSecretKey: refreshSecretKey,
		accessTokenTTL:   accessTokenTTL,
		refreshTokenTTL:  refreshTokenTTL,
	}
}

// GenerateToken генерирует пару access и refresh токенов
func (m *Manager) GenerateToken(userID, tenantID string, isAdmin bool) (string, string, error) {
	accessToken, err := m.GenerateAccessToken(userID, tenantID, isAdmin)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := m.GenerateRefreshToken(userID, tenantID, isAdmin)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// GenerateAccessToken генерирует access токен
func (m *Manager) GenerateAccessToken(userID, tenantID string, isAdmin bool) (string, error) {
	claims := &TokenClaims{
		UserID:    userID,
		TenantID:  tenantID,
		IsAdmin:   isAdmin,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(m.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			NotBefore: jwt.NewNumericDate(time.Now().UTC()),
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.accessSecretKey))
}

// GenerateRefreshToken генерирует refresh токен
func (m *Manager) GenerateRefreshToken(userID, tenantID string, isAdmin bool) (string, error) {
	claims := &TokenClaims{
		UserID:    userID,
		TenantID:  tenantID,
		IsAdmin:   isAdmin,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(m.refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			NotBefore: jwt.NewNumericDate(time.Now().UTC()),
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.refreshSecretKey))
}

// ValidateAccessToken валидирует access токен
func (m *Manager) ValidateAccessToken(token string) (*TokenClaims, error) {
	claims, err := m.validateTokenWithSecret(token, m.accessSecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to validate access token: %w", err)
	}

	// Проверяем тип токена
	if claims.TokenType != "access" {
		return nil, fmt.Errorf("invalid token type: expected 'access', got '%s'", claims.TokenType)
	}

	return claims, nil
}

// ValidateRefreshToken валидирует refresh токен
func (m *Manager) ValidateRefreshToken(token string) (*TokenClaims, error) {
	claims, err := m.validateTokenWithSecret(token, m.refreshSecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to validate refresh token: %w", err)
	}

	// Проверяем тип токена
	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type: expected 'refresh', got '%s'", claims.TokenType)
	}

	return claims, nil
}

// validateTokenWithSecret валидирует токен с указанным секретным ключом
func (m *Manager) validateTokenWithSecret(token, secretKey string) (*TokenClaims, error) {
	parsedToken, err := jwt.ParseWithClaims(token, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := parsedToken.Claims.(*TokenClaims); ok && parsedToken.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
