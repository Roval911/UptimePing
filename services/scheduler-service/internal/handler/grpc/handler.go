package grpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
	checkUseCase   *usecase.CheckUseCase
	validator      *validation.Validator
	rabbitProducer RabbitMQProducer // Добавлен RabbitMQ producer
}

// RabbitMQProducer интерфейс для публикации задач
type RabbitMQProducer interface {
	PublishTask(ctx context.Context, check *domain.Check, tenantID string) error
}

// NewHandlerFixed создает новый экземпляр HandlerFixed
func NewHandlerFixed(checkUseCase *usecase.CheckUseCase, logger logger.Logger, rabbitProducer RabbitMQProducer) *HandlerFixed {
	return &HandlerFixed{
		BaseHandler:    grpcBase.NewBaseHandler(logger),
		checkUseCase:   checkUseCase,
		validator:      validation.NewValidator(),
		rabbitProducer: rabbitProducer,
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

	// Генерация UUID для новой проверки
	checkID := uuid.New().String()

	// Конвертация запроса в доменную модель
	check := &domain.Check{
		ID:          checkID,      // ✅ ДОБАВЛЕНО!
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

	// Публикуем задачу в RabbitMQ если проверка активна
	if createdCheck.Enabled && h.rabbitProducer != nil {
		if err := h.rabbitProducer.PublishTask(ctx, createdCheck, req.TenantId); err != nil {
			// Логируем ошибку, но не прерываем операцию
			h.BaseHandler.LogError(ctx, err, "CreateCheck", req.TenantId)
		} else {
			h.BaseHandler.LogOperationSuccess(ctx, "PublishTask", map[string]interface{}{
				"check_id":  createdCheck.ID,
				"tenant_id": req.TenantId,
				"target":    createdCheck.Target,
			})
		}
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
		"type":     req.Type,
		"target":   req.Target,
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
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Логируем финальный check перед вызовом UpdateCheck
	h.BaseHandler.LogOperationSuccess(ctx, "FinalCheckBeforeUpdate", map[string]interface{}{
		"check_id":     check.ID,
		"final_name":   check.Name,
		"final_type":   string(check.Type),
		"final_target": check.Target,
	})

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

	// Конвертация в protobuf
	protoCheck := h.convertCheckToProto(updatedCheck)

	// Логируем успешное завершение
	h.BaseHandler.LogOperationSuccess(ctx, "UpdateCheck", map[string]interface{}{
		"check_id": req.CheckId,
	})

	return protoCheck, nil
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
	h.LogOperationStart(ctx, "ScheduleCheck", map[string]interface{}{
		"check_id":        req.CheckId,
		"cron_expression": req.CronExpression,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "ScheduleCheck", map[string]string{
		"check_id":        req.CheckId,
		"cron_expression": req.CronExpression,
	}); err != nil {
		return nil, h.LogError(ctx, err, "ScheduleCheck", "validation failed")
	}

	// Валидация UUID
	if err := h.validator.ValidateUUID(req.CheckId, "check_id"); err != nil {
		return nil, h.LogError(ctx, err, "ScheduleCheck", "invalid check ID format")
	}

	// Валидация cron выражения
	if err := h.validator.ValidateCronExpression(req.CronExpression); err != nil {
		return nil, h.LogError(ctx, err, "ScheduleCheck", "invalid cron expression")
	}

	// Создаем расписание через use case
	schedule, err := h.checkUseCase.CreateSchedule(ctx, req.CheckId, req.CronExpression)
	if err != nil {
		return nil, h.LogError(ctx, err, "ScheduleCheck", "failed to create schedule")
	}

	response := &schedulerv1.Schedule{
		CheckId:        schedule.CheckID,
		CronExpression: req.CronExpression,
		NextRun:        schedule.NextRunAt.Format(time.RFC3339),
	}

	h.LogOperationSuccess(ctx, "ScheduleCheck", map[string]interface{}{
		"schedule_id": schedule.ID,
		"check_id":    req.CheckId,
	})

	return response, nil
}

// UnscheduleCheck отменяет планирование проверки
func (h *HandlerFixed) UnscheduleCheck(ctx context.Context, req *schedulerv1.UnscheduleCheckRequest) (*schedulerv1.UnscheduleCheckResponse, error) {
	h.LogOperationStart(ctx, "UnscheduleCheck", map[string]interface{}{
		"check_id": req.CheckId,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "UnscheduleCheck", map[string]string{
		"check_id": req.CheckId,
	}); err != nil {
		return nil, h.LogError(ctx, err, "UnscheduleCheck", "validation failed")
	}

	// Валидация UUID
	if err := h.validator.ValidateUUID(req.CheckId, "check_id"); err != nil {
		return nil, h.LogError(ctx, err, "UnscheduleCheck", "invalid check ID format")
	}

	// Удаляем расписание через use case
	err := h.checkUseCase.DeleteSchedule(ctx, req.CheckId)
	if err != nil {
		return nil, h.LogError(ctx, err, "UnscheduleCheck", "failed to delete schedule")
	}

	response := &schedulerv1.UnscheduleCheckResponse{
		Success: true,
	}

	h.LogOperationSuccess(ctx, "UnscheduleCheck", map[string]interface{}{
		"check_id": req.CheckId,
	})

	return response, nil
}

// GetSchedule возвращает информацию о расписании проверки
func (h *HandlerFixed) GetSchedule(ctx context.Context, req *schedulerv1.GetScheduleRequest) (*schedulerv1.Schedule, error) {
	h.LogOperationStart(ctx, "GetSchedule", map[string]interface{}{
		"check_id": req.CheckId,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "GetSchedule", map[string]string{
		"check_id": req.CheckId,
	}); err != nil {
		return nil, h.LogError(ctx, err, "GetSchedule", "validation failed")
	}

	// Валидация UUID
	if err := h.validator.ValidateUUID(req.CheckId, "check_id"); err != nil {
		return nil, h.LogError(ctx, err, "GetSchedule", "invalid check ID format")
	}

	// Получаем расписание через use case
	schedule, err := h.checkUseCase.GetScheduleByCheckID(ctx, req.CheckId)
	if err != nil {
		return nil, h.LogError(ctx, err, "GetSchedule", "failed to get schedule")
	}

	response := &schedulerv1.Schedule{
		CheckId:        schedule.CheckID,
		CronExpression: schedule.CronExpression,
		NextRun:        schedule.NextRunAt.Format(time.RFC3339),
	}

	h.LogOperationSuccess(ctx, "GetSchedule", map[string]interface{}{
		"schedule_id": schedule.ID,
		"check_id":    req.CheckId,
	})

	return response, nil
}

// UpdateSchedule обновляет расписание проверки
func (h *HandlerFixed) UpdateSchedule(ctx context.Context, req *schedulerv1.UpdateScheduleRequest) (*schedulerv1.Schedule, error) {
	h.LogOperationStart(ctx, "UpdateSchedule", map[string]interface{}{
		"check_id":        req.CheckId,
		"cron_expression": req.CronExpression,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "UpdateSchedule", map[string]string{
		"check_id":        req.CheckId,
		"cron_expression": req.CronExpression,
	}); err != nil {
		return nil, h.LogError(ctx, err, "UpdateSchedule", "validation failed")
	}

	// Валидация UUID
	if err := h.validator.ValidateUUID(req.CheckId, "check_id"); err != nil {
		return nil, h.LogError(ctx, err, "UpdateSchedule", "invalid check ID format")
	}

	// Валидация cron выражения
	if err := h.validator.ValidateCronExpression(req.CronExpression); err != nil {
		return nil, h.LogError(ctx, err, "UpdateSchedule", "invalid cron expression")
	}

	// Обновляем расписание через use case
	schedule, err := h.checkUseCase.UpdateSchedule(ctx, req.CheckId, req.CronExpression)
	if err != nil {
		return nil, h.LogError(ctx, err, "UpdateSchedule", "failed to update schedule")
	}

	response := &schedulerv1.Schedule{
		CheckId:        schedule.CheckID,
		CronExpression: schedule.CronExpression,
		NextRun:        schedule.NextRunAt.Format(time.RFC3339),
	}

	h.LogOperationSuccess(ctx, "UpdateSchedule", map[string]interface{}{
		"schedule_id": schedule.ID,
		"check_id":    req.CheckId,
	})

	return response, nil
}

// ListSchedules возвращает список расписаний с пагинацией
func (h *HandlerFixed) ListSchedules(ctx context.Context, req *schedulerv1.ListSchedulesRequest) (*schedulerv1.ListSchedulesResponse, error) {
	h.LogOperationStart(ctx, "ListSchedules", map[string]interface{}{
		"filter":     req.Filter,
		"page_size":  req.PageSize,
		"page_token": req.PageToken,
	})

	// Извлекаем tenant_id из фильтра
	tenantID := ""
	if req.Filter != "" {
		// Парсим фильтр формата "tenant_id:xxx"
		parts := strings.Split(req.Filter, ":")
		if len(parts) == 2 && parts[0] == "tenant_id" {
			tenantID = parts[1]
		}
	}

	if tenantID == "" {
		return nil, h.LogError(ctx, fmt.Errorf("tenant_id is required"), "ListSchedules", "validation failed")
	}

	// Получаем расписания через use case
	schedules, err := h.checkUseCase.ListSchedules(ctx, tenantID, int(req.PageSize), fmt.Sprintf("%d", req.PageToken))
	if err != nil {
		return nil, h.LogError(ctx, err, "ListSchedules", "failed to list schedules")
	}

	// Конвертируем в protobuf
	var protoSchedules []*schedulerv1.Schedule
	for _, schedule := range schedules {
		protoSchedule := &schedulerv1.Schedule{
			CheckId:        schedule.CheckID,
			CronExpression: "", // TODO: добавить в доменную модель
			NextRun:        schedule.NextRunAt.Format(time.RFC3339),
		}
		protoSchedules = append(protoSchedules, protoSchedule)
	}

	response := &schedulerv1.ListSchedulesResponse{
		Schedules:     protoSchedules,
		NextPageToken: 0, // TODO: реализовать пагинацию
	}

	h.LogOperationSuccess(ctx, "ListSchedules", map[string]interface{}{
		"schedules_count": len(protoSchedules),
		"tenant_id":       tenantID,
	})

	return response, nil
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
