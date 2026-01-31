package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TokenInfo содержит информацию о токенах
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TenantID     string    `json:"tenant_id"`
	TenantName   string    `json:"tenant_name"`
	Email        string    `json:"email"`
}

// TokenStore управляет хранением токенов
type TokenStore struct {
	tokensPath string
}

// NewTokenStore создает новое хранилище токенов
func NewTokenStore() (*TokenStore, error) {
	// Сначала проверяем переменную окружения
	home := os.Getenv("UPTIMEPING_HOME")
	if home == "" {
		// Если переменная не установлена, используем домашнюю директорию
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("ошибка получения домашней директории: %w", err)
		}
	}

	// Создаем директорию если она не существует
	uptimeDir := filepath.Join(home, ".uptimeping")
	if err := os.MkdirAll(uptimeDir, 0700); err != nil {
		return nil, fmt.Errorf("ошибка создания директории %s: %w", uptimeDir, err)
	}

	tokensPath := filepath.Join(uptimeDir, "tokens")

	return &TokenStore{
		tokensPath: tokensPath,
	}, nil
}

// SaveTokens сохраняет токены в файл
func (ts *TokenStore) SaveTokens(tokenInfo *TokenInfo) error {
	// Сериализуем токены
	data, err := json.MarshalIndent(tokenInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации токенов: %w", err)
	}

	// Сохраняем в файл
	if err := os.WriteFile(ts.tokensPath, data, 0600); err != nil {
		return fmt.Errorf("ошибка сохранения токенов: %w", err)
	}

	return nil
}

// LoadTokens загружает токены из файла
func (ts *TokenStore) LoadTokens() (*TokenInfo, error) {
	// Проверяем существует ли файл
	if _, err := os.Stat(ts.tokensPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("файл токенов не найден")
	}

	// Читаем данные
	data, err := os.ReadFile(ts.tokensPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла токенов: %w", err)
	}

	// Десериализуем токены
	var tokenInfo TokenInfo
	if err := json.Unmarshal(data, &tokenInfo); err != nil {
		return nil, fmt.Errorf("ошибка десериализации токенов: %w", err)
	}

	return &tokenInfo, nil
}

// HasTokens проверяет наличие токенов
func (ts *TokenStore) HasTokens() bool {
	_, err := os.Stat(ts.tokensPath)
	return !os.IsNotExist(err)
}

// ClearTokens удаляет файл токенов
func (ts *TokenStore) ClearTokens() error {
	if err := os.Remove(ts.tokensPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ошибка удаления файла токенов: %w", err)
	}
	return nil
}

// GetAccessToken возвращает access токен
func (ts *TokenStore) GetAccessToken() string {
	if tokenInfo, err := ts.LoadTokens(); err == nil {
		return tokenInfo.AccessToken
	}
	return ""
}
