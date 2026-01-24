package interceptor

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"UptimePingPlatform/pkg/logger"
)

// CircuitBreakerState состояние circuit breaker
type CircuitBreakerState int

const (
	// StateClosed circuit breaker закрыт, запросы проходят
	StateClosed CircuitBreakerState = iota
	// StateOpen circuit breaker открыт, запросы блокируются
	StateOpen
	// StateHalfOpen circuit breaker в полузакрытом состоянии, один запрос проходит для проверки
	StateHalfOpen
)

// String возвращает строковое представление состояния
func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitBreakerInterceptor обеспечивает защиту от каскадных сбоев
type CircuitBreakerInterceptor struct {
	name             string
	failureThreshold int           // Количество последовательных ошибок, после которых открывается breaker
	recoveryTimeout  time.Duration // Время в состоянии open перед переходом в half-open
	halfOpenAttempts int           // Количество попыток в состоянии half-open
	failureWindow    time.Duration // Окно времени для подсчета ошибок

	// Внутреннее состояние
	state           CircuitBreakerState
	failureCount    int
	lastFailureTime time.Time
	halfOpenCount   int
	stateChangeTime time.Time
	mtx             sync.RWMutex
	log             logger.Logger
}

// NewCircuitBreakerInterceptor создает новый CircuitBreakerInterceptor
func NewCircuitBreakerInterceptor(name string, failureThreshold int, recoveryTimeout, failureWindow time.Duration, halfOpenAttempts int, log logger.Logger) *CircuitBreakerInterceptor {
	return &CircuitBreakerInterceptor{
		name:             name,
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
		failureWindow:    failureWindow,
		halfOpenAttempts: halfOpenAttempts,
		state:            StateClosed,
		log:              log,
	}
}

// UnaryClientInterceptor возвращает gRPC interceptor
func (cb *CircuitBreakerInterceptor) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
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
			logger.String("circuit_breaker", cb.name),
		}

		// Проверяем состояние circuit breaker
		if !cb.allowsRequest() {
			cb.log.Warn("Circuit breaker open, request rejected", logFields...)
			return status.Error(codes.Unavailable, "circuit breaker is open")
		}

		// Логируем начало вызова
		cb.log.Debug("Circuit breaker allows request", logFields...)

		// Выполняем вызов
		err := invoker(ctx, method, req, reply, cc, opts...)

		// Обновляем состояние circuit breaker
		cb.onRequestComplete(err != nil, logFields)

		return err
	}
}

// allowsRequest проверяет, можно ли выполнить запрос
func (cb *CircuitBreakerInterceptor) allowsRequest() bool {
	cb.mtx.Lock()
	defer cb.mtx.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Проверяем, не превышено ли количество ошибок в окне
		if now.Sub(cb.lastFailureTime) > cb.failureWindow {
			// Окно истекло, сбрасываем счетчик
			cb.failureCount = 0
		}
		return true
	case StateOpen:
		// Проверяем, не истекло ли время ожидания
		if now.Sub(cb.stateChangeTime) >= cb.recoveryTimeout {
			// Время истекло, переходим в полузакрытое состояние
			cb.state = StateHalfOpen
			cb.stateChangeTime = now
			cb.halfOpenCount = 0
			cb.failureCount = 0
			cb.log.Info("Circuit breaker state changed",
				logger.String("circuit_breaker", cb.name),
				logger.String("old_state", StateOpen.String()),
				logger.String("new_state", StateHalfOpen.String()),
			)
			// В состоянии half-open разрешаем одну попытку
			cb.halfOpenCount = 1
			return true
		}
		return false
	case StateHalfOpen:
		// Разрешаем ограниченное количество запросов
		if cb.halfOpenCount >= cb.halfOpenAttempts {
			// Достигнуто максимальное количество попыток, возвращаемся в открытое состояние
			cb.state = StateOpen
			cb.stateChangeTime = now
			cb.halfOpenCount = 0
			cb.log.Info("Circuit breaker state changed",
				logger.String("circuit_breaker", cb.name),
				logger.String("old_state", StateHalfOpen.String()),
				logger.String("new_state", StateOpen.String()),
			)
			return false
		}
		// Инкрементируем счетчик попыток
		cb.halfOpenCount++
		return true
	default:
		return true
	}
}

// onRequestComplete обрабатывает завершение запроса
func (cb *CircuitBreakerInterceptor) onRequestComplete(failed bool, logFields []logger.Field) {
	cb.mtx.Lock()
	defer cb.mtx.Unlock()

	now := time.Now()

	if failed {
		// Запрос завершился с ошибкой
		cb.failureCount++
		cb.lastFailureTime = now

		// Проверяем, нужно ли открыть circuit breaker
		if cb.state == StateClosed && cb.failureCount >= cb.failureThreshold {
			cb.log.Warn("Circuit breaker tripped",
				append(logFields,
					logger.Int("failure_count", cb.failureCount),
					logger.Int("failure_threshold", cb.failureThreshold),
				)...)
			cb.state = StateOpen
			cb.stateChangeTime = now
			cb.halfOpenCount = 0
			cb.log.Info("Circuit breaker state changed",
				logger.String("circuit_breaker", cb.name),
				logger.String("old_state", StateClosed.String()),
				logger.String("new_state", StateOpen.String()),
			)
		}
	} else {
		// Запрос успешен
		if cb.state == StateHalfOpen {
			// В состоянии half-open успешный запрос означает восстановление
			cb.log.Info("Circuit breaker restored", logFields...)
			cb.state = StateClosed
			cb.stateChangeTime = now
			cb.failureCount = 0
			cb.halfOpenCount = 0
			cb.log.Info("Circuit breaker state changed",
				logger.String("circuit_breaker", cb.name),
				logger.String("old_state", StateHalfOpen.String()),
				logger.String("new_state", StateClosed.String()),
			)
		} else if cb.state == StateClosed {
			// В состоянии closed успешный запрос сбрасывает счетчик ошибок
			cb.failureCount = 0
		}
	}
}
