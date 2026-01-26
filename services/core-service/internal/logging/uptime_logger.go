package logging

import (
	"context"
	"fmt"
	"time"

	"UptimePingPlatform/pkg/logger"
)

// UptimeLogger обертка над pkg/logger для uptime проверок
type UptimeLogger struct {
	base logger.Logger
}

// NewUptimeLogger создает новый экземпляр логгера для uptime проверок
func NewUptimeLogger(baseLogger logger.Logger) *UptimeLogger {
	return &UptimeLogger{
		base: baseLogger,
	}
}

// LogCheckStart логирует начало проверки
func (ul *UptimeLogger) LogCheckStart(ctx context.Context, checkType, target, checkID, executionID string) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "check_started"),
		logger.String("check_type", checkType),
		logger.String("target", target),
		logger.String("check_id", checkID),
		logger.String("execution_id", executionID),
		logger.String("component", "uptime_checker"),
	).Info("Starting uptime check")
}

// LogCheckComplete логирует завершение проверки
func (ul *UptimeLogger) LogCheckComplete(ctx context.Context, checkType, target, checkID, executionID string, duration time.Duration, success bool, statusCode int, responseSize int64) {
	status := "success"
	if !success {
		status = "failure"
	}
	
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "check_completed"),
		logger.String("check_type", checkType),
		logger.String("target", target),
		logger.String("check_id", checkID),
		logger.String("execution_id", executionID),
		logger.String("status", status),
		logger.String("component", "uptime_checker"),
		logger.Float64("duration_seconds", duration.Seconds()),
		logger.Int("status_code", statusCode),
		logger.Int("response_size_bytes", int(responseSize)),
	).Info("Uptime check completed")
}

// LogCheckError логирует ошибку проверки
func (ul *UptimeLogger) LogCheckError(ctx context.Context, checkType, target, checkID, executionID string, err error, duration time.Duration) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "check_failed"),
		logger.String("check_type", checkType),
		logger.String("target", target),
		logger.String("check_id", checkID),
		logger.String("execution_id", executionID),
		logger.String("component", "uptime_checker"),
		logger.Error(err),
		logger.Float64("duration_seconds", duration.Seconds()),
	).Error("Uptime check failed")
}

// LogCheckDebug логирует отладочную информацию о проверке
func (ul *UptimeLogger) LogCheckDebug(ctx context.Context, checkType, target, checkID, executionID string, message string, fields ...logger.Field) {
	allFields := []logger.Field{
		logger.CtxField(ctx),
		logger.String("event", "check_debug"),
		logger.String("check_type", checkType),
		logger.String("target", target),
		logger.String("check_id", checkID),
		logger.String("execution_id", executionID),
		logger.String("component", "uptime_checker"),
		logger.String("debug_message", message),
	}
	allFields = append(allFields, fields...)
	
	ul.base.With(allFields...).Debug("Uptime check debug")
}

// LogTaskReceived логирует получение задачи из очереди
func (ul *UptimeLogger) LogTaskReceived(ctx context.Context, taskID, checkID, tenantID string) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "task_received"),
		logger.String("task_id", taskID),
		logger.String("check_id", checkID),
		logger.String("tenant_id", tenantID),
		logger.String("component", "task_processor"),
	).Info("Task received from queue")
}

// LogTaskProcessed логирует обработку задачи
func (ul *UptimeLogger) LogTaskProcessed(ctx context.Context, taskID, checkID, tenantID string, success bool, err error) {
	status := "success"
	if !success {
		status = "failure"
	}
	
	fields := []logger.Field{
		logger.CtxField(ctx),
		logger.String("event", "task_processed"),
		logger.String("task_id", taskID),
		logger.String("check_id", checkID),
		logger.String("tenant_id", tenantID),
		logger.String("status", status),
		logger.String("component", "task_processor"),
	}
	
	if err != nil {
		fields = append(fields, logger.Error(err))
	}
	
	ul.base.With(fields...).Info("Task processing completed")
}

// LogIncidentCreated логирует создание инцидента
func (ul *UptimeLogger) LogIncidentCreated(ctx context.Context, checkID, tenantID, incidentID string, severity string) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "incident_created"),
		logger.String("check_id", checkID),
		logger.String("tenant_id", tenantID),
		logger.String("incident_id", incidentID),
		logger.String("severity", severity),
		logger.String("component", "incident_manager"),
	).Info("Incident created")
}

// LogIncidentUpdated логирует обновление инцидента
func (ul *UptimeLogger) LogIncidentUpdated(ctx context.Context, incidentID string, oldStatus, newStatus string) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "incident_updated"),
		logger.String("incident_id", incidentID),
		logger.String("old_status", oldStatus),
		logger.String("new_status", newStatus),
		logger.String("component", "incident_manager"),
	).Info("Incident updated")
}

// LogIncidentError логирует ошибку при работе с инцидентами
func (ul *UptimeLogger) LogIncidentError(ctx context.Context, operation string, incidentID string, err error) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "incident_error"),
		logger.String("operation", operation),
		logger.String("incident_id", incidentID),
		logger.String("component", "incident_manager"),
		logger.Error(err),
	).Error("Incident operation failed")
}

// LogConsumerStarted логирует запуск consumer
func (ul *UptimeLogger) LogConsumerStarted(ctx context.Context, queueName string) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "consumer_started"),
		logger.String("queue_name", queueName),
		logger.String("component", "rabbitmq_consumer"),
	).Info("RabbitMQ consumer started")
}

// LogConsumerStopped логирует остановку consumer
func (ul *UptimeLogger) LogConsumerStopped(ctx context.Context, queueName string) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "consumer_stopped"),
		logger.String("queue_name", queueName),
		logger.String("component", "rabbitmq_consumer"),
	).Info("RabbitMQ consumer stopped")
}

// LogConnectionError логирует ошибку подключения
func (ul *UptimeLogger) LogConnectionError(ctx context.Context, component string, err error) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "connection_error"),
		logger.String("component", component),
		logger.Error(err),
	).Error("Connection error")
}

// LogRetryAttempt логирует попытку retry
func (ul *UptimeLogger) LogRetryAttempt(ctx context.Context, operation string, attempt int, maxAttempts int, delay time.Duration) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "retry_attempt"),
		logger.String("operation", operation),
		logger.Int("attempt", attempt),
		logger.Int("max_attempts", maxAttempts),
		logger.Float64("delay_seconds", delay.Seconds()),
		logger.String("component", "retry_handler"),
	).Warn("Retry attempt")
}

// LogMetricsExported логирует экспорт метрик
func (ul *UptimeLogger) LogMetricsExported(ctx context.Context, metricsCount int) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "metrics_exported"),
		logger.Int("metrics_count", metricsCount),
		logger.String("component", "metrics_exporter"),
	).Debug("Metrics exported")
}

// LogServiceStarted логирует запуск сервиса
func (ul *UptimeLogger) LogServiceStarted(ctx context.Context, serviceName string, version string) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "service_started"),
		logger.String("service_name", serviceName),
		logger.String("version", version),
		logger.String("component", "service"),
	).Info("Service started")
}

// LogServiceStopped логирует остановку сервиса
func (ul *UptimeLogger) LogServiceStopped(ctx context.Context, serviceName string) {
	ul.base.With(
		logger.CtxField(ctx),
		logger.String("event", "service_stopped"),
		logger.String("service_name", serviceName),
		logger.String("component", "service"),
	).Info("Service stopped")
}

// WithCheckContext создает логгер с контекстом проверки
func (ul *UptimeLogger) WithCheckContext(ctx context.Context, checkType, target, checkID, executionID string) *UptimeLogger {
	return &UptimeLogger{
		base: ul.base.With(
			logger.CtxField(ctx),
			logger.String("check_type", checkType),
			logger.String("target", target),
			logger.String("check_id", checkID),
			logger.String("execution_id", executionID),
			logger.String("component", "uptime_checker"),
		),
	}
}

// WithComponent создает логгер с указанным компонентом
func (ul *UptimeLogger) WithComponent(component string) *UptimeLogger {
	return &UptimeLogger{
		base: ul.base.With(
			logger.String("component", component),
		),
	}
}

// WithTenant создает логгер с указанным tenant
func (ul *UptimeLogger) WithTenant(tenantID string) *UptimeLogger {
	return &UptimeLogger{
		base: ul.base.With(
			logger.String("tenant_id", tenantID),
		),
	}
}

// GetBaseLogger возвращает базовый логгер
func (ul *UptimeLogger) GetBaseLogger() logger.Logger {
	return ul.base
}

// Sync синхронизирует буферы логгера
func (ul *UptimeLogger) Sync() error {
	return ul.base.Sync()
}

// Контекстные функции для работы с trace_id

// ContextKey ключи для контекста
type ContextKey string

const (
	TraceIDKey     ContextKey = "trace_id"
	CheckIDKey     ContextKey = "check_id"
	ExecutionIDKey ContextKey = "execution_id"
	TenantIDKey    ContextKey = "tenant_id"
)

// WithTraceID добавляет trace_id в контекст
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// WithCheckID добавляет check_id в контекст
func WithCheckID(ctx context.Context, checkID string) context.Context {
	return context.WithValue(ctx, CheckIDKey, checkID)
}

// WithExecutionID добавляет execution_id в контекст
func WithExecutionID(ctx context.Context, executionID string) context.Context {
	return context.WithValue(ctx, ExecutionIDKey, executionID)
}

// WithTenantID добавляет tenant_id в контекст
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

// GetTraceID извлекает trace_id из контекста
func GetTraceID(ctx context.Context) string {
	if traceID := ctx.Value(TraceIDKey); traceID != nil {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	return ""
}

// GetCheckID извлекает check_id из контекста
func GetCheckID(ctx context.Context) string {
	if checkID := ctx.Value(CheckIDKey); checkID != nil {
		if id, ok := checkID.(string); ok {
			return id
		}
	}
	return ""
}

// GetExecutionID извлекает execution_id из контекста
func GetExecutionID(ctx context.Context) string {
	if executionID := ctx.Value(ExecutionIDKey); executionID != nil {
		if id, ok := executionID.(string); ok {
			return id
		}
	}
	return ""
}

// GetTenantID извлекает tenant_id из контекста
func GetTenantID(ctx context.Context) string {
	if tenantID := ctx.Value(TenantIDKey); tenantID != nil {
		if id, ok := tenantID.(string); ok {
			return id
		}
	}
	return ""
}

// GenerateTraceID генерирует новый trace ID
func GenerateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// WithCheckContext добавляет все контекстные данные проверки
func WithCheckContext(ctx context.Context, traceID, checkID, executionID, tenantID string) context.Context {
	ctx = WithTraceID(ctx, traceID)
	ctx = WithCheckID(ctx, checkID)
	ctx = WithExecutionID(ctx, executionID)
	if tenantID != "" {
		ctx = WithTenantID(ctx, tenantID)
	}
	return ctx
}

// Глобальный логгер для удобства использования
var GlobalUptimeLogger *UptimeLogger

// InitGlobalUptimeLogger инициализирует глобальный логгер
func InitGlobalUptimeLogger(baseLogger logger.Logger) {
	GlobalUptimeLogger = NewUptimeLogger(baseLogger)
}

// GetGlobalUptimeLogger возвращает глобальный логгер
func GetGlobalUptimeLogger() *UptimeLogger {
	if GlobalUptimeLogger == nil {
		// Создаем базовый логгер по умолчанию
		baseLogger, _ := logger.NewLogger("development", "info", "core-service", false)
		GlobalUptimeLogger = NewUptimeLogger(baseLogger)
	}
	return GlobalUptimeLogger
}
