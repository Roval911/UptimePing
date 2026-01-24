package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"UptimePingPlatform/pkg/logger"
)

// LoggingMiddleware логирует все HTTP запросы
func LoggingMiddleware(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Генерируем trace_id для запроса
			traceID := generateTraceID()

			// Создаем новый контекст с trace_id
			ctx := context.WithValue(r.Context(), "trace_id", traceID)
			r = r.WithContext(ctx)

			// Создаем поле для логирования
			logFields := []logger.Field{
				logger.String("method", r.Method),
				logger.String("url", r.URL.String()),
				logger.String("remote_addr", r.RemoteAddr),
				logger.String("user_agent", r.UserAgent()),
				logger.String("trace_id", traceID),
			}

			// Логируем начало запроса
			log.Info("Started request", logFields...)

			// Запоминаем время начала
			start := time.Now()

			// Создаем обертку для ResponseWriter для перехвата статуса
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Выполняем следующий обработчик
			next.ServeHTTP(wrapped, r)

			// Добавляем статус код к полям лога
			logFields = append(logFields, logger.Int("status_code", wrapped.statusCode))
			logFields = append(logFields, logger.Float64("duration_ms", float64(time.Since(start).Milliseconds())))

			// Логируем завершение запроса
			log.Info("Completed request", logFields...)
		})
	}
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

// generateTraceID генерирует уникальный идентификатор запроса
func generateTraceID() string {
	// Генерируем UUID v4 с использованием криптографически безопасного генератора
	// Формат: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	b := make([]byte, 16)
	// Заполняем случайными байтами
	_, err := rand.Read(b)
	if err != nil {
		// В случае ошибки генерации возвращаем fallback вариант
		return "trace-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Устанавливаем версию UUID (4)
	b[6] = (b[6] & 0x0f) | 0x40
	// Устанавливаем variant (RFC 4122)
	b[8] = (b[8] & 0x3f) | 0x80

	// Форматируем в стандартный UUID формат
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
