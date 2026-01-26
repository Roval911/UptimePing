package domain

import (
	"time"
)

// Task представляет задачу на выполнение проверки
type Task struct {
	ID            string                 `json:"id"`
	CheckID       string                 `json:"check_id"`
	Target        string                 `json:"target"`
	Type          string                 `json:"type"`
	Config        map[string]interface{} `json:"config"`
	ExecutionID   string                 `json:"execution_id"`
	ScheduledTime time.Time              `json:"scheduled_time"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// TaskType представляет тип задачи
type TaskType string

const (
	TaskTypeHTTP TaskType = "http"
	TaskTypeTCP  TaskType = "tcp"
	TaskTypeICMP TaskType = "icmp"
)

// TaskStatus представляет статус задачи
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// CheckResult представляет результат выполнения проверки
type CheckResult struct {
	ID           string            `json:"id"`
	CheckID      string            `json:"check_id"`
	ExecutionID  string            `json:"execution_id"`
	Success      bool              `json:"success"`
	DurationMs   int64             `json:"duration_ms"`
	StatusCode   int               `json:"status_code,omitempty"`
	Error        string            `json:"error,omitempty"`
	ResponseBody string            `json:"response_body,omitempty"`
	CheckedAt    time.Time         `json:"checked_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// CheckStatus представляет статус проверки
type CheckStatus string

const (
	CheckStatusUp   CheckStatus = "up"
	CheckStatusDown CheckStatus = "down"
	CheckStatusUnknown CheckStatus = "unknown"
)

// HTTPConfig представляет конфигурацию HTTP проверки
type HTTPConfig struct {
	Method            string            `json:"method"`
	URL               string            `json:"url"`
	Headers           map[string]string `json:"headers,omitempty"`
	Body              string            `json:"body,omitempty"`
	ExpectedStatus    int               `json:"expected_status"`
	Timeout           time.Duration     `json:"timeout"`
	FollowRedirects   bool              `json:"follow_redirects"`
	ValidateSSL       bool              `json:"validate_ssl"`
}

// TCPConfig представляет конфигурацию TCP проверки
type TCPConfig struct {
	Host     string        `json:"host"`
	Port     int           `json:"port"`
	Timeout  time.Duration `json:"timeout"`
}

// ICMPConfig представляет конфигурацию ICMP проверки
type ICMPConfig struct {
	Target   string        `json:"target"`
	Count    int           `json:"count"`
	Timeout  time.Duration `json:"timeout"`
	Interval time.Duration `json:"interval"`
}

// Validate валидирует задачу
func (t *Task) Validate() error {
	if t.CheckID == "" {
		return ErrInvalidTaskID
	}
	if t.Target == "" {
		return ErrInvalidTarget
	}
	if t.Type == "" {
		return ErrInvalidTaskType
	}
	if t.ExecutionID == "" {
		return ErrInvalidExecutionID
	}
	if t.ScheduledTime.IsZero() {
		return ErrInvalidScheduledTime
	}
	return nil
}

// GetType возвращает тип задачи
func (t *Task) GetType() TaskType {
	return TaskType(t.Type)
}

// GetHTTPConfig извлекает HTTP конфигурацию
func (t *Task) GetHTTPConfig() (*HTTPConfig, error) {
	if t.Type != string(TaskTypeHTTP) {
		return nil, ErrInvalidTaskType
	}
	
	config := &HTTPConfig{}
	if method, ok := t.Config["method"].(string); ok {
		config.Method = method
	}
	if url, ok := t.Config["url"].(string); ok {
		config.URL = url
	}
	if expectedStatus, ok := t.Config["expected_status"].(float64); ok {
		config.ExpectedStatus = int(expectedStatus)
	}
	if timeout, ok := t.Config["timeout"].(string); ok {
		if duration, err := time.ParseDuration(timeout); err == nil {
			config.Timeout = duration
		}
	}
	if followRedirects, ok := t.Config["follow_redirects"].(bool); ok {
		config.FollowRedirects = followRedirects
	}
	if validateSSL, ok := t.Config["validate_ssl"].(bool); ok {
		config.ValidateSSL = validateSSL
	}
	
	return config, nil
}

// GetTCPConfig извлекает TCP конфигурацию
func (t *Task) GetTCPConfig() (*TCPConfig, error) {
	if t.Type != string(TaskTypeTCP) {
		return nil, ErrInvalidTaskType
	}
	
	config := &TCPConfig{}
	if host, ok := t.Config["host"].(string); ok {
		config.Host = host
	}
	if port, ok := t.Config["port"].(float64); ok {
		config.Port = int(port)
	}
	if timeout, ok := t.Config["timeout"].(string); ok {
		if duration, err := time.ParseDuration(timeout); err == nil {
			config.Timeout = duration
		}
	}
	
	return config, nil
}

// GetICMPConfig извлекает ICMP конфигурацию
func (t *Task) GetICMPConfig() (*ICMPConfig, error) {
	if t.Type != string(TaskTypeICMP) {
		return nil, ErrInvalidTaskType
	}
	
	config := &ICMPConfig{}
	if target, ok := t.Config["target"].(string); ok {
		config.Target = target
	}
	if count, ok := t.Config["count"].(float64); ok {
		config.Count = int(count)
	}
	if timeout, ok := t.Config["timeout"].(string); ok {
		if duration, err := time.ParseDuration(timeout); err == nil {
			config.Timeout = duration
		}
	}
	if interval, ok := t.Config["interval"].(string); ok {
		if duration, err := time.ParseDuration(interval); err == nil {
			config.Interval = duration
		}
	}
	
	return config, nil
}

// IsSuccess возвращает true если проверка успешна
func (r *CheckResult) IsSuccess() bool {
	return r.Success
}

// GetStatus возвращает статус проверки на основе результата
func (r *CheckResult) GetStatus() CheckStatus {
	if r.Success {
		return CheckStatusUp
	}
	return CheckStatusDown
}

// Validate валидирует результат проверки
func (r *CheckResult) Validate() error {
	if r.CheckID == "" {
		return ErrInvalidCheckID
	}
	if r.ExecutionID == "" {
		return ErrInvalidExecutionID
	}
	if r.CheckedAt.IsZero() {
		return ErrInvalidCheckedTime
	}
	return nil
}

// NewTask создает новую задачу
func NewTask(checkID, target, taskType, executionID string, scheduledTime time.Time, config map[string]interface{}) *Task {
	now := time.Now().UTC()
	return &Task{
		CheckID:       checkID,
		Target:        target,
		Type:          taskType,
		Config:        config,
		ExecutionID:   executionID,
		ScheduledTime: scheduledTime,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// NewCheckResult создает новый результат проверки
func NewCheckResult(checkID, executionID string, success bool, durationMs int64, statusCode int, error, responseBody string) *CheckResult {
	return &CheckResult{
		CheckID:      checkID,
		ExecutionID:  executionID,
		Success:      success,
		DurationMs:   durationMs,
		StatusCode:   statusCode,
		Error:        error,
		ResponseBody: responseBody,
		CheckedAt:    time.Now().UTC(),
		Metadata:     make(map[string]string),
	}
}

// Ошибки валидации
var (
	ErrInvalidTaskID         = NewValidationError("task_id", "required")
	ErrInvalidTarget         = NewValidationError("target", "required")
	ErrInvalidTaskType       = NewValidationError("task_type", "invalid")
	ErrInvalidExecutionID    = NewValidationError("execution_id", "required")
	ErrInvalidScheduledTime  = NewValidationError("scheduled_time", "required")
	ErrInvalidCheckID        = NewValidationError("check_id", "required")
	ErrInvalidCheckedTime    = NewValidationError("checked_at", "required")
)

// ValidationError представляет ошибку валидации
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError создает новую ошибку валидации
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
