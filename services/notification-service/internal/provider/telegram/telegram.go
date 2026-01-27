package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/domain"
)

// TelegramProvider отправляет уведомления через Telegram Bot API
type TelegramProvider struct {
	config TelegramConfig
	logger logger.Logger
	client *http.Client
}

// TelegramConfig конфигурация Telegram провайдера
type TelegramConfig struct {
	BotToken      string        `json:"bot_token" yaml:"bot_token"`
	APIURL        string        `json:"api_url" yaml:"api_url"`
	Timeout       time.Duration `json:"timeout" yaml:"timeout"`
	RetryAttempts int           `json:"retry_attempts" yaml:"retry_attempts"`
}

// TelegramMessage структура сообщения Telegram
type TelegramMessage struct {
	ChatID    interface{} `json:"chat_id"`
	Text      string      `json:"text"`
	ParseMode string      `json:"parse_mode,omitempty"`
}

// TelegramResponse структура ответа Telegram API
type TelegramResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
}

// NewTelegramProvider создает новый Telegram провайдер
func NewTelegramProvider(config TelegramConfig, logger logger.Logger) *TelegramProvider {
	if config.APIURL == "" {
		config.APIURL = "https://api.telegram.org"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}

	return &TelegramProvider{
		config: config,
		logger: logger,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Send отправляет уведомление через Telegram
func (p *TelegramProvider) Send(ctx context.Context, notification *domain.Notification) error {
	p.logger.Info("Sending Telegram notification",
		logger.String("notification_id", notification.ID),
		logger.String("chat_id", notification.Recipient),
	)

	// Форматирование сообщения для Telegram
	message := p.formatMessage(notification)

	// Создание запроса к Telegram API
	telegramMsg := TelegramMessage{
		ChatID:    p.parseChatID(notification.Recipient),
		Text:      message,
		ParseMode: "HTML",
	}

	// Отправка с retry логикой
	err := p.sendWithRetry(ctx, telegramMsg)
	if err != nil {
		p.logger.Error("Failed to send Telegram notification",
			logger.Error(err),
			logger.String("notification_id", notification.ID),
			logger.String("chat_id", notification.Recipient),
		)
		return fmt.Errorf("failed to send Telegram notification: %w", err)
	}

	p.logger.Info("Telegram notification sent successfully",
		logger.String("notification_id", notification.ID),
		logger.String("chat_id", notification.Recipient),
	)

	return nil
}

// sendWithRetry отправляет сообщение с retry логикой
func (p *TelegramProvider) sendWithRetry(ctx context.Context, message TelegramMessage) error {
	var lastErr error

	for attempt := 1; attempt <= p.config.RetryAttempts; attempt++ {
		if attempt > 1 {
			// Экспоненциальная backoff задержка
			delay := time.Duration(attempt-1) * time.Second
			p.logger.Debug("Retrying Telegram send",
				logger.Int("attempt", attempt),
				logger.Duration("delay", delay),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := p.sendMessage(ctx, message)
		if err == nil {
			return nil
		}

		lastErr = err
		p.logger.Warn("Telegram send attempt failed",
			logger.Error(err),
			logger.Int("attempt", attempt),
		)

		// Проверяем, не стоит ли прекращать попытки
		if !p.shouldRetry(err) {
			break
		}
	}

	return lastErr
}

// sendMessage отправляет одно сообщение в Telegram
func (p *TelegramProvider) sendMessage(ctx context.Context, message TelegramMessage) error {
	// Сериализация сообщения
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Telegram message: %w", err)
	}

	// Создание запроса
	url := fmt.Sprintf("%s/bot%s/sendMessage", p.config.APIURL, p.config.BotToken)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Отправка запроса
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Парсинг ответа
	var telegramResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверка ответа
	if !telegramResp.OK {
		return fmt.Errorf("Telegram API error: %d - %s", telegramResp.ErrorCode, telegramResp.Description)
	}

	return nil
}

// formatMessage форматирует сообщение для Telegram
func (p *TelegramProvider) formatMessage(notification *domain.Notification) string {
	var severityIcon string
	switch notification.Severity {
	case domain.SeverityCritical:
		severityIcon = "🔴"
	case domain.SeverityHigh:
		severityIcon = "🟠"
	case domain.SeverityMedium:
		severityIcon = "🟡"
	case domain.SeverityLow:
		severityIcon = "🟢"
	default:
		severityIcon = "ℹ️"
	}

	// Базовое форматирование
	var message bytes.Buffer

	// Заголовок с иконкой серьезности
	message.WriteString(fmt.Sprintf("%s <b>%s</b>\n\n", severityIcon, notification.Subject))

	// Основное сообщение
	message.WriteString(fmt.Sprintf("%s\n\n", notification.Body))

	// Метаданные
	message.WriteString("<b>Details:</b>\n")
	message.WriteString(fmt.Sprintf("• <b>Type:</b> %s\n", notification.Type))
	message.WriteString(fmt.Sprintf("• <b>Severity:</b> %s\n", notification.Severity))
	message.WriteString(fmt.Sprintf("• <b>Time:</b> %s\n", notification.CreatedAt.Format("2006-01-02 15:04:05 UTC")))

	// Дополнительные данные если есть
	if len(notification.Data) > 0 {
		message.WriteString("\n<b>Additional Info:</b>\n")
		for key, value := range notification.Data {
			message.WriteString(fmt.Sprintf("• <b>%s:</b> %v\n", key, value))
		}
	}

	// Подпись
	message.WriteString("\n<i>Sent by UptimePing Platform</i>")

	return message.String()
}

// parseChatID парсит ID чата из строки
func (p *TelegramProvider) parseChatID(recipient string) interface{} {
	// Если recipient начинается с @, это username
	if len(recipient) > 0 && recipient[0] == '@' {
		return recipient
	}

	// Иначе пытаем преобразовать в число (chat_id)
	var chatID int64
	_, err := fmt.Sscanf(recipient, "%d", &chatID)
	if err == nil {
		return chatID
	}

	// Если не удалось, возвращаем как есть
	return recipient
}

// shouldRetry определяет, нужно ли повторять попытку
func (p *TelegramProvider) shouldRetry(err error) bool {
	//todo Здесь можно добавить логику для определения ошибок,
	// которые требуют повторной попытки
	// Например: network errors, timeouts, rate limiting

	// Для простоты всегда возвращаем true для всех ошибок,
	// кроме context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	return true
}

// GetType возвращает тип провайдера
func (p *TelegramProvider) GetType() string {
	return "telegram"
}

// IsHealthy проверяет здоровье провайдера
func (p *TelegramProvider) IsHealthy(ctx context.Context) bool {
	// Проверка здоровья через getMe метод Telegram API
	url := fmt.Sprintf("%s/bot%s/getMe", p.config.APIURL, p.config.BotToken)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetStats возвращает статистику провайдера
func (p *TelegramProvider) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":           "telegram",
		"api_url":        p.config.APIURL,
		"timeout":        p.config.Timeout.String(),
		"retry_attempts": p.config.RetryAttempts,
		"healthy":        p.IsHealthy(context.Background()),
	}
}
