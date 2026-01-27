package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"UptimePingPlatform/pkg/logger"
	forgev1 "UptimePingPlatform/gen/go/proto/api/forge/v1"
)

// MockForgeClient мок для ForgeServiceClient
type MockForgeClient struct {
	responses map[string]interface{}
	errors    map[string]error
}

func (m *MockForgeClient) GenerateConfig(ctx context.Context, protoContent string, options *forgev1.ConfigOptions) (*forgev1.GenerateConfigResponse, error) {
	if err, exists := m.errors["GenerateConfig"]; exists {
		return nil, err
	}
	if resp, exists := m.responses["GenerateConfig"]; exists {
		return resp.(*forgev1.GenerateConfigResponse), nil
	}
	return &forgev1.GenerateConfigResponse{
		ConfigYaml: "test_config_yaml",
		CheckConfig: &forgev1.CheckConfig{
			Name: "test-check",
			Type: forgev1.CheckType_CHECK_TYPE_HTTP,
			Target: "http://example.com",
		},
	}, nil
}

func (m *MockForgeClient) ParseProto(ctx context.Context, protoContent, fileName string) (*forgev1.ParseProtoResponse, error) {
	if err, exists := m.errors["ParseProto"]; exists {
		return nil, err
	}
	if resp, exists := m.responses["ParseProto"]; exists {
		return resp.(*forgev1.ParseProtoResponse), nil
	}
	return &forgev1.ParseProtoResponse{
		ServiceInfo: &forgev1.ServiceInfo{
			PackageName: "test.package",
			ServiceName: "TestService",
		},
		IsValid: true,
	}, nil
}

func (m *MockForgeClient) GenerateCode(ctx context.Context, protoContent string, options *forgev1.CodeOptions) (*forgev1.GenerateCodeResponse, error) {
	if err, exists := m.errors["GenerateCode"]; exists {
		return nil, err
	}
	if resp, exists := m.responses["GenerateCode"]; exists {
		return resp.(*forgev1.GenerateCodeResponse), nil
	}
	return &forgev1.GenerateCodeResponse{
		Code:     "test_code",
		Filename: "test.go",
		Language: "go",
	}, nil
}

func (m *MockForgeClient) ValidateProto(ctx context.Context, protoContent string) (*forgev1.ValidateProtoResponse, error) {
	if err, exists := m.errors["ValidateProto"]; exists {
		return nil, err
	}
	if resp, exists := m.responses["ValidateProto"]; exists {
		return resp.(*forgev1.ValidateProtoResponse), nil
	}
	return &forgev1.ValidateProtoResponse{
		IsValid: true,
	}, nil
}

func (m *MockForgeClient) Close() error {
	return nil
}

// MockForgeLogger мок для логгера
type MockForgeLogger struct{}

func (m *MockForgeLogger) Debug(msg string, fields ...logger.Field) {}
func (m *MockForgeLogger) Info(msg string, fields ...logger.Field)  {}
func (m *MockForgeLogger) Warn(msg string, fields ...logger.Field)  {}
func (m *MockForgeLogger) Error(msg string, fields ...logger.Field) {}
func (m *MockForgeLogger) With(fields ...logger.Field) logger.Logger {
	return m
}
func (m *MockForgeLogger) Sync() error {
	return nil
}

func createTestHandler() *Handler {
	mockAuthService := &MockAuthService{}
	mockHealthHandler := &MockHealthHandler{}
	mockSchedulerClient := &MockSchedulerClient{}
	mockForgeClient := &MockForgeClient{}
	mockLogger := &MockForgeLogger{}

	return NewHandler(mockAuthService, mockHealthHandler, mockSchedulerClient, mockForgeClient, mockLogger)
}

func TestHandleForgeProxy_GenerateConfig(t *testing.T) {
	handler := createTestHandler()

	// Создаем тестовый запрос
	reqBody := map[string]interface{}{
		"proto_content": "syntax = \"proto3\"; package test; service Test {}",
		"action":       "generate_config",
		"options": map[string]interface{}{
			"target_host": "example.com",
			"target_port": 8080,
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/forge/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleForgeProxy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if !response["success"].(bool) {
		t.Error("Expected success to be true")
	}

	if response["config_yaml"] == nil {
		t.Error("Expected config_yaml in response")
	}
}

func TestHandleForgeProxy_ParseProto(t *testing.T) {
	handler := createTestHandler()

	// Создаем тестовый запрос
	reqBody := map[string]interface{}{
		"proto_content": "syntax = \"proto3\"; package test; service Test {}",
		"action":       "parse_proto",
		"file_name":    "test.proto",
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/forge/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleForgeProxy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if !response["success"].(bool) {
		t.Error("Expected success to be true")
	}

	if response["service_info"] == nil {
		t.Error("Expected service_info in response")
	}
}

func TestHandleForgeProxy_GenerateCode(t *testing.T) {
	handler := createTestHandler()

	// Создаем тестовый запрос
	reqBody := map[string]interface{}{
		"proto_content": "syntax = \"proto3\"; package test; service Test {}",
		"action":       "generate_code",
		"options": map[string]interface{}{
			"language":  "go",
			"framework": "grpc",
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/forge/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleForgeProxy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if !response["success"].(bool) {
		t.Error("Expected success to be true")
	}

	if response["code"] == nil {
		t.Error("Expected code in response")
	}
}

func TestHandleForgeProxy_ValidateProto(t *testing.T) {
	handler := createTestHandler()

	// Создаем тестовый запрос
	reqBody := map[string]interface{}{
		"proto_content": "syntax = \"proto3\"; package test; service Test {}",
		"action":       "validate_proto",
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/forge/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleForgeProxy(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if !response["success"].(bool) {
		t.Error("Expected success to be true")
	}

	if response["is_valid"] == nil {
		t.Error("Expected is_valid in response")
	}
}

func TestHandleForgeProxy_InvalidMethod(t *testing.T) {
	handler := createTestHandler()

	req := httptest.NewRequest("GET", "/api/v1/forge/generate", nil)
	w := httptest.NewRecorder()

	handler.handleForgeProxy(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleForgeProxy_InvalidJSON(t *testing.T) {
	handler := createTestHandler()

	req := httptest.NewRequest("POST", "/api/v1/forge/generate", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleForgeProxy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleForgeProxy_MissingRequiredFields(t *testing.T) {
	handler := createTestHandler()

	// Запрос без обязательных полей
	reqBody := map[string]interface{}{
		"options": map[string]interface{}{
			"target_host": "example.com",
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/forge/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleForgeProxy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleForgeProxy_InvalidAction(t *testing.T) {
	handler := createTestHandler()

	reqBody := map[string]interface{}{
		"proto_content": "syntax = \"proto3\"; package test; service Test {}",
		"action":       "invalid_action",
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/forge/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleForgeProxy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleForgeProxy_ProtoContentTooShort(t *testing.T) {
	handler := createTestHandler()

	reqBody := map[string]interface{}{
		"proto_content": "short",
		"action":       "generate_config",
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/forge/generate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleForgeProxy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
