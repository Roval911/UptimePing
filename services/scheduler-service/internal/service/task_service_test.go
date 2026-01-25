package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/scheduler-service/internal/domain"
)

// MockCheckRepository - мок для CheckRepository
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Check), args.Error(1)
}

func (m *MockCheckRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCheckRepository) GetActiveChecks(ctx context.Context) ([]*domain.Check, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Check), args.Error(1)
}

func (m *MockCheckRepository) GetActiveChecksByTenant(ctx context.Context, tenantID string) ([]*domain.Check, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Check), args.Error(1)
}

func (m *MockCheckRepository) Update(ctx context.Context, check *domain.Check) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

// MockTaskRepository - мок для TaskRepository
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Task), args.Error(1)
}

// MockLockRepository - мок для LockRepository
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

// MockSchedulerRepository - мок для SchedulerRepository
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

func (m *MockSchedulerRepository) GetScheduledChecks(ctx context.Context) ([]*domain.Check, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Check), args.Error(1)
}

func (m *MockSchedulerRepository) UpdateCheck(ctx context.Context, check *domain.Check) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

// setupTestTaskService создает тестовый TaskService
func setupTestTaskService() (*TaskService, *MockCheckRepository, *MockTaskRepository, *MockLockRepository, *MockSchedulerRepository, *logger.MockLogger) {
	mockCheckRepo := &MockCheckRepository{}
	mockTaskRepo := &MockTaskRepository{}
	mockLockRepo := &MockLockRepository{}
	mockSchedulerRepo := &MockSchedulerRepository{}
	mockLogger := &logger.MockLogger{}

	taskService := NewTaskService(mockCheckRepo, mockTaskRepo, mockLockRepo, mockSchedulerRepo, nil, mockLogger)

	return taskService, mockCheckRepo, mockTaskRepo, mockLockRepo, mockSchedulerRepo, mockLogger
}

func TestTaskService_ExecuteCronTask_Success(t *testing.T) {
	ctx := context.Background()
	taskService, mockCheckRepo, mockTaskRepo, mockLockRepo, mockSchedulerRepo, mockLogger := setupTestTaskService()

	checkID := "test-check-123"
	now := time.Now()
	
	check := &domain.Check{
		ID:       checkID,
		TenantID: "tenant-123",
		Name:     "Test Check",
		Status:   domain.CheckStatusActive,
		Priority: domain.PriorityNormal,
		NextRunAt: &now,
	}

	lockInfo := &domain.LockInfo{
		CheckID:   checkID,
		WorkerID:  taskService.workerID,
		LockedAt:  now,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	// Настройка моков
	mockLockRepo.On("TryLock", ctx, checkID, taskService.workerID, 5*time.Minute).Return(lockInfo, nil)
	mockCheckRepo.On("GetByID", ctx, checkID).Return(check, nil)
	mockTaskRepo.On("CreateTask", ctx, mock.AnythingOfType("*domain.Task")).Return(nil)
	mockCheckRepo.On("Update", ctx, mock.AnythingOfType("*domain.Check")).Return(nil)
	mockSchedulerRepo.On("UpdateCheck", ctx, mock.AnythingOfType("*domain.Check")).Return(nil)
	mockLockRepo.On("ReleaseLock", ctx, checkID, taskService.workerID).Return(nil)
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything)

	// Act
	err := taskService.ExecuteCronTask(ctx, checkID)

	// Assert
	assert.NoError(t, err)
	mockLockRepo.AssertExpectations(t)
	mockCheckRepo.AssertExpectations(t)
	mockTaskRepo.AssertExpectations(t)
	mockSchedulerRepo.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTaskService_ExecuteCronTask_LockFailed(t *testing.T) {
	ctx := context.Background()
	taskService, _, _, mockLockRepo, _, mockLogger := setupTestTaskService()

	checkID := "test-check-123"

	// Настройка моков - блокировка уже занята
	lockError := errors.New(errors.ErrConflict, "lock already acquired")
	mockLockRepo.On("TryLock", ctx, checkID, taskService.workerID, 5*time.Minute).Return(nil, lockError)
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything)

	// Act
	err := taskService.ExecuteCronTask(ctx, checkID)

	// Assert
	assert.NoError(t, err) // Не ошибка, просто пропускаем
	mockLockRepo.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTaskService_ExecuteCronTask_CheckNotFound(t *testing.T) {
	ctx := context.Background()
	taskService, mockCheckRepo, _, mockLockRepo, _, mockLogger := setupTestTaskService()

	checkID := "test-check-123"
	now := time.Now()
	
	lockInfo := &domain.LockInfo{
		CheckID:   checkID,
		WorkerID:  taskService.workerID,
		LockedAt:  now,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	// Настройка моков
	mockLockRepo.On("TryLock", ctx, checkID, taskService.workerID, 5*time.Minute).Return(lockInfo, nil)
	mockCheckRepo.On("GetByID", ctx, checkID).Return(nil, assert.AnError)
	mockLockRepo.On("ReleaseLock", ctx, checkID, taskService.workerID).Return(nil)
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything)

	// Act
	err := taskService.ExecuteCronTask(ctx, checkID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get check configuration")
	mockLockRepo.AssertExpectations(t)
	mockCheckRepo.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTaskService_LoadActiveChecksOnStartup(t *testing.T) {
	ctx := context.Background()
	taskService, mockCheckRepo, _, _, _, mockLogger := setupTestTaskService()

	checks := []*domain.Check{
		{
			ID:       "check-1",
			Status:   domain.CheckStatusActive,
			Priority: domain.PriorityNormal,
		},
		{
			ID:       "check-2",
			Status:   domain.CheckStatusActive,
			Priority: domain.PriorityHigh,
		},
	}

	// Настройка моков
	mockCheckRepo.On("GetActiveChecks", ctx).Return(checks, nil)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything)
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything) // Для check-1 с nil NextRunAt
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything)

	// Act
	err := taskService.LoadActiveChecksOnStartup(ctx)

	// Assert
	assert.NoError(t, err)
	mockCheckRepo.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestTaskService_LoadActiveChecksOnStartup_Error(t *testing.T) {
	ctx := context.Background()
	taskService, mockCheckRepo, _, _, _, mockLogger := setupTestTaskService()

	// Настройка моков
	mockCheckRepo.On("GetActiveChecks", ctx).Return(nil, assert.AnError)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything) // "Loading active checks on startup"
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Maybe() // "Failed to load active checks"

	// Act
	err := taskService.LoadActiveChecksOnStartup(ctx)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load active checks")
	mockCheckRepo.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}
