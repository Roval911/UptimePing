package middleware

import (
	"net/http"
	"time"

	"UptimePingPlatform/pkg/ratelimit"
)

// RateLimitMiddleware создает middleware для ограничения частоты запросов
func RateLimitMiddleware(rateLimiter ratelimit.RateLimiter, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Определяем ключ для ограничения по IP адресу
			key := "ip:" + getIP(r)

			// Проверяем лимит запросов
			limitExceeded, err := rateLimiter.CheckRateLimit(r.Context(), key, limit, window)
			if err != nil {
				// В случае ошибки Rate Limiter разрешаем запрос
				http.Error(w, "Rate limit service unavailable", http.StatusInternalServerError)
				return
			}

			// Если лимит превышен, возвращаем ошибку
			if limitExceeded {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":{"code":"TOO_MANY_REQUESTS","message":"too many requests"}}`))
				return
			}

			// Продолжаем обработку запроса
			next.ServeHTTP(w, r)
		})
	}
}

// getIP извлекает IP адрес из запроса
func getIP(r *http.Request) string {
	// Проверяем X-Forwarded-For заголовок
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	// Проверяем X-Real-IP заголовок
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Возвращаем REMOTE_ADDR как последний вариант
	return r.RemoteAddr
}
