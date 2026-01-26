package mocks

import (
	"context"
	"github.com/stretchr/testify/mock"

	"UptimePingPlatform/services/scheduler-service/internal/domain"
)

// ProducerInterface интерфейс для продюсера сообщений
type ProducerInterface interface {
	PublishTask(ctx context.Context, task *domain.Task) error
	Close() error
}

// MockProducer - мок для ProducerInterface
type MockProducer struct {
	mock.Mock
}

func (m *MockProducer) PublishTask(ctx context.Context, task *domain.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockProducer) Close() error {
	args := m.Called()
	return args.Error(0)
}
