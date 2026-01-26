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

func TestIncidentService_ProcessCheckResultEvent_Success(t *testing.T) {
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
	
	err = service.ProcessCheckResultEvent(context.Background(), result)
	
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResultEvent_Success_ResolveIncident(t *testing.T) {
	repo := &MockIncidentRepository{}
	config := &IncidentConfig{
		AutoResolveTimeout: 1 * time.Millisecond,
		IncidentTTL:       7 * 24 * time.Hour,
	}
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
	
	err = service.ProcessCheckResultEvent(context.Background(), result)
	
	assert.NoError(t, err)
	assert.True(t, existingIncident.IsResolved())
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResultEvent_Error_NewIncident(t *testing.T) {
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
	
	err = service.ProcessCheckResultEvent(context.Background(), result)
	
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResultEvent_Error_UpdateExistingIncident(t *testing.T) {
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
	
	err = service.ProcessCheckResultEvent(context.Background(), result)
	
	assert.NoError(t, err)
	assert.Equal(t, 4, existingIncident.Count) // Счетчик увеличился
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResultEvent_Error_ResolvedIncident(t *testing.T) {
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
	
	err = service.ProcessCheckResultEvent(context.Background(), result)
	
	assert.NoError(t, err)
	assert.True(t, existingIncident.IsOpen()) // Инцидент повторно открыт
	repo.AssertExpectations(t)
}

func TestIncidentService_ProcessCheckResultEvent_ValidationError(t *testing.T) {
	repo := &MockIncidentRepository{}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, DefaultIncidentConfig(), log)
	
	// Невалидный результат - пустой check_id
	result := &CheckResult{
		CheckID:   "",
		TenantID:  "550e8400-e29b-41d4-a716-446655440001",
		IsSuccess: true,
		Timestamp: time.Now(),
	}
	
	err = service.ProcessCheckResultEvent(context.Background(), result)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	repo.AssertNotCalled(t, "GetByTenantID")
}

func TestGenerateErrorHash(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{"simple error", "connection timeout", "be3c6a0fb7720bb7"},
		{"another error", "database connection failed", "519da8564d3f5fd0"},
		{"empty message", "", "e3b0c44298fc1c14"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := generateErrorHash(tt.message)
			assert.Equal(t, tt.expected, hash)
		})
	}
}

func TestNormalizeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{"simple message", "Connection timeout", "connection timeout"},
		{"message with spaces", "  Connection timeout  ", "connection timeout"},
		{"empty message", "", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized := normalizeErrorMessage(tt.message)
			assert.Equal(t, tt.expected, normalized)
		})
	}
}
