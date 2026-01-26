package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncidentStatus_Constants(t *testing.T) {
	assert.Equal(t, IncidentStatus("open"), IncidentStatusOpen)
	assert.Equal(t, IncidentStatus("acknowledged"), IncidentStatusAcknowledged)
	assert.Equal(t, IncidentStatus("resolved"), IncidentStatusResolved)
}

func TestIncidentSeverity_Constants(t *testing.T) {
	assert.Equal(t, IncidentSeverity("warning"), IncidentSeverityWarning)
	assert.Equal(t, IncidentSeverity("error"), IncidentSeverityError)
	assert.Equal(t, IncidentSeverity("critical"), IncidentSeverityCritical)
}

func TestNewIncident(t *testing.T) {
	checkID := "check-123"
	tenantID := "tenant-456"
	severity := IncidentSeverityError
	errorMessage := "Connection timeout"

	incident := NewIncident(checkID, tenantID, severity, errorMessage)

	require.NotNil(t, incident)
	assert.Equal(t, checkID, incident.CheckID)
	assert.Equal(t, tenantID, incident.TenantID)
	assert.Equal(t, IncidentStatusOpen, incident.Status)
	assert.Equal(t, severity, incident.Severity)
	assert.Equal(t, errorMessage, incident.ErrorMessage)
	assert.Equal(t, 1, incident.Count)
	assert.NotEmpty(t, incident.ErrorHash)
	assert.NotZero(t, incident.FirstSeen)
	assert.NotZero(t, incident.LastSeen)
	assert.NotZero(t, incident.CreatedAt)
	assert.NotZero(t, incident.UpdatedAt)
}

func TestIncident_IsOpen(t *testing.T) {
	tests := []struct {
		name     string
		status   IncidentStatus
		expected bool
	}{
		{"open status", IncidentStatusOpen, true},
		{"acknowledged status", IncidentStatusAcknowledged, false},
		{"resolved status", IncidentStatusResolved, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{Status: tt.status}
			assert.Equal(t, tt.expected, incident.IsOpen())
		})
	}
}

func TestIncident_IsAcknowledged(t *testing.T) {
	tests := []struct {
		name     string
		status   IncidentStatus
		expected bool
	}{
		{"open status", IncidentStatusOpen, false},
		{"acknowledged status", IncidentStatusAcknowledged, true},
		{"resolved status", IncidentStatusResolved, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{Status: tt.status}
			assert.Equal(t, tt.expected, incident.IsAcknowledged())
		})
	}
}

func TestIncident_IsResolved(t *testing.T) {
	tests := []struct {
		name     string
		status   IncidentStatus
		expected bool
	}{
		{"open status", IncidentStatusOpen, false},
		{"acknowledged status", IncidentStatusAcknowledged, false},
		{"resolved status", IncidentStatusResolved, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{Status: tt.status}
			assert.Equal(t, tt.expected, incident.IsResolved())
		})
	}
}

func TestIncident_Acknowledge(t *testing.T) {
	t.Run("acknowledge open incident", func(t *testing.T) {
		incident := &Incident{Status: IncidentStatusOpen}
		before := time.Now()
		
		incident.Acknowledge()
		
		assert.Equal(t, IncidentStatusAcknowledged, incident.Status)
		assert.True(t, incident.UpdatedAt.After(before) || incident.UpdatedAt.Equal(before))
	})

	t.Run("acknowledge acknowledged incident", func(t *testing.T) {
		incident := &Incident{Status: IncidentStatusAcknowledged}
		before := incident.UpdatedAt
		
		incident.Acknowledge()
		
		assert.Equal(t, IncidentStatusAcknowledged, incident.Status)
		assert.Equal(t, before, incident.UpdatedAt)
	})

	t.Run("acknowledge resolved incident", func(t *testing.T) {
		incident := &Incident{Status: IncidentStatusResolved}
		before := incident.UpdatedAt
		
		incident.Acknowledge()
		
		assert.Equal(t, IncidentStatusResolved, incident.Status)
		assert.Equal(t, before, incident.UpdatedAt)
	})
}

func TestIncident_Resolve(t *testing.T) {
	t.Run("resolve open incident", func(t *testing.T) {
		incident := &Incident{Status: IncidentStatusOpen}
		before := time.Now()
		
		incident.Resolve()
		
		assert.Equal(t, IncidentStatusResolved, incident.Status)
		assert.True(t, incident.UpdatedAt.After(before) || incident.UpdatedAt.Equal(before))
	})

	t.Run("resolve acknowledged incident", func(t *testing.T) {
		incident := &Incident{Status: IncidentStatusAcknowledged}
		before := time.Now()
		
		incident.Resolve()
		
		assert.Equal(t, IncidentStatusResolved, incident.Status)
		assert.True(t, incident.UpdatedAt.After(before) || incident.UpdatedAt.Equal(before))
	})

	t.Run("resolve resolved incident", func(t *testing.T) {
		incident := &Incident{Status: IncidentStatusResolved}
		before := incident.UpdatedAt
		
		incident.Resolve()
		
		assert.Equal(t, IncidentStatusResolved, incident.Status)
		assert.Equal(t, before, incident.UpdatedAt)
	})
}

func TestIncident_Reopen(t *testing.T) {
	t.Run("reopen resolved incident", func(t *testing.T) {
		incident := &Incident{Status: IncidentStatusResolved}
		before := time.Now()
		
		incident.Reopen()
		
		assert.Equal(t, IncidentStatusOpen, incident.Status)
		assert.True(t, incident.UpdatedAt.After(before) || incident.UpdatedAt.Equal(before))
	})

	t.Run("reopen open incident", func(t *testing.T) {
		incident := &Incident{Status: IncidentStatusOpen}
		before := incident.UpdatedAt
		
		incident.Reopen()
		
		assert.Equal(t, IncidentStatusOpen, incident.Status)
		assert.Equal(t, before, incident.UpdatedAt)
	})

	t.Run("reopen acknowledged incident", func(t *testing.T) {
		incident := &Incident{Status: IncidentStatusAcknowledged}
		before := incident.UpdatedAt
		
		incident.Reopen()
		
		assert.Equal(t, IncidentStatusAcknowledged, incident.Status)
		assert.Equal(t, before, incident.UpdatedAt)
	})
}

func TestIncident_IncrementCount(t *testing.T) {
	incident := &Incident{
		Count:    1,
		LastSeen: time.Now().Add(-time.Hour),
	}
	before := time.Now()
	
	incident.IncrementCount()
	
	assert.Equal(t, 2, incident.Count)
	assert.True(t, incident.LastSeen.After(before) || incident.LastSeen.Equal(before))
	assert.True(t, incident.UpdatedAt.After(before) || incident.UpdatedAt.Equal(before))
}

func TestIncident_UpdateSeverity(t *testing.T) {
	t.Run("update to different severity", func(t *testing.T) {
		incident := &Incident{Severity: IncidentSeverityWarning}
		before := time.Now()
		
		incident.UpdateSeverity(IncidentSeverityError)
		
		assert.Equal(t, IncidentSeverityError, incident.Severity)
		assert.True(t, incident.UpdatedAt.After(before) || incident.UpdatedAt.Equal(before))
	})

	t.Run("update to same severity", func(t *testing.T) {
		incident := &Incident{Severity: IncidentSeverityError}
		before := incident.UpdatedAt
		
		incident.UpdateSeverity(IncidentSeverityError)
		
		assert.Equal(t, IncidentSeverityError, incident.Severity)
		assert.Equal(t, before, incident.UpdatedAt)
	})
}

func TestIncident_GetDuration(t *testing.T) {
	firstSeen := time.Now().Add(-time.Hour)
	lastSeen := time.Now()
	
	incident := &Incident{
		FirstSeen: firstSeen,
		LastSeen:  lastSeen,
	}
	
	duration := incident.GetDuration()
	assert.True(t, duration > time.Hour-time.Minute)
	assert.True(t, duration < time.Hour+time.Minute)
}

func TestIncident_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   IncidentStatus
		expected bool
	}{
		{"open status", IncidentStatusOpen, true},
		{"acknowledged status", IncidentStatusAcknowledged, true},
		{"resolved status", IncidentStatusResolved, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &Incident{Status: tt.status}
			assert.Equal(t, tt.expected, incident.IsActive())
		})
	}
}

func TestIsValidSeverity(t *testing.T) {
	tests := []struct {
		name     string
		severity IncidentSeverity
		expected bool
	}{
		{"warning severity", IncidentSeverityWarning, true},
		{"error severity", IncidentSeverityError, true},
		{"critical severity", IncidentSeverityCritical, true},
		{"invalid severity", IncidentSeverity("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidSeverity(tt.severity))
		})
	}
}

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   IncidentStatus
		expected bool
	}{
		{"open status", IncidentStatusOpen, true},
		{"acknowledged status", IncidentStatusAcknowledged, true},
		{"resolved status", IncidentStatusResolved, true},
		{"invalid status", IncidentStatus("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidStatus(tt.status))
		})
	}
}

func TestGenerateErrorHash(t *testing.T) {
	tests := []struct {
		name           string
		errorMessage   string
		expectedLength int
	}{
		{"simple error", "Connection timeout", 16},
		{"complex error", "Failed to connect to database: connection refused", 16},
		{"empty error", "", 16},
		{"similar errors", "Connection timeout", 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := generateErrorHash(tt.errorMessage)
			assert.Equal(t, tt.expectedLength, len(hash))
			assert.NotEmpty(t, hash)
		})
	}
}

func TestGenerateErrorHash_Consistency(t *testing.T) {
	errorMessage := "Connection timeout"
	hash1 := generateErrorHash(errorMessage)
	hash2 := generateErrorHash(errorMessage)
	
	assert.Equal(t, hash1, hash2)
}

func TestIncidentFilter(t *testing.T) {
	tenantID := "tenant-123"
	checkID := "check-456"
	status := IncidentStatusOpen
	severity := IncidentSeverityError
	from := time.Now().Add(-time.Hour)
	to := time.Now()

	filter := &IncidentFilter{
		TenantID: &tenantID,
		CheckID:  &checkID,
		Status:   &status,
		Severity: &severity,
		From:     &from,
		To:       &to,
		Limit:    10,
		Offset:   0,
	}

	assert.Equal(t, tenantID, *filter.TenantID)
	assert.Equal(t, checkID, *filter.CheckID)
	assert.Equal(t, status, *filter.Status)
	assert.Equal(t, severity, *filter.Severity)
	assert.Equal(t, from, *filter.From)
	assert.Equal(t, to, *filter.To)
	assert.Equal(t, 10, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
}

func TestIncidentStats(t *testing.T) {
	stats := &IncidentStats{
		Total:      100,
		ByStatus:   map[IncidentStatus]int{IncidentStatusOpen: 50, IncidentStatusResolved: 50},
		BySeverity: map[IncidentSeverity]int{IncidentSeverityError: 30, IncidentSeverityCritical: 20},
		Last24h:    10,
		Last7d:     50,
		Last30d:    100,
	}

	assert.Equal(t, 100, stats.Total)
	assert.Equal(t, 50, stats.ByStatus[IncidentStatusOpen])
	assert.Equal(t, 50, stats.ByStatus[IncidentStatusResolved])
	assert.Equal(t, 30, stats.BySeverity[IncidentSeverityError])
	assert.Equal(t, 20, stats.BySeverity[IncidentSeverityCritical])
	assert.Equal(t, 10, stats.Last24h)
	assert.Equal(t, 50, stats.Last7d)
	assert.Equal(t, 100, stats.Last30d)
}
