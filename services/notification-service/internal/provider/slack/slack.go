package slack

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

// SlackProvider отправляет уведомления через Slack Web API
type SlackProvider struct {
	config SlackConfig
	logger logger.Logger
	client *http.Client
}

// SlackConfig конфигурация Slack провайдера
type SlackConfig struct {
	BotToken      string        `json:"bot_token" yaml:"bot_token"`
	WebhookURL    string        `json:"webhook_url" yaml:"webhook_url"`
	APIURL        string        `json:"api_url" yaml:"api_url"`
	Timeout       time.Duration `json:"timeout" yaml:"timeout"`
	RetryAttempts int           `json:"retry_attempts" yaml:"retry_attempts"`
}

// SlackMessage структура сообщения Slack
type SlackMessage struct {
	Channel     string       `json:"channel"`
	Text        string       `json:"text,omitempty"`
	Blocks      []Block      `json:"blocks,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Block структура блока Slack
type Block struct {
	Type string      `json:"type"`
	Text *TextBlock  `json:"text,omitempty"`
	Accessory *AccessoryBlock `json:"accessory,omitempty"`
	Fields []FieldBlock `json:"fields,omitempty"`
}

// TextBlock структура текстового блока
type TextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Emoji bool `json:"emoji,omitempty"`
}

// AccessoryBlock структура аксессуарного блока
type AccessoryBlock struct {
	Type     string `json:"type"`
	Text     *TextBlock `json:"text,omitempty"`
	Value    string `json:"value,omitempty"`
	Url      string `json:"url,omitempty"`
}

// FieldBlock структура поля блока
type FieldBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Short bool `json:"short,omitempty"`
}

// Attachment структура вложения Slack
type Attachment struct {
	Color     string  `json:"color"`
	Title     string  `json:"title"`
	Text      string  `json:"text"`
	Fields    []FieldBlock `json:"fields,omitempty"`
	Timestamp int64   `json:"ts,omitempty"`
	Footer    string  `json:"footer,omitempty"`
}

// SlackResponse структура ответа Slack API
type SlackResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// NewSlackProvider создает новый Slack провайдер
func NewSlackProvider(config SlackConfig, logger logger.Logger) *SlackProvider {
	if config.APIURL == "" {
		config.APIURL = "https://slack.com/api"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}

	return &SlackProvider{
		config: config,
		logger: logger,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Send отправляет уведомление через Slack
func (p *SlackProvider) Send(ctx context.Context, notification *domain.Notification) error {
	p.logger.Info("Sending Slack notification",
		logger.String("notification_id", notification.ID),
		logger.String("channel", notification.Recipient),
	)

	// Форматирование сообщения для Slack
	message := p.formatMessage(notification)

	// Отправка с retry логикой
	err := p.sendWithRetry(ctx, message)
	if err != nil {
		p.logger.Error("Failed to send Slack notification",
			logger.Error(err),
			logger.String("notification_id", notification.ID),
			logger.String("channel", notification.Recipient),
		)
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}

	p.logger.Info("Slack notification sent successfully",
		logger.String("notification_id", notification.ID),
		logger.String("channel", notification.Recipient),
	)

	return nil
}

// sendWithRetry отправляет сообщение с retry логикой
func (p *SlackProvider) sendWithRetry(ctx context.Context, message SlackMessage) error {
	var lastErr error

	for attempt := 1; attempt <= p.config.RetryAttempts; attempt++ {
		if attempt > 1 {
			// Экспоненциальная backoff задержка
			delay := time.Duration(attempt-1) * time.Second
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			p.logger.Debug("Retrying Slack send",
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
		p.logger.Warn("Slack send attempt failed",
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

// sendMessage отправляет одно сообщение в Slack
func (p *SlackProvider) sendMessage(ctx context.Context, message SlackMessage) error {
	var url string
	
	// Выбор метода отправки: Webhook или Bot API
	if p.config.WebhookURL != "" {
		url = p.config.WebhookURL
	} else {
		url = fmt.Sprintf("%s/chat.postMessage", p.config.APIURL)
	}

	// Сериализация сообщения
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	// Создание запроса
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Добавление авторизации для Bot API
	if p.config.BotToken != "" && p.config.WebhookURL == "" {
		req.Header.Set("Authorization", "Bearer "+p.config.BotToken)
	}

	// Отправка запроса
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Парсинг ответа
	var slackResp SlackResponse
	if err := json.NewDecoder(resp.Body).Decode(&slackResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверка ответа
	if !slackResp.OK {
		return fmt.Errorf("Slack API error: %s", slackResp.Error)
	}

	return nil
}

// formatMessage форматирует сообщение для Slack с использованием блоков
func (p *SlackProvider) formatMessage(notification *domain.Notification) SlackMessage {
	var color string
	var severityIcon string
	
	switch notification.Severity {
	case domain.SeverityCritical:
		color = "#dc3545" // red
		severityIcon = "🔴"
	case domain.SeverityHigh:
		color = "#fd7e14" // orange
		severityIcon = "🟠"
	case domain.SeverityMedium:
		color = "#ffc107" // yellow
		severityIcon = "🟡"
	case domain.SeverityLow:
		color = "#28a745" // green
		severityIcon = "🟢"
	default:
		color = "#6c757d" // gray
		severityIcon = "ℹ️"
	}

	// Создание блоков для сообщения
	blocks := make([]Block, 0)

	// Заголовок
	headerBlock := Block{
		Type: "header",
		Text: &TextBlock{
			Type: "plain_text",
			Text: fmt.Sprintf("%s %s", severityIcon, notification.Subject),
			Emoji: true,
		},
	}
	blocks = append(blocks, headerBlock)

	// Основной текст
	if notification.Body != "" {
		textBlock := Block{
			Type: "section",
			Text: &TextBlock{
				Type: "mrkdwn",
				Text: notification.Body,
			},
		}
		blocks = append(blocks, textBlock)
	}

	// Поля с информацией
	fields := []FieldBlock{
		{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Type:*\n%s", notification.Type),
			Short: true,
		},
		{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Severity:*\n%s", notification.Severity),
			Short: true,
		},
		{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Time:*\n%s", notification.CreatedAt.Format("2006-01-02 15:04:05 UTC")),
			Short: true,
		},
		{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Tenant:*\n%s", notification.TenantID),
			Short: true,
		},
	}

	// Дополнительные данные если есть
	if len(notification.Data) > 0 {
		var dataText string
		for key, value := range notification.Data {
			dataText += fmt.Sprintf("*%s:* %v\n", key, value)
		}
		
		dataBlock := Block{
			Type: "section",
			Text: &TextBlock{
				Type: "mrkdwn",
				Text: dataText,
			},
		}
		blocks = append(blocks, dataBlock)
	}

	// Блок с полями
	fieldsBlock := Block{
		Type: "section",
		Fields: fields,
	}
	blocks = append(blocks, fieldsBlock)

	// Разделитель
	dividerBlock := Block{
		Type: "divider",
	}
	blocks = append(blocks, dividerBlock)

	// Создание сообщения
	message := SlackMessage{
		Channel: p.parseChannel(notification.Recipient),
		Blocks:  blocks,
	}

	// Добавление вложения для цветной индикации
	attachment := Attachment{
		Color:  color,
		Footer: "UptimePing Platform",
		Timestamp: time.Now().Unix(),
	}
	message.Attachments = []Attachment{attachment}

	return message
}

// parseChannel парсит канал из строки
func (p *SlackProvider) parseChannel(recipient string) string {
	// Если начинается с #, это канал
	if len(recipient) > 0 && recipient[0] == '#' {
		return recipient
	}
	
	// Если начинается с @, это пользователь
	if len(recipient) > 0 && recipient[0] == '@' {
		return recipient
	}
	
	// Иначе добавляем # для канала
	if !contains(recipient, "#") && !contains(recipient, "@") {
		return "#" + recipient
	}
	
	return recipient
}

// contains проверяет наличие подстроки
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr ||
			 indexOf(s, substr) >= 0)))
}

// indexOf возвращает индекс подстроки
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// shouldRetry определяет, нужно ли повторять попытку
func (p *SlackProvider) shouldRetry(err error) bool {
	// Проверяем на ошибки, которые требуют повторной попытки
	errStr := err.Error()
	
	// Network errors
	if contains(errStr, "connection refused") ||
	   contains(errStr, "timeout") ||
	   contains(errStr, "network") ||
	   contains(errStr, "temporary") {
		return true
	}
	
	// Rate limiting
	if contains(errStr, "rate_limited") ||
	   contains(errStr, "too many requests") {
		return true
	}
	
	// Server errors
	if contains(errStr, "internal_server_error") ||
	   contains(errStr, "service_unavailable") {
		return true
	}
	
	// Context cancellation не требует retry
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	
	return false
}

// GetType возвращает тип провайдера
func (p *SlackProvider) GetType() string {
	return "slack"
}

// IsHealthy проверяет здоровье провайдера
func (p *SlackProvider) IsHealthy(ctx context.Context) bool {
	// Проверка здоровья через auth.test метод Slack API
	if p.config.BotToken == "" {
		// Если нет токена, проверяем webhook
		return p.config.WebhookURL != ""
	}

	url := fmt.Sprintf("%s/auth.test", p.config.APIURL)
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return false
	}

	req.Header.Set("Authorization", "Bearer "+p.config.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetStats возвращает статистику провайдера
func (p *SlackProvider) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":           "slack",
		"api_url":        p.config.APIURL,
		"has_webhook":    p.config.WebhookURL != "",
		"has_bot_token":  p.config.BotToken != "",
		"timeout":        p.config.Timeout.String(),
		"retry_attempts": p.config.RetryAttempts,
		"healthy":        p.IsHealthy(context.Background()),
	}
}
