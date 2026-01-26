package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Config представляет конфигурацию приложения. Структура содержит вложенные структуры для различных компонентов приложения.
type Config struct {
	Server      ServerConfig   `json:"server" yaml:"server"`
	Database    DatabaseConfig `json:"database" yaml:"database"`
	Logger      LoggerConfig   `json:"logger" yaml:"logger"`
	Environment string         `json:"environment" yaml:"environment"`
}

// ServerConfig представляет конфигурацию сервера. Содержит настройки хоста и порта для HTTP-сервера.
type ServerConfig struct {
	Host string `json:"host" yaml:"host"`
	Port int    `json:"port" yaml:"port"`
}

// DatabaseConfig представляет конфигурацию базы данных. Содержит параметры подключения к базе данных, включая хост, порт, имя базы, пользователя и пароль.
type DatabaseConfig struct {
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Name     string `json:"name" yaml:"name"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password" yaml:"password"`
}

// LoggerConfig представляет конфигурацию логгера. Определяет уровень логирования и формат вывода логов.
type LoggerConfig struct {
	Level  string `json:"level" yaml:"level"`
	Format string `json:"format" yaml:"format"`
}

// LoadConfig загружает конфигурацию в следующем порядке приоритета:
// 1. Загрузка значений по умолчанию
// 2. Загрузка из файла (если указан)
// 3. Переопределение значениями из переменных окружения
// 4. Валидация конфигурации
// Возвращает готовую конфигурацию или ошибку.
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

	return nil
}

func validateConfig(config *Config) error {
	// Проверка корректности окружения. Поддерживаются только: dev, staging, prod
	switch config.Environment {
	case "dev", "staging", "prod":
		// Valid environment
	default:
		return fmt.Errorf("invalid environment: %s, must be one of: dev, staging, prod", config.Environment)
	}

	// Валидация конфигурации сервера
	// Проверяем, что хост не пустой и порт в допустимом диапазоне (1-65535)
	if config.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}

	// Валидация конфигурации базы данных
	// Проверяем, что все обязательные поля заполнены и порт в допустимом диапазоне
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
	// Проверяем, что уровень и формат логирования заданы
	if config.Logger.Level == "" {
		return fmt.Errorf("logger.level is required")
	}
	if config.Logger.Format == "" {
		return fmt.Errorf("logger.format is required")
	}

	return nil
}

// Save сохраняет конфигурацию в файл в формате YAML.
// Автоматически создает директорию, если она не существует.
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
