package metrics

import (
	"context"
	"time"

	"UptimePingPlatform/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
)

// UptimeMetrics содержит метрики для uptime проверок
type UptimeMetrics struct {
	// Базовые метрики из pkg
	base *metrics.Metrics
	
	// Специфичные метрики для uptime проверок
	checkDuration *prometheus.HistogramVec
	checkTotal    *prometheus.CounterVec
	checkErrors   *prometheus.CounterVec
	checkActive   prometheus.Gauge
	
	// Дополнительные метрики
	lastSuccessTimestamp *prometheus.GaugeVec
	responseSize          *prometheus.HistogramVec
}

// NewUptimeMetrics создает новый экземпляр метрик для uptime проверок
func NewUptimeMetrics(serviceName string) *UptimeMetrics {
	base := metrics.NewMetrics(serviceName)
	
	// Создаем специфичные метрики для uptime
	checkDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: serviceName,
			Subsystem: "uptime",
			Name:      "check_duration_seconds",
			Help:      "Duration of uptime checks in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"type", "target", "status"},
	)
	
	checkTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "uptime",
			Name:      "check_total",
			Help:      "Total number of uptime checks performed",
		},
		[]string{"type", "target", "status"},
	)
	
	checkErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "uptime",
			Name:      "check_errors_total",
			Help:      "Total number of uptime check errors",
		},
		[]string{"type", "target", "error_type"},
	)
	
	checkActive := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: serviceName,
			Subsystem: "uptime",
			Name:      "check_active",
			Help:      "Number of currently active uptime checks",
		},
	)
	
	lastSuccessTimestamp := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: serviceName,
			Subsystem: "uptime",
			Name:      "last_success_timestamp_seconds",
			Help:      "Timestamp of the last successful uptime check",
		},
		[]string{"type", "target"},
	)
	
	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: serviceName,
			Subsystem: "uptime",
			Name:      "response_size_bytes",
			Help:      "Size of uptime check response in bytes",
			Buckets:   []float64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000},
		},
		[]string{"type", "target", "status"},
	)
	
	// Регистрируем метрики в Prometheus
	registerMetric(checkDuration)
	registerMetric(checkTotal)
	registerMetric(checkErrors)
	registerMetric(checkActive)
	registerMetric(lastSuccessTimestamp)
	registerMetric(responseSize)
	
	return &UptimeMetrics{
		base:                  base,
		checkDuration:         checkDuration,
		checkTotal:            checkTotal,
		checkErrors:           checkErrors,
		checkActive:           checkActive,
		lastSuccessTimestamp:  lastSuccessTimestamp,
		responseSize:          responseSize,
	}
}

// registerMetric безопасно регистрирует метрику
func registerMetric(collector prometheus.Collector) {
	if err := prometheus.Register(collector); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
}

// RecordCheckDuration записывает длительность выполнения проверки
func (um *UptimeMetrics) RecordCheckDuration(checkType, target, status string, duration time.Duration) {
	um.checkDuration.WithLabelValues(checkType, target, status).Observe(duration.Seconds())
}

// IncrementCheckTotal инкрементирует счетчик общего количества проверок
func (um *UptimeMetrics) IncrementCheckTotal(checkType, target, status string) {
	um.checkTotal.WithLabelValues(checkType, target, status).Inc()
}

// IncrementCheckErrors инкрементирует счетчик ошибок проверок
func (um *UptimeMetrics) IncrementCheckErrors(checkType, target, errorType string) {
	um.checkErrors.WithLabelValues(checkType, target, errorType).Inc()
}

// IncrementActiveChecks инкрементирует счетчик активных проверок
func (um *UptimeMetrics) IncrementActiveChecks() {
	um.checkActive.Inc()
}

// DecrementActiveChecks декрементирует счетчик активных проверок
func (um *UptimeMetrics) DecrementActiveChecks() {
	um.checkActive.Dec()
}

// RecordLastSuccessTimestamp записывает время последней успешной проверки
func (um *UptimeMetrics) RecordLastSuccessTimestamp(checkType, target string, timestamp time.Time) {
	um.lastSuccessTimestamp.WithLabelValues(checkType, target).Set(float64(timestamp.Unix()))
}

// RecordResponseSize записывает размер ответа проверки
func (um *UptimeMetrics) RecordResponseSize(checkType, target, status string, sizeBytes int64) {
	um.responseSize.WithLabelValues(checkType, target, status).Observe(float64(sizeBytes))
}

// RecordCheckResult записывает все метрики для результата проверки
func (um *UptimeMetrics) RecordCheckResult(checkType, target string, duration time.Duration, success bool, responseSize int64, errorMsg string) {
	status := "success"
	errorType := "none"
	
	if !success {
		status = "failure"
		if errorMsg != "" {
			errorType = categorizeError(errorMsg)
		}
	}
	
	// Записываем основные метрики
	um.RecordCheckDuration(checkType, target, status, duration)
	um.IncrementCheckTotal(checkType, target, status)
	um.RecordResponseSize(checkType, target, status, responseSize)
	
	// Записываем метрику ошибок если проверка неуспешна
	if !success {
		um.IncrementCheckErrors(checkType, target, errorType)
	} else {
		// Записываем время последнего успеха
		um.RecordLastSuccessTimestamp(checkType, target, time.Now())
	}
}

// categorizeError категоризирует тип ошибки для метрик
func categorizeError(errorMsg string) string {
	if len(errorMsg) == 0 {
		return "unknown"
	}
	
	// Простая категоризация на основе ключевых слов
	if containsIgnoreCase(errorMsg, "timeout") || containsIgnoreCase(errorMsg, "deadline") {
		return "timeout"
	}
	if containsIgnoreCase(errorMsg, "connection") || containsIgnoreCase(errorMsg, "network") {
		return "connection"
	}
	if containsIgnoreCase(errorMsg, "dns") || containsIgnoreCase(errorMsg, "resolve") {
		return "dns"
	}
	if containsIgnoreCase(errorMsg, "ssl") || containsIgnoreCase(errorMsg, "tls") || containsIgnoreCase(errorMsg, "certificate") {
		return "ssl"
	}
	if containsIgnoreCase(errorMsg, "404") {
		return "not_found"
	}
	if containsIgnoreCase(errorMsg, "500") || containsIgnoreCase(errorMsg, "502") || containsIgnoreCase(errorMsg, "503") || containsIgnoreCase(errorMsg, "504") {
		return "server_error"
	}
	if containsIgnoreCase(errorMsg, "400") || containsIgnoreCase(errorMsg, "401") || containsIgnoreCase(errorMsg, "403") {
		return "client_error"
	}
	
	return "unknown"
}

// containsIgnoreCase проверяет наличие подстроки без учета регистра
func containsIgnoreCase(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	
	// Простая реализация без учета регистра
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// toLower преобразует символ в нижний регистр
func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

// GetBaseMetrics возвращает базовые метрики из pkg
func (um *UptimeMetrics) GetBaseMetrics() *metrics.Metrics {
	return um.base
}

// GetHandler возвращает HTTP обработчик для эндпоинта /metrics
func (um *UptimeMetrics) GetHandler() interface{} {
	return um.base.GetHandler()
}

// TraceCheck выполняет трассировку проверки с использованием OpenTelemetry
func (um *UptimeMetrics) TraceCheck(ctx context.Context, checkType, target string, fn func(context.Context) error) error {
	// Используем tracer из базовых метрик
	_, span := um.base.Tracer.Start(ctx, "uptime_check")
	defer span.End()
	
	// Добавляем атрибуты
	span.SetAttributes(
		attribute.String("check.type", checkType),
		attribute.String("check.target", target),
	)
	
	// Выполняем функцию с контекстом трассировки
	err := fn(ctx)
	
	// Добавляем результат в атрибуты
	if err != nil {
		span.SetAttributes(
			attribute.String("check.status", "failure"),
			attribute.String("check.error", err.Error()),
		)
	} else {
		span.SetAttributes(attribute.String("check.status", "success"))
	}
	
	return err
}

// CheckMetricsObserver интерфейс для наблюдения за метриками проверок
type CheckMetricsObserver interface {
	OnCheckStarted(checkType, target string)
	OnCheckCompleted(checkType, target string, duration time.Duration, success bool, responseSize int64, errorMsg string)
	OnCheckError(checkType, target, errorType string)
}

// UptimeMetricsAdapter адаптер для использования UptimeMetrics как CheckMetricsObserver
type UptimeMetricsAdapter struct {
	metrics *UptimeMetrics
}

// NewUptimeMetricsAdapter создает новый адаптер
func NewUptimeMetricsAdapter(metrics *UptimeMetrics) *UptimeMetricsAdapter {
	return &UptimeMetricsAdapter{
		metrics: metrics,
	}
}

// OnCheckStarted вызывается при начале проверки
func (a *UptimeMetricsAdapter) OnCheckStarted(checkType, target string) {
	a.metrics.IncrementActiveChecks()
}

// OnCheckCompleted вызывается при завершении проверки
func (a *UptimeMetricsAdapter) OnCheckCompleted(checkType, target string, duration time.Duration, success bool, responseSize int64, errorMsg string) {
	a.metrics.RecordCheckResult(checkType, target, duration, success, responseSize, errorMsg)
	a.metrics.DecrementActiveChecks()
}

// OnCheckError вызывается при ошибке проверки
func (a *UptimeMetricsAdapter) OnCheckError(checkType, target, errorType string) {
	a.metrics.IncrementCheckErrors(checkType, target, errorType)
}
