package client

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"UptimePingPlatform/gen/go/proto/api/config/v1"
	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/logger"
)

// ConfigClient интерфейс для клиента конфигурации
type ConfigClient interface {
	GetCheck(ctx context.Context, id string) (*v1.Check, error)
	ListChecks(ctx context.Context, tenantID string) ([]*v1.Check, error)
	Close() error
}

// GrpcConfigClient реализация ConfigClient с использованием gRPC
type GrpcConfigClient struct {
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewGrpcConfigClient создает новый экземпляр GrpcConfigClient
func NewGrpcConfigClient(cfg *config.Config, log logger.Logger) (*GrpcConfigClient, error) {
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
	address := cfg.ConfigService.Host + ":" + strconv.Itoa(cfg.ConfigService.Port)

	log.Info("Connecting to config service",
		logger.String("address", address),
		logger.String("host", cfg.ConfigService.Host),
		logger.Int("port", cfg.ConfigService.Port),
	)

	// Создание connection
	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		log.Error("Failed to connect to config service",
			logger.String("error", err.Error()),
			logger.String("address", address),
		)
		return nil, err
	}

	log.Info("Successfully connected to config service",
		logger.String("address", address),
	)

	return &GrpcConfigClient{
		conn:   conn,
		logger: log,
	}, nil
}

// GetCheck получает конфигурацию проверки
func (c *GrpcConfigClient) GetCheck(ctx context.Context, id string) (*v1.Check, error) {
	// Создаем клиент
	client := v1.NewConfigServiceClient(c.conn)

	// Создаем запрос - используйте правильное имя поля
	request := &v1.GetCheckRequest{
		CheckId: id, // Замените Id на CheckId
	}

	// Выполняем запрос
	response, err := client.GetCheck(ctx, request)
	if err != nil {
		c.logger.Error("Failed to get check",
			logger.String("error", err.Error()),
			logger.String("check_id", id),
		)
		return nil, err
	}

	c.logger.Debug("Successfully got check",
		logger.String("check_id", response.Id),
		logger.String("name", response.Name),
	)

	return response, nil
}

// ListChecks получает список проверок
func (c *GrpcConfigClient) ListChecks(ctx context.Context, tenantID string) ([]*v1.Check, error) {
	// Создаем клиент
	client := v1.NewConfigServiceClient(c.conn)

	// Создаем запрос
	request := &v1.ListChecksRequest{
		TenantId: tenantID,
	}

	// Выполняем запрос
	response, err := client.ListChecks(ctx, request)
	if err != nil {
		c.logger.Error("Failed to list checks",
			logger.String("error", err.Error()),
			logger.String("tenant_id", tenantID),
		)
		return nil, err
	}

	c.logger.Debug("Successfully listed checks",
		logger.Int("count", len(response.Checks)),
		logger.String("tenant_id", tenantID),
	)

	return response.Checks, nil
}

// Close закрывает соединение
func (c *GrpcConfigClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing config client connection")
		return c.conn.Close()
	}
	return nil
}
