package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/forge-service/internal/validation"
)

// ForgeService предоставляет методы для работы с .proto файлами
type ForgeService interface {
	// ParseProto парсит .proto файл и возвращает информацию о сервисе
	ParseProto(ctx context.Context, protoContent, fileName string) (*ForgeServiceInfo, bool, []string, error)

	// GenerateConfig генерирует конфигурацию проверки из .proto файла
	GenerateConfig(ctx context.Context, protoContent string, options *ConfigOptions) (string, *CheckConfig, error)

	// GenerateCode генерирует код для проверки gRPC методов
	GenerateCode(ctx context.Context, protoContent string, options *CodeOptions) (string, string, string, error)

	// ValidateProto проверяет валидность .proto файла
	ValidateProto(ctx context.Context, protoContent string) (bool, []string, []string, error)

	// GetTemplates возвращает доступные шаблоны для генерации кода
	GetTemplates(ctx context.Context, templateType, language string) ([]TemplateInfo, error)

	// AddTemplate добавляет пользовательский шаблон
	AddTemplate(ctx context.Context, template TemplateInfo) error

	// GenerateDynamicTemplate динамически генерирует шаблон на основе параметров
	GenerateDynamicTemplate(ctx context.Context, params map[string]interface{}) (*TemplateInfo, error)
}

// ForgeServiceInfo содержит информацию о сервисе из .proto файла
type ForgeServiceInfo struct {
	PackageName string             `json:"package_name"`
	ServiceName string             `json:"service_name"`
	Methods     []ForgeMethodInfo  `json:"methods"`
	Messages    []ForgeMessageInfo `json:"messages"`
}

// ForgeMethodInfo содержит информацию о методе
type ForgeMethodInfo struct {
	Name       string `json:"name"`
	InputType  string `json:"input_type"`
	OutputType string `json:"output_type"`
	HttpMethod string `json:"http_method"`
	HttpPath   string `json:"http_path"`
}

// ForgeMessageInfo содержит информацию о сообщении
type ForgeMessageInfo struct {
	Name   string           `json:"name"`
	Fields []ForgeFieldInfo `json:"fields"`
}

// ForgeFieldInfo содержит информацию о поле
type ForgeFieldInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Number   int    `json:"number"`
	Repeated bool   `json:"repeated"`
}

// ConfigOptions содержит опции генерации конфигурации
type ConfigOptions struct {
	TargetHost    string            `json:"target_host"`
	TargetPort    int               `json:"target_port"`
	CheckInterval int               `json:"check_interval"`
	Timeout       int               `json:"timeout"`
	TenantID      string            `json:"tenant_id"`
	Metadata      map[string]string `json:"metadata"`
}

// CheckConfig содержит конфигурацию проверки
type CheckConfig struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Target   string `json:"target"`
	Interval *int   `json:"interval,omitempty"`
	Timeout  *int   `json:"timeout,omitempty"`
	Config   string `json:"config"`
}

// CodeOptions содержит опции генерации кода
type CodeOptions struct {
	Language  string `json:"language"`
	Framework string `json:"framework"`
	Template  string `json:"template"`
}

// forgeService реализация ForgeService
type forgeService struct {
	logger        logger.Logger
	protoParser   *ProtoParser
	codeGenerator *CodeGenerator
	validator     *validation.ForgeValidator
	redisClient   *redis.Client
	templatesDir  string
}

// NewForgeService создает новый экземпляр ForgeService
func NewForgeService(logger logger.Logger, protoParser *ProtoParser, codeGenerator *CodeGenerator, validator *validation.ForgeValidator, redisClient *redis.Client, templatesDir string) ForgeService {
	if templatesDir == "" {
		templatesDir = "./templates"
	}

	return &forgeService{
		logger:        logger,
		protoParser:   protoParser,
		codeGenerator: codeGenerator,
		validator:     validator,
		redisClient:   redisClient,
		templatesDir:  templatesDir,
	}
}

// ParseProto парсит .proto файл и возвращает информацию о сервисе
func (s *forgeService) ParseProto(ctx context.Context, protoContent, fileName string) (*ForgeServiceInfo, bool, []string, error) {
	s.logger.Info("Parsing proto file",
		logger.String("file_name", fileName),
		logger.Int("content_length", len(protoContent)))

	// Валидация содержимого
	if err := s.validator.ValidateProtoContent(protoContent); err != nil {
		s.logger.Error("Proto validation failed",
			logger.String("file_name", fileName),
			logger.Error(err))
		return nil, false, nil, err
	}

	// Используем существующий парсер для извлечения информации
	services, err := s.protoParser.ParseProtoContent(protoContent)
	if err != nil {
		s.logger.Error("Failed to parse proto content", logger.Error(err))
		return nil, false, nil, err
	}

	if len(services) == 0 {
		warnings := []string{"No services found in proto file"}
		return nil, true, warnings, nil
	}

	// Берем первый сервис для простоты
	service := services[0]

	// Конвертируем методы
	methods := make([]ForgeMethodInfo, 0, len(service.Methods))
	for _, method := range service.Methods {
		methods = append(methods, ForgeMethodInfo{
			Name:       method.Name,
			InputType:  method.InputType,
			OutputType: method.OutputType,
		})
	}

	// Получаем сообщения из парсера
	messages := s.protoParser.GetMessages()
	messageInfos := make([]ForgeMessageInfo, 0, len(messages))
	for _, msg := range messages {
		fields := make([]ForgeFieldInfo, 0, len(msg.Fields))
		for _, field := range msg.Fields {
			fields = append(fields, ForgeFieldInfo{
				Name:     field.Name,
				Type:     field.Type,
				Number:   int(field.Number),
				Repeated: false, // По умолчанию не repeated
			})
		}
		messageInfos = append(messageInfos, ForgeMessageInfo{
			Name:   msg.Name,
			Fields: fields,
		})
	}

	serviceInfo := &ForgeServiceInfo{
		PackageName: service.Package,
		ServiceName: service.Name,
		Methods:     methods,
		Messages:    messageInfos,
	}

	s.logger.Info("Proto parsed successfully",
		logger.String("service_name", serviceInfo.ServiceName),
		logger.String("package_name", serviceInfo.PackageName),
		logger.Int("methods_count", len(methods)),
		logger.Int("messages_count", len(messageInfos)))

	return serviceInfo, true, nil, nil
}

// GenerateConfig генерирует конфигурацию проверки из .proto файла
func (s *forgeService) GenerateConfig(ctx context.Context, protoContent string, options *ConfigOptions) (string, *CheckConfig, error) {
	s.logger.Info("Generating config from proto",
		logger.Int("proto_length", len(protoContent)),
		logger.Bool("has_options", options != nil))

	// Валидация опций
	if options == nil {
		options = &ConfigOptions{
			TargetHost:    "localhost",
			TargetPort:    50051,
			CheckInterval: 60,
			Timeout:       5,
			TenantID:      "default",
			Metadata:      make(map[string]string),
		}
	}

	// Установка значений по умолчанию
	if options.TargetHost == "" {
		options.TargetHost = "localhost"
	}
	if options.TargetPort == 0 {
		options.TargetPort = 50051
	}
	if options.CheckInterval == 0 {
		options.CheckInterval = 60
	}
	if options.Timeout == 0 {
		options.Timeout = 5
	}
	if options.TenantID == "" {
		options.TenantID = "default"
	}
	if options.Metadata == nil {
		options.Metadata = make(map[string]string)
	}

	// Парсим proto для получения информации о сервисе
	serviceInfo, _, _, err := s.ParseProto(ctx, protoContent, "")
	if err != nil {
		return "", nil, err
	}

	if serviceInfo == nil || len(serviceInfo.Methods) == 0 {
		return "", nil, fmt.Errorf("no methods found in proto file")
	}

	// Создаем конфигурацию для первого метода
	method := serviceInfo.Methods[0]

	// Определяем тип проверки на основе метода
	checkType := "grpc" // По умолчанию для gRPC сервисов
	if method.HttpMethod != "" {
		checkType = "http"
	}

	// Формируем target с валидными значениями
	target := fmt.Sprintf("%s:%d", options.TargetHost, options.TargetPort)
	if options.TargetHost == "" {
		target = fmt.Sprintf("localhost:%d", options.TargetPort)
	}

	// Создаем YAML конфигурацию с валидными значениями
	configYaml := fmt.Sprintf(`name: %s
type: %s
target: %s
interval: %d
timeout: %d
tenant_id: %s
metadata:
  service_name: %s
  method_name: %s
  input_type: %s
  output_type: %s
`,
		method.Name,
		checkType,
		target,
		options.CheckInterval,
		options.Timeout,
		options.TenantID,
		serviceInfo.ServiceName,
		method.Name,
		method.InputType,
		method.OutputType,
	)

	// Создаем CheckConfig с валидными значениями (используем указатели)
	checkConfig := &CheckConfig{
		Name:     method.Name,
		Type:     checkType,
		Target:   target,
		Interval: &options.CheckInterval, // Теперь указатель на значение
		Timeout:  &options.Timeout,       // Теперь указатель на значение
		Config:   fmt.Sprintf("service_name: %s\nmethod_name: %s", serviceInfo.ServiceName, method.Name),
	}

	s.logger.Info("Config generated successfully",
		logger.String("check_name", checkConfig.Name),
		logger.String("check_type", checkConfig.Type),
		logger.String("target", checkConfig.Target),
		logger.Int("interval", *checkConfig.Interval),
		logger.Int("timeout", *checkConfig.Timeout))

	return configYaml, checkConfig, nil
}

// GenerateCode генерирует код для проверки gRPC методов
func (s *forgeService) GenerateCode(ctx context.Context, protoContent string, options *CodeOptions) (string, string, string, error) {
	s.logger.Info("Generating code from proto",
		logger.Int("proto_length", len(protoContent)),
		logger.String("language", options.Language),
		logger.String("framework", options.Framework))

	// Валидация опций
	if options == nil {
		options = &CodeOptions{
			Language:  "go",
			Framework: "grpc",
			Template:  "checker",
		}
	}

	// Установка значений по умолчанию
	if options.Language == "" {
		options.Language = "go"
	}
	if options.Framework == "" {
		options.Framework = "grpc"
	}
	if options.Template == "" {
		options.Template = "checker"
	}

	// Парсим proto для получения информации о сервисе
	serviceInfo, _, _, err := s.ParseProto(ctx, protoContent, "")
	if err != nil {
		return "", "", "", err
	}

	if serviceInfo == nil || len(serviceInfo.Methods) == 0 {
		return "", "", "", fmt.Errorf("no methods found in proto file")
	}

	// Генерируем код для Go
	language := options.Language
	if language == "" {
		language = "go"
	}

	// Генерируем имя файла с валидными значениями
	filename := fmt.Sprintf("%s_checker.go", serviceInfo.ServiceName)
	if serviceInfo.ServiceName == "" {
		filename = "service_checker.go"
	}

	// Генерируем базовый код для gRPC checker
	template := `package checkers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type %sChecker struct {
	target    string
	timeout   time.Duration
}

func New%sChecker(target string, timeout time.Duration) *%sChecker {
	return &%sChecker{
		target:  target,
		timeout: timeout,
	}
}

func (c *%sChecker) Execute(ctx context.Context) error {
	conn, err := grpc.DialContext(ctx, c.target, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to %%s: %%w", c.target, err)
	}
	defer conn.Close()

	// Health check
	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %%w", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("service is not healthy: %%v", resp.Status)
	}

	return nil
}
`

	// Используем валидное имя сервиса
	serviceName := serviceInfo.ServiceName
	if serviceName == "" {
		serviceName = "Service"
	}

	code := fmt.Sprintf(template,
		serviceName,
		serviceName,
		serviceName,
		serviceName,
		serviceName)

	s.logger.Info("Code generated successfully",
		logger.String("filename", filename),
		logger.String("language", language),
		logger.String("service_name", serviceName),
		logger.Int("code_length", len(code)))

	return code, filename, language, nil
}

// ValidateProto проверяет валидность .proto файла
func (s *forgeService) ValidateProto(ctx context.Context, protoContent string) (bool, []string, []string, error) {
	s.logger.Info("Validating proto file",
		logger.Int("content_length", len(protoContent)))

	// Используем существующий парсер для валидации
	_, err := s.protoParser.ParseProtoContent(protoContent)
	if err != nil {
		errors := []string{err.Error()}
		s.logger.Error("Proto validation failed", logger.Error(err))
		return false, errors, nil, nil
	}

	// Дополнительная валидация
	if len(protoContent) < 10 {
		errors := []string{"Proto content too short"}
		return false, errors, nil, nil
	}

	// Проверяем наличие основных ключевых слов
	requiredKeywords := []string{"syntax", "package", "service"}
	warnings := []string{}
	for _, keyword := range requiredKeywords {
		if !contains(protoContent, keyword) {
			warnings = append(warnings, fmt.Sprintf("Missing keyword: %s", keyword))
		}
	}

	s.logger.Info("Proto validation completed",
		logger.Bool("is_valid", true),
		logger.Int("warnings_count", len(warnings)))

	return true, nil, warnings, nil
}

// TemplateInfo представляет информацию о шаблоне для генерации кода
type TemplateInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Language    string            `json:"language"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
	Example     string            `json:"example"`
}

// GetTemplates возвращает доступные шаблоны для генерации кода
func (s *forgeService) GetTemplates(ctx context.Context, templateType, language string) ([]TemplateInfo, error) {
	s.logger.Info("Getting templates",
		logger.String("type", templateType),
		logger.String("language", language))

	// Сначала пытаемся загрузить из кеша Redis
	cachedTemplates, err := s.getTemplatesFromCache(ctx, templateType, language)
	if err == nil && cachedTemplates != nil {
		s.logger.Info("Templates retrieved from cache", logger.Int("count", len(cachedTemplates)))
		return cachedTemplates, nil
	}

	// Если в кеше нет, загружаем из файловой системы
	templates, err := s.loadTemplatesFromFileSystem(ctx, templateType, language)
	if err != nil {
		s.logger.Error("Failed to load templates from file system", logger.Error(err))
		// Возвращаем базовые шаблоны как fallback
		templates = s.getDefaultTemplates(templateType, language)
	}

	// Валидируем шаблоны перед использованием
	validatedTemplates := make([]TemplateInfo, 0, len(templates))
	for _, template := range templates {
		if err := s.validateTemplate(&template); err != nil {
			s.logger.Warn("Invalid template skipped",
				logger.String("name", template.Name),
				logger.Error(err))
			continue
		}
		validatedTemplates = append(validatedTemplates, template)
	}

	// Кешируем валидированные шаблоны
	if len(validatedTemplates) > 0 {
		if err := s.cacheTemplates(ctx, templateType, language, validatedTemplates); err != nil {
			s.logger.Warn("Failed to cache templates", logger.Error(err))
		}
	}

	s.logger.Info("Templates retrieved successfully", logger.Int("count", len(validatedTemplates)))
	return validatedTemplates, nil
}

// getTemplatesFromCache загружает шаблоны из Redis кеша
func (s *forgeService) getTemplatesFromCache(ctx context.Context, templateType, language string) ([]TemplateInfo, error) {
	if s.redisClient == nil {
		return nil, fmt.Errorf("redis client not available")
	}

	cacheKey := fmt.Sprintf("templates:%s:%s", templateType, language)
	data, err := s.redisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Шаблоны не найдены в кеше
		}
		return nil, fmt.Errorf("failed to get templates from cache: %w", err)
	}

	var templates []TemplateInfo
	if err := json.Unmarshal([]byte(data), &templates); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached templates: %w", err)
	}

	return templates, nil
}

// loadTemplatesFromFileSystem загружает шаблоны из файловой системы
func (s *forgeService) loadTemplatesFromFileSystem(ctx context.Context, templateType, language string) ([]TemplateInfo, error) {
	var templates []TemplateInfo

	// Формируем путь к директории шаблонов
	templateDir := filepath.Join(s.templatesDir, templateType, language)

	// Проверяем существование директории
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		s.logger.Info("Template directory not found", logger.String("path", templateDir))
		return nil, fmt.Errorf("template directory not found: %s", templateDir)
	}

	// Читаем файлы шаблонов
	files, err := ioutil.ReadDir(templateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read template directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Загружаем только .json файлы
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		templatePath := filepath.Join(templateDir, file.Name())
		template, err := s.loadTemplateFromFile(templatePath)
		if err != nil {
			s.logger.Warn("Failed to load template file",
				logger.String("file", file.Name()),
				logger.Error(err))
			continue
		}

		// Фильтруем по типу и языку если указаны
		if (templateType == "" || template.Type == templateType) &&
			(language == "" || template.Language == language) {
			templates = append(templates, *template)
		}
	}

	return templates, nil
}

// loadTemplateFromFile загружает один шаблон из файла
func (s *forgeService) loadTemplateFromFile(filePath string) (*TemplateInfo, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	var template TemplateInfo
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template: %w", err)
	}

	return &template, nil
}

// getDefaultTemplates возвращает базовые шаблоны как fallback
func (s *forgeService) getDefaultTemplates(templateType, language string) []TemplateInfo {
	// Базовые шаблоны для разных типов проверок
	templates := []TemplateInfo{
		{
			Name:        "HTTP Checker",
			Type:        "http",
			Language:    "go",
			Description: "HTTP checker for monitoring web endpoints",
			Parameters:  map[string]string{"timeout": "30s", "interval": "60s"},
			Example:     "https://example.com",
		},
		{
			Name:        "gRPC Checker",
			Type:        "grpc",
			Language:    "go",
			Description: "gRPC checker for monitoring gRPC services",
			Parameters:  map[string]string{"timeout": "10s", "service": "example"},
			Example:     "localhost:50051",
		},
		{
			Name:        "TCP Checker",
			Type:        "tcp",
			Language:    "go",
			Description: "TCP checker for monitoring TCP ports",
			Parameters:  map[string]string{"timeout": "5s", "port": "80"},
			Example:     "example.com:80",
		},
		{
			Name:        "GraphQL Checker",
			Type:        "graphql",
			Language:    "go",
			Description: "GraphQL checker for monitoring GraphQL endpoints",
			Parameters:  map[string]string{"timeout": "15s", "query": "health"},
			Example:     "https://api.example.com/graphql",
		},
		{
			Name:        "Ping Checker",
			Type:        "ping",
			Language:    "go",
			Description: "Ping checker for monitoring host availability",
			Parameters:  map[string]string{"timeout": "3s", "count": "3"},
			Example:     "example.com",
		},
	}

	// Фильтруем по типу и языку если указаны
	filtered := make([]TemplateInfo, 0)
	for _, template := range templates {
		if (templateType == "" || template.Type == templateType) &&
			(language == "" || template.Language == language) {
			filtered = append(filtered, template)
		}
	}

	return filtered
}

// validateTemplate валидирует шаблон перед использованием
func (s *forgeService) validateTemplate(template *TemplateInfo) error {
	// Проверяем обязательные поля
	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if template.Type == "" {
		return fmt.Errorf("template type is required")
	}
	if template.Language == "" {
		return fmt.Errorf("template language is required")
	}
	if template.Description == "" {
		return fmt.Errorf("template description is required")
	}

	// Проверяем поддерживаемые типы
	supportedTypes := map[string]bool{
		"http": true, "grpc": true, "tcp": true,
		"graphql": true, "ping": true,
	}
	if !supportedTypes[template.Type] {
		return fmt.Errorf("unsupported template type: %s", template.Type)
	}

	// Проверяем поддерживаемые языки
	supportedLanguages := map[string]bool{
		"go": true, "python": true, "javascript": true,
		"java": true, "typescript": true,
	}
	if !supportedLanguages[template.Language] {
		return fmt.Errorf("unsupported template language: %s", template.Language)
	}

	// Валидируем параметры
	if template.Parameters == nil {
		return fmt.Errorf("template parameters are required")
	}

	// Проверяем пример
	if template.Example == "" {
		return fmt.Errorf("template example is required")
	}

	return nil
}

// cacheTemplates кеширует шаблоны в Redis
func (s *forgeService) cacheTemplates(ctx context.Context, templateType, language string, templates []TemplateInfo) error {
	if s.redisClient == nil {
		return fmt.Errorf("redis client not available")
	}

	cacheKey := fmt.Sprintf("templates:%s:%s", templateType, language)

	data, err := json.Marshal(templates)
	if err != nil {
		return fmt.Errorf("failed to marshal templates: %w", err)
	}

	// Кешируем на 1 час
	err = s.redisClient.Set(ctx, cacheKey, data, time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to cache templates: %w", err)
	}

	s.logger.Info("Templates cached successfully",
		logger.String("key", cacheKey),
		logger.Int("count", len(templates)))

	return nil
}

// AddTemplate добавляет пользовательский шаблон
func (s *forgeService) AddTemplate(ctx context.Context, template TemplateInfo) error {
	s.logger.Info("Adding custom template",
		logger.String("name", template.Name),
		logger.String("type", template.Type),
		logger.String("language", template.Language))

	// Валидируем шаблон
	if err := s.validateTemplate(&template); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	// Создаем директорию для шаблона если нужно
	templateDir := filepath.Join(s.templatesDir, template.Type, template.Language)
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("failed to create template directory: %w", err)
	}

	// Сохраняем шаблон в файл
	templateFile := filepath.Join(templateDir, fmt.Sprintf("%s.json", template.Name))
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	if err := ioutil.WriteFile(templateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}

	// Очищаем кеш чтобы новый шаблон был доступен
	cacheKey := fmt.Sprintf("templates:%s:%s", template.Type, template.Language)
	if s.redisClient != nil {
		if err := s.redisClient.Del(ctx, cacheKey).Err(); err != nil {
			s.logger.Warn("Failed to clear template cache", logger.Error(err))
		}
	}

	s.logger.Info("Custom template added successfully",
		logger.String("file", templateFile))

	return nil
}

// GenerateDynamicTemplate динамически генерирует шаблон на основе параметров
func (s *forgeService) GenerateDynamicTemplate(ctx context.Context, params map[string]interface{}) (*TemplateInfo, error) {
	s.logger.Info("Generating dynamic template", logger.Any("params", params))

	// Извлекаем обязательные параметры
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("template name is required")
	}

	templateType, ok := params["type"].(string)
	if !ok || templateType == "" {
		return nil, fmt.Errorf("template type is required")
	}

	language, ok := params["language"].(string)
	if !ok || language == "" {
		return nil, fmt.Errorf("template language is required")
	}

	description, _ := params["description"].(string)
	if description == "" {
		description = fmt.Sprintf("Dynamic %s checker for %s", templateType, name)
	}

	parameters, _ := params["parameters"].(map[string]string)
	if parameters == nil {
		parameters = make(map[string]string)
	}

	example, _ := params["example"].(string)
	if example == "" {
		example = fmt.Sprintf("example.%s", templateType)
	}

	// Создаем динамический шаблон
	template := TemplateInfo{
		Name:        name,
		Type:        templateType,
		Language:    language,
		Description: description,
		Parameters:  parameters,
		Example:     example,
	}

	// Валидируем сгенерированный шаблон
	if err := s.validateTemplate(&template); err != nil {
		return nil, fmt.Errorf("generated template validation failed: %w", err)
	}

	s.logger.Info("Dynamic template generated successfully",
		logger.String("name", name),
		logger.String("type", templateType))

	return &template, nil
}

// contains проверяет наличие подстроки в строке
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

// findSubstring ищет подстроку в строке
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
