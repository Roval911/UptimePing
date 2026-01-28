package client

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"UptimePingPlatform/pkg/logger"
)

// MockLogger для тестов
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...logger.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	args := m.Called(fields)
	return args.Get(0).(logger.Logger)
}

func (m *MockLogger) Sync() error {
	args := m.Called()
	return args.Error(0)
}

// MockGRPCClient для тестов
type MockGRPCClient struct {
	mock.Mock
}

func (m *MockGRPCClient) CreateCheck(ctx context.Context, req *CheckCreateRequest) (*Check, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*Check), args.Error(1)
}

func (m *MockGRPCClient) GetCheck(ctx context.Context, checkID string) (*Check, error) {
	args := m.Called(ctx, checkID)
	return args.Get(0).(*Check), args.Error(1)
}

func (m *MockGRPCClient) UpdateCheck(ctx context.Context, checkID string, req *CheckUpdateRequest) (*Check, error) {
	args := m.Called(ctx, checkID, req)
	return args.Get(0).(*Check), args.Error(1)
}

func (m *MockGRPCClient) ListChecks(ctx context.Context, tags []string, enabled *bool, page, pageSize int) (*CheckListResponse, error) {
	args := m.Called(ctx, tags, enabled, page, pageSize)
	return args.Get(0).(*CheckListResponse), args.Error(1)
}

func (m *MockGRPCClient) RunCheck(ctx context.Context, checkID string) (*CheckRunResponse, error) {
	args := m.Called(ctx, checkID)
	return args.Get(0).(*CheckRunResponse), args.Error(1)
}

func (m *MockGRPCClient) GetCheckStatus(ctx context.Context, checkID string) (*CheckStatusResponse, error) {
	args := m.Called(ctx, checkID)
	return args.Get(0).(*CheckStatusResponse), args.Error(1)
}

func (m *MockGRPCClient) GetCheckHistory(ctx context.Context, checkID string, page, pageSize int) (*CheckHistoryResponse, error) {
	args := m.Called(ctx, checkID, page, pageSize)
	return args.Get(0).(*CheckHistoryResponse), args.Error(1)
}

func (m *MockGRPCClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestConfigClient_CreateCheck_MockMode(t *testing.T) {
	// Создаем mock логгер
	mockLogger := &MockLogger{}
	
	// Создаем клиент в mock режиме (useGRPC=false)
	client := NewConfigClient("http://localhost:8080", mockLogger)
	
	// Создаем запрос
	req := &CheckCreateRequest{
		Name:     "Test Check",
		Type:     "http",
		Target:   "https://example.com",
		Interval: 60,
		Timeout:  10,
		Tags:     []string{"test"},
		Metadata: map[string]string{"env": "test"},
	}
	
	// Вызываем метод
	ctx := context.Background()
	check, err := client.CreateCheck(ctx, req)
	
	// Проверяем результат
	assert.Error(t, err)
	assert.Nil(t, check)
	assert.Contains(t, err.Error(), "gRPC не настроен")
}

func TestConfigClient_CreateCheck_GRPCMode(t *testing.T) {
	// Создаем mock логгер
	mockLogger := &MockLogger{}
	
	// Создаем mock gRPC клиент
	mockGRPCClient := &MockGRPCClient{}
	
	// Настраиваем ожидания для mock
	expectedCheck := &Check{
		ID:        "check-123",
		Name:      "Test Check",
		Type:      "http",
		Target:    "https://example.com",
		Interval:  60,
		Timeout:   10,
		Enabled:   true,
		Tags:      []string{"test"},
		Metadata:  map[string]string{"env": "test"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockGRPCClient.On("CreateCheck", mock.Anything, mock.Anything).Return(expectedCheck, nil)
	
	// Создаем клиент с gRPC
	client := &ConfigClient{
		baseURL:    "http://localhost:8080",
		logger:     mockLogger,
		grpcClient: mockGRPCClient,
		useGRPC:    true,
	}
	
	// Создаем запрос
	req := &CheckCreateRequest{
		Name:     "Test Check",
		Type:     "http",
		Target:   "https://example.com",
		Interval: 60,
		Timeout:  10,
		Tags:     []string{"test"},
		Metadata: map[string]string{"env": "test"},
	}
	
	// Вызываем метод
	ctx := context.Background()
	check, err := client.CreateCheck(ctx, req)
	
	// Проверяем результат
	assert.NoError(t, err)
	assert.NotNil(t, check)
	assert.Equal(t, expectedCheck.ID, check.ID)
	assert.Equal(t, expectedCheck.Name, check.Name)
	assert.Equal(t, expectedCheck.Type, check.Type)
	assert.Equal(t, expectedCheck.Target, check.Target)
	
	// Проверяем, что mock был вызван
	mockGRPCClient.AssertExpectations(t)
}

func TestConfigClient_GetCheck_MockMode(t *testing.T) {
	mockLogger := &MockLogger{}
	client := NewConfigClient("http://localhost:8080", mockLogger)
	
	ctx := context.Background()
	check, err := client.GetCheck(ctx, "check-123")
	
	assert.Error(t, err)
	assert.Nil(t, check)
	assert.Contains(t, err.Error(), "gRPC не настроен")
}

func TestConfigClient_GetCheck_GRPCMode(t *testing.T) {
	mockLogger := &MockLogger{}
	mockGRPCClient := &MockGRPCClient{}
	
	expectedCheck := &Check{
		ID:        "check-123",
		Name:      "Test Check",
		Type:      "http",
		Target:    "https://example.com",
		Interval:  60,
		Timeout:   10,
		Enabled:   true,
		Tags:      []string{"test"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockGRPCClient.On("GetCheck", mock.Anything, "check-123").Return(expectedCheck, nil)
	
	client := &ConfigClient{
		baseURL:    "http://localhost:8080",
		logger:     mockLogger,
		grpcClient: mockGRPCClient,
		useGRPC:    true,
	}
	
	ctx := context.Background()
	check, err := client.GetCheck(ctx, "check-123")
	
	assert.NoError(t, err)
	assert.NotNil(t, check)
	assert.Equal(t, expectedCheck.ID, check.ID)
	assert.Equal(t, expectedCheck.Name, check.Name)
	
	mockGRPCClient.AssertExpectations(t)
}

func TestConfigClient_UpdateCheck_MockMode(t *testing.T) {
	mockLogger := &MockLogger{}
	client := NewConfigClient("http://localhost:8080", mockLogger)
	
	req := &CheckUpdateRequest{
		Name: stringPtr("Updated Name"),
	}
	
	ctx := context.Background()
	check, err := client.UpdateCheck(ctx, "check-123", req)
	
	assert.Error(t, err)
	assert.Nil(t, check)
	assert.Contains(t, err.Error(), "gRPC не настроен")
}

func TestConfigClient_UpdateCheck_GRPCMode(t *testing.T) {
	mockLogger := &MockLogger{}
	mockGRPCClient := &MockGRPCClient{}
	
	expectedCheck := &Check{
		ID:        "check-123",
		Name:      "Updated Name",
		Type:      "http",
		Target:    "https://example.com",
		Interval:  120,
		Timeout:   15,
		Enabled:   true,
		Tags:      []string{"updated"},
		UpdatedAt: time.Now(),
	}
	
	mockGRPCClient.On("UpdateCheck", mock.Anything, "check-123", mock.Anything).Return(expectedCheck, nil)
	
	client := &ConfigClient{
		baseURL:    "http://localhost:8080",
		logger:     mockLogger,
		grpcClient: mockGRPCClient,
		useGRPC:    true,
	}
	
	req := &CheckUpdateRequest{
		Name: stringPtr("Updated Name"),
	}
	
	ctx := context.Background()
	check, err := client.UpdateCheck(ctx, "check-123", req)
	
	assert.NoError(t, err)
	assert.NotNil(t, check)
	assert.Equal(t, expectedCheck.Name, check.Name)
	assert.Equal(t, expectedCheck.Interval, check.Interval)
	
	mockGRPCClient.AssertExpectations(t)
}

func TestConfigClient_ListChecks_MockMode(t *testing.T) {
	mockLogger := &MockLogger{}
	client := NewConfigClient("http://localhost:8080", mockLogger)
	
	ctx := context.Background()
	response, err := client.ListChecks(ctx, []string{"test"}, boolPtr(true), 1, 20)
	
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "gRPC не настроен")
}

func TestConfigClient_ListChecks_GRPCMode(t *testing.T) {
	mockLogger := &MockLogger{}
	mockGRPCClient := &MockGRPCClient{}
	
	expectedResponse := &CheckListResponse{
		Checks: []Check{
			{
				ID:        "check-1",
				Name:      "Check 1",
				Type:      "http",
				Target:    "https://example1.com",
				Interval:  60,
				Timeout:   10,
				Enabled:   true,
				Tags:      []string{"test"},
			},
			{
				ID:        "check-2",
				Name:      "Check 2",
				Type:      "tcp",
				Target:    "localhost:8080",
				Interval:  30,
				Timeout:   5,
				Enabled:   true,
				Tags:      []string{"test"},
			},
		},
		Total: 2,
	}
	
	mockGRPCClient.On("ListChecks", mock.Anything, []string{"test"}, boolPtr(true), 1, 20).Return(expectedResponse, nil)
	
	client := &ConfigClient{
		baseURL:    "http://localhost:8080",
		logger:     mockLogger,
		grpcClient: mockGRPCClient,
		useGRPC:    true,
	}
	
	ctx := context.Background()
	response, err := client.ListChecks(ctx, []string{"test"}, boolPtr(true), 1, 20)
	
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Checks, 2)
	assert.Equal(t, 2, response.Total)
	assert.Equal(t, "check-1", response.Checks[0].ID)
	assert.Equal(t, "check-2", response.Checks[1].ID)
	
	mockGRPCClient.AssertExpectations(t)
}

func TestConfigClient_RunCheck_MockMode(t *testing.T) {
	mockLogger := &MockLogger{}
	client := NewConfigClient("http://localhost:8080", mockLogger)
	
	ctx := context.Background()
	response, err := client.RunCheck(ctx, "check-123")
	
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "gRPC не настроен")
}

func TestConfigClient_RunCheck_GRPCMode(t *testing.T) {
	mockLogger := &MockLogger{}
	mockGRPCClient := &MockGRPCClient{}
	
	expectedResponse := &CheckRunResponse{
		ExecutionID: "exec-123456",
		Status:      "success",
		Message:     "Проверка выполнена успешно",
		StartedAt:   time.Now(),
	}
	
	mockGRPCClient.On("RunCheck", mock.Anything, "check-123").Return(expectedResponse, nil)
	
	client := &ConfigClient{
		baseURL:    "http://localhost:8080",
		logger:     mockLogger,
		grpcClient: mockGRPCClient,
		useGRPC:    true,
	}
	
	ctx := context.Background()
	response, err := client.RunCheck(ctx, "check-123")
	
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, expectedResponse.ExecutionID, response.ExecutionID)
	assert.Equal(t, expectedResponse.Status, response.Status)
	assert.Equal(t, expectedResponse.Message, response.Message)
	
	mockGRPCClient.AssertExpectations(t)
}

func TestConfigClient_GetCheckStatus_MockMode(t *testing.T) {
	mockLogger := &MockLogger{}
	client := NewConfigClient("http://localhost:8080", mockLogger)
	
	ctx := context.Background()
	response, err := client.GetCheckStatus(ctx, "check-123")
	
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "gRPC не настроен")
}

func TestConfigClient_GetCheckStatus_GRPCMode(t *testing.T) {
	mockLogger := &MockLogger{}
	mockGRPCClient := &MockGRPCClient{}
	
	expectedResponse := &CheckStatusResponse{
		CheckID:     "check-123",
		Status:      "success",
		LastRun:     time.Now().Add(-5 * time.Minute),
		NextRun:     time.Now().Add(55 * time.Minute),
		LastStatus:  "success",
		LastMessage: "Проверка прошла успешно",
		IsRunning:   false,
	}
	
	mockGRPCClient.On("GetCheckStatus", mock.Anything, "check-123").Return(expectedResponse, nil)
	
	client := &ConfigClient{
		baseURL:    "http://localhost:8080",
		logger:     mockLogger,
		grpcClient: mockGRPCClient,
		useGRPC:    true,
	}
	
	ctx := context.Background()
	response, err := client.GetCheckStatus(ctx, "check-123")
	
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, expectedResponse.CheckID, response.CheckID)
	assert.Equal(t, expectedResponse.Status, response.Status)
	assert.Equal(t, expectedResponse.IsRunning, response.IsRunning)
	
	mockGRPCClient.AssertExpectations(t)
}

func TestConfigClient_GetCheckHistory_MockMode(t *testing.T) {
	mockLogger := &MockLogger{}
	client := NewConfigClient("http://localhost:8080", mockLogger)
	
	ctx := context.Background()
	response, err := client.GetCheckHistory(ctx, "check-123", 1, 10)
	
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "gRPC не настроен")
}

func TestConfigClient_GetCheckHistory_GRPCMode(t *testing.T) {
	mockLogger := &MockLogger{}
	mockGRPCClient := &MockGRPCClient{}
	
	expectedResponse := &CheckHistoryResponse{
		Executions: []CheckExecution{
			{
				ExecutionID: "exec-1",
				CheckID:     "check-123",
				Status:      "success",
				Message:     "Проверка прошла успешно",
				Duration:    1250,
				StartedAt:   time.Now().Add(-5 * time.Minute),
				CompletedAt: time.Now().Add(-5 * time.Minute).Add(1250 * time.Millisecond),
			},
			{
				ExecutionID: "exec-2",
				CheckID:     "check-123",
				Status:      "failed",
				Message:     "Timeout",
				Duration:    10000,
				StartedAt:   time.Now().Add(-10 * time.Minute),
				CompletedAt: time.Now().Add(-10 * time.Minute).Add(10 * time.Second),
			},
		},
		Total:    2,
		Page:     1,
		PageSize: 10,
	}
	
	mockGRPCClient.On("GetCheckHistory", mock.Anything, "check-123", 1, 10).Return(expectedResponse, nil)
	
	client := &ConfigClient{
		baseURL:    "http://localhost:8080",
		logger:     mockLogger,
		grpcClient: mockGRPCClient,
		useGRPC:    true,
	}
	
	ctx := context.Background()
	response, err := client.GetCheckHistory(ctx, "check-123", 1, 10)
	
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Executions, 2)
	assert.Equal(t, 2, response.Total)
	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 10, response.PageSize)
	assert.Equal(t, "exec-1", response.Executions[0].ExecutionID)
	assert.Equal(t, "exec-2", response.Executions[1].ExecutionID)
	
	mockGRPCClient.AssertExpectations(t)
}

func TestConfigClient_Close_MockMode(t *testing.T) {
	mockLogger := &MockLogger{}
	client := NewConfigClient("http://localhost:8080", mockLogger)
	
	// В mock режиме Close должен возвращать nil (нет gRPC клиента)
	err := client.Close()
	assert.NoError(t, err)
}

func TestConfigClient_Close_GRPCMode(t *testing.T) {
	mockLogger := &MockLogger{}
	mockGRPCClient := &MockGRPCClient{}
	
	mockGRPCClient.On("Close").Return(nil)
	
	client := &ConfigClient{
		baseURL:    "http://localhost:8080",
		logger:     mockLogger,
		grpcClient: mockGRPCClient,
		useGRPC:    true,
	}
	
	err := client.Close()
	assert.NoError(t, err)
	
	mockGRPCClient.AssertExpectations(t)
}

// Вспомогательные функции
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
