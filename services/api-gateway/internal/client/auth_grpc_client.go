package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	authv1 "UptimePingPlatform/gen/go/proto/api/auth/v1"
)

// GRPCAuthClient gRPC клиент для AuthService
type GRPCAuthClient struct {
	client authv1.AuthServiceClient
	conn   *grpc.ClientConn
	baseHandler *grpcBase.BaseHandler
}

// NewGRPCAuthClient создает новый gRPC клиент для AuthService
func NewGRPCAuthClient(address string, timeout time.Duration, logger logger.Logger) (*GRPCAuthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Инициализируем BaseHandler
	baseHandler := grpcBase.NewBaseHandler(logger)

	// Логируем начало операции
	baseHandler.LogOperationStart(ctx, "grpc_auth_client_connect", map[string]interface{}{
		"address": address,
		"timeout": timeout.String(),
	})

	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		baseHandler.LogError(ctx, err, "grpc_auth_client_connect_failed", "")
		return nil, fmt.Errorf("failed to connect to auth service: %w", err)
	}

	// Проверяем соединение
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		baseHandler.LogError(ctx, fmt.Errorf("timeout while establishing connection"), "grpc_auth_client_connect_timeout", "")
		return nil, fmt.Errorf("timeout while establishing connection")
	}

	client := authv1.NewAuthServiceClient(conn)

	// Логируем успешное подключение
	baseHandler.LogOperationSuccess(ctx, "grpc_auth_client_connect", map[string]interface{}{
		"address": address,
	})

	return &GRPCAuthClient{
		client:      client,
		conn:        conn,
		baseHandler: baseHandler,
	}, nil
}

// Close закрывает соединение
func (c *GRPCAuthClient) Close() error {
	return c.conn.Close()
}

// ValidateToken проверяет валидность JWT токена
func (c *GRPCAuthClient) ValidateToken(ctx context.Context, token string) (*TokenClaims, error) {
	req := &authv1.ValidateTokenRequest{
		Token: token,
	}

	resp, err := c.client.ValidateToken(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	if !resp.IsValid {
		return nil, fmt.Errorf("invalid token")
	}

	return &TokenClaims{
		UserID:   resp.UserId,
		TenantID: resp.TenantId,
		IsAdmin:  false, // TODO: добавить в proto если нужно
	}, nil
}

// ValidateAPIKey проверяет валидность API ключа
func (c *GRPCAuthClient) ValidateAPIKey(ctx context.Context, key, secret string) (*APIKeyClaims, error) {
	req := &authv1.ValidateAPIKeyRequest{
		Key:    key,
		Secret: secret,
	}

	resp, err := c.client.ValidateAPIKey(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate API key: %w", err)
	}

	if !resp.IsValid {
		return nil, fmt.Errorf("invalid API key")
	}

	return &APIKeyClaims{
		TenantID: resp.TenantId,
		KeyID:    resp.KeyId,
	}, nil
}

// Login выполняет вход пользователя
func (c *GRPCAuthClient) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	c.baseHandler.LogOperationStart(ctx, "grpc_login", map[string]interface{}{
		"email": email,
	})

	req := &authv1.LoginRequest{
		Email:    email,
		Password: password,
	}

	resp, err := c.client.Login(ctx, req)
	if err != nil {
		c.baseHandler.LogError(ctx, err, "grpc_login_failed", "")
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	c.baseHandler.LogOperationSuccess(ctx, "grpc_login", map[string]interface{}{
		"email": email,
	})

	return &TokenPair{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}, nil
}

// Register выполняет регистрацию пользователя
func (c *GRPCAuthClient) Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error) {
	c.baseHandler.LogOperationStart(ctx, "grpc_register", map[string]interface{}{
		"email":       email,
		"tenant_name": tenantName,
	})

	req := &authv1.RegisterRequest{
		Email:      email,
		Password:   password,
		TenantName: tenantName,
	}

	resp, err := c.client.Register(ctx, req)
	if err != nil {
		c.baseHandler.LogError(ctx, err, "grpc_register_failed", "")
		return nil, fmt.Errorf("failed to register: %w", err)
	}

	c.baseHandler.LogOperationSuccess(ctx, "grpc_register", map[string]interface{}{
		"email":       email,
		"tenant_name": tenantName,
	})

	return &TokenPair{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}, nil
}

// RefreshToken обновляет токен доступа
func (c *GRPCAuthClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	c.baseHandler.LogOperationStart(ctx, "grpc_refresh_token", map[string]interface{}{})

	req := &authv1.RefreshTokenRequest{
		RefreshToken: refreshToken,
	}

	resp, err := c.client.RefreshToken(ctx, req)
	if err != nil {
		c.baseHandler.LogError(ctx, err, "grpc_refresh_token_failed", "")
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	c.baseHandler.LogOperationSuccess(ctx, "grpc_refresh_token", map[string]interface{}{})

	return &TokenPair{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}, nil
}

// Logout выполняет выход пользователя
func (c *GRPCAuthClient) Logout(ctx context.Context, userID, tokenID string) error {
	c.baseHandler.LogOperationStart(ctx, "grpc_logout", map[string]interface{}{
		"user_id":  userID,
		"token_id": tokenID,
	})

	req := &authv1.LogoutRequest{
		UserId:       userID,
		RefreshToken: tokenID, // Используем refreshToken как tokenId
	}

	_, err := c.client.Logout(ctx, req)
	if err != nil {
		c.baseHandler.LogError(ctx, err, "grpc_logout_failed", "")
		return fmt.Errorf("failed to logout: %w", err)
	}

	c.baseHandler.LogOperationSuccess(ctx, "grpc_logout", map[string]interface{}{
		"user_id":  userID,
		"token_id": tokenID,
	})

	return nil
}

// TokenPair структура для хранения пары токенов
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// TokenClaims структура для данных JWT токена
type TokenClaims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	IsAdmin  bool   `json:"is_admin"`
}

// APIKeyClaims структура для данных API ключа
type APIKeyClaims struct {
	TenantID string `json:"tenant_id"`
	KeyID    string `json:"key_id"`
}
