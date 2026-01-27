package http

import (
	"context"
	"net/http"

	schedulerv1 "UptimePingPlatform/gen/go/proto/api/scheduler/v1"
)

// MockAuthService мок для сервиса аутентификации
type MockAuthService struct{}

func (m *MockAuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	return &TokenPair{
		AccessToken:  "mock_access_token",
		RefreshToken: "mock_refresh_token",
	}, nil
}

func (m *MockAuthService) Register(ctx context.Context, email, password, tenantName string) (*TokenPair, error) {
	return &TokenPair{
		AccessToken:  "mock_access_token",
		RefreshToken: "mock_refresh_token",
	}, nil
}

func (m *MockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	return &TokenPair{
		AccessToken:  "mock_access_token",
		RefreshToken: "mock_refresh_token",
	}, nil
}

func (m *MockAuthService) Logout(ctx context.Context, userID, tokenID string) error {
	return nil
}

// MockHealthHandler мок для health handler
type MockHealthHandler struct{}

func (m *MockHealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (m *MockHealthHandler) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}

func (m *MockHealthHandler) LiveCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Live"))
}

// MockSchedulerClient мок для SchedulerClient
type MockSchedulerClient struct{}

func (m *MockSchedulerClient) ListChecks(ctx context.Context, req *schedulerv1.ListChecksRequest) (*schedulerv1.ListChecksResponse, error) {
	return &schedulerv1.ListChecksResponse{
		Checks: []*schedulerv1.Check{},
	}, nil
}

func (m *MockSchedulerClient) CreateCheck(ctx context.Context, req *schedulerv1.CreateCheckRequest) (*schedulerv1.Check, error) {
	return &schedulerv1.Check{
		Id:   "mock-check-id",
		Name: req.Name,
	}, nil
}

func (m *MockSchedulerClient) Close() error {
	return nil
}
