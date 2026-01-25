package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"UptimePingPlatform/pkg/errors"
)

// AuthMiddleware проверяет аутентификацию запроса
// Поддерживает два типа аутентификации:
// 1. Bearer токены (JWT) - для пользователей
// 2. APIKey - для сервисов
func AuthMiddleware(authClient AuthClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Проверка наличия заголовка Authorization
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header missing", http.StatusUnauthorized)
				return
			}

			// Определение типа аутентификации
			if isBearerToken(authHeader) {
				// Обработка Bearer токена (JWT)
				if err := handleBearerAuth(r, authHeader, authClient); err != nil {
					writeError(w, err)
					return
				}
			} else if isAPIKey(authHeader) {
				// Обработка APIKey
				if err := handleAPIKeyAuth(r, authHeader, authClient); err != nil {
					writeError(w, err)
					return
				}
			} else {
				// Неподдерживаемый тип аутентификации
				http.Error(w, "Unsupported authorization type", http.StatusUnauthorized)
				return
			}

			// Продолжение выполнения
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
func handleBearerAuth(r *http.Request, authHeader string, authClient AuthClient) error {
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
func handleAPIKeyAuth(r *http.Request, authHeader string, authClient AuthClient) error {
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

// writeError записывает ошибку в ответ
func writeError(w http.ResponseWriter, err error) {
	if customErr, ok := err.(*errors.Error); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(customErr.HTTPStatus())
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    customErr.Code,
				"message": customErr.GetUserMessage(),
				"details": customErr.Details,
			},
		})
	} else {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AuthClient интерфейс для клиента аутентификации
// В реальной реализации будет gRPC клиент для auth-service
type AuthClient interface {
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
	ValidateAPIKey(ctx context.Context, key, secret string) (*APIKeyClaims, error)
}

// TokenClaims структура для данных JWT токена
type TokenClaims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	IsAdmin  bool   `json:"is_admin"`
}

// APIKeyClaims структура для данных API ключа
type APIKeyClaims struct {
	TenantID string `json:"tenant_id"`
	KeyID    string `json:"key_id"`
}
