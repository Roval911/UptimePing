package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
)

// ConfigClient представляет клиент для взаимодействия с конфигурацией
type ConfigClient struct {
	baseURL    string
	logger     logger.Logger
	grpcClient ConfigClientInterface
	useGRPC    bool
}

// NewConfigClient создает новый клиент конфигурации
func NewConfigClient(baseURL string, log logger.Logger) *ConfigClient {
	return &ConfigClient{
		baseURL: baseURL,
		logger:  log,
		useGRPC: false, // По умолчанию используем mock
	}
}

// ConfigClientInterface определяет интерфейс для gRPC клиента
type ConfigClientInterface interface {
	CreateCheck(ctx context.Context, req *CheckCreateRequest) (*Check, error)
	GetCheck(ctx context.Context, id string) (*Check, error)
	RunCheck(ctx context.Context, id string) (*CheckRunResponse, error)
	GetCheckStatus(ctx context.Context, id string) (*CheckStatusResponse, error)
	GetCheckHistory(ctx context.Context, id string, page, limit int) (*CheckHistoryResponse, error)
	ListChecks(ctx context.Context, tags []string, filters map[string]interface{}, page, limit int) (*CheckListResponse, error)
	UpdateCheck(ctx context.Context, id string, req *CheckUpdateRequest) (*Check, error)
	Close() error
}

// NewConfigClientWithGRPC создает новый клиент конфигурации с gRPC
func NewConfigClientWithGRPC(baseURL, schedulerAddr, coreAddr string, log logger.Logger) (*ConfigClient, error) {
	grpcClient, err := NewGRPCClient(schedulerAddr, coreAddr, log)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания gRPC клиента: %w", err)
	}

	return &ConfigClient{
		baseURL:    baseURL,
		logger:     log,
		grpcClient: grpcClient,
		useGRPC:    true,
	}, nil
}

// Close закрывает соединения
func (c *ConfigClient) Close() error {
	if c.grpcClient != nil {
		return c.grpcClient.Close()
	}
	return nil
}

// Check представляет конфигурацию проверки
type Check struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`        // http, tcp, ping, grpc, graphql
	Target      string            `json:"target"`
	Interval    int               `json:"interval"`    // в секундах
	Timeout     int               `json:"timeout"`     // в секундах
	Enabled     bool              `json:"enabled"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CheckCreateRequest представляет запрос на создание проверки
type CheckCreateRequest struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Target   string            `json:"target"`
	Interval int               `json:"interval"`
	Timeout  int               `json:"timeout"`
	Tags     []string          `json:"tags"`
	Metadata map[string]string `json:"metadata"`
}

// CheckUpdateRequest представляет запрос на обновление проверки
type CheckUpdateRequest struct {
	Name     *string           `json:"name,omitempty"`
	Type     *string           `json:"type,omitempty"`
	Target   *string           `json:"target,omitempty"`
	Interval *int              `json:"interval,omitempty"`
	Timeout  *int              `json:"timeout,omitempty"`
	Enabled  *bool             `json:"enabled,omitempty"`
	Tags     []string          `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CheckListResponse представляет ответ со списком проверок
type CheckListResponse struct {
	Checks []Check `json:"checks"`
	Total  int     `json:"total"`
}

// CheckRunRequest представляет запрос на запуск проверки
type CheckRunRequest struct {
	CheckID string `json:"check_id"`
}

// CheckRunResponse представляет ответ запуска проверки
type CheckRunResponse struct {
	ExecutionID string    `json:"execution_id"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	StartedAt   time.Time `json:"started_at"`
}

// CheckStatusResponse представляет ответ статуса проверки
type CheckStatusResponse struct {
	CheckID      string    `json:"check_id"`
	Status       string    `json:"status"`       // pending, running, success, failed
	LastRun      time.Time `json:"last_run"`
	NextRun      time.Time `json:"next_run"`
	LastStatus   string    `json:"last_status"`
	LastMessage  string    `json:"last_message"`
	IsRunning    bool      `json:"is_running"`
}

// CheckHistoryResponse представляет ответ с историей проверок
type CheckHistoryResponse struct {
	Executions []CheckExecution `json:"executions"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
}

// CheckExecution представляет выполнение проверки
type CheckExecution struct {
	ExecutionID string    `json:"execution_id"`
	CheckID     string    `json:"check_id"`
	Status      string    `json:"status"`      // success, failed, timeout
	Message     string    `json:"message"`
	Duration    int       `json:"duration"`    // в миллисекунда
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// CreateCheck создает новую проверку
func (c *ConfigClient) CreateCheck(ctx context.Context, req *CheckCreateRequest) (*Check, error) {
	c.logger.Info("создание новой проверки", 
		logger.String("name", req.Name),
		logger.String("type", req.Type),
		logger.String("target", req.Target))

	// Валидация входных данных
	validator := &validation.Validator{}
	
	if err := validator.ValidateRequiredFields(map[string]interface{}{
		"name": req.Name,
		"type": req.Type,
		"target": req.Target,
	}, map[string]string{}); err != nil {
		c.logger.Error("ошибка валидации данных", logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "некорректные данные")
	}

	// Валидация типа проверки
	validTypes := map[string]bool{
		"http": true, "tcp": true, "ping": true, "grpc": true, "graphql": true,
	}
	if !validTypes[req.Type] {
		err := fmt.Errorf("некорректный тип проверки: %s", req.Type)
		c.logger.Error("ошибка валидации типа", logger.Error(err))
		return nil, errors.New(errors.ErrValidation, err.Error())
	}

	// Валидация интервалов
	if err := validator.ValidateInterval(int32(req.Interval), 10, 86400); err != nil {
		c.logger.Error("ошибка валидации интервала", logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "интервал должен быть от 10 до 86400 секунд")
	}

	if err := validator.ValidateTimeout(int32(req.Timeout), 1, 300); err != nil {
		c.logger.Error("ошибка валидации таймаута", logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "таймаут должен быть от 1 до 300 секунд")
	}

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.CreateCheck(ctx, req)
	}

	// Если gRPC не настроен, возвращаем ошибку
	return nil, fmt.Errorf("gRPC не настроен. Установите use_grpc: true в конфигурации")
}

// GetCheck получает проверку по ID
func (c *ConfigClient) GetCheck(ctx context.Context, checkID string) (*Check, error) {
	c.logger.Info("получение проверки", logger.String("check_id", checkID))

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.GetCheck(ctx, checkID)
	}

	// Если gRPC не настроен, возвращаем ошибку
	return nil, fmt.Errorf("gRPC не настроен. Установите use_grpc: true в конфигурации")
}

// UpdateCheck обновляет проверку
func (c *ConfigClient) UpdateCheck(ctx context.Context, checkID string, req *CheckUpdateRequest) (*Check, error) {
	c.logger.Info("обновление проверки", logger.String("check_id", checkID))

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.UpdateCheck(ctx, checkID, req)
	}

	// Если gRPC не настроен, возвращаем ошибку
	return nil, fmt.Errorf("gRPC не настроен. Установите use_grpc: true в конфигурации")
}

// ListChecks получает список проверок с фильтрацией
func (c *ConfigClient) ListChecks(ctx context.Context, tags []string, enabled *bool, page, pageSize int) (*CheckListResponse, error) {
	c.logger.Info("получение списка проверок", 
		logger.String("tags", strings.Join(tags, ",")),
		logger.Bool("enabled_filter", enabled != nil))

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		filters := make(map[string]interface{})
		if enabled != nil {
			filters["enabled"] = *enabled
		}
		return c.grpcClient.ListChecks(ctx, tags, filters, page, pageSize)
	}

	// Если gRPC не настроен, возвращаем ошибку
	return nil, fmt.Errorf("gRPC не настроен. Установите use_grpc: true в конфигурации")
}

// RunCheck запускает проверку вручную
func (c *ConfigClient) RunCheck(ctx context.Context, checkID string) (*CheckRunResponse, error) {
	c.logger.Info("запуск проверки", logger.String("check_id", checkID))

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.RunCheck(ctx, checkID)
	}

	// Если gRPC не настроен, возвращаем ошибку
	return nil, fmt.Errorf("gRPC не настроен. Установите use_grpc: true в конфигурации")
}

// GetCheckStatus получает статус проверки
func (c *ConfigClient) GetCheckStatus(ctx context.Context, checkID string) (*CheckStatusResponse, error) {
	c.logger.Info("получение статуса проверки", logger.String("check_id", checkID))

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.GetCheckStatus(ctx, checkID)
	}

	// Если gRPC не настроен, возвращаем ошибку
	return nil, fmt.Errorf("gRPC не настроен. Установите use_grpc: true в конфигурации")
}

// GetCheckHistory получает историю выполнения проверки
func (c *ConfigClient) GetCheckHistory(ctx context.Context, checkID string, page, pageSize int) (*CheckHistoryResponse, error) {
	c.logger.Info("получение истории проверки", 
		logger.String("check_id", checkID),
		logger.Int("page", page),
		logger.Int("page_size", pageSize))

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.GetCheckHistory(ctx, checkID, page, pageSize)
	}

	// Если gRPC не настроен, возвращаем ошибку
	return nil, fmt.Errorf("gRPC не настроен. Установите use_grpc: true в конфигурации")
}
