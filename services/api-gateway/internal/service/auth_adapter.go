package service

import (
	"context"

	"UptimePingPlatform/services/api-gateway/internal/client"
)

// AuthAdapter адаптирует HTTPAuthClient к интерфейсу AuthService
type AuthAdapter struct {
	authClient *client.HTTPAuthClient
}

// NewAuthAdapter создает новый адаптер
func NewAuthAdapter(authClient *client.HTTPAuthClient) *AuthAdapter {
	return &AuthAdapter{
		authClient: authClient,
	}
}

// Login выполняет вход пользователя через gRPC клиент
func (a *AuthAdapter) Login(ctx context.Context, email, password string) (*client.TokenPair, error) {
	return a.authClient.Login(ctx, email, password)
}

// Register выполняет регистрацию пользователя через gRPC клиент
func (a *AuthAdapter) Register(ctx context.Context, email, password, tenantName string) (*client.TokenPair, error) {
	return a.authClient.Register(ctx, email, password, tenantName)
}

// RefreshToken обновляет токен доступа через gRPC клиент
func (a *AuthAdapter) RefreshToken(ctx context.Context, refreshToken string) (*client.TokenPair, error) {
	return a.authClient.RefreshToken(ctx, refreshToken)
}

// Logout выполняет выход пользователя через HTTP клиент
func (a *AuthAdapter) Logout(ctx context.Context, userID, tokenID string) error {
	// HTTP клиент принимает только accessToken, используем tokenID как accessToken
	return a.authClient.Logout(ctx, tokenID)
}

// ValidateToken валидирует токен через gRPC клиент
func (a *AuthAdapter) ValidateToken(ctx context.Context, token string) (*client.UserInfo, error) {
	return a.authClient.ValidateToken(ctx, token)
}
