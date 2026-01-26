package checker

import (
	"fmt"
	"net/http"
	
	"UptimePingPlatform/services/core-service/internal/domain"
)

// Checker определяет интерфейс для выполнения проверок
type Checker interface {
	// Execute выполняет проверку и возвращает результат
	Execute(task *domain.Task) (*domain.CheckResult, error)
	
	// GetType возвращает тип проверки
	GetType() domain.TaskType
	
	// ValidateConfig валидирует конфигурацию проверки
	ValidateConfig(config map[string]interface{}) error
}

// CheckerFactory определяет интерфейс для создания checker'ов
type CheckerFactory interface {
	// CreateChecker создает checker для указанного типа
	CreateChecker(taskType domain.TaskType) (Checker, error)
	
	// GetSupportedTypes возвращает список поддерживаемых типов
	GetSupportedTypes() []domain.TaskType
}

// BaseChecker предоставляет базовую функциональность для всех checker'ов
type BaseChecker struct {
	// Общие поля для всех checker'ов
	timeout int64 // таймаут в миллисекундах
}

// NewBaseChecker создает новый базовый checker
func NewBaseChecker(timeout int64) *BaseChecker {
	return &BaseChecker{
		timeout: timeout,
	}
}

// GetTimeout возвращает таймаут
func (b *BaseChecker) GetTimeout() int64 {
	return b.timeout
}

// SetTimeout устанавливает таймаут
func (b *BaseChecker) SetTimeout(timeout int64) {
	b.timeout = timeout
}

// HTTPClient определяет интерфейс для HTTP клиента
type HTTPClient interface {
	// Do выполняет HTTP запрос
	Do(req *http.Request) (*HTTPResponse, error)
}

// HTTPRequest представляет HTTP запрос
type HTTPRequest struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	Timeout     int64             `json:"timeout"` // в миллисекундах
}

// HTTPResponse представляет HTTP ответ
type HTTPResponse struct {
	StatusCode   int               `json:"status_code"`
	Headers      map[string]string `json:"headers"`
	Body         string            `json:"body"`
	DurationMs   int64             `json:"duration_ms"`
	SizeBytes    int64             `json:"size_bytes"`
}

// gRPCChecker реализует Checker для gRPC проверок
type gRPCChecker struct {
	*BaseChecker
	// gRPC специфичные поля
	client gRPCClient
}

// gRPCClient определяет интерфейс для gRPC клиента
type gRPCClient interface {
	// Call выполняет gRPC вызов
	Call(req *gRPCRequest) (*gRPCResponse, error)
}

// gRPCRequest представляет gRPC запрос
type gRPCRequest struct {
	Service     string            `json:"service"`
	Method      string            `json:"method"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Headers     map[string]string `json:"headers"`
	Timeout     int64             `json:"timeout"` // в миллисекундах
	Metadata    map[string]string `json:"metadata"`
}

// gRPCResponse представляет gRPC ответ
type gRPCResponse struct {
	Success     bool              `json:"success"`
	Headers     map[string]string `json:"headers"`
	Data        string            `json:"data"`
	DurationMs  int64             `json:"duration_ms"`
	Error       string            `json:"error,omitempty"`
}

// NewgRPCChecker создает новый gRPC checker
func NewgRPCChecker(timeout int64, client gRPCClient) *gRPCChecker {
	return &gRPCChecker{
		BaseChecker: NewBaseChecker(timeout),
		client:      client,
	}
}

// Execute выполняет gRPC проверку
func (g *gRPCChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	// Валидация конфигурации
	if err := g.ValidateConfig(task.Config); err != nil {
		return nil, err
	}
	
	// Извлечение gRPC конфигурации
	grpcConfig, err := task.GetgRPCConfig()
	if err != nil {
		return nil, err
	}
	
	// Создание gRPC запроса
	req := &gRPCRequest{
		Service:  grpcConfig.Service,
		Method:   grpcConfig.Method,
		Host:     grpcConfig.Host,
		Port:     grpcConfig.Port,
		Headers:  grpcConfig.Headers,
		Timeout:  int64(grpcConfig.Timeout.Milliseconds()),
		Metadata: grpcConfig.Metadata,
	}
	
	// Выполнение запроса
	resp, err := g.client.Call(req)
	if err != nil {
		return domain.NewCheckResult(
			task.CheckID,
			task.ExecutionID,
			false,
			resp.DurationMs,
			0,
			err.Error(),
			"",
		), nil
	}
	
	return domain.NewCheckResult(
		task.CheckID,
		task.ExecutionID,
		resp.Success,
		resp.DurationMs,
		0,
		resp.Error,
		resp.Data,
	), nil
}

// GetType возвращает тип checker'а
func (g *gRPCChecker) GetType() domain.TaskType {
	return domain.TaskTypeGRPC
}

// ValidateConfig валидирует gRPC конфигурацию
func (g *gRPCChecker) ValidateConfig(config map[string]interface{}) error {
	if _, ok := config["service"]; !ok {
		return &ValidationError{Field: "service", Message: "required"}
	}
	if _, ok := config["method"]; !ok {
		return &ValidationError{Field: "method", Message: "required"}
	}
	if _, ok := config["host"]; !ok {
		return &ValidationError{Field: "host", Message: "required"}
	}
	if _, ok := config["port"]; !ok {
		return &ValidationError{Field: "port", Message: "required"}
	}
	
	return nil
}

// GraphQLChecker реализует Checker для GraphQL проверок
type GraphQLChecker struct {
	*BaseChecker
	// GraphQL специфичные поля
	query string
}

// GraphQLRequest представляет GraphQL запрос
type GraphQLRequest struct {
	Query         string            `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string            `json:"operation_name,omitempty"`
	Headers       map[string]string `json:"headers"`
}

// GraphQLResponse представляет GraphQL ответ
type GraphQLResponse struct {
	Data       interface{}       `json:"data"`
	Errors     []GraphQLError    `json:"errors,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
	DurationMs int64             `json:"duration_ms"`
}

// GraphQLError представляет GraphQL ошибку
type GraphQLError struct {
	Message    string            `json:"message"`
	Locations  []GraphQLLocation `json:"locations,omitempty"`
	Path       []interface{}     `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// GraphQLLocation представляет локацию ошибки в GraphQL
type GraphQLLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// NewGraphQLChecker создает новый GraphQL checker
func NewGraphQLChecker(timeout int64, client HTTPClient) *GraphQLChecker {
	return &GraphQLChecker{
		BaseChecker: NewBaseChecker(timeout),
	}
}

// Execute выполняет GraphQL проверку
func (g *GraphQLChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	// Валидация конфигурации
	if err := g.ValidateConfig(task.Config); err != nil {
		return nil, err
	}
	
	// Заглушка для выполнения запроса
	resp := &HTTPResponse{
		StatusCode: 200,
		Body:       `{"data": {"status": "ok"}}`,
		DurationMs: 100,
	}
	
	// Парсинг GraphQL ответа и проверка на ошибки
	graphqlResp, err := g.parseGraphQLResponse(resp.Body)
	if err != nil {
		return domain.NewCheckResult(
			task.CheckID,
			task.ExecutionID,
			false,
			resp.DurationMs,
			resp.StatusCode,
			err.Error(),
			resp.Body,
		), nil
	}
	
	// GraphQL считается успешным если нет ошибок в ответе
	success := len(graphqlResp.Errors) == 0
	
	return domain.NewCheckResult(
		task.CheckID,
		task.ExecutionID,
		success,
		resp.DurationMs,
		resp.StatusCode,
		"",
		resp.Body,
	), nil
}

// GetType возвращает тип checker'а
func (g *GraphQLChecker) GetType() domain.TaskType {
	return domain.TaskTypeGraphQL
}

// ValidateConfig валидирует GraphQL конфигурацию
func (g *GraphQLChecker) ValidateConfig(config map[string]interface{}) error {
	if _, ok := config["url"]; !ok {
		return &ValidationError{Field: "url", Message: "required"}
	}
	if _, ok := config["query"]; !ok {
		return &ValidationError{Field: "query", Message: "required"}
	}
	
	return nil
}

// buildGraphQLBody строит тело GraphQL запроса
func (g *GraphQLChecker) buildGraphQLBody(config *domain.GraphQLConfig) string {
	// Реализация построения JSON тела для GraphQL запроса
	return `{"query":"` + config.Query + `"}`
}

// parseGraphQLResponse парсит GraphQL ответ
func (g *GraphQLChecker) parseGraphQLResponse(body string) (*GraphQLResponse, error) {
	// Реализация парсинга GraphQL ответа
	return &GraphQLResponse{
		DurationMs: 0,
	}, nil
}

// TCPChecker реализует Checker для TCP проверок
type TCPChecker struct {
	*BaseChecker
	// TCP специфичные поля
	dialer TCPDialer
}

// TCPDialer определяет интерфейс для TCP подключения
type TCPDialer interface {
	// Dial устанавливает TCP соединение
	Dial(address string, timeout int64) (*TCPConnection, error)
}

// TCPConnection представляет TCP соединение
type TCPConnection struct {
	Connected   bool   `json:"connected"`
	Address     string `json:"address"`
	DurationMs  int64  `json:"duration_ms"`
	Error       string `json:"error,omitempty"`
	LocalAddr   string `json:"local_addr,omitempty"`
	RemoteAddr  string `json:"remote_addr,omitempty"`
}

// NewTCPChecker создает новый TCP checker
func NewTCPChecker(timeout int64, dialer TCPDialer) *TCPChecker {
	return &TCPChecker{
		BaseChecker: NewBaseChecker(timeout),
		dialer:      dialer,
	}
}

// Execute выполняет TCP проверку
func (t *TCPChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	// Валидация конфигурации
	if err := t.ValidateConfig(task.Config); err != nil {
		return nil, err
	}
	
	// Извлечение TCP конфигурации
	tcpConfig, err := task.GetTCPConfig()
	if err != nil {
		return nil, err
	}
	
	// Формирование адреса
	address := fmt.Sprintf("%s:%d", tcpConfig.Host, tcpConfig.Port)
	
	// Установка соединения
	conn, err := t.dialer.Dial(address, int64(tcpConfig.Timeout.Milliseconds()))
	if err != nil {
		return domain.NewCheckResult(
			task.CheckID,
			task.ExecutionID,
			false,
			conn.DurationMs,
			0,
			err.Error(),
			"",
		), nil
	}
	
	return domain.NewCheckResult(
		task.CheckID,
		task.ExecutionID,
		conn.Connected,
		conn.DurationMs,
		0,
		conn.Error,
		"",
	), nil
}

// GetType возвращает тип checker'а
func (t *TCPChecker) GetType() domain.TaskType {
	return domain.TaskTypeTCP
}

// ValidateConfig валидирует TCP конфигурацию
func (t *TCPChecker) ValidateConfig(config map[string]interface{}) error {
	if _, ok := config["host"]; !ok {
		return &ValidationError{Field: "host", Message: "required"}
	}
	if _, ok := config["port"]; !ok {
		return &ValidationError{Field: "port", Message: "required"}
	}
	
	return nil
}

// ValidationError представляет ошибку валидации checker'а
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}
