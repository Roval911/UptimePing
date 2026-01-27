package handlers

import (
	"context"
	"testing"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/auth-service/internal/service"
	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"

	grpc_auth "UptimePingPlatform/gen/go/proto/api/auth/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func (m *SimpleMockAuthService) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
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

func createSimpleTestHandler() (*AuthHandler, *SimpleMockAuthService, *jwt.Manager) {
	mockAuthService := &SimpleMockAuthService{}
	mockLogger := &SimpleMockLogger{}
	
	// Создаем JWT менеджер с большим сроком действия для тестов
	jwtManager := jwt.NewManager(
		"test-access-secret-key-12345678901234567890",
		"test-refresh-secret-key-12345678901234567890", 
		86400*7, // 7 дней
		86400*30, // 30 дней
	)

	handler := NewAuthHandler(mockAuthService, jwtManager, mockLogger)
	return handler, mockAuthService, jwtManager
}

func TestAuthHandler_ValidateToken_ValidToken(t *testing.T) {
	// Создаем JWT менеджер с большим сроком действия
	jwtManager := jwt.NewManager(
		"test-access-secret-key-12345678901234567890",
		"test-refresh-secret-key-12345678901234567890", 
		86400*7, // 7 дней
		86400*30, // 30 дней
	)

	// Создаем handler с этим JWT manager
	mockAuthService := &SimpleMockAuthService{}
	mockLogger := &SimpleMockLogger{}
	handler := NewAuthHandler(mockAuthService, jwtManager, mockLogger)

	// Создаем валидный JWT токен
	token, _, err := jwtManager.GenerateToken("user-123", "tenant-456", false)
	assert.NoError(t, err)

	req := &grpc_auth.ValidateTokenRequest{
		Token: token,
	}

	resp, err := handler.ValidateToken(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsValid)
	assert.Equal(t, "user-123", resp.UserId)
	assert.Equal(t, "tenant-456", resp.TenantId)
}

func TestAuthHandler_ValidateToken_InvalidToken(t *testing.T) {
	handler, _, _ := createSimpleTestHandler()

	req := &grpc_auth.ValidateTokenRequest{
		Token: "invalid-token-format",
	}

	resp, err := handler.ValidateToken(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestAuthHandler_ValidateToken_EmptyToken(t *testing.T) {
	handler, _, _ := createSimpleTestHandler()

	req := &grpc_auth.ValidateTokenRequest{
		Token: "",
	}

	resp, err := handler.ValidateToken(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestAuthHandler_ValidateToken_ShortToken(t *testing.T) {
	handler, _, _ := createSimpleTestHandler()

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
