package service

import (
	"context"
	"fmt"
	"time"

	"UptimePingPlatform/services/core-service/internal/domain"
	incidentv1 "UptimePingPlatform/gen/proto/api/incident/v1"
)

// IncidentManager определяет интерфейс для работы с инцидентами
type IncidentManager interface {
	// CreateIncident создает новый инцидент
	CreateIncident(ctx context.Context, incident *Incident) (*Incident, error)
	
	// UpdateIncident обновляет существующий инцидент
	UpdateIncident(ctx context.Context, incidentID string, updates *IncidentUpdates) (*Incident, error)
	
	// ResolveIncident закрывает инцидент
	ResolveIncident(ctx context.Context, incidentID string) error
	
	// GetIncident получает инцидент по ID
	GetIncident(ctx context.Context, incidentID string) (*Incident, error)
	
	// ListIncidents получает список инцидентов
	ListIncidents(ctx context.Context, filters *IncidentFilters) ([]*Incident, error)
	
	// GetActiveIncidents получает активные инциденты
	GetActiveIncidents(ctx context.Context, tenantID string) ([]*Incident, error)
}

// Incident представляет инцидент
type Incident struct {
	ID          string                 `json:"id"`
	CheckID     string                 `json:"check_id"`
	ExecutionID string                 `json:"execution_id"`
	TenantID    string                 `json:"tenant_id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Status      IncidentStatus         `json:"status"`
	Severity    IncidentSeverity       `json:"severity"`
	Error       string                 `json:"error"`
	StatusCode  int                    `json:"status_code"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

// IncidentStatus статус инцидента
type IncidentStatus string

const (
	IncidentStatusOpen     IncidentStatus = "open"
	IncidentStatusClosed   IncidentStatus = "closed"
	IncidentStatusResolved IncidentStatus = "resolved"
	IncidentStatusSuppressed IncidentStatus = "suppressed"
)

// IncidentSeverity уровень серьезности инцидента
type IncidentSeverity string

const (
	IncidentSeverityLow      IncidentSeverity = "low"
	IncidentSeverityMedium   IncidentSeverity = "medium"
	IncidentSeverityHigh     IncidentSeverity = "high"
	IncidentSeverityCritical IncidentSeverity = "critical"
)

// IncidentUpdates обновления инцидента
type IncidentUpdates struct {
	Status     *IncidentStatus   `json:"status,omitempty"`
	Severity   *IncidentSeverity `json:"severity,omitempty"`
	Title      *string           `json:"title,omitempty"`
	Descripton *string           `json:"description,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// IncidentFilters фильтры для списка инцидентов
type IncidentFilters struct {
	TenantID   string            `json:"tenant_id,omitempty"`
	Status     IncidentStatus    `json:"status,omitempty"`
	Severity   IncidentSeverity  `json:"severity,omitempty"`
	StartTime  *time.Time        `json:"start_time,omitempty"`
	EndTime    *time.Time        `json:"end_time,omitempty"`
	Limit      int               `json:"limit,omitempty"`
	Offset     int               `json:"offset,omitempty"`
}

// ConvertToProtoIncident конвертирует внутренний Incident в protobuf
func (i *Incident) ConvertToProtoIncident() *incidentv1.Incident {
	protoIncident := &incidentv1.Incident{
		Id:       i.ID,
		CheckId:  i.CheckID,
		TenantId: i.TenantID,
	}
	
	// Конвертация статуса
	switch i.Status {
	case IncidentStatusOpen:
		protoIncident.Status = incidentv1.IncidentStatus_INCIDENT_STATUS_OPEN
	case IncidentStatusClosed:
		protoIncident.Status = incidentv1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED
	case IncidentStatusResolved:
		protoIncident.Status = incidentv1.IncidentStatus_INCIDENT_STATUS_RESOLVED
	case IncidentStatusSuppressed:
		protoIncident.Status = incidentv1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED
	}
	
	// Конвертация серьезности
	switch i.Severity {
	case IncidentSeverityLow:
		protoIncident.Severity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_WARNING
	case IncidentSeverityMedium:
		protoIncident.Severity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_ERROR
	case IncidentSeverityHigh:
		protoIncident.Severity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_ERROR
	case IncidentSeverityCritical:
		protoIncident.Severity = incidentv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL
	}
	
	return protoIncident
}

// CreateIncidentFromCheckResult создает инцидент из результата проверки
func CreateIncidentFromCheckResult(result *domain.CheckResult, tenantID string) *Incident {
	status := IncidentStatusOpen
	severity := IncidentSeverityMedium
	
	// Определяем серьезность на основе типа ошибки
	if result.StatusCode >= 500 {
		severity = IncidentSeverityHigh
	} else if result.StatusCode >= 400 {
		severity = IncidentSeverityMedium
	} else if result.StatusCode == 0 {
		severity = IncidentSeverityCritical // Connection refused, timeout, etc.
	}
	
	title := fmt.Sprintf("Check failed: %s", result.CheckID)
	description := fmt.Sprintf("Check %s failed with status %d", result.CheckID, result.StatusCode)
	
	if result.Error != "" {
		description += fmt.Sprintf(": %s", result.Error)
	}
	
	// Конвертируем Metadata из map[string]string в map[string]interface{}
	metadata := make(map[string]interface{})
	for k, v := range result.Metadata {
		metadata[k] = v
	}
	
	return &Incident{
		CheckID:     result.CheckID,
		TenantID:    tenantID,
		Title:       title,
		Description: description,
		Status:      status,
		Severity:    severity,
		Error:       result.Error,
		StatusCode:  result.StatusCode,
		Metadata:    metadata,
		CreatedAt:   result.CheckedAt,
		UpdatedAt:   result.CheckedAt,
	}
}
