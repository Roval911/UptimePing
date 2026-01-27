package domain

import "fmt"

// InteractiveConfig представляет интерактивную конфигурацию
type InteractiveConfig struct {
	Server      ServerConfig   `json:"server"`
	Database    DatabaseConfig `json:"database"`
	Redis       RedisConfig    `json:"redis"`
	Telegram    TelegramConfig `json:"telegram"`
	Email       EmailConfig    `json:"email"`
	Logger      LoggerConfig   `json:"logger"`
	Environment string         `json:"environment"`
	Services    map[string]*ServiceConfig `json:"services"`
}

// ServerConfig представляет конфигурацию сервера
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// DatabaseConfig представляет конфигурацию базы данных
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// RedisConfig представляет конфигурацию Redis
type RedisConfig struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

// TelegramConfig представляет конфигурацию Telegram
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
	Enabled  bool   `json:"enabled"`
}

// EmailConfig представляет конфигурацию Email
type EmailConfig struct {
	SMTPHost    string `json:"smtp_host"`
	SMTPPort    int    `json:"smtp_port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
	Enabled     bool   `json:"enabled"`
}

// LoggerConfig представляет конфигурацию логгера
type LoggerConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// ServiceConfig представляет конфигурацию gRPC сервиса
type ServiceConfig struct {
	Host            string   `json:"host"`
	Port            int      `json:"port"`
	DefaultTimeout  string   `json:"default_timeout"`
	EnabledMethods  []string `json:"enabled_methods"`
	DisabledMethods []string `json:"disabled_methods"`
}

// Validate валидирует конфигурацию
func (c *InteractiveConfig) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Database.Port)
	}

	if c.Database.Name == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	if c.Database.User == "" {
		return fmt.Errorf("database user cannot be empty")
	}

	if c.Email.SMTPPort < 1 || c.Email.SMTPPort > 65535 {
		return fmt.Errorf("invalid SMTP port: %d", c.Email.SMTPPort)
	}

	if c.Telegram.Enabled && c.Telegram.BotToken == "" {
		return fmt.Errorf("telegram bot token cannot be empty when telegram is enabled")
	}

	if c.Telegram.Enabled && c.Telegram.ChatID == "" {
		return fmt.Errorf("telegram chat ID cannot be empty when telegram is enabled")
	}

	if c.Email.Enabled && c.Email.SMTPHost == "" {
		return fmt.Errorf("SMTP host cannot be empty when email is enabled")
	}

	if c.Email.Enabled && c.Email.FromAddress == "" {
		return fmt.Errorf("from address cannot be empty when email is enabled")
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLogLevels[c.Logger.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logger.Level)
	}

	validLogFormats := map[string]bool{
		"json": true,
		"text": true,
	}

	if !validLogFormats[c.Logger.Format] {
		return fmt.Errorf("invalid log format: %s", c.Logger.Format)
	}

	validEnvironments := map[string]bool{
		"dev":         true,
		"staging":     true,
		"prod":        true,
		"development": true,
		"production":  true,
	}

	if !validEnvironments[c.Environment] {
		return fmt.Errorf("invalid environment: %s", c.Environment)
	}

	return nil
}
