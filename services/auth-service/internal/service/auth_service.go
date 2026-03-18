package service

import (
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/pkg/hash"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/pkg/password"
	"UptimePingPlatform/services/auth-service/internal/repository"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
)

// ErrNotFound ошибка, когда пользователь не найден
var ErrNotFound = errors.New(errors.ErrNotFound, "user not found")

// ErrForbidden ошибка, когда пользователь не активен
var ErrForbidden = errors.New(errors.ErrForbidden, "user is not active")

// ErrUnauthorized ошибка, когда неверный пароль
var ErrUnauthorized = errors.New(errors.ErrUnauthorized, "invalid credentials")

// ErrConflict ошибка, когда пользователь уже существует
var ErrConflict = errors.New(errors.ErrConflict, "user already exists")

// TokenPair структура для хранения пары токенов
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TenantID     string `json:"tenant_id"`
}

// APIKeyPair структура для хранения пары API ключей
// Публичный ключ (key) и секретный ключ (secret)
type APIKeyPair struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

// Claims структура для данных, возвращаемых при валидации API ключа
// Содержит информацию о тенанте и ключе
type Claims struct {
	TenantID string `json:"tenant_id"`
	KeyID    string `json:"key_id"`
}

// AuthService интерфейс для сервиса аутентификации
type AuthService interface {
	Login(ctx context.Context, email, password string) (*TokenPair, error)
	Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, userID, tokenID string) error
	CreateAPIKey(ctx context.Context, tenantID, name string) (*APIKeyPair, error)
	ValidateAPIKey(ctx context.Context, key, secret string) (*Claims, error)
	RevokeAPIKey(ctx context.Context, keyID string) error
	GetUserByID(ctx context.Context, userID string) (*domain.User, error)
	GetUserRoles(ctx context.Context, userID string) ([]string, error)

	// Управление ролями
	AssignRole(ctx context.Context, userID, roleID string, assignedBy string, expiresAt *time.Time) error
	RemoveRole(ctx context.Context, userID, roleID string) error
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)
	GetAllRoles(ctx context.Context) ([]*domain.Role, error)
	CreateRole(ctx context.Context, role *domain.Role) error
	UpdateRole(ctx context.Context, roleID string, role *domain.Role) error
	DeleteRole(ctx context.Context, roleID string) error

	// Управление разрешениями
	GetRolePermissions(ctx context.Context, roleID string) ([]*domain.Permission, error)
	AssignPermissionToRole(ctx context.Context, roleID, permissionID string) error
	RemovePermissionFromRole(ctx context.Context, roleID, permissionID string) error
	GetAllPermissions(ctx context.Context) ([]*domain.Permission, error)
	CreatePermission(ctx context.Context, permission *domain.Permission) error
	UpdatePermission(ctx context.Context, permissionID string, permission *domain.Permission) error
	DeletePermission(ctx context.Context, permissionID string) error
}

// Service реализация AuthService
type Service struct {
	userRepository    repository.UserRepository
	tenantRepository  repository.TenantRepository
	sessionRepository repository.SessionRepository
	apiKeyRepository  repository.APIKeyRepository
	jwtManager        jwt.JWTManager
	passwordHasher    password.Hasher
	tokenHasher       *hash.TokenHasher
	redisClient       pkg_redis.Client
	log               logger.Logger
}

// NewAuthService создает новый экземпляр AuthService
func NewAuthService(
	userRepository repository.UserRepository,
	tenantRepository repository.TenantRepository,
	apiKeyRepository repository.APIKeyRepository,
	sessionRepository repository.SessionRepository,
	jwtManager jwt.JWTManager,
	passwordHasher password.Hasher,
	redisClient pkg_redis.Client,
	log logger.Logger,
) AuthService {
	return &Service{
		userRepository:    userRepository,
		tenantRepository:  tenantRepository,
		apiKeyRepository:  apiKeyRepository,
		sessionRepository: sessionRepository,
		jwtManager:        jwtManager,
		passwordHasher:    passwordHasher,
		tokenHasher:       hash.NewTokenHasher(),
		redisClient:       redisClient,
		log:               log,
	}
}

// generateAPIKey генерирует случайную строку заданной длины
func generateAPIKey(length int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		// В случае ошибки используем простую генерацию
		for i := range key {
			key[i] = chars[i%len(chars)]
		}
	} else {
		for i := range key {
			key[i] = chars[int(key[i])%len(chars)]
		}
	}
	return string(key)
}

// hashAPIKey хеширует ключ с использованием SHA256
func (s *Service) hashAPIKey(key string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
}

// CreateAPIKey создает новую пару API ключей
func (s *Service) CreateAPIKey(ctx context.Context, tenantID, name string) (*APIKeyPair, error) {
	// Валидация входных данных
	if tenantID == "" {
		return nil, errors.New(errors.ErrValidation, "tenant ID is required")
	}

	if name == "" {
		return nil, errors.New(errors.ErrValidation, "name is required")
	}

	// Генерация публичного ключа (key)
	key := generateAPIKey(16) // 16 символов для публичного ключа

	// Генерация секретного ключа (secret)
	secret := generateAPIKey(32) // 32 символа для секретного ключа

	// Хеширование ключей
	keyHash := s.hashAPIKey(key)
	secretHash := s.hashAPIKey(secret)

	// Убедимся, что хэши не пустые
	if keyHash == "" || secretHash == "" {
		return nil, errors.New(errors.ErrInternal, "failed to hash API keys")
	}
	// Создание новой записи API ключа
	apiKey := &domain.APIKey{
		ID:         uuid.New().String(),
		TenantID:   tenantID,
		KeyHash:    keyHash,
		SecretHash: secretHash,
		Name:       name,
		KeyPrefix: func() string {
			if len(key) >= 16 {
				return key[:16]
			}
			return key
		}(),
		Permissions: map[string]interface{}{},
		IsActive:    true,
		ExpiresAt:   func() *time.Time { t := time.Now().UTC().Add(365 * 24 * time.Hour); return &t }(), // Срок действия 1 год
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	// Сохранение в БД
	err := s.apiKeyRepository.Create(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	// Возврат публичного и секретного ключей
	// Секретный ключ возвращается только один раз
	return &APIKeyPair{
		Key:    key,
		Secret: secret,
	}, nil
}

// ValidateAPIKey проверяет валидность API ключа
func (s *Service) ValidateAPIKey(ctx context.Context, key, secret string) (*Claims, error) {
	// Валидация входных данных
	if key == "" {
		return nil, errors.New(errors.ErrValidation, "key is required")
	}

	if secret == "" {
		return nil, errors.New(errors.ErrValidation, "secret is required")
	}

	// Хешируем ключи для поиска
	keyHash := s.hashAPIKey(key)

	// Поиск API ключа в БД по хэшу публичного ключа
	apiKey, err := s.apiKeyRepository.FindByKeyHash(ctx, keyHash)
	if err != nil {
		return nil, ErrUnauthorized // ключ не найден
	}

	// Проверка активности ключа
	if !apiKey.IsActive {
		return nil, ErrForbidden // ключ деактивирован
	}

	// Проверка срока действия
	if apiKey.ExpiresAt.Before(time.Now().UTC()) {
		return nil, errors.New(errors.ErrUnauthorized, "API key expired") // срок действия истек
	}

	// Хешируем предоставленный секретный ключ
	secretHash := s.hashAPIKey(secret)

	// Сравниваем хэши секретных ключей
	if secretHash != apiKey.SecretHash {
		return nil, ErrUnauthorized // неверный секретный ключ
	}

	// Возвращаем данные для авторизации
	return &Claims{
		TenantID: apiKey.TenantID,
		KeyID:    apiKey.ID,
	}, nil
}

// RevokeAPIKey деактивирует API ключ
func (s *Service) RevokeAPIKey(ctx context.Context, keyID string) error {
	// Валидация входных данных
	if keyID == "" {
		return errors.New(errors.ErrValidation, "key ID is required")
	}

	// Поиск API ключа по ID
	apiKey, err := s.apiKeyRepository.FindByID(ctx, keyID)
	if err != nil {
		return ErrNotFound // ключ не найден
	}

	// Деактивация ключа
	apiKey.IsActive = false

	// Обновление в БД
	err = s.apiKeyRepository.Update(ctx, apiKey)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	return nil
}

// Login реализует аутентификацию пользователя
func (s *Service) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	s.log.Info("User login attempt", logger.String("email", email))

	// Валидация входных данных
	if email == "" {
		s.log.Warn("Login failed: email is required")
		return nil, errors.New(errors.ErrValidation, "email is required")
	}

	if password == "" {
		s.log.Warn("Login failed: password is required", logger.String("email", email))
		return nil, errors.New(errors.ErrValidation, "password is required")
	}

	// Поиск пользователя по email
	user, err := s.userRepository.FindByEmail(ctx, email)
	if err != nil {
		s.log.Warn("Login failed: user not found",
			logger.String("email", email),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrNotFound, "user not found")
	}

	// Проверка, что пользователь активен
	if !user.IsActive {
		s.log.Warn("Login failed: user is not active",
			logger.String("email", email),
			logger.String("user_id", user.ID))
		return nil, errors.New(errors.ErrForbidden, "user is not active")
	}

	// Проверка пароля
	if !s.passwordHasher.Check(password, user.PasswordHash) {
		s.log.Warn("Login failed: invalid password",
			logger.String("email", email),
			logger.String("user_id", user.ID))
		return nil, errors.New(errors.ErrUnauthorized, "invalid credentials")
	}

	// Генерация JWT токенов
	accessToken, refreshToken, err := s.jwtManager.GenerateToken(user.ID, user.TenantID, user.IsAdmin)
	if err != nil {
		s.log.Error("Failed to generate tokens",
			logger.String("email", email),
			logger.String("user_id", user.ID),
			logger.Error(err))
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Хешируем refresh токен для безопасного хранения
	refreshTokenHash, err := s.tokenHasher.Hash(refreshToken)
	if err != nil {
		s.log.Error("Failed to hash refresh token",
			logger.String("email", email),
			logger.String("user_id", user.ID),
			logger.Error(err))
		return nil, fmt.Errorf("failed to hash refresh token: %w", err)
	}

	// Создаем новую сессию
	session := &domain.Session{
		ID:               uuid.New().String(),
		UserID:           user.ID,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        func() *time.Time { t := time.Now().UTC().Add(7 * 24 * time.Hour); return &t }(), // 7 дней
		CreatedAt:        time.Now().UTC(),
	}

	// Сохраняем сессию в Redis
	err = s.sessionRepository.Create(ctx, session)
	if err != nil {
		s.log.Error("Failed to save session",
			logger.String("email", email),
			logger.String("user_id", user.ID),
			logger.String("session_id", session.ID),
			logger.Error(err))
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	s.log.Info("User login successful",
		logger.String("email", email),
		logger.String("user_id", user.ID),
		logger.String("session_id", session.ID))

	// Возвращаем токены
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TenantID:     user.TenantID,
	}, nil
}

// Register реализует регистрацию нового пользователя
func (s *Service) Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error) {
	s.log.Info("User registration attempt",
		logger.String("email", email),
		logger.String("tenant_name", tenantName))

	// Валидация email и password
	if email == "" {
		s.log.Warn("Registration failed: email is required")
		return nil, errors.New(errors.ErrValidation, "email is required")
	}

	if password == "" {
		s.log.Warn("Registration failed: password is required", logger.String("email", email))
		return nil, errors.New(errors.ErrValidation, "password is required")
	}

	if !s.passwordHasher.Validate(password) {
		s.log.Warn("Registration failed: password does not meet complexity requirements",
			logger.String("email", email))
		return nil, errors.New(errors.ErrValidation, "password does not meet complexity requirements")
	}

	// Валидация tenantName
	if tenantName == "" {
		s.log.Warn("Registration failed: tenant name is required", logger.String("email", email))
		return nil, errors.New(errors.ErrValidation, "tenant name is required")
	}

	// Проверка существования пользователя по email
	_, err := s.userRepository.FindByEmail(ctx, email)
	if err == nil {
		s.log.Warn("Registration failed: user already exists", logger.String("email", email))
		return nil, errors.New(errors.ErrConflict, "user already exists") // Пользователь уже существует
	}

	// Создание или получение tenant по имени
	tenant, err := s.tenantRepository.FindBySlug(ctx, generateSlug(tenantName))
	if err != nil {
		// Tenant не найден, создаем новый
		tenant = &domain.Tenant{
			ID:        uuid.New().String(),
			Name:      tenantName,
			Slug:      generateSlug(tenantName),
			Settings:  make(map[string]interface{}),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		err = s.tenantRepository.Create(ctx, tenant)
		if err != nil {
			s.log.Error("Failed to create tenant",
				logger.String("email", email),
				logger.String("tenant_name", tenantName),
				logger.Error(err))
			return nil, fmt.Errorf("failed to create tenant: %w", err)
		}
		s.log.Info("Created new tenant",
			logger.String("tenant_id", tenant.ID),
			logger.String("tenant_name", tenantName))
	}

	// Хеширование пароля
	passwordHash, err := s.passwordHasher.Hash(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Создание пользователя в БД
	user := &domain.User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: passwordHash,
		TenantID:     tenant.ID,
		IsActive:     true,
		IsAdmin:      true, // Первый пользователь в тенанте - админ
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	err = s.userRepository.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Генерация токенов
	accessToken, refreshToken, err := s.jwtManager.GenerateToken(user.ID, user.TenantID, user.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Хешируем refresh токен для безопасного хранения
	refreshTokenHash, err := s.tokenHasher.Hash(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash refresh token: %w", err)
	}

	// Создаем новую сессию
	session := &domain.Session{
		ID:               uuid.New().String(),
		UserID:           user.ID,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        func() *time.Time { t := time.Now().UTC().Add(7 * 24 * time.Hour); return &t }(),
		CreatedAt:        time.Now().UTC(),
	}

	// Сохраняем сессию в Redis
	err = s.sessionRepository.Create(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Возвращаем токены
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TenantID:     user.TenantID,
	}, nil
}

// RefreshToken обновляет пару токенов
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// Парсинг refresh токена
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrUnauthorized, "failed to validate refresh token")
	}

	// Хешируем refresh токен для поиска в Redis
	hashedRefreshToken, err := s.tokenHasher.Hash(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash refresh token: %w", err)
	}

	// Поиск токена в Redis
	session, err := s.sessionRepository.FindByRefreshTokenHash(ctx, hashedRefreshToken)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrUnauthorized, "refresh token not found")
	}

	// Удаление старого refresh токена из Redis
	err = s.sessionRepository.Delete(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old session: %w", err)
	}

	// Генерация новой пары токенов
	newAccessToken, newRefreshToken, err := s.jwtManager.GenerateToken(claims.UserID, claims.TenantID, claims.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new tokens: %w", err)
	}

	// Хешируем новый refresh токен для безопасного хранения
	newRefreshTokenHash, err := s.tokenHasher.Hash(newRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash new refresh token: %w", err)
	}

	// Создаем новую сессию
	newSession := &domain.Session{
		ID:               uuid.New().String(),
		UserID:           claims.UserID,
		RefreshTokenHash: newRefreshTokenHash,
		ExpiresAt:        func() *time.Time { t := time.Now().UTC().Add(7 * 24 * time.Hour); return &t }(),
		CreatedAt:        time.Now().UTC(),
	}

	// Сохраняем новый refresh токен в Redis
	err = s.sessionRepository.Create(ctx, newSession)
	if err != nil {
		return nil, fmt.Errorf("failed to save new session: %w", err)
	}

	// Возвращаем новые токены
	return &TokenPair{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TenantID:     claims.TenantID,
	}, nil
}

// Logout деактивирует сессию пользователя
func (s *Service) Logout(ctx context.Context, userID, tokenID string) error {
	// Поиск сессии по ID
	session, err := s.sessionRepository.FindByID(ctx, tokenID)
	if err != nil {
		return errors.Wrap(err, errors.ErrNotFound, "session not found")
	}

	// Проверка, что сессия принадлежит пользователю
	if session.UserID != userID {
		return ErrForbidden
	}

	// Удаление сессии из Redis
	err = s.sessionRepository.Delete(ctx, tokenID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// generateSlug генерирует slug из имени тенанта
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Удаление или замена других символов может быть добавлена по необходимости
	return slug
}

// GetUserByID получает пользователя по ID
func (s *Service) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	if userID == "" {
		return nil, errors.New(errors.ErrValidation, "user ID is required")
	}

	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get user by ID")
	}

	if user == nil {
		return nil, ErrNotFound
	}

	return user, nil
}

// GetUserRoles получает роли пользователя по ID с кешированием в Redis
func (s *Service) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	if userID == "" {
		return nil, errors.New(errors.ErrValidation, "user ID is required")
	}

	// Проверяем кеш в Redis
	cacheKey := fmt.Sprintf("user_roles:%s", userID)
	cachedRoles, err := s.redisClient.Client.Get(ctx, cacheKey).Result()
	if err == nil {
		// Кеш найден, десериализуем
		var roles []string
		if json.Unmarshal([]byte(cachedRoles), &roles) == nil {
			s.log.Debug("User roles loaded from cache", logger.String("user_id", userID))
			return roles, nil
		}
	}

	// Кеш не найден или ошибка десериализации, получаем из БД
	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get user for roles")
	}

	if user == nil {
		return nil, ErrNotFound
	}

	// Получаем роли из таблицы user_roles
	roles, err := s.getUserRolesFromDB(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Кешируем результат на 5 минут
	rolesJSON, _ := json.Marshal(roles)
	s.redisClient.Client.Set(ctx, cacheKey, rolesJSON, 5*time.Minute)

	return roles, nil
}

// getUserRolesFromDB получает роли пользователя из базы данных
func (s *Service) getUserRolesFromDB(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT r.name 
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND ur.is_active = true AND r.is_active = true
		AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
	`

	rows, err := s.userRepository.GetDB().Query(ctx, query, userID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to query user roles")
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var roleName string
		if err := rows.Scan(&roleName); err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan role")
		}
		roles = append(roles, roleName)
	}

	if len(roles) == 0 {
		// Если у пользователя нет ролей, даем базовую роль
		roles = []string{"user"}
	}

	return roles, nil
}

// AssignRole назначает роль пользователю
func (s *Service) AssignRole(ctx context.Context, userID, roleID, assignedBy string, expiresAt *time.Time) error {
	if userID == "" || roleID == "" || assignedBy == "" {
		return errors.New(errors.ErrValidation, "user ID, role ID, and assigned by are required")
	}

	// Проверяем существование пользователя и роли
	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to check user existence")
	}
	if user == nil {
		return ErrNotFound
	}

	// Назначаем роль
	query := `
		INSERT INTO user_roles (user_id, role_id, assigned_by, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, role_id) 
		DO UPDATE SET 
			assigned_by = EXCLUDED.assigned_by,
			expires_at = EXCLUDED.expires_at,
			is_active = true,
			updated_at = NOW()
	`

	_, err = s.userRepository.GetDB().Exec(ctx, query, userID, roleID, assignedBy, expiresAt)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to assign role")
	}

	// Очищаем кеш
	s.invalidateUserRolesCache(ctx, userID)

	s.log.Info("Role assigned to user",
		logger.String("user_id", userID),
		logger.String("role_id", roleID),
		logger.String("assigned_by", assignedBy))

	return nil
}

// RemoveRole удаляет роль у пользователя
func (s *Service) RemoveRole(ctx context.Context, userID, roleID string) error {
	if userID == "" || roleID == "" {
		return errors.New(errors.ErrValidation, "user ID and role ID are required")
	}

	query := `
		UPDATE user_roles 
		SET is_active = false, updated_at = NOW()
		WHERE user_id = $1 AND role_id = $2 AND is_active = true
	`

	result, err := s.userRepository.GetDB().Exec(ctx, query, userID, roleID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to remove role")
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "user role assignment not found")
	}

	// Очищаем кеш
	s.invalidateUserRolesCache(ctx, userID)

	s.log.Info("Role removed from user",
		logger.String("user_id", userID),
		logger.String("role_id", roleID))

	return nil
}

// GetUserPermissions получает все разрешения пользователя на основе его ролей
func (s *Service) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	if userID == "" {
		return nil, errors.New(errors.ErrValidation, "user ID is required")
	}

	// Проверяем кеш
	cacheKey := fmt.Sprintf("user_permissions:%s", userID)
	cachedPerms, err := s.redisClient.Client.Get(ctx, cacheKey).Result()
	if err == nil {
		var permissions []string
		if json.Unmarshal([]byte(cachedPerms), &permissions) == nil {
			return permissions, nil
		}
	}

	// Получаем разрешения из БД
	query := `
		SELECT DISTINCT p.name
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		INNER JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1 
			AND ur.is_active = true 
			AND p.is_active = true
			AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
	`

	rows, err := s.userRepository.GetDB().Query(ctx, query, userID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to query user permissions")
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var permissionName string
		if err := rows.Scan(&permissionName); err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan permission")
		}
		permissions = append(permissions, permissionName)
	}

	// Кешируем результат на 5 минут
	permsJSON, _ := json.Marshal(permissions)
	s.redisClient.Client.Set(ctx, cacheKey, permsJSON, 5*time.Minute)

	return permissions, nil
}

// GetAllRoles получает все роли системы
func (s *Service) GetAllRoles(ctx context.Context) ([]*domain.Role, error) {
	query := `
		SELECT id, name, description, is_active, created_at, updated_at
		FROM roles
		ORDER BY name
	`

	rows, err := s.userRepository.GetDB().Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to query roles")
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		role := &domain.Role{}
		err := rows.Scan(
			&role.ID, &role.Name, &role.Description, &role.IsActive,
			&role.CreatedAt, &role.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan role")
		}
		roles = append(roles, role)
	}

	return roles, nil
}

// CreateRole создает новую роль
func (s *Service) CreateRole(ctx context.Context, role *domain.Role) error {
	if role.Name == "" {
		return errors.New(errors.ErrValidation, "role name is required")
	}

	query := `
		INSERT INTO roles (name, description, is_active)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`

	err := s.userRepository.GetDB().QueryRow(ctx, query, role.Name, role.Description, role.IsActive).
		Scan(&role.ID, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to create role")
	}

	s.log.Info("Role created", logger.String("role_id", role.ID), logger.String("role_name", role.Name))
	return nil
}

// UpdateRole обновляет существующую роль
func (s *Service) UpdateRole(ctx context.Context, roleID string, role *domain.Role) error {
	if roleID == "" {
		return errors.New(errors.ErrValidation, "role ID is required")
	}

	query := `
		UPDATE roles 
		SET name = $1, description = $2, is_active = $3, updated_at = NOW()
		WHERE id = $4
	`

	result, err := s.userRepository.GetDB().Exec(ctx, query, role.Name, role.Description, role.IsActive, roleID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to update role")
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	s.log.Info("Role updated", logger.String("role_id", roleID))
	return nil
}

// DeleteRole удаляет роль
func (s *Service) DeleteRole(ctx context.Context, roleID string) error {
	if roleID == "" {
		return errors.New(errors.ErrValidation, "role ID is required")
	}

	// Проверяем, что роль не назначена пользователям
	var count int
	checkQuery := `SELECT COUNT(*) FROM user_roles WHERE role_id = $1 AND is_active = true`
	err := s.userRepository.GetDB().QueryRow(ctx, checkQuery, roleID).Scan(&count)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to check role assignments")
	}

	if count > 0 {
		return errors.New(errors.ErrConflict, "cannot delete role that is assigned to users")
	}

	query := `DELETE FROM roles WHERE id = $1`
	result, err := s.userRepository.GetDB().Exec(ctx, query, roleID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to delete role")
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	s.log.Info("Role deleted", logger.String("role_id", roleID))
	return nil
}

// GetRolePermissions получает разрешения для роли
func (s *Service) GetRolePermissions(ctx context.Context, roleID string) ([]*domain.Permission, error) {
	if roleID == "" {
		return nil, errors.New(errors.ErrValidation, "role ID is required")
	}

	query := `
		SELECT p.id, p.name, p.resource, p.action, p.description, p.is_active, p.created_at, p.updated_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.resource, p.action
	`

	rows, err := s.userRepository.GetDB().Query(ctx, query, roleID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to query role permissions")
	}
	defer rows.Close()

	var permissions []*domain.Permission
	for rows.Next() {
		perm := &domain.Permission{}
		err := rows.Scan(
			&perm.ID, &perm.Name, &perm.Resource, &perm.Action, &perm.Description,
			&perm.IsActive, &perm.CreatedAt, &perm.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan permission")
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// AssignPermissionToRole назначает разрешение роли
func (s *Service) AssignPermissionToRole(ctx context.Context, roleID, permissionID string) error {
	if roleID == "" || permissionID == "" {
		return errors.New(errors.ErrValidation, "role ID and permission ID are required")
	}

	query := `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ($1, $2)
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`

	_, err := s.userRepository.GetDB().Exec(ctx, query, roleID, permissionID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to assign permission to role")
	}

	s.log.Info("Permission assigned to role",
		logger.String("role_id", roleID),
		logger.String("permission_id", permissionID))

	return nil
}

// RemovePermissionFromRole удаляет разрешение у роли
func (s *Service) RemovePermissionFromRole(ctx context.Context, roleID, permissionID string) error {
	if roleID == "" || permissionID == "" {
		return errors.New(errors.ErrValidation, "role ID and permission ID are required")
	}

	query := `DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`

	result, err := s.userRepository.GetDB().Exec(ctx, query, roleID, permissionID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to remove permission from role")
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "role permission assignment not found")
	}

	s.log.Info("Permission removed from role",
		logger.String("role_id", roleID),
		logger.String("permission_id", permissionID))

	return nil
}

// GetAllPermissions получает все разрешения системы
func (s *Service) GetAllPermissions(ctx context.Context) ([]*domain.Permission, error) {
	query := `
		SELECT id, name, resource, action, description, is_active, created_at, updated_at
		FROM permissions
		ORDER BY resource, action
	`

	rows, err := s.userRepository.GetDB().Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to query permissions")
	}
	defer rows.Close()

	var permissions []*domain.Permission
	for rows.Next() {
		perm := &domain.Permission{}
		err := rows.Scan(
			&perm.ID, &perm.Name, &perm.Resource, &perm.Action, &perm.Description,
			&perm.IsActive, &perm.CreatedAt, &perm.UpdatedAt,
		)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan permission")
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// CreatePermission создает новое разрешение
func (s *Service) CreatePermission(ctx context.Context, permission *domain.Permission) error {
	if permission.Name == "" || permission.Resource == "" || permission.Action == "" {
		return errors.New(errors.ErrValidation, "permission name, resource, and action are required")
	}

	query := `
		INSERT INTO permissions (name, resource, action, description, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	err := s.userRepository.GetDB().QueryRow(ctx, query,
		permission.Name, permission.Resource, permission.Action,
		permission.Description, permission.IsActive).
		Scan(&permission.ID, &permission.CreatedAt, &permission.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to create permission")
	}

	s.log.Info("Permission created",
		logger.String("permission_id", permission.ID),
		logger.String("permission_name", permission.Name))
	return nil
}

// UpdatePermission обновляет существующее разрешение
func (s *Service) UpdatePermission(ctx context.Context, permissionID string, permission *domain.Permission) error {
	if permissionID == "" {
		return errors.New(errors.ErrValidation, "permission ID is required")
	}

	query := `
		UPDATE permissions 
		SET name = $1, resource = $2, action = $3, description = $4, is_active = $5, updated_at = NOW()
		WHERE id = $6
	`

	result, err := s.userRepository.GetDB().Exec(ctx, query,
		permission.Name, permission.Resource, permission.Action,
		permission.Description, permission.IsActive, permissionID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to update permission")
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	s.log.Info("Permission updated", logger.String("permission_id", permissionID))
	return nil
}

// DeletePermission удаляет разрешение
func (s *Service) DeletePermission(ctx context.Context, permissionID string) error {
	if permissionID == "" {
		return errors.New(errors.ErrValidation, "permission ID is required")
	}

	// Проверяем, что разрешение не назначено ролям
	var count int
	checkQuery := `SELECT COUNT(*) FROM role_permissions WHERE permission_id = $1`
	err := s.userRepository.GetDB().QueryRow(ctx, checkQuery, permissionID).Scan(&count)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to check permission assignments")
	}

	if count > 0 {
		return errors.New(errors.ErrConflict, "cannot delete permission that is assigned to roles")
	}

	query := `DELETE FROM permissions WHERE id = $1`
	result, err := s.userRepository.GetDB().Exec(ctx, query, permissionID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to delete permission")
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	s.log.Info("Permission deleted", logger.String("permission_id", permissionID))
	return nil
}

// invalidateUserRolesCache очищает кеш ролей пользователя
func (s *Service) invalidateUserRolesCache(ctx context.Context, userID string) {
	cacheKey := fmt.Sprintf("user_roles:%s", userID)
	s.redisClient.Client.Del(ctx, cacheKey)

	// Также очищаем кеш разрешений
	permCacheKey := fmt.Sprintf("user_permissions:%s", userID)
	s.redisClient.Client.Del(ctx, permCacheKey)
}
