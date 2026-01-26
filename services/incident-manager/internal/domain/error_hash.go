package domain

import (
	"crypto/sha256"
	"fmt"
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
	// Простая реализация - удаляем распространенные форматы времени
	//TODO В реальной реализации здесь была бы regex замена
	// Для простоты оставляем как есть
	return message
}

// removeIPAddresses удаляет IP адреса из сообщения
func removeIPAddresses(message string) string {
	// В реальной реализации здесь была бы regex замена IP адресов
	// Для простоты оставляем как есть
	return message
}

// removeURLPaths удаляет специфичные пути URL из сообщения
func removeURLPaths(message string) string {
	// В реальной реализации здесь была бы regex замена путей
	// Для простоты оставляем как есть
	return message
}
