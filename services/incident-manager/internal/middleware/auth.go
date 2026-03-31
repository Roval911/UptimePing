package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

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
	// TODO: Реализовать валидацию через Auth Service
	// Сейчас для демонстрации парсим JWT напрямую (в production использовать Auth Service)

	// Временная реализация - парсим JWT без верификации подписи
	// В реальном проекте здесь должен быть запрос к Auth Service
	claims := &JWTClaims{
		UserID:   "8df32706-16a2-40a6-b829-75ee0a890840", // Временный hardcoded user
		TenantID: "bb9dfee7-60f2-4566-9a6f-77f988195ea6", // Временный hardcoded tenant
		IsAdmin:  true,
		Permissions: []string{
			"incidents:read", "incidents:write", "incidents:resolve",
		},
	}

	return claims, nil
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
