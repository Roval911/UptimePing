package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/incident-manager/internal/domain"
	"UptimePingPlatform/services/incident-manager/internal/service"

	pb "UptimePingPlatform/gen/go/proto/api/incident/v1"
)

// IncidentHandler реализует gRPC handler для управления инцидентами
type IncidentHandler struct {
	grpcBase.BaseHandler
	service service.IncidentService
	logger  logger.Logger
}

// NewIncidentHandler создает новый handler
func NewIncidentHandler(service service.IncidentService, logger logger.Logger) *IncidentHandler {
	baseHandler := grpcBase.NewBaseHandler(logger)
	return &IncidentHandler{
		BaseHandler: *baseHandler,
		service:     service,
		logger:      logger,
	}
}

// CreateIncident создает новый инцидент
func (h *IncidentHandler) CreateIncident(ctx context.Context, req *pb.CreateIncidentRequest) (*pb.Incident, error) {
	h.LogOperationStart(ctx, "CreateIncident", map[string]interface{}{
		"check_id":  req.CheckId,
		"tenant_id": req.TenantId,
	})

	// Валидация запроса
	if err := h.validateCreateIncidentRequest(ctx, req); err != nil {
		h.LogError(ctx, err, "CreateIncident", req.CheckId)
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Создаем результат проверки
	result := &service.CheckResult{
		CheckID:      req.CheckId,
		TenantID:     req.TenantId,
		IsSuccess:    false, // Если создаем инцидент, значит проверка неуспешная
		ErrorMessage: req.ErrorMessage,
		Duration:     0,
		Timestamp:    time.Now(),
		Metadata:     map[string]interface{}{},
	}

	// Обрабатываем результат проверки
	incident, err := h.service.ProcessCheckResult(ctx, result)
	if err != nil {
		h.LogError(ctx, err, "CreateIncident", req.CheckId)
		return nil, status.Errorf(codes.Internal, "failed to process check result: %v", err)
	}

	// Конвертируем в protobuf
	pbIncident := h.incidentToProto(incident)

	h.LogOperationSuccess(ctx, "CreateIncident", map[string]interface{}{
		"incident_id": incident.ID,
		"check_id":    req.CheckId,
		"tenant_id":   req.TenantId,
	})

	return pbIncident, nil
}

// UpdateIncident обновляет существующий инцидент
func (h *IncidentHandler) UpdateIncident(ctx context.Context, req *pb.UpdateIncidentRequest) (*pb.Incident, error) {
	h.LogOperationStart(ctx, "UpdateIncident", map[string]interface{}{
		"incident_id": req.IncidentId,
	})

	// Валидация запроса
	if err := h.validateUpdateIncidentRequest(ctx, req); err != nil {
		h.LogError(ctx, err, "UpdateIncident", req.IncidentId)
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Получаем инцидент
	incident, err := h.service.GetIncident(ctx, req.IncidentId)
	if err != nil {
		h.LogError(ctx, err, "UpdateIncident", req.IncidentId)
		return nil, status.Errorf(codes.NotFound, "incident not found: %v", err)
	}

	// Обновляем поля
	if req.Status != pb.IncidentStatus_INCIDENT_STATUS_UNSPECIFIED {
		incident.Status = h.protoStatusToDomain(req.Status)
	}
	if req.Severity != pb.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED {
		incident.Severity = h.protoSeverityToDomain(req.Severity)
	}

	// Сохраняем изменения
	err = h.service.UpdateIncident(ctx, incident)
	if err != nil {
		h.LogError(ctx, err, "UpdateIncident", req.IncidentId)
		return nil, status.Errorf(codes.Internal, "failed to update incident: %v", err)
	}

	// Конвертируем в protobuf
	pbIncident := h.incidentToProto(incident)

	h.LogOperationSuccess(ctx, "UpdateIncident", map[string]interface{}{
		"incident_id": incident.ID,
	})

	return pbIncident, nil
}

// ResolveIncident закрывает инцидент
func (h *IncidentHandler) ResolveIncident(ctx context.Context, req *pb.ResolveIncidentRequest) (*pb.ResolveIncidentResponse, error) {
	h.LogOperationStart(ctx, "ResolveIncident", map[string]interface{}{
		"incident_id": req.IncidentId,
	})

	// Валидация запроса
	if err := h.validateResolveIncidentRequest(ctx, req); err != nil {
		h.LogError(ctx, err, "ResolveIncident", req.IncidentId)
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Закрываем инцидент
	err := h.service.ResolveIncident(ctx, req.IncidentId)
	if err != nil {
		h.LogError(ctx, err, "ResolveIncident", req.IncidentId)
		return nil, status.Errorf(codes.Internal, "failed to resolve incident: %v", err)
	}

	h.LogOperationSuccess(ctx, "ResolveIncident", map[string]interface{}{
		"incident_id": req.IncidentId,
	})

	return &pb.ResolveIncidentResponse{
		Success: true,
	}, nil
}

// ListIncidents получает список инцидентов с фильтрацией и пагинацией
func (h *IncidentHandler) ListIncidents(ctx context.Context, req *pb.ListIncidentsRequest) (*pb.ListIncidentsResponse, error) {
	h.LogOperationStart(ctx, "ListIncidents", map[string]interface{}{
		"tenant_id": req.TenantId,
		"page_size": req.PageSize,
	})

	// Валидация запроса
	if err := h.validateListIncidentsRequest(ctx, req); err != nil {
		h.LogError(ctx, err, "ListIncidents", req.TenantId)
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Создаем фильтр
	filter := &domain.IncidentFilter{
		TenantID: &req.TenantId,
	}

	// Добавляем фильтры
	if req.Status != pb.IncidentStatus_INCIDENT_STATUS_UNSPECIFIED {
		status := h.protoStatusToDomain(req.Status)
		filter.Status = &status
	}
	if req.Severity != pb.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED {
		severity := h.protoSeverityToDomain(req.Severity)
		filter.Severity = &severity
	}

	// Добавляем пагинацию
	if req.PageSize > 0 {
		filter.Limit = int(req.PageSize)
		if req.PageToken > 0 {
			filter.Offset = int(req.PageToken)
		}
	}

	// Получаем список инцидентов
	incidents, err := h.service.GetIncidents(ctx, filter)
	if err != nil {
		h.LogError(ctx, err, "ListIncidents", req.TenantId)
		return nil, status.Errorf(codes.Internal, "failed to get incidents: %v", err)
	}

	// Конвертируем в protobuf
	pbIncidents := make([]*pb.Incident, len(incidents))
	for i, incident := range incidents {
		pbIncidents[i] = h.incidentToProto(incident)
	}

	h.LogOperationSuccess(ctx, "ListIncidents", map[string]interface{}{
		"tenant_id": req.TenantId,
		"count":     len(incidents),
	})

	return &pb.ListIncidentsResponse{
		Incidents:      pbIncidents,
		NextPageToken:  0, // TODO: Implement proper pagination
	}, nil
}

// GetIncident получает инцидент с историей
func (h *IncidentHandler) GetIncident(ctx context.Context, req *pb.GetIncidentRequest) (*pb.GetIncidentResponse, error) {
	h.LogOperationStart(ctx, "GetIncident", map[string]interface{}{
		"incident_id": req.IncidentId,
	})

	// Валидация запроса
	if err := h.validateGetIncidentRequest(ctx, req); err != nil {
		h.LogError(ctx, err, "GetIncident", req.IncidentId)
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Получаем инцидент
	incident, err := h.service.GetIncident(ctx, req.IncidentId)
	if err != nil {
		h.LogError(ctx, err, "GetIncident", req.IncidentId)
		return nil, status.Errorf(codes.NotFound, "incident not found: %v", err)
	}

	// Получаем историю инцидента
	history, err := h.service.GetIncidentHistory(ctx, req.IncidentId)
	if err != nil {
		h.LogError(ctx, err, "GetIncident", req.IncidentId)
		return nil, status.Errorf(codes.Internal, "failed to get incident history: %v", err)
	}

	// Конвертируем в protobuf
	pbIncident := h.incidentToProto(incident)
	pbHistory := make([]*pb.IncidentEvent, len(history))
	for i, event := range history {
		pbHistory[i] = h.incidentEventToProto(event)
	}

	h.LogOperationSuccess(ctx, "GetIncident", map[string]interface{}{
		"incident_id": incident.ID,
		"history_count": len(history),
	})

	return &pb.GetIncidentResponse{
		Incident: pbIncident,
		Events:   pbHistory,
	}, nil
}
