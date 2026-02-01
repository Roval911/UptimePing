package service

import (
	"context"

	"UptimePingPlatform/services/api-gateway/internal/client"
)

// AuthHTTPAdapter адаптирует HTTPAuthClient к интерфейсу AuthService
type AuthHTTPAdapter struct {
	authClient *client.HTTPAuthClient
}

// NewAuthHTTPAdapter создает новый HTTP адаптер
func NewAuthHTTPAdapter(authClient *client.HTTPAuthClient) *AuthHTTPAdapter {
	return &AuthHTTPAdapter{
		authClient: authClient,
	}
}

// Login выполняет вход пользователя через HTTP клиент
func (a *AuthHTTPAdapter) Login(ctx context.Context, email, password string) (*client.TokenPair, error) {
	return a.authClient.Login(ctx, email, password)
}

// Register выполняет регистрацию пользователя через HTTP клиент
func (a *AuthHTTPAdapter) Register(ctx context.Context, email, password, tenantName string) (*client.TokenPair, error) {
	return a.authClient.Register(ctx, email, password, tenantName)
}

// RefreshToken обновляет токен доступа через HTTP клиент
func (a *AuthHTTPAdapter) RefreshToken(ctx context.Context, refreshToken string) (*client.TokenPair, error) {
	return a.authClient.RefreshToken(ctx, refreshToken)
}
