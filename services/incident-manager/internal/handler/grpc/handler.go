package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/incident-manager/internal/domain"
	"UptimePingPlatform/services/incident-manager/internal/service"

	incidentv1 "UptimePingPlatform/proto/api/incident/v1"
)

// Helper функции
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// IncidentHandler реализует gRPC handler для управления инцидентами
type IncidentHandler struct {
	grpcBase.BaseHandler
	service   service.IncidentService
	logger    logger.Logger
	validator *validation.Validator
}

// NewIncidentHandler создает новый handler
func NewIncidentHandler(service service.IncidentService, logger logger.Logger) *IncidentHandler {
	baseHandler := grpcBase.NewBaseHandler(logger)
	return &IncidentHandler{
		BaseHandler: *baseHandler,
		service:     service,
		logger:      logger,
		validator:   validation.NewValidator(),
	}
}

// CreateIncident создает новый инцидент
func (h *IncidentHandler) CreateIncident(ctx context.Context, req *incidentv1.CreateIncidentRequest) (*incidentv1.Incident, error) {
	h.LogOperationStart(ctx, "CreateIncident", map[string]interface{}{
		"check_id": req.CheckId,
		"title":    req.Title,
	})

	// Валидация запроса
	if err := h.validateCreateIncidentRequest(ctx, req); err != nil {
		h.LogError(ctx, err, "CreateIncident", req.CheckId)
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Создаем результат проверки
	result := &service.CheckResult{
		CheckID:      req.CheckId,
		TenantID:     "",              // Убираем tenant_id
		IsSuccess:    false,           // Если создаем инцидент, значит проверка неуспешная
		ErrorMessage: req.Description, // Используем description как error message
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
		"title":       req.Title,
	})

	return pbIncident, nil
}

// UpdateIncident обновляет существующий инцидент
func (h *IncidentHandler) UpdateIncident(ctx context.Context, req *incidentv1.UpdateIncidentRequest) (*incidentv1.Incident, error) {
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
	if req.Status != incidentv1.IncidentStatus_INCIDENT_STATUS_UNSPECIFIED {
		incident.Status = h.protoStatusToDomain(req.Status)
	}
	if req.Severity != incidentv1.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED {
		incident.Severity = h.protoSeverityToDomain(req.Severity)
	}

	// Сохраняем обновленный инцидент
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
func (h *IncidentHandler) ResolveIncident(ctx context.Context, req *incidentv1.ResolveIncidentRequest) (*incidentv1.ResolveIncidentResponse, error) {
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

	return &incidentv1.ResolveIncidentResponse{
		Success: true,
	}, nil
}

// ListIncidents получает список инцидентов с фильтрацией и пагинацией
func (h *IncidentHandler) ListIncidents(ctx context.Context, req *incidentv1.ListIncidentsRequest) (*incidentv1.ListIncidentsResponse, error) {
	h.LogOperationStart(ctx, "ListIncidents", map[string]interface{}{
		"check_id":  req.CheckId,
		"page_size": req.PageSize,
	})

	// Валидация запроса
	if err := h.validateListIncidentsRequest(ctx, req); err != nil {
		h.LogError(ctx, err, "ListIncidents", req.CheckId)
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Создаем фильтр
	filter := &domain.IncidentFilter{
		CheckID: &req.CheckId,
	}

	// Добавляем фильтры
	if req.Status != incidentv1.IncidentStatus_INCIDENT_STATUS_UNSPECIFIED {
		status := h.protoStatusToDomain(req.Status)
		filter.Status = &status
	}
	if req.Severity != incidentv1.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED {
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
		h.LogError(ctx, err, "ListIncidents", req.CheckId)
		return nil, status.Errorf(codes.Internal, "failed to get incidents: %v", err)
	}

	// Конвертируем в protobuf
	pbIncidents := make([]*incidentv1.Incident, len(incidents))
	for i, incident := range incidents {
		pbIncidents[i] = h.incidentToProto(incident)
	}

	// Вычисляем NextPageToken для пагинации
	nextPageToken := int32(0)
	if req.PageSize > 0 && len(incidents) == int(req.PageSize) {
		// Если вернули полное количество записей, возможно есть следующая страница
		nextPageToken = int32(req.PageToken + req.PageSize)
	}

	h.LogOperationSuccess(ctx, "ListIncidents", map[string]interface{}{
		"check_id":        req.CheckId,
		"count":           len(incidents),
		"page_size":       req.PageSize,
		"page_token":      req.PageToken,
		"next_page_token": nextPageToken,
	})

	return &incidentv1.ListIncidentsResponse{
		Incidents:     pbIncidents,
		NextPageToken: nextPageToken,
	}, nil
}

// GetIncident получает инцидент с историей
func (h *IncidentHandler) GetIncident(ctx context.Context, req *incidentv1.GetIncidentRequest) (*incidentv1.GetIncidentResponse, error) {
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
	pbHistory := make([]*incidentv1.IncidentEvent, len(history))
	for i, event := range history {
		pbHistory[i] = h.incidentEventToProto(ctx, event)
	}

	h.LogOperationSuccess(ctx, "GetIncident", map[string]interface{}{
		"incident_id":   incident.ID,
		"history_count": len(history),
	})

	return &incidentv1.GetIncidentResponse{
		Incident: pbIncident,
		Events:   pbHistory,
	}, nil
}

// Валидационные методы

// validateCreateIncidentRequest валидирует запрос на создание инцидента
func (h *IncidentHandler) validateCreateIncidentRequest(ctx context.Context, req *incidentv1.CreateIncidentRequest) error {
	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "CreateIncident", map[string]string{
		"check_id": req.CheckId,
		"title":    req.Title,
	}); err != nil {
		return err
	}

	// Валидация check_id
	if err := h.validator.ValidateStringLength(req.CheckId, "check_id", 1, 100); err != nil {
		return err
	}

	// Валидация title
	if err := h.validator.ValidateStringLength(req.Title, "title", 1, 255); err != nil {
		return err
	}

	// Валидация description (опционально)
	if req.Description != "" {
		if err := h.validator.ValidateStringLength(req.Description, "description", 0, 1000); err != nil {
			return err
		}
	}

	return nil
}

// validateUpdateIncidentRequest валидирует запрос на обновление инцидента
func (h *IncidentHandler) validateUpdateIncidentRequest(ctx context.Context, req *incidentv1.UpdateIncidentRequest) error {
	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "UpdateIncident", map[string]string{
		"incident_id": req.IncidentId,
	}); err != nil {
		return err
	}

	// Валидация incident_id
	if err := h.validator.ValidateStringLength(req.IncidentId, "incident_id", 1, 100); err != nil {
		return err
	}

	// Валидация status
	if req.Status < incidentv1.IncidentStatus_INCIDENT_STATUS_OPEN ||
		req.Status > incidentv1.IncidentStatus_INCIDENT_STATUS_RESOLVED {
		return status.Errorf(codes.InvalidArgument, "invalid status value")
	}

	// Валидация severity
	if req.Severity < incidentv1.IncidentSeverity_INCIDENT_SEVERITY_LOW ||
		req.Severity > incidentv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL {
		return status.Errorf(codes.InvalidArgument, "invalid severity value")
	}

	return nil
}

// validateResolveIncidentRequest валидирует запрос на закрытие инцидента
func (h *IncidentHandler) validateResolveIncidentRequest(ctx context.Context, req *incidentv1.ResolveIncidentRequest) error {
	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "ResolveIncident", map[string]string{
		"incident_id": req.IncidentId,
	}); err != nil {
		return err
	}

	// Валидация incident_id
	if err := h.validator.ValidateStringLength(req.IncidentId, "incident_id", 1, 100); err != nil {
		return err
	}

	return nil
}

// validateListIncidentsRequest валидирует запрос на получение списка инцидентов
func (h *IncidentHandler) validateListIncidentsRequest(ctx context.Context, req *incidentv1.ListIncidentsRequest) error {
	// Валидация check_id если указан
	if req.CheckId != "" {
		if err := h.validator.ValidateStringLength(req.CheckId, "check_id", 1, 100); err != nil {
			return err
		}
	}

	// Валидация page_size
	if req.PageSize < 0 || req.PageSize > 1000 {
		return status.Errorf(codes.InvalidArgument, "page_size must be between 0 and 1000")
	}

	// Валидация page_token
	if req.PageToken < 0 {
		return status.Errorf(codes.InvalidArgument, "page_token must be non-negative")
	}

	// Валидация status
	if req.Status < incidentv1.IncidentStatus_INCIDENT_STATUS_UNSPECIFIED ||
		req.Status > incidentv1.IncidentStatus_INCIDENT_STATUS_RESOLVED {
		return status.Errorf(codes.InvalidArgument, "invalid status value")
	}

	// Валидация severity
	if req.Severity < incidentv1.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED ||
		req.Severity > incidentv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL {
		return status.Errorf(codes.InvalidArgument, "invalid severity value")
	}

	return nil
}

// validateGetIncidentRequest валидирует запрос на получение инцидента
func (h *IncidentHandler) validateGetIncidentRequest(ctx context.Context, req *incidentv1.GetIncidentRequest) error {
	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "GetIncident", map[string]string{
		"incident_id": req.IncidentId,
	}); err != nil {
		return err
	}

	// Валидация incident_id
	if err := h.validator.ValidateStringLength(req.IncidentId, "incident_id", 1, 100); err != nil {
		return err
	}

	return nil
}

// Конвертеры из converter.go

// incidentToProto конвертирует доменный инцидент в protobuf
func (h *IncidentHandler) incidentToProto(incident *domain.Incident) *incidentv1.Incident {
	return &incidentv1.Incident{
		Id:          incident.ID,
		CheckId:     incident.CheckID,
		Title:       incident.Title,
		Description: incident.Description,
		Status:      h.domainStatusToProto(incident.Status),
		Severity:    h.domainSeverityToProto(incident.Severity),
		StartedAt:   incident.StartedAt.Format(time.RFC3339),
		ResolvedAt:  formatTimePtr(incident.ResolvedAt),
		CreatedAt:   incident.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   incident.UpdatedAt.Format(time.RFC3339),
	}
}

// incidentEventToProto конвертирует событие инцидента в protobuf
func (h *IncidentHandler) incidentEventToProto(ctx context.Context, event *domain.IncidentEvent) *incidentv1.IncidentEvent {
	// Извлекаем user ID из контекста
	userID := h.extractUserIDFromContext(ctx)

	return &incidentv1.IncidentEvent{
		Id:          event.ID,
		IncidentId:  event.IncidentID,
		Type:        event.EventType,
		Description: event.Message,
		CreatedAt:   event.CreatedAt.Format(time.RFC3339),
		UserId:      userID,
	}
}

// domainStatusToProto конвертирует статус домена в protobuf
func (h *IncidentHandler) domainStatusToProto(status domain.IncidentStatus) incidentv1.IncidentStatus {
	switch status {
	case domain.IncidentStatusOpen:
		return incidentv1.IncidentStatus_INCIDENT_STATUS_OPEN
	case domain.IncidentStatusAcknowledged:
		return incidentv1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED
	case domain.IncidentStatusResolved:
		return incidentv1.IncidentStatus_INCIDENT_STATUS_RESOLVED
	default:
		return incidentv1.IncidentStatus_INCIDENT_STATUS_UNSPECIFIED
	}
}

// protoStatusToDomain конвертирует статус protobuf в домен
func (h *IncidentHandler) protoStatusToDomain(status incidentv1.IncidentStatus) domain.IncidentStatus {
	switch status {
	case incidentv1.IncidentStatus_INCIDENT_STATUS_OPEN:
		return domain.IncidentStatusOpen
	case incidentv1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED:
		return domain.IncidentStatusAcknowledged
	case incidentv1.IncidentStatus_INCIDENT_STATUS_RESOLVED:
		return domain.IncidentStatusResolved
	default:
		return domain.IncidentStatusOpen
	}
}

// domainSeverityToProto конвертирует серьезность домена в protobuf
func (h *IncidentHandler) domainSeverityToProto(severity domain.IncidentSeverity) incidentv1.IncidentSeverity {
	switch severity {
	case domain.IncidentSeverityLow:
		return incidentv1.IncidentSeverity_INCIDENT_SEVERITY_LOW
	case domain.IncidentSeverityMedium:
		return incidentv1.IncidentSeverity_INCIDENT_SEVERITY_MEDIUM
	case domain.IncidentSeverityHigh:
		return incidentv1.IncidentSeverity_INCIDENT_SEVERITY_HIGH
	case domain.IncidentSeverityCritical:
		return incidentv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL
	default:
		return incidentv1.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED
	}
}

// protoSeverityToDomain конвертирует серьезность protobuf в домен
func (h *IncidentHandler) protoSeverityToDomain(severity incidentv1.IncidentSeverity) domain.IncidentSeverity {
	switch severity {
	case incidentv1.IncidentSeverity_INCIDENT_SEVERITY_LOW:
		return domain.IncidentSeverityLow
	case incidentv1.IncidentSeverity_INCIDENT_SEVERITY_MEDIUM:
		return domain.IncidentSeverityMedium
	case incidentv1.IncidentSeverity_INCIDENT_SEVERITY_HIGH:
		return domain.IncidentSeverityHigh
	case incidentv1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL:
		return domain.IncidentSeverityCritical
	default:
		return domain.IncidentSeverityLow
	}
}

// extractUserIDFromContext извлекает user ID из контекста
func (h *IncidentHandler) extractUserIDFromContext(ctx context.Context) string {
	// Извлекаем из gRPC метаданных
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if userIDs := md["user_id"]; len(userIDs) > 0 {
			return userIDs[0]
		}

		// Альтернативные поля метаданных
		if userIDs := md["x-user-id"]; len(userIDs) > 0 {
			return userIDs[0]
		}
		if userIDs := md["x-user"]; len(userIDs) > 0 {
			return userIDs[0]
		}
	}

	// Извлекаем из контекстных значений
	if userID := ctx.Value("user_id"); userID != nil {
		if uid, ok := userID.(string); ok {
			return uid
		}
	}

	// Возвращаем "system" если пользователь не определен
	return "system"
}
