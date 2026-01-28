package validation

import (
	"fmt"
	"regexp"
	"strings"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/validation"
)

// ForgeValidator валидатор для Forge Service
type ForgeValidator struct {
	validator validation.Validator
}

// NewForgeValidator создает новый валидатор
func NewForgeValidator() *ForgeValidator {
	return &ForgeValidator{
		validator: validation.Validator{},
	}
}

// ValidateProtoContent валидирует содержимое .proto файла
func (v *ForgeValidator) ValidateProtoContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return errors.New(errors.ErrValidation, "proto content cannot be empty")
	}

	// Проверяем наличие обязательных ключевых слов
	requiredKeywords := []string{"syntax", "package", "service"}
	for _, keyword := range requiredKeywords {
		if !strings.Contains(content, keyword) {
			return errors.New(errors.ErrValidation, "missing required keyword: "+keyword)
		}
	}

	// Валидация синтаксиса proto
	if err := v.validateProtoSyntax(content); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "invalid proto syntax")
	}

	return nil
}

// ValidateConfigOptions валидирует опции конфигурации
func (v *ForgeValidator) ValidateConfigOptions(options interface{}) error {
	// Базовая валидация опций
	if options == nil {
		return errors.New(errors.ErrValidation, "config options cannot be nil")
	}

	// Специфическая валидация в зависимости от типа опций
	switch opts := options.(type) {
	case map[string]interface{}:
		// Валидация target host
		if host, exists := opts["target_host"]; !exists || host == "" {
			return errors.New(errors.ErrValidation, "target host cannot be empty")
		}
		
		// Валидация target port
		if port, exists := opts["target_port"]; exists {
			if portStr, ok := port.(float64); ok {
				if portStr < 1 || portStr > 65535 {
					return errors.New(errors.ErrValidation, "target port must be between 1 and 65535")
				}
			}
		}
		
		// Валидация check interval
		if interval, exists := opts["check_interval"]; exists {
			if intervalVal, ok := interval.(float64); ok {
				if intervalVal < 1 || intervalVal > 86400 {
					return errors.New(errors.ErrValidation, "check interval must be between 1 and 86400 seconds")
				}
			}
		}
		
		// Валидация timeout
		if timeout, exists := opts["timeout"]; exists {
			if timeoutVal, ok := timeout.(float64); ok {
				if timeoutVal < 1 || timeoutVal > 300 {
					return errors.New(errors.ErrValidation, "timeout must be between 1 and 300 seconds")
				}
			}
		}
		
		// Валидация tenant ID
		if tenantID, exists := opts["tenant_id"]; exists {
			if tenantIDStr, ok := tenantID.(string); ok {
				if err := v.validator.ValidateStringLength(tenantIDStr, "tenant_id", 1, 100); err != nil {
					return errors.Wrap(err, errors.ErrValidation, "invalid tenant ID")
				}
			}
		}

	default:
		return errors.New(errors.ErrValidation, "unknown config options type")
	}

	return nil
}

// ValidateCodeOptions валидирует опции генерации кода
func (v *ForgeValidator) ValidateCodeOptions(options interface{}) error {
	if options == nil {
		return errors.New(errors.ErrValidation, "code options cannot be nil")
	}

	// Специфическая валидация опций кода
	switch opts := options.(type) {
	case map[string]interface{}:
		// Валидация языка программирования
		if language, exists := opts["language"]; exists {
			if languageStr, ok := language.(string); ok {
				supportedLanguages := []string{"go", "python", "java", "typescript", "rust"}
				if err := v.validator.ValidateEnum(languageStr, supportedLanguages, "language"); err != nil {
					return errors.Wrap(err, errors.ErrValidation, "unsupported language")
				}
			}
		}

		// Валидация фреймворка
		if framework, exists := opts["framework"]; exists {
			if frameworkStr, ok := framework.(string); ok && frameworkStr != "" {
				supportedFrameworks := []string{"grpc", "http", "rest", "graphql"}
				if err := v.validator.ValidateEnum(frameworkStr, supportedFrameworks, "framework"); err != nil {
					return errors.Wrap(err, errors.ErrValidation, "unsupported framework")
				}
			}
		}

		// Валидация шаблона
		if template, exists := opts["template"]; exists {
			if templateStr, ok := template.(string); ok && templateStr != "" {
				supportedTemplates := []string{"basic", "advanced", "production", "testing"}
				if err := v.validator.ValidateEnum(templateStr, supportedTemplates, "template"); err != nil {
					return errors.Wrap(err, errors.ErrValidation, "unsupported template")
				}
			}
		}

	default:
		return errors.New(errors.ErrValidation, "unknown code options type")
	}
	return nil
}

// ValidateTemplateRequest валидирует запрос шаблонов
func (v *ForgeValidator) ValidateTemplateRequest(templateType, language string) error {
	// Валидация типа шаблона
	supportedTypes := []string{"http", "grpc", "tcp", "graphql", "ping"}
	if err := v.validator.ValidateEnum(templateType, supportedTypes, "template_type"); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "unsupported template type")
	}

	// Валидация языка
	supportedLanguages := []string{"go", "python", "java", "typescript", "rust"}
	if language != "" {
		if err := v.validator.ValidateEnum(language, supportedLanguages, "language"); err != nil {
			return errors.Wrap(err, errors.ErrValidation, "unsupported language")
		}
	}

	return nil
}

// validateProtoSyntax валидирует базовый синтаксис proto файла
func (v *ForgeValidator) validateProtoSyntax(content string) error {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Проверяем синтаксис объявления пакета
		if strings.HasPrefix(line, "package") {
			if err := v.validatePackageLine(line); err != nil {
				return fmt.Errorf("line %d: %w", i+1, err)
			}
		}

		// Проверяем синтаксис объявления сервиса
		if strings.HasPrefix(line, "service") {
			if err := v.validateServiceLine(line); err != nil {
				return fmt.Errorf("line %d: %w", i+1, err)
			}
		}

		// Проверяем синтаксис объявления сообщения
		if strings.HasPrefix(line, "message") {
			if err := v.validateMessageLine(line); err != nil {
				return fmt.Errorf("line %d: %w", i+1, err)
			}
		}
	}

	return nil
}

// validatePackageLine валидирует строку объявления пакета
func (v *ForgeValidator) validatePackageLine(line string) error {
	packageRegex := regexp.MustCompile(`^package\s+([a-zA-Z][a-zA-Z0-9_]*)\s*;?\s*$`)
	if !packageRegex.MatchString(line) {
		return errors.New(errors.ErrValidation, "invalid package declaration format")
	}
	return nil
}

// validateServiceLine валидирует строку объявления сервиса
func (v *ForgeValidator) validateServiceLine(line string) error {
	serviceRegex := regexp.MustCompile(`^service\s+([a-zA-Z][a-zA-Z0-9_]*)\s*\{?\s*$`)
	if !serviceRegex.MatchString(line) {
		return errors.New(errors.ErrValidation, "invalid service declaration format")
	}
	return nil
}

// validateMessageLine валидирует строку объявления сообщения
func (v *ForgeValidator) validateMessageLine(line string) error {
	messageRegex := regexp.MustCompile(`^message\s+([a-zA-Z][a-zA-Z0-9_]*)\s*\{?\s*$`)
	if !messageRegex.MatchString(line) {
		return errors.New(errors.ErrValidation, "invalid message declaration format")
	}
	return nil
}

// ValidateServiceName валидирует имя сервиса
func (v *ForgeValidator) ValidateServiceName(name string) error {
	if err := v.validator.ValidateStringLength(name, "service_name", 1, 100); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "invalid service name")
	}

	// Проверяем, что имя содержит только допустимые символы
	serviceNameRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	if !serviceNameRegex.MatchString(name) {
		return errors.New(errors.ErrValidation, "service name can only contain letters, numbers and underscores")
	}

	return nil
}

// ValidateMethodName валидирует имя метода
func (v *ForgeValidator) ValidateMethodName(name string) error {
	if err := v.validator.ValidateStringLength(name, "method_name", 1, 100); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "invalid method name")
	}

	// Проверяем, что имя содержит только допустимые символы
	methodNameRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	if !methodNameRegex.MatchString(name) {
		return errors.New(errors.ErrValidation, "method name can only contain letters, numbers and underscores")
	}

	return nil
}

// ValidateMessageName валидирует имя сообщения
func (v *ForgeValidator) ValidateMessageName(name string) error {
	if err := v.validator.ValidateStringLength(name, "message_name", 1, 100); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "invalid message name")
	}

	// Проверяем, что имя содержит только допустимые символы
	messageNameRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	if !messageNameRegex.MatchString(name) {
		return errors.New(errors.ErrValidation, "message name can only contain letters, numbers and underscores")
	}

	return nil
}

// ValidateFileName валидирует имя файла
func (v *ForgeValidator) ValidateFileName(name string) error {
	if err := v.validator.ValidateStringLength(name, "file_name", 1, 255); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "invalid file name")
	}

	// Проверяем расширение файла
	if !strings.HasSuffix(name, ".proto") {
		return errors.New(errors.ErrValidation, "file must have .proto extension")
	}

	// Проверяем, что имя файла содержит только допустимые символы
	fileNameRegex := regexp.MustCompile(`^[a-zA-Z0-9_.-]+\.proto$`)
	if !fileNameRegex.MatchString(name) {
		return errors.New(errors.ErrValidation, "file name can only contain letters, numbers, dots, hyphens and underscores")
	}

	return nil
}
