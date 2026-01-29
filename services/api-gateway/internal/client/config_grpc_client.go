package client

import (
	"context"
	"fmt"
	"time"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/config"
)

// ConfigClient клиент для управления конфигурацией
// Использует pkg/config напрямую, без gRPC
type ConfigClient struct {
	config      *config.Config
	baseHandler *grpcBase.BaseHandler
}

// NewConfigClient создает новый клиент для конфигурации
func NewConfigClient(timeout time.Duration, logger logger.Logger) (*ConfigClient, error) {
	// Инициализируем BaseHandler
	baseHandler := grpcBase.NewBaseHandler(logger)

	// Загружаем конфигурацию
	cfg, err := config.LoadConfig("")
	if err != nil {
		baseHandler.LogError(context.Background(), err, "config_load_failed", "")
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Логируем успешную загрузку
	baseHandler.LogOperationSuccess(context.Background(), "config_load_success", map[string]interface{}{
		"environment": cfg.Environment,
		"server_port": cfg.Server.Port,
	})

	return &ConfigClient{
		config:      cfg,
		baseHandler: baseHandler,
	}, nil
}

// Close закрывает соединение (для ConfigClient не требуется)
func (c *ConfigClient) Close() error {
	return nil
}

// GetConfig возвращает текущую конфигурацию
func (c *ConfigClient) GetConfig(ctx context.Context) *config.Config {
	c.baseHandler.LogOperationSuccess(ctx, "config_retrieved", map[string]interface{}{
		"environment": c.config.Environment,
	})
	return c.config
}

// UpdateConfig обновляет конфигурацию
func (c *ConfigClient) UpdateConfig(ctx context.Context, newConfig *config.Config) error {
	c.config = newConfig
	c.baseHandler.LogOperationSuccess(ctx, "config_updated", map[string]interface{}{
		"environment": c.config.Environment,
	})
	return nil
}
