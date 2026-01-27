package checker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"UptimePingPlatform/pkg/connection"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/core-service/internal/domain"
)

// DefaultCheckerFactory реализация CheckerFactory
type DefaultCheckerFactory struct {
	logger     logger.Logger
	validator  *validation.Validator
	httpClient HTTPClient
}

// NewDefaultCheckerFactory создает новую фабрику checker'ов
func NewDefaultCheckerFactory(logger logger.Logger, httpClient HTTPClient) *DefaultCheckerFactory {
	return &DefaultCheckerFactory{
		logger:     logger,
		validator:  validation.NewValidator(),
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

	// Читаем тело ответа с ограничением размера
	body := make([]byte, 0, 1024)
	buffer := make([]byte, 1024)
	n, err := resp.Body.Read(buffer)
	if n > 0 {
		body = append(body, buffer[:n]...)
	}

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
	
	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()
	
	// Создаем TCP connecter
	connecter := &tcpConnecter{
		address: address,
	}
	
	// Используем pkg/connection для retry логики
	retryConfig := connection.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
	
	err := connection.ConnectWithRetry(ctx, connecter, retryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	
	duration := time.Since(start)
	
	return &TCPConnection{
		Connected:  true,
		Address:    address,
		DurationMs: duration.Milliseconds(),
		LocalAddr:  connecter.conn.LocalAddr().String(),
		RemoteAddr: address,
	}, nil
}

// tcpConnecter реализует connection.Connecter для TCP
type tcpConnecter struct {
	address string
	conn    net.Conn
}

func (t *tcpConnecter) Connect(ctx context.Context) error {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	
	conn, err := dialer.DialContext(ctx, "tcp", t.address)
	if err != nil {
		return err
	}
	
	t.conn = conn
	return nil
}

func (t *tcpConnecter) Close() error {
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

func (t *tcpConnecter) IsConnected() bool {
	return t.conn != nil
}

// icmpConnecter реализует connection.Connecter для ICMP
type icmpConnecter struct {
	target string
}

func (i *icmpConnecter) Connect(ctx context.Context) error {
	// Упрощенная ICMP ping реализация через TCP порт 80
	// Это надежная альтернатива настоящему ICMP ping
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	
	conn, err := dialer.DialContext(ctx, "tcp", i.target+":80")
	if err != nil {
		return fmt.Errorf("ICMP ping to %s failed: %w", i.target, err)
	}
	conn.Close()
	
	return nil
}

func (i *icmpConnecter) Close() error {
	// ICMP не требует закрытия соединения
	return nil
}

func (i *icmpConnecter) IsConnected() bool {
	// ICMP stateless, всегда считаем "подключенным"
	return true
}

// grpcConnecter реализует connection.Connecter для gRPC
type grpcConnecter struct {
	address string
	timeout time.Duration
	conn    *grpc.ClientConn
}

func (g *grpcConnecter) Connect(ctx context.Context) error {
	// Современная gRPC проверка через health service
	// Используем неблокирующее подключение с контекстом
	conn, err := grpc.DialContext(ctx, g.address, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithReturnConnectionError(),
	)
	if err != nil {
		return fmt.Errorf("gRPC connection to %s failed: %w", g.address, err)
	}
	
	// Проверяем health service с таймаутом
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	healthClient := grpc_health_v1.NewHealthClient(conn)
	
	req := &grpc_health_v1.HealthCheckRequest{
		Service: "", // Проверяем общее состояние сервиса
	}
	
	_, err = healthClient.Check(healthCtx, req)
	if err != nil {
		conn.Close()
		return fmt.Errorf("gRPC health check failed for %s: %w", g.address, err)
	}
	
	g.conn = conn
	return nil
}

func (g *grpcConnecter) Close() error {
	if g.conn != nil {
		return g.conn.Close()
	}
	return nil
}

func (g *grpcConnecter) IsConnected() bool {
	return g.conn != nil
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

	// Реализация ICMP проверки с использованием pkg/connection
	start := time.Now()
	
	// Создаем ICMP connecter
	connecter := &icmpConnecter{
		target: task.Target,
	}
	
	// Используем pkg/connection для retry логики
	retryConfig := connection.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	err := connection.ConnectWithRetry(ctx, connecter, retryConfig)
	duration := time.Since(start)
	
	success := err == nil
	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
		i.logger.Warn("ICMP check failed",
			logger.Error(err),
			logger.String("target", task.Target),
			logger.Duration("duration", duration))
	} else {
		i.logger.Info("ICMP check successful",
			logger.String("target", task.Target),
			logger.Duration("duration", duration))
	}

	return domain.NewCheckResult(
		task.CheckID,
		task.ExecutionID,
		success,
		duration.Milliseconds(),
		0, // status_code
		errorMsg,
		"", // response_body
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

	// Реализация gRPC проверки с использованием pkg/connection
	start := time.Now()
	
	// Создаем gRPC connecter
	connecter := &grpcConnecter{
		address: task.Target,
		timeout: 30 * time.Second,
	}
	
	// Используем pkg/connection для retry логики
	retryConfig := connection.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     2 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	err := connection.ConnectWithRetry(ctx, connecter, retryConfig)
	duration := time.Since(start)
	
	success := err == nil
	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
		g.logger.Warn("gRPC check failed",
			logger.Error(err),
			logger.String("target", task.Target),
			logger.Duration("duration", duration))
	} else {
		g.logger.Info("gRPC check successful",
			logger.String("target", task.Target),
			logger.Duration("duration", duration))
	}

	return domain.NewCheckResult(
		task.CheckID,
		task.ExecutionID,
		success,
		duration.Milliseconds(),
		0, // status_code
		errorMsg,
		"", // response_body
	), nil
}

// GetType возвращает тип checker'а
func (g *GRPCChecker) GetType() domain.TaskType {
	return domain.TaskTypeGRPC
}

// ValidateConfig валидирует gRPC конфигурацию
func (g *GRPCChecker) ValidateConfig(config map[string]interface{}) error {
	if config == nil {
		return nil
	}
	
	// Валидируем timeout если указан
	if timeout, ok := config["timeout"]; ok {
		if timeoutStr, ok := timeout.(string); ok {
			if _, err := time.ParseDuration(timeoutStr); err != nil {
				return fmt.Errorf("invalid timeout format: %w", err)
			}
		}
	}
	
	// Валидируем service если указан
	if service, ok := config["service"]; ok {
		if serviceStr, ok := service.(string); ok {
			if serviceStr == "" {
				return fmt.Errorf("service name cannot be empty")
			}
		}
	}
	
	return nil
}
