package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/ratelimit"
	"UptimePingPlatform/pkg/redis"
)

// RateLimitMiddleware middleware для rate limiting запросов
type RateLimitMiddleware struct {
	logger      logger.Logger
	rateLimiter ratelimit.RateLimiter
}

// NewRateLimitMiddleware создает новый middleware для rate limiting
func NewRateLimitMiddleware(redisClient *redis.Client, logger logger.Logger) *RateLimitMiddleware {
	rateLimiter := ratelimit.NewRedisRateLimiter(redisClient.Client)

	return &RateLimitMiddleware{
		logger:      logger,
		rateLimiter: rateLimiter,
	}
}

// RateLimit применяет rate limiting к запросам
func (m *RateLimitMiddleware) RateLimit(requests int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем ключ для rate limiting (IP адрес)
			key := m.getRateLimitKey(r)

			// Проверяем rate limit
			exceeded, err := m.rateLimiter.CheckRateLimit(r.Context(), key, requests, window)
			if err != nil {
				m.logger.Error("Rate limit check failed",
					logger.String("key", key),
					logger.Error(err))
				// При ошибке rate limiting пропускаем запрос
				next.ServeHTTP(w, r)
				return
			}

			if exceeded {
				m.logger.Warn("Rate limit exceeded",
					logger.String("key", key),
					logger.String("path", r.URL.Path),
					logger.String("method", r.Method))

				m.writeRateLimitResponse(w)
				return
			}

			// Добавляем заголовки с информацией о rate limit
			m.addRateLimitHeaders(w, key, requests, window)

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByTenant применяет rate limiting для каждого tenant
func (m *RateLimitMiddleware) RateLimitByTenant(requests int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Получаем tenant ID из контекста (заглушка для демонстрации)
			tenantID := "default" // В реальной реализации здесь будет извлечение из контекста
			if tenantID == "" || tenantID == "default" {
				// Для default tenant применяем IP-based rate limiting
				m.RateLimit(requests, window)(next).ServeHTTP(w, r)
				return
			}

			// Используем tenant ID как ключ для rate limiting
			key := "tenant:" + tenantID

			// Проверяем rate limit
			exceeded, err := m.rateLimiter.CheckRateLimit(r.Context(), key, requests, window)
			if err != nil {
				m.logger.Error("Tenant rate limit check failed",
					logger.String("tenant_id", tenantID),
					logger.Error(err))
				next.ServeHTTP(w, r)
				return
			}

			if exceeded {
				m.logger.Warn("Tenant rate limit exceeded",
					logger.String("tenant_id", tenantID),
					logger.String("path", r.URL.Path),
					logger.String("method", r.Method))

				m.writeRateLimitResponse(w)
				return
			}

			// Добавляем заголовки с информацией о rate limit
			m.addRateLimitHeaders(w, key, requests, window)

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByEndpoint применяет rate limiting для разных эндпоинтов
func (m *RateLimitMiddleware) RateLimitByEndpoint(limits map[string]EndpointLimit) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Определяем лимит для данного эндпоинта
			endpointKey := r.Method + ":" + r.URL.Path
			limit, exists := limits[endpointKey]

			if !exists {
				// Если для эндпоинта нет лимита, применяем default
				limit = limits["default"]
				if !exists {
					// Если нет default, пропускаем запрос
					next.ServeHTTP(w, r)
					return
				}
			}

			// Получаем ключ для rate limiting
			key := m.getRateLimitKey(r) + ":" + endpointKey

			// Проверяем rate limit
			exceeded, err := m.rateLimiter.CheckRateLimit(r.Context(), key, limit.Requests, limit.Window)
			if err != nil {
				m.logger.Error("Endpoint rate limit check failed",
					logger.String("endpoint", endpointKey),
					logger.Error(err))
				next.ServeHTTP(w, r)
				return
			}

			if exceeded {
				m.logger.Warn("Endpoint rate limit exceeded",
					logger.String("endpoint", endpointKey),
					logger.String("key", key))

				m.writeRateLimitResponse(w)
				return
			}

			// Добавляем заголовки с информацией о rate limit
			m.addRateLimitHeaders(w, key, limit.Requests, limit.Window)

			next.ServeHTTP(w, r)
		})
	}
}

// EndpointLimit определяет лимиты для эндпоинта
type EndpointLimit struct {
	Requests int
	Window   time.Duration
}

// getRateLimitKey получает ключ для rate limiting
func (m *RateLimitMiddleware) getRateLimitKey(r *http.Request) string {
	// Используем IP адрес как основной ключ
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	return "ip:" + ip
}

// addRateLimitHeaders добавляет заголовки с информацией о rate limit
func (m *RateLimitMiddleware) addRateLimitHeaders(w http.ResponseWriter, key string, requests int, window time.Duration) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(requests))
	w.Header().Set("X-RateLimit-Window", window.String())
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(requests-1)) // Упрощенно
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(window).Unix(), 10))
}

// writeRateLimitResponse отправляет ответ о превышении rate limit
func (m *RateLimitMiddleware) writeRateLimitResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "60") // 60 секунд
	w.WriteHeader(http.StatusTooManyRequests)

	response := map[string]interface{}{
		"success": false,
		"error":   "Rate limit exceeded",
		"code":    http.StatusTooManyRequests,
		"message": "Too many requests. Please try again later.",
	}

	// JSON сериализация
	jsonData, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte(`{"success": false, "error": "Rate limit exceeded"}`))
		return
	}

	w.Write(jsonData)
}

// DefaultRateLimits возвращает лимиты по умолчанию
func DefaultRateLimits() map[string]EndpointLimit {
	return map[string]EndpointLimit{
		"default": {
			Requests: 100,
			Window:   time.Minute,
		},
		"POST:/api/v1/notification/send": {
			Requests: 10,
			Window:   time.Minute,
		},
		"POST:/api/v1/notification/channels": {
			Requests: 5,
			Window:   time.Minute,
		},
		"GET:/api/v1/notification/channels": {
			Requests: 50,
			Window:   time.Minute,
		},
		"DELETE:/api/v1/notification/channels/*": {
			Requests: 10,
			Window:   time.Minute,
		},
	}
}
