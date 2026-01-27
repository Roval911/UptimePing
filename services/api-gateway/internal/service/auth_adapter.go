package service

import (
	"context"

	"UptimePingPlatform/services/api-gateway/internal/client"
	httphandler "UptimePingPlatform/services/api-gateway/internal/handler/http"
)

// AuthAdapter адаптирует GRPCAuthClient к интерфейсу AuthService
type AuthAdapter struct {
	authClient *client.GRPCAuthClient
}

// NewAuthAdapter создает новый адаптер
func NewAuthAdapter(authClient *client.GRPCAuthClient) *AuthAdapter {
	return &AuthAdapter{
		authClient: authClient,
	}
}

// Login выполняет вход пользователя через gRPC клиент
func (a *AuthAdapter) Login(ctx context.Context, email, password string) (*httphandler.TokenPair, error) {
	tokenPair, err := a.authClient.Login(ctx, email, password)
	if err != nil {
		return nil, err
	}

	return &httphandler.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

// Register выполняет регистрацию пользователя через gRPC клиент
func (a *AuthAdapter) Register(ctx context.Context, email, password, tenantName string) (*httphandler.TokenPair, error) {
	tokenPair, err := a.authClient.Register(ctx, email, password, tenantName)
	if err != nil {
		return nil, err
	}

	return &httphandler.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

// RefreshToken обновляет токен доступа через gRPC клиент
func (a *AuthAdapter) RefreshToken(ctx context.Context, refreshToken string) (*httphandler.TokenPair, error) {
	tokenPair, err := a.authClient.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	return &httphandler.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

// Logout выполняет выход пользователя через gRPC клиент
func (a *AuthAdapter) Logout(ctx context.Context, userID, tokenID string) error {
	return a.authClient.Logout(ctx, userID, tokenID)
}
