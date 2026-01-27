package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
func (s *Schedule) UpdateNextRun() error {
	if s.CronExpression == "" {
		return fmt.Errorf("cron expression is required")
	}

	now := time.Now()
	s.LastRun = &now

	// Вычисляем следующее время запуска
	nextRun, err := calculateNextRunTime(s.CronExpression, now)
	if err != nil {
		return fmt.Errorf("failed to calculate next run time: %w", err)
	}

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
	if err := validateCronExpression(s.CronExpression); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

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

// validateCronExpression валидирует cron выражение
func validateCronExpression(cronExpr string) error {
	if cronExpr == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}

	// Разделяем выражение на 5 полей (minute hour day month weekday)
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return fmt.Errorf("cron expression must have exactly 5 fields (minute hour day month weekday), got %d", len(fields))
	}

	// Regex паттерны для каждого поля
	patterns := []string{
		`^(\*|[0-5]?\d|([0-5]?\d-[0-5]?\d)(/[0-5]?\d)?|([0-5]?\d)(,[0-5]?\d)*|\*/[0-5]?\d)$`,           // minute: 0-59
		`^(\*|[01]?\d|2[0-3]|([01]?\d-2[0-3])(/[01]?\d)?|([01]?\d|2[0-3])(,([01]?\d|2[0-3]))*|\*/[01]?\d)$`, // hour: 0-23
		`^(\*|[12]?\d|3[01]|([12]?\d-3[01])(/[12]?\d)?|([12]?\d|3[01])(,([12]?\d|3[01]))*|\*/[12]?\d)$`, // day: 1-31
		`^(\*|[1]?\d|([1]?\d-1[2])(/[1]?\d)?|([1]?\d)(,([1]?\d))*)$`,                     // month: 1-12
		`^(\*|[0-6]|([0-6]-[0-6])(/[0-6])?|[0-6](,[0-6])*)$`,                                 // weekday: 0-6 (0=Sunday)
	}

	// Проверяем каждое поле
	for i, field := range fields {
		pattern := regexp.MustCompile(patterns[i])
		if !pattern.MatchString(field) {
			fieldNames := []string{"minute", "hour", "day", "month", "weekday"}
			return fmt.Errorf("invalid %s field: %s", fieldNames[i], field)
		}
	}

	return nil
}

// parseCronField парсит одно поле cron выражения
func parseCronField(field string, min, max int) ([]int, error) {
	var values []int

	// Обработка wildcard
	if field == "*" {
		for i := min; i <= max; i++ {
			values = append(values, i)
		}
		return values, nil
	}

	// Обработка списков (например, "1,3,5")
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		for _, part := range parts {
			val, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				return nil, fmt.Errorf("invalid value in list: %s", part)
			}
			if val < min || val > max {
				return nil, fmt.Errorf("value %d out of range [%d, %d]", val, min, max)
			}
			values = append(values, val)
		}
		return values, nil
	}

	// Обработка диапазонов (например, "1-5")
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format: %s", field)
		}

		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid range start: %s", parts[0])
		}

		end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid range end: %s", parts[1])
		}

		// Обработка шага (например, "1-5/2")
		step := 1
		if strings.Contains(parts[1], "/") {
			stepParts := strings.Split(parts[1], "/")
			if len(stepParts) != 2 {
				return nil, fmt.Errorf("invalid step format: %s", parts[1])
			}
			stepVal, err := strconv.Atoi(strings.TrimSpace(stepParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid step value: %s", stepParts[1])
			}
			if stepVal <= 0 {
				return nil, fmt.Errorf("step must be positive, got %d", stepVal)
			}
			step = stepVal
			end, err = strconv.Atoi(strings.TrimSpace(stepParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid range end with step: %s", stepParts[0])
			}
		}

		if start < min || start > max || end < min || end > max {
			return nil, fmt.Errorf("range [%d, %d] out of bounds [%d, %d]", start, end, min, max)
		}

		if start > end {
			return nil, fmt.Errorf("range start %d greater than end %d", start, end)
		}

		for i := start; i <= end; i += step {
			values = append(values, i)
		}
		return values, nil
	}

	// Обработка шага с wildcard (например, "*/5")
	if strings.Contains(field, "/") {
		parts := strings.Split(field, "/")
		if len(parts) != 2 || parts[0] != "*" {
			return nil, fmt.Errorf("invalid step format: %s", field)
		}

		step, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid step value: %s", parts[1])
		}
		if step <= 0 {
			return nil, fmt.Errorf("step must be positive, got %d", step)
		}

		for i := min; i <= max; i += step {
			values = append(values, i)
		}
		return values, nil
	}

	// Обработка простого числа
	val, err := strconv.Atoi(strings.TrimSpace(field))
	if err != nil {
		return nil, fmt.Errorf("invalid numeric value: %s", field)
	}

	if val < min || val > max {
		return nil, fmt.Errorf("value %d out of range [%d, %d]", val, min, max)
	}

	values = append(values, val)
	return values, nil
}

// calculateNextRunTime вычисляет следующее время запуска на основе cron выражения
func calculateNextRunTime(cronExpr string, from time.Time) (time.Time, error) {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("invalid cron expression format")
	}

	// Парсим каждое поле
	minutes, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid minute field: %w", err)
	}

	hours, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour field: %w", err)
	}

	days, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day field: %w", err)
	}

	months, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month field: %w", err)
	}

	weekdays, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid weekday field: %w", err)
	}

	// Ищем следующее время запуска
	next := from.Add(time.Minute).Truncate(time.Minute)

	// Максимальное количество итераций для предотвращения бесконечного цикла
	maxIterations := 366 * 24 * 60 // примерно год вперед
	for i := 0; i < maxIterations; i++ {
		// Проверяем месяц
		if !contains(months, int(next.Month())) {
			next = time.Date(next.Year(), next.Month(), 1, 0, 0, 0, 0, next.Location()).AddDate(0, 1, 0)
			continue
		}

		// Проверяем день (учитываем и день месяца, и день недели)
		dayOfMonth := next.Day()
		weekday := int(next.Weekday())
		
		// В cron: 0=Sunday, 6=Saturday
		// В Go: Sunday=0, Monday=1, ..., Saturday=6
		goWeekday := weekday
		if goWeekday == 0 {
			goWeekday = 7 // Преобразуем Sunday=0 в Sunday=7 для cron
		}
		cronWeekday := goWeekday % 7

		dayMatch := contains(days, dayOfMonth)
		weekdayMatch := contains(weekdays, cronWeekday)

		// Если указаны и день месяца, и день недели, используется логика ИЛИ
		// Если указан только один из них, используется логика И
		if len(days) > 1 || days[0] != 1 || !contains(days, dayOfMonth) {
			if len(weekdays) > 1 || weekdays[0] != 0 || !contains(weekdays, cronWeekday) {
				// Оба поля указаны - используем ИЛИ
				if !dayMatch && !weekdayMatch {
					next = next.Add(time.Hour)
					continue
				}
			}
		} else {
			// Только день недели указан
			if !weekdayMatch {
				next = next.Add(time.Hour)
				continue
			}
		}

		// Проверяем час
		if !contains(hours, next.Hour()) {
			next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour()+1, 0, 0, 0, next.Location())
			continue
		}

		// Проверяем минуту
		if !contains(minutes, next.Minute()) {
			next = next.Add(time.Minute)
			continue
		}

		// Все условия выполнены
		return next, nil
	}

	return time.Time{}, fmt.Errorf("failed to calculate next run time within reasonable timeframe")
}

// contains проверяет наличие элемента в слайсе
func contains(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
