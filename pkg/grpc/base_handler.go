package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"UptimePingPlatform/pkg/logger"
)

// BaseHandler предоставляет общую функциональность для gRPC обработчиков
type BaseHandler struct {
	logger logger.Logger
}

// NewBaseHandler создает новый BaseHandler
func NewBaseHandler(logger logger.Logger) *BaseHandler {
	return &BaseHandler{
		logger: logger,
	}
}

// ValidateRequiredFields проверяет обязательные поля
func (h *BaseHandler) ValidateRequiredFields(ctx context.Context, operation string, fields map[string]string) error {
	for field, value := range fields {
		if value == "" {
			h.logger.Warn("Validation failed",
				logger.CtxField(ctx),
				logger.String("operation", operation),
				logger.String("field", field),
				logger.String("error", field+" is required"),
			)
			return status.Errorf(codes.InvalidArgument, "%s is required", field)
		}
	}
	return nil
}

// LogOperationStart логирует начало операции
func (h *BaseHandler) LogOperationStart(ctx context.Context, operation string, details map[string]interface{}) {
	fields := []logger.Field{
		logger.String("operation", operation),
		logger.CtxField(ctx),
	}

	for key, value := range details {
		switch v := value.(type) {
		case string:
			fields = append(fields, logger.String(key, v))
		case int:
			fields = append(fields, logger.Int(key, v))
		case int32:
			fields = append(fields, logger.Int32(key, v))
		case bool:
			fields = append(fields, logger.Bool(key, v))
		}
	}

	h.logger.Info("Operation started", fields...)
}

// LogOperationSuccess логирует успешное завершение операции
func (h *BaseHandler) LogOperationSuccess(ctx context.Context, operation string, details map[string]interface{}) {
	fields := []logger.Field{
		logger.String("operation", operation),
		logger.CtxField(ctx),
	}

	for key, value := range details {
		switch v := value.(type) {
		case string:
			fields = append(fields, logger.String(key, v))
		case int:
			fields = append(fields, logger.Int(key, v))
		case int32:
			fields = append(fields, logger.Int32(key, v))
		case bool:
			fields = append(fields, logger.Bool(key, v))
		}
	}

	h.logger.Info("Operation completed successfully", fields...)
}

// LogError логирует ошибку с конвертацией в gRPC статус
func (h *BaseHandler) LogError(ctx context.Context, err error, operation string, id string) error {
	if err == nil {
		return nil
	}

	// Логируем ошибку с контекстом
	h.logger.Error("Operation failed",
		logger.CtxField(ctx),
		logger.String("operation", operation),
		logger.String("id", id),
		logger.Error(err),
	)

	// Конвертация в gRPC статус
	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "not found"):
		return status.Errorf(codes.NotFound, "resource not found: %s", errMsg)
	case strings.Contains(errMsg, "already exists"):
		return status.Errorf(codes.AlreadyExists, "resource already exists: %s", errMsg)
	case strings.Contains(errMsg, "validation failed") || strings.Contains(errMsg, "required"):
		return status.Errorf(codes.InvalidArgument, "validation failed: %s", errMsg)
	case strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "forbidden"):
		return status.Errorf(codes.PermissionDenied, "access denied: %s", errMsg)
	case strings.Contains(errMsg, "timeout"):
		return status.Errorf(codes.DeadlineExceeded, "operation timeout: %s", errMsg)
	case strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network"):
		return status.Errorf(codes.Unavailable, "service unavailable: %s", errMsg)
	default:
		return status.Errorf(codes.Internal, "internal server error: %s", errMsg)
	}
}
