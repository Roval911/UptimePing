package middleware

import (
	"net/http"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/ratelimit"
	"UptimePingPlatform/services/auth-service/internal/service"
)

// RateLimitMiddleware создает middleware для ограничения частоты запросов
// Поддерживает лимиты по IP адресу и по пользователю
// Использует sliding window алгоритм из pkg/ratelimit
func RateLimitMiddleware(rateLimiter ratelimit.RateLimiter, limit int, window time.Duration, byUser bool, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var key string

			// Определяем ключ для ограничения
			if byUser {
				// Получаем ID пользователя из контекста
				if userID, ok := r.Context().Value(service.UserIDKey).(string); ok {
					key = "user:" + userID
					log.Debug("Rate limiting by user",
						logger.String("user_id", userID),
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path))
				} else {
					// Если пользователь не авторизован, используем IP
					key = "ip:" + getIP(r)
					log.Debug("Rate limiting by IP (user not authenticated)",
						logger.String("key", key),
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path))
				}
			} else {
				// Используем IP адрес как ключ
				key = "ip:" + getIP(r)
				log.Debug("Rate limiting by IP",
					logger.String("key", key),
					logger.String("method", r.Method),
					logger.String("path", r.URL.Path))
			}

			// Проверяем лимит запросов
			limitExceeded, err := rateLimiter.CheckRateLimit(r.Context(), key, limit, window)
			if err != nil {
				// В случае ошибки Redis разрешаем запрос
				log.Error("Rate limit check failed",
					logger.Error(err),
					logger.String("key", key))
				http.Error(w, "Rate limit service unavailable", http.StatusInternalServerError)
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
			log.Debug("Rate limit check passed",
				logger.String("key", key),
				logger.String("method", r.Method),
				logger.String("path", r.URL.Path))
			next.ServeHTTP(w, r)
		})
	}
}

// getIP извлекает IP адрес из запроса
// Проверяет X-Forwarded-For и X-Real-IP заголовки
// Возвращает REMOTE_ADDR если заголовки отсутствуют
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
