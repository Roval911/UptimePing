package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// TokenInfo представляет информацию о токене
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	UserID      string    `json:"user_id"`
	Email       string    `json:"email"`
	TenantID    string    `json:"tenant_id"`
	TenantName  string    `json:"tenant_name"`
}

// TokenStore управляет хранением токенов
type TokenStore struct {
	encryptionKey []byte
	tokensPath    string
}

// NewTokenStore создает новое хранилище токенов
func NewTokenStore() (*TokenStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("ошибка получения домашней директории: %w", err)
	}

	// Получаем или создаем ключ шифрования
	keyPath := filepath.Join(home, ".uptimeping", ".key")
	encryptionKey, err := getOrCreateEncryptionKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения ключа шифрования: %w", err)
	}

	tokensPath := filepath.Join(home, ".uptimeping", "tokens")

	return &TokenStore{
		encryptionKey: encryptionKey,
		tokensPath:    tokensPath,
	}, nil
}

// getOrCreateEncryptionKey получает или создает ключ шифрования
func getOrCreateEncryptionKey(keyPath string) ([]byte, error) {
	// Если ключ существует, читаем его
	if _, err := os.Stat(keyPath); err == nil {
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения ключа: %w", err)
		}
		return data, nil
	}

	// Создаем директорию, если она не существует
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("ошибка создания директории: %w", err)
	}

	// Генерируем новый ключ
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("ошибка генерации ключа: %w", err)
	}

	// Сохраняем ключ с ограниченными правами доступа
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("ошибка сохранения ключа: %w", err)
	}

	return key, nil
}

// encrypt шифрует данные
func (ts *TokenStore) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(ts.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("ошибка создания cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("ошибка создания GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("ошибка генерации nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt расшифровывает данные
func (ts *TokenStore) decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("ошибка декодирования base64: %w", err)
	}

	block, err := aes.NewCipher(ts.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("ошибка создания cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("ошибка создания GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("недостаточный размер данных")
	}

	nonce := data[:nonceSize]
	ciphertextBytes := data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("ошибка расшифровки: %w", err)
	}

	return string(plaintext), nil
}

// SaveTokens сохраняет токены в зашифрованном виде
func (ts *TokenStore) SaveTokens(tokens *TokenInfo) error {
	// Сериализуем токены
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("ошибка сериализации токенов: %w", err)
	}

	// Шифруем данные
	encrypted, err := ts.encrypt(string(data))
	if err != nil {
		return fmt.Errorf("ошибка шифрования токенов: %w", err)
	}

	// Создаем директорию, если она не существует
	if err := os.MkdirAll(filepath.Dir(ts.tokensPath), 0700); err != nil {
		return fmt.Errorf("ошибка создания директории: %w", err)
	}

	// Сохраняем с ограниченными правами доступа
	if err := os.WriteFile(ts.tokensPath, []byte(encrypted), 0600); err != nil {
		return fmt.Errorf("ошибка сохранения токенов: %w", err)
	}

	return nil
}

// LoadTokens загружает токены из зашифрованного файла
func (ts *TokenStore) LoadTokens() (*TokenInfo, error) {
	// Проверяем существование файла
	if _, err := os.Stat(ts.tokensPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("файл токенов не найден")
	}

	// Читаем зашифрованные данные
	data, err := os.ReadFile(ts.tokensPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла токенов: %w", err)
	}

	// Расшифровываем данные
	decrypted, err := ts.decrypt(string(data))
	if err != nil {
		return nil, fmt.Errorf("ошибка расшифровки токенов: %w", err)
	}

	// Десериализуем токены
	var tokens TokenInfo
	if err := json.Unmarshal([]byte(decrypted), &tokens); err != nil {
		return nil, fmt.Errorf("ошибка десериализации токенов: %w", err)
	}

	return &tokens, nil
}

// HasTokens проверяет наличие сохраненных токенов
func (ts *TokenStore) HasTokens() bool {
	_, err := os.Stat(ts.tokensPath)
	return !os.IsNotExist(err)
}

// IsTokenExpired проверяет, истек ли срок действия токена
func (ts *TokenStore) IsTokenExpired() (bool, error) {
	tokens, err := ts.LoadTokens()
	if err != nil {
		return true, err
	}

	return time.Now().After(tokens.ExpiresAt), nil
}

// ShouldRefreshToken проверяет, нужно ли обновить токен
func (ts *TokenStore) ShouldRefreshToken(threshold time.Duration) (bool, error) {
	tokens, err := ts.LoadTokens()
	if err != nil {
		return true, err
	}

	return time.Until(tokens.ExpiresAt) < threshold, nil
}

// ClearTokens удаляет сохраненные токены
func (ts *TokenStore) ClearTokens() error {
	if err := os.Remove(ts.tokensPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ошибка удаления файла токенов: %w", err)
	}
	return nil
}

// GetAccessToken возвращает access токен
func (ts *TokenStore) GetAccessToken() (string, error) {
	tokens, err := ts.LoadTokens()
	if err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

// GetRefreshToken возвращает refresh токен
func (ts *TokenStore) GetRefreshToken() (string, error) {
	tokens, err := ts.LoadTokens()
	if err != nil {
		return "", err
	}
	return tokens.RefreshToken, nil
}

// GetCurrentTenant возвращает информацию о текущем тенанте
func (ts *TokenStore) GetCurrentTenant() (string, string, error) {
	tokens, err := ts.LoadTokens()
	if err != nil {
		return "", "", err
	}
	return tokens.TenantID, tokens.TenantName, nil
}

// GetUserInfo возвращает информацию о пользователе
func (ts *TokenStore) GetUserInfo() (string, string, error) {
	tokens, err := ts.LoadTokens()
	if err != nil {
		return "", "", err
	}
	return tokens.UserID, tokens.Email, nil
}

// UpdateTokens обновляет информацию о токенах
func (ts *TokenStore) UpdateTokens(accessToken, refreshToken string, expiresAt time.Time) error {
	tokens, err := ts.LoadTokens()
	if err != nil {
		return fmt.Errorf("ошибка загрузки токенов для обновления: %w", err)
	}

	tokens.AccessToken = accessToken
	tokens.RefreshToken = refreshToken
	tokens.ExpiresAt = expiresAt

	return ts.SaveTokens(tokens)
}

// hashPassword хеширует пароль для дополнительной безопасности
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return base64.StdEncoding.EncodeToString(hash[:])
}
