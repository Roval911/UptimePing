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
	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/services/core-service/internal/service"

	corev1 "UptimePingPlatform/proto/api/core/v1"
	schedulerv1 "UptimePingPlatform/proto/api/scheduler/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CoreHandler реализует gRPC обработчики для CoreService
type CoreHandler struct {
	*grpcBase.BaseHandler
	corev1.UnimplementedCoreServiceServer
	checkService *service.CheckService
	validator    *validation.Validator
}

// NewCoreHandler создает новый экземпляр CoreHandler
func NewCoreHandler(checkService *service.CheckService, logger logger.Logger) *CoreHandler {
	return &CoreHandler{
		BaseHandler:  grpcBase.NewBaseHandler(logger),
		checkService: checkService,
		validator:    validation.NewValidator(),
	}
}

// ExecuteCheck выполняет проверку немедленно
func (h *CoreHandler) ExecuteCheck(ctx context.Context, req *corev1.ExecuteCheckRequest) (*corev1.CheckResult, error) {
	h.LogOperationStart(ctx, "ExecuteCheck", map[string]interface{}{
		"check_id": req.CheckId,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "ExecuteCheck", map[string]string{
		"check_id": req.CheckId,
	}); err != nil {
		return nil, err
	}

	// Валидация check_id
	if err := h.validator.ValidateStringLength(req.CheckId, "check_id", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "ExecuteCheck", req.CheckId)
	}

	// Получаем информацию о проверке из Scheduler Service (чтобы узнать target и type)
	var target, typ string
	{
		// Подключаемся к Scheduler Service
		conn, err := grpc.DialContext(ctx, "scheduler-service:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			defer conn.Close()
			sClient := schedulerv1.NewSchedulerServiceClient(conn)
			checkResp, err := sClient.GetCheck(ctx, &schedulerv1.GetCheckRequest{CheckId: req.CheckId})
			if err == nil && checkResp != nil {
				target = checkResp.Target
				typ = checkResp.Type
			} else {
				// логируем, но продолжаем — некоторые чекеры могут работать только по check id
				h.LogOperationStart(ctx, "ExecuteCheck_GetCheckFailed", map[string]interface{}{"check_id": req.CheckId, "error": err})
			}
		} else {
			h.LogOperationStart(ctx, "ExecuteCheck_DialSchedulerFailed", map[string]interface{}{"check_id": req.CheckId, "error": err})
		}
	}

	// Создаем задачу для выполнения
	task := &domain.Task{
		CheckID:     req.CheckId,
		Type:        typ,
		Target:      target,
		ExecutionID: generateExecutionID(),
		CreatedAt:   time.Now().UTC(),
		Config:      make(map[string]interface{}),
	}

	// Устанавливаем разумные значения по умолчанию для HTTP чекера
	if task.Type == string(domain.TaskTypeHTTP) {
		if _, ok := task.Config["method"]; !ok {
			task.Config["method"] = "GET"
		}
		// Если в конфиге нет URL, используем target из Check
		if _, ok := task.Config["url"]; !ok {
			task.Config["url"] = task.Target
		}
		// По умолчанию ожидаемый статус 200
		if _, ok := task.Config["expected_status"]; !ok {
			task.Config["expected_status"] = float64(200)
		}
	}

	// Выполняем проверку
	result, err := h.checkService.ExecuteCheck(ctx, task)
	if err != nil {
		h.LogError(ctx, err, "ExecuteCheck", req.CheckId)
		return nil, status.Errorf(codes.Internal, "failed to execute check: %v", err)
	}

	// Конвертируем результат в protobuf
	protoResult := h.convertCheckResultToProto(result)

	h.LogOperationSuccess(ctx, "ExecuteCheck", map[string]interface{}{
		"check_id":    req.CheckId,
		"success":     result.Status == "up",
		"duration_ms": result.ResponseTimeMs,
	})

	return protoResult, nil
}

// GetCheckStatus возвращает текущий статус проверки
func (h *CoreHandler) GetCheckStatus(ctx context.Context, req *corev1.GetCheckStatusRequest) (*corev1.CheckStatusResponse, error) {
	h.LogOperationStart(ctx, "GetCheckStatus", map[string]interface{}{
		"check_id": req.CheckId,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "GetCheckStatus", map[string]string{
		"check_id": req.CheckId,
	}); err != nil {
		return nil, err
	}

	// Валидация check_id
	if err := h.validator.ValidateStringLength(req.CheckId, "check_id", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "GetCheckStatus", req.CheckId)
	}

	// Получаем статус проверки
	checkStatus, err := h.checkService.GetCheckStatus(ctx, req.CheckId)
	if err != nil {
		// Если не удалось получить статус (например, нет результатов в БД), возвращаем unknown вместо ошибки
		h.LogOperationStart(ctx, "GetCheckStatus_FallbackUnknown", map[string]interface{}{"check_id": req.CheckId, "error": err.Error()})
		return &corev1.CheckStatusResponse{
			CheckId:        req.CheckId,
			Status:         "unknown",
			ResponseTimeMs: 0,
			LastCheckedAt:  "",
		}, nil
	}

	// Конвертируем в protobuf
	status := "down"
	if checkStatus.IsHealthy {
		status = "up"
	}

	protoStatus := &corev1.CheckStatusResponse{
		CheckId:        req.CheckId,
		Status:         status,
		ResponseTimeMs: float64(checkStatus.ResponseTimeMs),
		LastCheckedAt:  checkStatus.LastCheckedAt,
	}

	h.LogOperationSuccess(ctx, "GetCheckStatus", map[string]interface{}{
		"check_id":         req.CheckId,
		"is_healthy":       checkStatus.IsHealthy,
		"response_time_ms": checkStatus.ResponseTimeMs,
	})

	return protoStatus, nil
}

// GetCheckHistory возвращает историю выполнения проверки
func (h *CoreHandler) GetCheckHistory(ctx context.Context, req *corev1.GetCheckHistoryRequest) (*corev1.GetCheckHistoryResponse, error) {
	h.LogOperationStart(ctx, "GetCheckHistory", map[string]interface{}{
		"check_id": req.CheckId,
		"limit":    req.Limit,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "GetCheckHistory", map[string]string{
		"check_id": req.CheckId,
	}); err != nil {
		return nil, err
	}

	// Валидация check_id
	if err := h.validator.ValidateStringLength(req.CheckId, "check_id", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "GetCheckHistory", req.CheckId)
	}

	// Валидация limit
	if req.Limit < 1 || req.Limit > 1000 {
		return nil, h.LogError(ctx, fmt.Errorf("limit must be between 1 and 1000"), "GetCheckHistory", req.CheckId)
	}

	// Валидация времени если указано
	var startTime, endTime *time.Time
	if req.StartTime != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return nil, h.LogError(ctx, err, "GetCheckHistory", req.CheckId)
		}
		startTime = &t
	}

	if req.EndTime != "" {
		t, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return nil, h.LogError(ctx, err, "GetCheckHistory", req.CheckId)
		}
		endTime = &t
	}

	// Получаем историю проверок
	history, err := h.checkService.GetCheckHistory(ctx, req.CheckId, int(req.Limit), startTime, endTime)
	if err != nil {
		h.LogError(ctx, err, "GetCheckHistory", req.CheckId)
		return nil, status.Errorf(codes.Internal, "failed to get check history: %v", err)
	}

	// Конвертируем результаты в protobuf
	results := make([]*corev1.CheckResult, len(history))
	for i, result := range history {
		results[i] = h.convertCheckResultToProto(result)
	}

	h.LogOperationSuccess(ctx, "GetCheckHistory", map[string]interface{}{
		"check_id": req.CheckId,
		"count":    len(results),
		"limit":    req.Limit,
	})

	return &corev1.GetCheckHistoryResponse{
		Results: results,
	}, nil
}

// Вспомогательные методы

// convertCheckResultToProto конвертирует CheckResult в protobuf
func (h *CoreHandler) convertCheckResultToProto(result *domain.CheckResult) *corev1.CheckResult {
	if result == nil {
		return nil
	}

	statusCode := int32(0)
	if result.StatusCode != nil {
		statusCode = int32(*result.StatusCode)
	}

	return &corev1.CheckResult{
		CheckId:        result.CheckID,
		Status:         result.Status,
		ResponseTimeMs: result.ResponseTimeMs,
		StatusCode:     statusCode,
		ErrorMessage:   result.ErrorMessage,
		ResponseBody:   result.ResponseBody,
		CreatedAt:      result.CreatedAt.Format(time.RFC3339),
	}
}

// generateExecutionID генерирует уникальный ID выполнения
func generateExecutionID() string {
	return fmt.Sprintf("exec_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond()%1000)
}
