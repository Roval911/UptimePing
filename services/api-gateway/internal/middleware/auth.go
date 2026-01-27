package middleware

import (
	"context"
	"net/http"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/api-gateway/internal/client"
)

// AuthMiddleware проверяет аутентификацию запроса
// Поддерживает два типа аутентификации:
// 1. Bearer токены (JWT) - для пользователей
// 2. APIKey - для сервисов
func AuthMiddleware(authClient *client.GRPCAuthClient, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
func handleBearerAuth(r *http.Request, authHeader string, authClient *client.GRPCAuthClient) error {
	// Извлечение токена
	token := authHeader[7:] // Убираем "Bearer "

	// Вызов Auth Service: ValidateToken()
	claims, err := authClient.ValidateToken(r.Context(), token)
	if err != nil {
		return errors.Wrap(err, errors.ErrUnauthorized, "failed to validate token")
	}

	// Добавление user_id, tenant_id в контекст
	ctx := r.Context()
	ctx = context.WithValue(ctx, "user_id", claims.UserID)
	ctx = context.WithValue(ctx, "tenant_id", claims.TenantID)
	ctx = context.WithValue(ctx, "is_admin", claims.IsAdmin)

	// Обновляем запрос с новым контекстом
	r = r.WithContext(ctx)

	return nil
}

// handleAPIKeyAuth обрабатывает аутентификацию через API ключ
func handleAPIKeyAuth(r *http.Request, authHeader string, authClient *client.GRPCAuthClient) error {
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
