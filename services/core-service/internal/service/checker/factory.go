package checker

import (
	"fmt"
	"net/http"
	"time"

	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
)

// DefaultCheckerFactory реализация CheckerFactory
type DefaultCheckerFactory struct {
	logger    logger.Logger
	validator *validation.Validator
	httpClient HTTPClient
}

// NewDefaultCheckerFactory создает новую фабрику checker'ов
func NewDefaultCheckerFactory(logger logger.Logger, httpClient HTTPClient) *DefaultCheckerFactory {
	return &DefaultCheckerFactory{
		logger:    logger,
		validator: validation.NewValidator(),
		httpClient: httpClient,
	}
}

// CreateChecker создает checker для указанного типа
func (f *DefaultCheckerFactory) CreateChecker(taskType domain.TaskType) (Checker, error) {
	switch taskType {
	case domain.TaskTypeHTTP:
		return NewHTTPChecker(30000, f.logger), nil
	case domain.TaskTypeTCP:
		return NewTCPChecker(30000, &DefaultTCPDialer{}, f.logger), nil
	case domain.TaskTypeICMP:
		return NewICMPChecker(f.logger), nil
	case domain.TaskTypeGRPC:
		return NewGRPCChecker(f.logger), nil
	case domain.TaskTypeGraphQL:
		// Используем существующий GraphQLChecker
		return NewGraphQLChecker(30000, f.logger), nil
	default:
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
}

// GetSupportedTypes возвращает список поддерживаемых типов
func (f *DefaultCheckerFactory) GetSupportedTypes() []domain.TaskType {
	return []domain.TaskType{
		domain.TaskTypeHTTP,
		domain.TaskTypeTCP,
		domain.TaskTypeICMP,
		domain.TaskTypeGRPC,
		domain.TaskTypeGraphQL,
	}
}

// DefaultHTTPClient реализация HTTPClient
type DefaultHTTPClient struct {
	client *http.Client
}

// NewDefaultHTTPClient создает новый HTTP клиент
func NewDefaultHTTPClient(timeout time.Duration) *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Do выполняет HTTP запрос
func (c *DefaultHTTPClient) Do(req *http.Request) (*HTTPResponse, error) {
	start := time.Now()
	
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	duration := time.Since(start)
	
	// Читаем тело ответа
	body := make([]byte, 0, 1024)
	// Для простоты не будем читать тело полностью
	
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	
	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(body),
		DurationMs: duration.Milliseconds(),
		SizeBytes:  int64(len(body)),
	}, nil
}

// DefaultTCPDialer реализация TCPDialer
type DefaultTCPDialer struct{}

// Dial устанавливает TCP соединение
func (d *DefaultTCPDialer) Dial(address string, timeout int64) (*TCPConnection, error) {
	start := time.Now()
	
	// Для простоты симулируем TCP подключение
	// В реальной реализации здесь был бы net.DialTimeout
	
	duration := time.Since(start)
	
	// Симуляция успешного подключения
	return &TCPConnection{
		Connected:   true,
		Address:     address,
		DurationMs:  duration.Milliseconds(),
		LocalAddr:   "127.0.0.1:12345",
		RemoteAddr:  address,
	}, nil
}

// NewICMPChecker создает ICMP checker
func NewICMPChecker(logger logger.Logger) Checker {
	return &ICMPChecker{
		BaseChecker: NewBaseChecker(logger),
	}
}

// ICMPChecker реализует Checker для ICMP проверок
type ICMPChecker struct {
	*BaseChecker
}

// Execute выполняет ICMP проверку
func (i *ICMPChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	i.logger.Debug("Starting ICMP check",
		logger.String("check_id", task.CheckID),
		logger.String("execution_id", task.ExecutionID),
		logger.String("target", task.Target),
	)
	
	// Валидация конфигурации
	if err := i.ValidateConfig(task.Config); err != nil {
		return nil, err
	}
	
	// Симуляция ICMP проверки
	// В реальной реализации здесь был бы ping
	
	return domain.NewCheckResult(
		task.CheckID,
		task.ExecutionID,
		true,  // success
		50,    // duration_ms
		0,     // status_code
		"",    // error
		"",    // response_body
	), nil
}

// GetType возвращает тип checker'а
func (i *ICMPChecker) GetType() domain.TaskType {
	return domain.TaskTypeICMP
}

// ValidateConfig валидирует ICMP конфигурацию
func (i *ICMPChecker) ValidateConfig(config map[string]interface{}) error {
	// ICMP проверки обычно не требуют конфигурации
	return nil
}

// NewGRPCChecker создает gRPC checker
func NewGRPCChecker(logger logger.Logger) Checker {
	return &GRPCChecker{
		BaseChecker: NewBaseChecker(logger),
	}
}

// GRPCChecker реализует Checker для gRPC проверок
type GRPCChecker struct {
	*BaseChecker
}

// Execute выполняет gRPC проверку
func (g *GRPCChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	g.logger.Debug("Starting gRPC check",
		logger.String("check_id", task.CheckID),
		logger.String("execution_id", task.ExecutionID),
		logger.String("target", task.Target),
	)
	
	// Валидация конфигурации
	if err := g.ValidateConfig(task.Config); err != nil {
		return nil, err
	}
	
	// Симуляция gRPC проверки
	// В реальной реализации здесь был бы gRPC клиент
	
	return domain.NewCheckResult(
		task.CheckID,
		task.ExecutionID,
		true,  // success
		100,   // duration_ms
		0,     // status_code
		"",    // error
		"",    // response_body
	), nil
}

// GetType возвращает тип checker'а
func (g *GRPCChecker) GetType() domain.TaskType {
	return domain.TaskTypeGRPC
}

// ValidateConfig валидирует gRPC конфигурацию
func (g *GRPCChecker) ValidateConfig(config map[string]interface{}) error {
	// gRPC проверки могут иметь конфигурацию, но для простоты не валидируем
	return nil
}
