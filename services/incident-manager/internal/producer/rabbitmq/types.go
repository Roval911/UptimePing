package rabbitmq

import (
	"time"
)

// CheckResult представляет результат проверки
type CheckResult struct {
	CheckID      string                 `json:"check_id"`
	TenantID     string                 `json:"tenant_id"`
	IsSuccess    bool                   `json:"is_success"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Duration     time.Duration          `json:"duration"`
	Timestamp    time.Time              `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
