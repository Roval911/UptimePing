package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v2"
)

// Config представляет конфигурацию CLI
type Config struct {
	// API настройки
	API struct {
		BaseURL string `yaml:"base_url" json:"base_url" env:"API_BASE_URL"`
		Timeout int    `yaml:"timeout" json:"timeout" env:"API_TIMEOUT"`
	} `yaml:"api" json:"api"`

	// gRPC настройки
	GRPC struct {
		SchedulerAddress string `yaml:"scheduler_address" json:"scheduler_address" env:"GRPC_SCHEDULER_ADDRESS"`
		CoreAddress     string `yaml:"core_address" json:"core_address" env:"GRPC_CORE_ADDRESS"`
		AuthAddress     string `yaml:"auth_address" json:"auth_address" env:"GRPC_AUTH_ADDRESS"`
		UseGRPC          bool   `yaml:"use_grpc" json:"use_grpc" env:"GRPC_USE_GRPC"`
		Timeout          int    `yaml:"timeout" json:"timeout" env:"GRPC_TIMEOUT"`
	} `yaml:"grpc" json:"grpc"`

	// Аутентификация
	Auth struct {
		TokenExpiry       int `yaml:"token_expiry" json:"token_expiry" env:"AUTH_TOKEN_EXPIRY"`
		RefreshThreshold  int `yaml:"refresh_threshold" json:"refresh_threshold" env:"AUTH_REFRESH_THRESHOLD"`
		AccessSecret      string `yaml:"access_secret" json:"access_secret" env:"AUTH_ACCESS_SECRET"`
		RefreshSecret     string `yaml:"refresh_secret" json:"refresh_secret" env:"AUTH_REFRESH_SECRET"`
	} `yaml:"auth" json:"auth"`

	// Настройки вывода
	Output struct {
		Format string `yaml:"format" json:"format" env:"OUTPUT_FORMAT"` // table, json, yaml
		Colors bool   `yaml:"colors" json:"colors" env:"OUTPUT_COLORS"`
	} `yaml:"output" json:"output"`

	// Текущий тенант
	CurrentTenant string `yaml:"current_tenant" json:"current_tenant" env:"CURRENT_TENANT"`

	// Путь к файлу конфигурации
	Path string `yaml:"-" json:"-"`
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *Config {
	config := &Config{}
	
	// API настройки по умолчанию
	config.API.BaseURL = "http://localhost:8080"
	config.API.Timeout = 30
	
	// gRPC настройки по умолчанию
	config.GRPC.SchedulerAddress = "localhost:50051"
	config.GRPC.CoreAddress = "localhost:50052"
	config.GRPC.AuthAddress = "localhost:50053"
	config.GRPC.UseGRPC = false // По умолчанию выключен для разработки
	config.GRPC.Timeout = 30
	
	// Настройки аутентификации по умолчанию
	config.Auth.TokenExpiry = 3600 // 1 час
	config.Auth.RefreshThreshold = 300 // 5 минут до истечения
	config.Auth.AccessSecret = "" // Будет загружено из переменных окружения
	config.Auth.RefreshSecret = "" // Будет загружено из переменных окружения
	
	// Настройки вывода по умолчанию
	config.Output.Format = "table"
	config.Output.Colors = true
	
	// Текущий тенант по умолчанию
	config.CurrentTenant = "default-tenant"
	
	return config
}

// LoadConfig загружает конфигурацию из файла с приоритетом переменных окружения
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()
	config.Path = path

	// Если файл не существует, загружаем из переменных окружения
	if _, err := os.Stat(path); os.IsNotExist(err) {
		loadConfigFromEnv(config)
		return config, nil
	}

	// Читаем файл
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла конфигурации: %w", err)
	}

	// Парсим YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("ошибка парсинга конфигурации: %w", err)
	}

	// Загружаем из переменных окружения (имеют приоритет над файлом)
	if err := loadConfigFromEnv(config); err != nil {
		return nil, fmt.Errorf("ошибка загрузки переменных окружения: %w", err)
	}

	return config, nil
}

// loadConfigFromEnv загружает конфигурацию из переменных окружения
func loadConfigFromEnv(config *Config) error {
	// API настройки
	if baseURL := os.Getenv("API_BASE_URL"); baseURL != "" {
		config.API.BaseURL = baseURL
	}
	if timeout := os.Getenv("API_TIMEOUT"); timeout != "" {
		if parsed, err := strconv.Atoi(timeout); err == nil {
			config.API.Timeout = parsed
		}
	}

	// gRPC настройки
	if schedulerAddr := os.Getenv("GRPC_SCHEDULER_ADDRESS"); schedulerAddr != "" {
		config.GRPC.SchedulerAddress = schedulerAddr
	}
	if coreAddr := os.Getenv("GRPC_CORE_ADDRESS"); coreAddr != "" {
		config.GRPC.CoreAddress = coreAddr
	}
	if authAddr := os.Getenv("GRPC_AUTH_ADDRESS"); authAddr != "" {
		config.GRPC.AuthAddress = authAddr
	}
	if useGRPC := os.Getenv("GRPC_USE_GRPC"); useGRPC != "" {
		if parsed, err := strconv.ParseBool(useGRPC); err == nil {
			config.GRPC.UseGRPC = parsed
		}
	}
	if timeout := os.Getenv("GRPC_TIMEOUT"); timeout != "" {
		if parsed, err := strconv.Atoi(timeout); err == nil {
			config.GRPC.Timeout = parsed
		}
	}

	// Аутентификация
	if tokenExpiry := os.Getenv("AUTH_TOKEN_EXPIRY"); tokenExpiry != "" {
		if parsed, err := strconv.Atoi(tokenExpiry); err == nil {
			config.Auth.TokenExpiry = parsed
		}
	}
	if refreshThreshold := os.Getenv("AUTH_REFRESH_THRESHOLD"); refreshThreshold != "" {
		if parsed, err := strconv.Atoi(refreshThreshold); err == nil {
			config.Auth.RefreshThreshold = parsed
		}
	}
	if accessSecret := os.Getenv("AUTH_ACCESS_SECRET"); accessSecret != "" {
		config.Auth.AccessSecret = accessSecret
	}
	if refreshSecret := os.Getenv("AUTH_REFRESH_SECRET"); refreshSecret != "" {
		config.Auth.RefreshSecret = refreshSecret
	}

	// Настройки вывода
	if format := os.Getenv("OUTPUT_FORMAT"); format != "" {
		config.Output.Format = format
	}
	if colors := os.Getenv("OUTPUT_COLORS"); colors != "" {
		if parsed, err := strconv.ParseBool(colors); err == nil {
			config.Output.Colors = parsed
		}
	}

	// Текущий тенант
	if tenant := os.Getenv("CURRENT_TENANT"); tenant != "" {
		config.CurrentTenant = tenant
	}

	return nil
}

// Save сохраняет конфигурацию в файл
func (c *Config) Save() error {
	if c.Path == "" {
		return fmt.Errorf("путь к файлу конфигурации не указан")
	}

	// Создаем директорию, если она не существует
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("ошибка создания директории: %w", err)
	}

	// Сериализуем в YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("ошибка сериализации конфигурации: %w", err)
	}

	// Записываем в файл
	if err := os.WriteFile(c.Path, data, 0644); err != nil {
		return fmt.Errorf("ошибка записи файла конфигурации: %w", err)
	}

	return nil
}

// GetConfigPath возвращает путь к файлу конфигурации
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ошибка получения домашней директории: %w", err)
	}

	return filepath.Join(home, ".uptimeping", "config.yaml"), nil
}

// InitConfig инициализирует конфигурацию в домашней директории пользователя
func InitConfig() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	config.Path = path

	// Сохраняем конфигурацию по умолчанию
	if err := config.Save(); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate проверяет валидность конфигурации
func (c *Config) Validate() error {
	// Проверяем URL
	if c.API.BaseURL == "" {
		return fmt.Errorf("API BaseURL не может быть пустым")
	}

	// Проверяем таймаут
	if c.API.Timeout <= 0 {
		return fmt.Errorf("API таймаут должен быть положительным числом")
	}

	// Проверяем формат вывода
	validFormats := map[string]bool{
		"table": true,
		"json":  true,
		"yaml":  true,
	}
	if !validFormats[c.Output.Format] {
		return fmt.Errorf("неверный формат вывода: %s", c.Output.Format)
	}

	return nil
}

// SetAPISettings устанавливает настройки API
func (c *Config) SetAPISettings(baseURL string, timeout int) {
	c.API.BaseURL = baseURL
	c.API.Timeout = timeout
}

// SetOutputSettings устанавливает настройки вывода
func (c *Config) SetOutputSettings(format string, colors bool) {
	c.Output.Format = format
	c.Output.Colors = colors
}

// SetCurrentTenant устанавливает текущий тенант
func (c *Config) SetCurrentTenant(tenantID string) {
	c.CurrentTenant = tenantID
}
