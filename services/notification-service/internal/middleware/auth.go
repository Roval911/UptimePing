package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
)

// AuthMiddleware middleware для аутентификации запросов
type AuthMiddleware struct {
	logger       logger.Logger
	secretKey    []byte
	expectedIssuer string
}

// JWTClaims содержит claims для JWT токена
type JWTClaims struct {
	TenantID string `json:"tenant_id"`
	Issuer   string `json:"iss"`
	jwt.RegisteredClaims
}

// NewAuthMiddleware создает новый middleware для аутентификации
func NewAuthMiddleware(logger logger.Logger, secretKey string, expectedIssuer string) *AuthMiddleware {
	return &AuthMiddleware{
		logger:        logger,
		secretKey:     []byte(secretKey),
		expectedIssuer: expectedIssuer,
	}
}

// Authenticate проверяет аутентификацию запроса
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.logger.Info("Authenticating request",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path))

		// Извлекаем токен из заголовка Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.logger.Warn("Missing Authorization header")
			m.writeErrorResponse(w, http.StatusUnauthorized, "Missing Authorization header")
			return
		}

		// Проверяем формат Bearer token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			m.logger.Warn("Invalid Authorization header format")
			m.writeErrorResponse(w, http.StatusUnauthorized, "Invalid Authorization header format")
			return
		}

		token := tokenParts[1]

		// Валидация токена
		tenantID, err := m.validateToken(token)
		if err != nil {
			m.logger.Error("Token validation failed", logger.Error(err))
			m.writeErrorResponse(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Добавляем tenant ID в контекст
		ctx := context.WithValue(r.Context(), "tenant_id", tenantID)
		ctx = context.WithValue(ctx, "user_authenticated", true)

		m.logger.Info("Request authenticated successfully",
			logger.String("tenant_id", tenantID))

		// Передаем управление следующему обработчику с обновленным контекстом
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateToken валидирует JWT токен
func (m *AuthMiddleware) validateToken(token string) (string, error) {
	// Парсинг JWT токена
	tokenString := strings.TrimPrefix(token, "Bearer ")
	
	// Парсим токен с claims
	claims := &JWTClaims{}
	parsedToken, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Возвращаем секретный ключ для проверки подписи
		return m.secretKey, nil
	})
	
	if err != nil {
		m.logger.Error("JWT parsing failed", logger.Error(err))
		return "", errors.New(errors.ErrValidation, "invalid token format")
	}
	
	// Проверяем валидность токена
	if !parsedToken.Valid {
		m.logger.Error("Invalid token", logger.String("error", parsedToken.Raw))
		return "", errors.New(errors.ErrValidation, "invalid token")
	}
	
	// Проверяем срок действия
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		m.logger.Warn("Token expired", 
			logger.Duration("expires_at", time.Until(claims.ExpiresAt.Time)))
		return "", errors.New(errors.ErrValidation, "token expired")
	}
	
	// Проверяем issuer
	if claims.Issuer != m.expectedIssuer {
		m.logger.Error("Invalid token issuer",
			logger.String("expected", m.expectedIssuer),
			logger.String("actual", claims.Issuer))
		return "", errors.New(errors.ErrValidation, "invalid token issuer")
	}
	
	// Проверяем наличие tenant ID
	if claims.TenantID == "" {
		m.logger.Error("Missing tenant ID in token")
		return "", errors.New(errors.ErrValidation, "missing tenant ID")
	}
	
	m.logger.Info("Token validated successfully",
		logger.String("tenant_id", claims.TenantID),
		logger.String("issuer", claims.Issuer))
	
	return claims.TenantID, nil
}

// writeErrorResponse отправляет ошибку в формате JSON
func (m *AuthMiddleware) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"success": false,
		"error":   message,
		"code":    statusCode,
	}

	// JSON сериализация
	jsonData, err := json.Marshal(response)
	if err != nil {
		// Если не удалось сериализовать, отправляем простой текст
		w.Write([]byte(`{"success": false, "error": "internal server error"}`))
		return
	}
	
	w.Write(jsonData)
}

// OptionalAuthentication middleware для опциональной аутентификации
func (m *AuthMiddleware) OptionalAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if authHeader != "" {
			// Если заголовок есть, выполняем полную аутентификацию
			m.Authenticate(next).ServeHTTP(w, r)
		} else {
			// Если заголовка нет, пропускаем без аутентификации
			ctx := context.WithValue(r.Context(), "user_authenticated", false)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// RequireTenant middleware для обязательного наличия tenant ID
func (m *AuthMiddleware) RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value("tenant_id")
		if tenantID == nil {
			m.logger.Warn("Missing tenant ID in context")
			m.writeErrorResponse(w, http.StatusForbidden, "Tenant ID required")
			return
		}

		next.ServeHTTP(w, r)
	})
}
