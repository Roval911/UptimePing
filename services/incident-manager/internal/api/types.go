package api

import "time"

// ListIncidentsRequest представляет запрос на получение списка инцидентов
type ListIncidentsRequest struct {
	Status   string `json:"status"`   // active, resolved, acknowledged
	Severity string `json:"severity"` // critical, high, medium, low
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// Incident представляет инцидент
type Incident struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Severity    string    `json:"severity"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// ListIncidentsResponse представляет ответ со списком инцидентов
type ListIncidentsResponse struct {
	Incidents []Incident `json:"incidents"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// GetIncidentRequest представляет запрос на получение инцидента
type GetIncidentRequest struct {
	ID string `json:"id"`
}

// GetIncidentResponse представляет ответ с инцидентом
type GetIncidentResponse struct {
	Incident *Incident `json:"incident"`
}

// AcknowledgeIncidentRequest представляет запрос на подтверждение инцидента
type AcknowledgeIncidentRequest struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	Assignee    string `json:"assignee"`
}

// AcknowledgeIncidentResponse представляет ответ на подтверждение
type AcknowledgeIncidentResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Incident  *Incident `json:"incident"`
	Timestamp time.Time `json:"timestamp"`
}

// ResolveIncidentRequest представляет запрос на решение инцидента
type ResolveIncidentRequest struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	Resolution  string `json:"resolution"`
}

// ResolveIncidentResponse представляет ответ на решение
type ResolveIncidentResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Incident  *Incident `json:"incident"`
	Timestamp time.Time `json:"timestamp"`
}
