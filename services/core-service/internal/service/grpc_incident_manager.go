package service

import (
	"context"
	"time"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/core-service/internal/client"
	"UptimePingPlatform/services/core-service/internal/domain"
	incidentv1 "UptimePingPlatform/proto/api/incident/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCIncidentManager реализация IncidentManager через gRPC клиент
type GRPCIncidentManager struct {
	client client.IncidentClient
	logger logger.Logger
}

// NewGRPCIncidentManager создает новый gRPC Incident Manager
func NewGRPCIncidentManager(client client.IncidentClient, logger logger.Logger) IncidentManager {
	return &GRPCIncidentManager{
		client: client,
		logger: logger,
	}
}

// CreateIncident создает новый инцидент
func (g *GRPCIncidentManager) CreateIncident(ctx context.Context, incident *Incident) (*Incident, error) {
	g.logger.Debug("Creating incident",
		logger.String("check_id", incident.CheckID),
		logger.String("tenant_id", incident.TenantID),
	)

	// Создаем CheckResult из Incident для передачи в клиент
	checkResult := &domain.CheckResult{
		CheckID:     incident.CheckID,
		ExecutionID: incident.ExecutionID,
		Success:     false, // Инциденты создаются только для неудачных проверок
		Error:       incident.Error,
		StatusCode:  incident.StatusCode,
		CheckedAt:   incident.CreatedAt,
		Metadata:    make(map[string]string),
	}
	
	// Конвертируем Metadata из map[string]interface{} в map[string]string
	for k, v := range incident.Metadata {
		if str, ok := v.(string); ok {
			checkResult.Metadata[k] = str
		}
	}

	// Вызов gRPC сервиса с правильными параметрами
	createdIncident, err := g.client.CreateIncident(ctx, checkResult, incident.TenantID)
	if err != nil {
		g.logger.Error("Failed to create incident",
			logger.String("check_id", incident.CheckID),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to create incident")
	}

	g.logger.Info("Incident created successfully",
		logger.String("incident_id", createdIncident.Id),
		logger.String("check_id", incident.CheckID),
	)

	// Конвертируем protobuf в доменную модель
	result := g.convertFromProtoIncident(createdIncident)
	return result, nil
}

// UpdateIncident обновляет существующий инцидент
func (g *GRPCIncidentManager) UpdateIncident(ctx context.Context, incidentID string, updates *IncidentUpdates) (*Incident, error) {
	g.logger.Debug("Updating incident",
		logger.String("incident_id", incidentID),
	)

	// Получаем текущий инцидент
	current, err := g.GetIncident(ctx, incidentID)
	if err != nil {
		return nil, err
	}

	// Применяем обновления
	if updates.Status != nil {
		current.Status = *updates.Status
	}
	if updates.Severity != nil {
		current.Severity = *updates.Severity
	}
	if updates.Title != nil {
		current.Title = *updates.Title
	}
	if updates.Descripton != nil {
		current.Description = *updates.Descripton
	}
	if updates.Metadata != nil {
		current.Metadata = updates.Metadata
	}
	current.UpdatedAt = time.Now()

	// Обновляем через gRPC
	var protoStatus incidentv1.IncidentStatus
	switch current.Status {
	case IncidentStatusOpen:
		protoStatus = incidentv1.IncidentStatus_INCIDENT_STATUS_OPEN
	case IncidentStatusClosed:
		protoStatus = incidentv1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED
	case IncidentStatusResolved:
		protoStatus = incidentv1.IncidentStatus_INCIDENT_STATUS_RESOLVED
	case IncidentStatusSuppressed:
		protoStatus = incidentv1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED
	}
	
	var protoSeverity incidentv1.IncidentSeverity
	switch current.Severity {
	case IncidentSeverityLow:
		protoSeverity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_WARNING
	case IncidentSeverityMedium:
		protoSeverity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_ERROR
	case IncidentSeverityHigh:
		protoSeverity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_ERROR
	case IncidentSeverityCritical:
		protoSeverity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL
	}
	
	updatedIncident, err := g.client.UpdateIncident(ctx, incidentID, protoStatus, protoSeverity)
	if err != nil {
		g.logger.Error("Failed to update incident",
			logger.String("incident_id", incidentID),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to update incident")
	}

	g.logger.Info("Incident updated successfully",
		logger.String("incident_id", incidentID),
	)

	// Конвертируем protobuf в доменную модель
	result := g.convertFromProtoIncident(updatedIncident)
	return result, nil
}

// ResolveIncident закрывает инцидент
func (g *GRPCIncidentManager) ResolveIncident(ctx context.Context, incidentID string) error {
	g.logger.Debug("Resolving incident",
		logger.String("incident_id", incidentID),
	)

	err := g.client.ResolveIncident(ctx, incidentID)
	if err != nil {
		g.logger.Error("Failed to resolve incident",
			logger.String("incident_id", incidentID),
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to resolve incident")
	}

	g.logger.Info("Incident resolved successfully",
		logger.String("incident_id", incidentID),
	)

	return nil
}

// GetIncident получает инцидент по ID
func (g *GRPCIncidentManager) GetIncident(ctx context.Context, incidentID string) (*Incident, error) {
	g.logger.Debug("Getting incident",
		logger.String("incident_id", incidentID),
	)

	protoIncident, err := g.client.GetIncident(ctx, incidentID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errors.New(errors.ErrNotFound, "incident not found")
		}
		g.logger.Error("Failed to get incident",
			logger.String("incident_id", incidentID),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get incident")
	}

	result := g.convertFromProtoIncident(protoIncident)

	g.logger.Debug("Incident retrieved successfully",
		logger.String("incident_id", incidentID),
	)

	return result, nil
}

// ListIncidents получает список инцидентов
func (g *GRPCIncidentManager) ListIncidents(ctx context.Context, filters *IncidentFilters) ([]*Incident, error) {
	g.logger.Debug("Listing incidents",
		logger.String("tenant_id", filters.TenantID),
	)

	// Конвертация фильтров в protobuf
	var protoStatus incidentv1.IncidentStatus
	if filters.Status != "" {
		switch filters.Status {
		case IncidentStatusOpen:
			protoStatus = incidentv1.IncidentStatus_INCIDENT_STATUS_OPEN
		case IncidentStatusClosed:
			protoStatus = incidentv1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED
		case IncidentStatusResolved:
			protoStatus = incidentv1.IncidentStatus_INCIDENT_STATUS_RESOLVED
		case IncidentStatusSuppressed:
			protoStatus = incidentv1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED
		}
	}

	var protoSeverity incidentv1.IncidentSeverity
	if filters.Severity != "" {
		switch filters.Severity {
		case IncidentSeverityLow:
			protoSeverity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_WARNING
		case IncidentSeverityMedium:
			protoSeverity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_ERROR
		case IncidentSeverityHigh:
			protoSeverity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_ERROR
		case IncidentSeverityCritical:
			protoSeverity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL
		}
	}

	protoIncidents, _, err := g.client.ListIncidents(
		ctx,
		filters.TenantID,
		protoStatus,
		protoSeverity,
		int32(filters.Limit),
		int32(filters.Offset),
	)

	if err != nil {
		g.logger.Error("Failed to list incidents",
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to list incidents")
	}

	var results []*Incident
	for _, protoIncident := range protoIncidents {
		incident := g.convertFromProtoIncident(protoIncident)
		results = append(results, incident)
	}

	g.logger.Debug("Incidents listed successfully",
		logger.Int("count", len(results)),
	)

	return results, nil
}

// GetActiveIncidents получает активные инциденты
func (g *GRPCIncidentManager) GetActiveIncidents(ctx context.Context, tenantID string) ([]*Incident, error) {
	g.logger.Debug("Getting active incidents",
		logger.String("tenant_id", tenantID),
	)

	filters := &IncidentFilters{
		TenantID: tenantID,
		Status:   IncidentStatusOpen,
		Limit:    100, // Ограничение для активных инцидентов
	}

	return g.ListIncidents(ctx, filters)
}

// convertFromProtoIncident конвертирует protobuf Incident в доменную модель
func (g *GRPCIncidentManager) convertFromProtoIncident(protoIncident *incidentv1.Incident) *Incident {
	incident := &Incident{
		ID:          protoIncident.Id,
		CheckID:     protoIncident.CheckId,
		TenantID:    protoIncident.TenantId,
		Error:       protoIncident.ErrorMessage,
		CreatedAt:   time.Now(), // Нет временных полей в protobuf
		UpdatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	// Конвертация статуса
	switch protoIncident.Status {
	case incidentv1.IncidentStatus_INCIDENT_STATUS_OPEN:
		incident.Status = IncidentStatusOpen
	case incidentv1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED:
		incident.Status = IncidentStatusClosed
	case incidentv1.IncidentStatus_INCIDENT_STATUS_RESOLVED:
		incident.Status = IncidentStatusResolved
	}

	// Конвертация серьезности
	switch protoIncident.Severity {
	case incidentv1.IncidentSeverity_INCIDENT_SEVERITY_WARNING:
		incident.Severity = IncidentSeverityLow
	case incidentv1.IncidentSeverity_INCIDENT_SEVERITY_ERROR:
		incident.Severity = IncidentSeverityMedium
	case incidentv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL:
		incident.Severity = IncidentSeverityCritical
	}

	return incident
}
