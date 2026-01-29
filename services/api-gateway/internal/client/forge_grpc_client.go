package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	forgev1 "UptimePingPlatform/proto/api/forge/v1"
)

// GRPCForgeClient gRPC клиент для ForgeService
type GRPCForgeClient struct {
	client      forgev1.ForgeServiceClient
	conn        *grpc.ClientConn
	baseHandler *grpcBase.BaseHandler
}

// NewGRPCForgeClient создает новый gRPC клиент для ForgeService
func NewGRPCForgeClient(address string, timeout time.Duration, logger logger.Logger) (*GRPCForgeClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Инициализируем BaseHandler
	baseHandler := grpcBase.NewBaseHandler(logger)

	// Логируем начало операции
	baseHandler.LogOperationStart(ctx, "grpc_forge_client_connect", map[string]interface{}{
		"address": address,
		"timeout": timeout.String(),
	})

	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		baseHandler.LogError(ctx, err, "grpc_forge_client_connect_failed", "")
		return nil, fmt.Errorf("failed to connect to forge service: %w", err)
	}

	// Проверяем соединение
	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		baseHandler.LogError(ctx, fmt.Errorf("timeout while establishing connection"), "grpc_forge_client_connect_timeout", "")
		return nil, fmt.Errorf("timeout while establishing connection")
	}

	client := forgev1.NewForgeServiceClient(conn)

	// Логируем успешное подключение
	baseHandler.LogOperationSuccess(ctx, "grpc_forge_client_connect", map[string]interface{}{
		"address": address,
	})

	return &GRPCForgeClient{
		client:      client,
		conn:        conn,
		baseHandler: baseHandler,
	}, nil
}

// GenerateConfig генерирует конфигурацию проверки из .proto файла
func (c *GRPCForgeClient) GenerateConfig(ctx context.Context, protoContent string, options *forgev1.ConfigOptions) (*forgev1.GenerateConfigResponse, error) {
	c.baseHandler.LogOperationStart(ctx, "forge_generate_config", map[string]interface{}{
		"proto_length": len(protoContent),
		"has_options": options != nil,
	})

	req := &forgev1.GenerateConfigRequest{
		ProtoContent: protoContent,
		Options:      options,
	}

	resp, err := c.client.GenerateConfig(ctx, req)
	if err != nil {
		c.baseHandler.LogError(ctx, err, "forge_generate_config_failed", "")
		return nil, fmt.Errorf("failed to generate config: %w", err)
	}

	c.baseHandler.LogOperationSuccess(ctx, "forge_generate_config", map[string]interface{}{
		"config_length": len(resp.ConfigYaml),
		"has_check_config": resp.CheckConfig != nil,
	})

	return resp, nil
}

// ParseProto парсит .proto файл
func (c *GRPCForgeClient) ParseProto(ctx context.Context, protoContent, fileName string) (*forgev1.ParseProtoResponse, error) {
	c.baseHandler.LogOperationStart(ctx, "forge_parse_proto", map[string]interface{}{
		"file_name": fileName,
		"proto_length": len(protoContent),
	})

	req := &forgev1.ParseProtoRequest{
		ProtoContent: protoContent,
		FileName:     fileName,
	}

	resp, err := c.client.ParseProto(ctx, req)
	if err != nil {
		c.baseHandler.LogError(ctx, err, "forge_parse_proto_failed", "")
		return nil, fmt.Errorf("failed to parse proto: %w", err)
	}

	c.baseHandler.LogOperationSuccess(ctx, "forge_parse_proto", map[string]interface{}{
		"is_valid": resp.IsValid,
		"warnings_count": len(resp.Warnings),
	})

	return resp, nil
}

// GenerateCode генерирует код для проверки gRPC методов
func (c *GRPCForgeClient) GenerateCode(ctx context.Context, protoContent string, options *forgev1.CodeOptions) (*forgev1.GenerateCodeResponse, error) {
	c.baseHandler.LogOperationStart(ctx, "forge_generate_code", map[string]interface{}{
		"proto_length": len(protoContent),
		"has_options": options != nil,
	})

	req := &forgev1.GenerateCodeRequest{
		ProtoContent: protoContent,
		Options:      options,
	}

	resp, err := c.client.GenerateCode(ctx, req)
	if err != nil {
		c.baseHandler.LogError(ctx, err, "forge_generate_code_failed", "")
		return nil, fmt.Errorf("failed to generate code: %w", err)
	}

	c.baseHandler.LogOperationSuccess(ctx, "forge_generate_code", map[string]interface{}{
		"code_length": len(resp.Code),
		"filename": resp.Filename,
		"language": resp.Language,
	})

	return resp, nil
}

// ValidateProto проверяет валидность .proto файла
func (c *GRPCForgeClient) ValidateProto(ctx context.Context, protoContent string) (*forgev1.ValidateProtoResponse, error) {
	c.baseHandler.LogOperationStart(ctx, "forge_validate_proto", map[string]interface{}{
		"proto_length": len(protoContent),
	})

	req := &forgev1.ValidateProtoRequest{
		ProtoContent: protoContent,
	}

	resp, err := c.client.ValidateProto(ctx, req)
	if err != nil {
		c.baseHandler.LogError(ctx, err, "forge_validate_proto_failed", "")
		return nil, fmt.Errorf("failed to validate proto: %w", err)
	}

	c.baseHandler.LogOperationSuccess(ctx, "forge_validate_proto", map[string]interface{}{
		"is_valid": resp.IsValid,
		"errors_count": len(resp.Errors),
		"warnings_count": len(resp.Warnings),
	})

	return resp, nil
}

// Close закрывает соединение
func (c *GRPCForgeClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
