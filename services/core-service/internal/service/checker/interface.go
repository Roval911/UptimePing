package checker

import (
	"fmt"
	"net/http"
	
	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
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

// TCPChecker реализует Checker для TCP проверок
type TCPChecker struct {
	*BaseChecker
	// TCP специфичные поля
	dialer    TCPDialer
	logger    logger.Logger
	validator *validation.Validator
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
func NewTCPChecker(timeout int64, dialer TCPDialer, log logger.Logger) *TCPChecker {
	return &TCPChecker{
		BaseChecker: NewBaseChecker(timeout),
		dialer:      dialer,
		logger:      log,
		validator:   validation.NewValidator(),
	}
}

// Execute выполняет TCP проверку
func (t *TCPChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	t.logger.Debug("Starting TCP check",
		logger.String("check_id", task.CheckID),
		logger.String("execution_id", task.ExecutionID),
		logger.String("target", task.Target),
	)
	
	// Валидация конфигурации
	if err := t.ValidateConfig(task.Config); err != nil {
		t.logger.Error("TCP config validation failed",
			logger.String("check_id", task.CheckID),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrValidation, "config validation failed")
	}
	
	// Извлечение TCP конфигурации
	tcpConfig, err := task.GetTCPConfig()
	if err != nil {
		t.logger.Error("Failed to extract TCP config",
			logger.String("check_id", task.CheckID),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to extract TCP config")
	}
	
	// Формирование адреса
	address := fmt.Sprintf("%s:%d", tcpConfig.Host, tcpConfig.Port)
	t.logger.Debug("Connecting to TCP service",
		logger.String("address", address),
		logger.Int64("timeout_ms", tcpConfig.Timeout.Milliseconds()),
	)
	
	// Установка соединения
	conn, err := t.dialer.Dial(address, int64(tcpConfig.Timeout.Milliseconds()))
	if err != nil {
		t.logger.Error("Failed to connect to TCP service",
			logger.String("address", address),
			logger.Int64("duration_ms", conn.DurationMs),
			logger.Error(err),
		)
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
	
	t.logger.Info("Successfully connected to TCP service",
		logger.String("address", address),
		logger.Int64("duration_ms", conn.DurationMs),
	)
	
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
	// Валидация обязательных полей с использованием pkg/validation
	requiredFields := map[string]interface{}{
		"host": config["host"],
		"port": config["port"],
	}
	
	if err := t.validator.ValidateRequiredFields(requiredFields, map[string]string{
		"host": "Host address",
		"port": "Port number",
	}); err != nil {
		t.logger.Debug("TCP config validation failed", logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "required fields validation failed")
	}
	
	// Валидация host:port формата
	host := config["host"].(string)
	port := config["port"]
	address := fmt.Sprintf("%s:%v", host, port)
	
	if err := t.validator.ValidateHostPort(address); err != nil {
		t.logger.Debug("TCP config validation failed: invalid host:port", 
			logger.String("host_port", address),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "invalid host:port format")
	}
	
	t.logger.Debug("TCP config validation passed")
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
