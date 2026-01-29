package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	schedulerv1 "UptimePingPlatform/proto/api/scheduler/v1"
)

// SchedulerClient gRPC клиент для SchedulerService
type SchedulerClient struct {
	client schedulerv1.SchedulerServiceClient
	conn   *grpc.ClientConn
	baseHandler *grpcBase.BaseHandler
}

// NewSchedulerClient создает новый gRPC клиент для SchedulerService
func NewSchedulerClient(address string, timeout time.Duration, logger logger.Logger) (*SchedulerClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Инициализируем BaseHandler
	baseHandler := grpcBase.NewBaseHandler(logger)

	// Логируем начало операции
	baseHandler.LogOperationStart(ctx, "grpc_scheduler_client_connect", map[string]interface{}{
		"address": address,
		"timeout": timeout.String(),
	})

	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		baseHandler.LogError(ctx, err, "grpc_scheduler_client_connect_failed", "")
		return nil, fmt.Errorf("failed to connect to scheduler service: %w", err)
	}

	// Проверяем соединение
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		baseHandler.LogError(ctx, fmt.Errorf("timeout while establishing connection"), "grpc_scheduler_client_connect_timeout", "")
		return nil, fmt.Errorf("timeout while establishing connection")
	}

	client := schedulerv1.NewSchedulerServiceClient(conn)

	// Логируем успешное подключение
	baseHandler.LogOperationSuccess(ctx, "grpc_scheduler_client_connect", map[string]interface{}{
		"address": address,
	})

	return &SchedulerClient{
		client:      client,
		conn:        conn,
		baseHandler: baseHandler,
	}, nil
}

// Close закрывает соединение
func (c *SchedulerClient) Close() error {
	return c.conn.Close()
}

// CreateCheck создает новую проверку
func (c *SchedulerClient) CreateCheck(ctx context.Context, req *schedulerv1.CreateCheckRequest) (*schedulerv1.Check, error) {
	return c.client.CreateCheck(ctx, req)
}

// GetCheck получает проверку по ID
func (c *SchedulerClient) GetCheck(ctx context.Context, req *schedulerv1.GetCheckRequest) (*schedulerv1.Check, error) {
	return c.client.GetCheck(ctx, req)
}

// ListChecks получает список проверок
func (c *SchedulerClient) ListChecks(ctx context.Context, req *schedulerv1.ListChecksRequest) (*schedulerv1.ListChecksResponse, error) {
	return c.client.ListChecks(ctx, req)
}

// UpdateCheck обновляет проверку
func (c *SchedulerClient) UpdateCheck(ctx context.Context, req *schedulerv1.UpdateCheckRequest) (*schedulerv1.Check, error) {
	return c.client.UpdateCheck(ctx, req)
}

// DeleteCheck удаляет проверку
func (c *SchedulerClient) DeleteCheck(ctx context.Context, req *schedulerv1.DeleteCheckRequest) (*schedulerv1.DeleteCheckResponse, error) {
	return c.client.DeleteCheck(ctx, req)
}

// ScheduleCheck планирует проверку
func (c *SchedulerClient) ScheduleCheck(ctx context.Context, req *schedulerv1.ScheduleCheckRequest) (*schedulerv1.Schedule, error) {
	return c.client.ScheduleCheck(ctx, req)
}

// UnscheduleCheck отменяет планирование проверки
func (c *SchedulerClient) UnscheduleCheck(ctx context.Context, req *schedulerv1.UnscheduleCheckRequest) (*schedulerv1.UnscheduleCheckResponse, error) {
	return c.client.UnscheduleCheck(ctx, req)
}

// GetSchedule получает расписание проверки
func (c *SchedulerClient) GetSchedule(ctx context.Context, req *schedulerv1.GetScheduleRequest) (*schedulerv1.Schedule, error) {
	return c.client.GetSchedule(ctx, req)
}

// ListSchedules получает список расписаний
func (c *SchedulerClient) ListSchedules(ctx context.Context, req *schedulerv1.ListSchedulesRequest) (*schedulerv1.ListSchedulesResponse, error) {
	return c.client.ListSchedules(ctx, req)
}
