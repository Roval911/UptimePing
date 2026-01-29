package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	corev1 "UptimePingPlatform/proto/api/core/v1"
)

// CoreClient gRPC клиент для CoreService
type CoreClient struct {
	client      corev1.CoreServiceClient
	conn        *grpc.ClientConn
	baseHandler *grpcBase.BaseHandler
}

// NewCoreClient создает новый gRPC клиент для CoreService
func NewCoreClient(address string, timeout time.Duration, logger logger.Logger) (*CoreClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Инициализируем BaseHandler
	baseHandler := grpcBase.NewBaseHandler(logger)

	// Логируем начало операции
	baseHandler.LogOperationStart(ctx, "grpc_core_client_connect", map[string]interface{}{
		"address": address,
		"timeout": timeout.String(),
	})

	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		baseHandler.LogError(ctx, err, "grpc_core_client_connect_failed", "")
		return nil, fmt.Errorf("failed to connect to core service: %w", err)
	}

	// Проверяем соединение
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		baseHandler.LogError(ctx, fmt.Errorf("timeout while establishing connection"), "grpc_core_client_connect_timeout", "")
		return nil, fmt.Errorf("timeout while establishing connection")
	}

	client := corev1.NewCoreServiceClient(conn)

	// Логируем успешное подключение
	baseHandler.LogOperationSuccess(ctx, "grpc_core_client_connect", map[string]interface{}{
		"address": address,
	})

	return &CoreClient{
		client:      client,
		conn:        conn,
		baseHandler: baseHandler,
	}, nil
}

// Close закрывает соединение
func (c *CoreClient) Close() error {
	return c.conn.Close()
}

// ExecuteCheck выполняет проверку
func (c *CoreClient) ExecuteCheck(ctx context.Context, req *corev1.ExecuteCheckRequest) (*corev1.CheckResult, error) {
	return c.client.ExecuteCheck(ctx, req)
}

// GetCheckStatus получает статус проверки
func (c *CoreClient) GetCheckStatus(ctx context.Context, req *corev1.GetCheckStatusRequest) (*corev1.CheckStatusResponse, error) {
	return c.client.GetCheckStatus(ctx, req)
}

// GetCheckHistory получает историю выполнения проверки
func (c *CoreClient) GetCheckHistory(ctx context.Context, req *corev1.GetCheckHistoryRequest) (*corev1.GetCheckHistoryResponse, error) {
	return c.client.GetCheckHistory(ctx, req)
}
