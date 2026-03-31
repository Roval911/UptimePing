package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// ChannelConfig представляет конфигурацию канала уведомлений
type ChannelConfig struct {
	ID        string                 `json:"id" db:"id"`
	TenantID  string                 `json:"tenant_id" db:"tenant_id"`
	Name      string                 `json:"name" db:"name"`
	Type      string                 `json:"type" db:"type"` // email, telegram, slack
	Config    map[string]interface{} `json:"config" db:"config"`
	IsActive  bool                   `json:"is_active" db:"is_active"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" db:"updated_at"`
}

// ConfigJSON для работы с JSON в базе данных
type ConfigJSON map[string]interface{}

// Value реализует driver.Valuer для сохранения в базу
func (c ConfigJSON) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan реализует sql.Scanner для чтения из базы
func (c *ConfigJSON) Scan(value interface{}) error {
	if value == nil {
		*c = make(ConfigJSON)
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, &c)
	case string:
		return json.Unmarshal([]byte(v), &c)
	default:
		return nil
	}
}

// EmailConfig содержит специфичные настройки для email
type EmailConfig struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUser     string `json:"smtp_user"`
	SMTPPassword string `json:"smtp_password"`
	FromEmail    string `json:"from_email"`
	FromName     string `json:"from_name"`
	UseTLS       bool   `json:"use_tls"`
	UseStartTLS  bool   `json:"use_starttls"`
	Timeout      int    `json:"timeout"` // в секундах
}

// TelegramConfig содержит специфичные настройки для Telegram
type TelegramConfig struct {
	BotToken      string `json:"bot_token"`
	ChatID        string `json:"chat_id"`
	APIURL        string `json:"api_url"`
	Timeout       int    `json:"timeout"` // в секундах
	RetryAttempts int    `json:"retry_attempts"`
}

// SlackConfig содержит специфичные настройки для Slack
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel"`
	Username   string `json:"username"`
	IconEmoji  string `json:"icon_emoji"`
}
