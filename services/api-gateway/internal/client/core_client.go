package client

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"UptimePingPlatform/gen/go/proto/api/core/v1"
	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/logger"
)

// CoreClient интерфейс для клиента ядра
type CoreClient interface {
	ExecuteCheck(ctx context.Context, checkID string) (*v1.CheckResult, error)
	GetCheckStatus(ctx context.Context, checkID string) (*v1.CheckStatusResponse, error)
	GetCheckHistory(ctx context.Context, checkID string, limit int32, startTime, endTime string) (*v1.GetCheckHistoryResponse, error)
	Close() error
}

// GrpcCoreClient реализация CoreClient с использованием gRPC
type GrpcCoreClient struct {
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewGrpcCoreClient создает новый экземпляр GrpcCoreClient
func NewGrpcCoreClient(cfg *config.Config, log logger.Logger) (*GrpcCoreClient, error) {
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
	address := cfg.CoreService.Host + ":" + strconv.Itoa(cfg.CoreService.Port)

	log.Info("Connecting to core service",
		logger.String("address", address),
		logger.String("host", cfg.CoreService.Host),
		logger.Int("port", cfg.CoreService.Port),
	)

	// Создание connection
	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		log.Error("Failed to connect to core service",
			logger.String("error", err.Error()),
			logger.String("address", address),
		)
		return nil, err
	}

	log.Info("Successfully connected to core service",
		logger.String("address", address),
	)

	return &GrpcCoreClient{
		conn:   conn,
		logger: log,
	}, nil
}

// ExecuteCheck выполняет проверку немедленно
func (c *GrpcCoreClient) ExecuteCheck(ctx context.Context, checkID string) (*v1.CheckResult, error) {
	// Создаем клиент
	client := v1.NewCoreServiceClient(c.conn)

	// Создаем запрос
	request := &v1.ExecuteCheckRequest{
		CheckId: checkID,
	}

	// Выполняем запрос
	response, err := client.ExecuteCheck(ctx, request)
	if err != nil {
		c.logger.Error("Failed to execute check",
			logger.String("error", err.Error()),
			logger.String("check_id", checkID),
		)
		return nil, err
	}

	c.logger.Debug("Successfully executed check",
		logger.String("check_id", response.CheckId),
		logger.Bool("success", response.Success),
		logger.Int32("duration_ms", response.DurationMs),
	)

	return response, nil
}

// GetCheckStatus возвращает текущий статус проверки
func (c *GrpcCoreClient) GetCheckStatus(ctx context.Context, checkID string) (*v1.CheckStatusResponse, error) {
	// Создаем клиент
	client := v1.NewCoreServiceClient(c.conn)

	// Создаем запрос
	request := &v1.GetCheckStatusRequest{
		CheckId: checkID,
	}

	// Выполняем запрос
	response, err := client.GetCheckStatus(ctx, request)
	if err != nil {
		c.logger.Error("Failed to get check status",
			logger.String("error", err.Error()),
			logger.String("check_id", checkID),
		)
		return nil, err
	}

	c.logger.Debug("Successfully got check status",
		logger.String("check_id", response.CheckId),
		logger.Bool("is_healthy", response.IsHealthy),
		logger.Int32("response_time_ms", response.ResponseTimeMs),
	)

	return response, nil
}

// GetCheckHistory возвращает историю выполнения проверки
func (c *GrpcCoreClient) GetCheckHistory(ctx context.Context, checkID string, limit int32, startTime, endTime string) (*v1.GetCheckHistoryResponse, error) {
	// Создаем клиент
	client := v1.NewCoreServiceClient(c.conn)

	// Создаем запрос
	request := &v1.GetCheckHistoryRequest{
		CheckId:   checkID,
		Limit:     limit,
		StartTime: startTime,
		EndTime:   endTime,
	}

	// Выполняем запрос
	response, err := client.GetCheckHistory(ctx, request)
	if err != nil {
		c.logger.Error("Failed to get check history",
			logger.String("error", err.Error()),
			logger.String("check_id", checkID),
			logger.Int32("limit", limit),
			logger.String("start_time", startTime),
			logger.String("end_time", endTime),
		)
		return nil, err
	}

	c.logger.Debug("Successfully got check history",
		logger.String("check_id", checkID),
		logger.Int("results_count", len(response.Results)),
	)

	return response, nil
}

// Close закрывает соединение
func (c *GrpcCoreClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing core client connection")
		return c.conn.Close()
	}
	return nil
}
