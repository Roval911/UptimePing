package grpc

import (
	"context"
	"fmt"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/forge-service/internal/service"

	forgev1 "UptimePingPlatform/proto/api/forge/v1"
)

// ForgeHandler реализует gRPC обработчики для ForgeService
type ForgeHandler struct {
	*grpcBase.BaseHandler
	forgev1.UnimplementedForgeServiceServer
	forgeService service.ForgeService
	validator    *validation.Validator
}

// NewForgeHandler создает новый экземпляр ForgeHandler
func NewForgeHandler(forgeService service.ForgeService, logger logger.Logger) *ForgeHandler {
	return &ForgeHandler{
		BaseHandler:  grpcBase.NewBaseHandler(logger),
		forgeService: forgeService,
		validator:    validation.NewValidator(),
	}
}

// ParseProto парсит .proto файл
func (h *ForgeHandler) ParseProto(ctx context.Context, req *forgev1.ParseProtoRequest) (*forgev1.ParseProtoResponse, error) {
	h.LogOperationStart(ctx, "ParseProto", map[string]interface{}{
		"file_name":    req.FileName,
		"proto_length": len(req.ProtoContent),
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "ParseProto", map[string]string{
		"proto_content": req.ProtoContent,
		"file_name":     req.FileName,
	}); err != nil {
		return nil, err
	}

	// Валидация длины полей
	if err := h.validator.ValidateStringLength(req.ProtoContent, "proto_content", 10, 100000); err != nil {
		return nil, h.LogError(ctx, err, "ParseProto", req.FileName)
	}

	if err := h.validator.ValidateStringLength(req.FileName, "file_name", 1, 255); err != nil {
		return nil, h.LogError(ctx, err, "ParseProto", req.FileName)
	}

	// Парсинг proto файла
	serviceInfo, isValid, warnings, err := h.forgeService.ParseProto(ctx, req.ProtoContent, req.FileName)
	if err != nil {
		return nil, h.LogError(ctx, err, "ParseProto", req.FileName)
	}

	// Конвертация в protobuf
	protoServiceInfo := h.convertForgeServiceInfoToProto(serviceInfo)

	h.LogOperationSuccess(ctx, "ParseProto", map[string]interface{}{
		"file_name":    req.FileName,
		"is_valid":     isValid,
		"warnings_count": len(warnings),
		"methods_count": len(protoServiceInfo.Methods),
	})

	return &forgev1.ParseProtoResponse{
		ServiceInfo: protoServiceInfo,
		IsValid:     isValid,
		Warnings:    warnings,
	}, nil
}

// GenerateConfig генерирует конфигурацию проверки из .proto файла
func (h *ForgeHandler) GenerateConfig(ctx context.Context, req *forgev1.GenerateConfigRequest) (*forgev1.GenerateConfigResponse, error) {
	h.LogOperationStart(ctx, "GenerateConfig", map[string]interface{}{
		"proto_length": len(req.ProtoContent),
		"has_options":  req.Options != nil,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "GenerateConfig", map[string]string{
		"proto_content": req.ProtoContent,
	}); err != nil {
		return nil, err
	}

	// Валидация длины proto контента
	if err := h.validator.ValidateStringLength(req.ProtoContent, "proto_content", 10, 100000); err != nil {
		return nil, h.LogError(ctx, err, "GenerateConfig", "")
	}

	// Валидация опций если они есть
	if req.Options != nil {
		if err := h.validateConfigOptions(req.Options); err != nil {
			return nil, h.LogError(ctx, err, "GenerateConfig", "")
		}
	}

	// Конвертация опций
	options := h.convertConfigOptionsFromProto(req.Options)

	// Генерация конфигурации
	configYaml, checkConfig, err := h.forgeService.GenerateConfig(ctx, req.ProtoContent, options)
	if err != nil {
		return nil, h.LogError(ctx, err, "GenerateConfig", "")
	}

	// Конвертация в protobuf
	protoCheckConfig := h.convertCheckConfigToProto(checkConfig)

	h.LogOperationSuccess(ctx, "GenerateConfig", map[string]interface{}{
		"config_length":  len(configYaml),
		"has_check_config": protoCheckConfig != nil,
	})

	return &forgev1.GenerateConfigResponse{
		ConfigYaml:  configYaml,
		CheckConfig: protoCheckConfig,
	}, nil
}

// GenerateCode генерирует код для проверки gRPC методов
func (h *ForgeHandler) GenerateCode(ctx context.Context, req *forgev1.GenerateCodeRequest) (*forgev1.GenerateCodeResponse, error) {
	h.LogOperationStart(ctx, "GenerateCode", map[string]interface{}{
		"proto_length": len(req.ProtoContent),
		"has_options":  req.Options != nil,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "GenerateCode", map[string]string{
		"proto_content": req.ProtoContent,
	}); err != nil {
		return nil, err
	}

	// Валидация длины proto контента
	if err := h.validator.ValidateStringLength(req.ProtoContent, "proto_content", 10, 100000); err != nil {
		return nil, h.LogError(ctx, err, "GenerateCode", "")
	}

	// Валидация опций если они есть
	if req.Options != nil {
		if err := h.validateCodeOptions(req.Options); err != nil {
			return nil, h.LogError(ctx, err, "GenerateCode", "")
		}
	}

	// Конвертация опций
	options := h.convertCodeOptionsFromProto(req.Options)

	// Генерация кода
	code, filename, language, err := h.forgeService.GenerateCode(ctx, req.ProtoContent, options)
	if err != nil {
		return nil, h.LogError(ctx, err, "GenerateCode", "")
	}

	h.LogOperationSuccess(ctx, "GenerateCode", map[string]interface{}{
		"code_length": len(code),
		"filename":   filename,
		"language":   language,
	})

	return &forgev1.GenerateCodeResponse{
		Code:     code,
		Filename: filename,
		Language: language,
	}, nil
}

// ValidateProto проверяет валидность .proto файла
func (h *ForgeHandler) ValidateProto(ctx context.Context, req *forgev1.ValidateProtoRequest) (*forgev1.ValidateProtoResponse, error) {
	h.LogOperationStart(ctx, "ValidateProto", map[string]interface{}{
		"proto_length": len(req.ProtoContent),
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "ValidateProto", map[string]string{
		"proto_content": req.ProtoContent,
	}); err != nil {
		return nil, err
	}

	// Валидация длины proto контента
	if err := h.validator.ValidateStringLength(req.ProtoContent, "proto_content", 10, 100000); err != nil {
		return nil, h.LogError(ctx, err, "ValidateProto", "")
	}

	// Валидация proto файла
	isValid, errors, warnings, err := h.forgeService.ValidateProto(ctx, req.ProtoContent)
	if err != nil {
		return nil, h.LogError(ctx, err, "ValidateProto", "")
	}

	h.LogOperationSuccess(ctx, "ValidateProto", map[string]interface{}{
		"is_valid":     isValid,
		"errors_count":  len(errors),
		"warnings_count": len(warnings),
	})

	return &forgev1.ValidateProtoResponse{
		IsValid:  isValid,
		Errors:   errors,
		Warnings: warnings,
	}, nil
}

// Вспомогательные методы конвертации

// convertForgeServiceInfoToProto конвертирует ForgeServiceInfo в protobuf
func (h *ForgeHandler) convertForgeServiceInfoToProto(info *service.ForgeServiceInfo) *forgev1.ServiceInfo {
	if info == nil {
		return nil
	}

	methods := make([]*forgev1.MethodInfo, len(info.Methods))
	for i, method := range info.Methods {
		methods[i] = &forgev1.MethodInfo{
			Name:       method.Name,
			InputType:  method.InputType,
			OutputType: method.OutputType,
		}
	}

	messages := make([]*forgev1.MessageInfo, len(info.Messages))
	for i, msg := range info.Messages {
		fields := make([]*forgev1.FieldInfo, len(msg.Fields))
		for j, field := range msg.Fields {
			fields[j] = &forgev1.FieldInfo{
				Name:     field.Name,
				Type:     field.Type,
				Number:   int32(field.Number),
				Repeated: field.Repeated,
			}
		}
		messages[i] = &forgev1.MessageInfo{
			Name:   msg.Name,
			Fields: fields,
		}
	}

	return &forgev1.ServiceInfo{
		PackageName: info.PackageName,
		ServiceName: info.ServiceName,
		Methods:     methods,
		Messages:    messages,
	}
}

// convertConfigOptionsFromProto конвертирует protobuf ConfigOptions в доменную модель
func (h *ForgeHandler) convertConfigOptionsFromProto(proto *forgev1.ConfigOptions) *service.ConfigOptions {
	if proto == nil {
		return nil
	}

	return &service.ConfigOptions{
		TargetHost:   proto.TargetHost,
		TargetPort:   int(proto.TargetPort),
		CheckInterval: int(proto.CheckInterval),
		Timeout:      int(proto.Timeout),
		TenantID:     proto.TenantId,
		Metadata:     proto.Metadata,
	}
}

// convertCheckConfigToProto конвертирует CheckConfig в protobuf
func (h *ForgeHandler) convertCheckConfigToProto(config *service.CheckConfig) *forgev1.CheckConfig {
	if config == nil {
		return nil
	}

	var checkType forgev1.CheckType
	switch config.Type {
	case "http":
		checkType = forgev1.CheckType_CHECK_TYPE_HTTP
	case "grpc":
		checkType = forgev1.CheckType_CHECK_TYPE_GRPC
	case "graphql":
		checkType = forgev1.CheckType_CHECK_TYPE_GRAPHQL
	default:
		checkType = forgev1.CheckType_CHECK_TYPE_UNSPECIFIED
	}

	return &forgev1.CheckConfig{
		Name:     config.Name,
		Type:     checkType,
		Target:   config.Target,
		Interval: int32(config.Interval),
		Timeout:  int32(config.Timeout),
		Config:   config.Config,
	}
}

// convertCodeOptionsFromProto конвертирует protobuf CodeOptions в доменную модель
func (h *ForgeHandler) convertCodeOptionsFromProto(proto *forgev1.CodeOptions) *service.CodeOptions {
	if proto == nil {
		return nil
	}

	return &service.CodeOptions{
		Language:  proto.Language,
		Framework: proto.Framework,
		Template:  proto.Template,
	}
}

// validateConfigOptions валидирует ConfigOptions
func (h *ForgeHandler) validateConfigOptions(options *forgev1.ConfigOptions) error {
	if options.TargetHost != "" {
		if err := h.validator.ValidateStringLength(options.TargetHost, "target_host", 1, 255); err != nil {
			return err
		}
	}

	if options.TargetPort < 1 || options.TargetPort > 65535 {
		return fmt.Errorf("target_port must be between 1 and 65535")
	}

	if options.CheckInterval < 1 || options.CheckInterval > 86400 {
		return fmt.Errorf("check_interval must be between 1 and 86400 seconds")
	}

	if options.Timeout < 1 || options.Timeout > 300 {
		return fmt.Errorf("timeout must be between 1 and 300 seconds")
	}

	if options.TenantId != "" {
		if err := h.validator.ValidateStringLength(options.TenantId, "tenant_id", 1, 100); err != nil {
			return err
		}
	}

	return nil
}

// validateCodeOptions валидирует CodeOptions
func (h *ForgeHandler) validateCodeOptions(options *forgev1.CodeOptions) error {
	if options.Language != "" {
		if err := h.validator.ValidateEnum(options.Language, []string{"go", "python", "java", "typescript"}, "language"); err != nil {
			return err
		}
	}

	if options.Framework != "" {
		if err := h.validator.ValidateStringLength(options.Framework, "framework", 1, 100); err != nil {
			return err
		}
	}

	if options.Template != "" {
		if err := h.validator.ValidateStringLength(options.Template, "template", 1, 100); err != nil {
			return err
		}
	}

	return nil
}
