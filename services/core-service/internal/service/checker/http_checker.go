package checker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
)

// HTTPChecker реализует Checker для HTTP проверок
type HTTPChecker struct {
	*BaseChecker
	client    *http.Client
	logger    logger.Logger
	validator *validation.Validator
}

// HTTPValidationRule представляет правило валидации HTTP ответа
type HTTPValidationRule struct {
	Type     string      `json:"type"`     // "json_path" или "regex"
	Path     string      `json:"path"`     // JSON path или regex pattern
	Expected interface{} `json:"expected"` // ожидаемое значение
	Operator string      `json:"operator"` // "equals", "contains", "not_empty"
}

// HTTPResponseDetails представляет детальную информацию об HTTP ответе
type HTTPResponseDetails struct {
	StatusCode    int               `json:"status_code"`
	Status        string            `json:"status"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	BodySize      int64             `json:"body_size"`
	DurationMs    int64             `json:"duration_ms"`
	ContentType   string            `json:"content_type"`
	RedirectCount int               `json:"redirect_count"`
}

// NewHTTPChecker создает новый HTTP checker
func NewHTTPChecker(timeout int64, log logger.Logger) *HTTPChecker {
	return &HTTPChecker{
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

// Execute выполняет HTTP проверку
func (h *HTTPChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	// Валидация конфигурации
	if err := h.ValidateConfig(task.Config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	// Извлечение HTTP конфигурации
	httpConfig, err := task.GetHTTPConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to extract HTTP config: %w", err)
	}
	
	// Создание HTTP запроса
	req, err := h.createHTTPRequest(httpConfig)
	if err != nil {
		return h.createErrorResult(task, 0, 0, fmt.Errorf("failed to create request: %w", err)), nil
	}
	
	// Выполнение запроса с измерением времени
	startTime := time.Now()
	resp, err := h.client.Do(req)
	duration := time.Since(startTime)
	
	if err != nil {
		return h.createErrorResult(task, 0, duration.Milliseconds(), fmt.Errorf("request failed: %w", err)), nil
	}
	defer resp.Body.Close()
	
	// Чтение тела ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return h.createErrorResult(task, resp.StatusCode, duration.Milliseconds(), fmt.Errorf("failed to read response body: %w", err)), nil
	}
	
	// Создание детальной информации об ответе
	responseDetails := &HTTPResponseDetails{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    h.extractHeaders(resp.Header),
		Body:       string(body),
		BodySize:   int64(len(body)),
		DurationMs: duration.Milliseconds(),
		ContentType: resp.Header.Get("Content-Type"),
	}
	
	// Проверка статус кода
	statusSuccess := h.checkStatusCode(resp.StatusCode, httpConfig.ExpectedStatus)
	
	// Валидация тела ответа если указана
	bodyValidationSuccess := true
	var bodyValidationError error
	
	if validationRules, ok := task.Config["validation_rules"].([]interface{}); ok && len(validationRules) > 0 {
		bodyValidationSuccess, bodyValidationError = h.validateResponseBody(string(body), validationRules)
	}
	
	// Общая успешность проверки
	success := statusSuccess && bodyValidationSuccess
	
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
	result.Metadata["content_type"] = responseDetails.ContentType
	result.Metadata["body_size"] = fmt.Sprintf("%d", responseDetails.BodySize)
	result.Metadata["status"] = responseDetails.Status
	
	if !statusSuccess {
		result.Error = fmt.Sprintf("status code mismatch: expected %d, got %d", httpConfig.ExpectedStatus, resp.StatusCode)
	}
	
	if !bodyValidationSuccess && bodyValidationError != nil {
		if result.Error != "" {
			result.Error += "; "
		}
		result.Error += fmt.Sprintf("body validation failed: %s", bodyValidationError.Error())
	}
	
	return result, nil
}

// GetType возвращает тип checker'а
func (h *HTTPChecker) GetType() domain.TaskType {
	return domain.TaskTypeHTTP
}

// ValidateConfig валидирует HTTP конфигурацию
func (h *HTTPChecker) ValidateConfig(config map[string]interface{}) error {
	// Валидация обязательных полей с использованием pkg/validation
	requiredFields := map[string]string{
		"method": "HTTP method",
		"url":    "URL",
	}
	
	if err := h.validator.ValidateRequiredFields(config, requiredFields); err != nil {
		h.logger.Debug("HTTP config validation failed", logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "required fields validation failed")
	}
	
	// Валидация HTTP метода
	method := config["method"].(string)
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	if err := h.validator.ValidateEnum(method, validMethods, "method"); err != nil {
		h.logger.Debug("HTTP config validation failed: invalid method", 
			logger.String("method", method),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "invalid HTTP method")
	}
	
	// Валидация URL
	url := config["url"].(string)
	if err := h.validator.ValidateURL(url, []string{"http", "https"}); err != nil {
		h.logger.Debug("HTTP config validation failed: invalid URL", 
			logger.String("url", url),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "invalid URL format")
	}
	
	// Валидация ожидаемого статус кода
	if expectedStatus, ok := config["expected_status"]; ok {
		if status, ok := expectedStatus.(float64); !ok || status < 100 || status > 599 {
			err := fmt.Errorf("must be a valid HTTP status code (100-599)")
			h.logger.Debug("HTTP config validation failed: invalid status code", 
				logger.Float64("expected_status", status),
				logger.Error(err))
			return errors.Wrap(err, errors.ErrValidation, "invalid status code")
		}
	}
	
	// Валидация таймаута
	if timeout, ok := config["timeout"]; ok {
		if timeoutStr, ok := timeout.(string); ok {
			if err := h.validator.ValidateTimeout(30, 1, 300); err != nil {
				h.logger.Debug("HTTP config validation failed: invalid timeout", 
					logger.String("timeout", timeoutStr),
					logger.Error(err))
				return errors.Wrap(err, errors.ErrValidation, "invalid timeout value")
			}
		}
	}
	
	h.logger.Debug("HTTP config validation passed")
	return nil
}

// createHTTPRequest создает HTTP запрос из конфигурации
func (h *HTTPChecker) createHTTPRequest(config *domain.HTTPConfig) (*http.Request, error) {
	var body io.Reader
	
	// Установка тела запроса для методов, которые его поддерживают
	if config.Body != "" && (config.Method == "POST" || config.Method == "PUT" || config.Method == "PATCH") {
		body = bytes.NewBufferString(config.Body)
	}
	
	// Создание запроса
	req, err := http.NewRequest(config.Method, config.URL, body)
	if err != nil {
		return nil, err
	}
	
	// Установка заголовков
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}
	
	// Установка заголовка Content-Type если не указан
	if config.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	
	// Установка User-Agent если не указан
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "UptimePing/1.0")
	}
	
	return req, nil
}

// checkStatusCode проверяет статус код
func (h *HTTPChecker) checkStatusCode(actual, expected int) bool {
	return actual == expected
}

// validateResponseBody валидирует тело ответа
func (h *HTTPChecker) validateResponseBody(body string, rules []interface{}) (bool, error) {
	for _, ruleInterface := range rules {
		ruleMap, ok := ruleInterface.(map[string]interface{})
		if !ok {
			return false, fmt.Errorf("invalid rule format")
		}
		
		ruleType, ok := ruleMap["type"].(string)
		if !ok {
			return false, fmt.Errorf("rule type is required")
		}
		
		switch ruleType {
		case "json_path":
			success, err := h.validateJSONPath(body, ruleMap)
			if !success || err != nil {
				return false, err
			}
		case "regex":
			success, err := h.validateRegex(body, ruleMap)
			if !success || err != nil {
				return false, err
			}
		default:
			return false, fmt.Errorf("unsupported validation type: %s", ruleType)
		}
	}
	
	return true, nil
}

// validateJSONPath валидирует тело ответа используя JSON path
func (h *HTTPChecker) validateJSONPath(body string, rule map[string]interface{}) (bool, error) {
	path, ok := rule["path"].(string)
	if !ok {
		return false, fmt.Errorf("json_path rule requires 'path'")
	}
	
	expected, ok := rule["expected"]
	if !ok {
		return false, fmt.Errorf("json_path rule requires 'expected'")
	}
	
	operator, ok := rule["operator"].(string)
	if !ok {
		operator = "equals" // по умолчанию
	}
	
	// Парсинг JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(body), &jsonData); err != nil {
		return false, fmt.Errorf("invalid JSON: %w", err)
	}
	
	// Простая реализация JSON path (поддержка только базовых путей)
	result, err := h.extractJSONPath(jsonData, path)
	if err != nil {
		return false, fmt.Errorf("JSON path evaluation failed: %w", err)
	}
	
	// Сравнение результата
	return h.compareValues(result, expected, operator), nil
}

// extractJSONPath извлекает значение по простому JSON path
func (h *HTTPChecker) extractJSONPath(data interface{}, path string) (interface{}, error) {
	// Удаляем префикс "$." если есть
	path = strings.TrimPrefix(path, "$.")
	
	if path == "" {
		return data, nil
	}
	
	// Разделяем путь на части
	parts := strings.Split(path, ".")
	current := data
	
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil, fmt.Errorf("path not found: %s", part)
			}
		case []interface{}:
			// Поддержка простых индексов массивов
			if part == "0" && len(v) > 0 {
				current = v[0]
			} else {
				return nil, fmt.Errorf("array index not supported: %s", part)
			}
		default:
			return nil, fmt.Errorf("invalid path segment: %s", part)
		}
	}
	
	return current, nil
}

// validateRegex валидирует тело ответа используя регулярные выражения
func (h *HTTPChecker) validateRegex(body string, rule map[string]interface{}) (bool, error) {
	pattern, ok := rule["path"].(string) // используем 'path' для паттерна regex
	if !ok {
		return false, fmt.Errorf("regex rule requires 'path' (pattern)")
	}
	
	operator, ok := rule["operator"].(string)
	if !ok {
		operator = "contains" // по умолчанию
	}
	
	// Компиляция регулярного выражения
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	// Поиск совпадений
	matches := regex.FindAllStringSubmatch(body, -1)
	hasMatches := len(matches) > 0
	
	switch operator {
	case "contains", "equals":
		return hasMatches, nil
	case "not_contains":
		return !hasMatches, nil
	default:
		return false, fmt.Errorf("unsupported regex operator: %s", operator)
	}
}

// compareValues сравнивает значения с указанным оператором
func (h *HTTPChecker) compareValues(actual, expected interface{}, operator string) bool {
	switch operator {
	case "equals":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case "not_equals":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected)
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", actual), fmt.Sprintf("%v", expected))
	case "not_empty":
		return actual != nil && fmt.Sprintf("%v", actual) != ""
	case "empty":
		return actual == nil || fmt.Sprintf("%v", actual) == ""
	default:
		return false
	}
}

// extractHeaders извлекает заголовки ответа
func (h *HTTPChecker) extractHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// createErrorResult создает результат с ошибкой
func (h *HTTPChecker) createErrorResult(task *domain.Task, statusCode int, durationMs int64, err error) *domain.CheckResult {
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
func (h *HTTPChecker) SetTimeout(timeout time.Duration) {
	h.client.Timeout = timeout
}

// GetClient возвращает HTTP клиент для тестирования
func (h *HTTPChecker) GetClient() *http.Client {
	return h.client
}
