package domain

import (
	"time"
)

// Event представляет событие системы
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Severity  string                 `json:"severity"`
	TenantID  string                 `json:"tenant_id"`
	Source    string                 `json:"source"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// Notification представляет уведомление
type Notification struct {
	ID          string                 `json:"id"`
	EventID     string                 `json:"event_id"`
	Type        string                 `json:"type"`
	Channel     string                 `json:"channel"`
	Recipient   string                 `json:"recipient"`
	Subject     string                 `json:"subject"`
	Body        string                 `json:"body"`
	TenantID    string                 `json:"tenant_id"`
	Severity    string                 `json:"severity"`
	Status      string                 `json:"status"`
	Data        map[string]interface{} `json:"data"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	SentAt      *time.Time             `json:"sent_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	MaxRetries  int                    `json:"max_retries"`
}

// NotificationGroup представляет группу уведомлений
type NotificationGroup struct {
	ID           string                    `json:"id"`
	Type         string                    `json:"type"`
	Channel      string                    `json:"channel"`
	Recipient    string                    `json:"recipient"`
	TenantID     string                    `json:"tenant_id"`
	Notifications []*Notification          `json:"notifications"`
	GroupData    map[string]interface{}    `json:"group_data"`
	Metadata     map[string]interface{}    `json:"metadata"`
	CreatedAt    time.Time                 `json:"created_at"`
	ProcessedAt  *time.Time                `json:"processed_at,omitempty"`
}

// Статусы уведомлений
const (
	NotificationStatusPending   = "pending"
	NotificationStatusSending   = "sending"
	NotificationStatusSent      = "sent"
	NotificationStatusFailed    = "failed"
	NotificationStatusCancelled = "cancelled"
)

// Уровни серьезности
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Типы уведомлений
const (
	NotificationTypeIncidentCreated = "incident.created"
	NotificationTypeIncidentUpdated = "incident.updated"
	NotificationTypeIncidentResolved = "incident.resolved"
	NotificationTypeCheckFailed     = "check.failed"
	NotificationTypeCheckRecovered  = "check.recovered"
	NotificationTypeSystemAlert     = "system.alert"
)

// Каналы уведомлений
const (
	ChannelEmail = "email"
	ChannelSlack = "slack"
	ChannelSMS   = "sms"
	ChannelWebhook = "webhook"
)

// GetGroupKey возвращает ключ для группировки
func (e *Event) GetGroupKey() string {
	// Группировка по tenant_id, типу и каналу
	return e.TenantID + ":" + e.Type + ":" + e.Severity
}

// ShouldGroup определяет, нужно ли группировать это событие
func (e *Event) ShouldGroup() bool {
	// Группируем только инциденты и проверки
	switch e.Type {
	case NotificationTypeIncidentCreated,
		 NotificationTypeIncidentUpdated,
		 NotificationTypeCheckFailed:
		return true
	default:
		return false
	}
}

// GetNotificationPriority возвращает приоритет уведомления
func (n *Notification) GetNotificationPriority() int {
	switch n.Severity {
	case SeverityCritical:
		return 1
	case SeverityHigh:
		return 2
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 4
	default:
		return 5
	}
}

// CanRetry проверяет, можно ли повторить отправку
func (n *Notification) CanRetry() bool {
	return n.RetryCount < n.MaxRetries && n.Status == NotificationStatusFailed
}

// MarkAsSent отмечает уведомление как отправленное
func (n *Notification) MarkAsSent() {
	n.Status = NotificationStatusSent
	now := time.Now()
	n.SentAt = &now
}

// MarkAsFailed отмечает уведомление как неуспешное
func (n *Notification) MarkAsFailed(err error) {
	n.Status = NotificationStatusFailed
	n.Error = err.Error()
	n.RetryCount++
}

// GetRetryDelay возвращает задержку для повторной попытки
func (n *Notification) GetRetryDelay() time.Duration {
	// Экспоненциальная задержка: 30s, 60s, 120s
	baseDelay := 30 * time.Second
	return baseDelay * time.Duration(1<<(n.RetryCount-1))
}

// IsExpired проверяет, истекло ли время жизни уведомления
func (n *Notification) IsExpired() bool {
	// Уведомления считаются истекшими через 24 часа
	expiryTime := 24 * time.Hour
	return time.Since(n.CreatedAt) > expiryTime
}

// ToMap преобразует уведомление в map для шаблонов
func (n *Notification) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"id":           n.ID,
		"event_id":     n.EventID,
		"type":         n.Type,
		"channel":      n.Channel,
		"recipient":    n.Recipient,
		"subject":      n.Subject,
		"body":         n.Body,
		"tenant_id":    n.TenantID,
		"severity":     n.Severity,
		"status":       n.Status,
		"created_at":   n.CreatedAt,
		"retry_count":  n.RetryCount,
		"max_retries":  n.MaxRetries,
	}

	// Добавляем данные если есть
	if len(n.Data) > 0 {
		result["data"] = n.Data
	}

	// Добавляем метаданные если есть
	if len(n.Metadata) > 0 {
		result["metadata"] = n.Metadata
	}

	// Добавляем время отправки если есть
	if n.SentAt != nil {
		result["sent_at"] = *n.SentAt
	}

	// Добавляем ошибку если есть
	if n.Error != "" {
		result["error"] = n.Error
	}

	return result
}

// ToMap преобразует событие в map для шаблонов
func (e *Event) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"id":        e.ID,
		"type":      e.Type,
		"severity":  e.Severity,
		"tenant_id": e.TenantID,
		"source":    e.Source,
		"title":     e.Title,
		"message":   e.Message,
		"timestamp": e.Timestamp,
	}

	// Добавляем данные если есть
	if len(e.Data) > 0 {
		result["data"] = e.Data
	}

	// Добавляем метаданные если есть
	if len(e.Metadata) > 0 {
		result["metadata"] = e.Metadata
	}

	return result
}
