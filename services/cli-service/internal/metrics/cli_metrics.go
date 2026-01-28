package metrics

import (
	"context"
	"time"

	"UptimePingPlatform/pkg/metrics"
	"UptimePingPlatform/pkg/logger"
)

// CLIMetrics содержит метрики для CLI операций
type CLIMetrics struct {
	metrics.Metrics
	logger logger.Logger
}

// NewCLIMetrics создает новые метрики для CLI
func NewCLIMetrics(logger logger.Logger) *CLIMetrics {
	m := metrics.NewMetrics("cli-service")
	
	return &CLIMetrics{
		Metrics: *m,
		logger:  logger,
	}
}

// CommandExecuted регистрирует выполнение команды
func (c *CLIMetrics) CommandExecuted(ctx context.Context, command string, success bool, duration time.Duration) {
	c.logger.Info("Command executed",
		logger.String("command", command),
		logger.Bool("success", success),
		logger.Duration("duration", duration))

	// Регистрируем метрики
	c.RequestCount.WithLabelValues(
		"cli",
		"command",
		command,
		getStatusLabel(success),
	).Inc()

	c.RequestDuration.WithLabelValues(
		"cli",
		"command",
		command,
	).Observe(duration.Seconds())

	// Если команда неуспешна, увеличиваем счетчик ошибок
	if !success {
		c.ErrorsCount.WithLabelValues(
			"cli",
			"command",
			command,
			"execution_failed",
		).Inc()
	}
}

// OutputGenerated регистрирует генерацию вывода
func (c *CLIMetrics) OutputGenerated(ctx context.Context, format string, recordCount int, duration time.Duration) {
	c.logger.Info("Output generated",
		logger.String("format", format),
		logger.Int("record_count", recordCount),
		logger.Duration("duration", duration))

	// Регистрируем метрики
	c.RequestCount.WithLabelValues(
		"cli",
		"output",
		format,
		"success",
	).Inc()

	c.RequestDuration.WithLabelValues(
		"cli",
		"output",
		format,
	).Observe(duration.Seconds())
}

// CompletionGenerated регистрирует генерацию автодополнения
func (c *CLIMetrics) CompletionGenerated(ctx context.Context, shell string, success bool, duration time.Duration) {
	c.logger.Info("Completion generated",
		logger.String("shell", shell),
		logger.Bool("success", success),
		logger.Duration("duration", duration))

	// Регистрируем метрики
	c.RequestCount.WithLabelValues(
		"cli",
		"completion",
		shell,
		getStatusLabel(success),
	).Inc()

	c.RequestDuration.WithLabelValues(
		"cli",
		"completion",
		shell,
	).Observe(duration.Seconds())
}

// ExportPerformed регистрирует операцию экспорта
func (c *CLIMetrics) ExportPerformed(ctx context.Context, exportFormat string, success bool, duration time.Duration) {
	c.logger.Info("Export performed",
		logger.String("format", exportFormat),
		logger.Bool("success", success),
		logger.Duration("duration", duration))

	// Регистрируем метрики
	c.RequestCount.WithLabelValues(
		"cli",
		"export",
		exportFormat,
		getStatusLabel(success),
	).Inc()

	c.RequestDuration.WithLabelValues(
		"cli",
		"export",
		exportFormat,
	).Observe(duration.Seconds())
}

// ContextOperation регистрирует операцию с контекстом
func (c *CLIMetrics) ContextOperation(ctx context.Context, operation string, success bool, duration time.Duration) {
	c.logger.Info("Context operation",
		logger.String("operation", operation),
		logger.Bool("success", success),
		logger.Duration("duration", duration))

	// Регистрируем метрики
	c.RequestCount.WithLabelValues(
		"cli",
		"context",
		operation,
		getStatusLabel(success),
	).Inc()

	c.RequestDuration.WithLabelValues(
		"cli",
		"context",
		operation,
	).Observe(duration.Seconds())
}

// APIRequest регистрирует запрос к API
func (c *CLIMetrics) APIRequest(ctx context.Context, endpoint string, method string, statusCode int, duration time.Duration) {
	c.logger.Info("API request",
		logger.String("endpoint", endpoint),
		logger.String("method", method),
		logger.Int("status_code", statusCode),
		logger.Duration("duration", duration))

	// Регистрируем метрики
	c.RequestCount.WithLabelValues(
		"cli",
		"api",
		endpoint,
		getStatusLabel(statusCode >= 200 && statusCode < 300),
	).Inc()

	c.RequestDuration.WithLabelValues(
		"cli",
		"api",
		endpoint,
	).Observe(duration.Seconds())
}

// ConfigOperation регистрирует операцию с конфигурацией
func (c *CLIMetrics) ConfigOperation(ctx context.Context, operation string, success bool, duration time.Duration) {
	c.logger.Info("Config operation",
		logger.String("operation", operation),
		logger.Bool("success", success),
		logger.Duration("duration", duration))

	// Регистрируем метрики
	c.RequestCount.WithLabelValues(
		"cli",
		"config",
		operation,
		getStatusLabel(success),
	).Inc()

	c.RequestDuration.WithLabelValues(
		"cli",
		"config",
		operation,
	).Observe(duration.Seconds())
}

// ValidationPerformed регистрирует операцию валидации
func (c *CLIMetrics) ValidationPerformed(ctx context.Context, validationType string, success bool, duration time.Duration) {
	c.logger.Info("Validation performed",
		logger.String("type", validationType),
		logger.Bool("success", success),
		logger.Duration("duration", duration))

	// Регистрируем метрики
	c.RequestCount.WithLabelValues(
		"cli",
		"validation",
		validationType,
		getStatusLabel(success),
	).Inc()

	c.RequestDuration.WithLabelValues(
		"cli",
		"validation",
		validationType,
	).Observe(duration.Seconds())
}

// GetMetricsHandler возвращает HTTP handler для метрик Prometheus
func (c *CLIMetrics) GetMetricsHandler() interface{} {
	return c.GetHandler()
}

// RecordError регистрирует ошибку
func (c *CLIMetrics) RecordError(ctx context.Context, component, operation, errorType string) {
	c.logger.Error("Error recorded",
		logger.String("component", component),
		logger.String("operation", operation),
		logger.String("error_type", errorType))

	c.ErrorsCount.WithLabelValues(
		"cli",
		component,
		operation,
		errorType,
	).Inc()
}

// RecordLatency записывает задержку операции
func (c *CLIMetrics) RecordLatency(ctx context.Context, component, operation string, duration time.Duration) {
	c.RequestDuration.WithLabelValues(
		"cli",
		component,
		operation,
	).Observe(duration.Seconds())
}

// RecordCounter увеличивает счетчик операций
func (c *CLIMetrics) RecordCounter(ctx context.Context, component, operation, status string) {
	c.RequestCount.WithLabelValues(
		"cli",
		component,
		operation,
		status,
	).Inc()
}

// getStatusLabel возвращает метку статуса
func getStatusLabel(success bool) string {
	if success {
		return "success"
	}
	return "error"
}

// OperationTimer измеряет время выполнения операции
type OperationTimer struct {
	metrics *CLIMetrics
	ctx     context.Context
	start   time.Time
}

// NewOperationTimer создает новый таймер операции
func (c *CLIMetrics) NewOperationTimer(ctx context.Context) *OperationTimer {
	return &OperationTimer{
		metrics: c,
		ctx:     ctx,
		start:   time.Now(),
	}
}

// Finish завершает операцию и регистрирует метрики
func (t *OperationTimer) Finish(component, operation string, success bool) {
	duration := time.Since(t.start)
	
	if success {
		t.metrics.RecordCounter(t.ctx, component, operation, "success")
	} else {
		t.metrics.RecordCounter(t.ctx, component, operation, "error")
	}
	
	t.metrics.RecordLatency(t.ctx, component, operation, duration)
}

// CommandTimer таймер для команд CLI
type CommandTimer struct {
	*OperationTimer
}

// NewCommandTimer создает новый таймер для команды
func (c *CLIMetrics) NewCommandTimer(ctx context.Context) *CommandTimer {
	return &CommandTimer{
		OperationTimer: c.NewOperationTimer(ctx),
	}
}

// Finish завершает команду и регистрирует метрики
func (t *CommandTimer) Finish(command string, success bool) {
	duration := time.Since(t.start)
	
	t.metrics.CommandExecuted(t.ctx, command, success, duration)
	t.OperationTimer.Finish("command", command, success)
}

// OutputTimer таймер для генерации вывода
type OutputTimer struct {
	*OperationTimer
}

// NewOutputTimer создает новый таймер для вывода
func (c *CLIMetrics) NewOutputTimer(ctx context.Context) *OutputTimer {
	return &OutputTimer{
		OperationTimer: c.NewOperationTimer(ctx),
	}
}

// Finish завершает генерацию вывода и регистрирует метрики
func (t *OutputTimer) Finish(format string, recordCount int, success bool) {
	duration := time.Since(t.start)
	
	t.metrics.OutputGenerated(t.ctx, format, recordCount, duration)
	t.OperationTimer.Finish("output", format, success)
}
