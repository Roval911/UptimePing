package service

import (
	"context"

	"UptimePingPlatform/services/api-gateway/internal/client"
	httphandler "UptimePingPlatform/services/api-gateway/internal/handler/http"
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
func (a *AuthHTTPAdapter) Login(ctx context.Context, email, password string) (*httphandler.TokenPair, error) {
	tokenPair, err := a.authClient.Login(ctx, email, password)
	if err != nil {
		return nil, err
	}

	return &httphandler.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

// Register выполняет регистрацию пользователя через HTTP клиент
func (a *AuthHTTPAdapter) Register(ctx context.Context, email, password, tenantName string) (*httphandler.TokenPair, error) {
	tokenPair, err := a.authClient.Register(ctx, email, password, tenantName)
	if err != nil {
		return nil, err
	}

	return &httphandler.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

// RefreshToken обновляет токен доступа через HTTP клиент
func (a *AuthHTTPAdapter) RefreshToken(ctx context.Context, refreshToken string) (*httphandler.TokenPair, error) {
	tokenPair, err := a.authClient.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	return &httphandler.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}
