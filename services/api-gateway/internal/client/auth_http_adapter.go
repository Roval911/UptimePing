package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AuthHTTPClientInterface интерфейс для HTTP клиента аутентификации
type AuthHTTPClientInterface interface {
	GetBaseURL() string
	Login(ctx context.Context, email, password string) (*TokenPair, error)
	Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	ValidateToken(ctx context.Context, accessToken string) (*UserInfo, error)
	ValidateAPIKey(ctx context.Context, key, secret string) (*APIKeyClaims, error)
	Logout(ctx context.Context, accessToken string) error
}

// TokenPair структура для хранения пары токенов
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TenantID     string `json:"tenant_id"` // Добавлено
}

// UserInfo содержит информацию о пользователе
type UserInfo struct {
	UserID      string   `json:"user_id"`
	TenantID    string   `json:"tenant_id"`
	Email       string   `json:"email"`
	IsAdmin     bool     `json:"is_admin"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	ExpiresAt   int64    `json:"expires_at"`
}

// APIKeyClaims содержит информацию об API ключе
type APIKeyClaims struct {
	TenantID string `json:"tenant_id"`
	KeyID    string `json:"key_id"`
	IsValid  bool   `json:"is_valid"`
}

// AuthHTTPAdapter адаптирует HTTPAuthClient к интерфейсу AuthService
type AuthHTTPAdapter struct {
	authClient AuthHTTPClientInterface
}

// NewAuthHTTPAdapter создает новый адаптер
func NewAuthHTTPAdapter(authClient AuthHTTPClientInterface) *AuthHTTPAdapter {
	return &AuthHTTPAdapter{
		authClient: authClient,
	}
}

// GetBaseURL возвращает базовый URL
func (a *AuthHTTPAdapter) GetBaseURL() string {
	return a.authClient.GetBaseURL()
}

// Login выполняет вход пользователя через HTTP клиент
func (a *AuthHTTPAdapter) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	httpTokenPair, err := a.authClient.Login(ctx, email, password)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  httpTokenPair.AccessToken,
		RefreshToken: httpTokenPair.RefreshToken,
		TenantID:     httpTokenPair.TenantID, // Добавлено
	}, nil
}

// Register выполняет регистрацию пользователя через HTTP клиент
func (a *AuthHTTPAdapter) Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error) {
	return a.authClient.Register(ctx, email, password, tenantName)
}

// RefreshToken обновляет токен доступа через HTTP клиент
func (a *AuthHTTPAdapter) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	return a.authClient.RefreshToken(ctx, refreshToken)
}

// ValidateToken проверяет валидность токена
func (a *AuthHTTPAdapter) ValidateToken(ctx context.Context, accessToken string) (*UserInfo, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("токен не может быть пустым")
	}

	// Создаем HTTP запрос для валидации токена
	body := map[string]interface{}{
		"access_token": accessToken,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	// Определяем URL для валидации токена
	url := fmt.Sprintf("%s/api/v1/auth/validate", a.authClient.GetBaseURL())

	// Создаем HTTP запрос
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("невалидный токен, статус: %d", resp.StatusCode)
	}

	// Парсим ответ
	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	return &userInfo, nil
}

// ValidateAPIKey проверяет валидность API ключа
func (a *AuthHTTPAdapter) ValidateAPIKey(ctx context.Context, key, secret string) (*APIKeyClaims, error) {
	if key == "" || secret == "" {
		return nil, fmt.Errorf("ключ и секрет не могут быть пустыми")
	}

	// Создаем HTTP запрос для валидации API ключа
	body := map[string]interface{}{
		"key":    key,
		"secret": secret,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	// Определяем URL для валидации API ключа
	url := fmt.Sprintf("%s/api/v1/auth/validate-api-key", a.authClient.GetBaseURL())

	// Создаем HTTP запрос
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("невалидный API ключ, статус: %d", resp.StatusCode)
	}

	// Парсим ответ
	var apiKeyClaims APIKeyClaims
	if err := json.NewDecoder(resp.Body).Decode(&apiKeyClaims); err != nil {
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	return &apiKeyClaims, nil
}

// Logout выполняет выход пользователя
func (a *AuthHTTPAdapter) Logout(ctx context.Context, accessToken string) error {
	if accessToken == "" {
		return fmt.Errorf("токен не может быть пустым")
	}

	// Создаем HTTP запрос для выхода
	body := map[string]interface{}{
		"access_token": accessToken,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	// Определяем URL для выхода
	url := fmt.Sprintf("%s/api/v1/auth/logout", a.authClient.GetBaseURL())

	// Создаем HTTP запрос
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("ошибка выхода, статус: %d", resp.StatusCode)
	}

	return nil
}
