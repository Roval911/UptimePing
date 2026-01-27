package domain

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

// generateErrorHash генерирует хеш для дедупликации ошибок
func generateErrorHash(errorMessage string) string {
	// Нормализуем сообщение об ошибке для дедупликации
	normalized := normalizeErrorMessage(errorMessage)

	// Генерируем SHA256 хеш
	hash := sha256.Sum256([]byte(normalized))

	// Возвращаем первые 16 символов хеша для компактности
	return fmt.Sprintf("%x", hash)[:16]
}

// normalizeErrorMessage нормализует сообщение об ошибке
func normalizeErrorMessage(errorMessage string) string {
	// Приводим к нижнему регистру
	normalized := strings.ToLower(errorMessage)

	// Удаляем лишние пробелы
	normalized = strings.TrimSpace(normalized)

	// Заменяем множественные пробелы на один
	for strings.Contains(normalized, "  ") {
		normalized = strings.ReplaceAll(normalized, "  ", " ")
	}

	// Удаляем специфичные для времени значения
	normalized = removeTimestamps(normalized)

	// Удаляем специфичные для IP адресов значения
	normalized = removeIPAddresses(normalized)

	// Удаляем специфичные для URL пути значения
	normalized = removeURLPaths(normalized)

	return normalized
}

// removeTimestamps удаляет временные метки из сообщения
func removeTimestamps(message string) string {
	// Regex паттерны для различных форматов временных меток
	timestampPatterns := []*regexp.Regexp{
		// ISO 8601: 2023-12-25T14:30:45Z, 2023-12-25T14:30:45+03:00
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:\d{2})`),
		// RFC 3339: 2023-12-25 14:30:45, 2023-12-25 14:30:45+03:00
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\s*[+-]\d{2}:\d{2})?`),
		// Дата и время: 25/12/2023 14:30:45, 12/25/2023 14:30:45
		regexp.MustCompile(`\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2}`),
		// Время: 14:30:45, 14:30
		regexp.MustCompile(`\d{1,2}:\d{2}(?::\d{2})?`),
		// Unix timestamp в миллисекундах: 1703506645123
		regexp.MustCompile(`\b\d{10,13}\b`),
		// Месяц день, год: Dec 25, 2023
		regexp.MustCompile(`\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2},?\s+\d{4}\b`),
		// День месяц: 25 Dec 2023
		regexp.MustCompile(`\b\d{1,2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{4}\b`),
	}

	result := message
	for _, pattern := range timestampPatterns {
		result = pattern.ReplaceAllString(result, "TIMESTAMP")
	}

	return result
}

// removeIPAddresses удаляет IP адреса из сообщения
func removeIPAddresses(message string) string {
	// Regex паттерны для различных форматов IP адресов
	ipPatterns := []*regexp.Regexp{
		// IPv4 адреса: 192.168.1.1, 10.0.0.1
		regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		// IPv6 адреса: 2001:0db8:85a3:0000:0000:8a2e:0370:7334
		regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}\b`),
		// IPv6 сокращенный формат: ::1, fe80::1
		regexp.MustCompile(`\b(?:[0-9a-fA-F]{1,4}:){0,6}::[0-9a-fA-F]{1,4}\b`),
		// IPv6 с портами: [2001:db8::1]:8080
		regexp.MustCompile(`\[[0-9a-fA-F:]+\](?::\d{1,3})?`),
	}

	result := message
	for _, pattern := range ipPatterns {
		result = pattern.ReplaceAllString(result, "IP_ADDRESS")
	}

	return result
}

// removeURLPaths удаляет специфичные пути URL из сообщения
func removeURLPaths(message string) string {
	// Regex паттерны для различных форматов URL и путей
	urlPatterns := []*regexp.Regexp{
		// URL с протоколом: https://example.com/api/v1/users/123
		regexp.MustCompile(`https?://[^\s/$]+/[^\s]*`),
		// URL без протокола: example.com/api/v1/users/123
		regexp.MustCompile(`\b[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}(?:/[^\s]*)`),
		// Путь с UUID: /api/v1/users/550e8400-e29b-41d4-a716-446655440000
		regexp.MustCompile(`/[^\s]*/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}(?:/[^\s]*)?`),
		// Путь с ID: /api/v1/users/12345
		regexp.MustCompile(`/[^\s]*/\d+(?:/[^\s]*)?`),
		// Путь с параметрами: /api/v1/users?name=test&age=25
		regexp.MustCompile(`/[^\s]*\?[^\s]*`),
		// Путь с якорем: /api/v1/users#section1
		regexp.MustCompile(`/[^\s]*#[^\s]*`),
		// Просто путь: /api/v1/users
		regexp.MustCompile(`/[a-zA-Z0-9/_-]+`),
	}

	result := message
	for _, pattern := range urlPatterns {
		result = pattern.ReplaceAllString(result, "URL_PATH")
	}

	return result
}
