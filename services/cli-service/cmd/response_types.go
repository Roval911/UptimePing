package cmd

import (
	"time"
)

// Response types for mock clients
type LoginResponse struct {
	Token string `json:"token"`
}

type RegisterResponse struct {
	UserId string `json:"user_id"`
}

type ValidateTokenResponse struct {
	UserId   string    `json:"user_id"`
	Email    string    `json:"email"`
	TenantId string    `json:"tenant_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type CreateAPIKeyResponse struct {
	KeyId     string    `json:"key_id"`
	ApiKey    string    `json:"api_key"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ListAPIKeysResponse struct {
	Keys []interface{} `json:"keys"`
}

type ExecuteCheckResponse struct {
	CheckId      string `json:"check_id"`
	Status       string `json:"status"`
	ResponseTime int32  `json:"response_time"`
	Message      string `json:"message"`
}

type GetCheckStatusResponse struct {
	CheckId      string    `json:"check_id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	LastCheck    time.Time `json:"last_check"`
	NextCheck    time.Time `json:"next_check"`
	SuccessRate  float64   `json:"success_rate"`
	TotalChecks  int32     `json:"total_checks"`
	FailedChecks int32     `json:"failed_checks"`
	Url          string    `json:"url"`
	Interval     string    `json:"interval"`
	Timeout      int32     `json:"timeout"`
	TenantId     string    `json:"tenant_id"`
}

type GetCheckHistoryResponse struct {
	Results []CheckResult `json:"results"`
}

type ListChecksResponse struct {
	Checks []CheckInfo `json:"checks"`
}

type CheckResult struct {
	Timestamp    time.Time `json:"timestamp"`
	Status       string    `json:"status"`
	ResponseTime int32     `json:"response_time"`
	Message      string    `json:"message"`
}

type CheckInfo struct {
	CheckId string `json:"check_id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Url     string `json:"url"`
}

type ListIncidentsResponse struct {
	Incidents []IncidentInfo `json:"incidents"`
}

type IncidentInfo struct {
	IncidentId string    `json:"incident_id"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Severity   string    `json:"severity"`
	CreatedAt  time.Time `json:"created_at"`
}

type GetIncidentResponse struct {
	IncidentId     string    `json:"incident_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	Severity      string    `json:"severity"`
	TenantId      string    `json:"tenant_id"`
	CheckId       string    `json:"check_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string    `json:"acknowledged_by,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy     string    `json:"resolved_by,omitempty"`
	Events        []IncidentEvent `json:"events"`
}

type IncidentEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
}

type AcknowledgeIncidentResponse struct {
	AcknowledgedAt time.Time `json:"acknowledged_at"`
	AcknowledgedBy string    `json:"acknowledged_by"`
}

type ResolveIncidentResponse struct {
	ResolvedAt time.Time `json:"resolved_at"`
	ResolvedBy string    `json:"resolved_by"`
}

type CreateChannelResponse struct {
	ChannelId string `json:"channel_id"`
}

type ListChannelsResponse struct {
	Channels []ChannelInfo `json:"channels"`
}

type ChannelInfo struct {
	ChannelId string `json:"channel_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Address   string `json:"address"`
	Enabled   bool   `json:"enabled"`
}

type SendNotificationResponse struct {
	NotificationId string    `json:"notification_id"`
	Status         string    `json:"status"`
	SentAt         time.Time `json:"sent_at"`
}

type GenerateResponse struct {
	GeneratedFiles int32    `json:"generated_files"`
	OutputPath     string   `json:"output_path"`
	GenerationTime time.Time `json:"generation_time"`
	Files          []string `json:"files"`
}

type ValidateResponse struct {
	Valid          bool        `json:"valid"`
	Status         string      `json:"status"`
	FilesChecked  int32       `json:"files_checked"`
	Errors         []ValidationError `json:"errors"`
	Warnings       []ValidationWarning `json:"warnings"`
	ValidationTime time.Time   `json:"validation_time"`
}

type ValidationError struct {
	File    string `json:"file"`
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

type ValidationWarning struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

type CreateConfigResponse struct {
	ConfigId  string    `json:"config_id"`
	CreatedAt time.Time `json:"created_at"`
}

type ListConfigsResponse struct {
	Configs []ConfigInfo `json:"configs"`
}

type ConfigInfo struct {
	ConfigId    string `json:"config_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Format      string `json:"format"`
}
