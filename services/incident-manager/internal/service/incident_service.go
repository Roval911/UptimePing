package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"UptimePingPlatform/pkg/errors"
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	pkg_redis "UptimePingPlatform/pkg/redis"
	"UptimePingPlatform/services/incident-manager/internal/domain"
	"UptimePingPlatform/services/incident-manager/internal/producer/rabbitmq"
	logger "UptimePingPlatform/pkg/logger"
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

	// CreateIncident создает новый инцидент
	CreateIncident(ctx context.Context, incident *domain.Incident) error

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
			domain.IncidentSeverityLow:      30 * time.Minute,
			domain.IncidentSeverityMedium:   15 * time.Minute,
			domain.IncidentSeverityCritical: 5 * time.Minute,
		},
		MaxRetriesBeforeEscalation: map[domain.IncidentSeverity]int{
			domain.IncidentSeverityLow:      10,
			domain.IncidentSeverityMedium:   5,
			domain.IncidentSeverityCritical: 2,
		},
		AutoResolveTimeout: 10 * time.Minute,
		IncidentTTL:        7 * 24 * time.Hour, // 7 дней
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
	if time.Since(activeIncident.UpdatedAt) < s.config.AutoResolveTimeout {
		// Слишком рано для автоматического разрешения
		s.logger.Debug("Too early for auto-resolve",
			logger.String("incident_id", activeIncident.ID),
			logger.String("check_id", result.CheckID),
			logger.Duration("time_since_last_seen", time.Since(activeIncident.UpdatedAt)),
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
	// Обновление времени последнего появления
	incident.UpdateSeverity(severity)

	// Проверяем необходимость эскалации при длительных инцидентах
	s.checkEscalation(incident)

	// Если инцидент был разрешен, повторно открываем его
	if incident.IsResolved() {
		incident.Reopen()
		s.logger.Info("Reopening resolved incident",
			logger.String("incident_id", incident.ID),
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", result.TenantID))
	}

	s.logger.Debug("Updating existing incident",
		logger.String("incident_id", incident.ID),
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
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
	incident.UpdateSeverity(severity)

	// Добавляем информацию о группировке в лог
	s.logger.Debug("Grouping with similar incident",
		logger.String("incident_id", incident.ID),
		logger.String("check_id", result.CheckID))

	s.logger.Info("Grouping with similar incident",
		logger.String("incident_id", incident.ID),
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.String("error_message", result.ErrorMessage))

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
	// Создание нового инцидент
	title := fmt.Sprintf("Check failed: %s", result.CheckID)
	description := result.ErrorMessage
	newIncident := domain.NewIncident(result.CheckID, severity, title, description)

	s.logger.Info("Creating new incident",
		logger.String("incident_id", newIncident.ID),
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", result.TenantID),
		logger.String("severity", string(severity)),
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
	if time.Since(incident.UpdatedAt) < s.config.AutoResolveTimeout {
		// Слишком рано для автоматического разрешения
		s.logger.Debug("Too early for auto-resolve",
			logger.String("incident_id", incident.ID),
			logger.String("check_id", result.CheckID),
			logger.Duration("time_since_last_seen", time.Since(incident.UpdatedAt)),
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
	title := fmt.Sprintf("Check failed: %s", result.CheckID)
	description := result.ErrorMessage
	newIncident = domain.NewIncident(result.CheckID, severity, title, description)

	// Ищем существующий инцидент по check_id
	existingIncident, err := s.repo.GetByCheckAndErrorHash(ctx, result.CheckID, "")
	if err != nil {
		s.logger.Error("Failed to find existing incident",
			logger.String("check_id", result.CheckID),
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
	title = fmt.Sprintf("Check failed: %s", result.CheckID)
	description = result.ErrorMessage
	createdIncident := domain.NewIncident(result.CheckID, severity, title, description)
	newIncident = createdIncident

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
		return domain.IncidentSeverityHigh
	}

	// Ошибки на основе сообщения
	if containsErrorKeyword(errorMessage) {
		return domain.IncidentSeverityHigh
	}

	// По умолчанию - низкий уровень
	return domain.IncidentSeverityLow
}

// checkEscalation проверяет необходимость эскалации инцидента
func (s *incidentService) checkEscalation(incident *domain.Incident) {
	originalSeverity := incident.Severity
	escalated := false

	// Этап 1: Проверяем эскалацию на основе времени существования
	if escalationTimeout, exists := s.config.EscalationTimeouts[incident.Severity]; exists {
		if time.Since(incident.StartedAt) > escalationTimeout {
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

	// Этап 2: Проверяем эскалацию на основе времени существования
	if !escalated {
		// Эскалация только на основе времени, так как поле Count отсутствует
		s.logger.Debug("No escalation based on count - using time-based escalation only",
			logger.String("incident_id", incident.ID))
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

	// Этап 4: Логируем эскалацию
	if escalated {
		s.logger.Info("Incident escalated",
			logger.String("incident_id", incident.ID),
			logger.String("from_severity", string(originalSeverity)),
			logger.String("to_severity", string(incident.Severity)))
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
		return 1.0 // Одна ошибка если длительность очень маленькая
	}

	return 1.0 / durationMinutes // Одна ошибка за весь период
}

// getEscalationReason определяет причину эскалации
func (s *incidentService) getEscalationReason(originalSeverity domain.IncidentSeverity, incident *domain.Incident) string {
	// Сначала проверяем эскалацию на основе частоты
	if s.shouldEscalateBasedOnFrequency(incident) {
		return "high_frequency"
	}

	// Затем проверяем эскалацию на основе времени
	if escalationTimeout, exists := s.config.EscalationTimeouts[originalSeverity]; exists {
		if time.Since(incident.StartedAt) > escalationTimeout {
			return "timeout"
		}
	}

	// Наконец проверяем эскалацию на основе времени существования
	if time.Since(incident.StartedAt) > time.Hour {
		return "long_duration"
	}

	return "unknown"
}

// escalateSeverity повышает уровень серьезности инцидента
func (s *incidentService) escalateSeverity(incident *domain.Incident) {
	switch incident.Severity {
	case domain.IncidentSeverityLow:
		incident.UpdateSeverity(domain.IncidentSeverityMedium)
	case domain.IncidentSeverityMedium:
		incident.UpdateSeverity(domain.IncidentSeverityHigh)
	case domain.IncidentSeverityHigh:
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
		logger.String("incident_id", id))

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
		logger.String("severity", string(incident.Severity)),
		logger.String("status", string(incident.Status)),
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
			"event_type":    eventType,
			"incident_id":   incident.ID,
			"check_id":      result.CheckID,
			"severity":      string(incident.Severity),
			"status":        string(incident.Status),
			"error_message": result.ErrorMessage,
			"duration":      result.Duration.Milliseconds(),
			"started_at":    incident.StartedAt,
			"resolved_at":   incident.ResolvedAt,
			"timestamp":     time.Now(),
			"service":       "incident-manager",
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
		logger.String("incident_id", incident.ID))

	// Валидация
	if err := s.validator.ValidateRequiredFields(
		map[string]interface{}{
			"id": incident.ID,
		},
		map[string]string{
			"id": "incident ID is required",
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

	s.logger.Debug("Incident history retrieved",
		logger.String("incident_id", incidentID),
		logger.Int("events_count", len(history)))

	return history, nil
}

// removeTimestamps удаляет временные метки из сообщения
func removeTimestamps(message string) string {
	// Regex паттерны для различных форматов временных меток
	timestampPatterns := []*regexp.Regexp{
		// ISO 8601: 2023-12-25T14:30:45Z, 2023-12-25T14:30:45+03:00
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:\d{2})`),
		// RFC 3339: 2023-12-25 14:30:45, 2023-12-25 14:30:45+03:00
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\s*[+-]\d{2}:\d{2})?`),
		// Дата и время: 25/12/2023 14:30:45, 12/25/2023 14:30:45
		regexp.MustCompile(`\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2}`),
		// Время: 14:30:45, 14:30
		regexp.MustCompile(`\d{1,2}:\d{2}(?::\d{2})?`),
		// Unix timestamp в миллисекундах: 1703506645123
		regexp.MustCompile(`\b\d{10,13}\b`),
		// Месяц день, год: Dec 25, 2023
		regexp.MustCompile(`\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2},?\s+\d{4}\b`),
		// День месяц: 25 Dec 2023
		regexp.MustCompile(`\b\d{1,2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{4}\b`),
	}

	result := message
	for _, pattern := range timestampPatterns {
		result = pattern.ReplaceAllString(result, "TIMESTAMP")
	}

	return result
}

// InMemoryIncidentService реализация хранилища в памяти для демонстрации
type InMemoryIncidentService struct {
	logger    pkglogger.Logger
	redis     pkg_redis.Client
	mu        sync.RWMutex
	incidents map[string]*domain.Incident
	events    map[string][]*domain.IncidentEvent
}

// NewInMemoryIncidentService создает новый сервис инцидентов в памяти
func NewInMemoryIncidentService(logger pkglogger.Logger, redis pkg_redis.Client) *InMemoryIncidentService {
	return &InMemoryIncidentService{
		logger:    logger,
		redis:     redis,
		incidents: make(map[string]*domain.Incident),
		events:    make(map[string][]*domain.IncidentEvent),
	}
}

// ProcessCheckResult обрабатывает результат проверки
func (s *InMemoryIncidentService) ProcessCheckResult(ctx context.Context, result *CheckResult) (*domain.Incident, error) {
	s.logger.Info("Processing check result",
		pkglogger.String("check_id", result.CheckID),
		pkglogger.Bool("is_success", result.IsSuccess))

	// Для демонстрации создаем инцидент если проверка неуспешна
	if !result.IsSuccess {
		incident := domain.NewIncident(
			result.CheckID,
			domain.IncidentSeverityHigh,
			fmt.Sprintf("Check %s failed", result.CheckID),
			result.ErrorMessage,
		)
		
		s.mu.Lock()
		s.incidents[incident.ID] = incident
		s.events[incident.ID] = []*domain.IncidentEvent{
			{
				ID:         fmt.Sprintf("event-%s", incident.ID),
				IncidentID: incident.ID,
				EventType:  "created",
				OldStatus:  "",
				NewStatus:  incident.Status,
				Message:    "Incident created automatically",
				CreatedAt:  time.Now(),
			},
		}
		s.mu.Unlock()

		s.logger.Info("Incident created",
			pkglogger.String("incident_id", incident.ID),
			pkglogger.String("severity", string(incident.Severity)))

		return incident, nil
	}

	return nil, nil
}

// ProcessCheckResultEvent обрабатывает результат проверки с публикацией событий
func (s *InMemoryIncidentService) ProcessCheckResultEvent(ctx context.Context, result *CheckResult) error {
	incident, err := s.ProcessCheckResult(ctx, result)
	if err != nil {
		return err
	}

	if incident != nil {
		// Здесь можно добавить публикацию в RabbitMQ
		s.logger.Info("Incident event published",
			pkglogger.String("incident_id", incident.ID))
	}

	return nil
}

// GetIncident получает инцидент по ID
func (s *InMemoryIncidentService) GetIncident(ctx context.Context, id string) (*domain.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	incident, exists := s.incidents[id]
	if !exists {
		return nil, errors.New(errors.ErrNotFound, fmt.Sprintf("incident %s not found", id))
	}

	return incident, nil
}

// GetIncidents получает список инцидентов с фильтрацией
func (s *InMemoryIncidentService) GetIncidents(ctx context.Context, filter *domain.IncidentFilter) ([]*domain.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var incidents []*domain.Incident
	for _, incident := range s.incidents {
		// Простая фильтрация по статусу
		if filter != nil && filter.Status != nil && string(incident.Status) != string(*filter.Status) {
			continue
		}
		incidents = append(incidents, incident)
	}

	return incidents, nil
}

// UpdateIncident обновляет инцидент (создает если не существует)
func (s *InMemoryIncidentService) UpdateIncident(ctx context.Context, incident *domain.Incident) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.incidents[incident.ID]; !exists {
		// Создаем новый инцидент если не существует
		incident.UpdatedAt = time.Now()
		s.incidents[incident.ID] = incident
		s.events[incident.ID] = []*domain.IncidentEvent{
			{
				ID:         fmt.Sprintf("event-%s", incident.ID),
				IncidentID: incident.ID,
				EventType:  "created",
				NewStatus:  incident.Status,
				Message:    "Incident created via API",
				CreatedAt:  time.Now(),
			},
		}

		s.logger.Info("Incident created",
			pkglogger.String("incident_id", incident.ID))

		return nil
	}

	incident.UpdatedAt = time.Now()
	s.incidents[incident.ID] = incident

	s.logger.Info("Incident updated",
		pkglogger.String("incident_id", incident.ID))

	return nil
}

// AcknowledgeIncident подтверждает инцидент
func (s *InMemoryIncidentService) AcknowledgeIncident(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	incident, exists := s.incidents[id]
	if !exists {
		return errors.New(errors.ErrNotFound, fmt.Sprintf("incident %s not found", id))
	}

	incident.Acknowledge()
	incident.UpdatedAt = time.Now()

	// Добавляем событие
	event := &domain.IncidentEvent{
		ID:         fmt.Sprintf("event-%s-ack", id),
		IncidentID: id,
		EventType:  "acknowledged",
		NewStatus:  incident.Status,
		Message:    "Incident acknowledged",
		CreatedAt:  time.Now(),
	}
	s.events[id] = append(s.events[id], event)

	s.logger.Info("Incident acknowledged",
		pkglogger.String("incident_id", id))

	return nil
}

// ResolveIncident разрешает инцидент
func (s *InMemoryIncidentService) ResolveIncident(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	incident, exists := s.incidents[id]
	if !exists {
		return errors.New(errors.ErrNotFound, fmt.Sprintf("incident %s not found", id))
	}

	incident.Resolve()
	incident.UpdatedAt = time.Now()

	// Добавляем событие
	event := &domain.IncidentEvent{
		ID:         fmt.Sprintf("event-%s-resolved", id),
		IncidentID: id,
		EventType:  "resolved",
		NewStatus:  incident.Status,
		Message:    "Incident resolved",
		CreatedAt:  time.Now(),
	}
	s.events[id] = append(s.events[id], event)

	s.logger.Info("Incident resolved",
		pkglogger.String("incident_id", id))

	return nil
}

// GetIncidentHistory получает историю инцидента
func (s *InMemoryIncidentService) GetIncidentHistory(ctx context.Context, incidentID string) ([]*domain.IncidentEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events, exists := s.events[incidentID]
	if !exists {
		return []*domain.IncidentEvent{}, nil
	}

	return events, nil
}

// GetIncidentStats получает статистику по инцидентам
func (s *InMemoryIncidentService) GetIncidentStats(ctx context.Context, tenantID string) (*domain.IncidentStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &domain.IncidentStats{
		Total:      len(s.incidents),
		ByStatus:   make(map[domain.IncidentStatus]int),
		BySeverity: make(map[domain.IncidentSeverity]int),
		Last24h:    0,
		Last7d:     0,
		Last30d:    0,
	}

	now := time.Now()
	for _, incident := range s.incidents {
		// Статистика по статусам
		stats.ByStatus[incident.Status]++
		
		// Статистика по серьезности
		stats.BySeverity[incident.Severity]++
		
		// Статистика по времени
		if now.Sub(incident.StartedAt) <= 24*time.Hour {
			stats.Last24h++
		}
		if now.Sub(incident.StartedAt) <= 7*24*time.Hour {
			stats.Last7d++
		}
		if now.Sub(incident.StartedAt) <= 30*24*time.Hour {
			stats.Last30d++
		}
	}

	return stats, nil
}
