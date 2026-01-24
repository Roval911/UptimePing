package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config представляет конфигурацию приложения
type Config struct {
	Server           ServerConfig   `json:"server" yaml:"server"`
	Database         DatabaseConfig `json:"database" yaml:"database"`
	Logger           LoggerConfig   `json:"logger" yaml:"logger"`
	Environment      string         `json:"environment" yaml:"environment"`
	AuthService      ServiceConfig  `json:"auth_service" yaml:"auth_service"`
	ConfigService    ServiceConfig  `json:"config_service" yaml:"config_service"`
	CoreService      ServiceConfig  `json:"core_service" yaml:"core_service"`
	ForgeService     ServiceConfig  `json:"forge_service" yaml:"forge_service"`
	IncidentService  ServiceConfig  `json:"incident_service" yaml:"incident_service"`
	SchedulerService ServiceConfig  `json:"scheduler_service" yaml:"scheduler_service"`
}

// ServerConfig представляет конфигурацию сервера
type ServerConfig struct {
	Host string `json:"host" yaml:"host"`
	Port int    `json:"port" yaml:"port"`
}

// DatabaseConfig представляет конфигурацию базы данных
type DatabaseConfig struct {
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Name     string `json:"name" yaml:"name"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password" yaml:"password"`
}

// LoggerConfig представляет конфигурацию логгера
type LoggerConfig struct {
	Level  string `json:"level" yaml:"level"`
	Format string `json:"format" yaml:"format"`
}

// ServiceConfig представляет конфигурацию для внешних сервисов
type ServiceConfig struct {
	Host string `json:"host" yaml:"host"`
	Port int    `json:"port" yaml:"port"`
}

// LoadConfig загружает конфигурацию
func LoadConfig(configFile string) (*Config, error) {
	// Initialize config with default values
	config := &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Name:     "uptimeping",
			User:     "uptimeping",
			Password: "uptimeping",
		},
		Logger: LoggerConfig{
			Level:  "info",
			Format: "json",
		},
		Environment: "dev",
		AuthService: ServiceConfig{
			Host: "localhost",
			Port: 50051,
		},
		ConfigService: ServiceConfig{
			Host: "localhost",
			Port: 50052,
		},
		CoreService: ServiceConfig{
			Host: "localhost",
			Port: 50053,
		},
		ForgeService: ServiceConfig{
			Host: "localhost",
			Port: 50054,
		},
		IncidentService: ServiceConfig{
			Host: "localhost",
			Port: 50055,
		},
		SchedulerService: ServiceConfig{
			Host: "localhost",
			Port: 50056,
		},
	}

	// Load from file if specified
	if configFile != "" {
		if err := loadConfigFromFile(config, configFile); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Load from environment variables
	if err := loadConfigFromEnv(config); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

func loadConfigFromFile(config *Config, filename string) error {
	// Expand environment variables in the file path
	filename = os.ExpandEnv(filename)

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", filename)
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// Try to unmarshal as YAML first, then JSON
	if err := yaml.Unmarshal(content, config); err != nil {
		// If YAML fails, try JSON
		if jsonErr := json.Unmarshal(content, config); jsonErr != nil {
			return fmt.Errorf("failed to unmarshal config file as YAML or JSON: %w", err)
		}
	}

	return nil
}

func loadConfigFromEnv(config *Config) error {
	// Server config
	if host := os.Getenv("SERVER_HOST"); host != "" {
		config.Server.Host = host
	}
	if port := os.Getenv("SERVER_PORT"); port != "" {
		if _, err := fmt.Sscanf(port, "%d", &config.Server.Port); err != nil {
			return fmt.Errorf("invalid SERVER_PORT: %s", port)
		}
	}

	// Database config
	if host := os.Getenv("DATABASE_HOST"); host != "" {
		config.Database.Host = host
	}
	if port := os.Getenv("DATABASE_PORT"); port != "" {
		if _, err := fmt.Sscanf(port, "%d", &config.Database.Port); err != nil {
			return fmt.Errorf("invalid DATABASE_PORT: %s", port)
		}
	}
	if name := os.Getenv("DATABASE_NAME"); name != "" {
		config.Database.Name = name
	}
	if user := os.Getenv("DATABASE_USER"); user != "" {
		config.Database.User = user
	}
	if password := os.Getenv("DATABASE_PASSWORD"); password != "" {
		config.Database.Password = password
	}

	// Logger config
	if level := os.Getenv("LOGGER_LEVEL"); level != "" {
		config.Logger.Level = level
	}
	if format := os.Getenv("LOGGER_FORMAT"); format != "" {
		config.Logger.Format = format
	}

	// Environment
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		config.Environment = env
	}

	// Auth service config
	if host := os.Getenv("AUTH_SERVICE_HOST"); host != "" {
		config.AuthService.Host = host
	}
	if port := os.Getenv("AUTH_SERVICE_PORT"); port != "" {
		if _, err := fmt.Sscanf(port, "%d", &config.AuthService.Port); err != nil {
			return fmt.Errorf("invalid AUTH_SERVICE_PORT: %s", port)
		}
	}

	// Config service config
	if host := os.Getenv("CONFIG_SERVICE_HOST"); host != "" {
		config.ConfigService.Host = host
	}
	if port := os.Getenv("CONFIG_SERVICE_PORT"); port != "" {
		if _, err := fmt.Sscanf(port, "%d", &config.ConfigService.Port); err != nil {
			return fmt.Errorf("invalid CONFIG_SERVICE_PORT: %s", port)
		}
	}

	// Core service config
	if host := os.Getenv("CORE_SERVICE_HOST"); host != "" {
		config.CoreService.Host = host
	}
	if port := os.Getenv("CORE_SERVICE_PORT"); port != "" {
		if _, err := fmt.Sscanf(port, "%d", &config.CoreService.Port); err != nil {
			return fmt.Errorf("invalid CORE_SERVICE_PORT: %s", port)
		}
	}

	// Forge service config
	if host := os.Getenv("FORGE_SERVICE_HOST"); host != "" {
		config.ForgeService.Host = host
	}
	if port := os.Getenv("FORGE_SERVICE_PORT"); port != "" {
		if _, err := fmt.Sscanf(port, "%d", &config.ForgeService.Port); err != nil {
			return fmt.Errorf("invalid FORGE_SERVICE_PORT: %s", port)
		}
	}

	// Incident service config
	if host := os.Getenv("INCIDENT_SERVICE_HOST"); host != "" {
		config.IncidentService.Host = host
	}
	if port := os.Getenv("INCIDENT_SERVICE_PORT"); port != "" {
		if _, err := fmt.Sscanf(port, "%d", &config.IncidentService.Port); err != nil {
			return fmt.Errorf("invalid INCIDENT_SERVICE_PORT: %s", port)
		}
	}

	// Scheduler service config
	if host := os.Getenv("SCHEDULER_SERVICE_HOST"); host != "" {
		config.SchedulerService.Host = host
	}
	if port := os.Getenv("SCHEDULER_SERVICE_PORT"); port != "" {
		if _, err := fmt.Sscanf(port, "%d", &config.SchedulerService.Port); err != nil {
			return fmt.Errorf("invalid SCHEDULER_SERVICE_PORT: %s", port)
		}
	}

	return nil
}

func validateConfig(config *Config) error {
	// Проверка корректности окружения
	switch config.Environment {
	case "dev", "staging", "prod":
		// Valid environment
	default:
		return fmt.Errorf("invalid environment: %s, must be one of: dev, staging, prod", config.Environment)
	}

	// Валидация конфигурации сервера
	if config.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}

	// Валидация конфигурации базы данных
	if config.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if config.Database.Port <= 0 || config.Database.Port > 65535 {
		return fmt.Errorf("database.port must be between 1 and 65535")
	}
	if config.Database.Name == "" {
		return fmt.Errorf("database.name is required")
	}
	if config.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if config.Database.Password == "" {
		return fmt.Errorf("database.password is required")
	}

	// Валидация конфигурации логгера
	if config.Logger.Level == "" {
		return fmt.Errorf("logger.level is required")
	}
	if config.Logger.Format == "" {
		return fmt.Errorf("logger.format is required")
	}

	// Валидация конфигурации сервисов
	services := []struct {
		name   string
		config ServiceConfig
	}{
		{"auth_service", config.AuthService},
		{"config_service", config.ConfigService},
		{"core_service", config.CoreService},
		{"forge_service", config.ForgeService},
		{"incident_service", config.IncidentService},
		{"scheduler_service", config.SchedulerService},
	}

	for _, service := range services {
		if service.config.Host == "" {
			return fmt.Errorf("%s.host is required", service.name)
		}
		if service.config.Port <= 0 || service.config.Port > 65535 {
			return fmt.Errorf("%s.port must be between 1 and 65535", service.name)
		}
	}

	return nil
}

// Save сохраняет конфигурацию в файл в формате YAML
func (c *Config) Save(filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal to YAML
	content, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(filename, content, 0644)
}
