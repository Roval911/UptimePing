package service

import (
	"context"
	"fmt"

	"UptimePingPlatform/pkg/logger"
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
}

// ForgeServiceInfo содержит информацию о сервисе из .proto файла
type ForgeServiceInfo struct {
	PackageName string           `json:"package_name"`
	ServiceName string           `json:"service_name"`
	Methods     []ForgeMethodInfo `json:"methods"`
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
	Name   string            `json:"name"`
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
	TargetHost   string            `json:"target_host"`
	TargetPort   int               `json:"target_port"`
	CheckInterval int               `json:"check_interval"`
	Timeout      int               `json:"timeout"`
	TenantID     string            `json:"tenant_id"`
	Metadata     map[string]string `json:"metadata"`
}

// CheckConfig содержит конфигурацию проверки
type CheckConfig struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Target   string `json:"target"`
	Interval int    `json:"interval"`
	Timeout  int    `json:"timeout"`
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
	codeGenerator  *CodeGenerator
}

// NewForgeService создает новый экземпляр ForgeService
func NewForgeService(logger logger.Logger, protoParser *ProtoParser, codeGenerator *CodeGenerator) ForgeService {
	return &forgeService{
		logger:       logger,
		protoParser:  protoParser,
		codeGenerator: codeGenerator,
	}
}

// ParseProto парсит .proto файл и возвращает информацию о сервисе
func (s *forgeService) ParseProto(ctx context.Context, protoContent, fileName string) (*ForgeServiceInfo, bool, []string, error) {
	s.logger.Info("Parsing proto file", 
		logger.String("file_name", fileName),
		logger.Int("content_length", len(protoContent)))

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

	// Формируем target
	target := fmt.Sprintf("%s:%d", options.TargetHost, options.TargetPort)
	if target == ":0" {
		target = "localhost:50051" // По умолчанию
	}

	// Создаем YAML конфигурацию
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

	// Создаем CheckConfig
	checkConfig := &CheckConfig{
		Name:     method.Name,
		Type:     checkType,
		Target:   target,
		Interval: options.CheckInterval,
		Timeout:  options.Timeout,
		Config:   fmt.Sprintf("service_name: %s\nmethod_name: %s", serviceInfo.ServiceName, method.Name),
	}

	s.logger.Info("Config generated successfully",
		logger.String("check_name", checkConfig.Name),
		logger.String("check_type", checkConfig.Type))

	return configYaml, checkConfig, nil
}

// GenerateCode генерирует код для проверки gRPC методов
func (s *forgeService) GenerateCode(ctx context.Context, protoContent string, options *CodeOptions) (string, string, string, error) {
	s.logger.Info("Generating code from proto",
		logger.Int("proto_length", len(protoContent)),
		logger.String("language", options.Language),
		logger.String("framework", options.Framework))

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

	filename := fmt.Sprintf("%s_checker.go", serviceInfo.ServiceName)
	
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

	code := fmt.Sprintf(template, 
		serviceInfo.ServiceName,
		serviceInfo.ServiceName,
		serviceInfo.ServiceName,
		serviceInfo.ServiceName,
		serviceInfo.ServiceName)

	s.logger.Info("Code generated successfully",
		logger.String("filename", filename),
		logger.String("language", language),
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
