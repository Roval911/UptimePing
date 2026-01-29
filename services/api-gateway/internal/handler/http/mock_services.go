package http

import (
	"context"
	"net/http"

	schedulerv1 "UptimePingPlatform/proto/api/scheduler/v1"
	"UptimePingPlatform/pkg/logger"
)

// MockAuthService мок для сервиса аутентификации
type MockAuthService struct {
	log logger.Logger
}

func NewMockAuthService(log logger.Logger) *MockAuthService {
	return &MockAuthService{log: log}
}

func (m *MockAuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	m.log.Info("Mock login",
		logger.String("email", email),
		logger.String("action", "mock_auth"))

	return &TokenPair{
		AccessToken:  "mock_access_token",
		RefreshToken: "mock_refresh_token",
	}, nil
}

func (m *MockAuthService) Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error) {
	m.log.Info("Mock register",
		logger.String("email", email),
		logger.String("tenant", tenantName),
		logger.String("action", "mock_auth"))

	return &TokenPair{
		AccessToken:  "mock_access_token",
		RefreshToken: "mock_refresh_token",
	}, nil
}

func (m *MockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	m.log.Info("Mock refresh token",
		logger.String("action", "mock_auth"))

	return &TokenPair{
		AccessToken:  "mock_access_token",
		RefreshToken: "mock_refresh_token",
	}, nil
}

func (m *MockAuthService) Logout(ctx context.Context, userID, tokenID string) error {
	m.log.Info("Mock logout",
		logger.String("user_id", userID),
		logger.String("action", "mock_auth"))
	return nil
}

// MockHealthHandler мок для health handler
type MockHealthHandler struct {
	log logger.Logger
}

func NewMockHealthHandler(log logger.Logger) *MockHealthHandler {
	return &MockHealthHandler{log: log}
}

func (m *MockHealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	m.log.Info("Mock health check",
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (m *MockHealthHandler) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	m.log.Info("Mock ready check",
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}

func (m *MockHealthHandler) LiveCheck(w http.ResponseWriter, r *http.Request) {
	m.log.Info("Mock live check",
		logger.String("method", r.Method),
		logger.String("path", r.URL.Path))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Live"))
}

// MockSchedulerClient мок для SchedulerClient
type MockSchedulerClient struct {
	log logger.Logger
}

func NewMockSchedulerClient(log logger.Logger) *MockSchedulerClient {
	return &MockSchedulerClient{log: log}
}

func (m *MockSchedulerClient) ListChecks(ctx context.Context, req *schedulerv1.ListChecksRequest) (*schedulerv1.ListChecksResponse, error) {
	m.log.Info("Mock list checks",
		logger.String("action", "mock_scheduler"))

	return &schedulerv1.ListChecksResponse{
		Checks: []*schedulerv1.Check{},
	}, nil
}

func (m *MockSchedulerClient) CreateCheck(ctx context.Context, req *schedulerv1.CreateCheckRequest) (*schedulerv1.Check, error) {
	m.log.Info("Mock create check",
		logger.String("name", req.Name),
		logger.String("action", "mock_scheduler"))

	return &schedulerv1.Check{
		Id:   "mock-check-id",
		Name: req.Name,
	}, nil
}

func (m *MockSchedulerClient) Close() error {
	m.log.Info("Mock scheduler client closed",
		logger.String("action", "mock_scheduler"))
	return nil
}

// MockForgeServiceClient мок для ForgeServiceClient
type MockForgeServiceClient struct {
	log logger.Logger
}

func NewMockForgeServiceClient(log logger.Logger) *MockForgeServiceClient {
	return &MockForgeServiceClient{log: log}
}

func (m *MockForgeServiceClient) GenerateConfig(ctx context.Context, protoContent string, options interface{}) (interface{}, error) {
	m.log.Info("Mock generate config",
		logger.String("action", "mock_forge"))
	return nil, nil
}

func (m *MockForgeServiceClient) ParseProto(ctx context.Context, protoContent, fileName string) (interface{}, error) {
	m.log.Info("Mock parse proto",
		logger.String("action", "mock_forge"))
	return nil, nil
}

func (m *MockForgeServiceClient) GenerateCode(ctx context.Context, protoContent string, options interface{}) (interface{}, error) {
	m.log.Info("Mock generate code",
		logger.String("action", "mock_forge"))
	return nil, nil
}

func (m *MockForgeServiceClient) ValidateProto(ctx context.Context, protoContent string) (interface{}, error) {
	m.log.Info("Mock validate proto",
		logger.String("action", "mock_forge"))
	return nil, nil
}

func (m *MockForgeServiceClient) Close() error {
	m.log.Info("Mock forge client closed",
		logger.String("action", "mock_forge"))
	return nil
}
