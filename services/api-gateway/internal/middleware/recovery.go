package middleware

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"UptimePingPlatform/pkg/logger"
)

// RecoveryMiddleware обрабатывает паники в обработчиках HTTP
func RecoveryMiddleware(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Откладываем обработку паники
			defer func() {
				if err := recover(); err != nil {
					// Логируем панику с трейсом стека
					log.Error("Panic recovered in HTTP handler",
						logger.Any("panic", err),
						logger.String("stack_trace", string(debugStack())),
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path),
						logger.String("remote_addr", r.RemoteAddr))

					// Устанавливаем заголовки ответа
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					// Формируем ответ об ошибке
					response := map[string]interface{}{
						"error": map[string]interface{}{
							"code":    "INTERNAL_ERROR",
							"message": "Internal server error",
						},
						"timestamp": time.Now().UTC().Format(time.RFC3339),
					}

					// Отправляем JSON ответ
					json.NewEncoder(w).Encode(response)
				}
			}()

			// Выполняем следующий обработчик
			next.ServeHTTP(w, r)
		})
	}
}

// debugStack возвращает трейс стека
func debugStack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}
