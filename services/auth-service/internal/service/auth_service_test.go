package service_test

import (
	"context"
	"testing"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/pkg/password"
	"UptimePingPlatform/services/auth-service/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRepositories для тестов
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockTenantRepository struct {
	mock.Mock
}

func (m *MockTenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockTenantRepository) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}

func (m *MockTenantRepository) FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}

func (m *MockTenantRepository) Update(ctx context.Context, tenant *domain.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockTenantRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockAPIKeyRepository struct {
	mock.Mock
}

func (m *MockAPIKeyRepository) Create(ctx context.Context, key *domain.APIKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) FindByID(ctx context.Context, id string) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) FindByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) ListByTenant(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) Update(ctx context.Context, key *domain.APIKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockSessionRepository struct {
	mock.Mock
}

func (m *MockSessionRepository) Create(ctx context.Context, session *domain.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockSessionRepository) FindByID(ctx context.Context, id string) (*domain.Session, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Session), args.Error(1)
}

func (m *MockSessionRepository) FindByAccessTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Session), args.Error(1)
}

func (m *MockSessionRepository) FindByRefreshTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Session), args.Error(1)
}

func (m *MockSessionRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockSessionRepository) CleanupExpired(ctx context.Context, before time.Time) error {
	args := m.Called(ctx, before)
	return args.Error(0)
}

func setupAuthService() (service.AuthService, *MockUserRepository, *MockTenantRepository, *MockAPIKeyRepository, *MockSessionRepository) {
	userRepo := &MockUserRepository{}
	tenantRepo := &MockTenantRepository{}
	apiKeyRepo := &MockAPIKeyRepository{}
	sessionRepo := &MockSessionRepository{}

	// Создаем тестовый logger
	testLogger, _ := logger.NewLogger("test", "info", "test-service", false)

	// Используем короткие секретные ключи для тестов
	jwtManager := jwt.NewManager("secret", "refresh", time.Hour, 24*time.Hour)
	passwordHasher := password.NewBcryptHasher(10)

	authService := service.NewAuthService(
		userRepo,
		tenantRepo,
		apiKeyRepo,
		sessionRepo,
		jwtManager,
		passwordHasher,
		testLogger,
	)

	return authService, userRepo, tenantRepo, apiKeyRepo, sessionRepo
}

func TestAuthService_Register_UserExists(t *testing.T) {
	authService, userRepo, _, _, _ := setupAuthService()

	ctx := context.Background()
	email := "existing@example.com"
	password := "Password123!"
	tenantName := "Test Tenant"

	existingUser := &domain.User{
		ID:       "user-1",
		Email:    email,
		IsActive: true,
	}

	// Мокаем существующего пользователя
	userRepo.On("FindByEmail", ctx, email).Return(existingUser, nil)

	// Вызываем метод
	tokenPair, err := authService.Register(ctx, email, password, tenantName)

	// Проверяем результаты
	assert.Error(t, err)
	assert.Nil(t, tokenPair)
	assert.Equal(t, service.ErrConflict, err)

	// Проверяем, что мок был вызван
	userRepo.AssertExpectations(t)
}

func TestAuthService_Login_InactiveUser(t *testing.T) {
	authService, userRepo, _, _, _ := setupAuthService()

	ctx := context.Background()
	email := "inactive@example.com"
	pwd := "Password123!"

	inactiveUser := &domain.User{
		ID:       "user-1",
		Email:    email,
		IsActive: false,
	}

	// Мокаем неактивного пользователя
	userRepo.On("FindByEmail", ctx, email).Return(inactiveUser, nil)

	// Вызываем метод
	tokenPair, err := authService.Login(ctx, email, pwd)

	// Проверяем результаты
	assert.Error(t, err)
	assert.Nil(t, tokenPair)
	assert.Equal(t, service.ErrForbidden, err)

	// Проверяем, что мок был вызван
	userRepo.AssertExpectations(t)
}

func TestAuthService_CreateAPIKey(t *testing.T) {
	authService, _, _, apiKeyRepo, _ := setupAuthService()

	ctx := context.Background()
	tenantID := "tenant-1"
	name := "Test API Key"

	// Мокаем создание API ключа
	apiKeyRepo.On("Create", ctx, mock.AnythingOfType("*domain.APIKey")).Return(nil)

	// Вызываем метод
	apiKeyPair, err := authService.CreateAPIKey(ctx, tenantID, name)

	// Проверяем результаты
	require.NoError(t, err)
	assert.NotEmpty(t, apiKeyPair.Key)
	assert.NotEmpty(t, apiKeyPair.Secret)
	assert.True(t, len(apiKeyPair.Key) > 10)
	assert.True(t, len(apiKeyPair.Secret) > 10)

	// Проверяем, что мок был вызван
	apiKeyRepo.AssertExpectations(t)
}

func TestAuthService_ValidateAPIKey_InvalidSecret(t *testing.T) {
	authService, _, _, apiKeyRepo, _ := setupAuthService()

	ctx := context.Background()
	key := "upk_testkey123"
	secret := "wrongsecret"

	// Создаем хеши
	passwordHasher := password.NewBcryptHasher(10)
	keyHash, _ := passwordHasher.Hash(key)
	secretHash, _ := passwordHasher.Hash("correctsecret")

	existingAPIKey := &domain.APIKey{
		ID:         "key-1",
		TenantID:   "tenant-1",
		KeyHash:    keyHash,
		SecretHash: secretHash,
		Name:       "Test API Key",
		IsActive:   true,
		ExpiresAt:  time.Now().Add(time.Hour), // Не истекший ключ
	}

	// Мокаем поиск по хэшу ключа
	apiKeyRepo.On("FindByKeyHash", ctx, mock.AnythingOfType("string")).Return(existingAPIKey, nil)

	// Вызываем метод
	claims, err := authService.ValidateAPIKey(ctx, key, secret)

	// Проверяем результаты
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Equal(t, service.ErrUnauthorized, err)

	// Проверяем, что мок был вызван
	apiKeyRepo.AssertExpectations(t)
}
