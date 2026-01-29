package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	incidentv1 "UptimePingPlatform/proto/api/incident/v1"
)

// IncidentClient gRPC клиент для IncidentService
type IncidentClient struct {
	client      incidentv1.IncidentServiceClient
	conn        *grpc.ClientConn
	baseHandler *grpcBase.BaseHandler
}

// NewIncidentClient создает новый gRPC клиент для IncidentService
func NewIncidentClient(address string, timeout time.Duration, logger logger.Logger) (*IncidentClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Инициализируем BaseHandler
	baseHandler := grpcBase.NewBaseHandler(logger)

	// Логируем начало операции
	baseHandler.LogOperationStart(ctx, "grpc_incident_client_connect", map[string]interface{}{
		"address": address,
		"timeout": timeout.String(),
	})

	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		baseHandler.LogError(ctx, err, "grpc_incident_client_connect_failed", "")
		return nil, fmt.Errorf("failed to connect to incident service: %w", err)
	}

	// Проверяем соединение
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		baseHandler.LogError(ctx, fmt.Errorf("timeout while establishing connection"), "grpc_incident_client_connect_timeout", "")
		return nil, fmt.Errorf("timeout while establishing connection")
	}

	client := incidentv1.NewIncidentServiceClient(conn)

	// Логируем успешное подключение
	baseHandler.LogOperationSuccess(ctx, "grpc_incident_client_connect", map[string]interface{}{
		"address": address,
	})

	return &IncidentClient{
		client:      client,
		conn:        conn,
		baseHandler: baseHandler,
	}, nil
}

// Close закрывает соединение
func (c *IncidentClient) Close() error {
	return c.conn.Close()
}

// CreateIncident создает новый инцидент
func (c *IncidentClient) CreateIncident(ctx context.Context, req *incidentv1.CreateIncidentRequest) (*incidentv1.Incident, error) {
	return c.client.CreateIncident(ctx, req)
}

// GetIncident получает инцидент по ID
func (c *IncidentClient) GetIncident(ctx context.Context, req *incidentv1.GetIncidentRequest) (*incidentv1.Incident, error) {
	resp, err := c.client.GetIncident(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Incident, nil
}

// ListIncidents получает список инцидентов
func (c *IncidentClient) ListIncidents(ctx context.Context, req *incidentv1.ListIncidentsRequest) (*incidentv1.ListIncidentsResponse, error) {
	return c.client.ListIncidents(ctx, req)
}

// ResolveIncident закрывает инцидент
func (c *IncidentClient) ResolveIncident(ctx context.Context, req *incidentv1.ResolveIncidentRequest) (*incidentv1.ResolveIncidentResponse, error) {
	return c.client.ResolveIncident(ctx, req)
}
