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
	IncidentSeverityWarning  IncidentSeverity = "warning"
	IncidentSeverityError    IncidentSeverity = "error"
	IncidentSeverityCritical IncidentSeverity = "critical"
)

// Incident представляет сущность инцидента
type Incident struct {
	ID          string             `json:"id" db:"id"`
	CheckID     string             `json:"check_id" db:"check_id"`
	TenantID    string             `json:"tenant_id" db:"tenant_id"`
	Status      IncidentStatus      `json:"status" db:"status"`
	Severity    IncidentSeverity    `json:"severity" db:"severity"`
	FirstSeen   time.Time          `json:"first_seen" db:"first_seen"`
	LastSeen    time.Time          `json:"last_seen" db:"last_seen"`
	Count       int                `json:"count" db:"count"`
	ErrorMessage string            `json:"error_message" db:"error_message"`
	ErrorHash   string             `json:"error_hash" db:"error_hash"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt   time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" db:"updated_at"`
}

// NewIncident создает новый инцидент
func NewIncident(checkID, tenantID string, severity IncidentSeverity, errorMessage string) *Incident {
	now := time.Now()
	errorHash := generateErrorHash(errorMessage)
	
	return &Incident{
		CheckID:      checkID,
		TenantID:     tenantID,
		Status:       IncidentStatusOpen,
		Severity:     severity,
		FirstSeen:    now,
		LastSeen:     now,
		Count:        1,
		ErrorMessage: errorMessage,
		ErrorHash:    errorHash,
		CreatedAt:    now,
		UpdatedAt:    now,
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
		i.UpdatedAt = time.Now()
	}
}

// Reopen повторно открывает инцидент
func (i *Incident) Reopen() {
	if i.Status == IncidentStatusResolved {
		i.Status = IncidentStatusOpen
		i.UpdatedAt = time.Now()
	}
}

// IncrementCount увеличивает счетчик инцидента и обновляет время последнего обнаружения
func (i *Incident) IncrementCount() {
	i.Count++
	i.LastSeen = time.Now()
	i.UpdatedAt = time.Now()
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
	return i.LastSeen.Sub(i.FirstSeen)
}

// IsActive проверяет, активен ли инцидент (не разрешен)
func (i *Incident) IsActive() bool {
	return i.Status != IncidentStatusResolved
}

// IsValidSeverity проверяет валидность уровня серьезности
func IsValidSeverity(severity IncidentSeverity) bool {
	switch severity {
	case IncidentSeverityWarning, IncidentSeverityError, IncidentSeverityCritical:
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
	TenantID   *string             `json:"tenant_id,omitempty"`
	CheckID    *string             `json:"check_id,omitempty"`
	Status     *IncidentStatus     `json:"status,omitempty"`
	Severity   *IncidentSeverity   `json:"severity,omitempty"`
	From       *time.Time          `json:"from,omitempty"`
	To         *time.Time          `json:"to,omitempty"`
	Limit      int                 `json:"limit,omitempty"`
	Offset     int                 `json:"offset,omitempty"`
}

// IncidentStats представляет статистику инцидентов
type IncidentStats struct {
	Total      int                    `json:"total"`
	ByStatus   map[IncidentStatus]int `json:"by_status"`
	BySeverity map[IncidentSeverity]int `json:"by_severity"`
	Last24h    int                    `json:"last_24h"`
	Last7d     int                    `json:"last_7d"`
	Last30d    int                    `json:"last_30d"`
}
