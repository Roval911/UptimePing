package rabbitmq

import (
	"context"

	"UptimePingPlatform/services/incident-manager/internal/domain"
)

// IncidentProducerInterface определяет интерфейс для producer событий инцидентов
type IncidentProducerInterface interface {
	// PublishIncidentEvent публикует событие инцидента
	PublishIncidentEvent(ctx context.Context, eventType string, incident *domain.Incident, result *CheckResult) error
	
	// PublishIncidentEventWithRetry публикует событие с retry логикой
	PublishIncidentEventWithRetry(ctx context.Context, eventType string, incident *domain.Incident, result *CheckResult) error
	
	// Close закрывает producer
	Close() error
	
	// IsConnected проверяет состояние подключения
	IsConnected() bool
}
