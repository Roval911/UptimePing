package usecase

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateCronExpression(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid simple cron",
			expression:  "0 12 * * *",
			expectError: false,
		},
		{
			name:        "Valid cron with range",
			expression:  "0 9-17 * * 1-5",
			expectError: false,
		},
		{
			name:        "Valid cron with step",
			expression:  "*/15 * * * *",
			expectError: false,
		},
		{
			name:        "Valid cron with complex expression",
			expression:  "0,30 9-17 * * 1-5",
			expectError: false,
		},
		{
			name:        "Empty expression",
			expression:  "",
			expectError: true,
			errorMsg:    "cron expression cannot be empty",
		},
		{
			name:        "Invalid field count",
			expression:  "0 12 * *",
			expectError: true,
			errorMsg:    "expected exactly 5 fields, found 4",
		},
		{
			name:        "Invalid minute value",
			expression:  "60 12 * * *",
			expectError: true,
			errorMsg:    "end of range (60) above maximum (59)",
		},
		{
			name:        "Invalid hour value",
			expression:  "0 25 * * *",
			expectError: true,
			errorMsg:    "end of range (25) above maximum (23)",
		},
		{
			name:        "Invalid day value",
			expression:  "0 12 32 * *",
			expectError: true,
			errorMsg:    "end of range (32) above maximum (31)",
		},
		{
			name:        "Invalid month value",
			expression:  "0 12 * 13 *",
			expectError: true,
			errorMsg:    "end of range (13) above maximum (12)",
		},
		{
			name:        "Invalid weekday value",
			expression:  "0 12 * * 8",
			expectError: true,
			errorMsg:    "end of range (8) above maximum (6)",
		},
		{
			name:        "Invalid range",
			expression:  "0 12 10-5 * *",
			expectError: true,
			errorMsg:    "beginning of range (10) beyond end of range (5)",
		},
		{
			name:        "Invalid step",
			expression:  "0 */0 * * *",
			expectError: true,
			errorMsg:    "step of range should be a positive number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCronExpression(tt.expression)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCalculateNextRun(t *testing.T) {
	tests := []struct {
		name          string
		expression    string
		shouldNotFail bool
	}{
		{
			name:          "Every minute",
			expression:    "* * * * *",
			shouldNotFail: true,
		},
		{
			name:          "Every hour at minute 0",
			expression:    "0 * * * *",
			shouldNotFail: true,
		},
		{
			name:          "Daily at noon",
			expression:    "0 12 * * *",
			shouldNotFail: true,
		},
		{
			name:          "Every 15 minutes",
			expression:    "*/15 * * * *",
			shouldNotFail: true,
		},
		{
			name:          "Weekdays at 9 AM",
			expression:    "0 9 * * 1-5",
			shouldNotFail: true,
		},
		{
			name:          "Invalid cron expression",
			expression:    "invalid",
			shouldNotFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextRun, err := calculateNextRun(tt.expression)

			if tt.shouldNotFail {
				assert.NoError(t, err, "Should not fail for valid cron expression")
				assert.True(t, nextRun.After(time.Now()), "Next run should be in the future")
			} else {
				assert.Error(t, err, "Should fail for invalid cron expression")
			}
		})
	}
}

// Вспомогательная функция для расчета дельты до следующего буднего дня
func calculateWeekdayDelta(weekday time.Weekday, hour int) time.Duration {
	now := time.Now()
	daysUntilWeekday := int(weekday) - int(now.Weekday())
	if daysUntilWeekday <= 0 {
		daysUntilWeekday += 7
	}

	nextWeekday := time.Date(now.Year(), now.Month(), now.Day()+daysUntilWeekday, hour, 0, 0, 0, now.Location())
	return nextWeekday.Sub(now)
}

func TestValidateCronEdgeCases(t *testing.T) {
	t.Run("Cron with seconds should fail", func(t *testing.T) {
		err := validateCronExpression("0 0 12 * * *")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected exactly 5 fields, found 6")
	})

	t.Run("Valid edge of ranges", func(t *testing.T) {
		// Граничные значения должны проходить валидацию
		// Note: weekday 7 = Sunday (0 or 7 both valid for Sunday)
		assert.NoError(t, validateCronExpression("59 23 31 12 0"))
	})

	t.Run("Complex valid expression", func(t *testing.T) {
		// Сложное но валидное выражение
		expr := "0,15,30,45 9,12,15,18 1,15,31 1,6,12 1-5"
		assert.NoError(t, validateCronExpression(expr))
	})
}
