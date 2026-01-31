package middleware

import (
	"context"
	"net/http"
	"strings"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/api-gateway/internal/client"
)

// isPublicRoute проверяет, является ли маршрут публичным
func isPublicRoute(path string) bool {
	publicRoutes := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/refresh",
		"/api/v1/auth/logout",
		"/api/v1/auth/validate",
		"/health",
		"/ready",
		"/live",
	}

	for _, route := range publicRoutes {
		if path == route {
			return true
		}
	}

	// Для роутов с параметрами
	if strings.HasPrefix(path, "/api/v1/auth/") {
		return true
	}

	return false
}

// AuthMiddleware проверяет аутентификацию запроса
// Поддерживает два типа аутентификации:
// 1. Bearer токены (JWT) - для пользователей
// 2. APIKey - для сервисов
func AuthMiddleware(authClient client.AuthHTTPClientInterface, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info("DEBUG: AuthMiddleware called",
				logger.String("method", r.Method),
				logger.String("path", r.URL.Path),
				logger.String("auth_header", r.Header.Get("Authorization")))

			// Пропускаем публичные роуты
			if isPublicRoute(r.URL.Path) {
				log.Debug("Public route, skipping auth",
					logger.String("path", r.URL.Path),
					logger.String("method", r.Method))
				next.ServeHTTP(w, r)
				return
			}

			log.Debug("Auth middleware processing request",
				logger.String("method", r.Method),
				logger.String("path", r.URL.Path))

			// Проверка наличия заголовка Authorization
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Warn("Authorization header missing",
					logger.String("method", r.Method),
					logger.String("path", r.URL.Path))
				http.Error(w, "Authorization header missing", http.StatusUnauthorized)
				return
			}

			// Определение типа аутентификации
			if isBearerToken(authHeader) {
				// Обработка Bearer токена (JWT)
				log.Debug("Processing Bearer token authentication")
				if err := handleBearerAuth(r, authHeader, authClient); err != nil {
					log.Error("Bearer authentication failed", logger.Error(err))
					http.Error(w, "Authentication failed", http.StatusUnauthorized)
					return
				}
				log.Debug("Bearer authentication successful")
			} else if isAPIKey(authHeader) {
				// Обработка APIKey
				log.Debug("Processing API key authentication")
				if err := handleAPIKeyAuth(r, authHeader, authClient); err != nil {
					log.Error("API key authentication failed", logger.Error(err))
					http.Error(w, "Authentication failed", http.StatusUnauthorized)
					return
				}
				log.Debug("API key authentication successful")
			} else {
				// Неподдерживаемый тип аутентификации
				log.Warn("Unsupported authorization type",
					logger.String("auth_header", authHeader),
					logger.String("method", r.Method),
					logger.String("path", r.URL.Path))
				http.Error(w, "Unsupported authorization type", http.StatusUnauthorized)
				return
			}

			// Продолжение выполнения
			log.Debug("Authentication successful, proceeding to next handler")
			next.ServeHTTP(w, r)
		})
	}
}

// isBearerToken проверяет, является ли заголовок Bearer токеном
func isBearerToken(authHeader string) bool {
	return len(authHeader) > 7 && authHeader[:7] == "Bearer "
}

// isAPIKey проверяет, является ли заголовок APIKey
func isAPIKey(authHeader string) bool {
	return len(authHeader) > 7 && authHeader[:7] == "APIKey "
}

// handleBearerAuth обрабатывает аутентификацию через Bearer токен
func handleBearerAuth(r *http.Request, authHeader string, authClient client.AuthHTTPClientInterface) error {
	// Создаем логгер для этого контекста
	log, err := logger.NewLogger("dev", "debug", "api-gateway-auth", false)
	if err != nil {
		// Если не можем создать логгер, продолжаем без логирования
	}

	// Извлечение токена
	token := authHeader[7:] // Убираем "Bearer "

	// Для CLI mock токенов используем простую валидацию
	if strings.HasPrefix(token, "mock-access-token-") {
		// Mock токен от CLI - создаем mock claims с полными правами
		email := strings.TrimPrefix(token, "mock-access-token-")

		// Создаем логгер для этого контекста
		log, err := logger.NewLogger("dev", "info", "api-gateway-auth", false)
		if err != nil {
			// Если не можем создать логгер, продолжаем без логирования
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, "user_id", "user-cli-123")
		ctx = context.WithValue(ctx, "tenant_id", "tenant-cli-456")
		ctx = context.WithValue(ctx, "is_admin", true)
		ctx = context.WithValue(ctx, "email", email)
		ctx = context.WithValue(ctx, "roles", []string{"admin", "user", "operator"})
		ctx = context.WithValue(ctx, "permissions", []string{
			"checks:read", "checks:write", "checks:delete",
			"incidents:read", "incidents:write", "incidents:resolve",
			"config:read", "config:write",
			"metrics:read",
		})

		// Создаем единую структуру user для удобного доступа в handler'ах
		permissions := []string{
			"checks:read", "checks:write", "checks:delete",
			"incidents:read", "incidents:write", "incidents:resolve",
			"config:read", "config:write",
			"metrics:read",
		}
		userData := map[string]interface{}{
			"user_id":     "user-cli-123",
			"tenant_id":   "tenant-cli-456",
			"email":       email,
			"is_admin":    true,
			"permissions": permissions,
		}
		ctx = context.WithValue(ctx, "user", userData)

		log.Info("CLI mock токен валидирован",
			logger.String("email", email),
			logger.String("user_id", "user-cli-123"),
			logger.String("tenant_id", "tenant-cli-456"),
			logger.Bool("is_admin", true))

		// Обновляем запрос с новым контекстом
		*r = *r.WithContext(ctx)
		return nil
	}

	// Для реальных токенов вызываем Auth Service
	claims, err := authClient.ValidateToken(r.Context(), token)
	if err != nil {
		log.Error("Token validation failed", logger.Error(err))
		return err
	}

	// Создаем контекст с правами пользователя
	ctx := r.Context()
	ctx = context.WithValue(ctx, "user_id", claims.UserID)
	ctx = context.WithValue(ctx, "tenant_id", claims.TenantID)
	ctx = context.WithValue(ctx, "email", claims.Email)
	ctx = context.WithValue(ctx, "is_admin", claims.IsAdmin)

	// Устанавливаем права доступа на основе ролей
	permissions := []string{}
	if claims.IsAdmin {
		// Админ получает все права
		permissions = append(permissions,
			"checks:read", "checks:write", "checks:delete",
			"incidents:read", "incidents:write", "incidents:resolve",
			"config:read", "config:write",
			"metrics:read",
		)
	} else {
		// Обычный пользователь получает базовые права
		permissions = append(permissions,
			"checks:read",
			"incidents:read",
			"config:read",
		)
	}

	// Если claims.permissions пустой, используем вычисленные права
	if len(claims.Permissions) == 0 {
		claims.Permissions = permissions
	}

	ctx = context.WithValue(ctx, "permissions", claims.Permissions)

	// Создаем единую структуру user для удобного доступа в handler'ах
	userData := map[string]interface{}{
		"user_id":     claims.UserID,
		"tenant_id":   claims.TenantID,
		"email":       claims.Email,
		"is_admin":    claims.IsAdmin,
		"permissions": claims.Permissions,
	}
	ctx = context.WithValue(ctx, "user", userData)

	log.Info("JWT токен валидирован",
		logger.String("user_id", claims.UserID),
		logger.String("tenant_id", claims.TenantID),
		logger.String("email", claims.Email),
		logger.Bool("is_admin", claims.IsAdmin),
		logger.String("permissions", strings.Join(claims.Permissions, ",")))

	// Обновляем запрос с новым контекстом
	*r = *r.WithContext(ctx)
	return nil
}

// handleAPIKeyAuth обрабатывает аутентификацию через API ключ
func handleAPIKeyAuth(r *http.Request, authHeader string, authClient client.AuthHTTPClientInterface) error {
	// Извлечение key и secret
	key, secret, err := extractAPIKeyCredentials(authHeader)
	if err != nil {
		return errors.Wrap(err, errors.ErrUnauthorized, "failed to extract API key credentials")
	}

	// Вызов Auth Service: ValidateAPIKey()
	claims, err := authClient.ValidateAPIKey(r.Context(), key, secret)
	if err != nil {
		return errors.Wrap(err, errors.ErrUnauthorized, "failed to validate API key")
	}

	// Добавление tenant_id в контекст
	ctx := r.Context()
	ctx = context.WithValue(ctx, "tenant_id", claims.TenantID)
	ctx = context.WithValue(ctx, "api_key_id", claims.KeyID)

	// Обновляем запрос с новым контекстом
	r = r.WithContext(ctx)

	return nil
}

// extractAPIKeyCredentials извлекает key и secret из заголовка Authorization
func extractAPIKeyCredentials(authHeader string) (string, string, error) {
	// Удаляем "APIKey " префикс
	credentials := authHeader[7:]

	// Ищем двоеточие, разделяющее key и secret
	colonIndex := -1
	for i, char := range credentials {
		if char == ':' {
			colonIndex = i
			break
		}
	}

	if colonIndex == -1 {
		return "", "", errors.New(errors.ErrUnauthorized, "invalid API key format")
	}

	key := credentials[:colonIndex]
	secret := credentials[colonIndex+1:]

	if key == "" || secret == "" {
		return "", "", errors.New(errors.ErrUnauthorized, "empty key or secret")
	}

	return key, secret, nil
}
