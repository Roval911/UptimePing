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

	corev1 "UptimePingPlatform/gen/proto/api/core/v1"
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

	// Создаем задачу для выполнения
	task := &domain.Task{
		CheckID:     req.CheckId,
		ExecutionID: generateExecutionID(),
		CreatedAt:   time.Now().UTC(),
		Config:      make(map[string]interface{}),
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
		"check_id":     req.CheckId,
		"execution_id": result.ExecutionID,
		"success":      result.Success,
		"duration_ms":  result.DurationMs,
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
		h.LogError(ctx, err, "GetCheckStatus", req.CheckId)
		return nil, status.Errorf(codes.NotFound, "check not found: %v", err)
	}

	// Конвертируем в protobuf
	protoStatus := &corev1.CheckStatusResponse{
		CheckId:        req.CheckId,
		IsHealthy:      checkStatus.IsHealthy,
		ResponseTimeMs: int32(checkStatus.ResponseTimeMs),
		LastCheckedAt: checkStatus.LastCheckedAt,
	}

	h.LogOperationSuccess(ctx, "GetCheckStatus", map[string]interface{}{
		"check_id":        req.CheckId,
		"is_healthy":      checkStatus.IsHealthy,
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

	return &corev1.CheckResult{
		CheckId:      result.CheckID,
		ExecutionId:  result.ExecutionID,
		Success:      result.Success,
		DurationMs:   int32(result.DurationMs),
		StatusCode:   int32(result.StatusCode),
		Error:        result.Error,
		ResponseBody: result.ResponseBody,
		CheckedAt:    result.CheckedAt.Format(time.RFC3339),
	}
}

// generateExecutionID генерирует уникальный ID выполнения
func generateExecutionID() string {
	return fmt.Sprintf("exec_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond()%1000)
}
