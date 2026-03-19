package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/incident-manager/internal/domain"
	"UptimePingPlatform/services/incident-manager/internal/repository"
)

// PostgreSQLIncidentService реализация сервиса инцидентов с PostgreSQL
type PostgreSQLIncidentService struct {
	logger            pkglogger.Logger
	incidentRepo      repository.IncidentRepository
	incidentEventRepo repository.IncidentEventRepository
}

// NewPostgreSQLIncidentService создает новый экземпляр PostgreSQLIncidentService
func NewPostgreSQLIncidentService(
	logger pkglogger.Logger,
	incidentRepo repository.IncidentRepository,
	incidentEventRepo repository.IncidentEventRepository,
) *PostgreSQLIncidentService {
	return &PostgreSQLIncidentService{
		logger:            logger,
		incidentRepo:      incidentRepo,
		incidentEventRepo: incidentEventRepo,
	}
}

// ProcessCheckResult обрабатывает результат проверки
func (s *PostgreSQLIncidentService) ProcessCheckResult(ctx context.Context, result *CheckResult) (*domain.Incident, error) {
	// Для демонстрации создаем инцидент если проверка неуспешна
	if !result.IsSuccess {
		incident := domain.NewIncident(
			result.CheckID,
			domain.IncidentSeverityHigh,
			fmt.Sprintf("Check %s failed", result.CheckID),
			result.ErrorMessage,
		)

		// Сохраняем инцидент
		if err := s.incidentRepo.Create(ctx, incident); err != nil {
			s.logger.Error("Failed to create incident",
				pkglogger.String("check_id", result.CheckID),
				pkglogger.Error(err))
			return nil, err
		}

		// Создаем событие
		event := &domain.IncidentEvent{
			ID:         uuid.New().String(),
			IncidentID: incident.ID,
			EventType:  "created",
			NewStatus:  incident.Status,
			Message:    "Incident created from check result",
			CreatedAt:  time.Now(),
		}

		if err := s.incidentEventRepo.Create(ctx, event); err != nil {
			s.logger.Error("Failed to create incident event",
				pkglogger.String("incident_id", incident.ID),
				pkglogger.Error(err))
			// Не возвращаем ошибку, так как инцидент уже создан
		}

		return incident, nil
	}

	return nil, nil
}

// ProcessCheckResultEvent обрабатывает событие результата проверки
func (s *PostgreSQLIncidentService) ProcessCheckResultEvent(ctx context.Context, result *CheckResult) error {
	s.logger.Info("Processing check result event",
		pkglogger.String("check_id", result.CheckID))

	// Здесь можно добавить логику обработки событий
	return nil
}

// GetIncident получает инцидент по ID
func (s *PostgreSQLIncidentService) GetIncident(ctx context.Context, id string) (*domain.Incident, error) {
	return s.incidentRepo.GetByID(ctx, id)
}

// GetIncidents получает список инцидентов с фильтрацией
func (s *PostgreSQLIncidentService) GetIncidents(ctx context.Context, filter *domain.IncidentFilter) ([]*domain.Incident, error) {
	return s.incidentRepo.List(ctx, filter)
}

// CreateIncident создает новый инцидент
func (s *PostgreSQLIncidentService) CreateIncident(ctx context.Context, incident *domain.Incident) error {
	// Создаем инцидент в БД
	if err := s.incidentRepo.Create(ctx, incident); err != nil {
		s.logger.Error("Failed to create incident",
			pkglogger.String("incident_id", incident.ID),
			pkglogger.Error(err))
		return err
	}

	// Создаем событие о создании
	event := &domain.IncidentEvent{
		ID:        uuid.New().String(),
		IncidentID: incident.ID,
		EventType: "created",
		NewStatus: incident.Status,
		Message:   "Incident created via API",
		CreatedAt: time.Now(),
	}

	if err := s.incidentEventRepo.Create(ctx, event); err != nil {
		s.logger.Error("Failed to create incident event",
			pkglogger.String("incident_id", incident.ID),
			pkglogger.Error(err))
		// Не возвращаем ошибку, так как инцидент уже создан
	}

	s.logger.Info("Incident created",
		pkglogger.String("incident_id", incident.ID))

	return nil
}

// UpdateIncident обновляет инцидент
func (s *PostgreSQLIncidentService) UpdateIncident(ctx context.Context, incident *domain.Incident) error {
	// Получаем текущий инцидент для сравнения
	current, err := s.incidentRepo.GetByID(ctx, incident.ID)
	if err != nil {
		return err
	}

	// Создаем событие об изменении
	event := &domain.IncidentEvent{
		ID:          uuid.New().String(),
		IncidentID:  incident.ID,
		EventType:   "updated",
		OldStatus:   current.Status,
		NewStatus:   incident.Status,
		OldSeverity: current.Severity,
		NewSeverity: incident.Severity,
		Message:     "Incident updated",
		CreatedAt:   time.Now(),
	}

	// Обновляем инцидент
	if err := s.incidentRepo.Update(ctx, incident); err != nil {
		return err
	}

	// Сохраняем событие
	if err := s.incidentEventRepo.Create(ctx, event); err != nil {
		s.logger.Error("Failed to create update event",
			pkglogger.String("incident_id", incident.ID),
			pkglogger.Error(err))
		// Не возвращаем ошибку, так как инцидент уже обновлен
	}

	return nil
}

// AcknowledgeIncident подтверждает инцидент
func (s *PostgreSQLIncidentService) AcknowledgeIncident(ctx context.Context, id string) error {
	incident, err := s.incidentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Обновляем статус
	incident.Acknowledge()
	if err := s.incidentRepo.Update(ctx, incident); err != nil {
		return err
	}

	// Создаем событие
	event := &domain.IncidentEvent{
		ID:         uuid.New().String(),
		IncidentID: id,
		EventType:  "acknowledged",
		NewStatus:  incident.Status,
		Message:    "Incident acknowledged",
		CreatedAt:  time.Now(),
	}

	if err := s.incidentEventRepo.Create(ctx, event); err != nil {
		s.logger.Error("Failed to create acknowledge event",
			pkglogger.String("incident_id", id),
			pkglogger.Error(err))
	}

	return nil
}

// ResolveIncident разрешает инцидент
func (s *PostgreSQLIncidentService) ResolveIncident(ctx context.Context, id string) error {
	incident, err := s.incidentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Обновляем статус
	incident.Resolve()
	if err := s.incidentRepo.Update(ctx, incident); err != nil {
		return err
	}

	// Создаем событие
	event := &domain.IncidentEvent{
		ID:         uuid.New().String(),
		IncidentID: id,
		EventType:  "resolved",
		NewStatus:  incident.Status,
		Message:    "Incident resolved",
		CreatedAt:  time.Now(),
	}

	if err := s.incidentEventRepo.Create(ctx, event); err != nil {
		s.logger.Error("Failed to create resolve event",
			pkglogger.String("incident_id", id),
			pkglogger.Error(err))
	}

	return nil
}

// GetIncidentHistory получает историю инцидента
func (s *PostgreSQLIncidentService) GetIncidentHistory(ctx context.Context, id string) ([]*domain.IncidentEvent, error) {
	return s.incidentEventRepo.GetByIncidentID(ctx, id)
}

// GetIncidentStats получает статистику по инцидентам
func (s *PostgreSQLIncidentService) GetIncidentStats(ctx context.Context, tenantID string) (*domain.IncidentStats, error) {
	return s.incidentRepo.GetStats(ctx, tenantID)
}
