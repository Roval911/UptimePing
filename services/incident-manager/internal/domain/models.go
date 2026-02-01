package domain

import (
	"time"
)

// IncidentStatus представляет статус инцидента
type IncidentStatus string

const (
	IncidentStatusOpen         IncidentStatus = "open"
	IncidentStatusAcknowledged IncidentStatus = "acknowledged"
	IncidentStatusResolved     IncidentStatus = "resolved"
)

// IncidentSeverity представляет уровень серьезности инцидента
type IncidentSeverity string

const (
	IncidentSeverityLow      IncidentSeverity = "low"
	IncidentSeverityMedium   IncidentSeverity = "medium"
	IncidentSeverityHigh     IncidentSeverity = "high"
	IncidentSeverityCritical IncidentSeverity = "critical"
)

// Incident представляет сущность инцидента
type Incident struct {
	ID          string           `json:"id" db:"id"`
	CheckID     string           `json:"check_id" db:"check_id"`
	Title       string           `json:"title" db:"title"`
	Description string           `json:"description" db:"description"`
	Status      IncidentStatus   `json:"status" db:"status"`
	Severity    IncidentSeverity `json:"severity" db:"severity"`
	StartedAt   time.Time        `json:"started_at" db:"started_at"`
	ResolvedAt  *time.Time       `json:"resolved_at" db:"resolved_at"`
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at" db:"updated_at"`
}

// IncidentEvent представляет событие в жизненном цикле инцидента
type IncidentEvent struct {
	ID          string                 `json:"id"`
	IncidentID  string                 `json:"incident_id"`
	EventType   string                 `json:"event_type"`
	OldStatus   IncidentStatus         `json:"old_status"`
	NewStatus   IncidentStatus         `json:"new_status"`
	OldSeverity IncidentSeverity       `json:"old_severity"`
	NewSeverity IncidentSeverity       `json:"new_severity"`
	Message     string                 `json:"message"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
}

// NewIncident создает новый инцидент
func NewIncident(checkID string, severity IncidentSeverity, title, description string) *Incident {
	now := time.Now()

	return &Incident{
		CheckID:     checkID,
		Title:       title,
		Description: description,
		Status:      IncidentStatusOpen,
		Severity:    severity,
		StartedAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// IsOpen проверяет, является ли инцидент открытым
func (i *Incident) IsOpen() bool {
	return i.Status == IncidentStatusOpen
}

// IsAcknowledged проверяет, является ли инцидент подтвержденным
func (i *Incident) IsAcknowledged() bool {
	return i.Status == IncidentStatusAcknowledged
}

// IsResolved проверяет, является ли инцидент разрешенным
func (i *Incident) IsResolved() bool {
	return i.Status == IncidentStatusResolved
}

// Acknowledge подтверждает инцидент
func (i *Incident) Acknowledge() {
	if i.Status == IncidentStatusOpen {
		i.Status = IncidentStatusAcknowledged
		i.UpdatedAt = time.Now()
	}
}

// Resolve разрешает инцидент
func (i *Incident) Resolve() {
	if i.Status != IncidentStatusResolved {
		i.Status = IncidentStatusResolved
		now := time.Now()
		i.ResolvedAt = &now
		i.UpdatedAt = now
	}
}

// Reopen повторно открывает инцидент
func (i *Incident) Reopen() {
	if i.Status == IncidentStatusResolved {
		i.Status = IncidentStatusOpen
		i.UpdatedAt = time.Now()
	}
}

// UpdateSeverity обновляет уровень серьезности инцидента
func (i *Incident) UpdateSeverity(severity IncidentSeverity) {
	if i.Severity != severity {
		i.Severity = severity
		i.UpdatedAt = time.Now()
	}
}

// GetDuration возвращает продолжительность инцидента
func (i *Incident) GetDuration() time.Duration {
	if i.ResolvedAt != nil {
		return i.ResolvedAt.Sub(i.StartedAt)
	}
	return time.Since(i.StartedAt)
}

// IsActive проверяет, активен ли инцидент (не разрешен)
func (i *Incident) IsActive() bool {
	return i.Status != IncidentStatusResolved
}

// IsValidSeverity проверяет валидность уровня серьезности
func IsValidSeverity(severity IncidentSeverity) bool {
	switch severity {
	case IncidentSeverityLow, IncidentSeverityMedium, IncidentSeverityHigh, IncidentSeverityCritical:
		return true
	default:
		return false
	}
}

// IsValidStatus проверяет валидность статуса
func IsValidStatus(status IncidentStatus) bool {
	switch status {
	case IncidentStatusOpen, IncidentStatusAcknowledged, IncidentStatusResolved:
		return true
	default:
		return false
	}
}

// IncidentFilter представляет фильтры для поиска инцидентов
type IncidentFilter struct {
	TenantID *string           `json:"tenant_id,omitempty"`
	CheckID  *string           `json:"check_id,omitempty"`
	Status   *IncidentStatus   `json:"status,omitempty"`
	Severity *IncidentSeverity `json:"severity,omitempty"`
	From     *time.Time        `json:"from,omitempty"`
	To       *time.Time        `json:"to,omitempty"`
	Limit    int               `json:"limit,omitempty"`
	Offset   int               `json:"offset,omitempty"`
}

// IncidentStats представляет статистику инцидентов
type IncidentStats struct {
	Total      int                      `json:"total"`
	ByStatus   map[IncidentStatus]int   `json:"by_status"`
	BySeverity map[IncidentSeverity]int `json:"by_severity"`
	Last24h    int                      `json:"last_24h"`
	Last7d     int                      `json:"last_7d"`
	Last30d    int                      `json:"last_30d"`
}
