package client

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"UptimePingPlatform/gen/go/proto/api/auth/v1"
	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/logger"
)

// AuthClient интерфейс для клиента аутентификации
type AuthClient interface {
	ValidateToken(ctx context.Context, token string) (*v1.ValidateTokenResponse, error)
	ValidateAPIKey(ctx context.Context, key, secret string) (*v1.ValidateAPIKeyResponse, error)
	Close() error
}

// GrpcAuthClient реализация AuthClient с использованием gRPC
type GrpcAuthClient struct {
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewGrpcAuthClient создает новый экземпляр GrpcAuthClient
func NewGrpcAuthClient(cfg *config.Config, log logger.Logger) (*GrpcAuthClient, error) {
	// Настройка keepalive
	keepAliveParams := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             2 * time.Second,
		PermitWithoutStream: true,
	}

	// Подключение с опциями
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepAliveParams),
	}

	// Формируем адрес сервиса - преобразуем порт в строку
	address := cfg.AuthService.Host + ":" + strconv.Itoa(cfg.AuthService.Port)

	log.Info("Connecting to auth service",
		logger.String("address", address),
		logger.String("host", cfg.AuthService.Host),
		logger.Int("port", cfg.AuthService.Port),
	)

	// Создание connection
	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		log.Error("Failed to connect to auth service",
			logger.String("error", err.Error()),
			logger.String("address", address),
		)
		return nil, err
	}

	log.Info("Successfully connected to auth service",
		logger.String("address", address),
	)

	return &GrpcAuthClient{
		conn:   conn,
		logger: log,
	}, nil
}

// ValidateToken проверяет валидность токена
func (c *GrpcAuthClient) ValidateToken(ctx context.Context, token string) (*v1.ValidateTokenResponse, error) {
	// Создаем клиент
	client := v1.NewAuthServiceClient(c.conn)

	// Создаем запрос
	request := &v1.ValidateTokenRequest{
		Token: token,
	}

	// Выполняем запрос
	response, err := client.ValidateToken(ctx, request)
	if err != nil {
		c.logger.Error("Failed to validate token",
			logger.String("error", err.Error()),
		)
		return nil, err
	}

	c.logger.Debug("Token validated successfully",
		logger.String("user_id", response.UserId),
		logger.String("tenant_id", response.TenantId),
	)

	return response, nil
}

// ValidateAPIKey проверяет валидность API ключа
func (c *GrpcAuthClient) ValidateAPIKey(ctx context.Context, key, secret string) (*v1.ValidateAPIKeyResponse, error) {
	// Создаем клиент
	client := v1.NewAuthServiceClient(c.conn)

	// Создаем запрос
	request := &v1.ValidateAPIKeyRequest{
		Key:    key,
		Secret: secret,
	}

	// Выполняем запрос
	response, err := client.ValidateAPIKey(ctx, request)
	if err != nil {
		c.logger.Error("Failed to validate API key",
			logger.String("error", err.Error()),
		)
		return nil, err
	}

	c.logger.Debug("API key validated successfully",
		logger.String("tenant_id", response.TenantId),
		logger.String("key_id", response.KeyId),
	)

	return response, nil
}

// Close закрывает соединение
func (c *GrpcAuthClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing auth client connection")
		return c.conn.Close()
	}
	return nil
}
