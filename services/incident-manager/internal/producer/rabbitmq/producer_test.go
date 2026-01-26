package rabbitmq

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/incident-manager/internal/domain"
)

// MockLogger для тестов
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	args := m.Called(fields)
	return args.Get(0).(logger.Logger)
}

func (m *MockLogger) Sync() error {
	args := m.Called()
	return args.Error(0)
}

func TestIncidentProducerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *IncidentProducerConfig
		wantErr bool
	}{
		{
			name: "Valid config",
			config: &IncidentProducerConfig{
				URL:        "amqp://localhost:5672",
				Exchange:   "test.exchange",
				Multiplier: 2.0,
			},
			wantErr: false,
		},
		{
			name: "Empty URL",
			config: &IncidentProducerConfig{
				URL:      "",
				Exchange: "test.exchange",
			},
			wantErr: true,
		},
		{
			name: "Empty exchange",
			config: &IncidentProducerConfig{
				URL:      "amqp://localhost:5672",
				Exchange: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultIncidentProducerConfig(t *testing.T) {
	config := DefaultIncidentProducerConfig()
	
	assert.Equal(t, "amqp://guest:guest@localhost:5672/", config.URL)
	assert.Equal(t, "incident.events", config.Exchange)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.InitialDelay)
	assert.Equal(t, 5*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
	assert.Equal(t, 10, config.PrefetchCount)
	assert.Equal(t, 0, config.PrefetchSize)
	assert.Equal(t, false, config.Global)
}

func TestIncidentEvent_Serialization(t *testing.T) {
	incident := &domain.Incident{
		ID:          "test-incident-id",
		CheckID:     "test-check-id",
		TenantID:    "test-tenant-id",
		Status:      domain.IncidentStatusOpen,
		Severity:    domain.IncidentSeverityError,
		Count:       5,
		ErrorMessage: "Test error message",
		ErrorHash:   "test-hash",
		FirstSeen:   time.Now().Add(-1 * time.Hour),
		LastSeen:    time.Now(),
		Metadata: map[string]interface{}{
			"test-key": "test-value",
		},
	}

	result := &CheckResult{
		CheckID:      "test-check-id",
		TenantID:     "test-tenant-id",
		IsSuccess:    false,
		ErrorMessage: "Test error message",
		Duration:     5 * time.Second,
		Timestamp:    time.Now(),
		Metadata:     map[string]interface{}{},
	}

	event := &IncidentEvent{
		EventType:    "incident.opened",
		Timestamp:   time.Now(),
		Service:     "incident-manager",
		IncidentID:  incident.ID,
		CheckID:     result.CheckID,
		TenantID:    result.TenantID,
		Status:      incident.Status,
		Severity:    incident.Severity,
		Count:       incident.Count,
		Duration:    result.Duration.Milliseconds(),
		ErrorMessage: result.ErrorMessage,
		ErrorHash:   incident.ErrorHash,
		FirstSeen:   incident.FirstSeen,
		LastSeen:    incident.LastSeen,
		Metadata:    incident.Metadata,
	}

	// Проверяем что все поля заполнены правильно
	assert.Equal(t, "incident.opened", event.EventType)
	assert.Equal(t, "incident-manager", event.Service)
	assert.Equal(t, incident.ID, event.IncidentID)
	assert.Equal(t, result.CheckID, event.CheckID)
	assert.Equal(t, result.TenantID, event.TenantID)
	assert.Equal(t, incident.Status, event.Status)
	assert.Equal(t, incident.Severity, event.Severity)
	assert.Equal(t, incident.Count, event.Count)
	assert.Equal(t, result.Duration.Milliseconds(), event.Duration)
	assert.Equal(t, result.ErrorMessage, event.ErrorMessage)
	assert.Equal(t, incident.ErrorHash, event.ErrorHash)
	assert.Equal(t, incident.FirstSeen, event.FirstSeen)
	assert.Equal(t, incident.LastSeen, event.LastSeen)
	assert.Equal(t, incident.Metadata, event.Metadata)
}

func TestCalculateDuration(t *testing.T) {
	tests := []struct {
		name     string
		result   *CheckResult
		expected int64
	}{
		{
			name: "Normal duration",
			result: &CheckResult{
				Duration: 5 * time.Second,
			},
			expected: 5000,
		},
		{
			name: "Zero duration",
			result: &CheckResult{
				Duration: 0,
			},
			expected: 0,
		},
		{
			name: "Nil result",
			result: nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := calculateDuration(tt.result)
			assert.Equal(t, tt.expected, duration)
		})
	}
}
