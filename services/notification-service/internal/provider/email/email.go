package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/domain"
)

// EmailProvider отправляет уведомления через SMTP
type EmailProvider struct {
	config EmailConfig
	logger logger.Logger
}

// EmailConfig конфигурация Email провайдера
type EmailConfig struct {
	SMTPHost     string        `json:"smtp_host" yaml:"smtp_host"`
	SMTPPort     int           `json:"smtp_port" yaml:"smtp_port"`
	Username     string        `json:"username" yaml:"username"`
	Password     string        `json:"password" yaml:"password"`
	FromAddress  string        `json:"from_address" yaml:"from_address"`
	FromName     string        `json:"from_name" yaml:"from_name"`
	UseTLS       bool          `json:"use_tls" yaml:"use_tls"`
	UseStartTLS  bool          `json:"use_starttls" yaml:"use_starttls"`
	InsecureSkipVerify bool     `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	Timeout      time.Duration `json:"timeout" yaml:"timeout"`
	RetryAttempts int           `json:"retry_attempts" yaml:"retry_attempts"`
}

// EmailTemplate структура email шаблона
type EmailTemplate struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}

// NewEmailProvider создает новый Email провайдер
func NewEmailProvider(config EmailConfig, logger logger.Logger) *EmailProvider {
	if config.SMTPPort == 0 {
		config.SMTPPort = 587
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.FromName == "" {
		config.FromName = "UptimePing Platform"
	}

	return &EmailProvider{
		config: config,
		logger: logger,
	}
}

// Send отправляет уведомление через Email
func (p *EmailProvider) Send(ctx context.Context, notification *domain.Notification) error {
	p.logger.Info("Sending email notification",
		logger.String("notification_id", notification.ID),
		logger.String("recipient", notification.Recipient),
	)

	// Генерация email шаблона
	template := p.generateTemplate(notification)

	// Отправка с retry логикой
	err := p.sendWithRetry(ctx, notification.Recipient, template)
	if err != nil {
		p.logger.Error("Failed to send email notification",
			logger.Error(err),
			logger.String("notification_id", notification.ID),
			logger.String("recipient", notification.Recipient),
		)
		return fmt.Errorf("failed to send email notification: %w", err)
	}

	p.logger.Info("Email notification sent successfully",
		logger.String("notification_id", notification.ID),
		logger.String("recipient", notification.Recipient),
	)

	return nil
}

// sendWithRetry отправляет email с retry логикой
func (p *EmailProvider) sendWithRetry(ctx context.Context, recipient string, template EmailTemplate) error {
	var lastErr error

	for attempt := 1; attempt <= p.config.RetryAttempts; attempt++ {
		if attempt > 1 {
			// Экспоненциальная backoff задержка
			delay := time.Duration(attempt-1) * time.Second
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			p.logger.Debug("Retrying email send",
				logger.Int("attempt", attempt),
				logger.Duration("delay", delay),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := p.sendEmail(ctx, recipient, template)
		if err == nil {
			return nil
		}

		lastErr = err
		p.logger.Warn("Email send attempt failed",
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

// sendEmail отправляет одно email
func (p *EmailProvider) sendEmail(ctx context.Context, recipient string, template EmailTemplate) error {
	// Создание SMTP адреса
	smtpAddr := fmt.Sprintf("%s:%d", p.config.SMTPHost, p.config.SMTPPort)

	// Создание SMTP клиента с TLS конфигурацией
	var smtpClient *smtp.Client
	var err error

	if p.config.UseTLS {
		// TLS соединение (обычно порт 465)
		tlsConfig := &tls.Config{
			InsecureSkipVerify: p.config.InsecureSkipVerify,
			ServerName:         p.config.SMTPHost,
		}

		smtpClient, err = smtp.Dial(smtpAddr)
		if err != nil {
			return fmt.Errorf("failed to dial SMTP server: %w", err)
		}

		tlsConn := smtpClient.StartTLS(tlsConfig)
		if tlsConn != nil {
			err = tlsConn
		}
	} else {
		// Обычное соединение с STARTTLS (обычно порт 587)
		smtpClient, err = smtp.Dial(smtpAddr)
		if err != nil {
			return fmt.Errorf("failed to dial SMTP server: %w", err)
		}

		if p.config.UseStartTLS {
			host, _, err := net.SplitHostPort(smtpAddr)
			if err != nil {
				return fmt.Errorf("failed to parse SMTP address: %w", err)
			}

			tlsConfig := &tls.Config{
				InsecureSkipVerify: p.config.InsecureSkipVerify,
				ServerName:         host,
			}

			err = smtpClient.StartTLS(tlsConfig)
			if err != nil {
				return fmt.Errorf("failed to start TLS: %w", err)
			}
		}
	}

	// Аутентификация
	auth := smtp.PlainAuth("", p.config.Username, p.config.Password, "")
	err = smtpClient.Auth(auth)
	if err != nil {
		smtpClient.Close()
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	// Установка отправителя
	from := fmt.Sprintf("%s <%s>", p.config.FromName, p.config.FromAddress)
	err = smtpClient.Mail(from)
	if err != nil {
		smtpClient.Close()
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Установка получателя
	err = smtpClient.Rcpt(recipient)
	if err != nil {
		smtpClient.Close()
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Создание письма с использованием Data
	message := p.buildEmailMessage(from, recipient, template)
	
	wc, err := smtpClient.Data()
	if err != nil {
		smtpClient.Close()
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	
	_, err = wc.Write([]byte(message))
	if err != nil {
		smtpClient.Close()
		return fmt.Errorf("failed to write email data: %w", err)
	}

	// Закрытие соединения
	err = smtpClient.Close()
	if err != nil {
		return fmt.Errorf("failed to close SMTP connection: %w", err)
	}

	return nil
}

// buildEmailMessage строит email сообщение
func (p *EmailProvider) buildEmailMessage(from, to string, template EmailTemplate) string {
	var message strings.Builder

	// Заголовки
	message.WriteString(fmt.Sprintf("From: %s\r\n", from))
	message.WriteString(fmt.Sprintf("To: %s\r\n", to))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", template.Subject))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: multipart/alternative; boundary=BOUNDARY\r\n")
	message.WriteString("\r\n")

	// Текстовая часть
	message.WriteString("--BOUNDARY\r\n")
	message.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	message.WriteString(template.Text)
	message.WriteString("\r\n")

	// HTML часть
	message.WriteString("--BOUNDARY\r\n")
	message.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
	message.WriteString(template.HTML)
	message.WriteString("\r\n")

	// Завершение multipart
	message.WriteString("--BOUNDARY--\r\n")

	return message.String()
}

// generateTemplate генерирует email шаблон
func (p *EmailProvider) generateTemplate(notification *domain.Notification) EmailTemplate {
	var severityColor string
	var severityIcon string
	
	switch notification.Severity {
	case domain.SeverityCritical:
		severityColor = "#dc3545" // red
		severityIcon = "🔴"
	case domain.SeverityHigh:
		severityColor = "#fd7e14" // orange
		severityIcon = "🟠"
	case domain.SeverityMedium:
		severityColor = "#ffc107" // yellow
		severityIcon = "🟡"
	case domain.SeverityLow:
		severityColor = "#28a745" // green
		severityIcon = "🟢"
	default:
		severityColor = "#6c757d" // gray
		severityIcon = "ℹ️"
	}

	// HTML шаблон
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background-color: %s;
            color: white;
            padding: 20px;
            border-radius: 5px 5px 0 0;
            text-align: center;
        }
        .content {
            background-color: #f8f9fa;
            padding: 20px;
            border: 1px solid #dee2e6;
            border-top: none;
            border-radius: 0 0 5px 5px;
        }
        .details {
            margin-top: 20px;
        }
        .detail-row {
            display: flex;
            justify-content: space-between;
            padding: 5px 0;
            border-bottom: 1px solid #e9ecef;
        }
        .detail-label {
            font-weight: bold;
        }
        .footer {
            margin-top: 20px;
            padding-top: 20px;
            border-top: 1px solid #dee2e6;
            text-align: center;
            color: #6c757d;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>%s %s</h1>
    </div>
    <div class="content">
        <p>%s</p>
        
        <div class="details">
            <div class="detail-row">
                <span class="detail-label">Type:</span>
                <span>%s</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Severity:</span>
                <span>%s</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Time:</span>
                <span>%s</span>
            </div>
            <div class="detail-row">
                <span class="detail-label">Tenant:</span>
                <span>%s</span>
            </div>
        </div>
        
        %s
    </div>
    <div class="footer">
        <p>Sent by UptimePing Platform</p>
    </div>
</body>
</html>`,
		notification.Subject,
		severityIcon,
		notification.Subject,
		notification.Body,
		severityColor,
		notification.Type,
		notification.Severity,
		notification.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		notification.TenantID,
		p.formatAdditionalData(notification.Data),
	)

	// Текстовый шаблон
	text := fmt.Sprintf(`%s %s

%s

---
Details:
Type: %s
Severity: %s
Time: %s
Tenant: %s
%s

---
Sent by UptimePing Platform`,
		severityIcon,
		notification.Subject,
		notification.Body,
		notification.Type,
		notification.Severity,
		notification.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		notification.TenantID,
		p.formatAdditionalDataText(notification.Data),
	)

	return EmailTemplate{
		Subject: notification.Subject,
		HTML:    html,
		Text:    text,
	}
}

// formatAdditionalData форматирует дополнительные данные для HTML
func (p *EmailProvider) formatAdditionalData(data map[string]interface{}) string {
	if len(data) == 0 {
		return ""
	}

	var html strings.Builder
	html.WriteString(`<div class="details">`)
	html.WriteString(`<h3>Additional Information</h3>`)
	
	for key, value := range data {
		html.WriteString(fmt.Sprintf(`<div class="detail-row">
            <span class="detail-label">%s:</span>
            <span>%v</span>
        </div>`, key, value))
	}
	
	html.WriteString(`</div>`)
	return html.String()
}

// formatAdditionalDataText форматирует дополнительные данные для текста
func (p *EmailProvider) formatAdditionalDataText(data map[string]interface{}) string {
	if len(data) == 0 {
		return ""
	}

	var text strings.Builder
	text.WriteString("Additional Information:\n")
	
	for key, value := range data {
		text.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
	}
	
	return text.String()
}

// shouldRetry определяет, нужно ли повторять попытку
func (p *EmailProvider) shouldRetry(err error) bool {
	// Проверяем на ошибки, которые требуют повторной попытки
	errStr := err.Error()
	
	// Network ошибки
	if contains(errStr, "connection refused") ||
	   contains(errStr, "timeout") ||
	   contains(errStr, "network") ||
	   contains(errStr, "temporary") ||
	   contains(errStr, "connection reset") ||
	   contains(errStr, "no such host") ||
	   contains(errStr, "connection refused") {
		return true
	}
	
	// Rate limiting ошибки
	if contains(errStr, "rate limited") ||
	   contains(errStr, "too many requests") ||
	   contains(errStr, "rate limit") ||
	   contains(errStr, "quota exceeded") {
		return true
	}
	
	// Server ошибки
	if contains(errStr, "internal server error") ||
	   contains(errStr, "service unavailable") ||
	   contains(errStr, "bad gateway") ||
	   contains(errStr, "service temporarily unavailable") {
		return true
	}
	
	// Database ошибки
	if contains(errStr, "connection lost") ||
	   contains(errStr, "database locked") ||
	   contains(errStr, "deadlock") ||
	   contains(errStr, "connection timed out") {
		return true
	}
	
	// HTTP ошибки
	if contains(errStr, "502") ||
	   contains(errStr, "503") ||
	   contains(errStr, "504") ||
	   contains(errStr, "507") ||
	   contains(errStr, "509") ||
	   contains(errStr, "429") {
		return true
	}
	
	// Не retryable ошибки
	if contains(errStr, "unauthorized") ||
	   contains(errStr, "forbidden") ||
	   contains(errStr, "not found") ||
	   contains(errStr, "bad request") ||
	   contains(errStr, "validation") ||
	   contains(errStr, "invalid") ||
	   contains(errStr, "authentication") ||
	   contains(errStr, "permission") {
		return false
	}
	
	// Context cancellation не требует retry
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	
	// По умолчанию считаем ошибку retryable
	return true
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

// GetType возвращает тип провайдера
func (p *EmailProvider) GetType() string {
	return "email"
}

// IsHealthy проверяет здоровье провайдера
func (p *EmailProvider) IsHealthy(ctx context.Context) bool {
	// Проверка здоровья через тестовое соединение к SMTP
	smtpAddr := fmt.Sprintf("%s:%d", p.config.SMTPHost, p.config.SMTPPort)
	
	client, err := smtp.Dial(smtpAddr)
	if err != nil {
		return false
	}
	defer client.Close()

	// Проверяем, что сервер отвечает
	return client != nil
}

// GetStats возвращает статистику провайдера
func (p *EmailProvider) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":                "email",
		"smtp_host":           p.config.SMTPHost,
		"smtp_port":           p.config.SMTPPort,
		"use_tls":             p.config.UseTLS,
		"use_starttls":        p.config.UseStartTLS,
		"insecure_skip_verify": p.config.InsecureSkipVerify,
		"timeout":             p.config.Timeout.String(),
		"retry_attempts":      p.config.RetryAttempts,
		"healthy":             p.IsHealthy(context.Background()),
	}
}
