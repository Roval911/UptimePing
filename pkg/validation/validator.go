package validation

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Validator предоставляет общие функции валидации
type Validator struct{}

// NewValidator создает новый Validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateRequiredFields проверяет обязательные поля в структуре
func (v *Validator) ValidateRequiredFields(req interface{}, requiredFields map[string]string) error {
	// Используем reflection или type assertion для проверки полей
	// Это базовая реализация, которую можно расширить

	switch r := req.(type) {
	case map[string]interface{}:
		for field, fieldName := range requiredFields {
			if value, exists := r[field]; !exists || value == nil || value == "" {
				return fmt.Errorf("%s is required", fieldName)
			}
		}
	default:
		// Для конкретных типов можно добавить type assertion
		return fmt.Errorf("unsupported request type for validation")
	}

	return nil
}

// ValidateURL проверяет корректность URL
func (v *Validator) ValidateURL(target string, allowedSchemes []string) error {
	if target == "" {
		return fmt.Errorf("target is required")
	}

	parsedURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Проверяем схему
	if len(allowedSchemes) > 0 {
		schemeValid := false
		for _, scheme := range allowedSchemes {
			if parsedURL.Scheme == scheme {
				schemeValid = true
				break
			}
		}
		if !schemeValid {
			return fmt.Errorf("URL must use one of allowed schemes %v, got: %s", allowedSchemes, parsedURL.Scheme)
		}
	}

	// Проверяем хост
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid host")
	}

	// Проверяем, что нет недопустимых символов
	if strings.ContainsAny(target, " \t\n\r") {
		return fmt.Errorf("URL contains invalid whitespace characters")
	}

	return nil
}

// ValidateHostPort проверяет корректность host:port формата
func (v *Validator) ValidateHostPort(target string) error {
	if target == "" {
		return fmt.Errorf("target is required")
	}

	// Проверяем базовый формат
	if strings.ContainsAny(target, " \t\n\r") {
		return fmt.Errorf("target contains invalid whitespace characters")
	}

	// Проверяем, что target не содержит недопустимых схем
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return fmt.Errorf("target should not include http/https scheme")
	}

	return nil
}

// ValidateInterval проверяет корректность интервала
func (v *Validator) ValidateInterval(interval int32, min, max int32) error {
	if interval < min {
		return fmt.Errorf("interval must be at least %d seconds, got: %d", min, interval)
	}
	if interval > max {
		return fmt.Errorf("interval must not exceed %d seconds, got: %d", max, interval)
	}
	return nil
}

// ValidateTimeout проверяет корректность таймаута
func (v *Validator) ValidateTimeout(timeout int32, min, max int32) error {
	if timeout < min {
		return fmt.Errorf("timeout must be at least %d second, got: %d", min, timeout)
	}
	if timeout > max {
		return fmt.Errorf("timeout must not exceed %d seconds, got: %d", max, timeout)
	}
	return nil
}

// ValidateCronExpression выполняет базовую валидацию cron выражения
func (v *Validator) ValidateCronExpression(cronExpr string) error {
	if cronExpr == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}

	// Базовая проверка формата - должно содержать 5 полей, разделенных пробелами
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return fmt.Errorf("cron expression must have exactly 5 fields (minute hour day month weekday), got %d", len(fields))
	}

	// Проверяем, что поля не содержат недопустимых символов
	for i, field := range fields {
		if field == "*" {
			continue // wildcard разрешен
		}

		// Проверяем, что поле состоит только из допустимых символов
		for _, char := range field {
			if !((char >= '0' && char <= '9') || char == ',' || char == '-' || char == '/' || char == '*') {
				return fmt.Errorf("invalid character '%c' in cron expression field %d", char, i+1)
			}
		}
	}

	return nil
}

// ValidateEnum проверяет значение на соответствие enum
func (v *Validator) ValidateEnum(value string, allowedValues []string, fieldName string) error {
	if value == "" {
		return fmt.Errorf("%s is required", fieldName)
	}

	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}

	return fmt.Errorf("invalid %s: %s, allowed values: %v", fieldName, value, allowedValues)
}

// ValidateStringLength проверяет длину строки
func (v *Validator) ValidateStringLength(value, fieldName string, min, max int) error {
	length := len(value)
	if length < min {
		return fmt.Errorf("%s must be at least %d characters, got: %d", fieldName, min, length)
	}
	if length > max {
		return fmt.Errorf("%s must not exceed %d characters, got: %d", fieldName, max, length)
	}
	return nil
}

// ValidateUUID проверяет формат UUID
func (v *Validator) ValidateUUID(uuid string, fieldName string) error {
	if uuid == "" {
		return fmt.Errorf("%s is required", fieldName)
	}

	// Базовая проверка формата UUID (длина и дефисы)
	if len(uuid) != 36 {
		return fmt.Errorf("invalid %s format: must be 36 characters", fieldName)
	}

	if strings.Count(uuid, "-") != 4 {
		return fmt.Errorf("invalid %s format: must contain 4 hyphens", fieldName)
	}

	return nil
}

// ValidateTimestamp проверяет временной штамп
func (v *Validator) ValidateTimestamp(ts time.Time, fieldName string) error {
	if ts.IsZero() {
		return fmt.Errorf("%s cannot be zero", fieldName)
	}

	if ts.After(time.Now().Add(24 * time.Hour)) {
		return fmt.Errorf("%s cannot be more than 24 hours in the future", fieldName)
	}

	return nil
}
