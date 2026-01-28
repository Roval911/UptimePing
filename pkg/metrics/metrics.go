package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	
	"UptimePingPlatform/pkg/config"
)

// Metrics представляет систему метрик
type Metrics struct {
	// Стандартные метрики Prometheus
	RequestCount    *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	ErrorsCount     *prometheus.CounterVec

	// Дополнительные метрики
	ActiveConnections *prometheus.GaugeVec
	QueueSize         *prometheus.GaugeVec

	// OpenTelemetry Tracer
	Tracer trace.Tracer `json:"-"`
}

// NewMetricsFromConfig создает метрики из конфигурации pkg/config
func NewMetricsFromConfig(serviceName string, config *config.MetricsConfig) *Metrics {
	metricsConfig := &MetricsConfig{
		Enabled:        config.Enabled,
		Port:           config.Port,
		Path:           config.Path,
		ScrapeInterval: config.ScrapeInterval,
		Timeout:        config.Timeout,
		RetryAttempts:  config.RetryAttempts,
		Namespace:      config.Namespace,
		Subsystem:      config.Subsystem,
		Buckets:        config.Buckets,
		TracingEnabled: config.TracingEnabled,
		TracerName:     config.TracerName,
		SamplingRate:   config.SamplingRate,
		ServiceName:    config.ServiceName,
		ServiceVersion: config.ServiceVersion,
		EnableCustomMetrics: config.EnableCustomMetrics,
		EnableSystemMetrics:  config.EnableSystemMetrics,
	}
	
	return NewMetricsWithConfig(serviceName, metricsConfig)
}

// GetMetricsPath возвращает путь для метрик из конфигурации
func (m *Metrics) GetMetricsPath(config *config.MetricsConfig) string {
	if config != nil && config.Path != "" {
		return config.Path
	}
	return "/metrics"
}

// GetMetricsPort возвращает порт для метрик из конфигурации
func (m *Metrics) GetMetricsPort(config *config.MetricsConfig) int {
	if config != nil && config.Port != 0 {
		return config.Port
	}
	return 9090
}

// NewMetrics создает новую систему метрик (обратная совместимость)
func NewMetrics(serviceName string) *Metrics {
	return NewMetricsWithConfig(serviceName, &MetricsConfig{
		Enabled:        true,
		Namespace:     "uptimeping",
		Subsystem:     "http",
		Buckets:       prometheus.DefBuckets,
		TracingEnabled: true,
		TracerName:    "uptimeping-tracer",
		SamplingRate:  1.0,
		ServiceName:   serviceName,
		ServiceVersion: "1.0.0",
	})
}

// NewMetricsWithConfig создает новую систему метрик с кастомной конфигурацией
func NewMetricsWithConfig(serviceName string, config *MetricsConfig) *Metrics {
	if !config.Enabled {
		return &Metrics{}
	}

	// Устанавливаем имя сервиса если не указано
	if config.ServiceName == "" {
		config.ServiceName = serviceName
	}

	// Регистрируем стандартные метрики Prometheus
	requestCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "request_duration_seconds",
			Help:      "Duration of HTTP requests in seconds",
			Buckets:   config.Buckets,
		},
		[]string{"method", "endpoint"},
	)

	errorsCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Namespace,
			Subsystem: config.Subsystem,
			Name:      "errors_total",
			Help:      "Total number of HTTP errors",
		},
		[]string{"method", "endpoint", "error_type"},
	)

	// Дополнительные метрики
	var activeConnections *prometheus.GaugeVec
	var queueSize *prometheus.GaugeVec

	if config.EnableCustomMetrics {
		activeConnections = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: "system",
				Name:      "active_connections",
				Help:      "Number of active connections",
			},
			[]string{"type"},
		)

		queueSize = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: "system",
				Name:      "queue_size",
				Help:      "Size of message queue",
			},
			[]string{"queue_name"},
		)
	} else {
		// Создаем пустые метрики для предотвращения nil pointer dereference
		activeConnections = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: "system",
				Name:      "active_connections",
				Help:      "Number of active connections",
			},
			[]string{"type"},
		)

		queueSize = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: "system",
				Name:      "queue_size",
				Help:      "Size of message queue",
			},
			[]string{"queue_name"},
		)
	}

	// Регистрируем метрики в Prometheus
	registerMetricSafe(requestCount)
	registerMetricSafe(requestDuration)
	registerMetricSafe(errorsCount)
	registerMetricSafe(activeConnections)
	registerMetricSafe(queueSize)

	// Инициализируем OpenTelemetry если включено
	var tracer trace.Tracer
	if config.TracingEnabled {
		tracer = initializeOpenTelemetry(config)
	}

	return &Metrics{
		RequestCount:       requestCount,
		RequestDuration:   requestDuration,
		ErrorsCount:       errorsCount,
		ActiveConnections: activeConnections,
		QueueSize:         queueSize,
		Tracer:            tracer,
	}
}

// MetricsConfig представляет конфигурацию метрик
type MetricsConfig struct {
	Enabled        bool
	Port           int
	Path           string
	ScrapeInterval string
	Timeout        string
	RetryAttempts  int
	Namespace      string
	Subsystem      string
	Buckets        []float64
	TracingEnabled bool
	TracerName     string
	SamplingRate   float64
	ServiceName    string
	ServiceVersion string
	EnableCustomMetrics bool
	EnableSystemMetrics  bool
}

// registerMetricSafe безопасно регистрирует метрику
func registerMetricSafe(collector prometheus.Collector) {
	if err := prometheus.Register(collector); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
}

// initializeOpenTelemetry инициализирует OpenTelemetry с конфигурацией
func initializeOpenTelemetry(config *MetricsConfig) trace.Tracer {
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		)),
		tracesdk.WithSampler(tracesdk.TraceIDRatioBased(config.SamplingRate)),
	)

	otel.SetTracerProvider(tp)

	tracer := tp.Tracer(config.TracerName)

	return tracer
}

// GetHandler возвращает HTTP обработчик для эндпоинта /metrics
func (m *Metrics) GetHandler() http.Handler {
	return promhttp.Handler()
}

// Middleware создает middleware для сбора метрик
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Начинаем трассировку с OpenTelemetry
		_, span := m.Tracer.Start(r.Context(), r.URL.Path)
		defer span.End()

		// Создаем обертку для ResponseWriter для перехвата статуса
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Запоминаем время начала запроса
		start := time.Now()

		// Выполняем следующий обработчик
		next.ServeHTTP(wrapped, r)

		// Собираем метрики
		duration := time.Since(start).Seconds()
		epoch := r.URL.Path

		// Обновляем счетчики
		m.RequestCount.WithLabelValues(r.Method, epoch, fmt.Sprintf("%d", wrapped.statusCode)).Inc()
		m.RequestDuration.WithLabelValues(r.Method, epoch).Observe(duration)

		// Если статус ошибочный, увеличиваем счетчик ошибок
		if wrapped.statusCode >= 400 {
			errorType := "unknown"
			if wrapped.statusCode >= 500 {
				errorType = "server_error"
			} else {
				errorType = "client_error"
			}
			m.ErrorsCount.WithLabelValues(r.Method, epoch, errorType).Inc()
		}

		// Добавляем атрибуты в спан OpenTelemetry
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.Int("http.status_code", wrapped.statusCode),
			attribute.Float64("http.duration", duration),
		)
	})
}

// responseWriter обертка для перехвата статуса ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader перехватывает установку статуса
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// InitializeOpenTelemetry инициализирует OpenTelemetry с экспортером
func InitializeOpenTelemetry(serviceName string) error {
	// Создаем базовый провайдер трассировки
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		)),
	)

	// Устанавливаем глобальный провайдер трассировки
	otel.SetTracerProvider(tp)

	return nil
}

// SetActiveConnections устанавливает количество активных подключений
func (m *Metrics) SetActiveConnections(connectionType string, count float64) {
	m.ActiveConnections.WithLabelValues(connectionType).Set(count)
}

// IncrementActiveConnections увеличивает счетчик активных подключений
func (m *Metrics) IncrementActiveConnections(connectionType string) {
	m.ActiveConnections.WithLabelValues(connectionType).Inc()
}

// DecrementActiveConnections уменьшает счетчик активных подключений
func (m *Metrics) DecrementActiveConnections(connectionType string) {
	m.ActiveConnections.WithLabelValues(connectionType).Dec()
}

// SetQueueSize устанавливает размер очереди
func (m *Metrics) SetQueueSize(queueName string, size float64) {
	m.QueueSize.WithLabelValues(queueName).Set(size)
}

// IncrementQueueSize увеличивает размер очереди
func (m *Metrics) IncrementQueueSize(queueName string) {
	m.QueueSize.WithLabelValues(queueName).Inc()
}

// DecrementQueueSize уменьшает размер очереди
func (m *Metrics) DecrementQueueSize(queueName string) {
	m.QueueSize.WithLabelValues(queueName).Dec()
}
