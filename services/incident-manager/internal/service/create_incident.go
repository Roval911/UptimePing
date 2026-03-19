package service

import (
	"context"
	"fmt"
	"time"

	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/incident-manager/internal/domain"
)

// CreateIncident создает новый инцидент
func (s *incidentService) CreateIncident(ctx context.Context, incident *domain.Incident) error {
	if incident == nil {
		return fmt.Errorf("incident cannot be nil")
	}

	s.logger.Debug("Creating incident",
		pkglogger.String("incident_id", incident.ID))

	// Валидация
	if err := s.validator.ValidateRequiredFields(
		map[string]interface{}{
			"id":     incident.ID,
			"title":  incident.Title,
			"status": incident.Status,
		},
		map[string]string{
			"id":     "incident ID is required",
			"title":  "incident title is required",
			"status": "incident status is required",
		},
	); err != nil {
		s.logger.Error("Incident validation failed",
			pkglogger.String("incident_id", incident.ID),
			pkglogger.Error(err))
		return err
	}

	// Устанавливаем время создания если не установлено
	if incident.CreatedAt.IsZero() {
		incident.CreatedAt = time.Now()
	}
	incident.UpdatedAt = time.Now()

	// Сохраняем через репозиторий
	if err := s.repo.Create(ctx, incident); err != nil {
		s.logger.Error("Failed to create incident",
			pkglogger.String("incident_id", incident.ID),
			pkglogger.Error(err))
		return fmt.Errorf("failed to create incident: %w", err)
	}

	s.logger.Info("Incident created successfully",
		pkglogger.String("incident_id", incident.ID))

	return nil
}

// CreateIncident создает новый инцидент (InMemory)
func (s *InMemoryIncidentService) CreateIncident(ctx context.Context, incident *domain.Incident) error {
	if incident == nil {
		return fmt.Errorf("incident cannot be nil")
	}

	s.logger.Debug("Creating incident",
		pkglogger.String("incident_id", incident.ID))

	// Устанавливаем время создания если не установлено
	if incident.CreatedAt.IsZero() {
		incident.CreatedAt = time.Now()
	}
	incident.UpdatedAt = time.Now()

	// Сохраняем в память
	s.mu.Lock()
	s.incidents[incident.ID] = incident
	s.mu.Unlock()

	s.logger.Info("Incident created successfully",
		pkglogger.String("incident_id", incident.ID))

	return nil
}
