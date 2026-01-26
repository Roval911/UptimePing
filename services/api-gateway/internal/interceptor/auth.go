package interceptor

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
)

// AuthInterceptor обеспечивает передачу JWT токена между сервисами
func AuthInterceptor(log logger.Logger) grpc.UnaryClientInterceptor {
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
		log.Debug("gRPC call started", logFields...)

		// Извлекаем JWT токен из контекста
		//TODO В реальном приложении токен может извлекаться из разных источников
		// Например, из контекста HTTP запроса, который был передан в gRPC контекст
		token, ok := ctx.Value("jwt_token").(string)
		if !ok {
			log.Warn("JWT token not found in context", logFields...)
			// Продолжаем вызов без токена - некоторые методы могут быть публичными
		} else {
			// Добавляем токен в metadata
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}

		// Выполняем вызов
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			// Логируем ошибку
			logFields = append(logFields, logger.String("error", err.Error()))
			log.Error("gRPC call failed", logFields...)

			// Преобразуем gRPC ошибку в кастомную ошибку
			return errors.Wrap(err, errors.ErrInternal, "gRPC call failed")
		}

		// Логируем успешный вызов
		log.Debug("gRPC call completed", logFields...)
		return nil
	}
}

// AuthServerInterceptor обеспечивает извлечение JWT токена на сервере
func AuthServerInterceptor(log logger.Logger) grpc.UnaryServerInterceptor {
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
		log.Debug("gRPC server call started", logFields...)

		// Извлекаем metadata из контекста
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			log.Warn("No metadata in context", logFields...)
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		// Ищем authorization заголовок
		authHeaders := md["authorization"]
		if len(authHeaders) == 0 {
			log.Warn("Authorization header missing", logFields...)
			return nil, status.Error(codes.Unauthenticated, "authorization header missing")
		}

		// Извлекаем токен (ожидаем формат "Bearer <token>")
		authHeader := authHeaders[0]
		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			log.Warn("Invalid authorization format",
				logger.String("auth_header", authHeader),
				logger.String("grpc_method", info.FullMethod),
				logger.String("trace_id", traceID),
			)
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
		}

		// Извлекаем токен
		token := authHeader[7:]
		log.Debug("JWT token extracted",
			logger.String("grpc_method", info.FullMethod),
			logger.String("trace_id", traceID),
			logger.Int("token_length", len(token)),
		)

		// Добавляем токен в контекст для дальнейшего использования
		ctx = context.WithValue(ctx, "jwt_token", token)

		// Выполняем обработчик
		resp, err := handler(ctx, req)
		if err != nil {
			// Логируем ошибку
			logFields = append(logFields, logger.String("error", err.Error()))
			log.Error("gRPC server call failed", logFields...)
			return nil, err
		}

		// Логируем успешный вызов
		log.Debug("gRPC server call completed", logFields...)
		return resp, nil
	}
}
