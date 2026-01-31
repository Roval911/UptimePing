package auth

import (
	"context"
	"time"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/cli-service/internal/client"
	"UptimePingPlatform/services/cli-service/internal/config"
	"UptimePingPlatform/services/cli-service/internal/store"
)

// TokenStoreInterface определяет интерфейс для хранилища токенов
type TokenStoreInterface interface {
	SaveTokens(tokenInfo *store.TokenInfo) error
	LoadTokens() (*store.TokenInfo, error)
	HasTokens() bool
	ClearTokens() error
	GetAccessToken() string
}

// AuthManager управляет аутентификацией
type AuthManager struct {
	config     *config.Config
	tokenStore TokenStoreInterface
	validator  *validation.Validator
	httpClient *client.HTTPAuthClient
	useGRPC    bool // Всегда false для CLI
}

// NewAuthManager создает новый менеджер аутентификации
func NewAuthManager(cfg *config.Config) (*AuthManager, error) {
	tokenStore, err := store.NewTokenStore()
	if err != nil {
		return nil, errors.New(errors.ErrInternal, "ошибка создания хранилища токенов")
	}

	// Создаем валидатор
	validator := validation.NewValidator()

	// CLI всегда использует HTTP через API Gateway
	// Убираем gRPC для правильной архитектуры: CLI → API Gateway → Сервисы
	httpClient, err := client.NewAuthHTTPClient(cfg.API.BaseURL)
	if err != nil {
		return nil, errors.New(errors.ErrInternal, "ошибка создания HTTP клиента для Auth Service")
	}

	return &AuthManager{
		config:     cfg,
		tokenStore: tokenStore,
		validator:  validator,
		httpClient: httpClient,
		useGRPC:    false,
	}, nil
}

// NewAuthManagerWithTokenStore создает новый менеджер аутентификации с кастомным хранилищем токенов
func NewAuthManagerWithTokenStore(cfg *config.Config, tokenStore TokenStoreInterface) (*AuthManager, error) {
	// Создаем валидатор
	validator := validation.NewValidator()

	// CLI всегда использует HTTP через API Gateway
	httpClient, err := client.NewAuthHTTPClient(cfg.API.BaseURL)
	if err != nil {
		return nil, errors.New(errors.ErrInternal, "ошибка создания HTTP клиента для Auth Service")
	}

	return &AuthManager{
		config:     cfg,
		tokenStore: tokenStore,
		validator:  validator,
		httpClient: httpClient,
		useGRPC:    false,
	}, nil
}

// LoginInput содержит данные для входа
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login выполняет вход пользователя
func (am *AuthManager) Login(ctx context.Context, input *LoginInput) error {
	// Валидация входных данных
	if err := am.validator.ValidateRequiredFields(map[string]interface{}{
		"email":    input.Email,
		"password": input.Password,
	}, map[string]string{
		"email":    "Email address",
		"password": "Password",
	}); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "невалидные данные входа")
	}

	// Выполняем вход через HTTP
	tokenPair, err := am.httpClient.Login(ctx, input.Email, input.Password)
	if err != nil {
		return errors.Wrap(err, errors.ErrUnauthorized, "ошибка входа через HTTP")
	}

	// Сохраняем токены
	tokenInfo := &store.TokenInfo{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(am.config.Auth.TokenExpiry) * time.Second),
		TenantID:     tokenPair.TenantID,
		TenantName:   tokenPair.TenantName,
		Email:        input.Email,
	}

	if err := am.tokenStore.SaveTokens(tokenInfo); err != nil {
		return errors.Wrap(err, errors.ErrInternal, "ошибка сохранения токенов")
	}

	return nil
}

// Logout выполняет выход пользователя
func (am *AuthManager) Logout(ctx context.Context) error {
	// Выполняем выход через HTTP
	if err := am.httpClient.Logout(ctx, am.tokenStore.GetAccessToken()); err != nil {
		// Продолжаем с локальным выходом даже если HTTP запрос не удался
	}

	// Удаляем локальные токены
	if err := am.tokenStore.ClearTokens(); err != nil {
		return errors.Wrap(err, errors.ErrInternal, "ошибка удаления токенов")
	}

	return nil
}

// GetStatus возвращает статус аутентификации
func (am *AuthManager) GetStatus(ctx context.Context) (*store.TokenInfo, error) {
	tokenInfo, err := am.tokenStore.LoadTokens()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrUnauthorized, "токен истек")
	}

	// Проверяем срок действия токена
	if time.Now().After(tokenInfo.ExpiresAt) {
		return nil, errors.New(errors.ErrUnauthorized, "токен истек")
	}

	return tokenInfo, nil
}

// EnsureValidToken проверяет и обновляет токен при необходимости
func (am *AuthManager) EnsureValidToken(ctx context.Context) error {
	_, err := am.GetStatus(ctx)
	return err
}

// GetTokenStore возвращает хранилище токенов
func (am *AuthManager) GetTokenStore() TokenStoreInterface {
	return am.tokenStore
}

// GetLogger возвращает логгер
func (am *AuthManager) GetLogger() logger.Logger {
	// Создаем базовый логгер если нужно
	log, _ := logger.NewLogger("dev", "info", "cli-service", false)
	return log
}

// Close закрывает AuthManager
func (am *AuthManager) Close() error {
	// Закрываем HTTP клиент
	if am.httpClient != nil {
		am.httpClient.Close()
	}

	return nil
}
