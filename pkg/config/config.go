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
	Server         ServerConfig    `json:"server" yaml:"server"`
	Database       DatabaseConfig  `json:"database" yaml:"database"`
	Logger         LoggerConfig    `json:"logger" yaml:"logger"`
	Environment    string          `json:"environment" yaml:"environment"`
	Redis          RedisConfig     `json:"redis" yaml:"redis"`
	JWT            JWTConfig       `json:"jwt" yaml:"jwt"`
	RabbitMQ       RabbitMQConfig  `json:"rabbitmq" yaml:"rabbitmq"`
	GRPC          GRPCConfig      `json:"grpc" yaml:"grpc"`
	RateLimiting   RateLimitConfig `json:"rate_limiting" yaml:"rate_limiting"`
	Providers      ProvidersConfig `json:"providers" yaml:"providers"`
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

// RabbitMQConfig представляет конфигурацию RabbitMQ
type RabbitMQConfig struct {
	URL        string `json:"url" yaml:"url"`
	Exchange   string `json:"exchange" yaml:"exchange"`
	RoutingKey string `json:"routing_key" yaml:"routing_key"`
	Queue      string `json:"queue" yaml:"queue"`
}

// RedisConfig представляет конфигурацию Redis
type RedisConfig struct {
	Addr           string        `json:"addr" yaml:"addr"`
	Password       string        `json:"password" yaml:"password"`
	DB             int           `json:"db" yaml:"db"`
	PoolSize       int           `json:"pool_size" yaml:"pool_size"`
	MinIdleConn    int           `json:"min_idle_conn" yaml:"min_idle_conn"`
	MaxRetries     int           `json:"max_retries" yaml:"max_retries"`
	RetryInterval  string        `json:"retry_interval" yaml:"retry_interval"`
	HealthCheck    string        `json:"health_check" yaml:"health_check"`
}

// IncidentManagerConfig представляет конфигурацию Incident Manager
type IncidentManagerConfig struct {
	Address string `json:"address" yaml:"address"`
}

// RateLimitConfig представляет конфигурацию Rate Limiting
type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute" yaml:"requests_per_minute"`
}

// JWTConfig представляет конфигурацию JWT
type JWTConfig struct {
	AccessSecret           string `json:"access_secret" yaml:"access_secret"`
	RefreshSecret          string `json:"refresh_secret" yaml:"refresh_secret"`
	AccessTokenDuration    string `json:"access_token_duration" yaml:"access_token_duration"`
	RefreshTokenDuration   string `json:"refresh_token_duration" yaml:"refresh_token_duration"`
}

// GRPCConfig представляет конфигурацию gRPC
type GRPCConfig struct {
	Port int `json:"port" yaml:"port"`
}

// ProvidersConfig представляет конфигурацию провайдеров уведомлений
type ProvidersConfig struct {
	Telegram TelegramProviderConfig `json:"telegram" yaml:"telegram"`
	Slack    SlackProviderConfig    `json:"slack" yaml:"slack"`
	Email    EmailProviderConfig    `json:"email" yaml:"email"`
}

// TelegramProviderConfig представляет конфигурацию Telegram провайдера
type TelegramProviderConfig struct {
	BotToken    string `json:"bot_token" yaml:"bot_token"`
	APIURL      string `json:"api_url" yaml:"api_url"`
	Timeout     string `json:"timeout" yaml:"timeout"`
	RetryAttempts int  `json:"retry_attempts" yaml:"retry_attempts"`
}

// SlackProviderConfig представляет конфигурацию Slack провайдера
type SlackProviderConfig struct {
	BotToken    string `json:"bot_token" yaml:"bot_token"`
	WebhookURL  string `json:"webhook_url" yaml:"webhook_url"`
	APIURL      string `json:"api_url" yaml:"api_url"`
	Timeout     string `json:"timeout" yaml:"timeout"`
	RetryAttempts int  `json:"retry_attempts" yaml:"retry_attempts"`
}

// EmailProviderConfig представляет конфигурацию Email провайдера
type EmailProviderConfig struct {
	SMTPHost     string `json:"smtp_host" yaml:"smtp_host"`
	SMTPPort     int    `json:"smtp_port" yaml:"smtp_port"`
	Username     string `json:"username" yaml:"username"`
	Password     string `json:"password" yaml:"password"`
	FromAddress  string `json:"from_address" yaml:"from_address"`
	FromName     string `json:"from_name" yaml:"from_name"`
	UseStartTLS  bool   `json:"use_starttls" yaml:"use_starttls"`
	Timeout      string `json:"timeout" yaml:"timeout"`
	RetryAttempts int    `json:"retry_attempts" yaml:"retry_attempts"`
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
		Redis: RedisConfig{
			Addr:           "localhost:6379",
			Password:       "",
			DB:             0,
			PoolSize:       10,
			MinIdleConn:    2,
			MaxRetries:     3,
			RetryInterval:  "1s",
			HealthCheck:    "30s",
		},
		JWT: JWTConfig{
			AccessSecret:         "your-access-secret",
			RefreshSecret:        "your-refresh-secret",
			AccessTokenDuration:  "15m",
			RefreshTokenDuration: "7d",
		},
		RabbitMQ: RabbitMQConfig{
			URL:        "amqp://guest:guest@localhost:5672/",
			Exchange:   "notifications",
			RoutingKey: "notification.events",
			Queue:      "notifications",
		},
		GRPC: GRPCConfig{
			Port: 50051,
		},
		RateLimiting: RateLimitConfig{
			RequestsPerMinute: 100,
		},
		Providers: ProvidersConfig{
			Telegram: TelegramProviderConfig{
				BotToken:     "",
				APIURL:       "https://api.telegram.org",
				Timeout:      "30s",
				RetryAttempts: 3,
			},
			Slack: SlackProviderConfig{
				BotToken:     "",
				WebhookURL:   "",
				APIURL:       "https://slack.com/api",
				Timeout:      "30s",
				RetryAttempts: 3,
			},
			Email: EmailProviderConfig{
				SMTPHost:     "smtp.gmail.com",
				SMTPPort:     587,
				Username:     "",
				Password:     "",
				FromAddress:  "noreply@uptimeping.com",
				FromName:     "UptimePing Platform",
				UseStartTLS:  true,
				Timeout:      "30s",
				RetryAttempts: 3,
			},
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
