package config

import (
	"time"

	"UptimePingPlatform/services/notification-service/internal/provider/email"
	"UptimePingPlatform/services/notification-service/internal/provider/retry"
	"UptimePingPlatform/services/notification-service/internal/provider/slack"
	"UptimePingPlatform/services/notification-service/internal/provider/telegram"
)

// ProvidersConfig конфигурация провайдеров уведомлений
type ProvidersConfig struct {
	Telegram telegram.TelegramConfig `json:"telegram" yaml:"telegram"`
	Slack    slack.SlackConfig    `json:"slack" yaml:"slack"`
	Email    email.EmailConfig    `json:"email" yaml:"email"`
	Retry    retry.RetryConfig    `json:"retry" yaml:"retry"`
}

// DefaultProvidersConfig возвращает конфигурацию по умолчанию
func DefaultProvidersConfig() ProvidersConfig {
	return ProvidersConfig{
		Telegram: telegram.TelegramConfig{
			APIURL:        "https://api.telegram.org",
			Timeout:       30 * time.Second,
			RetryAttempts: 3,
		},
		Slack: slack.SlackConfig{
			APIURL:        "https://slack.com/api",
			Timeout:       30 * time.Second,
			RetryAttempts: 3,
		},
		Email: email.EmailConfig{
			SMTPPort:     587,
			UseStartTLS:  true,
			Timeout:      30 * time.Second,
			RetryAttempts: 3,
			FromName:     "UptimePing Platform",
		},
		Retry: retry.DefaultRetryConfig(),
	}
}
