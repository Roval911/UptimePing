package interceptor

import (
	"context"
	"math/rand"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"UptimePingPlatform/pkg/logger"
)

// RetryInterceptor обеспечивает повторные попытки при ошибках gRPC
func RetryInterceptor(maxRetries int, backoffStrategy func(int) time.Duration, log logger.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var lastErr error

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

		// Пытаемся выполнить вызов с повторными попытками
		for attempt := 0; attempt <= maxRetries; attempt++ {
			// Выполняем вызов
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err == nil {
				// Успешно
				logFields = append(logFields, logger.Int("attempt", attempt+1))
				log.Debug("gRPC call succeeded", logFields...)
				return nil
			}

			// Сохраняем последнюю ошибку
			lastErr = err

			// Проверяем, можно ли повторить попытку
			if !isRetryableError(err) {
				// Ошибка не подлежит повтору
				logFields = append(logFields,
					logger.String("error", err.Error()),
					logger.Bool("retryable", false),
					logger.Int("attempt", attempt+1),
				)
				log.Error("gRPC call failed with non-retryable error", logFields...)
				return err
			}

			// Проверяем, не превышено ли количество попыток
			if attempt >= maxRetries {
				// Достигнуто максимальное количество попыток
				logFields = append(logFields,
					logger.String("error", err.Error()),
					logger.Bool("retryable", true),
					logger.Int("attempt", attempt+1),
					logger.Int("max_retries", maxRetries),
				)
				log.Error("gRPC call failed after max retries", logFields...)
				return lastErr
			}

			// Вычисляем задержку перед следующей попыткой
			delay := backoffStrategy(attempt)

			// Ждем перед следующей попыткой
			logFields = append(logFields,
				logger.String("error", err.Error()),
				logger.Bool("retryable", true),
				logger.Int("attempt", attempt+1),
				logger.Int("max_retries", maxRetries),
				logger.Float64("delay_ms", float64(delay.Milliseconds())),
			)
			log.Warn("gRPC call failed, retrying", logFields...)

			// Ждем задержку
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				// Контекст отменен
				logFields = append(logFields, logger.String("context_error", ctx.Err().Error()))
				log.Error("Context cancelled during retry", logFields...)
				return ctx.Err()
			}
		}

		// Возвращаем последнюю ошибку
		return lastErr
	}
}

// isRetryableError определяет, можно ли повторить попытку при ошибке
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Получаем статус ошибки
	st, ok := status.FromError(err)
	if !ok {
		// Не gRPC ошибка, считаем не подлежащей повтору
		return false
	}

	// Определяем коды ошибок, которые подлежат повтору
	switch st.Code() {
	case codes.DeadlineExceeded: // Превышено время ожидания
		return true
	case codes.Unavailable: // Сервис недоступен
		return true
	case codes.ResourceExhausted: // Ресурс исчерпан (например, rate limiting)
		return true
	case codes.Aborted: // Операция прервана
		return true
	case codes.Internal: // Внутренняя ошибка (может быть временной)
		return true
	case codes.Unknown: // Неизвестная ошибка (может быть временной)
		return true
	default:
		return false
	}
}

// BackoffLinear линейная стратегия backoff
func BackoffLinear(baseDelay time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		return baseDelay * time.Duration(attempt+1)
	}
}

// BackoffExponential экспоненциальная стратегия backoff
func BackoffExponential(baseDelay, maxDelay time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		// Используем степень двойки для экспоненциального роста
		delay := baseDelay
		for i := 0; i < attempt; i++ {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
				break
			}
		}
		return delay
	}
}

// BackoffExponentialWithJitter экспоненциальная стратегия backoff с jitter
func BackoffExponentialWithJitter(baseDelay, maxDelay time.Duration, jitterFactor float64) func(int) time.Duration {
	return func(attempt int) time.Duration {
		// Экспоненциальный рост
		delay := baseDelay
		for i := 0; i < attempt; i++ {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
				break
			}
		}

		// Добавляем jitter
		jitter := float64(delay) * jitterFactor
		jitterDuration := time.Duration(jitter)

		// Генерируем случайное значение в диапазоне [-jitter, +jitter]
		randomJitter := rand.Int63n(int64(jitterDuration*2)) - int64(jitterDuration)

		result := delay + time.Duration(randomJitter)

		// Гарантируем, что результат неотрицательный
		if result < 0 {
			result = 0
		}

		return result
	}
}
