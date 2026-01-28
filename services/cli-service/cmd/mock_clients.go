package cmd

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// Mock proto types for demonstration
type MockAuthClient struct{}
type MockConfigClient struct{}
type MockCoreClient struct{}
type MockIncidentClient struct{}
type MockNotificationClient struct{}
type MockForgeClient struct{}

// Mock auth methods
func (m *MockAuthClient) Login(ctx context.Context, req interface{}) (interface{}, error) {
	return &LoginResponse{Token: "mock-token"}, nil
}

func (m *MockAuthClient) Register(ctx context.Context, req interface{}) (interface{}, error) {
	return &RegisterResponse{UserId: "mock-user-id"}, nil
}

func (m *MockAuthClient) ValidateToken(ctx context.Context, req interface{}) (interface{}, error) {
	return &ValidateTokenResponse{
		UserId:    "mock-user-id",
		Email:     "user@example.com",
		TenantId:  "mock-tenant",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil
}

func (m *MockAuthClient) CreateAPIKey(ctx context.Context, req interface{}) (interface{}, error) {
	return &CreateAPIKeyResponse{
		KeyId:     "mock-key-id",
		ApiKey:    "mock-api-key",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}, nil
}

func (m *MockAuthClient) ListAPIKeys(ctx context.Context, req interface{}) (interface{}, error) {
	return &ListAPIKeysResponse{Keys: []interface{}{}}, nil
}

func (m *MockAuthClient) RevokeAPIKey(ctx context.Context, req interface{}) (interface{}, error) {
	return &struct{}{}, nil
}

// Mock core methods
func (m *MockCoreClient) ExecuteCheck(ctx context.Context, req interface{}) (interface{}, error) {
	return &ExecuteCheckResponse{
		CheckId:      "mock-check-id",
		Status:       "success",
		ResponseTime: 100,
		Message:      "Check completed successfully",
	}, nil
}

func (m *MockCoreClient) GetCheckStatus(ctx context.Context, req interface{}) (interface{}, error) {
	return &GetCheckStatusResponse{
		CheckId:      "mock-check-id",
		Name:         "Mock Check",
		Type:         "http",
		Status:       "active",
		LastCheck:    time.Now(),
		NextCheck:    time.Now().Add(time.Minute),
		SuccessRate:  99.5,
		TotalChecks:  1000,
		FailedChecks: 5,
	}, nil
}

func (m *MockCoreClient) GetCheckHistory(ctx context.Context, req interface{}) (interface{}, error) {
	return &GetCheckHistoryResponse{Results: []CheckResult{}}, nil
}

func (m *MockCoreClient) ListChecks(ctx context.Context, req interface{}) (interface{}, error) {
	return &ListChecksResponse{Checks: []CheckInfo{}}, nil
}

// Mock incident methods
func (m *MockIncidentClient) ListIncidents(ctx context.Context, req interface{}) (interface{}, error) {
	return &ListIncidentsResponse{Incidents: []IncidentInfo{}}, nil
}

func (m *MockIncidentClient) GetIncident(ctx context.Context, req interface{}) (interface{}, error) {
	return &GetIncidentResponse{
		IncidentId: "mock-incident-id",
		Title:      "Mock Incident",
		Status:     "open",
		Severity:   "medium",
		CreatedAt:  time.Now(),
	}, nil
}

func (m *MockIncidentClient) AcknowledgeIncident(ctx context.Context, req interface{}) (interface{}, error) {
	return &AcknowledgeIncidentResponse{
		AcknowledgedAt: time.Now(),
		AcknowledgedBy: "mock-user",
	}, nil
}

func (m *MockIncidentClient) ResolveIncident(ctx context.Context, req interface{}) (interface{}, error) {
	return &ResolveIncidentResponse{
		ResolvedAt: time.Now(),
		ResolvedBy: "mock-user",
	}, nil
}

// Mock notification methods
func (m *MockNotificationClient) CreateChannel(ctx context.Context, req interface{}) (interface{}, error) {
	return &CreateChannelResponse{ChannelId: "mock-channel-id"}, nil
}

func (m *MockNotificationClient) DeleteChannel(ctx context.Context, req interface{}) (interface{}, error) {
	return &struct{}{}, nil
}

func (m *MockNotificationClient) ListChannels(ctx context.Context, req interface{}) (interface{}, error) {
	return &ListChannelsResponse{Channels: []ChannelInfo{}}, nil
}

func (m *MockNotificationClient) SendNotification(ctx context.Context, req interface{}) (interface{}, error) {
	return &SendNotificationResponse{
		NotificationId: "mock-notification-id",
		Status:         "sent",
		SentAt:         time.Now(),
	}, nil
}

// Mock forge methods
func (m *MockForgeClient) Generate(ctx context.Context, req interface{}) (interface{}, error) {
	return &GenerateResponse{
		GeneratedFiles: 5,
		OutputPath:     "/tmp/generated",
		GenerationTime: time.Now(),
		Files:          []string{"file1.go", "file2.go"},
	}, nil
}

func (m *MockForgeClient) Validate(ctx context.Context, req interface{}) (interface{}, error) {
	return &ValidateResponse{
		Valid:          true,
		Status:         "success",
		FilesChecked:   3,
		Errors:         []ValidationError{},
		Warnings:       []ValidationWarning{},
		ValidationTime: time.Now(),
	}, nil
}

// Mock config methods
func (m *MockConfigClient) CreateConfig(ctx context.Context, req interface{}) (interface{}, error) {
	return &CreateConfigResponse{ConfigId: "mock-config-id"}, nil
}

func (m *MockConfigClient) ListConfigs(ctx context.Context, req interface{}) (interface{}, error) {
	return &ListConfigsResponse{Configs: []ConfigInfo{}}, nil
}

// Mock client creators
func getMockAuthClient() (*MockAuthClient, *grpc.ClientConn, error) {
	return &MockAuthClient{}, nil, nil
}

func getMockConfigClient() (*MockConfigClient, *grpc.ClientConn, error) {
	return &MockConfigClient{}, nil, nil
}

func getMockCoreClient() (*MockCoreClient, *grpc.ClientConn, error) {
	return &MockCoreClient{}, nil, nil
}

func getMockIncidentClient() (*MockIncidentClient, *grpc.ClientConn, error) {
	return &MockIncidentClient{}, nil, nil
}

func getMockNotificationClient() (*MockNotificationClient, *grpc.ClientConn, error) {
	return &MockNotificationClient{}, nil, nil
}

func getMockForgeClient() (*MockForgeClient, *grpc.ClientConn, error) {
	return &MockForgeClient{}, nil, nil
}
