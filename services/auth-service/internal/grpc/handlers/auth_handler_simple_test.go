package handlers

import (
	"context"
	"testing"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/auth-service/internal/service"
	"UptimePingPlatform/services/auth-service/internal/domain"
	jwtPkg "UptimePingPlatform/services/auth-service/internal/pkg/jwt"

	grpc_auth "UptimePingPlatform/proto/api/auth/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/golang-jwt/jwt/v5"
)

// SimpleMockAuthService мок для AuthService
type SimpleMockAuthService struct {
	mock.Mock
}

func (m *SimpleMockAuthService) Login(ctx context.Context, email, password string) (*service.TokenPair, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.TokenPair), args.Error(1)
}

func (m *SimpleMockAuthService) Register(ctx context.Context, email, password, tenantName string) (*service.TokenPair, error) {
	args := m.Called(ctx, email, password, tenantName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.TokenPair), args.Error(1)
}

func (m *SimpleMockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*service.TokenPair, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.TokenPair), args.Error(1)
}

func (m *SimpleMockAuthService) Logout(ctx context.Context, userID, tokenID string) error {
	args := m.Called(ctx, userID, tokenID)
	return args.Error(0)
}

func (m *SimpleMockAuthService) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *SimpleMockAuthService) CreateAPIKey(ctx context.Context, tenantID, name string) (*service.APIKeyPair, error) {
	args := m.Called(ctx, tenantID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.APIKeyPair), args.Error(1)
}

func (m *SimpleMockAuthService) ValidateAPIKey(ctx context.Context, key, secret string) (*service.Claims, error) {
	args := m.Called(ctx, key, secret)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.Claims), args.Error(1)
}

func (m *SimpleMockAuthService) RevokeAPIKey(ctx context.Context, keyID string) error {
	args := m.Called(ctx, keyID)
	return args.Error(0)
}

// SimpleMockLogger мок для логгера без testify/mock
type SimpleMockLogger struct{}

func (m *SimpleMockLogger) Debug(msg string, fields ...logger.Field) {}
func (m *SimpleMockLogger) Info(msg string, fields ...logger.Field)  {}
func (m *SimpleMockLogger) Warn(msg string, fields ...logger.Field)  {}
func (m *SimpleMockLogger) Error(msg string, fields ...logger.Field) {}
func (m *SimpleMockLogger) With(fields ...logger.Field) logger.Logger {
	return m
}
func (m *SimpleMockLogger) Sync() error {
	return nil
}

// MockJWTManager мок для JWT менеджера
type MockJWTManager struct {
	mock.Mock
}

func (m *MockJWTManager) GenerateToken(userID, tenantID string, isAdmin bool) (string, string, error) {
	args := m.Called(userID, tenantID, isAdmin)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockJWTManager) GenerateAccessToken(userID, tenantID string, isAdmin bool) (string, error) {
	args := m.Called(userID, tenantID, isAdmin)
	return args.String(0), args.Error(1)
}

func (m *MockJWTManager) GenerateRefreshToken(userID, tenantID string, isAdmin bool) (string, error) {
	args := m.Called(userID, tenantID, isAdmin)
	return args.String(0), args.Error(1)
}

func (m *MockJWTManager) ValidateAccessToken(token string) (*jwtPkg.TokenClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwtPkg.TokenClaims), args.Error(1)
}

func (m *MockJWTManager) ValidateRefreshToken(token string) (*jwtPkg.TokenClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwtPkg.TokenClaims), args.Error(1)
}

func createSimpleTestHandler() (*AuthHandler, *SimpleMockAuthService, *MockJWTManager) {
	mockAuthService := &SimpleMockAuthService{}
	mockLogger := &SimpleMockLogger{}
	mockJWTManager := &MockJWTManager{}
	
	handler := NewAuthHandler(mockAuthService, mockJWTManager, mockLogger)
	return handler, mockAuthService, mockJWTManager
}

func TestAuthHandler_ValidateToken_ValidToken(t *testing.T) {
	handler, mockAuthService, jwtManager := createSimpleTestHandler()
	
	// Настраиваем мок для возврата пользователя
	mockAuthService.On("GetUserByID", mock.Anything, "user-123").Return(&domain.User{
		ID:        "user-123",
		Email:     "test@example.com",
		TenantID:  "tenant-456",
		CreatedAt: time.Now(),
	}, nil)

	// Настраиваем мок для JWT валидации
	validClaims := &jwtPkg.TokenClaims{
		UserID:    "user-123",
		TenantID:  "tenant-456",
		IsAdmin:   false,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   "user-123",
		},
	}
	
	jwtManager.On("ValidateAccessToken", "valid-token").Return(validClaims, nil)

	req := &grpc_auth.ValidateTokenRequest{
		Token: "valid-token",
	}

	resp, err := handler.ValidateToken(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsValid)
	assert.Equal(t, "user-123", resp.UserId)
	assert.Equal(t, "tenant-456", resp.TenantId)
}

func TestAuthHandler_ValidateToken_InvalidToken(t *testing.T) {
	handler, _, jwtManager := createSimpleTestHandler()

	// Настраиваем мок для возврата ошибки
	jwtManager.On("ValidateAccessToken", "invalid-token-format").Return(nil, assert.AnError)

	req := &grpc_auth.ValidateTokenRequest{
		Token: "invalid-token-format",
	}

	resp, err := handler.ValidateToken(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestAuthHandler_ValidateToken_EmptyToken(t *testing.T) {
	handler, _, jwtManager := createSimpleTestHandler()

	// Настраиваем мок для возврата ошибки
	jwtManager.On("ValidateAccessToken", "").Return(nil, assert.AnError)

	req := &grpc_auth.ValidateTokenRequest{
		Token: "",
	}

	resp, err := handler.ValidateToken(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestAuthHandler_ValidateToken_ShortToken(t *testing.T) {
	handler, _, jwtManager := createSimpleTestHandler()

	// Настраиваем мок для возврата ошибки
	jwtManager.On("ValidateAccessToken", "short").Return(nil, assert.AnError)

	req := &grpc_auth.ValidateTokenRequest{
		Token: "short",
	}

	resp, err := handler.ValidateToken(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestAuthHandler_Login_Success(t *testing.T) {
	handler, mockAuthService, _ := createSimpleTestHandler()

	expectedTokenPair := &service.TokenPair{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
	}

	mockAuthService.On("Login", mock.Anything, "test@example.com", "password123").
		Return(expectedTokenPair, nil)

	req := &grpc_auth.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	resp, err := handler.Login(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "access-token-123", resp.AccessToken)
	assert.Equal(t, "refresh-token-456", resp.RefreshToken)
	assert.Equal(t, int64(86400), resp.ExpiresIn)

	mockAuthService.AssertExpectations(t)
}

func TestAuthHandler_Register_Success(t *testing.T) {
	handler, mockAuthService, _ := createSimpleTestHandler()

	expectedTokenPair := &service.TokenPair{
		AccessToken:  "access-token-789",
		RefreshToken: "refresh-token-012",
	}

	mockAuthService.On("Register", mock.Anything, "test@example.com", "password123", "TestTenant").
		Return(expectedTokenPair, nil)

	req := &grpc_auth.RegisterRequest{
		Email:      "test@example.com",
		Password:    "password123",
		TenantName: "TestTenant",
	}

	resp, err := handler.Register(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "access-token-789", resp.AccessToken)
	assert.Equal(t, "refresh-token-012", resp.RefreshToken)

	mockAuthService.AssertExpectations(t)
}

func TestAuthHandler_ConvertError_CustomError(t *testing.T) {
	handler, _, _ := createSimpleTestHandler()

	err := handler.convertError(service.ErrNotFound)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestAuthHandler_ConvertError_Nil(t *testing.T) {
	handler, _, _ := createSimpleTestHandler()

	err := handler.convertError(nil)
	assert.NoError(t, err)
}

func TestAuthHandler_ConvertError_ContextCanceled(t *testing.T) {
	handler, _, _ := createSimpleTestHandler()

	err := handler.convertError(context.Canceled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request canceled")
}

func TestAuthHandler_Logout_Success(t *testing.T) {
	handler, mockAuthService, _ := createSimpleTestHandler()

	mockAuthService.On("Logout", mock.Anything, "user-123", "refresh-token").
		Return(nil)

	req := &grpc_auth.LogoutRequest{
		UserId:       "user-123",
		RefreshToken: "refresh-token",
	}

	resp, err := handler.Logout(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)

	mockAuthService.AssertExpectations(t)
}

func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	handler, mockAuthService, _ := createSimpleTestHandler()

	expectedTokenPair := &service.TokenPair{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
	}

	mockAuthService.On("RefreshToken", mock.Anything, "old-refresh-token").
		Return(expectedTokenPair, nil)

	req := &grpc_auth.RefreshTokenRequest{
		RefreshToken: "old-refresh-token",
	}

	resp, err := handler.RefreshToken(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "new-access-token", resp.AccessToken)
	assert.Equal(t, "new-refresh-token", resp.RefreshToken)

	mockAuthService.AssertExpectations(t)
}

func TestAuthHandler_CreateAPIKey_Success(t *testing.T) {
	handler, mockAuthService, _ := createSimpleTestHandler()

	expectedAPIKeyPair := &service.APIKeyPair{
		Key:    "test-api-key",
		Secret: "test-secret",
	}

	mockAuthService.On("CreateAPIKey", mock.Anything, "tenant-123", "TestKey").
		Return(expectedAPIKeyPair, nil)

	req := &grpc_auth.CreateAPIKeyRequest{
		TenantId: "tenant-123",
		Name:     "TestKey",
	}

	resp, err := handler.CreateAPIKey(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-api-key", resp.Key)
	assert.Equal(t, "test-secret", resp.Secret)
	assert.Equal(t, "TestKey", resp.Name)
	assert.Equal(t, "tenant-123", resp.TenantId)

	mockAuthService.AssertExpectations(t)
}

func TestAuthHandler_ValidateAPIKey_Success(t *testing.T) {
	handler, mockAuthService, _ := createSimpleTestHandler()

	expectedClaims := &service.Claims{
		TenantID: "tenant-123",
		KeyID:    "key-456",
	}

	mockAuthService.On("ValidateAPIKey", mock.Anything, "test-key", "test-secret").
		Return(expectedClaims, nil)

	req := &grpc_auth.ValidateAPIKeyRequest{
		Key:    "test-key",
		Secret: "test-secret",
	}

	resp, err := handler.ValidateAPIKey(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "tenant-123", resp.TenantId)
	assert.Equal(t, "key-456", resp.KeyId)
	assert.True(t, resp.IsValid)

	mockAuthService.AssertExpectations(t)
}

func TestAuthHandler_RevokeAPIKey_Success(t *testing.T) {
	handler, mockAuthService, _ := createSimpleTestHandler()

	mockAuthService.On("RevokeAPIKey", mock.Anything, "key-123").
		Return(nil)

	req := &grpc_auth.RevokeAPIKeyRequest{
		KeyId: "key-123",
	}

	resp, err := handler.RevokeAPIKey(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)

	mockAuthService.AssertExpectations(t)
}
