package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockCheckRepository - универсальный мок для CheckRepository
type MockCheckRepository struct {
	mock.Mock
}

func (m *MockCheckRepository) Create(ctx context.Context, check interface{}) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockCheckRepository) GetByID(ctx context.Context, id string) (interface{}, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

func (m *MockCheckRepository) GetByTenantID(ctx context.Context, tenantID string) ([]interface{}, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockCheckRepository) Update(ctx context.Context, check interface{}) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockCheckRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCheckRepository) GetActiveChecks(ctx context.Context) ([]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockCheckRepository) GetActiveChecksByTenant(ctx context.Context, tenantID string) ([]interface{}, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]interface{}), args.Error(1)
}

// MockSchedulerRepository - универсальный мок для SchedulerRepository
type MockSchedulerRepository struct {
	mock.Mock
}

func (m *MockSchedulerRepository) AddCheck(ctx context.Context, check interface{}) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockSchedulerRepository) RemoveCheck(ctx context.Context, checkID string) error {
	args := m.Called(ctx, checkID)
	return args.Error(0)
}

func (m *MockSchedulerRepository) UpdateCheck(ctx context.Context, check interface{}) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockSchedulerRepository) GetScheduledChecks(ctx context.Context) ([]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).([]interface{}), args.Error(1)
}

// Методы для работы с расписаниями
func (m *MockSchedulerRepository) CreateSchedule(ctx context.Context, schedule interface{}) error {
	args := m.Called(ctx, schedule)
	return args.Error(0)
}

func (m *MockSchedulerRepository) GetScheduleByCheckID(ctx context.Context, checkID string) (interface{}, error) {
	args := m.Called(ctx, checkID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

func (m *MockSchedulerRepository) UpdateSchedule(ctx context.Context, schedule interface{}) error {
	args := m.Called(ctx, schedule)
	return args.Error(0)
}

func (m *MockSchedulerRepository) DeleteSchedule(ctx context.Context, checkID string) error {
	args := m.Called(ctx, checkID)
	return args.Error(0)
}

func (m *MockSchedulerRepository) GetAllSchedules(ctx context.Context) ([]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockSchedulerRepository) GetActiveSchedules(ctx context.Context) ([]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).([]interface{}), args.Error(1)
}
