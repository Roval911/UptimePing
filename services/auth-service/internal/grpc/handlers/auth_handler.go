package handlers

import (
	"context"
	"fmt"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	pkgErrors "UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/auth-service/internal/service"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"

	grpc_auth "UptimePingPlatform/gen/proto/api/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthHandler реализация gRPC обработчиков для AuthService
type AuthHandler struct {
	*grpcBase.BaseHandler
	grpc_auth.UnimplementedAuthServiceServer
	authService service.AuthService
	jwtManager  jwt.JWTManager
	validator   *validation.Validator
}

// NewAuthHandler создает новый экземпляр AuthHandler
func NewAuthHandler(authService service.AuthService, jwtManager jwt.JWTManager, logger logger.Logger) *AuthHandler {
	return &AuthHandler{
		BaseHandler: grpcBase.NewBaseHandler(logger),
		authService: authService,
		jwtManager:  jwtManager,
		validator:   validation.NewValidator(),
	}
}

// Register создает нового пользователя и возвращает пару токенов
func (h *AuthHandler) Register(ctx context.Context, req *grpc_auth.RegisterRequest) (*grpc_auth.TokenPair, error) {
	h.LogOperationStart(ctx, "Register", map[string]interface{}{
		"email":      req.Email,
		"tenant_name": req.TenantName,
	})

	// Валидация входных данных с использованием pkg/validation
	requiredFields := map[string]interface{}{
		"email":       req.Email,
		"password":    req.Password,
		"tenant_name": req.TenantName,
	}
	
	if err := h.validator.ValidateRequiredFields(requiredFields, map[string]string{
		"email":       "Email address",
		"password":    "Password",
		"tenant_name": "Tenant name",
	}); err != nil {
		return nil, h.LogError(ctx, err, "Register", "validation failed")
	}

	// Валидация длины полей
	if err := h.validator.ValidateStringLength(req.Email, "email", 5, 100); err != nil {
		return nil, h.LogError(ctx, err, "Register", "invalid email length")
	}

	if err := h.validator.ValidateStringLength(req.Password, "password", 8, 128); err != nil {
		return nil, h.LogError(ctx, err, "Register", "invalid password length")
	}

	if err := h.validator.ValidateStringLength(req.TenantName, "tenant_name", 2, 50); err != nil {
		return nil, h.LogError(ctx, err, "Register", "invalid tenant name length")
	}

	tokenPair, err := h.authService.Register(ctx, req.Email, req.Password, req.TenantName)
	if err != nil {
		return nil, h.convertError(err)
	}

	h.LogOperationSuccess(ctx, "Register", map[string]interface{}{
		"access_token": tokenPair.AccessToken,
	})

	return &grpc_auth.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:   86400, // 24 часа в секундах
	}, nil
}

// Login аутентифицирует пользователя и возвращает пару токенов
func (h *AuthHandler) Login(ctx context.Context, req *grpc_auth.LoginRequest) (*grpc_auth.TokenPair, error) {
	h.LogOperationStart(ctx, "Login", map[string]interface{}{
		"email": req.Email,
	})

	tokenPair, err := h.authService.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, h.convertError(err)
	}

	h.LogOperationSuccess(ctx, "Login", map[string]interface{}{
		"access_token": tokenPair.AccessToken,
	})

	return &grpc_auth.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:   86400, // 24 часа в секундах
	}, nil
}

// ValidateToken проверяет валидность JWT токена
func (h *AuthHandler) ValidateToken(ctx context.Context, req *grpc_auth.ValidateTokenRequest) (*grpc_auth.ValidateTokenResponse, error) {
	h.LogOperationStart(ctx, "ValidateToken", map[string]interface{}{
		"token": req.Token,
	})

	// Валидация входных данных
	if err := h.validator.ValidateRequiredFields(map[string]interface{}{
		"token": req.Token,
	}, map[string]string{
		"token": "JWT token",
	}); err != nil {
		return nil, h.LogError(ctx, err, "ValidateToken", "validation failed")
	}

	// Валидация длины токена
	if err := h.validator.ValidateStringLength(req.Token, "token", 10, 1000); err != nil {
		return nil, h.LogError(ctx, err, "ValidateToken", "invalid token length")
	}

	// Валидация JWT токена через JWT manager
	claims, err := h.jwtManager.ValidateAccessToken(req.Token)
	if err != nil {
		h.LogOperationSuccess(ctx, "ValidateToken", map[string]interface{}{
			"is_valid": false,
			"error":    err.Error(),
		})
		
		return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("invalid token: %v", err))
	}

	// Получаем email пользователя из базы данных
	user, err := h.authService.GetUserByID(ctx, claims.UserID)
	if err != nil {
		h.LogOperationSuccess(ctx, "ValidateToken", map[string]interface{}{
			"is_valid": false,
			"error":    err.Error(),
		})
		
		return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("user not found: %v", err))
	}

	h.LogOperationSuccess(ctx, "ValidateToken", map[string]interface{}{
		"is_valid":  true,
		"user_id":   claims.UserID,
		"tenant_id": claims.TenantID,
		"email":     user.Email,
		"is_admin":  claims.IsAdmin,
	})

	return &grpc_auth.ValidateTokenResponse{
		UserId:   claims.UserID,
		Email:    user.Email,
		TenantId: claims.TenantID,
		IsValid:  true,
	}, nil
}

// RefreshToken обновляет пару токенов по refresh токену
func (h *AuthHandler) RefreshToken(ctx context.Context, req *grpc_auth.RefreshTokenRequest) (*grpc_auth.TokenPair, error) {
	h.LogOperationStart(ctx, "RefreshToken", map[string]interface{}{
		"refresh_token": req.RefreshToken,
	})

	tokenPair, err := h.authService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, h.convertError(err)
	}

	h.LogOperationSuccess(ctx, "RefreshToken", map[string]interface{}{
		"access_token": tokenPair.AccessToken,
	})

	return &grpc_auth.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:   86400, // 24 часа в секундах
	}, nil
}

// Logout отзывает refresh токен
func (h *AuthHandler) Logout(ctx context.Context, req *grpc_auth.LogoutRequest) (*grpc_auth.LogoutResponse, error) {
	h.LogOperationStart(ctx, "Logout", map[string]interface{}{
		"user_id": req.UserId,
		"refresh_token": req.RefreshToken,
	})

	err := h.authService.Logout(ctx, req.UserId, req.RefreshToken)
	if err != nil {
		return nil, h.convertError(err)
	}

	h.LogOperationSuccess(ctx, "Logout", map[string]interface{}{
		"user_id": req.UserId,
	})

	return &grpc_auth.LogoutResponse{
		Success: true,
	}, nil
}

// CreateAPIKey создает новый API ключ для tenant
func (h *AuthHandler) CreateAPIKey(ctx context.Context, req *grpc_auth.CreateAPIKeyRequest) (*grpc_auth.APIKeyPair, error) {
	h.LogOperationStart(ctx, "CreateAPIKey", map[string]interface{}{
		"tenant_id": req.TenantId,
		"name": req.Name,
	})

	apiKeyPair, err := h.authService.CreateAPIKey(ctx, req.TenantId, req.Name)
	if err != nil {
		return nil, h.convertError(err)
	}

	h.LogOperationSuccess(ctx, "CreateAPIKey", map[string]interface{}{
		"tenant_id": req.TenantId,
		"key": apiKeyPair.Key,
	})

	return &grpc_auth.APIKeyPair{
		Key:      apiKeyPair.Key,
		Secret:   apiKeyPair.Secret,
		Name:     req.Name,
		TenantId: req.TenantId,
		ExpiresAt: 0, // API ключи не имеют срока действия в текущей реализации
	}, nil
}

// ValidateAPIKey проверяет валидность API ключа
func (h *AuthHandler) ValidateAPIKey(ctx context.Context, req *grpc_auth.ValidateAPIKeyRequest) (*grpc_auth.ValidateAPIKeyResponse, error) {
	h.LogOperationStart(ctx, "ValidateAPIKey", map[string]interface{}{
		"key": req.Key,
	})

	claims, err := h.authService.ValidateAPIKey(ctx, req.Key, req.Secret)
	if err != nil {
		return nil, h.convertError(err)
	}

	h.LogOperationSuccess(ctx, "ValidateAPIKey", map[string]interface{}{
		"tenant_id": claims.TenantID,
		"key_id": claims.KeyID,
	})

	return &grpc_auth.ValidateAPIKeyResponse{
		TenantId: claims.TenantID,
		KeyId:    claims.KeyID,
		IsValid:  true,
	}, nil
}

// RevokeAPIKey отзывает API ключ
func (h *AuthHandler) RevokeAPIKey(ctx context.Context, req *grpc_auth.RevokeAPIKeyRequest) (*grpc_auth.RevokeAPIKeyResponse, error) {
	h.LogOperationStart(ctx, "RevokeAPIKey", map[string]interface{}{
		"key_id": req.KeyId,
	})

	err := h.authService.RevokeAPIKey(ctx, req.KeyId)
	if err != nil {
		return nil, h.convertError(err)
	}

	h.LogOperationSuccess(ctx, "RevokeAPIKey", map[string]interface{}{
		"key_id": req.KeyId,
	})

	return &grpc_auth.RevokeAPIKeyResponse{
		Success: true,
	}, nil
}

// convertError конвертирует ошибки сервиса в gRPC ошибки
func (h *AuthHandler) convertError(err error) error {
	if err == nil {
		return nil
	}

	// Используем pkg/errors для получения кода ошибки
	if customErr, ok := err.(*pkgErrors.Error); ok {
		switch customErr.Code {
		case pkgErrors.ErrValidation:
			return status.Error(codes.InvalidArgument, customErr.Message)
		case pkgErrors.ErrUnauthorized:
			return status.Error(codes.Unauthenticated, customErr.Message)
		case pkgErrors.ErrForbidden:
			return status.Error(codes.PermissionDenied, customErr.Message)
		case pkgErrors.ErrNotFound:
			return status.Error(codes.NotFound, customErr.Message)
		case pkgErrors.ErrConflict:
			return status.Error(codes.AlreadyExists, customErr.Message)
		case pkgErrors.ErrInternal:
			return status.Error(codes.Internal, customErr.Message)
		default:
			return status.Error(codes.Internal, customErr.Message)
		}
	}

	// Для стандартных ошибок Go
	switch {
	case err == context.Canceled:
		return status.Error(codes.Canceled, "request canceled")
	case err == context.DeadlineExceeded:
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	default:
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}
