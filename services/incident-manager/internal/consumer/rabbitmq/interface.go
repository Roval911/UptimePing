package rabbitmq

import (
	"context"
)

// IncidentConsumerInterface определяет интерфейс для consumer результатов проверок
type IncidentConsumerInterface interface {
	// Setup настраивает consumer
	Setup() error

	// Start начинает потребление сообщений
	Start(ctx context.Context) error

	// Close закрывает consumer
	Close() error
}
