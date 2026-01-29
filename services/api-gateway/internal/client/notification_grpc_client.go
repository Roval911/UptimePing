package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	notificationv1 "UptimePingPlatform/proto/api/notification/v1"
)

// NotificationClient gRPC клиент для NotificationService
type NotificationClient struct {
	client      notificationv1.NotificationServiceClient
	conn        *grpc.ClientConn
	baseHandler *grpcBase.BaseHandler
}

// NewNotificationClient создает новый gRPC клиент для NotificationService
func NewNotificationClient(address string, timeout time.Duration, logger logger.Logger) (*NotificationClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Инициализируем BaseHandler
	baseHandler := grpcBase.NewBaseHandler(logger)

	// Логируем начало операции
	baseHandler.LogOperationStart(ctx, "grpc_notification_client_connect", map[string]interface{}{
		"address": address,
		"timeout": timeout.String(),
	})

	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		baseHandler.LogError(ctx, err, "grpc_notification_client_connect_failed", "")
		return nil, fmt.Errorf("failed to connect to notification service: %w", err)
	}

	// Проверяем соединение
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		baseHandler.LogError(ctx, fmt.Errorf("timeout while establishing connection"), "grpc_notification_client_connect_timeout", "")
		return nil, fmt.Errorf("timeout while establishing connection")
	}

	client := notificationv1.NewNotificationServiceClient(conn)

	// Логируем успешное подключение
	baseHandler.LogOperationSuccess(ctx, "grpc_notification_client_connect", map[string]interface{}{
		"address": address,
	})

	return &NotificationClient{
		client:      client,
		conn:        conn,
		baseHandler: baseHandler,
	}, nil
}

// Close закрывает соединение
func (c *NotificationClient) Close() error {
	return c.conn.Close()
}

// SendNotification отправляет уведомление
func (c *NotificationClient) SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	return c.client.SendNotification(ctx, req)
}

// GetNotificationChannels получает каналы уведомлений
func (c *NotificationClient) GetNotificationChannels(ctx context.Context, req *notificationv1.ListChannelsRequest) (*notificationv1.ListChannelsResponse, error) {
	return c.client.ListChannels(ctx, req)
}

// RegisterChannel регистрирует канал уведомлений
func (c *NotificationClient) RegisterChannel(ctx context.Context, req *notificationv1.RegisterChannelRequest) (*notificationv1.Channel, error) {
	return c.client.RegisterChannel(ctx, req)
}
