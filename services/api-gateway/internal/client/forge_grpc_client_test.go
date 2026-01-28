package client

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	forgev1 "UptimePingPlatform/gen/proto/api/forge/v1"
)

// MockLogger мок для логгера
type MockLogger struct{}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {}
func (m *MockLogger) Info(msg string, fields ...logger.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...logger.Field)  {}
func (m *MockLogger) Error(msg string, fields ...logger.Field) {}
func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	return m
}
func (m *MockLogger) Sync() error {
	return nil
}

// MockForgeServiceClient мок для ForgeServiceClient
type MockForgeServiceClient struct {
	responses map[string]interface{}
	errors    map[string]error
}

func (m *MockForgeServiceClient) GenerateConfig(ctx context.Context, req *forgev1.GenerateConfigRequest, opts ...grpc.CallOption) (*forgev1.GenerateConfigResponse, error) {
	if err, exists := m.errors["GenerateConfig"]; exists {
		return nil, err
	}
	if resp, exists := m.responses["GenerateConfig"]; exists {
		return resp.(*forgev1.GenerateConfigResponse), nil
	}
	return &forgev1.GenerateConfigResponse{
		ConfigYaml: "dummy config",
		CheckConfig: &forgev1.CheckConfig{
			Name: "dummy-check",
			Type: forgev1.CheckType_CHECK_TYPE_HTTP,
			Target: "http://example.com",
		},
	}, nil
}

func (m *MockForgeServiceClient) ParseProto(ctx context.Context, req *forgev1.ParseProtoRequest, opts ...grpc.CallOption) (*forgev1.ParseProtoResponse, error) {
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

func (m *MockForgeServiceClient) GenerateCode(ctx context.Context, req *forgev1.GenerateCodeRequest, opts ...grpc.CallOption) (*forgev1.GenerateCodeResponse, error) {
	if err, exists := m.errors["GenerateCode"]; exists {
		return nil, err
	}
	if resp, exists := m.responses["GenerateCode"]; exists {
		return resp.(*forgev1.GenerateCodeResponse), nil
	}
	return &forgev1.GenerateCodeResponse{
		Code:     "dummy code",
		Filename: "dummy.go",
		Language: "go",
	}, nil
}

func (m *MockForgeServiceClient) ValidateProto(ctx context.Context, req *forgev1.ValidateProtoRequest, opts ...grpc.CallOption) (*forgev1.ValidateProtoResponse, error) {
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

// TestNewGRPCForgeClient тестирует создание клиента
func TestNewGRPCForgeClient(t *testing.T) {
	// Этот тест требует реальное gRPC соединение, поэтому мы просто проверяем структуру
	mockLogger := &MockLogger{}
	client := &GRPCForgeClient{
		baseHandler: grpcBase.NewBaseHandler(mockLogger),
	}
	
	if client.baseHandler == nil {
		t.Error("BaseHandler should be set correctly")
	}
}

// TestGRPCForgeClient_GenerateConfig тестирует генерацию конфигурации
func TestGRPCForgeClient_GenerateConfig(t *testing.T) {
	mockClient := &MockForgeServiceClient{}
	mockLogger := &MockLogger{}
	client := &GRPCForgeClient{
		client:      mockClient,
		baseHandler: grpcBase.NewBaseHandler(mockLogger),
	}

	ctx := context.Background()
	protoContent := "syntax = \"proto3\"; package test; service Test {}"
	options := &forgev1.ConfigOptions{
		TargetHost: "example.com",
		TargetPort: 80,
	}

	resp, err := client.GenerateConfig(ctx, protoContent, options)
	if err != nil {
		t.Errorf("GenerateConfig should not return error, got %v", err)
	}

	if resp == nil {
		t.Error("Response should not be nil")
	}
}

// TestGRPCForgeClient_ParseProto тестирует парсинг proto
func TestGRPCForgeClient_ParseProto(t *testing.T) {
	mockClient := &MockForgeServiceClient{}
	mockLogger := &MockLogger{}
	client := &GRPCForgeClient{
		client:      mockClient,
		baseHandler: grpcBase.NewBaseHandler(mockLogger),
	}

	ctx := context.Background()
	protoContent := "syntax = \"proto3\"; package test; service Test {}"
	fileName := "test.proto"

	resp, err := client.ParseProto(ctx, protoContent, fileName)
	if err != nil {
		t.Errorf("ParseProto should not return error, got %v", err)
	}

	if resp == nil {
		t.Error("Response should not be nil")
	}

	if !resp.IsValid {
		t.Error("Response should be valid")
	}
}

// TestGRPCForgeClient_GenerateCode тестирует генерацию кода
func TestGRPCForgeClient_GenerateCode(t *testing.T) {
	mockClient := &MockForgeServiceClient{}
	mockLogger := &MockLogger{}
	client := &GRPCForgeClient{
		client:      mockClient,
		baseHandler: grpcBase.NewBaseHandler(mockLogger),
	}

	ctx := context.Background()
	protoContent := "syntax = \"proto3\"; package test; service Test {}"
	options := &forgev1.CodeOptions{
		Language: "go",
		Framework: "grpc",
	}

	resp, err := client.GenerateCode(ctx, protoContent, options)
	if err != nil {
		t.Errorf("GenerateCode should not return error, got %v", err)
	}

	if resp == nil {
		t.Error("Response should not be nil")
	}

	if resp.Language != "go" {
		t.Errorf("Expected language 'go', got %s", resp.Language)
	}
}

// TestGRPCForgeClient_ValidateProto тестирует валидацию proto
func TestGRPCForgeClient_ValidateProto(t *testing.T) {
	mockClient := &MockForgeServiceClient{}
	mockLogger := &MockLogger{}
	client := &GRPCForgeClient{
		client:      mockClient,
		baseHandler: grpcBase.NewBaseHandler(mockLogger),
	}

	ctx := context.Background()
	protoContent := "syntax = \"proto3\"; package test; service Test {}"

	resp, err := client.ValidateProto(ctx, protoContent)
	if err != nil {
		t.Errorf("ValidateProto should not return error, got %v", err)
	}

	if resp == nil {
		t.Error("Response should not be nil")
	}

	if !resp.IsValid {
		t.Error("Response should be valid")
	}
}

// TestGRPCForgeClient_Close тестирует закрытие соединения
func TestGRPCForgeClient_Close(t *testing.T) {
	mockLogger := &MockLogger{}
	client := &GRPCForgeClient{
		baseHandler: grpcBase.NewBaseHandler(mockLogger),
	}

	// Close не должен возвращать ошибку, даже если conn равен nil
	err := client.Close()
	if err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}
}

// BenchmarkGRPCForgeClient_GenerateConfig бенчмарк для GenerateConfig
func BenchmarkGRPCForgeClient_GenerateConfig(b *testing.B) {
	mockClient := &MockForgeServiceClient{}
	mockLogger := &MockLogger{}
	client := &GRPCForgeClient{
		client:      mockClient,
		baseHandler: grpcBase.NewBaseHandler(mockLogger),
	}

	ctx := context.Background()
	protoContent := "syntax = \"proto3\"; package test; service Test {}"
	options := &forgev1.ConfigOptions{
		TargetHost: "example.com",
		TargetPort: 80,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GenerateConfig(ctx, protoContent, options)
		if err != nil {
			b.Fatalf("GenerateConfig failed: %v", err)
		}
	}
}
