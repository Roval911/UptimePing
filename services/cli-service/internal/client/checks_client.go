package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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

// Check represents a monitoring check
type Check struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Target    string                 `json:"target"`
	Interval  int                    `json:"interval"`
	Timeout   int                    `json:"timeout"`
	Enabled   bool                   `json:"enabled"`
	TenantID  string                 `json:"tenant_id"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
	Status    string                 `json:"status"`
	Tags      []string               `json:"tags"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ChecksClient представляет клиент для взаимодействия с проверками
type ChecksClient struct {
	baseURL    string
	httpClient *http.Client
	tokenStore TokenStoreInterface
}

// NewChecksClient создает новый клиент для проверок
func NewChecksClient(baseURL string, tokenStore TokenStoreInterface) *ChecksClient {
	return &ChecksClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		tokenStore: tokenStore,
	}
}

// extractTokenFromContext извлекает access token из контекста
func (c *ChecksClient) extractTokenFromContext(ctx context.Context) string {
	// Извлекаем из контекстных значений
	if c.tokenStore != nil {
		if tokenInfo, err := c.tokenStore.LoadTokens(); err == nil {
			return tokenInfo.AccessToken
		}
	}

	return ""
}

// ListChecks получает список проверок
func (c *ChecksClient) ListChecks(ctx context.Context) ([]Check, error) {
	token := c.extractTokenFromContext(ctx)
	if token == "" {
		return nil, fmt.Errorf("токен авторизации не найден")
	}

	url := fmt.Sprintf("%s/api/v1/checks", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	var checks []Check
	if err := json.NewDecoder(resp.Body).Decode(&checks); err != nil {
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	return checks, nil
}

// GetCheck получает проверку по ID
func (c *ChecksClient) GetCheck(ctx context.Context, checkID string) (*Check, error) {
	fmt.Printf("DEBUG: GetCheck called with checkID: %s\n", checkID)

	token := c.extractTokenFromContext(ctx)
	if token == "" {
		return nil, fmt.Errorf("токен авторизации не найден")
	}

	fmt.Printf("DEBUG: Token extracted successfully\n")

	url := fmt.Sprintf("%s/api/v1/checks/%s", c.baseURL, checkID)
	fmt.Printf("DEBUG: Making request to: %s\n", url)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	fmt.Printf("DEBUG: Sending HTTP request...\n")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("DEBUG: Response status: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	fmt.Printf("DEBUG: Reading response body...\n")
	var check Check
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	fmt.Printf("DEBUG: Successfully decoded check: %+v\n", check)
	return &check, nil
}

// CreateCheck создает новую проверку
func (c *ChecksClient) CreateCheck(ctx context.Context, check *Check) (*Check, error) {
	token := c.extractTokenFromContext(ctx)
	if token == "" {
		return nil, fmt.Errorf("токен авторизации не найден")
	}

	url := fmt.Sprintf("%s/api/v1/checks", c.baseURL)

	jsonBody, err := json.Marshal(check)
	if err != nil {
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	// Читаем тело ответа один раз
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Пробуем сначала стандартный формат
	var response struct {
		Success bool   `json:"success"`
		Check   Check  `json:"check"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(bodyBytes, &response); err == nil {
		if response.Check.ID != "" || response.Check.Name != "" {
			return &response.Check, nil
		}
	}

	// Пробуем альтернативный формат
	var altResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &altResponse); err == nil {
		check := &Check{
			ID:       getString(altResponse, "id"),
			Name:     getString(altResponse, "name"),
			Type:     getString(altResponse, "type"),
			Target:   getString(altResponse, "target"),
			Interval: getInt(altResponse, "interval"),
			Timeout:  getInt(altResponse, "timeout"),
			Status:   getString(altResponse, "status"),
			Enabled:  true,
		}
		return check, nil
	}

	return nil, fmt.Errorf("не удалось декодировать ответ: %s", string(bodyBytes))
}

// Вспомогательные функции для извлечения данных
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		if num, ok := val.(float64); ok {
			return int(num)
		}
	}
	return 0
}

// UpdateCheck обновляет проверку
func (c *ChecksClient) UpdateCheck(ctx context.Context, checkID string, updates *Check) (*Check, error) {
	token := c.extractTokenFromContext(ctx)
	if token == "" {
		return nil, fmt.Errorf("токен авторизации не найден")
	}

	url := fmt.Sprintf("%s/api/v1/checks/%s", c.baseURL, checkID)

	jsonBody, err := json.Marshal(updates)
	if err != nil {
		return nil, fmt.Errorf("ошибка кодирования запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	var response struct {
		Success bool   `json:"success"`
		Check   Check  `json:"check"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("ошибка декодирования ответа: %w", err)
	}

	return &response.Check, nil
}

// DeleteCheck удаляет проверку
func (c *ChecksClient) DeleteCheck(ctx context.Context, checkID string) error {
	token := c.extractTokenFromContext(ctx)
	if token == "" {
		return fmt.Errorf("токен авторизации не найден")
	}

	url := fmt.Sprintf("%s/api/v1/checks/%s", c.baseURL, checkID)

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ошибка выполнения HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	return nil
}

// Close закрывает клиент
func (c *ChecksClient) Close() error {
	fmt.Printf("Закрытие ChecksClient\n")
	return nil
}
