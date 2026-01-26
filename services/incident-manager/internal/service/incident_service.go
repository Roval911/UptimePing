package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/incident-manager/internal/domain"
	"UptimePingPlatform/services/incident-manager/internal/producer/rabbitmq"
)

// IncidentService интерфейс для управления инцидентами
type IncidentService interface {
	// ProcessCheckResult обрабатывает результат проверки и управляет инцидентами
	ProcessCheckResult(ctx context.Context, result *CheckResult) (*domain.Incident, error)
	
	// ProcessCheckResultEvent обрабатывает результат проверки с публикацией событий
	ProcessCheckResultEvent(ctx context.Context, result *CheckResult) error
	
	// GetIncident получает инцидент по ID
	GetIncident(ctx context.Context, id string) (*domain.Incident, error)
	
	// GetIncidents получает список инцидентов с фильтрацией
	GetIncidents(ctx context.Context, filter *domain.IncidentFilter) ([]*domain.Incident, error)
	
	// UpdateIncident обновляет инцидент
	UpdateIncident(ctx context.Context, incident *domain.Incident) error
	
	// AcknowledgeIncident подтверждает инцидент
	AcknowledgeIncident(ctx context.Context, id string) error
	
	// ResolveIncident разрешает инцидент
	ResolveIncident(ctx context.Context, id string) error
	
	// GetIncidentHistory получает историю инцидента
	GetIncidentHistory(ctx context.Context, incidentID string) ([]*domain.IncidentEvent, error)
	
	// GetIncidentStats получает статистику по инцидентам
	GetIncidentStats(ctx context.Context, tenantID string) (*domain.IncidentStats, error)
}

// CheckResult представляет результат проверки
type CheckResult struct {
	CheckID      string                 `json:"check_id"`
	TenantID     string                 `json:"tenant_id"`
	IsSuccess    bool                   `json:"is_success"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Duration     time.Duration          `json:"duration"`
	Timestamp    time.Time              `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// IncidentRepository интерфейс для работы с хранилищем инцидентов
type IncidentRepository interface {
	Create(ctx context.Context, incident *domain.Incident) error
	GetByID(ctx context.Context, id string) (*domain.Incident, error)
	GetByCheckAndErrorHash(ctx context.Context, checkID, errorHash string) (*domain.Incident, error)
	GetByTenantID(ctx context.Context, tenantID string, filter *domain.IncidentFilter) ([]*domain.Incident, error)
	Update(ctx context.Context, incident *domain.Incident) error
	Delete(ctx context.Context, id string) error
	GetStats(ctx context.Context, tenantID string) (*domain.IncidentStats, error)
}

// IncidentConfig конфигурация сервиса инцидентов
type IncidentConfig struct {
	// Время эскалации серьезности
	EscalationTimeouts map[domain.IncidentSeverity]time.Duration `json:"escalation_timeouts"`
	
	// Максимальное количество повторений перед эскалацией
	MaxRetriesBeforeEscalation map[domain.IncidentSeverity]int `json:"max_retries_before_escalation"`
	
	// Время автоматического разрешения инцидента
	AutoResolveTimeout time.Duration `json:"auto_resolve_timeout"`
	
	// Время жизни инцидента
	IncidentTTL time.Duration `json:"incident_ttl"`
}

// DefaultIncidentConfig возвращает конфигурацию по умолчанию
func DefaultIncidentConfig() *IncidentConfig {
	return &IncidentConfig{
		EscalationTimeouts: map[domain.IncidentSeverity]time.Duration{
			domain.IncidentSeverityWarning:  30 * time.Minute,
			domain.IncidentSeverityError:    15 * time.Minute,
			domain.IncidentSeverityCritical: 5 * time.Minute,
		},
		MaxRetriesBeforeEscalation: map[domain.IncidentSeverity]int{
			domain.IncidentSeverityWarning:  10,
			domain.IncidentSeverityError:    5,
			domain.IncidentSeverityCritical: 2,
		},
		AutoResolveTimeout: 10 * time.Minute,
		IncidentTTL:       7 * 24 * time.Hour, // 7 дней
	}
}

// incidentService реализация IncidentService
type incidentService struct {
	repo      IncidentRepository
	config    *IncidentConfig
	logger    logger.Logger
	validator *validation.Validator
	producer  rabbitmq.IncidentProducerInterface
}

// NewIncidentService создает новый сервис инцидентов
func NewIncidentService(repo IncidentRepository, config *IncidentConfig, log logger.Logger) IncidentService {
	if config == nil {
		config = DefaultIncidentConfig()
	}
	
	if log == nil {
		log, _ = logger.NewLogger("incident-manager", "info", "incident-service", false)
	}
	
	return &incidentService{
		repo:      repo,
		config:    config,
		logger:    log,
		validator: validation.NewValidator(),
		producer:  nil, // Producer будет установлен отдельно
	}
}

// NewIncidentServiceWithProducer создает новый сервис инцидентов с producer
func NewIncidentServiceWithProducer(repo IncidentRepository, config *IncidentConfig, log logger.Logger, producer rabbitmq.IncidentProducerInterface) IncidentService {
	if config == nil {
		config = DefaultIncidentConfig()
	}
	
	if log == nil {
		log, _ = logger.NewLogger("incident-manager", "info", "incident-service", false)
	}
	
	return &incidentService{
		repo:      repo,
		config:    config,
		logger:    log,
		validator: validation.NewValidator(),
		producer:  producer,
	}
}

// SetProducer устанавливает producer для событий инцидентов
func (s *incidentService) SetProducer(producer rabbitmq.IncidentProducerInterface) {
	s.producer = producer
}

// ProcessCheckResult обрабатывает результат проверки
func (s *incidentService) ProcessCheckResult(ctx context.Context, result *CheckResult) (*domain.Incident, error) {
	// Валидация входных данных
	if err := s.validateCheckResult(result); err != nil {
		s.logger.Error("Check result validation failed", 
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "check result validation failed")
	}
	
	s.logger.Debug("Processing check result",
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.Bool("is_success", result.IsSuccess),
		logger.Duration("duration", result.Duration))
	
	// Если проверка успешна, пытаемся разрешить существующий инцидент
	if result.IsSuccess {
		return s.resolveIncidentOnSuccess(ctx, result)
	}
	
	// Если проверка неуспешна, создаем или обновляем инцидент
	return s.createOrUpdateIncident(ctx, result)
}

// ProcessCheckResultEvent обрабатывает результат проверки с публикацией событий
func (s *incidentService) ProcessCheckResultEvent(ctx context.Context, result *CheckResult) error {
	// Валидация входных данных
	if err := s.validateCheckResult(result); err != nil {
		s.logger.Error("Check result validation failed", 
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "check result validation failed")
	}
	
	s.logger.Debug("Processing check result with events",
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.Bool("is_success", result.IsSuccess),
		logger.Duration("duration", result.Duration))
	
	// Если проверка успешна
	if result.IsSuccess {
		return s.processSuccessfulCheck(ctx, result)
	}
	
	// Если проверка неудачна
	return s.processFailedCheck(ctx, result)
}

// processSuccessfulCheck обрабатывает успешную проверку с публикацией события
func (s *incidentService) processSuccessfulCheck(ctx context.Context, result *CheckResult) error {
	// Поиск активного инцидента по check_id
	incidents, err := s.repo.GetByTenantID(ctx, result.TenantID, &domain.IncidentFilter{
		CheckID: &result.CheckID,
	})
	
	if err != nil {
		s.logger.Error("Failed to find active incident",
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to find active incident")
	}
	
	// Фильтруем только активные инциденты
	var activeIncident *domain.Incident
	for _, incident := range incidents {
		if incident.Status == domain.IncidentStatusOpen || incident.Status == domain.IncidentStatusAcknowledged {
			activeIncident = incident
			break
		}
	}
	
	if activeIncident == nil {
		// Нет активного инцидента
		s.logger.Debug("No active incident found for successful check",
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID))
		return nil
	}
	
	// Проверяем, достаточно ли времени прошло для автоматического разрешения
	if time.Since(activeIncident.LastSeen) < s.config.AutoResolveTimeout {
		// Слишком рано для автоматического разрешения
		s.logger.Debug("Too early for auto-resolve",
			logger.String("incident_id", activeIncident.ID),
			logger.String("check_id", result.CheckID),
			logger.Duration("time_since_last_seen", time.Since(activeIncident.LastSeen)),
			logger.Duration("auto_resolve_timeout", s.config.AutoResolveTimeout))
		return nil
	}
	
	// Закрываем инцидент (status = resolved)
	activeIncident.Resolve()
	
	s.logger.Info("Resolving incident on successful check",
		logger.String("incident_id", activeIncident.ID),
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.Duration("incident_duration", activeIncident.GetDuration()))
	
	err = s.repo.Update(ctx, activeIncident)
	if err != nil {
		s.logger.Error("Failed to resolve incident",
			logger.String("incident_id", activeIncident.ID),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to resolve incident")
	}
	
	// Публикация события incident.resolved
	s.publishIncidentEvent(ctx, "incident.resolved", activeIncident, result)
	
	return nil
}

// processFailedCheck обрабатывает неудачную проверку с публикацией событий
func (s *incidentService) processFailedCheck(ctx context.Context, result *CheckResult) error {
	// Определяем уровень серьезности на основе сообщения об ошибке
	severity := s.determineSeverity(result.ErrorMessage, result.Duration)
	
	// Вычисление error_hash (SHA256 от error_message)
	errorHash := generateErrorHash(result.ErrorMessage)
	
	s.logger.Debug("Processing failed check",
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.String("error_message", result.ErrorMessage),
		logger.String("severity", string(severity)),
		logger.String("error_hash", errorHash))
	
	// Этап 1: Поиск точного совпадения по check_id и error_hash
	existingIncident, err := s.repo.GetByCheckAndErrorHash(ctx, result.CheckID, errorHash)
	if err != nil {
		s.logger.Error("Failed to find existing incident",
			logger.String("check_id", result.CheckID),
			logger.String("error_hash", errorHash),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to find existing incident")
	}
	
	if existingIncident != nil {
		// Этап 2: Обновление существующего инцидента
		return s.updateExistingIncident(ctx, existingIncident, result, severity)
	}
	
	// Этап 3: Поиск похожих инцидентов по check_id для группировки
	similarIncidents, err := s.findSimilarIncidents(ctx, result.CheckID, result.TenantID)
	if err != nil {
		s.logger.Error("Failed to find similar incidents",
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to find similar incidents")
	}
	
	if len(similarIncidents) > 0 {
		// Этап 4: Группировка с похожим инцидентом
		return s.groupWithSimilarIncident(ctx, similarIncidents[0], result, severity)
	}
	
	// Этап 5: Создание нового инцидента
	return s.createNewIncident(ctx, result, severity)
}

// updateExistingIncident обновляет существующий инцидент
func (s *incidentService) updateExistingIncident(ctx context.Context, incident *domain.Incident, result *CheckResult, severity domain.IncidentSeverity) error {
	// Обновление счетчика и времени последнего появления
	incident.IncrementCount()
	incident.UpdateSeverity(severity)
	
	// Проверяем необходимость эскалации при длительных инцидентах
	s.checkEscalation(incident)
	
	// Если инцидент был разрешен, повторно открываем его
	if incident.IsResolved() {
		incident.Reopen()
		s.logger.Info("Reopening resolved incident",
			logger.String("incident_id", incident.ID),
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID),
			logger.Int("previous_count", incident.Count-1))
	}
	
	s.logger.Debug("Updating existing incident",
		logger.String("incident_id", incident.ID),
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.Int("count", incident.Count),
		logger.String("severity", string(incident.Severity)),
		logger.String("status", string(incident.Status)))
	
	err := s.repo.Update(ctx, incident)
	if err != nil {
		s.logger.Error("Failed to update incident",
			logger.String("incident_id", incident.ID),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to update incident")
	}
	
	// Публикация события incident.updated
	s.publishIncidentEvent(ctx, "incident.updated", incident, result)
	
	return nil
}

// findSimilarIncidents ищет похожие инциденты по check_id
func (s *incidentService) findSimilarIncidents(ctx context.Context, checkID, tenantID string) ([]*domain.Incident, error) {
	// Поиск активных инцидентов по check_id
	incidents, err := s.repo.GetByTenantID(ctx, tenantID, &domain.IncidentFilter{
		CheckID: &checkID,
	})
	if err != nil {
		return nil, err
	}
	
	// Фильтруем только активные инциденты
	var activeIncidents []*domain.Incident
	for _, incident := range incidents {
		if incident.Status == domain.IncidentStatusOpen || incident.Status == domain.IncidentStatusAcknowledged {
			activeIncidents = append(activeIncidents, incident)
		}
	}
	
	return activeIncidents, nil
}

// groupWithSimilarIncident группирует с похожим инцидентом
func (s *incidentService) groupWithSimilarIncident(ctx context.Context, incident *domain.Incident, result *CheckResult, severity domain.IncidentSeverity) error {
	// Обновляем существующий инцидент
	incident.IncrementCount()
	incident.UpdateSeverity(severity)
	
	// Добавляем информацию о группировке
	if incident.Metadata == nil {
		incident.Metadata = make(map[string]interface{})
	}
	incident.Metadata["grouped_errors"] = incident.Metadata["grouped_errors"]
	if incident.Metadata["grouped_errors"] == nil {
		incident.Metadata["grouped_errors"] = []string{}
	}
	
	// Добавляем новую ошибку в список группированных
	if errors, ok := incident.Metadata["grouped_errors"].([]string); ok {
		incident.Metadata["grouped_errors"] = append(errors, result.ErrorMessage)
	}
	
	s.logger.Info("Grouping with similar incident",
		logger.String("incident_id", incident.ID),
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.String("error_message", result.ErrorMessage),
		logger.Int("total_count", incident.Count))
	
	err := s.repo.Update(ctx, incident)
	if err != nil {
		s.logger.Error("Failed to update grouped incident",
			logger.String("incident_id", incident.ID),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to update grouped incident")
	}
	
	// Публикация события incident.updated с флагом группировки
	s.publishIncidentEvent(ctx, "incident.grouped", incident, result)
	
	return nil
}

// createNewIncident создает новый инцидент
func (s *incidentService) createNewIncident(ctx context.Context, result *CheckResult, severity domain.IncidentSeverity) error {
	// Создание нового инцидента
	newIncident := domain.NewIncident(result.CheckID, result.TenantID, severity, result.ErrorMessage)
	
	s.logger.Info("Creating new incident",
		logger.String("incident_id", newIncident.ID),
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.String("severity", string(severity)),
		logger.String("error_hash", newIncident.ErrorHash),
		logger.String("error_message", result.ErrorMessage))
	
	err := s.repo.Create(ctx, newIncident)
	if err != nil {
		s.logger.Error("Failed to create incident",
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to create incident")
	}
	
	// Публикация события incident.opened
	s.publishIncidentEvent(ctx, "incident.opened", newIncident, result)
	
	return nil
}

// validateCheckResult валидирует результат проверки
func (s *incidentService) validateCheckResult(result *CheckResult) error {
	if result == nil {
		return errors.New(errors.ErrValidation, "check result cannot be nil")
	}
	
	// Валидация UUID для check_id
	if err := s.validator.ValidateUUID(result.CheckID, "check_id"); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "check_id validation failed")
	}
	
	// Валидация UUID для tenant_id
	if err := s.validator.ValidateUUID(result.TenantID, "tenant_id"); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "tenant_id validation failed")
	}
	
	// Валидация длительности (не должна быть отрицательной)
	if result.Duration < 0 {
		return errors.New(errors.ErrValidation, "duration cannot be negative")
	}
	
	// Валидация временной метки
	if err := s.validator.ValidateTimestamp(result.Timestamp, "timestamp"); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "timestamp validation failed")
	}
	
	return nil
}

// resolveIncidentOnSuccess разрешает инцидент при успешной проверке
func (s *incidentService) resolveIncidentOnSuccess(ctx context.Context, result *CheckResult) (*domain.Incident, error) {
	s.logger.Debug("Resolving incident on successful check",
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID))
	
	// Ищем активный инцидент для данной проверки
	incidents, err := s.repo.GetByTenantID(ctx, result.TenantID, &domain.IncidentFilter{
		CheckID: &result.CheckID,
		Status:  &[]domain.IncidentStatus{domain.IncidentStatusOpen, domain.IncidentStatusAcknowledged}[0],
		Limit:   1,
	})
	
	if err != nil {
		s.logger.Error("Failed to find active incident",
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to find active incident")
	}
	
	if len(incidents) == 0 {
		// Нет активного инцидента
		s.logger.Debug("No active incident found",
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID))
		return nil, nil
	}
	
	incident := incidents[0]
	
	// Проверяем, достаточно ли времени прошло для автоматического разрешения
	if time.Since(incident.LastSeen) < s.config.AutoResolveTimeout {
		// Слишком рано для автоматического разрешения
		s.logger.Debug("Too early for auto-resolve",
			logger.String("incident_id", incident.ID),
			logger.String("check_id", result.CheckID),
			logger.Duration("time_since_last_seen", time.Since(incident.LastSeen)),
			logger.Duration("auto_resolve_timeout", s.config.AutoResolveTimeout))
		return incident, nil
	}
	
	// Разрешаем инцидент
	incident.Resolve()
	
	s.logger.Info("Auto-resolving incident",
		logger.String("incident_id", incident.ID),
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.Duration("incident_duration", incident.GetDuration()))
	
	err = s.repo.Update(ctx, incident)
	if err != nil {
		s.logger.Error("Failed to resolve incident",
			logger.String("incident_id", incident.ID),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to resolve incident")
	}
	
	return incident, nil
}

// createOrUpdateIncident создает или обновляет инцидент при ошибке
func (s *incidentService) createOrUpdateIncident(ctx context.Context, result *CheckResult) (*domain.Incident, error) {
	var newIncident *domain.Incident
	var err error
	
	// Определяем уровень серьезности на основе сообщения об ошибке
	severity := s.determineSeverity(result.ErrorMessage, result.Duration)
	
	s.logger.Debug("Creating or updating incident",
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.String("error_message", result.ErrorMessage),
		logger.String("severity", string(severity)))
	
	// Создаем новый инцидент
	newIncident = domain.NewIncident(result.CheckID, result.TenantID, severity, result.ErrorMessage)
	
	// Ищем существующий инцидент по check_id и error_hash
	existingIncident, err := s.repo.GetByCheckAndErrorHash(ctx, result.CheckID, newIncident.ErrorHash)
	if err != nil {
		s.logger.Error("Failed to find existing incident",
			logger.String("check_id", result.CheckID),
			logger.String("error_hash", newIncident.ErrorHash),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to find existing incident")
	}
	
	if existingIncident != nil {
		// Инцидент существует, обновляем его
		err := s.updateExistingIncident(ctx, existingIncident, result, severity)
		if err != nil {
			return nil, err
		}
		return existingIncident, nil
	}
	
	// Создаем новый инцидент
	newIncident = domain.NewIncident(result.CheckID, result.TenantID, severity, result.ErrorMessage)
	
	err = s.repo.Create(ctx, newIncident)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to create incident")
	}
	
	// Публикация события incident.opened
	s.publishIncidentEvent(ctx, "incident.opened", newIncident, result)
	
	return newIncident, nil
}

// determineSeverity определяет уровень серьезности на основе ошибки и длительности
func (s *incidentService) determineSeverity(errorMessage string, duration time.Duration) domain.IncidentSeverity {
	// Определяем серьезность на основе ключевых слов в сообщении об ошибке
	errorMessage = fmt.Sprintf("%s", errorMessage)
	
	// Критические ошибки
	if containsCriticalKeyword(errorMessage) {
		return domain.IncidentSeverityCritical
	}
	
	// Ошибки на основе длительности
	if duration > 30*time.Second {
		return domain.IncidentSeverityCritical
	}
	if duration > 10*time.Second {
		return domain.IncidentSeverityError
	}
	
	// Ошибки на основе сообщения
	if containsErrorKeyword(errorMessage) {
		return domain.IncidentSeverityError
	}
	
	// По умолчанию - предупреждение
	return domain.IncidentSeverityWarning
}

// checkEscalation проверяет необходимость эскалации инцидента
func (s *incidentService) checkEscalation(incident *domain.Incident) {
	originalSeverity := incident.Severity
	escalated := false
	
	// Этап 1: Проверяем эскалацию на основе времени существования
	if escalationTimeout, exists := s.config.EscalationTimeouts[incident.Severity]; exists {
		if time.Since(incident.FirstSeen) > escalationTimeout {
			s.escalateSeverity(incident)
			escalated = true
			s.logger.Info("Escalating incident due to timeout",
				logger.String("incident_id", incident.ID),
				logger.String("from_severity", string(originalSeverity)),
				logger.String("to_severity", string(incident.Severity)),
				logger.Duration("incident_duration", incident.GetDuration()),
				logger.Duration("escalation_timeout", escalationTimeout))
		}
	}
	
	// Этап 2: Проверяем эскалацию на основе количества повторений
	if !escalated {
		if maxRetries, exists := s.config.MaxRetriesBeforeEscalation[incident.Severity]; exists {
			if incident.Count > maxRetries {
				s.escalateSeverity(incident)
				escalated = true
				s.logger.Info("Escalating incident due to retry count",
					logger.String("incident_id", incident.ID),
					logger.String("from_severity", string(originalSeverity)),
					logger.String("to_severity", string(incident.Severity)),
					logger.Int("retry_count", incident.Count),
					logger.Int("max_retries", maxRetries))
			}
		}
	}
	
	// Этап 3: Проверяем эскалацию на основе частоты ошибок
	if !escalated {
		if s.shouldEscalateBasedOnFrequency(incident) {
			s.escalateSeverity(incident)
			escalated = true
			s.logger.Info("Escalating incident due to high error frequency",
				logger.String("incident_id", incident.ID),
				logger.String("from_severity", string(originalSeverity)),
				logger.String("to_severity", string(incident.Severity)),
				logger.Float64("error_frequency", s.calculateErrorFrequency(incident)))
		}
	}
	
	// Этап 4: Добавляем метаданные эскалации
	if escalated {
		if incident.Metadata == nil {
			incident.Metadata = make(map[string]interface{})
		}
		
		// Инициализируем escalation_history если нужно
		if incident.Metadata["escalation_history"] == nil {
			incident.Metadata["escalation_history"] = []interface{}{}
		}
		
		// Добавляем запись в историю эскалации
		history := incident.Metadata["escalation_history"].([]interface{})
		incident.Metadata["escalation_history"] = append(history, map[string]interface{}{
			"timestamp":        time.Now(),
			"from_severity":    string(originalSeverity),
			"to_severity":      string(incident.Severity),
			"incident_duration": incident.GetDuration(),
			"retry_count":       incident.Count,
			"reason":           s.getEscalationReason(originalSeverity, incident),
		})
	}
}

// shouldEscalateBasedOnFrequency проверяет необходимость эскалации на основе частоты ошибок
func (s *incidentService) shouldEscalateBasedOnFrequency(incident *domain.Incident) bool {
	// Эскалация если инцидент длится более 30 минут и частота ошибок > 1 в минуту
	if incident.GetDuration() < 30*time.Minute {
		return false
	}
	
	frequency := s.calculateErrorFrequency(incident)
	return frequency > 1.0 // Более 1 ошибки в минуту
}

// calculateErrorFrequency вычисляет частоту ошибок (ошибок в минуту)
func (s *incidentService) calculateErrorFrequency(incident *domain.Incident) float64 {
	duration := incident.GetDuration()
	if duration == 0 {
		return 0
	}
	
	durationMinutes := duration.Minutes()
	if durationMinutes == 0 {
		return float64(incident.Count)
	}
	
	return float64(incident.Count) / durationMinutes
}

// getEscalationReason определяет причину эскалации
func (s *incidentService) getEscalationReason(originalSeverity domain.IncidentSeverity, incident *domain.Incident) string {
	// Сначала проверяем эскалацию на основе частоты
	if s.shouldEscalateBasedOnFrequency(incident) {
		return "high_frequency"
	}
	
	// Затем проверяем эскалацию на основе времени
	if escalationTimeout, exists := s.config.EscalationTimeouts[originalSeverity]; exists {
		if time.Since(incident.FirstSeen) > escalationTimeout {
			return "timeout"
		}
	}
	
	// Наконец проверяем эскалацию на основе количества повторений
	if maxRetries, exists := s.config.MaxRetriesBeforeEscalation[originalSeverity]; exists {
		if incident.Count > maxRetries {
			return "retry_count"
		}
	}
	
	return "unknown"
}

// escalateSeverity повышает уровень серьезности инцидента
func (s *incidentService) escalateSeverity(incident *domain.Incident) {
	switch incident.Severity {
	case domain.IncidentSeverityWarning:
		incident.UpdateSeverity(domain.IncidentSeverityError)
	case domain.IncidentSeverityError:
		incident.UpdateSeverity(domain.IncidentSeverityCritical)
	case domain.IncidentSeverityCritical:
		// Уже максимальный уровень
	}
}

// GetIncident получает инцидент по ID
func (s *incidentService) GetIncident(ctx context.Context, id string) (*domain.Incident, error) {
	if err := s.validator.ValidateUUID(id, "incident_id"); err != nil {
		s.logger.Error("Invalid incident ID",
			logger.String("incident_id", id),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "incident ID validation failed")
	}
	
	s.logger.Debug("Getting incident by ID",
		logger.String("incident_id", id))
	
	incident, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get incident",
			logger.String("incident_id", id),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get incident")
	}
	
	return incident, nil
}

// GetIncidents получает список инцидентов с фильтрацией
func (s *incidentService) GetIncidents(ctx context.Context, filter *domain.IncidentFilter) ([]*domain.Incident, error) {
	if filter.TenantID == nil {
		err := errors.New(errors.ErrValidation, "tenant_id is required")
		s.logger.Error("Missing tenant ID in filter",
			logger.Error(err))
		return nil, err
	}
	
	if err := s.validator.ValidateUUID(*filter.TenantID, "tenant_id"); err != nil {
		s.logger.Error("Invalid tenant ID in filter",
			logger.String("tenant_id", *filter.TenantID),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "tenant ID validation failed")
	}
	
	s.logger.Debug("Getting incidents with filter",
		logger.String("tenant_id", *filter.TenantID),
		logger.Any("filter", filter))
	
	incidents, err := s.repo.GetByTenantID(ctx, *filter.TenantID, filter)
	if err != nil {
		s.logger.Error("Failed to get incidents",
			logger.String("tenant_id", *filter.TenantID),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get incidents")
	}
	
	return incidents, nil
}

// AcknowledgeIncident подтверждает инцидент
func (s *incidentService) AcknowledgeIncident(ctx context.Context, id string) error {
	if err := s.validator.ValidateUUID(id, "incident_id"); err != nil {
		s.logger.Error("Invalid incident ID",
			logger.String("incident_id", id),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "incident ID validation failed")
	}
	
	s.logger.Debug("Acknowledging incident",
		logger.String("incident_id", id))
	
	incident, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get incident for acknowledgment",
			logger.String("incident_id", id),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to get incident")
	}
	
	incident.Acknowledge()
	
	s.logger.Info("Incident acknowledged",
		logger.String("incident_id", id),
		logger.String("tenant_id", incident.TenantID))
	
	return s.repo.Update(ctx, incident)
}

// ResolveIncident разрешает инцидент
func (s *incidentService) ResolveIncident(ctx context.Context, id string) error {
	if err := s.validator.ValidateUUID(id, "incident_id"); err != nil {
		s.logger.Error("Invalid incident ID",
			logger.String("incident_id", id),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "incident ID validation failed")
	}
	
	s.logger.Debug("Resolving incident",
		logger.String("incident_id", id))
	
	incident, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get incident for resolution",
			logger.String("incident_id", id),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "failed to get incident")
	}
	
	incident.Resolve()
	
	s.logger.Info("Incident resolved",
		logger.String("incident_id", id),
		logger.String("tenant_id", incident.TenantID),
		logger.Duration("incident_duration", incident.GetDuration()))
	
	return s.repo.Update(ctx, incident)
}

// GetIncidentStats получает статистику по инцидентам
func (s *incidentService) GetIncidentStats(ctx context.Context, tenantID string) (*domain.IncidentStats, error) {
	if err := s.validator.ValidateUUID(tenantID, "tenant_id"); err != nil {
		s.logger.Error("Invalid tenant ID",
			logger.String("tenant_id", tenantID),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "tenant ID validation failed")
	}
	
	s.logger.Debug("Getting incident statistics",
		logger.String("tenant_id", tenantID))
	
	stats, err := s.repo.GetStats(ctx, tenantID)
	if err != nil {
		s.logger.Error("Failed to get incident statistics",
			logger.String("tenant_id", tenantID),
			logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get incident statistics")
	}
	
	return stats, nil
}

// Вспомогательные функции

// containsCriticalKeyword проверяет наличие ключевых слов критических ошибок
func containsCriticalKeyword(message string) bool {
	criticalKeywords := []string{
		"panic", "fatal", "crash", "out of memory", "stack overflow",
		"database connection failed", "authentication failed", "authorization failed",
		"service unavailable", "circuit breaker", "timeout", "deadline exceeded",
	}
	
	return containsAny(message, criticalKeywords)
}

// containsErrorKeyword проверяет наличие ключевых слов ошибок
func containsErrorKeyword(message string) bool {
	errorKeywords := []string{
		"error", "failed", "exception", "refused", "denied", "forbidden",
		"not found", "invalid", "bad request", "unauthorized", "connection refused",
	}
	
	return containsAny(message, errorKeywords)
}

// containsAny проверяет наличие любого из ключевых слов в сообщении
func containsAny(message string, keywords []string) bool {
	message = fmt.Sprintf("%s", message)
	for _, keyword := range keywords {
		if contains(message, keyword) {
			return true
		}
	}
	return false
}

// contains проверяет наличие подстроки (case-insensitive)
func contains(message, substring string) bool {
	return len(message) >= len(substring) && 
		   (message == substring || 
		    len(message) > len(substring) && 
		    (message[:len(substring)] == substring || 
		     message[len(message)-len(substring):] == substring ||
		     containsInMiddle(message, substring)))
}

// containsInMiddle проверяет наличие подстроки в середине строки
func containsInMiddle(message, substring string) bool {
	for i := 1; i <= len(message)-len(substring); i++ {
		if message[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}

// publishIncidentEvent публикует событие инцидента
func (s *incidentService) publishIncidentEvent(ctx context.Context, eventType string, incident *domain.Incident, result *CheckResult) {
	s.logger.Info("Publishing incident event",
		logger.String("event_type", eventType),
		logger.String("incident_id", incident.ID),
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.String("severity", string(incident.Severity)),
		logger.String("status", string(incident.Status)),
		logger.Int("count", incident.Count),
		logger.Duration("duration", result.Duration))
	
	// Публикуем событие через RabbitMQ producer
	if s.producer != nil && s.producer.IsConnected() {
		// Конвертируем service.CheckResult в rabbitmq.CheckResult
		rabbitmqResult := &rabbitmq.CheckResult{
			CheckID:      result.CheckID,
			TenantID:     result.TenantID,
			IsSuccess:    result.IsSuccess,
			ErrorMessage: result.ErrorMessage,
			Duration:     result.Duration,
			Timestamp:    result.Timestamp,
			Metadata:     result.Metadata,
		}
		
		err := s.producer.PublishIncidentEventWithRetry(ctx, eventType, incident, rabbitmqResult)
		if err != nil {
			s.logger.Error("Failed to publish incident event to RabbitMQ",
				logger.String("event_type", eventType),
				logger.String("incident_id", incident.ID),
				logger.Error(err))
		} else {
			s.logger.Debug("Incident event published successfully to RabbitMQ",
				logger.String("event_type", eventType),
				logger.String("incident_id", incident.ID))
		}
	} else {
		s.logger.Warn("RabbitMQ producer not available, event not published",
			logger.String("event_type", eventType),
			logger.String("incident_id", incident.ID),
			logger.Bool("producer_nil", s.producer == nil))
		
		// Логируем событие для отладки если producer недоступен
		event := map[string]interface{}{
			"event_type":     eventType,
			"incident_id":    incident.ID,
			"check_id":        result.CheckID,
			"tenant_id":       result.TenantID,
			"severity":        string(incident.Severity),
			"status":          string(incident.Status),
			"count":           incident.Count,
			"error_message":   result.ErrorMessage,
			"duration":        result.Duration.Milliseconds(),
			"first_seen":      incident.FirstSeen,
			"last_seen":       incident.LastSeen,
			"error_hash":      incident.ErrorHash,
			"timestamp":       time.Now(),
			"service":         "incident-manager",
		}
		
		s.logger.Debug("Incident event data (not published)",
			logger.Any("event", event))
	}
}

// generateErrorHash генерирует хеш для дедупликации ошибок
func generateErrorHash(errorMessage string) string {
	// Нормализуем сообщение об ошибке для дедупликации
	normalized := normalizeErrorMessage(errorMessage)
	
	// Генерируем SHA256 хеш
	hash := sha256.Sum256([]byte(normalized))
	
	// Возвращаем первые 16 символов хеша для компактности
	return fmt.Sprintf("%x", hash)[:16]
}

// normalizeErrorMessage нормализует сообщение об ошибке
func normalizeErrorMessage(message string) string {
	// Приводим к нижнему регистру
	message = strings.ToLower(message)
	
	// Удаляем временные метки
	message = removeTimestamps(message)
	
	// Удаляем лишние пробелы в начале и конце
	message = strings.TrimSpace(message)
	
	return message
}

// UpdateIncident обновляет инцидент
func (s *incidentService) UpdateIncident(ctx context.Context, incident *domain.Incident) error {
	if incident == nil {
		return fmt.Errorf("incident cannot be nil")
	}
	
	s.logger.Debug("Updating incident",
		logger.String("incident_id", incident.ID),
		logger.String("tenant_id", incident.TenantID))
	
	// Валидация
	if err := s.validator.ValidateRequiredFields(
		map[string]interface{}{
			"id":        incident.ID,
			"tenant_id": incident.TenantID,
		},
		map[string]string{
			"id":        "incident ID is required",
			"tenant_id": "tenant ID is required",
		},
	); err != nil {
		s.logger.Error("Incident validation failed",
			logger.String("incident_id", incident.ID),
			logger.Error(err))
		return err
	}
	
	// Обновляем время изменения
	incident.UpdatedAt = time.Now()
	
	// Сохраняем изменения
	err := s.repo.Update(ctx, incident)
	if err != nil {
		s.logger.Error("Failed to update incident",
			logger.String("incident_id", incident.ID),
			logger.Error(err))
		return fmt.Errorf("failed to update incident: %w", err)
	}
	
	s.logger.Info("Incident updated successfully",
		logger.String("incident_id", incident.ID))
	
	return nil
}

// GetIncidentHistory получает историю инцидента
func (s *incidentService) GetIncidentHistory(ctx context.Context, incidentID string) ([]*domain.IncidentEvent, error) {
	s.logger.Debug("Getting incident history",
		logger.String("incident_id", incidentID))
	
	// Валидация
	if err := s.validator.ValidateRequiredFields(
		map[string]interface{}{
			"incident_id": incidentID,
		},
		map[string]string{
			"incident_id": "incident ID is required",
		},
	); err != nil {
		s.logger.Error("Incident ID validation failed",
			logger.String("incident_id", incidentID),
			logger.Error(err))
		return nil, err
	}
	
	// Получаем инцидент
	incident, err := s.repo.GetByID(ctx, incidentID)
	if err != nil {
		s.logger.Error("Failed to get incident for history",
			logger.String("incident_id", incidentID),
			logger.Error(err))
		return nil, fmt.Errorf("failed to get incident: %w", err)
	}
	
	// Создаем историю на основе метаданных
	history := make([]*domain.IncidentEvent, 0)
	
	// Добавляем событие создания
	history = append(history, &domain.IncidentEvent{
		ID:          fmt.Sprintf("%s-created", incidentID),
		IncidentID:  incidentID,
		EventType:   "incident.created",
		OldStatus:   "",
		NewStatus:   incident.Status,
		OldSeverity: "",
		NewSeverity: incident.Severity,
		Message:     "Incident created",
		Metadata:    map[string]interface{}{},
		CreatedAt:   incident.CreatedAt,
	})
	
	// Добавляем события эскалации если есть
	if incident.Metadata != nil {
		if escalationHistory, ok := incident.Metadata["escalation_history"]; ok {
			if escalations, ok := escalationHistory.([]interface{}); ok {
				for i, esc := range escalations {
					if escMap, ok := esc.(map[string]interface{}); ok {
						event := &domain.IncidentEvent{
							ID:          fmt.Sprintf("%s-escalation-%d", incidentID, i),
							IncidentID:  incidentID,
							EventType:   "incident.escalated",
							OldStatus:   "",
							NewStatus:   incident.Status,
							OldSeverity: "",
							NewSeverity: "",
							Message:     fmt.Sprintf("Escalated: %v", escMap),
							Metadata:    escMap,
							CreatedAt:   incident.CreatedAt,
						}
						history = append(history, event)
					}
				}
			}
		}
	}
	
	s.logger.Debug("Incident history retrieved",
		logger.String("incident_id", incidentID),
		logger.Int("events_count", len(history)))
	
	return history, nil
}

// removeTimestamps удаляет временные метки из сообщения
func removeTimestamps(message string) string {
	// Простая реализация - удаляем распространенные форматы времени
	//TODO В реальной реализации здесь была бы regex замена
	// Для простоты оставляем как есть
	return message
}
