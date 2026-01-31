package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"UptimePingPlatform/pkg/logger"
)

// HTTPAuthClient HTTP клиент для AuthService
type HTTPAuthClient struct {
	baseURL string
	client  *http.Client
	logger  logger.Logger
}

// NewHTTPAuthClient создает новый HTTP клиент для AuthService
func NewHTTPAuthClient(baseURL string, timeout time.Duration, logger logger.Logger) (*HTTPAuthClient, error) {
	client := &HTTPAuthClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}

	logger.Info("HTTP клиент для Auth Service создан")

	return client, nil
}

// Login выполняет вход пользователя через HTTP API
func (c *HTTPAuthClient) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	c.logger.Info("попытка входа пользователя через HTTP")

	// Формируем тело запроса
	body := map[string]interface{}{
		"email":    email,
		"password": password,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		c.logger.Error("ошибка кодирования тела запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	// Создаем HTTP запрос
	url := fmt.Sprintf("%s/api/v1/auth/login", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("неверный статус ответа", logger.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	// Парсим ответ
	var tokenPair TokenPair
	if err := json.NewDecoder(resp.Body).Decode(&tokenPair); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	c.logger.Info("пользователь успешно вошел через HTTP")

	return &tokenPair, nil
}

// Register выполняет регистрацию пользователя через HTTP API
func (c *HTTPAuthClient) Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error) {
	c.logger.Info("попытка регистрации пользователя через HTTP",
		logger.String("email", email),
		logger.String("tenant_name", tenantName))

	// Формируем тело запроса
	body := map[string]interface{}{
		"email":       email,
		"password":    password,
		"tenant_name": tenantName,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		c.logger.Error("ошибка кодирования тела запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	// Создаем HTTP запрос - вызываем Auth Service напрямую
	url := fmt.Sprintf("%s/api/v1/auth/register", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.logger.Error("неверный статус ответа", logger.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	// Парсим ответ
	var tokenPair TokenPair
	if err := json.NewDecoder(resp.Body).Decode(&tokenPair); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	c.logger.Info("пользователь успешно зарегистрирован через HTTP")

	return &tokenPair, nil
}

// RefreshToken обновляет токен доступа через HTTP API
func (c *HTTPAuthClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	c.logger.Info("попытка обновления токена через HTTP")

	// Формируем тело запроса
	body := map[string]interface{}{
		"refresh_token": refreshToken,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		c.logger.Error("ошибка кодирования тела запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	// Создаем HTTP запрос
	url := fmt.Sprintf("%s/api/v1/auth/refresh", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("неверный статус ответа", logger.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	// Парсим ответ
	var tokenPair TokenPair
	if err := json.NewDecoder(resp.Body).Decode(&tokenPair); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	c.logger.Info("токен успешно обновлен через HTTP")

	return &tokenPair, nil
}

// ValidateToken проверяет валидность токена
func (c *HTTPAuthClient) ValidateToken(ctx context.Context, accessToken string) (*UserInfo, error) {
	c.logger.Info("выполнение ValidateToken через HTTP")

	body := map[string]interface{}{
		"access_token": accessToken,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		c.logger.Error("ошибка кодирования запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/auth/validate", bytes.NewBuffer(jsonBody))
	if err != nil {
		c.logger.Error("ошибка создания запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("неверный статус ответа", logger.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	// Парсим ответ
	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	c.logger.Info("токен успешно валидирован через HTTP")
	return &userInfo, nil
}

// ValidateAPIKey проверяет валидность API ключа через HTTP API
func (c *HTTPAuthClient) ValidateAPIKey(ctx context.Context, key, secret string) (*APIKeyClaims, error) {
	c.logger.Info("выполнение ValidateAPIKey через HTTP")

	body := map[string]interface{}{
		"key":    key,
		"secret": secret,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		c.logger.Error("ошибка кодирования запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/auth/validate-api-key", bytes.NewBuffer(jsonBody))
	if err != nil {
		c.logger.Error("ошибка создания запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("неверный статус ответа", logger.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	// Парсим ответ
	var apiKeyClaims APIKeyClaims
	if err := json.NewDecoder(resp.Body).Decode(&apiKeyClaims); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	c.logger.Info("API ключ успешно валидирован через HTTP")
	return &apiKeyClaims, nil
}

// Logout выполняет выход пользователя
func (c *HTTPAuthClient) Logout(ctx context.Context, accessToken string) error {
	c.logger.Info("выполнение Logout через HTTP")

	body := map[string]interface{}{
		"access_token": accessToken,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		c.logger.Error("ошибка кодирования запроса", logger.Error(err))
		return fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/auth/logout", bytes.NewBuffer(jsonBody))
	if err != nil {
		c.logger.Error("ошибка создания запроса", logger.Error(err))
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		return fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		c.logger.Error("неверный статус ответа", logger.Int("status", resp.StatusCode))
		return fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	c.logger.Info("выход успешно выполнен через HTTP")
	return nil
}

// GetBaseURL возвращает базовый URL клиента
func (c *HTTPAuthClient) GetBaseURL() string {
	return c.baseURL
}

// Close закрывает соединение
func (c *HTTPAuthClient) Close() error {
	c.logger.Info("закрытие HTTP клиента для Auth Service")
	return nil
}
