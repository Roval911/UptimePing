package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// Config представляет конфигурацию приложения. Структура содержит вложенные структуры для различных компонентов приложения.
type Config struct {
	Server       ServerConfig    `json:"server" yaml:"server"`
	Database     DatabaseConfig  `json:"database" yaml:"database"`
	Logger       LoggerConfig    `json:"logger" yaml:"logger"`
	Environment  string          `json:"environment" yaml:"environment"`
	Redis        RedisConfig     `json:"redis" yaml:"redis"`
	JWT          JWTConfig       `json:"jwt" yaml:"jwt"`
	RabbitMQ     RabbitMQConfig  `json:"rabbitmq" yaml:"rabbitmq"`
	GRPC         GRPCConfig      `json:"grpc" yaml:"grpc"`
	RateLimiting RateLimitConfig `json:"rate_limiting" yaml:"rate_limiting"`
	Providers    ProvidersConfig `json:"providers" yaml:"providers"`
	Forge        ForgeConfig     `json:"forge" yaml:"forge"`
	Metrics      MetricsConfig   `json:"metrics" yaml:"metrics"`
	Health       HealthConfig    `json:"health" yaml:"health"`
	Services     ServicesConfig  `json:"services" yaml:"services"`
	Recipients   RecipientsConfig `json:"recipients" yaml:"recipients"`
	Scheduler    SchedulerConfig `json:"scheduler" yaml:"scheduler"`
	IncidentManager IncidentManagerConfig `json:"incident_manager" yaml:"incident_manager"`
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
	Addr          string `json:"addr" yaml:"addr"`
	Password      string `json:"password" yaml:"password"`
	DB            int    `json:"db" yaml:"db"`
	PoolSize      int    `json:"pool_size" yaml:"pool_size"`
	MinIdleConn   int    `json:"min_idle_conn" yaml:"min_idle_conn"`
	MaxRetries    int    `json:"max_retries" yaml:"max_retries"`
	RetryInterval string `json:"retry_interval" yaml:"retry_interval"`
	HealthCheck   string `json:"health_check" yaml:"health_check"`
}

// IncidentManagerConfig представляет конфигурацию Incident Manager
type IncidentManagerConfig struct {
	Address string `json:"address" yaml:"address"`
}

// RateLimitConfig представляет конфигурацию Rate Limiting
type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute" yaml:"requests_per_minute"`
	BurstSize         int `json:"burst_size" yaml:"burst_size"`
}

// JWTConfig представляет конфигурацию JWT
type JWTConfig struct {
	AccessSecret         string `json:"access_secret" yaml:"access_secret"`
	RefreshSecret        string `json:"refresh_secret" yaml:"refresh_secret"`
	AccessTokenDuration  string `json:"access_token_duration" yaml:"access_token_duration"`
	RefreshTokenDuration string `json:"refresh_token_duration" yaml:"refresh_token_duration"`
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
	BotToken      string `json:"bot_token" yaml:"bot_token"`
	APIURL        string `json:"api_url" yaml:"api_url"`
	Timeout       string `json:"timeout" yaml:"timeout"`
	RetryAttempts int    `json:"retry_attempts" yaml:"retry_attempts"`
}

// SlackProviderConfig представляет конфигурацию Slack провайдера
type SlackProviderConfig struct {
	BotToken      string `json:"bot_token" yaml:"bot_token"`
	WebhookURL    string `json:"webhook_url" yaml:"webhook_url"`
	APIURL        string `json:"api_url" yaml:"api_url"`
	Timeout       string `json:"timeout" yaml:"timeout"`
	RetryAttempts int    `json:"retry_attempts" yaml:"retry_attempts"`
}

// EmailProviderConfig представляет конфигурацию Email провайдера
type EmailProviderConfig struct {
	SMTPHost      string `json:"smtp_host" yaml:"smtp_host"`
	SMTPPort      int    `json:"smtp_port" yaml:"smtp_port"`
	Username      string `json:"username" yaml:"username"`
	Password      string `json:"password" yaml:"password"`
	FromAddress   string `json:"from_address" yaml:"from_address"`
	FromName      string `json:"from_name" yaml:"from_name"`
	UseStartTLS   bool   `json:"use_starttls" yaml:"use_starttls"`
	Timeout       string `json:"timeout" yaml:"timeout"`
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
			Addr:          "localhost:6379",
			Password:      "",
			DB:            0,
			PoolSize:      10,
			MinIdleConn:   2,
			MaxRetries:    3,
			RetryInterval: "1s",
			HealthCheck:   "30s",
		},
		JWT: JWTConfig{
			AccessSecret:         "", // Будет загружено из переменных окружения
			RefreshSecret:        "", // Будет загружено из переменных окружения
			AccessTokenDuration:  "15m",
			RefreshTokenDuration: "7d",
		},
		RabbitMQ: RabbitMQConfig{
			URL:        "", // Будет загружено из переменных окружения
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
				BotToken:      "",
				APIURL:        "https://api.telegram.org",
				Timeout:       "30s",
				RetryAttempts: 3,
			},
			Slack: SlackProviderConfig{
				BotToken:      "",
				WebhookURL:    "",
				APIURL:        "https://slack.com/api",
				Timeout:       "30s",
				RetryAttempts: 3,
			},
			Email: EmailProviderConfig{
				SMTPHost:      "smtp.gmail.com",
				SMTPPort:      587,
				Username:      "",
				Password:      "",
				FromAddress:   "noreply@uptimeping.com",
				FromName:      "UptimePing Platform",
				UseStartTLS:   true,
				Timeout:       "30s",
				RetryAttempts: 3,
			},
		},
		Forge: ForgeConfig{
			ProtoDir:  "proto",
			OutputDir: "generated",
		},
		Metrics: MetricsConfig{
			// Основные настройки
			Enabled:        true,
			Port:           9090,
			Path:           "/metrics",
			
			// Настройки сбора метрик
			ScrapeInterval: "15s",
			Timeout:        "10s",
			RetryAttempts:  3,
			
			// Prometheus настройки
			Namespace:     "uptimeping",
			Subsystem:     "http",
			Buckets:       []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			
			// OpenTelemetry настройки
			TracingEnabled: true,
			TracerName:     "uptimeping-tracer",
			SamplingRate:   1.0,
			ServiceName:    "", // Будет установлено в каждом сервисе
			ServiceVersion: "1.0.0",
			
			// Дополнительные метрики
			EnableCustomMetrics: true,
			EnableSystemMetrics:  true,
		},
		Health: HealthConfig{
			Enabled:       true,
			Port:          8091,
			CheckInterval: "30s",
		},
		Services: ServicesConfig{
			AuthService: ServiceConfig{
				Address: "localhost:50051",
				Enabled: true,
			},
			CoreService: ServiceConfig{
				Address: "localhost:50052",
				Enabled: true,
			},
			SchedulerService: ServiceConfig{
				Address: "localhost:50053",
				Enabled: true,
			},
			APIGateway: ServiceConfig{
				Address: "localhost:8080",
				Enabled: true,
			},
		},
		Recipients: RecipientsConfig{
			DefaultEmails: []string{
				"admin@uptimeping.com",
				"ops@uptimeping.com",
			},
			DefaultSlack: []string{
				"#alerts",
				"#incidents",
			},
			DefaultSMS: []string{
				"+1234567890",
			},
			DefaultWebhooks: []string{
				"https://webhook.uptimeping.com/notifications",
			},
			TenantRecipients: map[string]TenantRecipients{},
			SeverityRecipients: map[string]SeverityRecipients{
				"critical": {
					Emails: []string{"critical@uptimeping.com"},
					Slack:  []string{"#critical-alerts"},
					SMS:    []string{"+1234567890"},
				},
				"high": {
					Emails: []string{"alerts@uptimeping.com"},
					Slack:  []string{"#high-alerts"},
				},
			},
		},
		Scheduler: SchedulerConfig{
			MaxConcurrentTasks: 10,
			TaskTimeout:        30 * time.Second,
			CleanupInterval:    1 * time.Hour,
			LockTimeout:        5 * time.Minute,
		},
	}

	// Load from file if specified and exists
	if configFile != "" {
		if err := loadConfigFromFile(config, configFile); err != nil {
			// If file doesn't exist, continue with defaults
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load config from file: %w", err)
			}
			// File doesn't exist, continue with defaults and env vars
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

	// Process environment variables in format ${VAR:default}
	processedContent := expandEnvVars(string(content))

	// Try to unmarshal as YAML first, then JSON
	if err := yaml.Unmarshal([]byte(processedContent), config); err != nil {
		// If YAML fails, try JSON
		if jsonErr := json.Unmarshal([]byte(processedContent), config); jsonErr != nil {
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

	// Forge config
	if protoDir := os.Getenv("PROTO_DIR"); protoDir != "" {
		config.Forge.ProtoDir = protoDir
	}
	if outputDir := os.Getenv("OUTPUT_DIR"); outputDir != "" {
		config.Forge.OutputDir = outputDir
	}

	// Metrics config
	if metricsEnabled := os.Getenv("METRICS_ENABLED"); metricsEnabled != "" {
		if enabled, err := strconv.ParseBool(metricsEnabled); err == nil {
			config.Metrics.Enabled = enabled
		}
	}
	if metricsPort := os.Getenv("METRICS_PORT"); metricsPort != "" {
		if _, err := fmt.Sscanf(metricsPort, "%d", &config.Metrics.Port); err != nil {
			return fmt.Errorf("invalid METRICS_PORT: %s", metricsPort)
		}
	}

	// Health config
	if healthEnabled := os.Getenv("HEALTH_ENABLED"); healthEnabled != "" {
		if enabled, err := strconv.ParseBool(healthEnabled); err == nil {
			config.Health.Enabled = enabled
		}
	}
	if healthPort := os.Getenv("HEALTH_PORT"); healthPort != "" {
		if _, err := fmt.Sscanf(healthPort, "%d", &config.Health.Port); err != nil {
			return fmt.Errorf("invalid HEALTH_PORT: %s", healthPort)
		}
	}

	// JWT config
	if accessSecret := os.Getenv("JWT_ACCESS_SECRET"); accessSecret != "" {
		config.JWT.AccessSecret = accessSecret
	}
	if refreshSecret := os.Getenv("JWT_REFRESH_SECRET"); refreshSecret != "" {
		config.JWT.RefreshSecret = refreshSecret
	}
	if accessTokenDuration := os.Getenv("JWT_ACCESS_TOKEN_DURATION"); accessTokenDuration != "" {
		config.JWT.AccessTokenDuration = accessTokenDuration
	}
	if refreshTokenDuration := os.Getenv("JWT_REFRESH_TOKEN_DURATION"); refreshTokenDuration != "" {
		config.JWT.RefreshTokenDuration = refreshTokenDuration
	}

	// RabbitMQ config
	if rabbitmqURL := os.Getenv("RABBITMQ_URL"); rabbitmqURL != "" {
		config.RabbitMQ.URL = rabbitmqURL
	}
	if rabbitmqExchange := os.Getenv("RABBITMQ_EXCHANGE"); rabbitmqExchange != "" {
		config.RabbitMQ.Exchange = rabbitmqExchange
	}
	if rabbitmqRoutingKey := os.Getenv("RABBITMQ_ROUTING_KEY"); rabbitmqRoutingKey != "" {
		config.RabbitMQ.RoutingKey = rabbitmqRoutingKey
	}
	if rabbitmqQueue := os.Getenv("RABBITMQ_QUEUE"); rabbitmqQueue != "" {
		config.RabbitMQ.Queue = rabbitmqQueue
	}

	// gRPC config
	if grpcPort := os.Getenv("GRPC_PORT"); grpcPort != "" {
		if _, err := fmt.Sscanf(grpcPort, "%d", &config.GRPC.Port); err != nil {
			return fmt.Errorf("invalid GRPC_PORT: %s", grpcPort)
		}
	}

	// Rate limiting config
	if rateLimitRequests := os.Getenv("RATE_LIMIT_REQUESTS_PER_MINUTE"); rateLimitRequests != "" {
		if _, err := fmt.Sscanf(rateLimitRequests, "%d", &config.RateLimiting.RequestsPerMinute); err != nil {
			return fmt.Errorf("invalid RATE_LIMIT_REQUESTS_PER_MINUTE: %s", rateLimitRequests)
		}
	}

	// Metrics config
	if metricsEnabled := os.Getenv("METRICS_ENABLED"); metricsEnabled != "" {
		if enabled, err := strconv.ParseBool(metricsEnabled); err == nil {
			config.Metrics.Enabled = enabled
		}
	}
	if metricsPort := os.Getenv("METRICS_PORT"); metricsPort != "" {
		if _, err := fmt.Sscanf(metricsPort, "%d", &config.Metrics.Port); err != nil {
			return fmt.Errorf("invalid METRICS_PORT: %s", metricsPort)
		}
	}
	if metricsPath := os.Getenv("METRICS_PATH"); metricsPath != "" {
		config.Metrics.Path = metricsPath
	}
	if metricsScrapeInterval := os.Getenv("METRICS_SCRAPE_INTERVAL"); metricsScrapeInterval != "" {
		config.Metrics.ScrapeInterval = metricsScrapeInterval
	}
	if metricsTimeout := os.Getenv("METRICS_TIMEOUT"); metricsTimeout != "" {
		config.Metrics.Timeout = metricsTimeout
	}
	if metricsRetryAttempts := os.Getenv("METRICS_RETRY_ATTEMPTS"); metricsRetryAttempts != "" {
		if _, err := fmt.Sscanf(metricsRetryAttempts, "%d", &config.Metrics.RetryAttempts); err != nil {
			return fmt.Errorf("invalid METRICS_RETRY_ATTEMPTS: %s", metricsRetryAttempts)
		}
	}
	if metricsNamespace := os.Getenv("METRICS_NAMESPACE"); metricsNamespace != "" {
		config.Metrics.Namespace = metricsNamespace
	}
	if metricsSubsystem := os.Getenv("METRICS_SUBSYSTEM"); metricsSubsystem != "" {
		config.Metrics.Subsystem = metricsSubsystem
	}
	if metricsTracingEnabled := os.Getenv("METRICS_TRACING_ENABLED"); metricsTracingEnabled != "" {
		if enabled, err := strconv.ParseBool(metricsTracingEnabled); err == nil {
			config.Metrics.TracingEnabled = enabled
		}
	}
	if metricsTracerName := os.Getenv("METRICS_TRACER_NAME"); metricsTracerName != "" {
		config.Metrics.TracerName = metricsTracerName
	}
	if metricsSamplingRate := os.Getenv("METRICS_SAMPLING_RATE"); metricsSamplingRate != "" {
		if rate, err := strconv.ParseFloat(metricsSamplingRate, 64); err == nil {
			config.Metrics.SamplingRate = rate
		}
	}
	if metricsServiceName := os.Getenv("METRICS_SERVICE_NAME"); metricsServiceName != "" {
		config.Metrics.ServiceName = metricsServiceName
	}
	if metricsServiceVersion := os.Getenv("METRICS_SERVICE_VERSION"); metricsServiceVersion != "" {
		config.Metrics.ServiceVersion = metricsServiceVersion
	}
	if metricsCustomEnabled := os.Getenv("METRICS_ENABLE_CUSTOM"); metricsCustomEnabled != "" {
		if enabled, err := strconv.ParseBool(metricsCustomEnabled); err == nil {
			config.Metrics.EnableCustomMetrics = enabled
		}
	}
	if metricsSystemEnabled := os.Getenv("METRICS_ENABLE_SYSTEM"); metricsSystemEnabled != "" {
		if enabled, err := strconv.ParseBool(metricsSystemEnabled); err == nil {
			config.Metrics.EnableSystemMetrics = enabled
		}
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

// expandEnvVars processes environment variables in format ${VAR:default}
func expandEnvVars(content string) string {
	// Use regex to find ${VAR:default} patterns
	re := regexp.MustCompile(`\$\{([^}:]+):([^}]*)\}`)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract VAR and default from the match
		parts := strings.SplitN(match[2:len(match)-1], ":", 2)
		if len(parts) != 2 {
			return match // Return original if format is wrong
		}

		varName := parts[0]
		defaultValue := parts[1]

		// Get environment variable or use default
		if envValue := os.Getenv(varName); envValue != "" {
			return envValue
		}
		return defaultValue
	})
}

// ForgeConfig представляет конфигурацию Forge Service
type ForgeConfig struct {
	ProtoDir  string `json:"proto_dir" yaml:"proto_dir"`
	OutputDir string `json:"output_dir" yaml:"output_dir"`
}

// MetricsConfig представляет конфигурацию метрик
type MetricsConfig struct {
	// Основные настройки
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	Port           int    `json:"port" yaml:"port"`
	Path           string `json:"path" yaml:"path"`
	
	// Настройки сбора метрик
	ScrapeInterval string `json:"scrape_interval" yaml:"scrape_interval"`
	Timeout        string `json:"timeout" yaml:"timeout"`
	RetryAttempts  int    `json:"retry_attempts" yaml:"retry_attempts"`
	
	// Prometheus настройки
	Namespace     string   `json:"namespace" yaml:"namespace"`
	Subsystem     string   `json:"subsystem" yaml:"subsystem"`
	Buckets       []float64 `json:"buckets" yaml:"buckets"`
	
	// OpenTelemetry настройки
	TracingEnabled bool     `json:"tracing_enabled" yaml:"tracing_enabled"`
	TracerName     string   `json:"tracer_name" yaml:"tracer_name"`
	SamplingRate   float64  `json:"sampling_rate" yaml:"sampling_rate"`
	ServiceName    string   `json:"service_name" yaml:"service_name"`
	ServiceVersion string   `json:"service_version" yaml:"service_version"`
	
	// Дополнительные метрики
	EnableCustomMetrics bool `json:"enable_custom_metrics" yaml:"enable_custom_metrics"`
	EnableSystemMetrics  bool `json:"enable_system_metrics" yaml:"enable_system_metrics"`
}

// HealthConfig представляет конфигурацию health check
type HealthConfig struct {
	Enabled       bool   `json:"enabled" yaml:"enabled"`
	Port          int    `json:"port" yaml:"port"`
	CheckInterval string `json:"check_interval" yaml:"check_interval"`
}

// ServicesConfig представляет конфигурацию сервисов для мониторинга
type ServicesConfig struct {
	AuthService      ServiceConfig `json:"auth_service" yaml:"auth_service"`
	CoreService      ServiceConfig `json:"core_service" yaml:"core_service"`
	SchedulerService ServiceConfig `json:"scheduler_service" yaml:"scheduler_service"`
	APIGateway       ServiceConfig `json:"api_gateway" yaml:"api_gateway"`
}

// ServiceConfig представляет конфигурацию отдельного сервиса
type ServiceConfig struct {
	Address string `json:"address" yaml:"address"`
	Enabled bool   `json:"enabled" yaml:"enabled"`
}

// RecipientsConfig представляет конфигурацию получателей уведомлений
type RecipientsConfig struct {
	// Получатели по умолчанию для каждого канала
	DefaultEmails []string          `json:"default_emails" yaml:"default_emails"`
	DefaultSlack  []string          `json:"default_slack" yaml:"default_slack"`
	DefaultSMS    []string          `json:"default_sms" yaml:"default_sms"`
	DefaultWebhooks []string        `json:"default_webhooks" yaml:"default_webhooks"`
	
	// Получатели по tenant
	TenantRecipients map[string]TenantRecipients `json:"tenant_recipients" yaml:"tenant_recipients"`
	
	// Получатели по серьезности
	SeverityRecipients map[string]SeverityRecipients `json:"severity_recipients" yaml:"severity_recipients"`
}

// TenantRecipients представляет получателей для конкретного tenant
type TenantRecipients struct {
	Emails   []string `json:"emails" yaml:"emails"`
	Slack    []string `json:"slack" yaml:"slack"`
	SMS      []string `json:"sms" yaml:"sms"`
	Webhooks []string `json:"webhooks" yaml:"webhooks"`
}

// SeverityRecipients представляет получателей для конкретного уровня серьезности
type SeverityRecipients struct {
	Emails   []string `json:"emails" yaml:"emails"`
	Slack    []string `json:"slack" yaml:"slack"`
	SMS      []string `json:"sms" yaml:"sms"`
	Webhooks []string `json:"webhooks" yaml:"webhooks"`
}

// SchedulerConfig конфигурация планировщика
type SchedulerConfig struct {
	MaxConcurrentTasks int           `json:"max_concurrent_tasks" yaml:"max_concurrent_tasks"`
	TaskTimeout        time.Duration `json:"task_timeout" yaml:"task_timeout"`
	CleanupInterval    time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
	LockTimeout        time.Duration `json:"lock_timeout" yaml:"lock_timeout"`
}
