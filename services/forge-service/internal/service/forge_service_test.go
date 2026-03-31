package service

import (
	"context"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"

	"UptimePingPlatform/pkg/logger"
)

func TestGetTemplates(t *testing.T) {
	// Создаем реальный Redis клиент для тестов (можно использовать mock в реальном проекте)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Используем стандартный адрес
	})

	// Создаем logger
	testLogger, _ := logger.NewLogger("test", "info", "forge-service-test", false)

	// Создаем сервис
	service := &forgeService{
		logger:       testLogger,
		redisClient:  redisClient,
		templatesDir: "/tmp/templates",
	}

	// Тестируем получение шаблонов
	templates, err := service.GetTemplates(context.Background(), "http", "go")

	// Проверяем результаты
	assert.NoError(t, err)
	assert.NotEmpty(t, templates)

	// Проверяем, что хотя бы один шаблон найден
	found := false
	for _, template := range templates {
		if template.Type == "http" && template.Language == "go" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected to find at least one HTTP Go template")
}

func TestAddTemplate(t *testing.T) {
	// Создаем logger
	testLogger, _ := logger.NewLogger("test", "info", "forge-service-test", false)

	// Создаем сервис без Redis для простого теста
	service := &forgeService{
		logger:       testLogger,
		redisClient:  nil, // Не используем Redis в этом тесте
		templatesDir: "/tmp/templates",
	}

	// Создаем тестовый шаблон
	template := TemplateInfo{
		Name:        "Test Template",
		Type:        "http",
		Language:    "go",
		Description: "Test template for unit testing",
		Parameters:  map[string]string{"timeout": "10s"},
		Example:     "https://example.com",
	}

	// Тестируем добавление шаблона
	err := service.AddTemplate(context.Background(), template)

	// Проверяем результаты
	assert.NoError(t, err)
}

func TestGenerateDynamicTemplate(t *testing.T) {
	// Создаем logger
	testLogger, _ := logger.NewLogger("test", "info", "forge-service-test", false)

	// Создаем сервис
	service := &forgeService{
		logger: testLogger,
	}

	// Тестируем динамическую генерацию
	params := map[string]interface{}{
		"name":        "Dynamic HTTP Checker",
		"type":        "http",
		"language":    "go",
		"description": "Dynamically generated HTTP checker",
		"parameters":  map[string]string{"timeout": "15s"},
		"example":     "https://dynamic.example.com",
	}

	template, err := service.GenerateDynamicTemplate(context.Background(), params)

	// Проверяем результаты
	assert.NoError(t, err)
	assert.NotNil(t, template)

	// Проверяем поля сгенерированного шаблона
	assert.Equal(t, "Dynamic HTTP Checker", template.Name)
	assert.Equal(t, "http", template.Type)
	assert.Equal(t, "go", template.Language)
	assert.Equal(t, "Dynamically generated HTTP checker", template.Description)
	assert.Equal(t, "15s", template.Parameters["timeout"])
	assert.Equal(t, "https://dynamic.example.com", template.Example)
}

func TestValidateTemplate(t *testing.T) {
	// Создаем сервис
	service := &forgeService{}

	// Тестируем валидный шаблон
	validTemplate := &TemplateInfo{
		Name:        "Valid Template",
		Type:        "http",
		Language:    "go",
		Description: "Valid template description",
		Parameters:  map[string]string{"timeout": "30s"},
		Example:     "https://example.com",
	}

	err := service.validateTemplate(validTemplate)
	assert.NoError(t, err)

	// Тестируем невалидный шаблон (пустое имя)
	invalidTemplate := &TemplateInfo{
		Name:        "",
		Type:        "http",
		Language:    "go",
		Description: "Invalid template",
		Parameters:  map[string]string{"timeout": "30s"},
		Example:     "https://example.com",
	}

	err = service.validateTemplate(invalidTemplate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template name is required")

	// Тестируем шаблон с неподдерживаемым типом
	invalidTypeTemplate := &TemplateInfo{
		Name:        "Invalid Type Template",
		Type:        "unsupported",
		Language:    "go",
		Description: "Template with unsupported type",
		Parameters:  map[string]string{"timeout": "30s"},
		Example:     "https://example.com",
	}

	err = service.validateTemplate(invalidTypeTemplate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported template type")
}
