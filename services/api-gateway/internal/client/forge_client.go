package client

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"UptimePingPlatform/gen/go/proto/api/forge/v1"
	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/logger"
)

// ForgeClient интерфейс для клиента forge
type ForgeClient interface {
	ParseProto(ctx context.Context, protoContent, fileName string) (*v1.ParseProtoResponse, error)
	GenerateConfig(ctx context.Context, protoContent string, options *v1.ConfigOptions) (*v1.GenerateConfigResponse, error)
	GenerateCode(ctx context.Context, protoContent string, options *v1.CodeOptions) (*v1.GenerateCodeResponse, error)
	ValidateProto(ctx context.Context, protoContent string) (*v1.ValidateProtoResponse, error)
	Close() error
}

// GrpcForgeClient реализация ForgeClient с использованием gRPC
type GrpcForgeClient struct {
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewGrpcForgeClient создает новый экземпляр GrpcForgeClient
func NewGrpcForgeClient(cfg *config.Config, log logger.Logger) (*GrpcForgeClient, error) {
	// Настройка keepalive
	keepAliveParams := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             2 * time.Second,
		PermitWithoutStream: true,
	}

	// Подключение с опциями
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepAliveParams),
	}

	// Формируем адрес сервиса - преобразуем порт в строку
	address := cfg.ForgeService.Host + ":" + strconv.Itoa(cfg.ForgeService.Port)

	log.Info("Connecting to forge service",
		logger.String("address", address),
		logger.String("host", cfg.ForgeService.Host),
		logger.Int("port", cfg.ForgeService.Port),
	)

	// Создание connection
	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		log.Error("Failed to connect to forge service",
			logger.String("error", err.Error()),
			logger.String("address", address),
		)
		return nil, err
	}

	log.Info("Successfully connected to forge service",
		logger.String("address", address),
	)

	return &GrpcForgeClient{
		conn:   conn,
		logger: log,
	}, nil
}

// ParseProto парсит proto файл
func (c *GrpcForgeClient) ParseProto(ctx context.Context, protoContent, fileName string) (*v1.ParseProtoResponse, error) {
	// Создаем клиент
	client := v1.NewForgeServiceClient(c.conn)

	// Создаем запрос согласно proto файлу
	request := &v1.ParseProtoRequest{
		ProtoContent: protoContent,
		FileName:     fileName,
	}

	// Выполняем запрос
	response, err := client.ParseProto(ctx, request)
	if err != nil {
		c.logger.Error("Failed to parse proto",
			logger.String("error", err.Error()),
			logger.Int("proto_length", len(protoContent)),
			logger.String("file_name", fileName),
		)
		return nil, err
	}

	c.logger.Debug("Successfully parsed proto",
		logger.String("service_name", response.ServiceInfo.ServiceName),
		logger.Int("method_count", len(response.ServiceInfo.Methods)),
		logger.Bool("is_valid", response.IsValid),
	)

	return response, nil
}

// GenerateConfig генерирует конфигурацию
func (c *GrpcForgeClient) GenerateConfig(ctx context.Context, protoContent string, options *v1.ConfigOptions) (*v1.GenerateConfigResponse, error) {
	// Создаем клиент
	client := v1.NewForgeServiceClient(c.conn)

	// Создаем запрос согласно proto файлу
	request := &v1.GenerateConfigRequest{
		ProtoContent: protoContent,
		Options:      options,
	}

	// Выполняем запрос
	response, err := client.GenerateConfig(ctx, request)
	if err != nil {
		c.logger.Error("Failed to generate config",
			logger.String("error", err.Error()),
			logger.Int("proto_length", len(protoContent)),
			logger.Any("options", options),
		)
		return nil, err
	}

	c.logger.Debug("Successfully generated config",
		logger.String("check_name", response.CheckConfig.Name),
		logger.String("check_type", response.CheckConfig.Type.String()),
	)

	return response, nil
}

// GenerateCode генерирует код
func (c *GrpcForgeClient) GenerateCode(ctx context.Context, protoContent string, options *v1.CodeOptions) (*v1.GenerateCodeResponse, error) {
	// Создаем клиент
	client := v1.NewForgeServiceClient(c.conn)

	// Создаем запрос согласно proto файлу
	request := &v1.GenerateCodeRequest{
		ProtoContent: protoContent,
		Options:      options,
	}

	// Выполняем запрос
	response, err := client.GenerateCode(ctx, request)
	if err != nil {
		c.logger.Error("Failed to generate code",
			logger.String("error", err.Error()),
			logger.Int("proto_length", len(protoContent)),
			logger.Any("options", options),
		)
		return nil, err
	}

	c.logger.Debug("Successfully generated code",
		logger.String("filename", response.Filename),
		logger.String("language", response.Language),
	)

	return response, nil
}

// ValidateProto валидирует proto файл
func (c *GrpcForgeClient) ValidateProto(ctx context.Context, protoContent string) (*v1.ValidateProtoResponse, error) {
	// Создаем клиент
	client := v1.NewForgeServiceClient(c.conn)

	// Создаем запрос согласно proto файлу
	request := &v1.ValidateProtoRequest{
		ProtoContent: protoContent,
	}

	// Выполняем запрос
	response, err := client.ValidateProto(ctx, request)
	if err != nil {
		c.logger.Error("Failed to validate proto",
			logger.String("error", err.Error()),
			logger.Int("proto_length", len(protoContent)),
		)
		return nil, err
	}

	c.logger.Debug("Successfully validated proto",
		logger.Bool("is_valid", response.IsValid),
		logger.Int("error_count", len(response.Errors)),
		logger.Int("warning_count", len(response.Warnings)),
	)

	return response, nil
}

// Close закрывает соединение
func (c *GrpcForgeClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing forge client connection")
		return c.conn.Close()
	}
	return nil
}
