package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TokenPair представляет пару токенов
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TenantID     string `json:"tenant_id"`
	TenantName   string `json:"tenant_name"`
}

// HTTPAuthClient HTTP клиент для AuthService
type HTTPAuthClient struct {
	baseURL string
	client  *http.Client
}

// NewAuthHTTPClient создает новый HTTP клиент для Auth Service
func NewAuthHTTPClient(baseURL string) (*HTTPAuthClient, error) {
	client := &HTTPAuthClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	fmt.Printf("HTTP клиент для Auth Service создан, base_url: %s\n", baseURL)

	return client, nil
}

// Login выполняет вход пользователя через HTTP API
func (c *HTTPAuthClient) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	fmt.Printf("Попытка входа пользователя через HTTP: %s\n", email)

	// Формируем тело запроса
	body := map[string]interface{}{
		"email":    email,
		"password": password,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	// Создаем HTTP запрос
	url := fmt.Sprintf("%s/api/v1/auth/login", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	// Выполняем запрос
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	// Парсим ответ
	var tokenPair TokenPair
	if err := json.NewDecoder(resp.Body).Decode(&tokenPair); err != nil {
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	fmt.Printf("Пользователь успешно вошел через HTTP\n")

	return &tokenPair, nil
}

// Logout выполняет выход пользователя через HTTP API
func (c *HTTPAuthClient) Logout(ctx context.Context, accessToken string) error {
	fmt.Printf("Попытка выхода пользователя через HTTP\n")

	// Формируем тело запроса
	requestBody := map[string]interface{}{
		"access_token": accessToken,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	// Создаем HTTP запрос
	url := fmt.Sprintf("%s/api/v1/auth/logout", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	// Выполняем запрос
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ошибка выполнения HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	fmt.Printf("Выход выполнен успешно через HTTP\n")
	return nil
}

// Close закрывает HTTP клиент
func (c *HTTPAuthClient) Close() error {
	fmt.Printf("Закрытие HTTP клиента для Auth Service\n")
	return nil
}
