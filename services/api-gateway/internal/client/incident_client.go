package client

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"UptimePingPlatform/gen/go/proto/api/incident/v1"
	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/logger"
)

// IncidentClient интерфейс для клиента инцидентов
type IncidentClient interface {
	CreateIncident(ctx context.Context, checkID, tenantID string, severity v1.IncidentSeverity, errorMessage string) (*v1.Incident, error)
	UpdateIncident(ctx context.Context, incidentID string, status v1.IncidentStatus, severity v1.IncidentSeverity) (*v1.Incident, error)
	ResolveIncident(ctx context.Context, incidentID string) (*v1.ResolveIncidentResponse, error)
	ListIncidents(ctx context.Context, tenantID string, status v1.IncidentStatus, severity v1.IncidentSeverity, pageSize, pageToken int32) (*v1.ListIncidentsResponse, error)
	GetIncident(ctx context.Context, incidentID string) (*v1.GetIncidentResponse, error)
	Close() error
}

// GrpcIncidentClient реализация IncidentClient с использованием gRPC
type GrpcIncidentClient struct {
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewGrpcIncidentClient создает новый экземпляр GrpcIncidentClient
func NewGrpcIncidentClient(cfg *config.Config, log logger.Logger) (*GrpcIncidentClient, error) {
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
	address := cfg.IncidentService.Host + ":" + strconv.Itoa(cfg.IncidentService.Port)

	log.Info("Connecting to incident service",
		logger.String("address", address),
		logger.String("host", cfg.IncidentService.Host),
		logger.Int("port", cfg.IncidentService.Port),
	)

	// Создание connection
	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		log.Error("Failed to connect to incident service",
			logger.String("error", err.Error()),
			logger.String("address", address),
		)
		return nil, err
	}

	log.Info("Successfully connected to incident service",
		logger.String("address", address),
	)

	return &GrpcIncidentClient{
		conn:   conn,
		logger: log,
	}, nil
}

// CreateIncident создает новый инцидент
func (c *GrpcIncidentClient) CreateIncident(ctx context.Context, checkID, tenantID string, severity v1.IncidentSeverity, errorMessage string) (*v1.Incident, error) {
	// Создаем клиент
	client := v1.NewIncidentServiceClient(c.conn)

	// Создаем запрос
	request := &v1.CreateIncidentRequest{
		CheckId:      checkID,
		TenantId:     tenantID,
		Severity:     severity,
		ErrorMessage: errorMessage,
	}

	// Выполняем запрос
	response, err := client.CreateIncident(ctx, request)
	if err != nil {
		c.logger.Error("Failed to create incident",
			logger.String("error", err.Error()),
			logger.String("check_id", checkID),
			logger.String("tenant_id", tenantID),
		)
		return nil, err
	}

	c.logger.Debug("Successfully created incident",
		logger.String("incident_id", response.Id),
		logger.String("check_id", response.CheckId),
		logger.String("severity", response.Severity.String()),
	)

	return response, nil
}

// UpdateIncident обновляет существующий инцидент
func (c *GrpcIncidentClient) UpdateIncident(ctx context.Context, incidentID string, status v1.IncidentStatus, severity v1.IncidentSeverity) (*v1.Incident, error) {
	// Создаем клиент
	client := v1.NewIncidentServiceClient(c.conn)

	// Создаем запрос
	request := &v1.UpdateIncidentRequest{
		IncidentId: incidentID,
		Status:     status,
		Severity:   severity,
	}

	// Выполняем запрос
	response, err := client.UpdateIncident(ctx, request)
	if err != nil {
		c.logger.Error("Failed to update incident",
			logger.String("error", err.Error()),
			logger.String("incident_id", incidentID),
			logger.String("status", status.String()),
			logger.String("severity", severity.String()),
		)
		return nil, err
	}

	c.logger.Debug("Successfully updated incident",
		logger.String("incident_id", response.Id),
		logger.String("status", response.Status.String()),
	)

	return response, nil
}

// ResolveIncident закрывает инцидент
func (c *GrpcIncidentClient) ResolveIncident(ctx context.Context, incidentID string) (*v1.ResolveIncidentResponse, error) {
	// Создаем клиент
	client := v1.NewIncidentServiceClient(c.conn)

	// Создаем запрос
	request := &v1.ResolveIncidentRequest{
		IncidentId: incidentID,
	}

	// Выполняем запрос
	response, err := client.ResolveIncident(ctx, request)
	if err != nil {
		c.logger.Error("Failed to resolve incident",
			logger.String("error", err.Error()),
			logger.String("incident_id", incidentID),
		)
		return nil, err
	}

	c.logger.Debug("Successfully resolved incident",
		logger.String("incident_id", incidentID),
		logger.Bool("success", response.Success),
	)

	return response, nil
}

// ListIncidents возвращает список инцидентов с фильтрацией
func (c *GrpcIncidentClient) ListIncidents(ctx context.Context, tenantID string, status v1.IncidentStatus, severity v1.IncidentSeverity, pageSize, pageToken int32) (*v1.ListIncidentsResponse, error) {
	// Создаем клиент
	client := v1.NewIncidentServiceClient(c.conn)

	// Создаем запрос
	request := &v1.ListIncidentsRequest{
		TenantId:  tenantID,
		Status:    status,
		Severity:  severity,
		PageSize:  pageSize,
		PageToken: pageToken,
	}

	// Выполняем запрос
	response, err := client.ListIncidents(ctx, request)
	if err != nil {
		c.logger.Error("Failed to list incidents",
			logger.String("error", err.Error()),
			logger.String("tenant_id", tenantID),
			logger.String("status", status.String()),
			logger.String("severity", severity.String()),
		)
		return nil, err
	}

	c.logger.Debug("Successfully listed incidents",
		logger.Int("count", len(response.Incidents)),
		logger.String("tenant_id", tenantID),
		logger.Int32("next_page_token", response.NextPageToken),
	)

	return response, nil
}

// GetIncident возвращает детали инцидента
func (c *GrpcIncidentClient) GetIncident(ctx context.Context, incidentID string) (*v1.GetIncidentResponse, error) {
	// Создаем клиент
	client := v1.NewIncidentServiceClient(c.conn)

	// Создаем запрос
	request := &v1.GetIncidentRequest{
		IncidentId: incidentID,
	}

	// Выполняем запрос
	response, err := client.GetIncident(ctx, request)
	if err != nil {
		c.logger.Error("Failed to get incident",
			logger.String("error", err.Error()),
			logger.String("incident_id", incidentID),
		)
		return nil, err
	}

	c.logger.Debug("Successfully got incident",
		logger.String("incident_id", response.Incident.Id),
		logger.String("status", response.Incident.Status.String()),
		logger.Int("event_count", len(response.Events)),
	)

	return response, nil
}

// Close закрывает соединение
func (c *GrpcIncidentClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing incident client connection")
		return c.conn.Close()
	}
	return nil
}
