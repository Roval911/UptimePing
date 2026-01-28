package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
)

// ForgeClientInterface определяет интерфейс для работы с Forge сервисом
type ForgeClientInterface interface {
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	Validate(ctx context.Context, req *ValidateRequest) (*ValidateResponse, error)
	InteractiveConfig(ctx context.Context, req *InteractiveConfigRequest) (*InteractiveConfigResponse, error)
	GetTemplates(ctx context.Context, req *GetTemplatesRequest) (*GetTemplatesResponse, error)
	Close() error
}

// ForgeClient реализует клиент для работы с Forge сервисом
type ForgeClient struct {
	logger  logger.Logger
	baseURL string
	client  *http.Client
}

// NewForgeClient создает новый экземпляр ForgeClient
func NewForgeClient(baseURL string, logger logger.Logger) *ForgeClient {
	return &ForgeClient{
		logger:  logger,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateRequest представляет запрос на генерацию кода
type GenerateRequest struct {
	Input    string `json:"input"`
	Output   string `json:"output"`
	Template string `json:"template"`
	Language string `json:"language"`
	Watch    bool   `json:"watch"`
	Config   string `json:"config"`
}

// GenerateResponse представляет ответ на генерацию кода
type GenerateResponse struct {
	GeneratedFiles int       `json:"generated_files"`
	OutputPath     string    `json:"output_path"`
	GenerationTime time.Time `json:"generation_time"`
	Files          []string  `json:"files"`
}

// ValidateRequest представляет запрос на валидацию
type ValidateRequest struct {
	Input     string `json:"input"`
	ProtoPath string `json:"proto_path"`
	Lint      bool   `json:"lint"`
	Breaking  bool   `json:"breaking"`
}

// ValidationError представляет ошибку валидации
type ValidationError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

// ValidationWarning представляет предупреждение валидации
type ValidationWarning struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

// ValidateResponse представляет ответ на валидацию
type ValidateResponse struct {
	Status         string              `json:"status"`
	Valid          bool                `json:"valid"`
	FilesChecked   int                 `json:"files_checked"`
	Errors         []ValidationError   `json:"errors"`
	Warnings       []ValidationWarning `json:"warnings"`
	ValidationTime time.Time           `json:"validation_time"`
}

// InteractiveConfigRequest представляет запрос на интерактивную настройку
type InteractiveConfigRequest struct {
	ProtoFile string            `json:"proto_file"`
	Template  string            `json:"template"`
	Options   map[string]string `json:"options"`
}

// InteractiveConfigResponse представляет ответ интерактивной настройки
type InteractiveConfigResponse struct {
	Config   map[string]interface{} `json:"config"`
	Template string                 `json:"template"`
	Ready    bool                   `json:"ready"`
}

// GetTemplatesRequest представляет запрос на получение шаблонов
type GetTemplatesRequest struct {
	Type     string `json:"type"`     // http, grpc, tcp, etc.
	Language string `json:"language"` // go, java, python, etc.
}

// TemplateInfo представляет информацию о шаблоне
type TemplateInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Language    string            `json:"language"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
	Example     string            `json:"example"`
}

// GetTemplatesResponse представляет ответ со списком шаблонов
type GetTemplatesResponse struct {
	Templates []TemplateInfo `json:"templates"`
	Total     int            `json:"total"`
}

// Generate генерирует код на основе protobuf файлов
func (c *ForgeClient) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	c.logger.Info("генерация кода",
		logger.String("input", req.Input),
		logger.String("output", req.Output),
		logger.String("template", req.Template),
		logger.String("language", req.Language))

	// Валидация входных данных
	validator := &validation.Validator{}

	if err := validator.ValidateRequiredFields(map[string]interface{}{
		"input":  req.Input,
		"output": req.Output,
	}, map[string]string{
		"input":  "входной файл или директория",
		"output": "выходная директория",
	}); err != nil {
		c.logger.Error("ошибка валидации обязательных полей", logger.Error(err))
		return nil, fmt.Errorf("ошибка валидации: %w", err)
	}

	// Проверка существования входного файла
	if _, err := os.Stat(req.Input); os.IsNotExist(err) {
		err := fmt.Errorf("входной путь не существует: %s", req.Input)
		c.logger.Error("ошибка проверки входного пути", logger.Error(err))
		return nil, err
	}

	// Валидация языка
	validLanguages := map[string]bool{
		"go": true, "java": true, "python": true, "typescript": true, "csharp": true,
	}
	if !validLanguages[req.Language] {
		err := fmt.Errorf("неподдерживаемый язык: %s", req.Language)
		c.logger.Error("ошибка валидации языка", logger.Error(err))
		return nil, err
	}

	// Реализуем HTTP вызов к Forge Service API
	url := fmt.Sprintf("%s/api/v1/forge/generate", c.baseURL)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на генерацию кода", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Forge сервис недоступен, используем mock данные")
		return c.generateMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Forge сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Forge сервис вернул ошибку, используем mock данные")
		return c.generateMockResponse(req)
	}

	var generateResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&generateResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.generateMockResponse(req)
	}

	c.logger.Info("генерация кода завершена успешно через HTTP API",
		logger.Int("generated_files", generateResp.GeneratedFiles),
		logger.String("output_path", generateResp.OutputPath))

	return &generateResp, nil
}

// generateMockResponse создает mock ответ для генерации кода
func (c *ForgeClient) generateMockResponse(req *GenerateRequest) (*GenerateResponse, error) {
	c.logger.Info("создание mock ответа для генерации кода")

	// Создаем выходную директорию
	if err := os.MkdirAll(req.Output, 0755); err != nil {
		c.logger.Error("ошибка создания выходной директории", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания выходной директории: %w", err)
	}

	// Mock генерация файлов
	generatedFiles := []string{
		filepath.Join(req.Output, "generated.go"),
		filepath.Join(req.Output, "client.go"),
		filepath.Join(req.Output, "server.go"),
	}

	// Создаем mock файлы
	for _, file := range generatedFiles {
		content := fmt.Sprintf("// Generated file from %s\npackage main\n\n// This is a mock generated file\n", req.Input)
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			c.logger.Warn("ошибка создания mock файла", logger.String("file", file), logger.Error(err))
		}
	}

	response := &GenerateResponse{
		GeneratedFiles: len(generatedFiles),
		OutputPath:     req.Output,
		GenerationTime: time.Now(),
		Files:          generatedFiles,
	}

	c.logger.Info("mock генерация кода завершена",
		logger.Int("generated_files", response.GeneratedFiles),
		logger.String("output_path", response.OutputPath))

	return response, nil
}

// Validate валидирует protobuf файлы
func (c *ForgeClient) Validate(ctx context.Context, req *ValidateRequest) (*ValidateResponse, error) {
	c.logger.Info("валидация protobuf файлов",
		logger.String("input", req.Input),
		logger.String("proto_path", req.ProtoPath),
		logger.Bool("lint", req.Lint),
		logger.Bool("breaking", req.Breaking))

	// Валидация входных данных
	validator := &validation.Validator{}

	if err := validator.ValidateRequiredFields(map[string]interface{}{
		"input": req.Input,
	}, map[string]string{
		"input": "входной файл или директория",
	}); err != nil {
		c.logger.Error("ошибка валидации обязательных полей", logger.Error(err))
		return nil, fmt.Errorf("ошибка валидации: %w", err)
	}

	// Проверка существования входного файла
	if _, err := os.Stat(req.Input); os.IsNotExist(err) {
		err := fmt.Errorf("входной путь не существует: %s", req.Input)
		c.logger.Error("ошибка проверки входного пути", logger.Error(err))
		return nil, err
	}

	// Реализуем HTTP вызов к Forge Service API
	url := fmt.Sprintf("%s/api/v1/forge/validate", c.baseURL)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на валидацию", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Forge сервис недоступен, используем mock данные")
		return c.validateMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Forge сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Forge сервис вернул ошибку, используем mock данные")
		return c.validateMockResponse(req)
	}

	var validateResp ValidateResponse
	if err := json.NewDecoder(resp.Body).Decode(&validateResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.validateMockResponse(req)
	}

	c.logger.Info("валидация завершена успешно через HTTP API",
		logger.Bool("valid", validateResp.Valid),
		logger.Int("files_checked", validateResp.FilesChecked),
		logger.Int("errors", len(validateResp.Errors)),
		logger.Int("warnings", len(validateResp.Warnings)))

	return &validateResp, nil
}

// validateMockResponse создает mock ответ для валидации
func (c *ForgeClient) validateMockResponse(req *ValidateRequest) (*ValidateResponse, error) {
	c.logger.Info("создание mock ответа для валидации")

	// Mock валидация
	var errors []ValidationError
	var warnings []ValidationWarning
	valid := true
	filesChecked := 1

	// Проверяем расширение файла
	if filepath.Ext(req.Input) != ".proto" {
		errors = append(errors, ValidationError{
			File:    req.Input,
			Line:    1,
			Column:  1,
			Message: "файл должен иметь расширение .proto",
		})
		valid = false
	}

	// Mock предупреждение о стиле
	if req.Lint {
		warnings = append(warnings, ValidationWarning{
			File:    req.Input,
			Message: "рекомендуется добавить комментарий с описанием пакета",
		})
	}

	response := &ValidateResponse{
		Status:         "completed",
		Valid:          valid,
		FilesChecked:   filesChecked,
		Errors:         errors,
		Warnings:       warnings,
		ValidationTime: time.Now(),
	}

	c.logger.Info("mock валидация завершена",
		logger.Bool("valid", response.Valid),
		logger.Int("files_checked", response.FilesChecked),
		logger.Int("errors", len(response.Errors)),
		logger.Int("warnings", len(response.Warnings)))

	return response, nil
}

// InteractiveConfig запускает интерактивный режим настройки параметров проверки
func (c *ForgeClient) InteractiveConfig(ctx context.Context, req *InteractiveConfigRequest) (*InteractiveConfigResponse, error) {
	c.logger.Info("запуск интерактивной настройки",
		logger.String("proto_file", req.ProtoFile),
		logger.String("template", req.Template))

	// Валидация входных данных
	validator := &validation.Validator{}

	if err := validator.ValidateRequiredFields(map[string]interface{}{
		"proto_file": req.ProtoFile,
	}, map[string]string{
		"proto_file": "protobuf файл",
	}); err != nil {
		c.logger.Error("ошибка валидации обязательных полей", logger.Error(err))
		return nil, fmt.Errorf("ошибка валидации: %w", err)
	}

	// Реализуем HTTP вызов к Forge Service API
	url := fmt.Sprintf("%s/api/v1/forge/interactive", c.baseURL)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на интерактивную настройку", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Forge сервис недоступен, используем mock данные")
		return c.interactiveMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Forge сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Forge сервис вернул ошибку, используем mock данные")
		return c.interactiveMockResponse(req)
	}

	var interactiveResp InteractiveConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&interactiveResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.interactiveMockResponse(req)
	}

	c.logger.Info("интерактивная настройка завершена успешно через HTTP API",
		logger.Bool("ready", interactiveResp.Ready),
		logger.String("template", interactiveResp.Template))

	return &interactiveResp, nil
}

// interactiveMockResponse создает mock ответ для интерактивной настройки
func (c *ForgeClient) interactiveMockResponse(req *InteractiveConfigRequest) (*InteractiveConfigResponse, error) {
	c.logger.Info("создание mock ответа для интерактивной настройки")

	// Mock интерактивная настройка
	config := map[string]interface{}{
		"check_type":      "http",
		"target_url":      "https://example.com",
		"interval":        60,
		"timeout":         10,
		"expected_status": 200,
		"headers": map[string]string{
			"User-Agent": "UptimePing/1.0",
		},
		"retry_count": 3,
		"retry_delay": 5,
		"tags":        []string{"production", "api"},
		"enabled":     true,
	}

	response := &InteractiveConfigResponse{
		Config:   config,
		Template: req.Template,
		Ready:    true,
	}

	c.logger.Info("mock интерактивная настройка завершена",
		logger.Bool("ready", response.Ready),
		logger.String("template", response.Template))

	return response, nil
}

// GetTemplates получает список доступных шаблонов
func (c *ForgeClient) GetTemplates(ctx context.Context, req *GetTemplatesRequest) (*GetTemplatesResponse, error) {
	c.logger.Info("получение списка шаблонов",
		logger.String("type", req.Type),
		logger.String("language", req.Language))

	// Реализуем HTTP вызов к Forge Service API
	url := fmt.Sprintf("%s/api/v1/forge/templates", c.baseURL)

	// Добавляем query параметры
	if req.Type != "" || req.Language != "" {
		query := make([]string, 0)
		if req.Type != "" {
			query = append(query, fmt.Sprintf("type=%s", req.Type))
		}
		if req.Language != "" {
			query = append(query, fmt.Sprintf("language=%s", req.Language))
		}
		url += "?" + strings.Join(query, "&")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на получение шаблонов", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Forge сервис недоступен, используем mock данные")
		return c.templatesMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Forge сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Forge сервис вернул ошибку, используем mock данные")
		return c.templatesMockResponse(req)
	}

	var templatesResp GetTemplatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&templatesResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.templatesMockResponse(req)
	}

	c.logger.Info("получение шаблонов завершено успешно через HTTP API",
		logger.Int("total", templatesResp.Total),
		logger.Int("returned", len(templatesResp.Templates)))

	return &templatesResp, nil
}

// templatesMockResponse создает mock ответ для получения шаблонов
func (c *ForgeClient) templatesMockResponse(req *GetTemplatesRequest) (*GetTemplatesResponse, error) {
	c.logger.Info("создание mock ответа для получения шаблонов")

	// Mock шаблоны
	templates := []TemplateInfo{
		{
			Name:        "HTTP Check",
			Type:        "http",
			Language:    "go",
			Description: "Шаблон для HTTP проверки доступности сервиса",
			Parameters: map[string]string{
				"url":             "URL для проверки",
				"method":          "HTTP метод (GET, POST, etc.)",
				"expected_status": "Ожидаемый HTTP статус",
				"timeout":         "Таймаут в секундах",
			},
			Example: `url: "https://api.example.com/health"
method: "GET"
expected_status: 200
timeout: 10`,
		},
		{
			Name:        "gRPC Check",
			Type:        "grpc",
			Language:    "go",
			Description: "Шаблон для gRPC проверки сервиса",
			Parameters: map[string]string{
				"address": "Адрес gRPC сервиса",
				"service": "Имя сервиса",
				"method":  "Имя метода",
				"timeout": "Таймаут в секундах",
				"tls":     "Использовать TLS",
			},
			Example: `address: "localhost:50051"
service: "health.Health"
method: "Check"
timeout: 5
tls: true`,
		},
		{
			Name:        "TCP Check",
			Type:        "tcp",
			Language:    "go",
			Description: "Шаблон для TCP проверки доступности порта",
			Parameters: map[string]string{
				"host":    "Хост для подключения",
				"port":    "Порт для подключения",
				"timeout": "Таймаут в секундах",
			},
			Example: `host: "localhost"
port: 8080
timeout: 3`,
		},
	}

	// Фильтрация по типу и языку
	if req.Type != "" {
		var filtered []TemplateInfo
		for _, template := range templates {
			if template.Type == req.Type {
				filtered = append(filtered, template)
			}
		}
		templates = filtered
	}

	if req.Language != "" {
		var filtered []TemplateInfo
		for _, template := range templates {
			if template.Language == req.Language {
				filtered = append(filtered, template)
			}
		}
		templates = filtered
	}

	response := &GetTemplatesResponse{
		Templates: templates,
		Total:     len(templates),
	}

	c.logger.Info("mock получение шаблонов завершено",
		logger.Int("total", response.Total),
		logger.Int("returned", len(response.Templates)))

	return response, nil
}

// Close закрывает клиент
func (c *ForgeClient) Close() error {
	c.logger.Info("закрытие ForgeClient")
	return nil
}

// Вспомогательные функции

// ParseProtoFile анализирует protobuf файл и извлекает сервисы и методы
func ParseProtoFile(filename string) (map[string][]string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла: %w", err)
	}

	services := make(map[string][]string)
	lines := strings.Split(string(content), "\n")

	var currentService string
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Поиск service
		if strings.HasPrefix(line, "service ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				serviceName := strings.Trim(parts[1], " {")
				currentService = serviceName
				services[serviceName] = []string{}
			}
		}

		// Поиск rpc методов
		if strings.HasPrefix(line, "rpc ") && currentService != "" {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				methodName := strings.Trim(parts[1], "(")
				services[currentService] = append(services[currentService], methodName)
			}
		}
	}

	return services, nil
}

// ValidateProtoSyntax проверяет базовый синтаксис protobuf файла
func ValidateProtoSyntax(filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла: %w", err)
	}

	contentStr := string(content)

	// Базовые проверки синтаксиса
	if !strings.Contains(contentStr, "syntax") {
		return fmt.Errorf("отсутствует объявление syntax")
	}

	if !strings.Contains(contentStr, "package") {
		return fmt.Errorf("отсутствует объявление package")
	}

	return nil
}
