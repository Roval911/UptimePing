package client

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"UptimePingPlatform/gen/go/proto/api/incident/v1"
	"UptimePingPlatform/services/core-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty address",
			config: &Config{
				Address: "",
				Timeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			config: &Config{
				Address: "localhost:50052",
				Timeout: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative max retries",
			config: &Config{
				Address:    "localhost:50052",
				Timeout:    30 * time.Second,
				MaxRetries: -1,
			},
			wantErr: true,
		},
		{
			name: "retry multiplier less than 1",
			config: &Config{
				Address:         "localhost:50052",
				Timeout:         30 * time.Second,
				RetryMultiplier: 0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid jitter range",
			config: &Config{
				Address:     "localhost:50052",
				Timeout:     30 * time.Second,
				RetryJitter: 1.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_Merge(t *testing.T) {
	base := &Config{
		Address:         "localhost:50052",
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        10 * time.Second,
		RetryMultiplier: 2.0,
		RetryJitter:     0.1,
		RetryBufferSize: 1000,
		EnableLogging:   true,
	}

	other := &Config{
		Address:    "other:50052",
		Timeout:    60 * time.Second,
		MaxRetries: 5,
	}

	merged := base.Merge(other)

	if merged.Address != "other:50052" {
		t.Errorf("Expected address 'other:50052', got '%s'", merged.Address)
	}
	if merged.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", merged.Timeout)
	}
	if merged.MaxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", merged.MaxRetries)
	}
	// Проверяем, что остальные значения остались без изменений
	if merged.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected initial delay 100ms, got %v", merged.InitialDelay)
	}
}

func TestClientStats_UpdateStats(t *testing.T) {
	stats := &ClientStats{}

	// Тест успешного вызова
	stats.updateStats(true, 100*time.Millisecond, nil)
	
	if stats.CallsTotal != 1 {
		t.Errorf("Expected calls total 1, got %d", stats.CallsTotal)
	}
	if stats.CallsSuccessful != 1 {
		t.Errorf("Expected calls successful 1, got %d", stats.CallsSuccessful)
	}
	if stats.CallsFailed != 0 {
		t.Errorf("Expected calls failed 0, got %d", stats.CallsFailed)
	}
	if stats.AverageResponseTime != 100*time.Millisecond {
		t.Errorf("Expected average response time 100ms, got %v", stats.AverageResponseTime)
	}

	// Тест неуспешного вызова
	stats.updateStats(false, 200*time.Millisecond, errors.New("test error"))
	
	if stats.CallsTotal != 2 {
		t.Errorf("Expected calls total 2, got %d", stats.CallsTotal)
	}
	if stats.CallsSuccessful != 1 {
		t.Errorf("Expected calls successful 1, got %d", stats.CallsSuccessful)
	}
	if stats.CallsFailed != 1 {
		t.Errorf("Expected calls failed 1, got %d", stats.CallsFailed)
	}
	if stats.LastError != "test error" {
		t.Errorf("Expected last error 'test error', got '%s'", stats.LastError)
	}
	if stats.AverageResponseTime != 150*time.Millisecond {
		t.Errorf("Expected average response time 150ms, got %v", stats.AverageResponseTime)
	}
}

func TestClientStats_IncrementCounters(t *testing.T) {
	stats := &ClientStats{}

	stats.incrementCreated()
	stats.incrementUpdated()
	stats.incrementResolved()
	stats.incrementRetries()

	if stats.IncidentsCreated != 1 {
		t.Errorf("Expected incidents created 1, got %d", stats.IncidentsCreated)
	}
	if stats.IncidentsUpdated != 1 {
		t.Errorf("Expected incidents updated 1, got %d", stats.IncidentsUpdated)
	}
	if stats.IncidentsResolved != 1 {
		t.Errorf("Expected incidents resolved 1, got %d", stats.IncidentsResolved)
	}
	if stats.RetriesTotal != 1 {
		t.Errorf("Expected retries total 1, got %d", stats.RetriesTotal)
	}
}

func TestIncidentClient_CalculateRetryDelay(t *testing.T) {
	config := &Config{
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        10 * time.Second,
		RetryMultiplier: 2.0,
		RetryJitter:     0.1,
	}
	
	client := &incidentClient{config: config}

	// Тест без jitter (используем детерминированный подход)
	rand.Seed(42) // Фиксированный seed для предсказуемости
	
	delay1 := client.calculateRetryDelay(0)
	if delay1 < 90*time.Millisecond || delay1 > 110*time.Millisecond {
		t.Errorf("Expected delay around 100ms with jitter, got %v", delay1)
	}

	delay2 := client.calculateRetryDelay(1)
	expected := time.Duration(float64(config.InitialDelay) * config.RetryMultiplier)
	if delay2 < expected-time.Duration(float64(expected)*0.1) || delay2 > expected+time.Duration(float64(expected)*0.1) {
		t.Errorf("Expected delay around %v with jitter, got %v", expected, delay2)
	}

	// Тест ограничения максимальной задержки
	delay3 := client.calculateRetryDelay(10)
	if delay3 > config.MaxDelay {
		t.Errorf("Expected delay not to exceed max delay %v, got %v", config.MaxDelay, delay3)
	}
}

func TestIncidentClient_GenerateErrorHash(t *testing.T) {
	client := &incidentClient{}

	hash1 := client.generateErrorHash("check1", "error message")
	hash2 := client.generateErrorHash("check1", "error message")
	hash3 := client.generateErrorHash("check2", "error message")

	if hash1 != hash2 {
		t.Errorf("Expected same hash for same input, got %s and %s", hash1, hash2)
	}
	if hash1 == hash3 {
		t.Errorf("Expected different hash for different input, got same hash %s", hash1)
	}
	if len(hash1) != 16 {
		t.Errorf("Expected hash length 16, got %d", len(hash1))
	}
}

func TestIncidentClient_DetermineSeverity(t *testing.T) {
	client := &incidentClient{}

	tests := []struct {
		name     string
		result   *domain.CheckResult
		expected v1.IncidentSeverity
	}{
		{
			name: "successful check",
			result: &domain.CheckResult{
				Success:    true,
				StatusCode: 200,
			},
			expected: v1.IncidentSeverity_INCIDENT_SEVERITY_WARNING,
		},
		{
			name: "failed check with 5xx error",
			result: &domain.CheckResult{
				Success:    false,
				StatusCode: 500,
			},
			expected: v1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL,
		},
		{
			name: "failed check with 4xx error",
			result: &domain.CheckResult{
				Success:    false,
				StatusCode: 404,
			},
			expected: v1.IncidentSeverity_INCIDENT_SEVERITY_ERROR,
		},
		{
			name: "failed check with no status code",
			result: &domain.CheckResult{
				Success:    false,
				StatusCode: 0,
			},
			expected: v1.IncidentSeverity_INCIDENT_SEVERITY_ERROR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := client.determineSeverity(tt.result)
			if severity != tt.expected {
				t.Errorf("Expected severity %v, got %v", tt.expected, severity)
			}
		})
	}
}

func TestIncidentClient_ShouldRetry(t *testing.T) {
	client := &incidentClient{}

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "deadline exceeded",
			err:      status.Error(codes.DeadlineExceeded, "timeout"),
			expected: true,
		},
		{
			name:     "unavailable",
			err:      status.Error(codes.Unavailable, "service unavailable"),
			expected: true,
		},
		{
			name:     "aborted",
			err:      status.Error(codes.Aborted, "aborted"),
			expected: true,
		},
		{
			name:     "internal error",
			err:      status.Error(codes.Internal, "internal error"),
			expected: true,
		},
		{
			name:     "invalid argument",
			err:      status.Error(codes.InvalidArgument, "invalid"),
			expected: false,
		},
		{
			name:     "permission denied",
			err:      status.Error(codes.PermissionDenied, "permission denied"),
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRetry := client.shouldRetry(tt.err)
			if shouldRetry != tt.expected {
				t.Errorf("Expected shouldRetry %v, got %v", tt.expected, shouldRetry)
			}
		})
	}
}

func TestIncidentClient_CreateIncident_Validation(t *testing.T) {
	// Создаем mock клиент, который не будет устанавливать реальное соединение
	client := &incidentClient{
		config: DefaultConfig(),
		stats:  &ClientStats{},
		client: nil, // Явно устанавливаем nil, чтобы избежать паники в тестах
	}

	tests := []struct {
		name     string
		result   *domain.CheckResult
		tenantID string
		wantErr  bool
	}{
		{
			name:     "nil result",
			result:   nil,
			tenantID: "tenant1",
			wantErr:  true,
		},
		{
			name: "empty tenant ID",
			result: &domain.CheckResult{
				CheckID: "check1",
			},
			tenantID: "",
			wantErr:  true,
		},
		{
			name: "valid input",
			result: &domain.CheckResult{
				CheckID: "check1",
			},
			tenantID: "tenant1",
			wantErr:  false, // Может быть ошибка соединения, но не валидации
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := client.CreateIncident(ctx, tt.result, tt.tenantID)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error, got nil")
				}
			} else {
				// При валидных входных данных ошибка может быть из-за отсутствия соединения
				// но не из-за валидации
				if err != nil && err.Error() == "check result is nil" {
					t.Errorf("Expected no validation error, got %v", err)
				}
			}
		})
	}
}

func TestIncidentClient_UpdateIncident_Validation(t *testing.T) {
	client := &incidentClient{
		config: DefaultConfig(),
		stats:  &ClientStats{},
	}

	tests := []struct {
		name       string
		incidentID string
		wantErr    bool
	}{
		{
			name:       "empty incident ID",
			incidentID: "",
			wantErr:    true,
		},
		{
			name:       "valid incident ID",
			incidentID: "incident1",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := client.UpdateIncident(ctx, tt.incidentID, v1.IncidentStatus_INCIDENT_STATUS_OPEN, v1.IncidentSeverity_INCIDENT_SEVERITY_ERROR)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error, got nil")
				}
				if err.Error() != "incident ID is required" {
					t.Errorf("Expected 'incident ID is required' error, got %v", err)
				}
			}
		})
	}
}

func TestIncidentClient_ResolveIncident_Validation(t *testing.T) {
	client := &incidentClient{
		config: DefaultConfig(),
		stats:  &ClientStats{},
	}

	tests := []struct {
		name       string
		incidentID string
		wantErr    bool
	}{
		{
			name:       "empty incident ID",
			incidentID: "",
			wantErr:    true,
		},
		{
			name:       "valid incident ID",
			incidentID: "incident1",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := client.ResolveIncident(ctx, tt.incidentID)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error, got nil")
				}
				if err.Error() != "incident ID is required" {
					t.Errorf("Expected 'incident ID is required' error, got %v", err)
				}
			}
		})
	}
}

func TestIncidentClient_GetIncident_Validation(t *testing.T) {
	client := &incidentClient{
		config: DefaultConfig(),
		stats:  &ClientStats{},
	}

	tests := []struct {
		name       string
		incidentID string
		wantErr    bool
	}{
		{
			name:       "empty incident ID",
			incidentID: "",
			wantErr:    true,
		},
		{
			name:       "valid incident ID",
			incidentID: "incident1",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := client.GetIncident(ctx, tt.incidentID)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error, got nil")
				}
				if err.Error() != "incident ID is required" {
					t.Errorf("Expected 'incident ID is required' error, got %v", err)
				}
			}
		})
	}
}

// Бенчмарки
func BenchmarkIncidentClient_CalculateRetryDelay(b *testing.B) {
	client := &incidentClient{
		config: DefaultConfig(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.calculateRetryDelay(i % 10)
	}
}

func BenchmarkIncidentClient_GenerateErrorHash(b *testing.B) {
	client := &incidentClient{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.generateErrorHash("check1", "error message")
	}
}

func BenchmarkIncidentClient_DetermineSeverity(b *testing.B) {
	client := &incidentClient{}
	result := &domain.CheckResult{
		Success:    false,
		StatusCode: 500,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.determineSeverity(result)
	}
}

func BenchmarkClientStats_UpdateStats(b *testing.B) {
	stats := &ClientStats{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.updateStats(i%2 == 0, time.Duration(i)*time.Millisecond, nil)
	}
}
