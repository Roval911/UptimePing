package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"UptimePingPlatform/pkg/errors"
	pkglogger "UptimePingPlatform/pkg/logger"
)

// JWTClaims представляет claims из JWT токена
type JWTClaims struct {
	UserID      string   `json:"user_id"`
	TenantID    string   `json:"tenant_id"`
	IsAdmin     bool     `json:"is_admin"`
	Permissions []string `json:"permissions"`
}

// ContextKey для хранения user информации в контексте
type ContextKey string

const (
	UserIDKey   ContextKey = "user_id"
	TenantIDKey ContextKey = "tenant_id"
	ClaimsKey   ContextKey = "claims"
)

// JWTAuthMiddleware создает middleware для JWT аутентификации
func JWTAuthMiddleware(logger pkglogger.Logger, authServiceURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Извлекаем токен из Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, errors.New(errors.ErrUnauthorized, "Authorization header required"))
				return
			}

			// Проверяем формат Bearer token
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
				writeError(w, errors.New(errors.ErrUnauthorized, "Invalid authorization header format"))
				return
			}

			token := tokenParts[1]

			// Валидируем токен через Auth Service
			claims, err := validateToken(token, authServiceURL, logger)
			if err != nil {
				logger.Error("Token validation failed",
					pkglogger.Error(err),
					pkglogger.String("token", token[:min(len(token), 20)]+"..."))
				writeError(w, err)
				return
			}

			// Добавляем информацию о пользователе в контекст
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, TenantIDKey, claims.TenantID)
			ctx = context.WithValue(ctx, ClaimsKey, claims)

			// Продолжаем выполнение с обновленным контекстом
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// validateToken валидирует JWT токен через Auth Service
func validateToken(token, authServiceURL string, logger pkglogger.Logger) (*JWTClaims, error) {
	// Создаем HTTP клиент для запроса к Auth Service
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Формируем запрос к Auth Service
	req, err := http.NewRequest("POST", authServiceURL+"/api/v1/auth/validate", nil)
	if err != nil {
		logger.Error("Failed to create request to Auth Service",
			pkglogger.Error(err),
			pkglogger.String("auth_service_url", authServiceURL))
		return nil, errors.New(errors.ErrInternal, "failed to validate token")
	}

	// Добавляем токен в заголовок
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	// Отправляем запрос
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to call Auth Service",
			pkglogger.Error(err),
			pkglogger.String("auth_service_url", authServiceURL))
		return nil, errors.New(errors.ErrInternal, "failed to validate token")
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		logger.Error("Auth Service returned error",
			pkglogger.Int("status_code", resp.StatusCode),
			pkglogger.String("auth_service_url", authServiceURL))
		return nil, errors.New(errors.ErrUnauthorized, "invalid token")
	}

	// Парсим ответ
	var validateResp struct {
		Valid       bool     `json:"valid"`
		UserID      string   `json:"user_id,omitempty"`
		TenantID    string   `json:"tenant_id,omitempty"`
		IsAdmin     bool     `json:"is_admin,omitempty"`
		Permissions []string `json:"permissions,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&validateResp); err != nil {
		logger.Error("Failed to decode Auth Service response",
			pkglogger.Error(err))
		return nil, errors.New(errors.ErrInternal, "failed to validate token")
	}

	if !validateResp.Valid {
		return nil, errors.New(errors.ErrUnauthorized, "invalid token")
	}

	// Возвращаем claims
	return &JWTClaims{
		UserID:      validateResp.UserID,
		TenantID:    validateResp.TenantID,
		IsAdmin:     validateResp.IsAdmin,
		Permissions: validateResp.Permissions,
	}, nil
}

// GetUserIDFromContext извлекает user_id из контекста
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

// GetTenantIDFromContext извлекает tenant_id из контекста
func GetTenantIDFromContext(ctx context.Context) (string, bool) {
	tenantID, ok := ctx.Value(TenantIDKey).(string)
	return tenantID, ok
}

// GetClaimsFromContext извлекает claims из контекста
func GetClaimsFromContext(ctx context.Context) (*JWTClaims, bool) {
	claims, ok := ctx.Value(ClaimsKey).(*JWTClaims)
	return claims, ok
}

// writeError пишет ошибку в формате JSON
func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	var statusCode int
	if customErr, ok := err.(*errors.Error); ok {
		switch customErr.Code {
		case errors.ErrUnauthorized:
			statusCode = http.StatusUnauthorized
		case errors.ErrForbidden:
			statusCode = http.StatusForbidden
		default:
			statusCode = http.StatusInternalServerError
		}
	} else {
		statusCode = http.StatusInternalServerError
	}

	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"success": false,
		"error":   err.Error(),
		"message": "Authentication failed",
	}

	// Добавляем детали если есть
	if customErr, ok := err.(*errors.Error); ok {
		if customErr.Details != "" {
			response["details"] = customErr.Details
		}
	}

	// Сериализуем в JSON
	jsonBytes, _ := json.Marshal(response)
	w.Write(jsonBytes)
}

// min возвращает минимальное из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
