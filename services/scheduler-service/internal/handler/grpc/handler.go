package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	schedulerv1 "UptimePingPlatform/proto/api/scheduler/v1"
	"UptimePingPlatform/services/scheduler-service/internal/domain"
	"UptimePingPlatform/services/scheduler-service/internal/usecase"
)

// HandlerFixed реализует gRPC обработчик с устранением DRY нарушений
type HandlerFixed struct {
	*grpcBase.BaseHandler
	schedulerv1.UnimplementedSchedulerServiceServer
	checkUseCase *usecase.CheckUseCase
	validator    *validation.Validator
}

// NewHandlerFixed создает новый экземпляр HandlerFixed
func NewHandlerFixed(checkUseCase *usecase.CheckUseCase, logger logger.Logger) *HandlerFixed {
	return &HandlerFixed{
		BaseHandler:  grpcBase.NewBaseHandler(logger),
		checkUseCase: checkUseCase,
		validator:    validation.NewValidator(),
	}
}

// validateTargetFormat проверяет корректность формата target
func (h *HandlerFixed) validateTargetFormat(checkType, target string) error {
	switch checkType {
	case "http", "https":
		return h.validator.ValidateURL(target, []string{"http", "https"})
	case "grpc":
		return h.validator.ValidateHostPort(target)
	case "graphql":
		return h.validator.ValidateURL(target, []string{"http", "https"})
	case "tcp":
		return h.validator.ValidateHostPort(target)
	default:
		return fmt.Errorf("invalid check type: %s", checkType)
	}
}

// validateCheckRequest выполняет общую валидацию для запросов проверки
func (h *HandlerFixed) validateCheckRequest(checkType, target string, interval, timeout int32, status string) error {
	// Валидация формата target
	if err := h.validateTargetFormat(checkType, target); err != nil {
		return err
	}

	// Валидация интервала (минимум 5 секунд)
	if err := h.validator.ValidateInterval(interval, 5, 86400); err != nil {
		return err
	}

	// Валидация таймаута (от 1 секунды до 5 минут)
	if err := h.validator.ValidateTimeout(timeout, 1, 300); err != nil {
		return err
	}

	// Валидация типа проверки
	if err := h.validator.ValidateEnum(checkType, []string{"http", "https", "grpc", "graphql", "tcp"}, "type"); err != nil {
		return err
	}

	// Валидация статуса
	if status == "" {
		status = "active" // значение по умолчанию
	} else if err := h.validator.ValidateEnum(status, []string{"active", "paused", "disabled"}, "status"); err != nil {
		return err
	}

	return nil
}

// CreateCheck создает новую проверку
func (h *HandlerFixed) CreateCheck(ctx context.Context, req *schedulerv1.CreateCheckRequest) (*schedulerv1.Check, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "CreateCheck", map[string]interface{}{
		"tenant_id": req.TenantId,
		"name":      req.Name,
		"type":      req.Type,
		"target":    req.Target,
	})

	// Валидация обязательных полей
	if err := h.BaseHandler.ValidateRequiredFields(ctx, "CreateCheck", map[string]string{
		"tenant_id": req.TenantId,
		"name":      req.Name,
		"type":      req.Type,
		"target":    req.Target,
	}); err != nil {
		return nil, err
	}

	// Общая валидация
	if err := h.validateCheckRequest(req.Type, req.Target, req.Interval, req.Timeout, req.Status); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Установка статуса по умолчанию
	status := req.Status
	if status == "" {
		status = "active"
	}

	// Конвертация запроса в доменную модель
	check := &domain.Check{
		TenantID:    req.TenantId, // ✅ ДОБАВЛЕНО!
		Name:        req.Name,
		Description: req.Description, // ✅ ДОБАВЛЕНО!
		Type:        domain.CheckType(req.Type),
		Target:      req.Target,
		Interval:    int(req.Interval),
		Timeout:     int(req.Timeout),
		Enabled:     true, // По умолчанию включена
		Config:      h.convertConfigMap(req.Config),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Обрабатываем специальное поле enabled из metadata
	if enabledStr, ok := req.Config["enabled"]; ok {
		if enabledStr == "false" {
			check.Enabled = false
		}
	}

	// Создание проверки
	createdCheck, err := h.checkUseCase.CreateCheck(ctx, req.TenantId, check)
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "CreateCheck", req.TenantId)
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "CreateCheck", map[string]interface{}{
		"check_id":  createdCheck.ID,
		"tenant_id": req.TenantId,
		"name":      req.Name,
	})

	return h.convertCheckToProto(createdCheck), nil
}

// UpdateCheck обновляет существующую проверку
func (h *HandlerFixed) UpdateCheck(ctx context.Context, req *schedulerv1.UpdateCheckRequest) (*schedulerv1.Check, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "UpdateCheck", map[string]interface{}{
		"check_id": req.CheckId,
		"name":     req.Name,
	})

	// Валидация обязательных полей
	if err := h.BaseHandler.ValidateRequiredFields(ctx, "UpdateCheck", map[string]string{
		"check_id": req.CheckId,
		"name":     req.Name,
		"type":     req.Type,
		"target":   req.Target,
	}); err != nil {
		return nil, err
	}

	// Общая валидация
	if err := h.validateCheckRequest(req.Type, req.Target, req.Interval, req.Timeout, req.Status); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	// Конвертация запроса в доменную модель
	check := &domain.Check{
		Name:        req.Name,
		Description: req.Description, // ✅ ДОБАВЛЕНО!
		Type:        domain.CheckType(req.Type),
		Target:      req.Target,
		Interval:    int(req.Interval),
		Timeout:     int(req.Timeout),
		Enabled:     true, // По умолчанию включена
		Config:      h.convertConfigMap(req.Config),
	}

	// Обновление проверки
	err := h.checkUseCase.UpdateCheck(ctx, req.CheckId, check)
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "UpdateCheck", req.CheckId)
	}

	// Получение обновленной проверки
	updatedCheck, err := h.checkUseCase.GetCheck(ctx, req.CheckId)
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "GetCheck", req.CheckId)
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "UpdateCheck", map[string]interface{}{
		"check_id": req.CheckId,
		"name":     updatedCheck.Name,
	})

	return h.convertCheckToProto(updatedCheck), nil
}

// DeleteCheck удаляет проверку
func (h *HandlerFixed) DeleteCheck(ctx context.Context, req *schedulerv1.DeleteCheckRequest) (*schedulerv1.DeleteCheckResponse, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "DeleteCheck", map[string]interface{}{
		"check_id": req.CheckId,
	})

	// Валидация обязательных полей
	if err := h.BaseHandler.ValidateRequiredFields(ctx, "DeleteCheck", map[string]string{
		"check_id": req.CheckId,
	}); err != nil {
		return nil, err
	}

	// Удаление проверки
	err := h.checkUseCase.DeleteCheck(ctx, req.CheckId)
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "DeleteCheck", req.CheckId)
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "DeleteCheck", map[string]interface{}{
		"check_id": req.CheckId,
	})

	return &schedulerv1.DeleteCheckResponse{Success: true}, nil
}

// GetCheck возвращает информацию о проверке по ID
func (h *HandlerFixed) GetCheck(ctx context.Context, req *schedulerv1.GetCheckRequest) (*schedulerv1.Check, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "GetCheck", map[string]interface{}{
		"check_id": req.CheckId,
	})

	// Валидация обязательных полей
	if err := h.BaseHandler.ValidateRequiredFields(ctx, "GetCheck", map[string]string{
		"check_id": req.CheckId,
	}); err != nil {
		return nil, err
	}

	// Получение проверки
	check, err := h.checkUseCase.GetCheck(ctx, req.CheckId)
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "GetCheck", req.CheckId)
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "GetCheck", map[string]interface{}{
		"check_id": req.CheckId,
		"name":     check.Name,
		"enabled":  check.Enabled,
	})

	return h.convertCheckToProto(check), nil
}

// ListChecks возвращает список проверок
func (h *HandlerFixed) ListChecks(ctx context.Context, req *schedulerv1.ListChecksRequest) (*schedulerv1.ListChecksResponse, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "ListChecks", map[string]interface{}{
		"tenant_id":  req.TenantId,
		"page_size":  req.PageSize,
		"page_token": req.PageToken,
	})

	// Установка значений по умолчанию
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Получение списка проверок
	checks, err := h.checkUseCase.ListChecks(ctx, req.TenantId, int(pageSize), fmt.Sprintf("%d", req.PageToken))
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "ListChecks", req.TenantId)
	}

	// Конвертация в proto формат
	protoChecks := make([]*schedulerv1.Check, len(checks))
	for i, check := range checks {
		protoChecks[i] = h.convertCheckToProto(check)
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "ListChecks", map[string]interface{}{
		"tenant_id":  req.TenantId,
		"count":      len(checks),
		"page_size":  pageSize,
		"page_token": req.PageToken,
	})

	return &schedulerv1.ListChecksResponse{
		Checks:        protoChecks,
		NextPageToken: 0, // Упрощенная пагинация
	}, nil
}

// ScheduleCheck планирует выполнение проверки
func (h *HandlerFixed) ScheduleCheck(ctx context.Context, req *schedulerv1.ScheduleCheckRequest) (*schedulerv1.Schedule, error) {
	return nil, status.Errorf(codes.Unimplemented, "ScheduleCheck not implemented yet")
}

// UnscheduleCheck отменяет планирование проверки
func (h *HandlerFixed) UnscheduleCheck(ctx context.Context, req *schedulerv1.UnscheduleCheckRequest) (*schedulerv1.UnscheduleCheckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "UnscheduleCheck not implemented yet")
}

// GetSchedule возвращает информацию о расписании проверки
func (h *HandlerFixed) GetSchedule(ctx context.Context, req *schedulerv1.GetScheduleRequest) (*schedulerv1.Schedule, error) {
	return nil, status.Errorf(codes.Unimplemented, "GetSchedule not implemented yet")
}

// ListSchedules возвращает список расписаний с пагинацией
func (h *HandlerFixed) ListSchedules(ctx context.Context, req *schedulerv1.ListSchedulesRequest) (*schedulerv1.ListSchedulesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "ListSchedules not implemented yet")
}

// HealthCheck проверяет состояние сервиса
func (h *HandlerFixed) HealthCheck(ctx context.Context, req *schedulerv1.HealthCheckRequest) (*schedulerv1.HealthCheckResponse, error) {
	return &schedulerv1.HealthCheckResponse{
		Healthy: true,
		Status:  "healthy",
	}, nil
}

// Вспомогательные методы конвертации

// convertConfigMap конвертирует map[string]string в map[string]interface{}
func (h *HandlerFixed) convertConfigMap(config map[string]string) map[string]interface{} {
	if config == nil {
		return make(map[string]interface{})
	}

	result := make(map[string]interface{})
	for k, v := range config {
		result[k] = v
	}
	return result
}

// convertCheckToProto конвертирует доменную модель Check в protobuf
func (h *HandlerFixed) convertCheckToProto(check *domain.Check) *schedulerv1.Check {
	protoCheck := &schedulerv1.Check{
		Id:          check.ID,
		TenantId:    check.TenantID,
		Name:        check.Name,
		Description: check.Description,
		Type:        string(check.Type),
		Target:      check.Target,
		Interval:    int32(check.Interval),
		Timeout:     int32(check.Timeout),
		Status: func() string {
			if check.Enabled {
				return "active"
			} else {
				return "disabled"
			}
		}(),
		Priority:  1,
		Tags:      []string{}, // Пустые теги, т.к. поле отсутствует в доменной модели
		CreatedAt: fmt.Sprintf("%d", check.CreatedAt.Unix()),
		UpdatedAt: fmt.Sprintf("%d", check.UpdatedAt.Unix()),
	}

	if check.LastRunAt != nil {
		protoCheck.LastRunAt = fmt.Sprintf("%d", check.LastRunAt.Unix())
	}

	if check.Config != nil {
		protoConfig := make(map[string]string)
		for k, v := range check.Config {
			protoConfig[k] = fmt.Sprintf("%v", v)
		}
		protoCheck.Config = protoConfig
	}

	return protoCheck
}
