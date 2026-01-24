package interceptor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"UptimePingPlatform/pkg/logger"
)

// MetricsInterceptor собирает метрики по gRPC вызовам
type MetricsInterceptor struct {
	counter   *prometheus.CounterVec
	histogram *prometheus.HistogramVec
	log       logger.Logger
}

// NewMetricsInterceptor создает новый MetricsInterceptor
func NewMetricsInterceptor(counter *prometheus.CounterVec, histogram *prometheus.HistogramVec, log logger.Logger) *MetricsInterceptor {
	return &MetricsInterceptor{
		counter:   counter,
		histogram: histogram,
		log:       log,
	}
}

// UnaryClientInterceptor собирает метрики для клиентских вызовов
func (m *MetricsInterceptor) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Извлекаем trace_id из контекста
		traceID, ok := ctx.Value("trace_id").(string)
		if !ok {
			traceID = "unknown"
		}

		// Создаем поле для логирования
		logFields := []logger.Field{
			logger.String("grpc_method", method),
			logger.String("trace_id", traceID),
		}

		// Подготавливаем метрики
		parts := splitMethodName(method)
		service := parts[0]
		methodStr := parts[1]

		// Начинаем таймер
		start := time.Now()

		// Выполняем вызов
		err := invoker(ctx, method, req, reply, cc, opts...)

		// Подсчитываем продолжительность
		duration := time.Since(start)

		// Определяем код ошибки
		code := "OK"
		if err != nil {
			st, _ := status.FromError(err)
			code = st.Code().String()
			// Логируем ошибку
			logFields = append(logFields,
				logger.String("error", err.Error()),
				logger.String("grpc_code", code),
			)
			m.log.Error("gRPC call failed", logFields...)
		} else {
			m.log.Debug("gRPC call completed", logFields...)
		}

		// Собираем метрики
		m.counter.WithLabelValues(service, methodStr, code).Inc()
		m.histogram.WithLabelValues(service, methodStr, code).Observe(duration.Seconds())

		return err
	}
}

// UnaryServerInterceptor собирает метрики для серверных вызовов
func (m *MetricsInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Извлекаем trace_id из контекста
		traceID, ok := ctx.Value("trace_id").(string)
		if !ok {
			traceID = "unknown"
		}

		// Создаем поле для логирования
		logFields := []logger.Field{
			logger.String("grpc_method", info.FullMethod),
			logger.String("trace_id", traceID),
		}

		// Подготавливаем метрики
		parts := splitMethodName(info.FullMethod)
		service := parts[0]
		methodStr := parts[1]

		// Начинаем таймер
		start := time.Now()

		// Выполняем обработчик
		resp, err := handler(ctx, req)

		// Подсчитываем продолжительность
		duration := time.Since(start)

		// Определяем код ошибки
		code := "OK"
		if err != nil {
			st, _ := status.FromError(err)
			code = st.Code().String()
			// Логируем ошибку
			logFields = append(logFields,
				logger.String("error", err.Error()),
				logger.String("grpc_code", code),
			)
			m.log.Error("gRPC server call failed", logFields...)
		} else {
			m.log.Debug("gRPC server call completed", logFields...)
		}

		// Собираем метрики
		m.counter.WithLabelValues(service, methodStr, code).Inc()
		m.histogram.WithLabelValues(service, methodStr, code).Observe(duration.Seconds())

		return resp, err
	}
}

// splitMethodName разбивает полное имя метода на service и method
func splitMethodName(fullMethod string) []string {
	parts := strings.Split(fullMethod, "/")
	if len(parts) != 3 {
		return []string{"unknown", "unknown"}
	}
	return []string{parts[1], parts[2]}
}

var (
	// once гарантирует однократную инициализацию
	once sync.Once
	// methodCounter счетчик вызовов методов
	methodCounter *prometheus.CounterVec
	// methodDurationHistogram гистограмма длительности вызовов методов
	methodDurationHistogram *prometheus.HistogramVec
)

// MustRegisterMetrics регистрирует метрики, паникуя при ошибке
func MustRegisterMetrics() {
	once.Do(func() {
		methodCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "grpc_method_count",
				Help: "Total number of gRPC method calls",
			},
			[]string{"service", "method", "code"},
		)
		if err := prometheus.Register(methodCounter); err != nil {
			panic(fmt.Sprintf("failed to register methodCounter: %v", err))
		}

		methodDurationHistogram = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "grpc_method_duration_seconds",
				Help:    "Histogram of gRPC method call duration",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "method", "code"},
		)
		if err := prometheus.Register(methodDurationHistogram); err != nil {
			panic(fmt.Sprintf("failed to register methodDurationHistogram: %v", err))
		}
	})
}
