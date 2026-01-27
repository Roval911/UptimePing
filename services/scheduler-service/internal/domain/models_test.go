package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status CheckStatus
		want   bool
	}{
		{"active check", CheckStatusActive, true},
		{"paused check", CheckStatusPaused, false},
		{"disabled check", CheckStatusDisabled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := &Check{Status: tt.status}
			assert.Equal(t, tt.want, check.IsActive())
		})
	}
}

func TestCheck_ShouldRun(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		status  CheckStatus
		nextRun *time.Time
		want    bool
	}{
		{
			name:    "active check with next run in past",
			status:  CheckStatusActive,
			nextRun: &time.Time{}, // zero time = past
			want:    true,
		},
		{
			name:    "active check with next run in future",
			status:  CheckStatusActive,
			nextRun: func() *time.Time { t := now.Add(time.Hour); return &t }(),
			want:    false,
		},
		{
			name:    "active check with nil next run",
			status:  CheckStatusActive,
			nextRun: nil,
			want:    true,
		},
		{
			name:    "paused check",
			status:  CheckStatusPaused,
			nextRun: nil,
			want:    false,
		},
		{
			name:    "disabled check",
			status:  CheckStatusDisabled,
			nextRun: nil,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := &Check{
				Status:    tt.status,
				NextRunAt: tt.nextRun,
			}
			assert.Equal(t, tt.want, check.ShouldRun())
		})
	}
}

func TestCheck_UpdateNextRun(t *testing.T) {
	check := &Check{
		Interval: 60, // 1 minute
	}

	check.UpdateNextRun()

	require.NotNil(t, check.LastRunAt)
	require.NotNil(t, check.NextRunAt)

	// NextRunAt должен быть примерно на 1 минуту позже LastRunAt
	expectedNextRun := check.LastRunAt.Add(time.Minute)
	assert.True(t, check.NextRunAt.After(expectedNextRun.Add(-time.Second)) || check.NextRunAt.Equal(expectedNextRun))
	assert.True(t, check.NextRunAt.Before(expectedNextRun.Add(time.Second)) || check.NextRunAt.Equal(expectedNextRun))
}

func TestCheck_Validate(t *testing.T) {
	tests := []struct {
		name    string
		check   Check
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid check",
			check: Check{
				ID:       "check-1",
				TenantID: "tenant-1",
				Name:     "Test Check",
				Type:     CheckTypeHTTP,
				Target:   "https://example.com",
				Interval: 60,
				Timeout:  30,
				Status:   CheckStatusActive,
				Priority: PriorityNormal,
			},
			wantErr: false,
		},
		{
			name: "missing id",
			check: Check{
				TenantID: "tenant-1",
				Name:     "Test Check",
				Type:     CheckTypeHTTP,
				Target:   "https://example.com",
				Interval: 60,
				Timeout:  30,
				Status:   CheckStatusActive,
			},
			wantErr: true,
			errMsg:  "check id is required",
		},
		{
			name: "invalid type",
			check: Check{
				ID:       "check-1",
				TenantID: "tenant-1",
				Name:     "Test Check",
				Type:     "invalid",
				Target:   "https://example.com",
				Interval: 60,
				Timeout:  30,
				Status:   CheckStatusActive,
			},
			wantErr: true,
			errMsg:  "invalid check type",
		},
		{
			name: "interval too small",
			check: Check{
				ID:       "check-1",
				TenantID: "tenant-1",
				Name:     "Test Check",
				Type:     CheckTypeHTTP,
				Target:   "https://example.com",
				Interval: 4, // less than 5
				Timeout:  30,
				Status:   CheckStatusActive,
			},
			wantErr: true,
			errMsg:  "interval must be between 5 seconds and 24 hours",
		},
		{
			name: "interval too large",
			check: Check{
				ID:       "check-1",
				TenantID: "tenant-1",
				Name:     "Test Check",
				Type:     CheckTypeHTTP,
				Target:   "https://example.com",
				Interval: 86401, // more than 24 hours
				Timeout:  30,
				Status:   CheckStatusActive,
			},
			wantErr: true,
			errMsg:  "interval must be between 5 seconds and 24 hours",
		},
		{
			name: "timeout too small",
			check: Check{
				ID:       "check-1",
				TenantID: "tenant-1",
				Name:     "Test Check",
				Type:     CheckTypeHTTP,
				Target:   "https://example.com",
				Interval: 60,
				Timeout:  0, // less than 1
				Status:   CheckStatusActive,
			},
			wantErr: true,
			errMsg:  "timeout must be between 1 second and 5 minutes",
		},
		{
			name: "timeout too large",
			check: Check{
				ID:       "check-1",
				TenantID: "tenant-1",
				Name:     "Test Check",
				Type:     CheckTypeHTTP,
				Target:   "https://example.com",
				Interval: 60,
				Timeout:  301, // more than 5 minutes
				Status:   CheckStatusActive,
			},
			wantErr: true,
			errMsg:  "timeout must be between 1 second and 5 minutes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.check.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheck_GetIntervalDuration(t *testing.T) {
	check := &Check{Interval: 60}
	assert.Equal(t, time.Minute, check.GetIntervalDuration())
}

func TestCheck_GetTimeoutDuration(t *testing.T) {
	check := &Check{Timeout: 30}
	assert.Equal(t, 30*time.Second, check.GetTimeoutDuration())
}

func TestSchedule_ShouldRun(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		isActive bool
		nextRun  *time.Time
		want     bool
	}{
		{
			name:     "active schedule with next run in past",
			isActive: true,
			nextRun:  &time.Time{}, // zero time = past
			want:     true,
		},
		{
			name:     "active schedule with next run in future",
			isActive: true,
			nextRun:  func() *time.Time { t := now.Add(time.Hour); return &t }(),
			want:     false,
		},
		{
			name:     "inactive schedule",
			isActive: false,
			nextRun:  nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := &Schedule{
				IsActive: tt.isActive,
				NextRun:  tt.nextRun,
			}
			assert.Equal(t, tt.want, schedule.ShouldRun())
		})
	}
}

func TestSchedule_Validate(t *testing.T) {
	tests := []struct {
		name     string
		schedule Schedule
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid schedule",
			schedule: Schedule{
				ID:             "schedule-1",
				CheckID:        "check-1",
				CronExpression: "0 */5 * * *", // 5 полей
				Priority:       PriorityNormal,
			},
			wantErr: false,
		},
		{
			name: "missing id",
			schedule: Schedule{
				CheckID:        "check-1",
				CronExpression: "0 */5 * * *", // 5 полей
				Priority:       PriorityNormal,
			},
			wantErr: true,
			errMsg:  "schedule id is required",
		},
		{
			name: "missing check id",
			schedule: Schedule{
				ID:             "schedule-1",
				CronExpression: "0 */5 * * *", // 5 полей
				Priority:       PriorityNormal,
			},
			wantErr: true,
			errMsg:  "check id is required",
		},
		{
			name: "missing cron expression",
			schedule: Schedule{
				ID:       "schedule-1",
				CheckID:  "check-1",
				Priority: PriorityNormal,
			},
			wantErr: true,
			errMsg:  "cron expression is required",
		},
		{
			name: "invalid priority",
			schedule: Schedule{
				ID:             "schedule-1",
				CheckID:        "check-1",
				CronExpression: "*/5 * * * *", // 5 полей: каждые 5 минут
				Priority:       Priority(5), // invalid priority
			},
			wantErr: true,
			errMsg:  "priority must be between 1 and 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schedule.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckWithSchedule_GetEffectivePriority(t *testing.T) {
	tests := []struct {
		name     string
		check    Check
		schedule *Schedule
		want     Priority
	}{
		{
			name:     "without schedule",
			check:    Check{Priority: PriorityHigh},
			schedule: nil,
			want:     PriorityHigh,
		},
		{
			name:     "with schedule",
			check:    Check{Priority: PriorityHigh},
			schedule: &Schedule{Priority: PriorityCritical},
			want:     PriorityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cws := &CheckWithSchedule{
				Check:    tt.check,
				Schedule: tt.schedule,
			}
			assert.Equal(t, tt.want, cws.GetEffectivePriority())
		})
	}
}

func TestCheckWithSchedule_ShouldRun(t *testing.T) {
	tests := []struct {
		name     string
		check    Check
		schedule *Schedule
		want     bool
	}{
		{
			name: "check without schedule - active",
			check: Check{
				Status:    CheckStatusActive,
				NextRunAt: &time.Time{}, // past
			},
			schedule: nil,
			want:     true,
		},
		{
			name: "check without schedule - paused",
			check: Check{
				Status:    CheckStatusPaused,
				NextRunAt: &time.Time{}, // past
			},
			schedule: nil,
			want:     false,
		},
		{
			name: "check with active schedule",
			check: Check{
				Status: CheckStatusActive,
			},
			schedule: &Schedule{
				IsActive: true,
				NextRun:  &time.Time{}, // past
			},
			want: true,
		},
		{
			name: "check with inactive schedule",
			check: Check{
				Status:    CheckStatusActive,
				NextRunAt: func() *time.Time { t := time.Now().Add(time.Hour); return &t }(),
			},
			schedule: &Schedule{
				IsActive: false,
				NextRun:  func() *time.Time { t := time.Now().Add(time.Hour); return &t }(),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cws := &CheckWithSchedule{
				Check:    tt.check,
				Schedule: tt.schedule,
			}
			assert.Equal(t, tt.want, cws.ShouldRun())
		})
	}
}

func TestNewTask(t *testing.T) {
	task := NewTask("check-1", "tenant-1", PriorityHigh)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "check-1", task.CheckID)
	assert.Equal(t, "tenant-1", task.TenantID)
	assert.Equal(t, PriorityHigh, task.Priority)
	now := time.Now()
	assert.True(t, task.ScheduledAt.Before(now.Add(time.Second)) || task.ScheduledAt.Equal(now))
	assert.True(t, task.CreatedAt.Before(now.Add(time.Second)) || task.CreatedAt.Equal(now))
}
