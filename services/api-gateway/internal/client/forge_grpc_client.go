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
	"encoding/json"
	"net/http"
	"strings"
)

// GRPCForgeClient gRPC клиент для ForgeService
type GRPCForgeClient struct {
	client      forgev1.ForgeServiceClient
	conn        *grpc.ClientConn
	baseHandler *grpcBase.BaseHandler
	httpBaseURL string
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

	// Try HTTP health check first — if forge exposes HTTP health, use HTTP fallback
	httpClient := &http.Client{Timeout: 3 * time.Second}
	healthURL := address
	if !strings.HasPrefix(healthURL, "http://") && !strings.HasPrefix(healthURL, "https://") {
		healthURL = "http://" + healthURL
	}
	healthURL = strings.TrimRight(healthURL, "/") + "/health"
	resp, err := httpClient.Get(healthURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		// Use HTTP fallback
		baseHandler.LogOperationSuccess(ctx, "forge_http_health_check", map[string]interface{}{"url": healthURL})
		return &GRPCForgeClient{
			client:      nil,
			conn:        nil,
			baseHandler: baseHandler,
			httpBaseURL: strings.TrimSuffix(healthURL, "/health"),
		}, nil
	}

	// Otherwise, try gRPC
	conn, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		baseHandler.LogError(ctx, err, "grpc_forge_client_connect_failed", "")
		return nil, fmt.Errorf("failed to connect to forge service via gRPC: %w", err)
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
		httpBaseURL: "",
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
	// If HTTP fallback is configured, call HTTP endpoint
	if c.client == nil && c.httpBaseURL != "" {
		payload := map[string]interface{}{
			"proto_content": protoContent,
			"options":       map[string]interface{}{},
		}
		if options != nil {
			// convert options (partial)
			payload["options"] = map[string]interface{}{
				"target_host":    options.TargetHost,
				"target_port":    options.TargetPort,
				"check_interval": options.CheckInterval,
				"timeout":        options.Timeout,
				"tenant_id":      options.GetTenantId(),
			}
		}
		bodyBytes, _ := json.Marshal(payload)
		httpClient := &http.Client{Timeout: 15 * time.Second}
		httpURL := strings.TrimRight(c.httpBaseURL, "/") + "/api/v1/forge/generate_config"
		httpResp, err := httpClient.Post(httpURL, "application/json", strings.NewReader(string(bodyBytes)))
		if err != nil {
			c.baseHandler.LogError(ctx, err, "forge_generate_config_http_failed", "")
			return nil, fmt.Errorf("failed to generate config via HTTP: %w", err)
		}
		defer httpResp.Body.Close()
		var parsed struct {
			Success     bool                   `json:"success"`
			Message     string                 `json:"message"`
			ConfigYaml  string                 `json:"config_yaml"`
			CheckConfig map[string]interface{} `json:"check_config"`
		}
		if err := json.NewDecoder(httpResp.Body).Decode(&parsed); err != nil {
			c.baseHandler.LogError(ctx, err, "forge_generate_config_http_decode_failed", "")
			return nil, fmt.Errorf("failed to decode HTTP response: %w", err)
		}
		// Map to protobuf response
		// map type string to enum
		var ctype forgev1.CheckType = forgev1.CheckType_CHECK_TYPE_UNSPECIFIED
		if t, ok := parsed.CheckConfig["type"].(string); ok {
			switch strings.ToLower(t) {
			case "http":
				ctype = forgev1.CheckType_CHECK_TYPE_HTTP
			case "grpc":
				ctype = forgev1.CheckType_CHECK_TYPE_GRPC
			case "graphql":
				ctype = forgev1.CheckType_CHECK_TYPE_GRAPHQL
			default:
				ctype = forgev1.CheckType_CHECK_TYPE_UNSPECIFIED
			}
		}
		// interval/timeout parsing
		var interval int32 = 0
		if v, ok := parsed.CheckConfig["interval"]; ok {
			switch val := v.(type) {
			case float64:
				interval = int32(val)
			case int:
				interval = int32(val)
			case int32:
				interval = val
			case string:
				fmt.Sscanf(val, "%d", &interval)
			}
		}
		var timeout int32 = 0
		if v, ok := parsed.CheckConfig["timeout"]; ok {
			switch val := v.(type) {
			case float64:
				timeout = int32(val)
			case int:
				timeout = int32(val)
			case int32:
				timeout = val
			case string:
				fmt.Sscanf(val, "%d", &timeout)
			}
		}
		respProto := &forgev1.GenerateConfigResponse{
			ConfigYaml: parsed.ConfigYaml,
			CheckConfig: &forgev1.CheckConfig{
				Name:     fmt.Sprint(parsed.CheckConfig["name"]),
				Type:     ctype,
				Target:   fmt.Sprint(parsed.CheckConfig["target"]),
				Interval: interval,
				Timeout:  timeout,
				Config:   fmt.Sprint(parsed.CheckConfig["config"]),
			},
		}
		c.baseHandler.LogOperationSuccess(ctx, "forge_generate_config_http", map[string]interface{}{
			"config_length": len(respProto.ConfigYaml),
		})
		return respProto, nil
	}

	// Default: gRPC call
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
	// HTTP fallback
	if c.client == nil && c.httpBaseURL != "" {
		payload := map[string]interface{}{
			"proto_content": protoContent,
			"file_name":     fileName,
		}
		bodyBytes, _ := json.Marshal(payload)
		httpClient := &http.Client{Timeout: 10 * time.Second}
		httpURL := strings.TrimRight(c.httpBaseURL, "/") + "/api/v1/forge/parse"
		httpResp, err := httpClient.Post(httpURL, "application/json", strings.NewReader(string(bodyBytes)))
		if err != nil {
			c.baseHandler.LogError(ctx, err, "forge_parse_proto_http_failed", "")
			return nil, fmt.Errorf("failed to parse proto via HTTP: %w", err)
		}
		defer httpResp.Body.Close()
		var parsed struct {
			Success     bool   `json:"success"`
			Message     string `json:"message"`
			ServiceInfo map[string]interface{} `json:"service_info"`
			IsValid     bool   `json:"is_valid"`
			Warnings    []string `json:"warnings"`
		}
		if err := json.NewDecoder(httpResp.Body).Decode(&parsed); err != nil {
			c.baseHandler.LogError(ctx, err, "forge_parse_proto_http_decode_failed", "")
			return nil, fmt.Errorf("failed to decode HTTP response: %w", err)
		}
		respProto := &forgev1.ParseProtoResponse{
			IsValid: parsed.IsValid,
			Warnings: parsed.Warnings,
		}
		c.baseHandler.LogOperationSuccess(ctx, "forge_parse_proto_http", map[string]interface{}{
			"is_valid": respProto.IsValid,
			"warnings_count": len(respProto.Warnings),
		})
		return respProto, nil
	}

	// Default gRPC
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
