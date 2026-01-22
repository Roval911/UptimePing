package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/service"
)

// MockUserRepository мок для UserRepository
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

// MockTenantRepository мок для TenantRepository
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

// MockSessionRepository мок для SessionRepository
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

func (m *MockSessionRepository) FindByAccessTokenHash(ctx context.Context, accessTokenHash string) (*domain.Session, error) {
	args := m.Called(ctx, accessTokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Session), args.Error(1)
}

func (m *MockSessionRepository) FindByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (*domain.Session, error) {
	args := m.Called(ctx, refreshTokenHash)
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

// MockJWTManager мок для JWTManager
type MockJWTManager struct {
	mock.Mock
}

func (m *MockJWTManager) GenerateToken(userID, tenantID string, isAdmin bool) (string, string, error) {
	args := m.Called(userID, tenantID, isAdmin)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockJWTManager) ValidateAccessToken(token string) (*jwt.TokenClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenClaims), args.Error(1)
}

func (m *MockJWTManager) ValidateRefreshToken(token string) (*jwt.TokenClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenClaims), args.Error(1)
}

func (m *MockJWTManager) GenerateAccessToken(userID, tenantID string, isAdmin bool) (string, error) {
	args := m.Called(userID, tenantID, isAdmin)
	return args.String(0), args.Error(1)
}

func (m *MockJWTManager) GenerateRefreshToken(userID, tenantID string, isAdmin bool) (string, error) {
	args := m.Called(userID, tenantID, isAdmin)
	return args.String(0), args.Error(1)
}

// MockPasswordHasher мок для Hasher
type MockPasswordHasher struct {
	mock.Mock
}

func (m *MockPasswordHasher) Hash(password string) (string, error) {
	args := m.Called(password)
	return args.String(0), args.Error(1)
}

func (m *MockPasswordHasher) Check(password, hash string) bool {
	args := m.Called(password, hash)
	return args.Bool(0)
}

func (m *MockPasswordHasher) Validate(password string) bool {
	args := m.Called(password)
	return args.Bool(0)
}

func TestAuthService_Register_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockTenantRepo := new(MockTenantRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockJWTManager := new(MockJWTManager)
	mockPasswordHasher := new(MockPasswordHasher)

	// Создаем сервис
	authService := service.NewAuthService(
		mockUserRepo,
		mockTenantRepo,
		mockSessionRepo,
		mockJWTManager,
		mockPasswordHasher,
	)

	// Подготовка данных
	ctx := context.Background()
	email := "test@example.com"
	password := "SecurePassword123"
	tenantName := "Test Tenant"

	// Моки
	// Порядок вызовов в методе Register:
	// 1. Validate(password)
	// 2. FindByEmail(email)
	// 3. FindBySlug(slug)
	// 4. Create(tenant)
	// 5. Hash(password)
	// 6. Create(user)
	// 7. GenerateToken(...)
	// 8. Hash(accessToken), Hash(refreshToken)
	// 9. Create(session)

	mockPasswordHasher.On("Validate", password).Return(true)
	mockUserRepo.On("FindByEmail", ctx, email).Return((*domain.User)(nil), errors.New("user not found"))

	tenantSlug := "test-tenant"
	mockTenantRepo.On("FindBySlug", ctx, tenantSlug).Return((*domain.Tenant)(nil), errors.New("tenant not found"))

	mockTenantRepo.On("Create", ctx, mock.MatchedBy(func(tenant *domain.Tenant) bool {
		return tenant.Name == tenantName &&
			tenant.Slug == tenantSlug &&
			len(tenant.ID) > 0 &&
			tenant.Settings != nil &&
			!tenant.CreatedAt.IsZero() &&
			!tenant.UpdatedAt.IsZero()
	})).Return(nil)

	passwordHash := "hashed_password"
	mockPasswordHasher.On("Hash", password).Return(passwordHash, nil)

	mockUserRepo.On("Create", ctx, mock.MatchedBy(func(user *domain.User) bool {
		return user.Email == email &&
			user.PasswordHash == passwordHash &&
			len(user.ID) > 0 &&
			user.IsActive == true &&
			user.IsAdmin == true &&
			!user.CreatedAt.IsZero() &&
			!user.UpdatedAt.IsZero()
	})).Return(nil)

	// Предполагаем, что ID пользователя и тенанта будут сгенерированы
	accessToken := "access_token"
	refreshToken := "refresh_token"
	mockJWTManager.On("GenerateToken", mock.AnythingOfType("string"), mock.AnythingOfType("string"), true).
		Return(accessToken, refreshToken, nil)

	accessTokenHash := "hashed_access_token"
	refreshTokenHash := "hashed_refresh_token"
	mockPasswordHasher.On("Hash", accessToken).Return(accessTokenHash, nil)
	mockPasswordHasher.On("Hash", refreshToken).Return(refreshTokenHash, nil)

	mockSessionRepo.On("Create", ctx, mock.MatchedBy(func(s *domain.Session) bool {
		return len(s.ID) > 0 &&
			len(s.UserID) > 0 &&
			s.AccessTokenHash == accessTokenHash &&
			s.RefreshTokenHash == refreshTokenHash &&
			!s.ExpiresAt.IsZero() &&
			!s.CreatedAt.IsZero()
	})).Return(nil)

	// Act
	result, err := authService.Register(ctx, email, password, tenantName)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, accessToken, result.AccessToken)
	assert.Equal(t, refreshToken, result.RefreshToken)

	// Проверяем, что все моки были вызваны
	mockUserRepo.AssertExpectations(t)
	mockTenantRepo.AssertExpectations(t)
	mockPasswordHasher.AssertExpectations(t)
	mockJWTManager.AssertExpectations(t)
	mockSessionRepo.AssertExpectations(t)
}

func TestAuthService_Register_UserExists(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockTenantRepo := new(MockTenantRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockJWTManager := new(MockJWTManager)
	mockPasswordHasher := new(MockPasswordHasher)

	// Создаем сервис
	authService := service.NewAuthService(
		mockUserRepo,
		mockTenantRepo,
		mockSessionRepo,
		mockJWTManager,
		mockPasswordHasher,
	)

	// Подготовка данных
	ctx := context.Background()
	email := "test@example.com"
	password := "SecurePassword123"
	tenantName := "Test Tenant"

	// Моки
	// Важно: Validate вызывается ДО FindByEmail в методе Register
	mockPasswordHasher.On("Validate", password).Return(true)

	existingUser := &domain.User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: "hash",
		TenantID:     uuid.New().String(),
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	mockUserRepo.On("FindByEmail", ctx, email).Return(existingUser, nil)

	// Act
	result, err := authService.Register(ctx, email, password, tenantName)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, service.ErrConflict, err)

	// Проверяем, что вызов FindByEmail произошел
	mockUserRepo.AssertExpectations(t)
	mockPasswordHasher.AssertExpectations(t)
	// Другие репозитории не должны быть вызваны
	mockTenantRepo.AssertNotCalled(t, "Create")
	mockPasswordHasher.AssertNotCalled(t, "Hash", password) // Не должен вызывать Hash для пароля
	mockJWTManager.AssertNotCalled(t, "GenerateToken")
	mockSessionRepo.AssertNotCalled(t, "Create")
}

func TestAuthService_Register_PasswordValidationFailed(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockTenantRepo := new(MockTenantRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockJWTManager := new(MockJWTManager)
	mockPasswordHasher := new(MockPasswordHasher)

	// Создаем сервис
	authService := service.NewAuthService(
		mockUserRepo,
		mockTenantRepo,
		mockSessionRepo,
		mockJWTManager,
		mockPasswordHasher,
	)

	// Подготовка данных
	ctx := context.Background()
	email := "test@example.com"
	password := "weak"
	tenantName := "Test Tenant"

	// Моки
	// Только валидация пароля, FindByEmail не должен вызываться
	mockPasswordHasher.On("Validate", password).Return(false)

	// Act
	result, err := authService.Register(ctx, email, password, tenantName)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "password does not meet complexity requirements")

	// Проверяем, что вызов Validate произошел
	mockPasswordHasher.AssertExpectations(t)
	// Другие репозитории не должны быть вызваны
	mockUserRepo.AssertNotCalled(t, "FindByEmail")
	mockTenantRepo.AssertNotCalled(t, "Create")
	mockPasswordHasher.AssertNotCalled(t, "Hash", password)
	mockJWTManager.AssertNotCalled(t, "GenerateToken")
	mockSessionRepo.AssertNotCalled(t, "Create")
}

func TestAuthService_RefreshToken_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockTenantRepo := new(MockTenantRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockJWTManager := new(MockJWTManager)
	mockPasswordHasher := new(MockPasswordHasher)

	// Создаем сервис
	authService := service.NewAuthService(
		mockUserRepo,
		mockTenantRepo,
		mockSessionRepo,
		mockJWTManager,
		mockPasswordHasher,
	)

	// Подготовка данных
	ctx := context.Background()
	refreshToken := "refresh_token"
	userID := "user-123"
	tenantID := "tenant-123"
	sessionID := "session-123"

	// Моки
	claims := &jwt.TokenClaims{
		UserID:    userID,
		TenantID:  tenantID,
		IsAdmin:   true,
		TokenType: "refresh",
	}
	mockJWTManager.On("ValidateRefreshToken", refreshToken).Return(claims, nil)

	hashedRefreshToken := "hashed_refresh_token"
	mockPasswordHasher.On("Hash", refreshToken).Return(hashedRefreshToken, nil)

	session := &domain.Session{
		ID:               sessionID,
		UserID:           userID,
		AccessTokenHash:  "old_access_hash",
		RefreshTokenHash: hashedRefreshToken,
		ExpiresAt:        time.Now().UTC().Add(7 * 24 * time.Hour),
		CreatedAt:        time.Now().UTC(),
	}
	mockSessionRepo.On("FindByRefreshTokenHash", ctx, hashedRefreshToken).Return(session, nil)

	mockSessionRepo.On("Delete", ctx, sessionID).Return(nil)

	newAccessToken := "new_access_token"
	newRefreshToken := "new_refresh_token"
	mockJWTManager.On("GenerateToken", userID, tenantID, true).Return(newAccessToken, newRefreshToken, nil)

	hashedNewAccessToken := "hashed_new_access_token"
	hashedNewRefreshToken := "hashed_new_refresh_token"
	mockPasswordHasher.On("Hash", newAccessToken).Return(hashedNewAccessToken, nil)
	mockPasswordHasher.On("Hash", newRefreshToken).Return(hashedNewRefreshToken, nil)

	mockSessionRepo.On("Create", ctx, mock.MatchedBy(func(s *domain.Session) bool {
		return len(s.ID) > 0 &&
			s.UserID == userID &&
			s.AccessTokenHash == hashedNewAccessToken &&
			s.RefreshTokenHash == hashedNewRefreshToken &&
			!s.ExpiresAt.IsZero() &&
			!s.CreatedAt.IsZero()
	})).Return(nil)

	// Act
	result, err := authService.RefreshToken(ctx, refreshToken)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, newAccessToken, result.AccessToken)
	assert.Equal(t, newRefreshToken, result.RefreshToken)

	// Проверяем, что все моки были вызваны
	mockJWTManager.AssertExpectations(t)
	mockPasswordHasher.AssertExpectations(t)
	mockSessionRepo.AssertExpectations(t)
}

func TestAuthService_RefreshToken_InvalidToken(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockTenantRepo := new(MockTenantRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockJWTManager := new(MockJWTManager)
	mockPasswordHasher := new(MockPasswordHasher)

	// Создаем сервис
	authService := service.NewAuthService(
		mockUserRepo,
		mockTenantRepo,
		mockSessionRepo,
		mockJWTManager,
		mockPasswordHasher,
	)

	// Подготовка данных
	ctx := context.Background()
	refreshToken := "invalid_refresh_token"

	// Моки
	mockJWTManager.On("ValidateRefreshToken", refreshToken).Return((*jwt.TokenClaims)(nil), errors.New("invalid token"))

	// Act
	result, err := authService.RefreshToken(ctx, refreshToken)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, service.ErrUnauthorized, err)

	// Проверяем, что ValidateRefreshToken был вызван
	mockJWTManager.AssertExpectations(t)
	// Другие методы не должны быть вызваны
	mockPasswordHasher.AssertNotCalled(t, "Hash")
	mockSessionRepo.AssertNotCalled(t, "FindByRefreshTokenHash")
}

func TestAuthService_RefreshToken_TokenNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockTenantRepo := new(MockTenantRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockJWTManager := new(MockJWTManager)
	mockPasswordHasher := new(MockPasswordHasher)

	// Создаем сервис
	authService := service.NewAuthService(
		mockUserRepo,
		mockTenantRepo,
		mockSessionRepo,
		mockJWTManager,
		mockPasswordHasher,
	)

	// Подготовка данных
	ctx := context.Background()
	refreshToken := "refresh_token"

	// Моки
	claims := &jwt.TokenClaims{
		UserID:    "user-123",
		TenantID:  "tenant-123",
		IsAdmin:   true,
		TokenType: "refresh",
	}
	mockJWTManager.On("ValidateRefreshToken", refreshToken).Return(claims, nil)

	hashedRefreshToken := "hashed_refresh_token"
	mockPasswordHasher.On("Hash", refreshToken).Return(hashedRefreshToken, nil)

	mockSessionRepo.On("FindByRefreshTokenHash", ctx, hashedRefreshToken).Return((*domain.Session)(nil), errors.New("session not found"))

	// Act
	result, err := authService.RefreshToken(ctx, refreshToken)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, service.ErrUnauthorized, err)

	// Проверяем, что все моки были вызваны
	mockJWTManager.AssertExpectations(t)
	mockPasswordHasher.AssertExpectations(t)
	mockSessionRepo.AssertExpectations(t)
}

func TestAuthService_Logout_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockTenantRepo := new(MockTenantRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockJWTManager := new(MockJWTManager)
	mockPasswordHasher := new(MockPasswordHasher)

	// Создаем сервис
	authService := service.NewAuthService(
		mockUserRepo,
		mockTenantRepo,
		mockSessionRepo,
		mockJWTManager,
		mockPasswordHasher,
	)

	// Подготовка данных
	ctx := context.Background()
	userID := "user-123"
	tokenID := "session-123"

	// Моки
	session := &domain.Session{
		ID:     tokenID,
		UserID: userID,
	}
	mockSessionRepo.On("FindByID", ctx, tokenID).Return(session, nil)

	mockSessionRepo.On("Delete", ctx, tokenID).Return(nil)

	// Act
	err := authService.Logout(ctx, userID, tokenID)

	// Assert
	require.NoError(t, err)

	// Проверяем, что все моки были вызваны
	mockSessionRepo.AssertExpectations(t)
}

func TestAuthService_Logout_SessionNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockTenantRepo := new(MockTenantRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockJWTManager := new(MockJWTManager)
	mockPasswordHasher := new(MockPasswordHasher)

	// Создаем сервис
	authService := service.NewAuthService(
		mockUserRepo,
		mockTenantRepo,
		mockSessionRepo,
		mockJWTManager,
		mockPasswordHasher,
	)

	// Подготовка данных
	ctx := context.Background()
	userID := "user-123"
	tokenID := "session-123"

	// Моки
	mockSessionRepo.On("FindByID", ctx, tokenID).Return((*domain.Session)(nil), errors.New("session not found"))

	// Act
	err := authService.Logout(ctx, userID, tokenID)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, service.ErrNotFound, err)

	// Проверяем, что FindByID был вызван
	mockSessionRepo.AssertExpectations(t)
	// Метод Delete не должен быть вызван
	mockSessionRepo.AssertNotCalled(t, "Delete", ctx, tokenID)
}

func TestAuthService_Logout_Forbidden(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockTenantRepo := new(MockTenantRepository)
	mockSessionRepo := new(MockSessionRepository)
	mockJWTManager := new(MockJWTManager)
	mockPasswordHasher := new(MockPasswordHasher)

	// Создаем сервис
	authService := service.NewAuthService(
		mockUserRepo,
		mockTenantRepo,
		mockSessionRepo,
		mockJWTManager,
		mockPasswordHasher,
	)

	// Подготовка данных
	ctx := context.Background()
	userID := "user-123"
	tokenID := "session-123"

	// Моки
	session := &domain.Session{
		ID:     tokenID,
		UserID: "other-user-456", // Другой пользователь
	}
	mockSessionRepo.On("FindByID", ctx, tokenID).Return(session, nil)

	// Act
	err := authService.Logout(ctx, userID, tokenID)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, service.ErrForbidden, err)

	// Проверяем, что FindByID был вызван
	mockSessionRepo.AssertExpectations(t)
	// Метод Delete не должен быть вызван
	mockSessionRepo.AssertNotCalled(t, "Delete", ctx, tokenID)
}
