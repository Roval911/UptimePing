package apikey

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// KeyPair представляет пару ключей API
type KeyPair struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

// GenerateKeyPair генерирует новую пару ключей API
func GenerateKeyPair() (*KeyPair, error) {
	// Генерируем публичную часть ключа (24 символа)
	keyRaw, err := generateRandomString(24)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	key := "upk_" + keyRaw

	// Генерируем приватную часть ключа (32 символа)
	secretRaw, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}
	secret := "sec_" + secretRaw

	return &KeyPair{
		Key:    key,
		Secret: secret,
	}, nil
}

// HashKey хеширует ключ с использованием bcrypt
func HashKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash key: %w", err)
	}
	return string(hash), nil
}

// ValidateKey проверяет, соответствует ли ключ хешу
func ValidateKey(key, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	return err == nil
}

// FormatKey форматирует ключ для отображения пользователю
func FormatKey(key string) string {
	// Для ключей с префиксом upk_
	if strings.HasPrefix(key, "upk_") {
		keyBody := key[4:] // после "upk_"

		// Если тело ключа слишком короткое (меньше 6 символов), не скрываем
		if len(keyBody) < 6 {
			return key
		}

		// Для upk_ ключей показываем первые 2 и последние 3 символа тела
		firstPart := keyBody[:2]
		lastPart := keyBody[len(keyBody)-3:]

		return "upk_" + firstPart + "***" + lastPart
	}

	// Для всех остальных ключей
	// Если ключ слишком короткий для скрытия (меньше 10 символов), возвращаем как есть
	if len(key) < 10 {
		return key
	}

	// Показываем первые 3 и последние 3 символа всей строки
	firstPart := key[:3]
	lastPart := key[len(key)-3:]

	return firstPart + "***" + lastPart
}

// generateRandomString генерирует случайную строку заданной длины
func generateRandomString(length int) (string, error) {
	// Вычисляем сколько байт нужно для получения нужного количества символов в base64
	byteLength := (length * 6) / 8 // Приблизительный расчет
	if byteLength < 1 {
		byteLength = 1
	}

	// Создаем буфер
	bytes := make([]byte, byteLength)

	// Генерируем случайные байты
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Кодируем в base64 URL безопасный вариант
	encoded := base64.RawURLEncoding.EncodeToString(bytes)

	// Обрезаем до нужной длины
	if len(encoded) > length {
		encoded = encoded[:length]
	}

	return encoded, nil
}

// ExtractKeyFromHeader извлекает ключ из заголовка Authorization
func ExtractKeyFromHeader(authHeader string) (string, string, error) {
	// Удаляем лишние пробелы
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return "", "", fmt.Errorf("empty authorization header")
	}

	// Разделяем заголовок по первому пробелу
	spaceIndex := strings.Index(authHeader, " ")
	if spaceIndex == -1 {
		return "", "", fmt.Errorf("invalid authorization header format")
	}

	authType := strings.ToLower(strings.TrimSpace(authHeader[:spaceIndex]))
	credentials := strings.TrimSpace(authHeader[spaceIndex:])

	// Проверяем тип аутентификации
	if authType != "bearer" && authType != "apikey" {
		return "", "", fmt.Errorf("unsupported authorization type: %s", authType)
	}

	// Проверяем credentials
	if credentials == "" {
		return "", "", fmt.Errorf("empty credentials")
	}

	// Разделяем по первому двоеточию
	colonIndex := strings.Index(credentials, ":")
	if colonIndex == -1 {
		return "", "", fmt.Errorf("invalid key format, expected key:secret")
	}

	key := strings.TrimSpace(credentials[:colonIndex])
	secret := strings.TrimSpace(credentials[colonIndex+1:])

	if key == "" || secret == "" {
		return "", "", fmt.Errorf("empty key or secret")
	}

	return key, secret, nil
}

// ValidateFormat проверяет формат ключа
func ValidateFormat(key, secret string) bool {
	// Проверяем префиксы
	if !strings.HasPrefix(key, "upk_") || !strings.HasPrefix(secret, "sec_") {
		return false
	}

	// Проверяем длину ключей (после префикса)
	if len(key) <= 4 || len(secret) <= 4 { // 4 символа - длина префикса
		return false
	}

	// Проверяем, что ключи содержат только допустимые символы
	// base64 URL безопасные символы: A-Z, a-z, 0-9, -, _
	keyBody := key[4:]       // после "upk_"
	secretBody := secret[4:] // после "sec_"

	for _, c := range keyBody {
		if !isValidBase64URLChar(c) {
			return false
		}
	}

	for _, c := range secretBody {
		if !isValidBase64URLChar(c) {
			return false
		}
	}

	return true
}

// isValidBase64URLChar проверяет, является ли символ допустимым для base64 URL
func isValidBase64URLChar(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' ||
		c == '_'
}
