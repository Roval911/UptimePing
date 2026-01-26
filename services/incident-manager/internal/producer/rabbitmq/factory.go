package rabbitmq

import (
	"context"
	"fmt"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/rabbitmq"
)

// IncidentProducerFactory создает producer событий инцидентов
type IncidentProducerFactory struct {
	logger logger.Logger
}

// NewIncidentProducerFactory создает новую factory
func NewIncidentProducerFactory(logger logger.Logger) *IncidentProducerFactory {
	return &IncidentProducerFactory{
		logger: logger,
	}
}

// CreateProducer создает новый producer событий инцидентов
func (f *IncidentProducerFactory) CreateProducer(config *IncidentProducerConfig) (IncidentProducerInterface, error) {
	// Валидируем конфигурацию
	if err := config.Validate(); err != nil {
		f.logger.Error("Invalid incident producer config",
			logger.Error(err))
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Создаем RabbitMQ connection
	rabbitConfig := &rabbitmq.Config{
		URL:        config.URL,
		Exchange:   config.Exchange,
		Queue:      "", // Producer не использует очередь
		DLX:        "",
		DLQ:        "",
		ReconnectInterval: config.ReconnectInterval,
		MaxRetries:        config.MaxRetries,
		PrefetchCount:     config.PrefetchCount,
		PrefetchSize:      config.PrefetchSize,
		Global:           config.Global,
	}

	conn, err := rabbitmq.Connect(context.Background(), rabbitConfig)
	if err != nil {
		f.logger.Error("Failed to create RabbitMQ connection",
			logger.String("url", config.URL),
			logger.Error(err))
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	// Создаем producer
	producer, err := NewIncidentProducer(conn, config, f.logger)
	if err != nil {
		f.logger.Error("Failed to create incident producer",
			logger.String("exchange", config.Exchange),
			logger.Error(err))
		conn.Close()
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	f.logger.Info("Incident producer created successfully",
		logger.String("exchange", config.Exchange),
		logger.String("url", config.URL))

	return producer, nil
}

// CreateProducerWithConnection создает producer используя существующее соединение
func (f *IncidentProducerFactory) CreateProducerWithConnection(conn *rabbitmq.Connection, config *IncidentProducerConfig) (IncidentProducerInterface, error) {
	// Валидируем конфигурацию
	if err := config.Validate(); err != nil {
		f.logger.Error("Invalid incident producer config",
			logger.Error(err))
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Проверяем состояние соединения
	if conn == nil || conn.Channel() == nil || conn.Channel().IsClosed() {
		f.logger.Error("RabbitMQ connection is not connected")
		return nil, ErrConnectionClosed
	}

	// Создаем producer
	producer, err := NewIncidentProducer(conn, config, f.logger)
	if err != nil {
		f.logger.Error("Failed to create incident producer",
			logger.String("exchange", config.Exchange),
			logger.Error(err))
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	f.logger.Info("Incident producer created successfully",
		logger.String("exchange", config.Exchange))

	return producer, nil
}
