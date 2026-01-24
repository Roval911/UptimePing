package interceptor

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"UptimePingPlatform/pkg/logger"
)

// LoggingInterceptor логирует gRPC вызовы
func LoggingInterceptor(log logger.Logger) grpc.UnaryClientInterceptor {
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

		// Логируем начало вызова
		start := time.Now()
		log.Debug("gRPC call started", logFields...)

		// Выполняем вызов
		err := invoker(ctx, method, req, reply, cc, opts...)

		// Подсчитываем продолжительность
		duration := time.Since(start)
		logFields = append(logFields, logger.Float64("duration_ms", float64(duration.Milliseconds())))

		if err != nil {
			// Логируем ошибку
			st, _ := status.FromError(err)
			logFields = append(logFields, 
				logger.String("error", err.Error()),
				logger.Int("grpc_code", int(st.Code())),
				logger.String("grpc_message", st.Message()),
			)
			log.Error("gRPC call failed", logFields...)
		} else {
			// Логируем успешный вызов
			log.Debug("gRPC call completed", logFields...)
		}

		return err
	}
}

// LoggingServerInterceptor логирует gRPC вызовы на сервере
func LoggingServerInterceptor(log logger.Logger) grpc.UnaryServerInterceptor {
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

		// Логируем начало вызова
		start := time.Now()
		log.Debug("gRPC server call started", logFields...)

		// Выполняем обработчик
		resp, err := handler(ctx, req)

		// Подсчитываем продолжительность
		duration := time.Since(start)
		logFields = append(logFields, logger.Float64("duration_ms", float64(duration.Milliseconds())))

		if err != nil {
			// Логируем ошибку
			st, _ := status.FromError(err)
			logFields = append(logFields, 
				logger.String("error", err.Error()),
				logger.Int("grpc_code", int(st.Code())),
				logger.String("grpc_message", st.Message()),
			)
			log.Error("gRPC server call failed", logFields...)
		} else {
			// Логируем успешный вызов
			log.Debug("gRPC server call completed", logFields...)
		}

		return resp, err
	}
}
