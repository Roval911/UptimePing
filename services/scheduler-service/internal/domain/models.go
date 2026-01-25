package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// CheckType представляет тип проверки
type CheckType string

const (
	CheckTypeHTTP    CheckType = "http"
	CheckTypeHTTPS   CheckType = "https"
	CheckTypeGRPC    CheckType = "grpc"
	CheckTypeGraphQL CheckType = "graphql"
	CheckTypeTCP     CheckType = "tcp"
)

// CheckStatus представляет статус проверки
type CheckStatus string

const (
	CheckStatusActive   CheckStatus = "active"
	CheckStatusPaused   CheckStatus = "paused"
	CheckStatusDisabled CheckStatus = "disabled"
)

// Priority представляет приоритет проверки
type Priority int

const (
	PriorityLow      Priority = 1
	PriorityNormal   Priority = 2
	PriorityHigh     Priority = 3
	PriorityCritical Priority = 4
)

// CheckConfig представляет конфигурацию специфичную для типа проверки
type CheckConfig map[string]interface{}

// Value реализует driver.Valuer для CheckConfig
func (c CheckConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan реализует sql.Scanner для CheckConfig
func (c *CheckConfig) Scan(value interface{}) error {
	if value == nil {
		*c = make(CheckConfig)
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, c)
	case string:
		return json.Unmarshal([]byte(v), c)
	default:
		return fmt.Errorf("cannot scan %T into CheckConfig", value)
	}
}

// Check представляет сущность проверки
type Check struct {
	ID        string      `json:"id" db:"id"`
	TenantID  string      `json:"tenant_id" db:"tenant_id"`
	Name      string      `json:"name" db:"name"`
	Type      CheckType   `json:"type" db:"type"`
	Target    string      `json:"target" db:"target"`
	Interval  int         `json:"interval" db:"interval"` // в секундах
	Timeout   int         `json:"timeout" db:"timeout"`   // в секундах
	Status    CheckStatus `json:"status" db:"status"`
	Config    CheckConfig `json:"config" db:"config"`
	Priority  Priority    `json:"priority" db:"priority"`
	Tags      []string    `json:"tags" db:"tags"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" db:"updated_at"`
	LastRunAt *time.Time  `json:"last_run_at" db:"last_run_at"`
	NextRunAt *time.Time  `json:"next_run_at" db:"next_run_at"`
}

// IsActive проверяет, активна ли проверка
func (c *Check) IsActive() bool {
	return c.Status == CheckStatusActive
}

// IsPaused проверяет, поставлена ли проверка на паузу
func (c *Check) IsPaused() bool {
	return c.Status == CheckStatusPaused
}

// IsDisabled проверяет, отключена ли проверка
func (c *Check) IsDisabled() bool {
	return c.Status == CheckStatusDisabled
}

// GetIntervalDuration возвращает интервал как time.Duration
func (c *Check) GetIntervalDuration() time.Duration {
	return time.Duration(c.Interval) * time.Second
}

// GetTimeoutDuration возвращает таймаут как time.Duration
func (c *Check) GetTimeoutDuration() time.Duration {
	return time.Duration(c.Timeout) * time.Second
}

// ShouldRun проверяет, пора ли выполнять проверку
func (c *Check) ShouldRun() bool {
	if !c.IsActive() {
		return false
	}

	if c.NextRunAt == nil {
		return true
	}

	return time.Now().After(*c.NextRunAt)
}

// UpdateNextRun обновляет время следующего запуска
func (c *Check) UpdateNextRun() {
	now := time.Now()
	c.LastRunAt = &now
	nextRun := now.Add(c.GetIntervalDuration())
	c.NextRunAt = &nextRun
}

// Validate валидирует данные проверки
func (c *Check) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("check id is required")
	}
	if c.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	if c.Name == "" {
		return fmt.Errorf("check name is required")
	}
	if c.Target == "" {
		return fmt.Errorf("check target is required")
	}

	// Валидация типа проверки
	switch c.Type {
	case CheckTypeHTTP, CheckTypeHTTPS, CheckTypeGRPC, CheckTypeGraphQL, CheckTypeTCP:
		// Valid types
	default:
		return fmt.Errorf("invalid check type: %s", c.Type)
	}

	// Валидация интервала (от 5 секунд до 24 часов)
	if c.Interval < 5 || c.Interval > 86400 {
		return fmt.Errorf("interval must be between 5 seconds and 24 hours")
	}

	// Валидация таймаута (от 1 секунды до 5 минут)
	if c.Timeout < 1 || c.Timeout > 300 {
		return fmt.Errorf("timeout must be between 1 second and 5 minutes")
	}

	// Валидация статуса
	switch c.Status {
	case CheckStatusActive, CheckStatusPaused, CheckStatusDisabled:
		// Valid statuses
	default:
		return fmt.Errorf("invalid check status: %s", c.Status)
	}

	// Валидация приоритета
	if c.Priority < PriorityLow || c.Priority > PriorityCritical {
		return fmt.Errorf("priority must be between %d and %d", PriorityLow, PriorityCritical)
	}

	return nil
}

// Schedule представляет расписание выполнения проверки
type Schedule struct {
	ID             string     `json:"id" db:"id"`
	CheckID        string     `json:"check_id" db:"check_id"`
	CronExpression string     `json:"cron_expression" db:"cron_expression"`
	NextRun        *time.Time `json:"next_run" db:"next_run"`
	LastRun        *time.Time `json:"last_run" db:"last_run"`
	IsActive       bool       `json:"is_active" db:"is_active"`
	Priority       Priority   `json:"priority" db:"priority"`
	Timezone       string     `json:"timezone" db:"timezone"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// IsActive проверяет, активно ли расписание
func (s *Schedule) IsScheduleActive() bool {
	return s.IsActive
}

// ShouldRun проверяет, пора ли выполнять проверку по расписанию
func (s *Schedule) ShouldRun() bool {
	if !s.IsActive {
		return false
	}

	if s.NextRun == nil {
		return false
	}

	return time.Now().After(*s.NextRun)
}

// UpdateNextRun обновляет время следующего запуска на основе cron выражения
// В реальной реализации здесь будет использоваться библиотека для работы с cron
func (s *Schedule) UpdateNextRun() error {
	// TODO: Реализовать парсинг cron выражения и вычисление следующего времени
	// Например, с использованием github.com/robfig/cron/v3

	now := time.Now()
	s.LastRun = &now

	// Временная реализация - добавляем час
	nextRun := now.Add(time.Hour)
	s.NextRun = &nextRun

	return nil
}

// Validate валидирует данные расписания
func (s *Schedule) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("schedule id is required")
	}
	if s.CheckID == "" {
		return fmt.Errorf("check id is required")
	}
	if s.CronExpression == "" {
		return fmt.Errorf("cron expression is required")
	}

	// Валидация cron выражения
	// TODO: Добавить валидацию cron выражения
	// if err := validateCronExpression(s.CronExpression); err != nil {
	//     return fmt.Errorf("invalid cron expression: %w", err)
	// }

	// Валидация приоритета
	if s.Priority < PriorityLow || s.Priority > PriorityCritical {
		return fmt.Errorf("priority must be between %d and %d", PriorityLow, PriorityCritical)
	}

	return nil
}

// CheckWithSchedule объединяет проверку и ее расписание
type CheckWithSchedule struct {
	Check    Check     `json:"check"`
	Schedule *Schedule `json:"schedule,omitempty"`
}

// GetEffectivePriority возвращает эффективный приоритет (из расписания или из проверки)
func (cws *CheckWithSchedule) GetEffectivePriority() Priority {
	if cws.Schedule != nil {
		return cws.Schedule.Priority
	}
	return cws.Check.Priority
}

// ShouldRun проверяет, пора ли выполнять проверку
func (cws *CheckWithSchedule) ShouldRun() bool {
	// Если есть расписание и оно активно, используем его
	if cws.Schedule != nil && cws.Schedule.IsScheduleActive() {
		return cws.Schedule.ShouldRun()
	}

	// Иначе используем интервал проверки
	return cws.Check.ShouldRun()
}

// UpdateNextRun обновляет время следующего запуска
func (cws *CheckWithSchedule) UpdateNextRun() error {
	if cws.Schedule != nil && cws.Schedule.IsScheduleActive() {
		return cws.Schedule.UpdateNextRun()
	}

	cws.Check.UpdateNextRun()
	return nil
}

// Task представляет задачу для выполнения
type Task struct {
	ID          string    `json:"id"`
	CheckID     string    `json:"check_id"`
	TenantID    string    `json:"tenant_id"`
	Priority    Priority  `json:"priority"`
	ScheduledAt time.Time `json:"scheduled_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewTask создает новую задачу
func NewTask(checkID, tenantID string, priority Priority) *Task {
	return &Task{
		ID:          generateID(),
		CheckID:     checkID,
		TenantID:    tenantID,
		Priority:    priority,
		ScheduledAt: time.Now(),
		CreatedAt:   time.Now(),
	}
}

// generateID генерирует уникальный ID
// В реальной реализации здесь будет использоваться UUID генератор
func generateID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}
