package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	metricsv1 "UptimePingPlatform/proto/api/metrics/v1"
)

// MetricsClient gRPC клиент для MetricsService
type MetricsClient struct {
	client      metricsv1.MetricsServiceClient
	conn        *grpc.ClientConn
	baseHandler *grpcBase.BaseHandler
}

// NewMetricsClient создает новый gRPC клиент для MetricsService
func NewMetricsClient(address string, timeout time.Duration, logger logger.Logger) (*MetricsClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Инициализируем BaseHandler
	baseHandler := grpcBase.NewBaseHandler(logger)

	// Логируем начало операции
	baseHandler.LogOperationStart(ctx, "grpc_metrics_client_connect", map[string]interface{}{
		"address": address,
		"timeout": timeout.String(),
	})

	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		baseHandler.LogError(ctx, err, "grpc_metrics_client_connect_failed", "")
		return nil, fmt.Errorf("failed to connect to metrics service: %w", err)
	}

	// Проверяем соединение
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		baseHandler.LogError(ctx, fmt.Errorf("timeout while establishing connection"), "grpc_metrics_client_connect_timeout", "")
		return nil, fmt.Errorf("timeout while establishing connection")
	}

	client := metricsv1.NewMetricsServiceClient(conn)

	// Логируем успешное подключение
	baseHandler.LogOperationSuccess(ctx, "grpc_metrics_client_connect", map[string]interface{}{
		"address": address,
	})

	return &MetricsClient{
		client:      client,
		conn:        conn,
		baseHandler: baseHandler,
	}, nil
}

// Close закрывает соединение
func (c *MetricsClient) Close() error {
	return c.conn.Close()
}

// GetMetrics получает метрики
func (c *MetricsClient) GetMetrics(ctx context.Context, req *metricsv1.GetMetricsRequest) (*metricsv1.GetMetricsResponse, error) {
	return c.client.GetMetrics(ctx, req)
}

// CollectMetrics собирает метрики
func (c *MetricsClient) CollectMetrics(ctx context.Context, req *metricsv1.CollectMetricsRequest) (*metricsv1.CollectMetricsResponse, error) {
	return c.client.CollectMetrics(ctx, req)
}
