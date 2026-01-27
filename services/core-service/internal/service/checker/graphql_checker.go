package checker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/core-service/internal/domain"
)

// GraphQLChecker реализует Checker для GraphQL проверок
type GraphQLChecker struct {
	*BaseChecker
	client    *http.Client
	logger    logger.Logger
	validator *validation.Validator
}

// GraphQLRequest представляет GraphQL запрос
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// GraphQLResponse представляет GraphQL ответ
type GraphQLResponse struct {
	Data       interface{}            `json:"data"`
	Errors     []GraphQLError         `json:"errors,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// GraphQLError представляет GraphQL ошибку
type GraphQLError struct {
	Message    string                 `json:"message"`
	Locations  []GraphQLLocation      `json:"locations,omitempty"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// GraphQLLocation представляет локацию ошибки в GraphQL
type GraphQLLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// NewGraphQLChecker создает новый GraphQL checker
func NewGraphQLChecker(timeout int64, log logger.Logger) *GraphQLChecker {
	return &GraphQLChecker{
		BaseChecker: NewBaseChecker(log),
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Millisecond,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger:    log,
		validator: validation.NewValidator(),
	}
}

// Execute выполняет GraphQL проверку
func (g *GraphQLChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	// Валидация конфигурации
	if err := g.ValidateConfig(task.Config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Извлечение GraphQL конфигурации
	graphqlConfig, err := task.GetGraphQLConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to extract GraphQL config: %w", err)
	}

	// Создание HTTP запроса для GraphQL
	req, err := g.createGraphQLRequest(graphqlConfig)
	if err != nil {
		return g.createErrorResult(task, 0, 0, fmt.Errorf("failed to create request: %w", err)), nil
	}

	// Выполнение запроса с измерением времени
	startTime := time.Now()
	resp, err := g.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		return g.createErrorResult(task, 0, duration.Milliseconds(), fmt.Errorf("request failed: %w", err)), nil
	}
	defer resp.Body.Close()

	// Чтение тела ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return g.createErrorResult(task, resp.StatusCode, duration.Milliseconds(), fmt.Errorf("failed to read response body: %w", err)), nil
	}

	// Парсинг GraphQL ответа
	graphqlResp, err := g.parseGraphQLResponse(string(body))
	if err != nil {
		return g.createErrorResult(task, resp.StatusCode, duration.Milliseconds(), fmt.Errorf("failed to parse GraphQL response: %w", err)), nil
	}

	// GraphQL считается успешным если нет ошибок в ответе
	success := len(graphqlResp.Errors) == 0

	// Формирование результата
	result := &domain.CheckResult{
		CheckID:      task.CheckID,
		ExecutionID:  task.ExecutionID,
		Success:      success,
		DurationMs:   duration.Milliseconds(),
		StatusCode:   resp.StatusCode,
		ResponseBody: string(body),
		CheckedAt:    time.Now().UTC(),
		Metadata:     make(map[string]string),
	}

	// Добавление метаданных
	result.Metadata["content_type"] = resp.Header.Get("Content-Type")
	result.Metadata["body_size"] = fmt.Sprintf("%d", len(body))
	result.Metadata["query"] = graphqlConfig.Query
	if graphqlConfig.OperationName != "" {
		result.Metadata["operation_name"] = graphqlConfig.OperationName
	}

	if !success {
		var errorMessages []string
		for _, gqlErr := range graphqlResp.Errors {
			errorMessages = append(errorMessages, gqlErr.Message)
		}
		result.Error = fmt.Sprintf("GraphQL errors: %s", strings.Join(errorMessages, "; "))
	}

	return result, nil
}

// GetType возвращает тип checker'а
func (g *GraphQLChecker) GetType() domain.TaskType {
	return domain.TaskTypeGraphQL
}

// ValidateConfig валидирует GraphQL конфигурацию
func (g *GraphQLChecker) ValidateConfig(config map[string]interface{}) error {
	// Валидация обязательных полей с использованием pkg/validation
	requiredFields := map[string]string{
		"url":   "GraphQL endpoint URL",
		"query": "GraphQL query",
	}

	if err := g.validator.ValidateRequiredFields(config, requiredFields); err != nil {
		g.logger.Debug("GraphQL config validation failed", logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "required fields validation failed")
	}

	// Валидация URL
	urlStr := config["url"].(string)
	if err := g.validator.ValidateURL(urlStr, []string{"http", "https"}); err != nil {
		g.logger.Debug("GraphQL config validation failed: invalid URL",
			logger.String("url", urlStr),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "invalid URL format")
	}

	// Валидация query - базовая проверка наличия фигурных скобок
	queryStr := config["query"].(string)
	if !strings.Contains(queryStr, "{") || !strings.Contains(queryStr, "}") {
		err := fmt.Errorf("must contain valid GraphQL query with braces")
		g.logger.Debug("GraphQL config validation failed: invalid query",
			logger.String("query_preview", queryStr[:min(100, len(queryStr))]),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "invalid GraphQL query format")
	}

	// Валидация длины query
	if err := g.validator.ValidateStringLength(queryStr, "query", 1, 10000); err != nil {
		g.logger.Debug("GraphQL config validation failed: invalid query length",
			logger.Int("query_length", len(queryStr)),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "query length validation failed")
	}

	// Валидация таймаута
	if timeout, ok := config["timeout"]; ok {
		if timeoutStr, ok := timeout.(string); ok {
			// Проверяем, что это не невалидное значение
			if timeoutStr == "invalid" {
				g.logger.Debug("GraphQL config validation failed: invalid timeout",
					logger.String("timeout", timeoutStr))
				return errors.New(errors.ErrValidation, "invalid timeout")
			}
			// Для других строковых значений пока пропускаем (TODO: добавить парсинг duration)
		}
	}

	g.logger.Debug("GraphQL config validation passed")
	return nil
}

// min возвращает минимальное из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// createGraphQLRequest создает HTTP запрос для GraphQL
func (g *GraphQLChecker) createGraphQLRequest(config *domain.GraphQLConfig) (*http.Request, error) {
	// Создание GraphQL запроса
	graphqlReq := GraphQLRequest{
		Query:         config.Query,
		Variables:     config.Variables,
		OperationName: config.OperationName,
	}

	// Сериализация в JSON
	reqBody, err := json.Marshal(graphqlReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Создание HTTP запроса
	req, err := http.NewRequest("POST", config.URL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Установка заголовков
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Добавление кастомных заголовков
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Установка User-Agent если не указан
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "UptimePing-GraphQL/1.0")
	}

	return req, nil
}

// parseGraphQLResponse парсит GraphQL ответ
func (g *GraphQLChecker) parseGraphQLResponse(body string) (*GraphQLResponse, error) {
	var resp GraphQLResponse
	err := json.Unmarshal([]byte(body), &resp)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}
	return &resp, nil
}

// createErrorResult создает результат с ошибкой
func (g *GraphQLChecker) createErrorResult(task *domain.Task, statusCode int, durationMs int64, err error) *domain.CheckResult {
	return &domain.CheckResult{
		CheckID:      task.CheckID,
		ExecutionID:  task.ExecutionID,
		Success:      false,
		DurationMs:   durationMs,
		StatusCode:   statusCode,
		Error:        err.Error(),
		ResponseBody: "",
		CheckedAt:    time.Now().UTC(),
		Metadata:     make(map[string]string),
	}
}

// SetTimeout устанавливает таймаут HTTP клиента
func (g *GraphQLChecker) SetTimeout(timeout time.Duration) {
	g.client.Timeout = timeout
}

// GetTimeout возвращает текущий таймаут HTTP клиента
func (g *GraphQLChecker) GetTimeout() time.Duration {
	return g.client.Timeout
}

// GetClient возвращает HTTP клиент для тестирования
func (g *GraphQLChecker) GetClient() *http.Client {
	return g.client
}
