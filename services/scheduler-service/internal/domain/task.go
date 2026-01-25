package domain

import (
	"time"
)

// TaskStatus представляет статус задачи
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// TaskResult представляет результат выполнения задачи
type TaskResult struct {
	TaskID       string     `json:"task_id"`
	CheckID      string     `json:"check_id"`
	Status       TaskStatus `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	Duration     int64      `json:"duration_ms"`
	CompletedAt  time.Time  `json:"completed_at"`
}

// LockInfo представляет информацию о блокировке
type LockInfo struct {
	CheckID    string    `json:"check_id"`
	WorkerID   string    `json:"worker_id"`
	LockedAt   time.Time `json:"locked_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// NewTaskForExecution создает новую задачу для выполнения
func NewTaskForExecution(checkID, tenantID string, scheduledTime time.Time, priority Priority) *Task {
	return &Task{
		ID:          generateID(),
		CheckID:     checkID,
		TenantID:    tenantID,
		Priority:    priority,
		ScheduledAt: scheduledTime,
		CreatedAt:   time.Now(),
	}
}
