package mocks

import (
	"github.com/stretchr/testify/mock"

	"UptimePingPlatform/pkg/logger"
)

// MockLogger - универсальный мок для logger.Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {
	args := m.Called(msg, fields)
	if len(args) > 0 {
		// Можно добавить обработку возвращаемых значений если нужно
	}
}

func (m *MockLogger) Info(msg string, fields ...logger.Field) {
	args := m.Called(msg, fields)
	if len(args) > 0 {
		// Можно добавить обработку возвращаемых значений если нужно
	}
}

func (m *MockLogger) Warn(msg string, fields ...logger.Field) {
	args := m.Called(msg, fields)
	if len(args) > 0 {
		// Можно добавить обработку возвращаемых значений если нужно
	}
}

func (m *MockLogger) Error(msg string, fields ...logger.Field) {
	args := m.Called(msg, fields)
	if len(args) > 0 {
		// Можно добавить обработку возвращаемых значений если нужно
	}
}

func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	args := m.Called(fields)
	if len(args) > 0 {
		return args.Get(0).(logger.Logger)
	}
	return m // Возвращаем себя если нет мока
}

func (m *MockLogger) Sync() error {
	args := m.Called()
	if len(args) > 0 {
		return args.Error(0)
	}
	return nil
}
