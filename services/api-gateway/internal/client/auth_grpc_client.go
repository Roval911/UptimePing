package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "UptimePingPlatform/gen/go/proto/api/auth/v1"
)

// GRPCAuthClient gRPC клиент для AuthService
type GRPCAuthClient struct {
	client authv1.AuthServiceClient
	conn   *grpc.ClientConn
}

// NewGRPCAuthClient создает новый gRPC клиент для AuthService
func NewGRPCAuthClient(address string, timeout time.Duration) (*GRPCAuthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to auth service: %w", err)
	}

	// Проверяем соединение
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		return nil, fmt.Errorf("timeout while establishing connection")
	}

	client := authv1.NewAuthServiceClient(conn)

	return &GRPCAuthClient{
		client: client,
		conn:   conn,
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
