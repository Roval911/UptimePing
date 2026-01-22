package service

import (
	"context"
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

// AuthService интерфейс для сервиса аутентификации
type AuthService interface {
	Login(ctx context.Context, email, password string) (*TokenPair, error)
	Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, userID, tokenID string) error
}

// Service реализация AuthService
type Service struct {
	userRepository    repository.UserRepository
	tenantRepository  repository.TenantRepository
	sessionRepository repository.SessionRepository
	jwtManager        jwt.JWTManager
	passwordHasher    password.Hasher
}

// NewAuthService создает новый экземпляр AuthService
func NewAuthService(
	userRepository repository.UserRepository,
	tenantRepository repository.TenantRepository,
	sessionRepository repository.SessionRepository,
	jwtManager jwt.JWTManager,
	passwordHasher password.Hasher,
) AuthService {
	return &Service{
		userRepository:    userRepository,
		tenantRepository:  tenantRepository,
		sessionRepository: sessionRepository,
		jwtManager:        jwtManager,
		passwordHasher:    passwordHasher,
	}
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
