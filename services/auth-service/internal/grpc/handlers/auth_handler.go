package handlers

import (
	"context"
	"fmt"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/auth-service/internal/service"

	grpc_auth "UptimePingPlatform/gen/go/proto/api/auth/v1"
)

// AuthHandler реализация gRPC обработчиков для AuthService
type AuthHandler struct {
	grpc_auth.UnimplementedAuthServiceServer
	authService service.AuthService
	logger      logger.Logger
}

// NewAuthHandler создает новый экземпляр AuthHandler
func NewAuthHandler(authService service.AuthService, logger logger.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

// Register создает нового пользователя и возвращает пару токенов
func (h *AuthHandler) Register(ctx context.Context, req *grpc_auth.RegisterRequest) (*grpc_auth.TokenPair, error) {
	h.logger.Debug("Register request received", logger.String("email", req.Email))

	tokenPair, err := h.authService.Register(ctx, req.Email, req.Password, req.TenantName)
	if err != nil {
		h.logger.Error("Register failed", logger.String("error", err.Error()))
		return nil, h.convertError(err)
	}

	return &grpc_auth.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:   86400, // 24 часа в секундах
	}, nil
}

// Login аутентифицирует пользователя и возвращает пару токенов
func (h *AuthHandler) Login(ctx context.Context, req *grpc_auth.LoginRequest) (*grpc_auth.TokenPair, error) {
	h.logger.Debug("Login request received", logger.String("email", req.Email))

	tokenPair, err := h.authService.Login(ctx, req.Email, req.Password)
	if err != nil {
		h.logger.Error("Login failed", logger.String("error", err.Error()))
		return nil, h.convertError(err)
	}

	return &grpc_auth.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:   86400, // 24 часа в секундах
	}, nil
}

// ValidateToken проверяет валидность JWT токена
func (h *AuthHandler) ValidateToken(ctx context.Context, req *grpc_auth.ValidateTokenRequest) (*grpc_auth.ValidateTokenResponse, error) {
	h.logger.Debug("ValidateToken request received")

	// TODO: Реализовать валидацию токена через JWT manager
	// Сейчас возвращаем заглушку для тестирования
	return &grpc_auth.ValidateTokenResponse{
		UserId:   "test-user-id",
		Email:    "test@example.com",
		TenantId: "test-tenant-id",
		IsValid:  true,
	}, nil
}

// RefreshToken обновляет пару токенов по refresh токену
func (h *AuthHandler) RefreshToken(ctx context.Context, req *grpc_auth.RefreshTokenRequest) (*grpc_auth.TokenPair, error) {
	h.logger.Debug("RefreshToken request received")

	tokenPair, err := h.authService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		h.logger.Error("RefreshToken failed", logger.String("error", err.Error()))
		return nil, h.convertError(err)
	}

	return &grpc_auth.TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:   86400, // 24 часа в секундах
	}, nil
}

// Logout отзывает refresh токен
func (h *AuthHandler) Logout(ctx context.Context, req *grpc_auth.LogoutRequest) (*grpc_auth.LogoutResponse, error) {
	h.logger.Debug("Logout request received", logger.String("user_id", req.UserId))

	err := h.authService.Logout(ctx, req.UserId, req.RefreshToken)
	if err != nil {
		h.logger.Error("Logout failed", logger.String("error", err.Error()))
		return nil, h.convertError(err)
	}

	return &grpc_auth.LogoutResponse{
		Success: true,
	}, nil
}

// CreateAPIKey создает новый API ключ для tenant
func (h *AuthHandler) CreateAPIKey(ctx context.Context, req *grpc_auth.CreateAPIKeyRequest) (*grpc_auth.APIKeyPair, error) {
	h.logger.Debug("CreateAPIKey request received", logger.String("tenant_id", req.TenantId))

	apiKeyPair, err := h.authService.CreateAPIKey(ctx, req.TenantId, req.Name)
	if err != nil {
		h.logger.Error("CreateAPIKey failed", logger.String("error", err.Error()))
		return nil, h.convertError(err)
	}

	return &grpc_auth.APIKeyPair{
		Key:      apiKeyPair.Key,
		Secret:   apiKeyPair.Secret,
		Name:     req.Name,
		TenantId: req.TenantId,
		ExpiresAt: 0, // TODO: добавить срок действия если нужно
	}, nil
}

// ValidateAPIKey проверяет валидность API ключа
func (h *AuthHandler) ValidateAPIKey(ctx context.Context, req *grpc_auth.ValidateAPIKeyRequest) (*grpc_auth.ValidateAPIKeyResponse, error) {
	h.logger.Debug("ValidateAPIKey request received")

	claims, err := h.authService.ValidateAPIKey(ctx, req.Key, req.Secret)
	if err != nil {
		h.logger.Error("ValidateAPIKey failed", logger.String("error", err.Error()))
		return nil, h.convertError(err)
	}

	return &grpc_auth.ValidateAPIKeyResponse{
		TenantId: claims.TenantID,
		KeyId:    claims.KeyID,
		IsValid:  true,
	}, nil
}

// RevokeAPIKey отзывает API ключ
func (h *AuthHandler) RevokeAPIKey(ctx context.Context, req *grpc_auth.RevokeAPIKeyRequest) (*grpc_auth.RevokeAPIKeyResponse, error) {
	h.logger.Debug("RevokeAPIKey request received", logger.String("key_id", req.KeyId))

	err := h.authService.RevokeAPIKey(ctx, req.KeyId)
	if err != nil {
		h.logger.Error("RevokeAPIKey failed", logger.String("error", err.Error()))
		return nil, h.convertError(err)
	}

	return &grpc_auth.RevokeAPIKeyResponse{
		Success: true,
	}, nil
}

// convertError конвертирует ошибки сервиса в gRPC ошибки
func (h *AuthHandler) convertError(err error) error {
	// TODO: Реализовать конвертацию ошибок в gRPC status errors
	// Сейчас возвращаем базовую ошибку
	return fmt.Errorf("auth service error: %w", err)
}
