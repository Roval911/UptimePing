package middleware

import (
	"fmt"
	"net/http"

	"UptimePingPlatform/pkg/logger"
)

// CORSMiddleware настраивает CORS заголовки
func CORSMiddleware(allowedOrigins []string, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = "*"
			}

			log.Debug("CORS middleware processing request",
				logger.String("method", r.Method),
				logger.String("path", r.URL.Path),
				logger.String("origin", origin))

			// Проверяем, разрешен ли источник
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || origin == allowedOrigin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				log.Debug("CORS origin allowed", logger.String("origin", origin))
			} else {
				log.Warn("CORS origin not allowed",
					logger.String("origin", origin),
					logger.String("allowed_origins", fmt.Sprintf("%v", allowedOrigins)))
			}

			// Разрешаем определенные методы
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

			// Разрешаем определенные заголовки
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

			// Разрешаем credentials
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			// Обработка preflight запросов
			if r.Method == "OPTIONS" {
				log.Debug("CORS preflight request handled",
					logger.String("method", r.Method),
					logger.String("path", r.URL.Path))
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
