package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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

func (m *MockSchedulerRepository) UpdateCheck(ctx context.Context, check *domain.Check) error {
	args := m.Called(ctx, check)
	return args.Error(0)
}

func (m *MockSchedulerRepository) GetScheduledChecks(ctx context.Context) ([]*domain.Check, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Check), args.Error(1)
}

func setupTestUseCase() (*CheckUseCase, *MockCheckRepository, *MockSchedulerRepository) {
	mockCheckRepo := &MockCheckRepository{}
	mockSchedulerRepo := &MockSchedulerRepository{}
	useCase := NewCheckUseCase(mockCheckRepo, mockSchedulerRepo)
	return useCase, mockCheckRepo, mockSchedulerRepo
}

func TestCheckUseCase_CreateCheck_Success(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	
	check := &domain.Check{
		Name:     "Test Check",
		Type:     domain.CheckTypeHTTP,
		Target:   "https://example.com",
		Interval: 60,
		Timeout:  30,
		Status:   domain.CheckStatusActive,
		Priority: domain.PriorityNormal,
		Config:   domain.CheckConfig{"method": "GET"},
		Tags:     []string{"test"},
	}

	useCase, mockCheckRepo, mockSchedulerRepo := setupTestUseCase()

	// Настройка моков
	mockCheckRepo.On("Create", ctx, mock.AnythingOfType("*domain.Check")).Return(nil)
	mockSchedulerRepo.On("AddCheck", ctx, mock.AnythingOfType("*domain.Check")).Return(nil)

	// Вызов метода
	result, err := useCase.CreateCheck(ctx, tenantID, check)

	// Проверки
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, tenantID, result.TenantID)
	assert.NotEmpty(t, result.ID)
	assert.NotZero(t, result.CreatedAt)
	assert.NotZero(t, result.UpdatedAt)
	assert.NotNil(t, result.NextRunAt)

	mockCheckRepo.AssertExpectations(t)
	mockSchedulerRepo.AssertExpectations(t)
}

func TestCheckUseCase_CreateCheck_ValidationError(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	
	check := &domain.Check{
		Name:     "", // Пустое имя вызовет ошибку валидации
		Type:     domain.CheckTypeHTTP,
		Target:   "https://example.com",
		Interval: 60,
		Timeout:  30,
		Status:   domain.CheckStatusActive,
		Priority: domain.PriorityNormal,
	}

	useCase, _, _ := setupTestUseCase()

	// Вызов метода
	result, err := useCase.CreateCheck(ctx, tenantID, check)

	// Проверки
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestCheckUseCase_CreateCheck_SchedulerError(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	
	check := &domain.Check{
		Name:     "Test Check",
		Type:     domain.CheckTypeHTTP,
		Target:   "https://example.com",
		Interval: 60,
		Timeout:  30,
		Status:   domain.CheckStatusActive,
		Priority: domain.PriorityNormal,
	}

	useCase, mockCheckRepo, mockSchedulerRepo := setupTestUseCase()

	// Настройка моков
	mockCheckRepo.On("Create", ctx, mock.AnythingOfType("*domain.Check")).Return(nil)
	mockSchedulerRepo.On("AddCheck", ctx, mock.AnythingOfType("*domain.Check")).Return(assert.AnError)

	// Вызов метода
	result, err := useCase.CreateCheck(ctx, tenantID, check)

	// Проверки
	assert.Error(t, err)
	assert.NotNil(t, result) // Проверка создана, но есть ошибка планировщика
	assert.Contains(t, err.Error(), "failed to add to scheduler")

	mockCheckRepo.AssertExpectations(t)
	mockSchedulerRepo.AssertExpectations(t)
}

func TestCheckUseCase_UpdateCheck_Success(t *testing.T) {
	ctx := context.Background()
	checkID := "check-123"
	
	existingCheck := &domain.Check{
		ID:        checkID,
		TenantID:  "tenant-123",
		Name:      "Old Name",
		Type:      domain.CheckTypeHTTP,
		Target:    "https://old.example.com",
		Interval:  60,
		Timeout:   30,
		Status:    domain.CheckStatusActive,
		Priority:  domain.PriorityNormal,
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now().Add(-time.Hour),
	}

	updatedCheck := &domain.Check{
		Name:     "Updated Name",
		Type:     domain.CheckTypeHTTP,
		Target:   "https://updated.example.com",
		Interval: 120,
		Timeout:  60,
		Status:   domain.CheckStatusActive,
		Priority: domain.PriorityHigh,
	}

	useCase, mockCheckRepo, mockSchedulerRepo := setupTestUseCase()

	// Настройка моков
	mockCheckRepo.On("GetByID", ctx, checkID).Return(existingCheck, nil)
	mockCheckRepo.On("Update", ctx, mock.AnythingOfType("*domain.Check")).Return(nil)
	mockSchedulerRepo.On("RemoveCheck", ctx, checkID).Return(nil)
	mockSchedulerRepo.On("AddCheck", ctx, mock.AnythingOfType("*domain.Check")).Return(nil)

	// Вызов метода
	err := useCase.UpdateCheck(ctx, checkID, updatedCheck)

	// Проверки
	assert.NoError(t, err)

	mockCheckRepo.AssertExpectations(t)
	mockSchedulerRepo.AssertExpectations(t)
}

func TestCheckUseCase_UpdateCheck_NotFound(t *testing.T) {
	ctx := context.Background()
	checkID := "non-existent-check"
	
	updatedCheck := &domain.Check{
		Name:     "Updated Name",
		Type:     domain.CheckTypeHTTP,
		Target:   "https://example.com",
		Interval: 60,
		Timeout:  30,
		Status:   domain.CheckStatusActive,
		Priority: domain.PriorityNormal,
	}

	useCase, mockCheckRepo, _ := setupTestUseCase()

	// Настройка моков - возвращаем nil и ошибку
	mockCheckRepo.On("GetByID", ctx, checkID).Return(nil, assert.AnError)

	// Вызов метода
	err := useCase.UpdateCheck(ctx, checkID, updatedCheck)

	// Проверки
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get existing check")

	mockCheckRepo.AssertExpectations(t)
}

func TestCheckUseCase_DeleteCheck_Success(t *testing.T) {
	ctx := context.Background()
	checkID := "check-123"
	
	existingCheck := &domain.Check{
		ID:        checkID,
		TenantID:  "tenant-123",
		Name:      "Test Check",
		Type:      domain.CheckTypeHTTP,
		Target:    "https://example.com",
		Interval:  60,
		Timeout:   30,
		Status:    domain.CheckStatusActive,
		Priority:  domain.PriorityNormal,
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now().Add(-time.Hour),
	}

	useCase, mockCheckRepo, mockSchedulerRepo := setupTestUseCase()

	// Настройка моков
	mockCheckRepo.On("GetByID", ctx, checkID).Return(existingCheck, nil)
	mockSchedulerRepo.On("RemoveCheck", ctx, checkID).Return(nil)
	mockCheckRepo.On("Delete", ctx, checkID).Return(nil)

	// Вызов метода
	err := useCase.DeleteCheck(ctx, checkID)

	// Проверки
	assert.NoError(t, err)

	mockCheckRepo.AssertExpectations(t)
	mockSchedulerRepo.AssertExpectations(t)
}

func TestCheckUseCase_DeleteCheck_NotFound(t *testing.T) {
	ctx := context.Background()
	checkID := "non-existent-check"

	useCase, mockCheckRepo, _ := setupTestUseCase()

	// Настройка моков
	mockCheckRepo.On("GetByID", ctx, checkID).Return(nil, assert.AnError)

	// Вызов метода
	err := useCase.DeleteCheck(ctx, checkID)

	// Проверки
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get check")

	mockCheckRepo.AssertExpectations(t)
}

func TestCheckUseCase_validateHTTPConfig(t *testing.T) {
	useCase, _, _ := setupTestUseCase()

	tests := []struct {
		name    string
		config  domain.CheckConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  domain.CheckConfig{"method": "GET", "expected_status": float64(200)},
			wantErr: false,
		},
		{
			name:    "invalid method",
			config:  domain.CheckConfig{"method": "INVALID"},
			wantErr: true,
		},
		{
			name:    "invalid status code",
			config:  domain.CheckConfig{"expected_status": float64(600)},
			wantErr: true,
		},
		{
			name:    "empty config",
			config:  nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := &domain.Check{
				Type:   domain.CheckTypeHTTP,
				Config: tt.config,
			}
			err := useCase.validateHTTPConfig(check)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckUseCase_validateTCPConfig(t *testing.T) {
	useCase, _, _ := setupTestUseCase()

	tests := []struct {
		name    string
		config  domain.CheckConfig
		wantErr bool
	}{
		{
			name:    "valid port",
			config:  domain.CheckConfig{"port": float64(8080)},
			wantErr: false,
		},
		{
			name:    "invalid port too low",
			config:  domain.CheckConfig{"port": float64(0)},
			wantErr: true,
		},
		{
			name:    "invalid port too high",
			config:  domain.CheckConfig{"port": float64(70000)},
			wantErr: true,
		},
		{
			name:    "empty config",
			config:  nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := &domain.Check{
				Type:   domain.CheckTypeTCP,
				Config: tt.config,
			}
			err := useCase.validateTCPConfig(check)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
