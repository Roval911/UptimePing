package filter

import (
	"strings"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/domain"
)

// EventFilterInterface интерфейс фильтра событий
type EventFilterInterface interface {
	ShouldProcess(event *domain.Event) bool
	GetFilterStats() map[string]interface{}
}

// EventFilter фильтрует события
type EventFilter struct {
	config FilterConfig
	logger logger.Logger
}

// FilterConfig конфигурация фильтра
type FilterConfig struct {
	// Разрешенные типы событий
	AllowedEventTypes []string `json:"allowed_event_types" yaml:"allowed_event_types"`
	
	// Разрешенные уровни серьезности
	AllowedSeverities []string `json:"allowed_severities" yaml:"allowed_severities"`
	
	// Блокированные источники
	BlockedSources []string `json:"blocked_sources" yaml:"blocked_sources"`
	
	// Блокированные тенанты
	BlockedTenants []string `json:"blocked_tenants" yaml:"blocked_tenants"`
	
	// Фильтрация по времени (не обрабатывать события старше N минут)
	MaxEventAgeMinutes int `json:"max_event_age_minutes" yaml:"max_event_age_minutes"`
	
	// Включить фильтрацию
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// NewEventFilter создает новый фильтр событий
func NewEventFilter(config FilterConfig, logger logger.Logger) *EventFilter {
	return &EventFilter{
		config: config,
		logger: logger,
	}
}

// ShouldProcess определяет, нужно ли обрабатывать событие
func (f *EventFilter) ShouldProcess(event *domain.Event) bool {
	// Если фильтрация отключена, обрабатываем все события
	if !f.config.Enabled {
		return true
	}

	// Фильтрация по типу события
	if !f.isAllowedEventType(event.Type) {
		return false
	}

	// Фильтрация по уровню серьезности
	if !f.isAllowedSeverity(event.Severity) {
		return false
	}

	// Фильтрация по источнику
	if f.isBlockedSource(event.Source) {
		return false
	}

	// Фильтрация по тенанту
	if f.isBlockedTenant(event.TenantID) {
		return false
	}

	// Фильтрация по времени
	if !f.isEventFreshEnough(event) {
		f.logger.Debug("Event filtered out by age",
			logger.String("event_id", event.ID),
			logger.String("event_type", event.Type),
			logger.String("severity", event.Severity),
		)
		return false
	}

	return true
}

// isAllowedEventType проверяет, разрешен ли тип события
func (f *EventFilter) isAllowedEventType(eventType string) bool {
	if len(f.config.AllowedEventTypes) == 0 {
		return true // Если список пуст, разрешены все типы
	}

	for _, allowedType := range f.config.AllowedEventTypes {
		if allowedType == eventType {
			return true
		}
	}

	return false
}

// isAllowedSeverity проверяет, разрешен ли уровень серьезности
func (f *EventFilter) isAllowedSeverity(severity string) bool {
	if len(f.config.AllowedSeverities) == 0 {
		return true // Если список пуст, разрешены все уровни
	}

	for _, allowedSeverity := range f.config.AllowedSeverities {
		if allowedSeverity == severity {
			return true
		}
	}

	return false
}

// isBlockedSource проверяет, не заблокирован ли источник
func (f *EventFilter) isBlockedSource(source string) bool {
	for _, blockedSource := range f.config.BlockedSources {
		if blockedSource == source {
			return true
		}
		// Поддержка wildcard
		if strings.HasSuffix(blockedSource, "*") {
			prefix := strings.TrimSuffix(blockedSource, "*")
			if strings.HasPrefix(source, prefix) {
				return true
			}
		}
	}

	return false
}

// isBlockedTenant проверяет, не заблокирован ли тенант
func (f *EventFilter) isBlockedTenant(tenantID string) bool {
	for _, blockedTenant := range f.config.BlockedTenants {
		if blockedTenant == tenantID {
			return true
		}
	}

	return false
}

// isEventFreshEnough проверяет, достаточно ли свежее событие
func (f *EventFilter) isEventFreshEnough(event *domain.Event) bool {
	if f.config.MaxEventAgeMinutes <= 0 {
		return true // Если ограничение не установлено, все события считаются свежими
	}

	maxAge := time.Duration(f.config.MaxEventAgeMinutes) * time.Minute
	eventAge := time.Since(event.Timestamp)

	return eventAge <= maxAge
}

// GetFilterStats возвращает статистику фильтрации
func (f *EventFilter) GetFilterStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":                f.config.Enabled,
		"allowed_event_types":   f.config.AllowedEventTypes,
		"allowed_severities":    f.config.AllowedSeverities,
		"blocked_sources_count": len(f.config.BlockedSources),
		"blocked_tenants_count": len(f.config.BlockedTenants),
		"max_event_age_minutes": f.config.MaxEventAgeMinutes,
	}
}

// DefaultFilterConfig возвращает конфигурацию по умолчанию
func DefaultFilterConfig() FilterConfig {
	return FilterConfig{
		AllowedEventTypes: []string{
			domain.NotificationTypeIncidentCreated,
			domain.NotificationTypeIncidentUpdated,
			domain.NotificationTypeIncidentResolved,
			domain.NotificationTypeCheckFailed,
			domain.NotificationTypeCheckRecovered,
		},
		AllowedSeverities: []string{
			domain.SeverityMedium,
			domain.SeverityHigh,
			domain.SeverityCritical,
		},
		BlockedSources:      []string{},
		BlockedTenants:      []string{},
		MaxEventAgeMinutes:  60, // 1 час
		Enabled:             true,
	}
}

// ProductionFilterConfig возвращает конфигурацию для production
func ProductionFilterConfig() FilterConfig {
	return FilterConfig{
		AllowedEventTypes: []string{
			domain.NotificationTypeIncidentCreated,
			domain.NotificationTypeIncidentUpdated,
			domain.NotificationTypeIncidentResolved,
			domain.NotificationTypeCheckFailed,
			domain.NotificationTypeCheckRecovered,
		},
		AllowedSeverities: []string{
			domain.SeverityHigh,
			domain.SeverityCritical,
		},
		BlockedSources:      []string{"test-*", "debug-*"},
		BlockedTenants:      []string{},
		MaxEventAgeMinutes:  30, // 30 минут
		Enabled:             true,
	}
}

// DevelopmentFilterConfig возвращает конфигурацию для разработки
func DevelopmentFilterConfig() FilterConfig {
	return FilterConfig{
		AllowedEventTypes: []string{}, // Пусто = все типы разрешены
		AllowedSeverities: []string{}, // Пусто = все уровни разрешены
		BlockedSources:      []string{},
		BlockedTenants:      []string{},
		MaxEventAgeMinutes:  0, // Без ограничения по времени
		Enabled:             false, // Фильтрация отключена для разработки
	}
}
