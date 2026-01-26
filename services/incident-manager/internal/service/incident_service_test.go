package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/incident-manager/internal/domain"
)

// MockIncidentRepository мок репозитория инцидентов
type MockIncidentRepository struct {
	mock.Mock
}

func (m *MockIncidentRepository) Create(ctx context.Context, incident *domain.Incident) error {
	args := m.Called(ctx, incident)
	return args.Error(0)
}

func (m *MockIncidentRepository) GetByID(ctx context.Context, id string) (*domain.Incident, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domain.Incident), args.Error(1)
}

func (m *MockIncidentRepository) GetByCheckAndErrorHash(ctx context.Context, checkID, errorHash string) (*domain.Incident, error) {
	args := m.Called(ctx, checkID, errorHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Incident), args.Error(1)
}

func (m *MockIncidentRepository) GetByTenantID(ctx context.Context, tenantID string, filter *domain.IncidentFilter) ([]*domain.Incident, error) {
	args := m.Called(ctx, tenantID, filter)
	return args.Get(0).([]*domain.Incident), args.Error(1)
}

func (m *MockIncidentRepository) Update(ctx context.Context, incident *domain.Incident) error {
	args := m.Called(ctx, incident)
	return args.Error(0)
}

func (m *MockIncidentRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockIncidentRepository) GetStats(ctx context.Context, tenantID string) (*domain.IncidentStats, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(*domain.IncidentStats), args.Error(1)
}

func TestNewIncidentService(t *testing.T) {
	repo := &MockIncidentRepository{}
	config := DefaultIncidentConfig()
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	
	service := NewIncidentService(repo, config, log)
	
	assert.NotNil(t, service)
}

func TestNewIncidentService_NilConfig(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	
	service := NewIncidentService(repo, nil, log)
	
	assert.NotNil(t, service)
}

func TestNewIncidentService_NilLogger(t *testing.T) {
	repo := &MockIncidentRepository{}
	config := DefaultIncidentConfig()
	
	service := NewIncidentService(repo, config, nil)
	
	assert.NotNil(t, service)
}

func TestIncidentService_ProcessCheckResult_Success(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	result := &CheckResult{
		CheckID:   "550e8400-e29b-41d4-a716-446655440000",
		TenantID:  "550e8400-e29b-41d4-a716-446655440001",
		IsSuccess: true,
		Timestamp: time.Now(),
	}
	
	// Мокируем отсутствие активных инцидентов
	repo.On("GetByTenantID", mock.Anything, result.TenantID, mock.AnythingOfType("*domain.IncidentFilter")).
		Return([]*domain.Incident{}, nil)
	
	incident, err := service.ProcessCheckResult(context.Background(), result)
	
	assert.NoError(t, err)
	assert.Nil(t, incident)
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResult_Success_ResolveIncident(t *testing.T) {
	repo := &MockIncidentRepository{}
	config := DefaultIncidentConfig()
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, config, log)
	
	existingIncident := domain.NewIncident("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001", domain.IncidentSeverityError, "Connection timeout")
	existingIncident.LastSeen = time.Now().Add(-config.AutoResolveTimeout - time.Minute)
	
	result := &CheckResult{
		CheckID:   "550e8400-e29b-41d4-a716-446655440000",
		TenantID:  "550e8400-e29b-41d4-a716-446655440001",
		IsSuccess: true,
		Timestamp: time.Now(),
	}
	
	// Мокируем поиск активного инцидента
	repo.On("GetByTenantID", mock.Anything, result.TenantID, mock.AnythingOfType("*domain.IncidentFilter")).
		Return([]*domain.Incident{existingIncident}, nil)
	
	// Мокируем обновление инцидента
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil)
	
	incident, err := service.ProcessCheckResult(context.Background(), result)
	
	assert.NoError(t, err)
	assert.NotNil(t, incident)
	assert.True(t, incident.IsResolved())
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResult_Error_NewIncident(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	result := &CheckResult{
		CheckID:      "550e8400-e29b-41d4-a716-446655440000",
		TenantID:     "550e8400-e29b-41d4-a716-446655440001",
		IsSuccess:    false,
		ErrorMessage: "Connection timeout",
		Duration:     5 * time.Second,
		Timestamp:    time.Now(),
	}
	
	// Мокируем отсутствие существующего инцидента
	repo.On("GetByCheckAndErrorHash", mock.Anything, result.CheckID, mock.AnythingOfType("string")).
		Return(nil, nil)
	
	// Мокируем создание нового инцидента
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil)
	
	incident, err := service.ProcessCheckResult(context.Background(), result)
	
	assert.NoError(t, err)
	assert.NotNil(t, incident)
	assert.Equal(t, result.CheckID, incident.CheckID)
	assert.Equal(t, result.TenantID, incident.TenantID)
	assert.Equal(t, result.ErrorMessage, incident.ErrorMessage)
	assert.Equal(t, 1, incident.Count)
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResult_Error_UpdateExistingIncident(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	existingIncident := domain.NewIncident("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001", domain.IncidentSeverityError, "Connection timeout")
	existingIncident.Count = 3
	
	result := &CheckResult{
		CheckID:      "550e8400-e29b-41d4-a716-446655440000",
		TenantID:     "550e8400-e29b-41d4-a716-446655440001",
		IsSuccess:    false,
		ErrorMessage: "Connection timeout",
		Duration:     5 * time.Second,
		Timestamp:    time.Now(),
	}
	
	// Мокируем поиск существующего инцидента
	repo.On("GetByCheckAndErrorHash", mock.Anything, result.CheckID, mock.AnythingOfType("string")).
		Return(existingIncident, nil)
	
	// Мокируем обновление инцидента
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil)
	
	incident, err := service.ProcessCheckResult(context.Background(), result)
	
	assert.NoError(t, err)
	assert.NotNil(t, incident)
	assert.Equal(t, 4, incident.Count) // Счетчик увеличился
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResult_Error_ResolvedIncident(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	existingIncident := domain.NewIncident("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001", domain.IncidentSeverityError, "Connection timeout")
	existingIncident.Resolve() // Инцидент разрешен
	
	result := &CheckResult{
		CheckID:      "550e8400-e29b-41d4-a716-446655440000",
		TenantID:     "550e8400-e29b-41d4-a716-446655440001",
		IsSuccess:    false,
		ErrorMessage: "Connection timeout",
		Duration:     5 * time.Second,
		Timestamp:    time.Now(),
	}
	
	// Мокируем поиск существующего инцидента
	repo.On("GetByCheckAndErrorHash", mock.Anything, result.CheckID, mock.AnythingOfType("string")).
		Return(existingIncident, nil)
	
	// Мокируем обновление инцидента
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil)
	
	incident, err := service.ProcessCheckResult(context.Background(), result)
	
	assert.NoError(t, err)
	assert.NotNil(t, incident)
	assert.True(t, incident.IsOpen()) // Инцидент повторно открыт
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResult_Error_Escalation(t *testing.T) {
	repo := &MockIncidentRepository{}
	config := &IncidentConfig{
		EscalationTimeouts: map[domain.IncidentSeverity]time.Duration{
			domain.IncidentSeverityWarning: 1 * time.Millisecond,
		},
		MaxRetriesBeforeEscalation: map[domain.IncidentSeverity]int{
			domain.IncidentSeverityWarning: 2,
		},
		AutoResolveTimeout: 10 * time.Minute,
		IncidentTTL:       7 * 24 * time.Hour,
	}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, config, log)
	
	existingIncident := domain.NewIncident("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001", domain.IncidentSeverityWarning, "Connection timeout")
	existingIncident.FirstSeen = time.Now().Add(-time.Hour) // Давно создан
	existingIncident.Count = 5 // Много повторений
	
	result := &CheckResult{
		CheckID:      "550e8400-e29b-41d4-a716-446655440000",
		TenantID:     "550e8400-e29b-41d4-a716-446655440001",
		IsSuccess:    false,
		ErrorMessage: "Connection timeout",
		Duration:     5 * time.Second,
		Timestamp:    time.Now(),
	}
	
	// Мокируем поиск существующего инцидента
	repo.On("GetByCheckAndErrorHash", mock.Anything, result.CheckID, mock.AnythingOfType("string")).
		Return(existingIncident, nil)
	
	// Мокируем обновление инцидента
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil)
	
	incident, err := service.ProcessCheckResult(context.Background(), result)
	
	assert.NoError(t, err)
	assert.NotNil(t, incident)
	assert.Equal(t, domain.IncidentSeverityCritical, incident.Severity) // Эскалация произошла
	repo.AssertExpectations(t)
}

func TestIncidentService_GetIncident(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	expectedIncident := &domain.Incident{ID: "550e8400-e29b-41d4-a716-446655440000"}
	
	repo.On("GetByID", mock.Anything, "550e8400-e29b-41d4-a716-446655440000").Return(expectedIncident, nil)
	
	incident, err := service.GetIncident(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	
	assert.NoError(t, err)
	assert.Equal(t, expectedIncident, incident)
	repo.AssertExpectations(t)
}

func TestIncidentService_GetIncidents(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	tenantID := "550e8400-e29b-41d4-a716-446655440001"
	expectedIncidents := []*domain.Incident{{ID: "550e8400-e29b-41d4-a716-446655440000"}}
	
	repo.On("GetByTenantID", mock.Anything, tenantID, mock.AnythingOfType("*domain.IncidentFilter")).
		Return(expectedIncidents, nil)
	
	incidents, err := service.GetIncidents(context.Background(), &domain.IncidentFilter{
		TenantID: &tenantID,
	})
	
	assert.NoError(t, err)
	assert.Equal(t, expectedIncidents, incidents)
	repo.AssertExpectations(t)
}

func TestIncidentService_GetIncidents_MissingTenantID(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	incidents, err := service.GetIncidents(context.Background(), &domain.IncidentFilter{})
	
	assert.Error(t, err)
	assert.Nil(t, incidents)
	assert.Contains(t, err.Error(), "tenant_id is required")
}

func TestIncidentService_AcknowledgeIncident(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	incident := domain.NewIncident("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001", domain.IncidentSeverityError, "Connection timeout")
	
	repo.On("GetByID", mock.Anything, "550e8400-e29b-41d4-a716-446655440000").Return(incident, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil)
	
	err = service.AcknowledgeIncident(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	
	assert.NoError(t, err)
	assert.True(t, incident.IsAcknowledged())
	repo.AssertExpectations(t)
}

func TestIncidentService_ResolveIncident(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	incident := domain.NewIncident("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001", domain.IncidentSeverityError, "Connection timeout")
	
	repo.On("GetByID", mock.Anything, "550e8400-e29b-41d4-a716-446655440000").Return(incident, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil)
	
	err = service.ResolveIncident(context.Background(), "550e8400-e29b-41d4-a716-446655440000")
	
	assert.NoError(t, err)
	assert.True(t, incident.IsResolved())
	repo.AssertExpectations(t)
}

func TestIncidentService_GetIncidentStats(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	expectedStats := &domain.IncidentStats{Total: 10}
	
	repo.On("GetStats", mock.Anything, "550e8400-e29b-41d4-a716-446655440001").Return(expectedStats, nil)
	
	stats, err := service.GetIncidentStats(context.Background(), "550e8400-e29b-41d4-a716-446655440001")
	
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, stats)
	repo.AssertExpectations(t)
}

func TestDetermineSeverity_Critical(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(nil, DefaultIncidentConfig(), log)
	
	tests := []struct {
		name        string
		errorMsg    string
		duration    time.Duration
		expected    domain.IncidentSeverity
	}{
		{"panic error", "panic: runtime error", 1 * time.Second, domain.IncidentSeverityCritical},
		{"fatal error", "fatal error occurred", 1 * time.Second, domain.IncidentSeverityCritical},
		{"long duration", "connection timeout", 35 * time.Second, domain.IncidentSeverityCritical},
		{"database error", "database connection failed", 1 * time.Second, domain.IncidentSeverityCritical},
		{"auth error", "authentication failed", 1 * time.Second, domain.IncidentSeverityCritical},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := service.(*incidentService).determineSeverity(tt.errorMsg, tt.duration)
			assert.Equal(t, tt.expected, severity)
		})
	}
}

func TestDetermineSeverity_Error(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(nil, DefaultIncidentConfig(), log)
	
	tests := []struct {
		name        string
		errorMsg    string
		duration    time.Duration
		expected    domain.IncidentSeverity
	}{
		{"error keyword", "some error occurred", 1 * time.Second, domain.IncidentSeverityError},
		{"failed keyword", "operation failed", 1 * time.Second, domain.IncidentSeverityError},
		{"medium duration", "connection timeout", 15 * time.Second, domain.IncidentSeverityCritical},
		{"exception keyword", "null pointer exception", 1 * time.Second, domain.IncidentSeverityError},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := service.(*incidentService).determineSeverity(tt.errorMsg, tt.duration)
			assert.Equal(t, tt.expected, severity)
		})
	}
}

func TestDetermineSeverity_Warning(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(nil, DefaultIncidentConfig(), log)
	
	tests := []struct {
		name        string
		errorMsg    string
		duration    time.Duration
		expected    domain.IncidentSeverity
	}{
		{"simple message", "something happened", 1 * time.Second, domain.IncidentSeverityWarning},
		{"short duration", "connection timeout", 5 * time.Second, domain.IncidentSeverityCritical},
		{"no keywords", "just a message", 1 * time.Second, domain.IncidentSeverityWarning},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := service.(*incidentService).determineSeverity(tt.errorMsg, tt.duration)
			assert.Equal(t, tt.expected, severity)
		})
	}
}

func TestDefaultIncidentConfig(t *testing.T) {
	config := DefaultIncidentConfig()
	
	assert.NotNil(t, config)
	assert.Equal(t, 30*time.Minute, config.EscalationTimeouts[domain.IncidentSeverityWarning])
	assert.Equal(t, 15*time.Minute, config.EscalationTimeouts[domain.IncidentSeverityError])
	assert.Equal(t, 5*time.Minute, config.EscalationTimeouts[domain.IncidentSeverityCritical])
	assert.Equal(t, 10, config.MaxRetriesBeforeEscalation[domain.IncidentSeverityWarning])
	assert.Equal(t, 5, config.MaxRetriesBeforeEscalation[domain.IncidentSeverityError])
	assert.Equal(t, 2, config.MaxRetriesBeforeEscalation[domain.IncidentSeverityCritical])
	assert.Equal(t, 10*time.Minute, config.AutoResolveTimeout)
	assert.Equal(t, 7*24*time.Hour, config.IncidentTTL)
}

func TestContainsCriticalKeyword(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{"panic", "panic: runtime error", true},
		{"fatal", "fatal error occurred", true},
		{"crash", "application crash", true},
		{"timeout", "request timeout", true},
		{"database", "database connection failed", true},
		{"auth", "authentication failed", true},
		{"simple", "simple message", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsCriticalKeyword(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsErrorKeyword(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{"error", "some error occurred", true},
		{"failed", "operation failed", true},
		{"exception", "null pointer exception", true},
		{"refused", "connection refused", true},
		{"denied", "access denied", true},
		{"simple", "simple message", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsErrorKeyword(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}
