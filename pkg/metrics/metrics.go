package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

// Metrics представляет систему метрик
type Metrics struct {
	// Стандартные метрики Prometheus
	RequestCount *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	ErrorsCount *prometheus.CounterVec

	// OpenTelemetry Tracer
	Tracer trace.Tracer `json:"-"`
}

// NewMetrics создает новую систему метрик
func NewMetrics(serviceName string) *Metrics {
	// Регистрируем стандартные метрики Prometheus
	requestCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: serviceName,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Duration of HTTP requests in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	errorsCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: "http",
			Name:      "errors_total",
			Help:      "Total number of HTTP errors",
		},
		[]string{"method", "endpoint", "error_type"},
	)

	// Регистрируем метрики в Prometheus
	if err := prometheus.Register(requestCount); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := prometheus.Register(requestDuration); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := prometheus.Register(errorsCount); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}

	// Создаем OpenTelemetry Tracer
	tracer := otel.Tracer(serviceName)

	return &Metrics{
		RequestCount:    requestCount,
		RequestDuration: requestDuration,
		ErrorsCount:     errorsCount,
		Tracer:          tracer,
	}
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

// InitializeOpenTelemetry инициализирует OpenTelemetry (опционально)
// В реальном приложении здесь будет настройка экспортеров (Jaeger, Zipkin и т.д.)
func InitializeOpenTelemetry(serviceName string) error {
	// В реальном приложении здесь будет настройка провайдера трассировки
	// Например:
	//
	// tp := tracesdk.NewTracerProvider(
	// 	tracesdk.WithSampler(tracesdk.AlwaysSample()),
	// 	tracesdk.WithBatcher(exporter),
	// 	tracesdk.WithResource(resource.NewWithAttributes(
	// 		semconv.SchemaURL,
	// 		semconv.ServiceNameKey.String(serviceName),
	// 	)),
	// )
	// otel.SetTracerProvider(tp)
	
	// Для примера просто устанавливаем базовый трейсер
	otel.SetTracerProvider(tracesdk.NewTracerProvider())
	
	return nil
}