package domain

import "time"

// Service представляет gRPC сервис
type Service struct {
	Name    string    `json:"name"`
	Package string    `json:"package"`
	Host    string    `json:"host"`
	Port    int       `json:"port"`
	Methods []Method  `json:"methods"`
}

// Method представляет метод gRPC сервиса
type Method struct {
	Name    string `json:"name"`
	Timeout string `json:"timeout"`
	Enabled bool   `json:"enabled"`
}

// Message представляет сообщение protobuf
type Message struct {
	Name    string                 `json:"name"`
	Package string                 `json:"package"`
	Fields  []Field                `json:"fields"`
}

// Field представляет поле сообщения
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// Enum представляет enum protobuf
type Enum struct {
	Name    string     `json:"name"`
	Package string     `json:"package"`
	Values  []EnumValue `json:"values"`
}

// EnumValue представляет значение enum
type EnumValue struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// TaskType представляет тип задачи
type TaskType string

const (
	TaskTypeHTTP    TaskType = "http"
	TaskTypeHTTPS   TaskType = "https"
	TaskTypeTCP    TaskType = "tcp"
	TaskTypeICMP   TaskType = "icmp"
	TaskTypeGRPC   TaskType = "grpc"
	TaskTypeGraphQL TaskType = "graphql"
)

// Task представляет задачу проверки
type Task struct {
	CheckID    string                 `json:"check_id"`
	Target     string                 `json:"target"`
	Type       TaskType               `json:"type"`
	ExecutionID string                 `json:"execution_id"`
	CreatedAt  time.Time              `json:"created_at"`
	Config     map[string]interface{} `json:"config"`
}

// CheckResult представляет результат проверки
type CheckResult struct {
	CheckID      string            `json:"check_id"`
	ExecutionID   string            `json:"execution_id"`
	Type         TaskType          `json:"type"`
	Target       string            `json:"target"`
	Success      bool              `json:"success"`
	StatusCode   int               `json:"status_code"`
	ResponseTime  int64             `json:"response_time"`
	Error        string            `json:"error,omitempty"`
	CheckedAt    time.Time         `json:"checked_at"`
	Metadata     map[string]string `json:"metadata"`
}

// NewTask создает новую задачу
func NewTask(checkID, target string, taskType TaskType, executionID string, createdAt time.Time, config map[string]interface{}) *Task {
	return &Task{
		CheckID:     checkID,
		Target:      target,
		Type:        taskType,
		ExecutionID: executionID,
		CreatedAt:   createdAt,
		Config:      config,
	}
}

// NewCheckResult создает новый результат проверки
func NewCheckResult(checkID, executionID string, taskType TaskType, target string, success bool, statusCode int, responseTime int64) *CheckResult {
	return &CheckResult{
		CheckID:      checkID,
		ExecutionID:   executionID,
		Type:         taskType,
		Target:       target,
		Success:      success,
		StatusCode:   statusCode,
		ResponseTime:  responseTime,
		CheckedAt:    time.Now().UTC(),
		Metadata:     make(map[string]string),
	}
}
