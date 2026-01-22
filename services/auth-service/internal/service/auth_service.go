package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"UptimePingPlatform/services/auth-service/internal/domain"
	"UptimePingPlatform/services/auth-service/internal/pkg/jwt"
	"UptimePingPlatform/services/auth-service/internal/pkg/password"
	"UptimePingPlatform/services/auth-service/internal/repository"
	"github.com/google/uuid"
)

// ErrNotFound ошибка, когда пользователь не найден
var ErrNotFound = errors.New("user not found")

// ErrForbidden ошибка, когда пользователь не активен
var ErrForbidden = errors.New("user is not active")

// ErrUnauthorized ошибка, когда неверный пароль
var ErrUnauthorized = errors.New("invalid credentials")

// ErrConflict ошибка, когда пользователь уже существует
var ErrConflict = errors.New("user already exists")

// TokenPair структура для хранения пары токенов
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
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
}

// Service реализация AuthService
type Service struct {
	userRepository    repository.UserRepository
	tenantRepository  repository.TenantRepository
	sessionRepository repository.SessionRepository
	apiKeyRepository  repository.APIKeyRepository
	jwtManager        jwt.JWTManager
	passwordHasher    password.Hasher
}

// NewAuthService создает новый экземпляр AuthService
func NewAuthService(
	userRepository repository.UserRepository,
	tenantRepository repository.TenantRepository,
	apiKeyRepository repository.APIKeyRepository,
	sessionRepository repository.SessionRepository,
	jwtManager jwt.JWTManager,
	passwordHasher password.Hasher,
) AuthService {
	return &Service{
		userRepository:    userRepository,
		tenantRepository:  tenantRepository,
		apiKeyRepository:  apiKeyRepository,
		sessionRepository: sessionRepository,
		jwtManager:        jwtManager,
		passwordHasher:    passwordHasher,
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
		return nil, errors.New("tenant ID is required")
	}

	if name == "" {
		return nil, errors.New("name is required")
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
		return nil, errors.New("failed to hash API keys")
	}
	// Создание новой записи API ключа
	apiKey := &domain.APIKey{
		ID:         uuid.New().String(),
		TenantID:   tenantID,
		KeyHash:    keyHash,
		SecretHash: secretHash,
		Name:       name,
		IsActive:   true,
		ExpiresAt:  time.Now().UTC().Add(365 * 24 * time.Hour), // Срок действия 1 год
		CreatedAt:  time.Now().UTC(),
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
		return nil, errors.New("key is required")
	}

	if secret == "" {
		return nil, errors.New("secret is required")
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
		return nil, ErrUnauthorized // срок действия истек
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
		return errors.New("key ID is required")
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
	// Валидация входных данных
	if email == "" {
		return nil, errors.New("email is required")
	}

	if password == "" {
		return nil, errors.New("password is required")
	}

	// Поиск пользователя по email
	user, err := s.userRepository.FindByEmail(ctx, email)
	if err != nil {
		return nil, ErrNotFound
	}

	// Проверка, что пользователь активен
	if !user.IsActive {
		return nil, ErrForbidden
	}

	// Проверка пароля
	if !s.passwordHasher.Check(password, user.PasswordHash) {
		return nil, ErrUnauthorized
	}

	// Генерация JWT токенов
	accessToken, refreshToken, err := s.jwtManager.GenerateToken(user.ID, user.TenantID, user.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Хешируем токены для безопасного хранения
	accessTokenHash, err := s.passwordHasher.Hash(accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash access token: %w", err)
	}

	refreshTokenHash, err := s.passwordHasher.Hash(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash refresh token: %w", err)
	}

	// Создаем новую сессию
	session := &domain.Session{
		ID:               uuid.New().String(),
		UserID:           user.ID,
		AccessTokenHash:  accessTokenHash,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        time.Now().UTC().Add(7 * 24 * time.Hour), // 7 дней
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
	}, nil
}

// Register реализует регистрацию нового пользователя
func (s *Service) Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error) {
	// Валидация email и password
	if email == "" {
		return nil, errors.New("email is required")
	}

	if password == "" {
		return nil, errors.New("password is required")
	}

	if !s.passwordHasher.Validate(password) {
		return nil, errors.New("password does not meet complexity requirements")
	}

	// Валидация tenantName
	if tenantName == "" {
		return nil, errors.New("tenant name is required")
	}

	// Проверка существования пользователя по email
	_, err := s.userRepository.FindByEmail(ctx, email)
	if err == nil {
		return nil, ErrConflict // Пользователь уже существует
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
			return nil, fmt.Errorf("failed to create tenant: %w", err)
		}
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

	// Хешируем токены для безопасного хранения
	accessTokenHash, err := s.passwordHasher.Hash(accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash access token: %w", err)
	}

	refreshTokenHash, err := s.passwordHasher.Hash(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash refresh token: %w", err)
	}

	// Создаем новую сессию
	session := &domain.Session{
		ID:               uuid.New().String(),
		UserID:           user.ID,
		AccessTokenHash:  accessTokenHash,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        time.Now().UTC().Add(7 * 24 * time.Hour),
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
	}, nil
}

// RefreshToken обновляет пару токенов
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// Парсинг refresh токена
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrUnauthorized
	}

	// Хешируем refresh токен для поиска в Redis
	hashedRefreshToken, err := s.passwordHasher.Hash(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash refresh token: %w", err)
	}

	// Поиск токена в Redis
	session, err := s.sessionRepository.FindByRefreshTokenHash(ctx, hashedRefreshToken)
	if err != nil {
		return nil, ErrUnauthorized // токен отозван или не найден
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

	// Хешируем новые токены для безопасного хранения
	newAccessTokenHash, err := s.passwordHasher.Hash(newAccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash new access token: %w", err)
	}

	newRefreshTokenHash, err := s.passwordHasher.Hash(newRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash new refresh token: %w", err)
	}

	// Создаем новую сессию
	newSession := &domain.Session{
		ID:               uuid.New().String(),
		UserID:           claims.UserID,
		AccessTokenHash:  newAccessTokenHash,
		RefreshTokenHash: newRefreshTokenHash,
		ExpiresAt:        time.Now().UTC().Add(7 * 24 * time.Hour),
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
	}, nil
}

// Logout деактивирует сессию пользователя
func (s *Service) Logout(ctx context.Context, userID, tokenID string) error {
	// Поиск сессии по ID
	session, err := s.sessionRepository.FindByID(ctx, tokenID)
	if err != nil {
		return ErrNotFound
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
	// Простая реализация slug: преобразуем в нижний регистр и заменяем пробелы на дефисы
	// В реальной системе может потребоваться более сложная логика
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Удаление или замена других символов может быть добавлена по необходимости
	return slug
}
