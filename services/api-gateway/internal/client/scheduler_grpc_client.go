package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	schedulerv1 "UptimePingPlatform/gen/go/proto/api/scheduler/v1"
)

// SchedulerClient gRPC клиент для SchedulerService
type SchedulerClient struct {
	client schedulerv1.SchedulerServiceClient
	conn   *grpc.ClientConn
}

// NewSchedulerClient создает новый gRPC клиент для SchedulerService
func NewSchedulerClient(address string, timeout time.Duration) (*SchedulerClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to scheduler service: %w", err)
	}

	// Проверяем соединение
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		return nil, fmt.Errorf("timeout while establishing connection")
	}

	client := schedulerv1.NewSchedulerServiceClient(conn)

	return &SchedulerClient{
		client: client,
		conn:   conn,
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
