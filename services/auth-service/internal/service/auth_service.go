package service

import (
	"context"
	"errors"
	"fmt"
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

// TokenPair структура для хранения пары токенов
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// AuthService интерфейс для сервиса аутентификации
type AuthService interface {
	Login(ctx context.Context, email, password string) (*TokenPair, error)
}

// Service реализация AuthService
type Service struct {
	userRepository    repository.UserRepository
	sessionRepository repository.SessionRepository
	jwtManager        jwt.JWTManager
	passwordHasher    password.Hasher
}

// NewAuthService создает новый экземпляр AuthService
func NewAuthService(
	userRepository repository.UserRepository,
	sessionRepository repository.SessionRepository,
	jwtManager jwt.JWTManager,
	passwordHasher password.Hasher,
) AuthService {
	return &Service{
		userRepository:    userRepository,
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
