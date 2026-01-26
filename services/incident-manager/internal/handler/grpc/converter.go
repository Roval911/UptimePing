package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"UptimePingPlatform/services/incident-manager/internal/domain"

	pb "UptimePingPlatform/gen/go/proto/api/incident/v1"
)

// validateCreateIncidentRequest валидирует запрос на создание инцидента
func (h *IncidentHandler) validateCreateIncidentRequest(ctx context.Context, req *pb.CreateIncidentRequest) error {
	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(
		ctx,
		"create_incident",
		map[string]string{
			"check_id":  req.CheckId,
			"tenant_id": req.TenantId,
		},
	); err != nil {
		return err
	}

	// Базовая валидация длины
	if len(req.CheckId) == 0 || len(req.CheckId) > 255 {
		return status.Errorf(codes.InvalidArgument, "check_id must be between 1 and 255 characters")
	}
	if len(req.TenantId) == 0 || len(req.TenantId) > 255 {
		return status.Errorf(codes.InvalidArgument, "tenant_id must be between 1 and 255 characters")
	}

	return nil
}

// validateUpdateIncidentRequest валидирует запрос на обновление инцидента
func (h *IncidentHandler) validateUpdateIncidentRequest(ctx context.Context, req *pb.UpdateIncidentRequest) error {
	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(
		ctx,
		"update_incident",
		map[string]string{
			"incident_id": req.IncidentId,
		},
	); err != nil {
		return err
	}

	// Базовая валидация длины
	if len(req.IncidentId) == 0 || len(req.IncidentId) > 255 {
		return status.Errorf(codes.InvalidArgument, "incident_id must be between 1 and 255 characters")
	}

	// Валидация enum значений
	switch req.Status {
	case pb.IncidentStatus_INCIDENT_STATUS_OPEN,
		pb.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED,
		pb.IncidentStatus_INCIDENT_STATUS_RESOLVED,
		pb.IncidentStatus_INCIDENT_STATUS_UNSPECIFIED:
		// OK
	default:
		return status.Errorf(codes.InvalidArgument, "invalid incident status")
	}

	switch req.Severity {
	case pb.IncidentSeverity_INCIDENT_SEVERITY_WARNING,
		pb.IncidentSeverity_INCIDENT_SEVERITY_ERROR,
		pb.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL,
		pb.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED:
		// OK
	default:
		return status.Errorf(codes.InvalidArgument, "invalid incident severity")
	}

	return nil
}

// validateResolveIncidentRequest валидирует запрос на закрытие инцидента
func (h *IncidentHandler) validateResolveIncidentRequest(ctx context.Context, req *pb.ResolveIncidentRequest) error {
	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(
		ctx,
		"resolve_incident",
		map[string]string{
			"incident_id": req.IncidentId,
		},
	); err != nil {
		return err
	}

	// Базовая валидация длины
	if len(req.IncidentId) == 0 || len(req.IncidentId) > 255 {
		return status.Errorf(codes.InvalidArgument, "incident_id must be between 1 and 255 characters")
	}

	return nil
}

// validateListIncidentsRequest валидирует запрос на получение списка инцидентов
func (h *IncidentHandler) validateListIncidentsRequest(ctx context.Context, req *pb.ListIncidentsRequest) error {
	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(
		ctx,
		"list_incidents",
		map[string]string{
			"tenant_id": req.TenantId,
		},
	); err != nil {
		return err
	}

	// Базовая валидация длины
	if len(req.TenantId) == 0 || len(req.TenantId) > 255 {
		return status.Errorf(codes.InvalidArgument, "tenant_id must be between 1 and 255 characters")
	}

	// Валидация пагинации
	if req.PageSize < 0 {
		return status.Errorf(codes.InvalidArgument, "page_size must be non-negative")
	}
	if req.PageToken < 0 {
		return status.Errorf(codes.InvalidArgument, "page_token must be non-negative")
	}

	// Валидация enum значений
	switch req.Status {
	case pb.IncidentStatus_INCIDENT_STATUS_OPEN,
		pb.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED,
		pb.IncidentStatus_INCIDENT_STATUS_RESOLVED,
		pb.IncidentStatus_INCIDENT_STATUS_UNSPECIFIED:
		// OK
	default:
		return status.Errorf(codes.InvalidArgument, "invalid incident status")
	}

	switch req.Severity {
	case pb.IncidentSeverity_INCIDENT_SEVERITY_WARNING,
		pb.IncidentSeverity_INCIDENT_SEVERITY_ERROR,
		pb.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL,
		pb.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED:
		// OK
	default:
		return status.Errorf(codes.InvalidArgument, "invalid incident severity")
	}

	return nil
}

// validateGetIncidentRequest валидирует запрос на получение инцидента
func (h *IncidentHandler) validateGetIncidentRequest(ctx context.Context, req *pb.GetIncidentRequest) error {
	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(
		ctx,
		"get_incident",
		map[string]string{
			"incident_id": req.IncidentId,
		},
	); err != nil {
		return err
	}

	// Базовая валидация длины
	if len(req.IncidentId) == 0 || len(req.IncidentId) > 255 {
		return status.Errorf(codes.InvalidArgument, "incident_id must be between 1 and 255 characters")
	}

	return nil
}

// incidentToProto конвертирует доменный инцидент в protobuf
func (h *IncidentHandler) incidentToProto(incident *domain.Incident) *pb.Incident {
	return &pb.Incident{
		Id:           incident.ID,
		CheckId:      incident.CheckID,
		TenantId:     incident.TenantID,
		Status:       h.domainStatusToProto(incident.Status),
		Severity:     h.domainSeverityToProto(incident.Severity),
		FirstSeen:    incident.FirstSeen.Format(time.RFC3339),
		LastSeen:     incident.LastSeen.Format(time.RFC3339),
		Count:        int32(incident.Count),
		ErrorMessage: incident.ErrorMessage,
		ErrorHash:    incident.ErrorHash,
	}
}

// incidentEventToProto конвертирует событие инцидента в protobuf
func (h *IncidentHandler) incidentEventToProto(event *domain.IncidentEvent) *pb.IncidentEvent {
	return &pb.IncidentEvent{
		Id:          event.ID,
		IncidentId:  event.IncidentID,
		Type:        event.EventType,
		Description: event.Message,
		CreatedAt:   event.CreatedAt.Format(time.RFC3339),
		UserId:      "", // TODO: Add user tracking
	}
}

// domainStatusToProto конвертирует статус домена в protobuf
func (h *IncidentHandler) domainStatusToProto(status domain.IncidentStatus) pb.IncidentStatus {
	switch status {
	case domain.IncidentStatusOpen:
		return pb.IncidentStatus_INCIDENT_STATUS_OPEN
	case domain.IncidentStatusAcknowledged:
		return pb.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED
	case domain.IncidentStatusResolved:
		return pb.IncidentStatus_INCIDENT_STATUS_RESOLVED
	default:
		return pb.IncidentStatus_INCIDENT_STATUS_UNSPECIFIED
	}
}

// protoStatusToDomain конвертирует статус protobuf в домен
func (h *IncidentHandler) protoStatusToDomain(status pb.IncidentStatus) domain.IncidentStatus {
	switch status {
	case pb.IncidentStatus_INCIDENT_STATUS_OPEN:
		return domain.IncidentStatusOpen
	case pb.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED:
		return domain.IncidentStatusAcknowledged
	case pb.IncidentStatus_INCIDENT_STATUS_RESOLVED:
		return domain.IncidentStatusResolved
	default:
		return domain.IncidentStatusOpen
	}
}

// domainSeverityToProto конвертирует серьезность домена в protobuf
func (h *IncidentHandler) domainSeverityToProto(severity domain.IncidentSeverity) pb.IncidentSeverity {
	switch severity {
	case domain.IncidentSeverityWarning:
		return pb.IncidentSeverity_INCIDENT_SEVERITY_WARNING
	case domain.IncidentSeverityError:
		return pb.IncidentSeverity_INCIDENT_SEVERITY_ERROR
	case domain.IncidentSeverityCritical:
		return pb.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL
	default:
		return pb.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED
	}
}

// protoSeverityToDomain конвертирует серьезность protobuf в домен
func (h *IncidentHandler) protoSeverityToDomain(severity pb.IncidentSeverity) domain.IncidentSeverity {
	switch severity {
	case pb.IncidentSeverity_INCIDENT_SEVERITY_WARNING:
		return domain.IncidentSeverityWarning
	case pb.IncidentSeverity_INCIDENT_SEVERITY_ERROR:
		return domain.IncidentSeverityError
	case pb.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL:
		return domain.IncidentSeverityCritical
	default:
		return domain.IncidentSeverityWarning
	}
}
