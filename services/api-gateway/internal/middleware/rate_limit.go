package middleware

import (
	"net/http"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/ratelimit"
)

// RateLimitMiddleware создает middleware для ограничения частоты запросов
func RateLimitMiddleware(rateLimiter ratelimit.RateLimiter, limit int, window time.Duration, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Определяем ключ для ограничения по IP адресу
			key := "ip:" + getIP(r)

			log.Debug("Rate limit middleware processing request",
				logger.String("method", r.Method),
				logger.String("path", r.URL.Path),
				logger.String("key", key),
				logger.Int("limit", limit),
				logger.String("window", window.String()))

			// Проверяем лимит запросов
			limitExceeded, err := rateLimiter.CheckRateLimit(r.Context(), key, limit, window)
			if err != nil {
				// В случае ошибки Rate Limiter разрешаем запрос
				log.Error("Rate limiter error, allowing request",
					logger.Error(err),
					logger.String("key", key))
				next.ServeHTTP(w, r)
				return
			}

			// Если лимит превышен, возвращаем ошибку
			if limitExceeded {
				log.Warn("Rate limit exceeded",
					logger.String("key", key),
					logger.Int("limit", limit),
					logger.String("window", window.String()),
					logger.String("method", r.Method),
					logger.String("path", r.URL.Path))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":{"code":"TOO_MANY_REQUESTS","message":"too many requests"}}`))
				return
			}

			// Продолжаем обработку запроса
			log.Debug("Rate limit check passed, proceeding to next handler")
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
