package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	schedulerv1 "UptimePingPlatform/gen/proto/api/scheduler/v1"
	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
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
		Name:     req.Name,
		Type:     domain.CheckType(req.Type),
		Target:   req.Target,
		Interval: int(req.Interval),
		Timeout:  int(req.Timeout),
		Status:   domain.CheckStatus(status),
		Config:   h.convertConfigMap(req.Config),
		Priority: domain.Priority(req.Priority),
		Tags:     req.Tags,
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
		Name:     req.Name,
		Type:     domain.CheckType(req.Type),
		Target:   req.Target,
		Interval: int(req.Interval),
		Timeout:  int(req.Timeout),
		Status:   domain.CheckStatus(req.Status),
		Config:   h.convertConfigMap(req.Config),
		Priority: domain.Priority(req.Priority),
		Tags:     req.Tags,
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
		"status":   string(check.Status),
	})

	return h.convertCheckToProto(check), nil
}

// ScheduleCheck планирует выполнение проверки
func (h *HandlerFixed) ScheduleCheck(ctx context.Context, req *schedulerv1.ScheduleCheckRequest) (*schedulerv1.Schedule, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "ScheduleCheck", map[string]interface{}{
		"check_id":        req.CheckId,
		"cron_expression": req.CronExpression,
	})

	// Валидация обязательных полей
	if err := h.BaseHandler.ValidateRequiredFields(ctx, "ScheduleCheck", map[string]string{
		"check_id":        req.CheckId,
		"cron_expression": req.CronExpression,
	}); err != nil {
		return nil, err
	}

	// Валидация cron выражения
	if err := h.validator.ValidateCronExpression(req.CronExpression); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid cron expression: %v", err)
	}

	// Планирование проверки
	schedule := &domain.Schedule{
		CheckID:        req.CheckId,
		CronExpression: req.CronExpression,
		IsActive:       true,
	}

	createdSchedule, err := h.checkUseCase.ScheduleCheck(ctx, schedule)
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "ScheduleCheck", req.CheckId)
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "ScheduleCheck", map[string]interface{}{
		"check_id":        req.CheckId,
		"cron_expression": req.CronExpression,
		"schedule_id":     createdSchedule.ID,
		"is_active":       createdSchedule.IsActive,
	})

	return h.convertScheduleToProto(createdSchedule), nil
}

// UnscheduleCheck отменяет планирование проверки
func (h *HandlerFixed) UnscheduleCheck(ctx context.Context, req *schedulerv1.UnscheduleCheckRequest) (*schedulerv1.UnscheduleCheckResponse, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "UnscheduleCheck", map[string]interface{}{
		"check_id": req.CheckId,
	})

	// Валидация обязательных полей
	if err := h.BaseHandler.ValidateRequiredFields(ctx, "UnscheduleCheck", map[string]string{
		"check_id": req.CheckId,
	}); err != nil {
		return nil, err
	}

	// Отмена планирования
	err := h.checkUseCase.UnscheduleCheck(ctx, req.CheckId)
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "UnscheduleCheck", req.CheckId)
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "UnscheduleCheck", map[string]interface{}{
		"check_id": req.CheckId,
	})

	return &schedulerv1.UnscheduleCheckResponse{Success: true}, nil
}

// GetSchedule возвращает информацию о расписании проверки
func (h *HandlerFixed) GetSchedule(ctx context.Context, req *schedulerv1.GetScheduleRequest) (*schedulerv1.Schedule, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "GetSchedule", map[string]interface{}{
		"check_id": req.CheckId,
	})

	// Валидация обязательных полей
	if err := h.BaseHandler.ValidateRequiredFields(ctx, "GetSchedule", map[string]string{
		"check_id": req.CheckId,
	}); err != nil {
		return nil, err
	}

	// Получение расписания
	schedule, err := h.checkUseCase.GetSchedule(ctx, req.CheckId)
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "GetSchedule", req.CheckId)
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "GetSchedule", map[string]interface{}{
		"check_id":        req.CheckId,
		"cron_expression": schedule.CronExpression,
		"is_active":       schedule.IsActive,
		"next_run":        schedule.NextRun,
	})

	return h.convertScheduleToProto(schedule), nil
}

// ListSchedules возвращает список расписаний с пагинацией
func (h *HandlerFixed) ListSchedules(ctx context.Context, req *schedulerv1.ListSchedulesRequest) (*schedulerv1.ListSchedulesResponse, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "ListSchedules", map[string]interface{}{
		"page_size":  req.PageSize,
		"page_token": req.PageToken,
		"filter":     req.Filter,
	})

	// Валидация и установка значений по умолчанию
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20 // значение по умолчанию
	}

	pageToken := req.PageToken
	if pageToken == 0 {
		pageToken = 1 // значение по умолчанию для первой страницы
	}

	// Получение списка расписаний
	schedules, total, err := h.checkUseCase.ListSchedules(ctx, usecase.ListSchedulesParams{
		Filter:    req.Filter,
		PageSize:  int(pageSize),
		PageToken: fmt.Sprintf("%d", pageToken),
	})
	if err != nil {
		return nil, h.BaseHandler.LogError(ctx, err, "ListSchedules", "")
	}

	// Конвертация в protobuf
	protoSchedules := make([]*schedulerv1.Schedule, len(schedules))
	for i, schedule := range schedules {
		protoSchedules[i] = h.convertScheduleToProto(schedule)
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "ListSchedules", map[string]interface{}{
		"count":      len(schedules),
		"page_size":  pageSize,
		"page_token": pageToken,
		"total":      total,
	})

	return &schedulerv1.ListSchedulesResponse{
		Schedules:     protoSchedules,
		NextPageToken: int32(total), // простая пагинация
	}, nil
}

// HealthCheck проверяет состояние сервиса
func (h *HandlerFixed) HealthCheck(ctx context.Context, req *schedulerv1.HealthCheckRequest) (*schedulerv1.HealthCheckResponse, error) {
	// Логируем начало операции
	h.BaseHandler.LogOperationStart(ctx, "HealthCheck", map[string]interface{}{})

	// Проверка состояния сервиса
	healthy := h.checkUseCase.HealthCheck(ctx)

	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}

	response := &schedulerv1.HealthCheckResponse{
		Healthy:       healthy,
		Status:        status,
		UptimeSeconds: int64(time.Since(time.Now()).Seconds()),
	}

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "HealthCheck", map[string]interface{}{
		"healthy":        healthy,
		"status":         status,
		"uptime_seconds": response.UptimeSeconds,
	})

	return response, nil
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
		Id:        check.ID,
		TenantId:  check.TenantID,
		Name:      check.Name,
		Type:      string(check.Type),
		Target:    check.Target,
		Interval:  int32(check.Interval),
		Timeout:   int32(check.Timeout),
		Status:    string(check.Status),
		Priority:  int32(check.Priority),
		Tags:      check.Tags,
		CreatedAt: fmt.Sprintf("%d", check.CreatedAt.Unix()),
		UpdatedAt: fmt.Sprintf("%d", check.UpdatedAt.Unix()),
	}

	if check.LastRunAt != nil {
		protoCheck.LastRunAt = fmt.Sprintf("%d", check.LastRunAt.Unix())
	}

	if check.NextRunAt != nil {
		protoCheck.NextRunAt = fmt.Sprintf("%d", check.NextRunAt.Unix())
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

// convertScheduleToProto конвертирует доменную модель Schedule в protobuf
func (h *HandlerFixed) convertScheduleToProto(schedule *domain.Schedule) *schedulerv1.Schedule {
	protoSchedule := &schedulerv1.Schedule{
		CheckId:        schedule.CheckID,
		CronExpression: schedule.CronExpression,
		IsActive:       schedule.IsActive,
	}

	if schedule.NextRun != nil {
		protoSchedule.NextRun = fmt.Sprintf("%d", schedule.NextRun.Unix())
	}

	if schedule.LastRun != nil {
		protoSchedule.LastRun = fmt.Sprintf("%d", schedule.LastRun.Unix())
	}

	return protoSchedule
}
