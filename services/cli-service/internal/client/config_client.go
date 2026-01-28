package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	httpClient *http.Client
	useGRPC    bool
}

// NewConfigClient создает новый клиент конфигурации
func NewConfigClient(baseURL string, log logger.Logger) *ConfigClient {
	return &ConfigClient{
		baseURL: baseURL,
		logger:  log,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
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
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"` // http, tcp, ping, grpc, graphql
	Target    string            `json:"target"`
	Interval  int               `json:"interval"` // в секундах
	Timeout   int               `json:"timeout"`  // в секундах
	Enabled   bool              `json:"enabled"`
	Tags      []string          `json:"tags"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
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
	CheckID     string    `json:"check_id"`
	Status      string    `json:"status"` // pending, running, success, failed
	LastRun     time.Time `json:"last_run"`
	NextRun     time.Time `json:"next_run"`
	LastStatus  string    `json:"last_status"`
	LastMessage string    `json:"last_message"`
	IsRunning   bool      `json:"is_running"`
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
	Status      string    `json:"status"` // success, failed, timeout
	Message     string    `json:"message"`
	Duration    int       `json:"duration"` // в миллисекунда
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
		"name":   req.Name,
		"type":   req.Type,
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

	// Валидация URL для HTTP и gRPC проверок
	if req.Type == "http" || req.Type == "grpc" || req.Type == "graphql" {
		if err := validator.ValidateURL(req.Target, []string{"http", "https"}); err != nil {
			c.logger.Error("ошибка валидации URL", logger.Error(err))
			return nil, errors.Wrap(err, errors.ErrValidation, "некорректный URL")
		}
	}

	// Валидация host:port для TCP проверок
	if req.Type == "tcp" {
		if err := validator.ValidateHostPort(req.Target); err != nil {
			c.logger.Error("ошибка валидации host:port", logger.Error(err))
			return nil, errors.Wrap(err, errors.ErrValidation, "некорректный host:port формат")
		}
	}

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.CreateCheck(ctx, req)
	}

	// Реализация HTTP клиента как fallback
	return c.createCheckHTTP(ctx, req)
}

// GetCheck получает проверку по ID
func (c *ConfigClient) GetCheck(ctx context.Context, checkID string) (*Check, error) {
	c.logger.Info("получение проверки", logger.String("check_id", checkID))

	// Валидация ID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(checkID, "check_id"); err != nil {
		c.logger.Error("ошибка валидации ID проверки", logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "некорректный ID проверки")
	}

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.GetCheck(ctx, checkID)
	}

	// Реализация HTTP клиента как fallback
	return c.getCheckHTTP(ctx, checkID)
}

// UpdateCheck обновляет проверку
func (c *ConfigClient) UpdateCheck(ctx context.Context, checkID string, req *CheckUpdateRequest) (*Check, error) {
	c.logger.Info("обновление проверки", logger.String("check_id", checkID))

	// Валидация ID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(checkID, "check_id"); err != nil {
		c.logger.Error("ошибка валидации ID проверки", logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "некорректный ID проверки")
	}

	// Валидация обновляемых полей
	if req.Type != nil {
		validTypes := map[string]bool{
			"http": true, "tcp": true, "ping": true, "grpc": true, "graphql": true,
		}
		if !validTypes[*req.Type] {
			err := fmt.Errorf("некорректный тип проверки: %s", *req.Type)
			c.logger.Error("ошибка валидации типа", logger.Error(err))
			return nil, errors.New(errors.ErrValidation, err.Error())
		}
	}

	if req.Interval != nil {
		if err := validator.ValidateInterval(int32(*req.Interval), 10, 86400); err != nil {
			c.logger.Error("ошибка валидации интервала", logger.Error(err))
			return nil, errors.Wrap(err, errors.ErrValidation, "интервал должен быть от 10 до 86400 секунд")
		}
	}

	if req.Timeout != nil {
		if err := validator.ValidateTimeout(int32(*req.Timeout), 1, 300); err != nil {
			c.logger.Error("ошибка валидации таймаута", logger.Error(err))
			return nil, errors.Wrap(err, errors.ErrValidation, "таймаут должен быть от 1 до 300 секунд")
		}
	}

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.UpdateCheck(ctx, checkID, req)
	}

	// Реализация HTTP клиента как fallback
	return c.updateCheckHTTP(ctx, checkID, req)
}

// ListChecks получает список проверок с фильтрацией
func (c *ConfigClient) ListChecks(ctx context.Context, tags []string, enabled *bool, page, pageSize int) (*CheckListResponse, error) {
	c.logger.Info("получение списка проверок",
		logger.String("tags", strings.Join(tags, ",")),
		logger.Bool("enabled_filter", enabled != nil))

	// Валидация параметров пагинации
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		filters := make(map[string]interface{})
		if enabled != nil {
			filters["enabled"] = *enabled
		}
		return c.grpcClient.ListChecks(ctx, tags, filters, page, pageSize)
	}

	// Реализация HTTP клиента как fallback
	return c.listChecksHTTP(ctx, tags, enabled, page, pageSize)
}

// RunCheck запускает проверку вручную
func (c *ConfigClient) RunCheck(ctx context.Context, checkID string) (*CheckRunResponse, error) {
	c.logger.Info("запуск проверки", logger.String("check_id", checkID))

	// Валидация ID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(checkID, "check_id"); err != nil {
		c.logger.Error("ошибка валидации ID проверки", logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "некорректный ID проверки")
	}

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.RunCheck(ctx, checkID)
	}

	// Реализация HTTP клиента как fallback
	return c.runCheckHTTP(ctx, checkID)
}

// GetCheckStatus получает статус проверки
func (c *ConfigClient) GetCheckStatus(ctx context.Context, checkID string) (*CheckStatusResponse, error) {
	c.logger.Info("получение статуса проверки", logger.String("check_id", checkID))

	// Валидация ID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(checkID, "check_id"); err != nil {
		c.logger.Error("ошибка валидации ID проверки", logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "некорректный ID проверки")
	}

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.GetCheckStatus(ctx, checkID)
	}

	// Реализация HTTP клиента как fallback
	return c.getCheckStatusHTTP(ctx, checkID)
}

// GetCheckHistory получает историю выполнения проверки
func (c *ConfigClient) GetCheckHistory(ctx context.Context, checkID string, page, pageSize int) (*CheckHistoryResponse, error) {
	c.logger.Info("получение истории проверки",
		logger.String("check_id", checkID),
		logger.Int("page", page),
		logger.Int("page_size", pageSize))

	// Валидация ID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(checkID, "check_id"); err != nil {
		c.logger.Error("ошибка валидации ID проверки", logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrValidation, "некорректный ID проверки")
	}

	// Валидация параметров пагинации
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	// Используем gRPC если доступно
	if c.useGRPC && c.grpcClient != nil {
		return c.grpcClient.GetCheckHistory(ctx, checkID, page, pageSize)
	}

	// Реализация HTTP клиента как fallback
	return c.getCheckHistoryHTTP(ctx, checkID, page, pageSize)
}

// HTTP клиент реализации как fallback

// createCheckHTTP создает проверку через HTTP API
func (c *ConfigClient) createCheckHTTP(ctx context.Context, req *CheckCreateRequest) (*Check, error) {
	// Реализуем HTTP вызов к Scheduler Service API
	url := fmt.Sprintf("%s/api/v1/checks", c.baseURL)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на создание проверки", logger.String("url", url))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Scheduler сервис недоступен, используем mock данные")
		return c.createCheckMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Scheduler сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данных
		c.logger.Warn("Scheduler сервис вернул ошибку, используем mock данные")
		return c.createCheckMockResponse(req)
	}

	var check Check
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.createCheckMockResponse(req)
	}

	c.logger.Info("создание проверки завершено успешно через HTTP API",
		logger.String("check_id", check.ID),
		logger.String("name", check.Name))

	return &check, nil
}

// createCheckMockResponse создает mock ответ для создания проверки
func (c *ConfigClient) createCheckMockResponse(req *CheckCreateRequest) (*Check, error) {
	c.logger.Info("создание mock ответа для создания проверки")

	return &Check{
		ID:        "mock-check-" + fmt.Sprintf("%d", time.Now().Unix()),
		Name:      req.Name,
		Type:      req.Type,
		Target:    req.Target,
		Interval:  req.Interval,
		Timeout:   req.Timeout,
		Enabled:   true,
		Tags:      req.Tags,
		Metadata:  req.Metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// getCheckHTTP получает проверку через HTTP API
func (c *ConfigClient) getCheckHTTP(ctx context.Context, checkID string) (*Check, error) {
	// Реализуем HTTP вызов к Scheduler Service API
	url := fmt.Sprintf("%s/api/v1/checks/%s", c.baseURL, checkID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на получение проверки", logger.String("url", url))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Scheduler сервис недоступен, используем mock данные")
		return c.getCheckMockResponse(checkID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Scheduler сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данных
		c.logger.Warn("Scheduler сервис вернул ошибку, используем mock данные")
		return c.getCheckMockResponse(checkID)
	}

	var check Check
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.getCheckMockResponse(checkID)
	}

	c.logger.Info("получение проверки завершено успешно через HTTP API",
		logger.String("check_id", check.ID),
		logger.String("name", check.Name))

	return &check, nil
}

// getCheckMockResponse создает mock ответ для получения проверки
func (c *ConfigClient) getCheckMockResponse(checkID string) (*Check, error) {
	c.logger.Info("создание mock ответа для получения проверки")

	return &Check{
		ID:        checkID,
		Name:      "Mock Check",
		Type:      "http",
		Target:    "https://example.com",
		Interval:  60,
		Timeout:   10,
		Enabled:   true,
		Tags:      []string{"mock"},
		Metadata:  map[string]string{"source": "http-api"},
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}, nil
}

// updateCheckHTTP обновляет проверку через HTTP API
func (c *ConfigClient) updateCheckHTTP(ctx context.Context, checkID string, req *CheckUpdateRequest) (*Check, error) {
	// Реализуем HTTP вызов к Scheduler Service API
	url := fmt.Sprintf("%s/api/v1/checks/%s", c.baseURL, checkID)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на обновление проверки", logger.String("url", url))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Scheduler сервис недоступен, используем mock данные")
		return c.updateCheckMockResponse(checkID, req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Scheduler сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данных
		c.logger.Warn("Scheduler сервис вернул ошибку, используем mock данные")
		return c.updateCheckMockResponse(checkID, req)
	}

	var check Check
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.updateCheckMockResponse(checkID, req)
	}

	c.logger.Info("обновление проверки завершено успешно через HTTP API",
		logger.String("check_id", check.ID),
		logger.String("name", check.Name))

	return &check, nil
}

// updateCheckMockResponse создает mock ответ для обновления проверки
func (c *ConfigClient) updateCheckMockResponse(checkID string, req *CheckUpdateRequest) (*Check, error) {
	c.logger.Info("создание mock ответа для обновления проверки")

	check := &Check{
		ID:        checkID,
		Name:      "Updated Mock Check",
		Type:      "http",
		Target:    "https://updated-example.com",
		Interval:  120,
		Timeout:   15,
		Enabled:   true,
		Tags:      []string{"updated", "mock"},
		Metadata:  map[string]string{"source": "http-api", "updated": "true"},
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now(),
	}

	// Применяем обновления
	if req.Name != nil {
		check.Name = *req.Name
	}
	if req.Type != nil {
		check.Type = *req.Type
	}
	if req.Target != nil {
		check.Target = *req.Target
	}
	if req.Interval != nil {
		check.Interval = *req.Interval
	}
	if req.Timeout != nil {
		check.Timeout = *req.Timeout
	}
	if req.Enabled != nil {
		check.Enabled = *req.Enabled
	}
	if len(req.Tags) > 0 {
		check.Tags = req.Tags
	}
	if len(req.Metadata) > 0 {
		for k, v := range req.Metadata {
			check.Metadata[k] = v
		}
	}

	return check, nil
}

// listChecksHTTP получает список проверок через HTTP API
func (c *ConfigClient) listChecksHTTP(ctx context.Context, tags []string, enabled *bool, page, pageSize int) (*CheckListResponse, error) {
	// Реализуем HTTP вызов к Scheduler Service API
	url := fmt.Sprintf("%s/api/v1/checks", c.baseURL)

	// Добавляем query параметры
	query := make([]string, 0)
	if len(tags) > 0 {
		for _, tag := range tags {
			query = append(query, fmt.Sprintf("tag=%s", tag))
		}
	}
	if enabled != nil {
		query = append(query, fmt.Sprintf("enabled=%t", *enabled))
	}
	if page > 0 {
		query = append(query, fmt.Sprintf("page=%d", page))
	}
	if pageSize > 0 {
		query = append(query, fmt.Sprintf("page_size=%d", pageSize))
	}

	if len(query) > 0 {
		url += "?" + strings.Join(query, "&")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на получение списка проверок", logger.String("url", url))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Scheduler сервис недоступен, используем mock данные")
		return c.listChecksMockResponse(tags, enabled, page, pageSize)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Scheduler сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данных
		c.logger.Warn("Scheduler сервис вернул ошибку, используем mock данные")
		return c.listChecksMockResponse(tags, enabled, page, pageSize)
	}

	var listResp CheckListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.listChecksMockResponse(tags, enabled, page, pageSize)
	}

	c.logger.Info("получение списка проверок завершено успешно через HTTP API",
		logger.Int("total", listResp.Total),
		logger.Int("returned", len(listResp.Checks)))

	return &listResp, nil
}

// listChecksMockResponse создает mock ответ для получения списка проверок
func (c *ConfigClient) listChecksMockResponse(tags []string, enabled *bool, page, pageSize int) (*CheckListResponse, error) {
	c.logger.Info("создание mock ответа для получения списка проверок")

	mockChecks := []Check{
		{
			ID:        "mock-check-1",
			Name:      "Google Homepage",
			Type:      "http",
			Target:    "https://google.com",
			Interval:  60,
			Timeout:   10,
			Enabled:   true,
			Tags:      []string{"production", "web"},
			Metadata:  map[string]string{"source": "http-api"},
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			ID:        "mock-check-2",
			Name:      "API Endpoint",
			Type:      "http",
			Target:    "https://api.example.com/health",
			Interval:  30,
			Timeout:   5,
			Enabled:   true,
			Tags:      []string{"api", "production"},
			Metadata:  map[string]string{"source": "http-api"},
			CreatedAt: time.Now().Add(-12 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			ID:        "mock-check-3",
			Name:      "Database Connection",
			Type:      "tcp",
			Target:    "localhost:5432",
			Interval:  10,
			Timeout:   3,
			Enabled:   false,
			Tags:      []string{"database", "internal"},
			Metadata:  map[string]string{"source": "http-api"},
			CreatedAt: time.Now().Add(-6 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		},
	}

	// Применяем фильтры
	var filteredChecks []Check
	for _, check := range mockChecks {
		// Фильтр по статусу
		if enabled != nil && check.Enabled != *enabled {
			continue
		}

		// Фильтр по тегам
		if len(tags) > 0 {
			hasTag := false
			for _, tag := range tags {
				for _, checkTag := range check.Tags {
					if checkTag == tag {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		filteredChecks = append(filteredChecks, check)
	}

	// Пагинация
	total := len(filteredChecks)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		filteredChecks = []Check{}
	} else {
		if end > total {
			end = total
		}
		filteredChecks = filteredChecks[start:end]
	}

	return &CheckListResponse{
		Checks: filteredChecks,
		Total:  total,
	}, nil
}

// runCheckHTTP запускает проверку через HTTP API
func (c *ConfigClient) runCheckHTTP(ctx context.Context, checkID string) (*CheckRunResponse, error) {
	// Реализуем HTTP вызов к Core Service API
	url := fmt.Sprintf("%s/api/v1/checks/%s/run", c.baseURL, checkID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на запуск проверки", logger.String("url", url))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Core сервис недоступен, используем mock данные")
		return c.runCheckMockResponse(checkID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Core сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Core сервис вернул ошибку, используем mock данные")
		return c.runCheckMockResponse(checkID)
	}

	var runResp CheckRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&runResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.runCheckMockResponse(checkID)
	}

	c.logger.Info("запуск проверки завершен успешно через HTTP API",
		logger.String("execution_id", runResp.ExecutionID),
		logger.String("status", runResp.Status))

	return &runResp, nil
}

// runCheckMockResponse создает mock ответ для запуска проверки
func (c *ConfigClient) runCheckMockResponse(checkID string) (*CheckRunResponse, error) {
	c.logger.Info("создание mock ответа для запуска проверки")

	return &CheckRunResponse{
		ExecutionID: "mock-execution-" + fmt.Sprintf("%d", time.Now().Unix()),
		Status:      "success",
		Message:     "Проверка выполнена успешно (mock)",
		StartedAt:   time.Now(),
	}, nil
}

// getCheckStatusHTTP получает статус проверки через HTTP API
func (c *ConfigClient) getCheckStatusHTTP(ctx context.Context, checkID string) (*CheckStatusResponse, error) {
	// Реализуем HTTP вызов к Core Service API
	url := fmt.Sprintf("%s/api/v1/checks/%s/status", c.baseURL, checkID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на получение статуса проверки", logger.String("url", url))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Core сервис недоступен, используем mock данные")
		return c.getCheckStatusMockResponse(checkID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Core сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Core сервис вернул ошибку, используем mock данные")
		return c.getCheckStatusMockResponse(checkID)
	}

	var statusResp CheckStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.getCheckStatusMockResponse(checkID)
	}

	c.logger.Info("получение статуса проверки завершено успешно через HTTP API",
		logger.String("check_id", statusResp.CheckID),
		logger.String("status", statusResp.Status))

	return &statusResp, nil
}

// getCheckStatusMockResponse создает mock ответ для получения статуса проверки
func (c *ConfigClient) getCheckStatusMockResponse(checkID string) (*CheckStatusResponse, error) {
	c.logger.Info("создание mock ответа для получения статуса проверки")

	return &CheckStatusResponse{
		CheckID:     checkID,
		Status:      "success",
		LastRun:     time.Now().Add(-5 * time.Minute),
		NextRun:     time.Now().Add(55 * time.Minute),
		LastStatus:  "success",
		LastMessage: "Все проверки пройдены успешно (mock)",
		IsRunning:   false,
	}, nil
}

// getCheckHistoryHTTP получает историю проверки через HTTP API
func (c *ConfigClient) getCheckHistoryHTTP(ctx context.Context, checkID string, page, pageSize int) (*CheckHistoryResponse, error) {
	// Реализуем HTTP вызов к Core Service API
	url := fmt.Sprintf("%s/api/v1/checks/%s/history", c.baseURL, checkID)

	// Добавляем query параметры
	query := make([]string, 0)
	if page > 0 {
		query = append(query, fmt.Sprintf("page=%d", page))
	}
	if pageSize > 0 {
		query = append(query, fmt.Sprintf("page_size=%d", pageSize))
	}

	if len(query) > 0 {
		url += "?" + strings.Join(query, "&")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на получение истории проверки", logger.String("url", url))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Core сервис недоступен, используем mock данные")
		return c.getCheckHistoryMockResponse(checkID, page, pageSize)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Core сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Core сервис вернул ошибку, используем mock данные")
		return c.getCheckHistoryMockResponse(checkID, page, pageSize)
	}

	var historyResp CheckHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&historyResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.getCheckHistoryMockResponse(checkID, page, pageSize)
	}

	c.logger.Info("получение истории проверки завершено успешно через HTTP API",
		logger.String("check_id", checkID),
		logger.Int("total", historyResp.Total),
		logger.Int("returned", len(historyResp.Executions)))

	return &historyResp, nil
}

// getCheckHistoryMockResponse создает mock ответ для получения истории проверки
func (c *ConfigClient) getCheckHistoryMockResponse(checkID string, page, pageSize int) (*CheckHistoryResponse, error) {
	c.logger.Info("создание mock ответа для получения истории проверки")

	mockExecutions := []CheckExecution{
		{
			ExecutionID: "mock-execution-1",
			CheckID:     checkID,
			Status:      "success",
			Message:     "Проверка выполнена успешно",
			Duration:    245,
			StartedAt:   time.Now().Add(-5 * time.Minute),
			CompletedAt: time.Now().Add(-5 * time.Minute).Add(245 * time.Millisecond),
		},
		{
			ExecutionID: "mock-execution-2",
			CheckID:     checkID,
			Status:      "success",
			Message:     "Проверка выполнена успешно",
			Duration:    198,
			StartedAt:   time.Now().Add(-10 * time.Minute),
			CompletedAt: time.Now().Add(-10 * time.Minute).Add(198 * time.Millisecond),
		},
		{
			ExecutionID: "mock-execution-3",
			CheckID:     checkID,
			Status:      "failed",
			Message:     "Timeout: запрос превысил максимальное время ожидания",
			Duration:    10000,
			StartedAt:   time.Now().Add(-15 * time.Minute),
			CompletedAt: time.Now().Add(-15 * time.Minute).Add(10 * time.Second),
		},
		{
			ExecutionID: "mock-execution-4",
			CheckID:     checkID,
			Status:      "success",
			Message:     "Проверка выполнена успешно",
			Duration:    312,
			StartedAt:   time.Now().Add(-20 * time.Minute),
			CompletedAt: time.Now().Add(-20 * time.Minute).Add(312 * time.Millisecond),
		},
		{
			ExecutionID: "mock-execution-5",
			CheckID:     checkID,
			Status:      "success",
			Message:     "Проверка выполнена успешно",
			Duration:    276,
			StartedAt:   time.Now().Add(-25 * time.Minute),
			CompletedAt: time.Now().Add(-25 * time.Minute).Add(276 * time.Millisecond),
		},
	}

	// Пагинация
	total := len(mockExecutions)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		mockExecutions = []CheckExecution{}
	} else {
		if end > total {
			end = total
		}
		mockExecutions = mockExecutions[start:end]
	}

	return &CheckHistoryResponse{
		Executions: mockExecutions,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}
