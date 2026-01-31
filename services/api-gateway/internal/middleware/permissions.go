package middleware

import (
	"context"
	"net/http"
	"strings"

	"UptimePingPlatform/pkg/logger"
)

// PermissionMiddleware проверяет права доступа к эндпоинтам
func PermissionMiddleware(requiredPermissions []string, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info("PermissionMiddleware called",
				logger.String("path", r.URL.Path),
				logger.String("method", r.Method))

			// Получаем права пользователя из контекста
			userPermissions := getUserPermissions(r.Context())

			log.Info("Checking permissions",
				logger.String("required", strings.Join(requiredPermissions, ",")),
				logger.String("user_permissions", strings.Join(userPermissions, ",")),
				logger.String("path", r.URL.Path),
				logger.String("method", r.Method))

			// Проверяем наличие всех требуемых прав
			for _, required := range requiredPermissions {
				if !hasPermission(userPermissions, required) {
					log.Warn("Insufficient permissions",
						logger.String("required", required),
						logger.String("user_permissions", strings.Join(userPermissions, ",")),
						logger.String("path", r.URL.Path))

					http.Error(w, "Insufficient permissions", http.StatusForbidden)
					return
				}
			}

			log.Debug("Permissions check passed",
				logger.String("required", strings.Join(requiredPermissions, ",")),
				logger.String("path", r.URL.Path))

			next.ServeHTTP(w, r)
		})
	}
}

// getUserPermissions получает права пользователя из контекста
func getUserPermissions(ctx context.Context) []string {
	if permissions := ctx.Value("permissions"); permissions != nil {
		// Если права уже как []string
		if permSlice, ok := permissions.([]string); ok {
			return permSlice
		}
		// Если права как строка с разделителями
		if permStr, ok := permissions.(string); ok && permStr != "" {
			return strings.Split(permStr, ",")
		}
	}
	return []string{}
}

// hasPermission проверяет наличие конкретного права
func hasPermission(permissions []string, required string) bool {
	for _, permission := range permissions {
		if permission == required {
			return true
		}
		// Проверяем wildcard права
		if strings.HasSuffix(permission, "*") {
			prefix := strings.TrimSuffix(permission, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}

// RequirePermissions создает middleware для требуемых прав
func RequirePermissions(permissions ...string) func(http.Handler) http.Handler {
	return PermissionMiddleware(permissions, nil)
}
