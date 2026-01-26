package mocks

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"UptimePingPlatform/services/scheduler-service/internal/domain"
)

// MockCheckRepository - универсальный мок для CheckRepository
type MockCheckRepository struct {
	mock.Mock
}

func (m *MockCheckRepository) Create(ctx context.Context, check *domain.Check) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockCheckRepository) GetByID(ctx context.Context, id string) (*domain.Check, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Check), args.Error(1)
}

func (m *MockCheckRepository) GetByTenantID(ctx context.Context, tenantID string) ([]*domain.Check, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*domain.Check), args.Error(1)
}

func (m *MockCheckRepository) Update(ctx context.Context, check *domain.Check) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockCheckRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCheckRepository) GetActiveChecks(ctx context.Context) ([]*domain.Check, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Check), args.Error(1)
}

func (m *MockCheckRepository) GetActiveChecksByTenant(ctx context.Context, tenantID string) ([]*domain.Check, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*domain.Check), args.Error(1)
}

func (m *MockCheckRepository) Ping(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

// MockTaskRepository - универсальный мок для TaskRepository
type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) CreateTask(ctx context.Context, task *domain.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) GetTaskByID(ctx context.Context, taskID string) (*domain.Task, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Task), args.Error(1)
}

func (m *MockTaskRepository) UpdateTaskStatus(ctx context.Context, taskID string, status domain.TaskStatus) error {
	args := m.Called(ctx, taskID, status)
	return args.Error(0)
}

func (m *MockTaskRepository) SaveTaskResult(ctx context.Context, result *domain.TaskResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *MockTaskRepository) GetPendingTasks(ctx context.Context, limit int) ([]*domain.Task, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*domain.Task), args.Error(1)
}

// MockLockRepository - универсальный мок для LockRepository
type MockLockRepository struct {
	mock.Mock
}

func (m *MockLockRepository) TryLock(ctx context.Context, checkID, workerID string, ttl time.Duration) (*domain.LockInfo, error) {
	args := m.Called(ctx, checkID, workerID, ttl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LockInfo), args.Error(1)
}

func (m *MockLockRepository) ReleaseLock(ctx context.Context, checkID, workerID string) error {
	args := m.Called(ctx, checkID, workerID)
	return args.Error(0)
}

func (m *MockLockRepository) IsLocked(ctx context.Context, checkID string) (bool, error) {
	args := m.Called(ctx, checkID)
	return args.Bool(0), args.Error(1)
}

func (m *MockLockRepository) GetLockInfo(ctx context.Context, checkID string) (*domain.LockInfo, error) {
	args := m.Called(ctx, checkID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LockInfo), args.Error(1)
}

// MockSchedulerRepository - универсальный мок для SchedulerRepository
type MockSchedulerRepository struct {
	mock.Mock
}

func (m *MockSchedulerRepository) AddCheck(ctx context.Context, check *domain.Check) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockSchedulerRepository) RemoveCheck(ctx context.Context, checkID string) error {
	args := m.Called(ctx, checkID)
	return args.Error(0)
}

func (m *MockSchedulerRepository) UpdateCheck(ctx context.Context, check *domain.Check) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockSchedulerRepository) GetScheduledChecks(ctx context.Context) ([]*domain.Check, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Check), args.Error(1)
}

func (m *MockSchedulerRepository) Create(ctx context.Context, schedule *domain.Schedule) (*domain.Schedule, error) {
	args := m.Called(ctx, schedule)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Schedule), args.Error(1)
}

func (m *MockSchedulerRepository) DeleteByCheckID(ctx context.Context, checkID string) error {
	args := m.Called(ctx, checkID)
	return args.Error(0)
}

func (m *MockSchedulerRepository) GetByCheckID(ctx context.Context, checkID string) (*domain.Schedule, error) {
	args := m.Called(ctx, checkID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Schedule), args.Error(1)
}

func (m *MockSchedulerRepository) List(ctx context.Context, pageSize int, pageToken string, filter string) ([]*domain.Schedule, error) {
	args := m.Called(ctx, pageSize, pageToken, filter)
	return args.Get(0).([]*domain.Schedule), args.Error(1)
}

func (m *MockSchedulerRepository) Count(ctx context.Context, filter string) (int, error) {
	args := m.Called(ctx, filter)
	return args.Int(0), args.Error(1)
}

func (m *MockSchedulerRepository) Ping(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

// Методы для работы с расписаниями
func (m *MockSchedulerRepository) CreateSchedule(ctx context.Context, schedule *domain.Schedule) error {
	args := m.Called(ctx, schedule)
	return args.Error(0)
}

func (m *MockSchedulerRepository) GetScheduleByCheckID(ctx context.Context, checkID string) (*domain.Schedule, error) {
	args := m.Called(ctx, checkID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Schedule), args.Error(1)
}

func (m *MockSchedulerRepository) UpdateSchedule(ctx context.Context, schedule *domain.Schedule) error {
	args := m.Called(ctx, schedule)
	return args.Error(0)
}

func (m *MockSchedulerRepository) DeleteSchedule(ctx context.Context, checkID string) error {
	args := m.Called(ctx, checkID)
	return args.Error(0)
}

func (m *MockSchedulerRepository) GetAllSchedules(ctx context.Context) ([]*domain.Schedule, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Schedule), args.Error(1)
}

func (m *MockSchedulerRepository) GetActiveSchedules(ctx context.Context) ([]*domain.Schedule, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Schedule), args.Error(1)
}
