package checker

import (
	"context"
	"strings"
	"time"

	"UptimePingPlatform/pkg/logger"
)

// BaseChecker базовая структура для всех checker'ов
type BaseChecker struct {
	logger logger.Logger
}

// NewBaseChecker создает новый BaseChecker
func NewBaseChecker(logger logger.Logger) *BaseChecker {
	return &BaseChecker{
		logger: logger,
	}
}

// LogOperationStart логирует начало операции
func (b *BaseChecker) LogOperationStart(ctx context.Context, operation string, fields ...map[string]interface{}) {
	loggerFields := make([]logger.Field, 0, len(fields))
	for _, field := range fields {
		for key, value := range field {
			loggerFields = append(loggerFields, logger.Any(key, value))
		}
	}
	b.logger.Info("Starting "+operation, loggerFields...)
}

// LogOperationSuccess логирует успешное завершение операции
func (b *BaseChecker) LogOperationSuccess(ctx context.Context, operation string, duration time.Duration) {
	b.logger.Info("Completed "+operation, 
		logger.Duration("duration", duration))
}

// LogError логирует ошибку
func (b *BaseChecker) LogError(ctx context.Context, err error, operation string, details ...string) {
	b.logger.Error("Error in "+operation, 
		logger.Error(err),
		logger.String("details", strings.Join(details, ", ")))
}
