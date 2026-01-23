package middleware

import (
	"encoding/json"
	//"fmt"
	"log"
	"net/http"
	"runtime"
)

// RecoveryMiddleware обрабатывает паники в обработчиках HTTP
func RecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Откладываем обработку паники
			defer func() {
				if err := recover(); err != nil {
					// Логируем панику с трейсом стека
					log.Printf("Panic recovered: %v\n", err)
					log.Printf("Stack trace:\n%s", string(debugStack()))

					// Устанавливаем заголовки ответа
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					// Формируем ответ об ошибке
					response := map[string]interface{}{
						"error": map[string]interface{}{
							"code":    "INTERNAL_ERROR",
							"message": "Internal server error",
						},
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
