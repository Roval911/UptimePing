package service

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"UptimePingPlatform/services/forge-service/internal/domain"
	"UptimePingPlatform/services/forge-service/internal/templates"
	pkglogger "UptimePingPlatform/pkg/logger"
)

// CodeGenerator генерирует код и конфигурации на основе proto файлов
type CodeGenerator struct {
	logger     pkglogger.Logger
	templates  *templates.TemplateManager
	outputDir  string
}

// NewCodeGenerator создает новый экземпляр генератора кода
func NewCodeGenerator(logger pkglogger.Logger, outputDir string) *CodeGenerator {
	return &CodeGenerator{
		logger:     logger,
		templates:  templates.NewTemplateManager(),
		outputDir:  outputDir,
	}
}

// GenerateConfig генерирует YAML конфигурацию для UptimePing Core
func (cg *CodeGenerator) GenerateConfig(services []domain.Service, configPath string) error {
	cg.logger.Info("Generating YAML configuration", 
		pkglogger.String("output", configPath),
		pkglogger.Int("services", len(services)))

	// Создаем структуру для конфигурации
	config := struct {
		Services []domain.Service
	}{
		Services: services,
	}

	// Загружаем шаблон YAML конфигурации
	tmpl, err := cg.templates.GetConfigTemplate()
	if err != nil {
		return fmt.Errorf("failed to load config template: %w", err)
	}

	// Генерируем YAML
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return fmt.Errorf("failed to execute config template: %w", err)
	}

	// Создаем директорию если не существует
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Записываем файл
	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	cg.logger.Info("YAML configuration generated successfully", 
		pkglogger.String("path", configPath))
	return nil
}

// GenerateGRPCCheckers генерирует Go код для проверки gRPC методов
func (cg *CodeGenerator) GenerateGRPCCheckers(services []domain.Service, outputPath string) error {
	cg.logger.Info("Generating gRPC checker code", 
		pkglogger.String("output", outputPath),
		pkglogger.Int("services", len(services)))

	// Создаем директорию если не существует
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Генерируем код для каждого сервиса
	for _, service := range services {
		if err := cg.generateServiceChecker(service, outputPath); err != nil {
			cg.logger.Error("Failed to generate checker for service", 
				pkglogger.String("service", service.Name),
				pkglogger.Error(err))
			continue
		}
	}

	cg.logger.Info("gRPC checker code generated successfully")
	return nil
}

// generateServiceChecker генерирует код для конкретного сервиса
func (cg *CodeGenerator) generateServiceChecker(service domain.Service, outputPath string) error {
	cg.logger.Debug("Generating checker for service", 
		pkglogger.String("service", service.Name),
		pkglogger.String("package", service.Package))

	// Загружаем шаблон gRPC checker
	tmpl, err := cg.templates.GetGRPCTemplate()
	if err != nil {
		return fmt.Errorf("failed to load gRPC template: %w", err)
	}

	// Подготавливаем данные для шаблона
	data := struct {
		Service     domain.Service
		PackageName string
		CheckerName string
	}{
		Service:     service,
		PackageName: cg.sanitizePackageName(service.Package),
		CheckerName: cg.generateCheckerName(service.Name),
	}

	// Генерируем код
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute gRPC template: %w", err)
	}

	// Формируем имя файла
	filename := fmt.Sprintf("%s_checker.go", strings.ToLower(data.CheckerName))
	filePath := filepath.Join(outputPath, filename)

	// Записываем файл
	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write checker file: %w", err)
	}

	cg.logger.Debug("Checker generated", 
		pkglogger.String("service", service.Name),
		pkglogger.String("file", filename))

	return nil
}

// GenerateInteractiveConfig генерирует интерактивную настройку
func (cg *CodeGenerator) GenerateInteractiveConfig(config *domain.InteractiveConfig) error {
	cg.logger.Info("Generating interactive configuration")

	// Создаем директорию если не существует
	if err := os.MkdirAll(cg.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Генерируем .env файл
	if err := cg.generateEnvFile(config); err != nil {
		return fmt.Errorf("failed to generate .env file: %w", err)
	}

	// Генерируем config.yaml
	if err := cg.generateConfigFile(config); err != nil {
		return fmt.Errorf("failed to generate config.yaml: %w", err)
	}

	cg.logger.Info("Interactive configuration generated successfully")
	return nil
}

// generateEnvFile генерирует .env файл
func (cg *CodeGenerator) generateEnvFile(config *domain.InteractiveConfig) error {
	envContent := fmt.Sprintf(`# UptimePing Core Configuration
# Generated by Forge Service

# Server Configuration
SERVER_HOST=%s
SERVER_PORT=%d

# Database Configuration
DB_HOST=%s
DB_PORT=%d
DB_NAME=%s
DB_USER=%s
DB_PASSWORD=%s

# Redis Configuration
REDIS_ADDR=%s
REDIS_PASSWORD=%s
REDIS_DB=%d

# Notification Configuration
# Telegram
TELEGRAM_BOT_TOKEN=%s
TELEGRAM_CHAT_ID=%s
TELEGRAM_ENABLED=%t

# Email
EMAIL_SMTP_HOST=%s
EMAIL_SMTP_PORT=%d
EMAIL_USERNAME=%s
EMAIL_PASSWORD=%s
EMAIL_FROM_ADDRESS=%s
EMAIL_FROM_NAME=%s
EMAIL_ENABLED=%t

# Logger Configuration
LOG_LEVEL=%s
LOG_FORMAT=%s

# Environment
ENVIRONMENT=%s
`,
		config.Server.Host,
		config.Server.Port,
		config.Database.Host,
		config.Database.Port,
		config.Database.Name,
		config.Database.User,
		config.Database.Password,
		config.Redis.Addr,
		config.Redis.Password,
		config.Redis.DB,
		config.Telegram.BotToken,
		config.Telegram.ChatID,
		config.Telegram.Enabled,
		config.Email.SMTPHost,
		config.Email.SMTPPort,
		config.Email.Username,
		config.Email.Password,
		config.Email.FromAddress,
		config.Email.FromName,
		config.Email.Enabled,
		config.Logger.Level,
		config.Logger.Format,
		config.Environment,
	)

	envPath := filepath.Join(cg.outputDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		return err
	}

	cg.logger.Info(".env file generated", pkglogger.String("path", envPath))
	return nil
}

// generateConfigFile генерирует config.yaml файл
func (cg *CodeGenerator) generateConfigFile(config *domain.InteractiveConfig) error {
	configContent := fmt.Sprintf(`# UptimePing Core Configuration
# Generated by Forge Service

server:
  host: "${SERVER_HOST:%s}"
  port: ${SERVER_PORT:%d}

database:
  host: "${DB_HOST:%s}"
  port: ${DB_PORT:%d}
  name: "${DB_NAME:%s}"
  user: "${DB_USER:%s}"
  password: "${DB_PASSWORD:%s}"

redis:
  addr: "${REDIS_ADDR:%s}"
  password: "${REDIS_PASSWORD:%s}"
  db: ${REDIS_DB:%d}

logger:
  level: "${LOG_LEVEL:%s}"
  format: "${LOG_FORMAT:%s}"

environment: "${ENVIRONMENT:%s}"

notifications:
  telegram:
    enabled: ${TELEGRAM_ENABLED:%t}
    bot_token: "${TELEGRAM_BOT_TOKEN:}"
    chat_id: "${TELEGRAM_CHAT_ID:}"
  
  email:
    enabled: ${EMAIL_ENABLED:%t}
    smtp_host: "${EMAIL_SMTP_HOST:%s}"
    smtp_port: ${EMAIL_SMTP_PORT:%d}
    username: "${EMAIL_USERNAME:}"
    password: "${EMAIL_PASSWORD:}"
    from_address: "${EMAIL_FROM_ADDRESS:%s}"
    from_name: "${EMAIL_FROM_NAME:%s}"
`,
		config.Server.Host,
		config.Server.Port,
		config.Database.Host,
		config.Database.Port,
		config.Database.Name,
		config.Database.User,
		config.Database.Password,
		config.Redis.Addr,
		config.Redis.Password,
		config.Redis.DB,
		config.Logger.Level,
		config.Logger.Format,
		config.Environment,
		config.Email.SMTPHost,
		config.Email.SMTPPort,
		config.Email.FromAddress,
		config.Email.FromName,
	)

	configPath := filepath.Join(cg.outputDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return err
	}

	cg.logger.Info("config.yaml file generated", pkglogger.String("path", configPath))
	return nil
}

// Вспомогательные функции

func (cg *CodeGenerator) sanitizePackageName(pkg string) string {
	// Удаляем недопустимые символы и приводим к нижнему регистру
	sanitized := strings.ToLower(pkg)
	sanitized = strings.ReplaceAll(sanitized, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, "-", "_")
	return sanitized
}

func (cg *CodeGenerator) generateCheckerName(serviceName string) string {
	// Генерируем имя checker'а из имени сервиса
	name := strings.Title(serviceName)
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, "_", "")
	return name + "Checker"
}

// GenerateAll генерирует все артефакты
func (cg *CodeGenerator) GenerateAll(services []domain.Service, config *domain.InteractiveConfig) error {
	cg.logger.Info("Starting full generation process")

	// Генерируем YAML конфигурацию
	configPath := filepath.Join(cg.outputDir, "uptime_config.yaml")
	if err := cg.GenerateConfig(services, configPath); err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Генерируем gRPC checker'ы
	checkersPath := filepath.Join(cg.outputDir, "checkers")
	if err := cg.GenerateGRPCCheckers(services, checkersPath); err != nil {
		return fmt.Errorf("failed to generate gRPC checkers: %w", err)
	}

	// Генерируем интерактивную настройку
	if config != nil {
		if err := cg.GenerateInteractiveConfig(config); err != nil {
			return fmt.Errorf("failed to generate interactive config: %w", err)
		}
	}

	cg.logger.Info("Full generation completed successfully")
	return nil
}
