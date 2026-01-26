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

func TestIncidentService_LifecycleManagement(t *testing.T) {
	repo := &MockIncidentRepository{}
	config := &IncidentConfig{
		AutoResolveTimeout: 1 * time.Millisecond,
		EscalationTimeouts: map[domain.IncidentSeverity]time.Duration{
			domain.IncidentSeverityWarning:  30 * time.Minute,
			domain.IncidentSeverityError:    15 * time.Minute,
			domain.IncidentSeverityCritical: 5 * time.Minute,
		},
		MaxRetriesBeforeEscalation: map[domain.IncidentSeverity]int{
			domain.IncidentSeverityWarning:  5,
			domain.IncidentSeverityError:    3,
			domain.IncidentSeverityCritical: 2,
		},
		IncidentTTL: 7 * 24 * time.Hour,
	}
	log, err := logger.NewLogger("test", "debug", "incident-service", false)
	require.NoError(t, err)
	service := NewIncidentService(repo, config, log)
	
	t.Run("Open incident on first error", func(t *testing.T) {
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
			Return(nil, nil).Once()
		repo.On("GetByTenantID", mock.Anything, result.TenantID, mock.AnythingOfType("*domain.IncidentFilter")).
			Return([]*domain.Incident{}, nil).Once()
		repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil).Once()
		
		// Используем ProcessCheckResultEvent для тестирования новой логики
		err := service.ProcessCheckResultEvent(context.Background(), result)
		
		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})
	
	t.Run("Update existing incident on repeated error", func(t *testing.T) {
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
			Return(existingIncident, nil).Once()
		repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil).Once()
		
		err := service.ProcessCheckResultEvent(context.Background(), result)
		
		assert.NoError(t, err)
		assert.Equal(t, 4, existingIncident.Count) // Счетчик увеличился
		repo.AssertExpectations(t)
	})
	
	t.Run("Close incident on successful check", func(t *testing.T) {
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
			Return([]*domain.Incident{existingIncident}, nil).Once()
		repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil).Once()
		
		err := service.ProcessCheckResultEvent(context.Background(), result)
		
		assert.NoError(t, err)
		assert.True(t, existingIncident.IsResolved())
		repo.AssertExpectations(t)
	})
	
	t.Run("Escalate incident on timeout", func(t *testing.T) {
		existingIncident := domain.NewIncident("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001", domain.IncidentSeverityWarning, "Service unavailable")
		existingIncident.FirstSeen = time.Now().Add(-35 * time.Minute) // Больше таймаута для warning
		existingIncident.Count = 2
		
		result := &CheckResult{
			CheckID:      "550e8400-e29b-41d4-a716-446655440000",
			TenantID:     "550e8400-e29b-41d4-a716-446655440001",
			IsSuccess:    false,
			ErrorMessage: "Service unavailable", // Это будет определено как warning
			Duration:     5 * time.Second,
			Timestamp:    time.Now(),
		}
		
		// Мокируем поиск существующего инцидента
		repo.On("GetByCheckAndErrorHash", mock.Anything, result.CheckID, mock.AnythingOfType("string")).
			Return(existingIncident, nil).Once()
		repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil).Once()
		
		err := service.ProcessCheckResultEvent(context.Background(), result)
		
		assert.NoError(t, err)
		assert.Equal(t, domain.IncidentSeverityError, existingIncident.Severity) // Эскалировано до error
		assert.NotNil(t, existingIncident.Metadata)
		assert.NotNil(t, existingIncident.Metadata["escalation_history"])
		repo.AssertExpectations(t)
	})
	
	t.Run("Group similar errors", func(t *testing.T) {
		existingIncident := domain.NewIncident("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001", domain.IncidentSeverityError, "Database connection failed")
		existingIncident.Metadata = make(map[string]interface{})
		existingIncident.Metadata["grouped_errors"] = []string{"Initial error"}
		
		result := &CheckResult{
			CheckID:      "550e8400-e29b-41d4-a716-446655440000",
			TenantID:     "550e8400-e29b-41d4-a716-446655440001",
			IsSuccess:    false,
			ErrorMessage: "Connection pool exhausted",
			Duration:     5 * time.Second,
			Timestamp:    time.Now(),
		}
		
		// Мокируем отсутствие точного совпадения
		repo.On("GetByCheckAndErrorHash", mock.Anything, result.CheckID, mock.AnythingOfType("string")).
			Return(nil, nil).Once()
		// Мокируем поиск похожих инцидентов
		repo.On("GetByTenantID", mock.Anything, result.TenantID, mock.AnythingOfType("*domain.IncidentFilter")).
			Return([]*domain.Incident{existingIncident}, nil).Once()
		repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil).Once()
		
		err := service.ProcessCheckResultEvent(context.Background(), result)
		
		assert.NoError(t, err)
		assert.Equal(t, 2, existingIncident.Count)
		assert.NotNil(t, existingIncident.Metadata["grouped_errors"])
		repo.AssertExpectations(t)
	})
	
	t.Run("Escalate on high frequency errors", func(t *testing.T) {
		existingIncident := domain.NewIncident("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001", domain.IncidentSeverityError, "Connection timeout")
		existingIncident.FirstSeen = time.Now().Add(-45 * time.Minute) // Длительный инцидент
		existingIncident.Count = 50 // Высокая частота ошибок (> 1 в минуту)
		
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
			Return(existingIncident, nil).Once()
		repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Incident")).Return(nil).Once()
		
		err := service.ProcessCheckResultEvent(context.Background(), result)
		
		assert.NoError(t, err)
		assert.Equal(t, domain.IncidentSeverityCritical, existingIncident.Severity) // Эскалировано до critical
		assert.NotNil(t, existingIncident.Metadata["escalation_history"])
		repo.AssertExpectations(t)
	})
}

func TestIncidentService_ErrorFrequency(t *testing.T) {
	service := &incidentService{}
	
	t.Run("Calculate error frequency", func(t *testing.T) {
		incident := &domain.Incident{
			FirstSeen: time.Now().Add(-10 * time.Minute),
			LastSeen:  time.Now(),
			Count:     20,
		}
		
		frequency := service.calculateErrorFrequency(incident)
		assert.InDelta(t, 2.0, frequency, 0.01) // 20 ошибок за 10 минут = 2 в минуту
	})
	
	t.Run("Should escalate based on frequency", func(t *testing.T) {
		incident := &domain.Incident{
			FirstSeen: time.Now().Add(-45 * time.Minute), // > 30 минут
			LastSeen:  time.Now(),
			Count:     100, // > 1 в минуту
		}
		
		shouldEscalate := service.shouldEscalateBasedOnFrequency(incident)
		assert.True(t, shouldEscalate)
	})
	
	t.Run("Should not escalate - too short duration", func(t *testing.T) {
		incident := &domain.Incident{
			FirstSeen: time.Now().Add(-10 * time.Minute), // < 30 минут
			LastSeen:  time.Now(),
			Count:     100,
		}
		
		shouldEscalate := service.shouldEscalateBasedOnFrequency(incident)
		assert.False(t, shouldEscalate)
	})
	
	t.Run("Should not escalate - low frequency", func(t *testing.T) {
		incident := &domain.Incident{
			FirstSeen: time.Now().Add(-45 * time.Minute), // > 30 минут
			LastSeen:  time.Now(),
			Count:     20, // < 1 в минуту
		}
		
		shouldEscalate := service.shouldEscalateBasedOnFrequency(incident)
		assert.False(t, shouldEscalate)
	})
}

func TestIncidentService_EscalationReason(t *testing.T) {
	service := &incidentService{}
	config := &IncidentConfig{
		EscalationTimeouts: map[domain.IncidentSeverity]time.Duration{
			domain.IncidentSeverityWarning: 30 * time.Minute,
		},
		MaxRetriesBeforeEscalation: map[domain.IncidentSeverity]int{
			domain.IncidentSeverityWarning: 5,
		},
	}
	service.config = config
	
	t.Run("Timeout escalation reason", func(t *testing.T) {
		incident := &domain.Incident{
			FirstSeen: time.Now().Add(-35 * time.Minute), // > 30 минут
			Count:     2,
		}
		
		reason := service.getEscalationReason(domain.IncidentSeverityWarning, incident)
		assert.Equal(t, "timeout", reason)
	})
	
	t.Run("Retry count escalation reason", func(t *testing.T) {
		incident := &domain.Incident{
			FirstSeen: time.Now().Add(-10 * time.Minute), // < 30 минут
			Count:     6, // > 5
		}
		
		reason := service.getEscalationReason(domain.IncidentSeverityWarning, incident)
		assert.Equal(t, "retry_count", reason)
	})
	
	t.Run("High frequency escalation reason", func(t *testing.T) {
		now := time.Now()
		incident := &domain.Incident{
			FirstSeen: now.Add(-45 * time.Minute), // > 30 минут
			LastSeen:  now, // Текущее время
			Count:     50, // > 1 в минуту (50/45 = 1.11)
		}
		
		// Создаем сервис с конфигурацией где timeout > 45 минут для проверки частоты
		config := &IncidentConfig{
			EscalationTimeouts: map[domain.IncidentSeverity]time.Duration{
				domain.IncidentSeverityWarning: 60 * time.Minute, // Больше 45 минут
			},
			MaxRetriesBeforeEscalation: map[domain.IncidentSeverity]int{
				domain.IncidentSeverityWarning: 100, // Больше 50
			},
		}
		service := &incidentService{config: config}
		
		// Отладочный вывод
		freq := service.calculateErrorFrequency(incident)
		shouldEscalate := service.shouldEscalateBasedOnFrequency(incident)
		t.Logf("Frequency: %.2f, Should escalate: %v, Duration: %v", freq, shouldEscalate, incident.GetDuration())
		
		reason := service.getEscalationReason(domain.IncidentSeverityWarning, incident)
		assert.Equal(t, "high_frequency", reason)
	})
}
