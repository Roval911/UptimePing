package health

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"UptimePingPlatform/pkg/connection"
	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/services/core-service/internal/client"
	"UptimePingPlatform/services/core-service/internal/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Status представляет статус здоровья компонента
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
	StatusUnknown   Status = "unknown"
)

// CheckResult представляет результат проверки здоровья
type CheckResult struct {
	Component string                 `json:"component"`
	Status    Status                 `json:"status"`
	Message   string                 `json:"message"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HealthChecker интерфейс для проверки здоровья
type HealthChecker interface {
	Check(ctx context.Context) *CheckResult
	Name() string
}

// Config конфигурация health checker
type Config struct {
	// Таймауты для проверок
	RabbitMQTimeout time.Duration `json:"rabbitmq_timeout"`
	DatabaseTimeout time.Duration `json:"database_timeout"`
	IncidentTimeout time.Duration `json:"incident_timeout"`

	// Интервалы проверок
	CheckInterval time.Duration `json:"check_interval"`

	// Настройки RabbitMQ
	RabbitMQURL   string `json:"rabbitmq_url"`
	RabbitMQQueue string `json:"rabbitmq_queue"`

	// Настройки базы данных
	DatabaseDSN string `json:"database_dsn"`

	// Настройки Incident Manager
	IncidentManagerAddress string `json:"incident_manager_address"`

	// Graceful shutdown
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	MaxShutdownWait time.Duration `json:"max_shutdown_wait"`
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *Config {
	return &Config{
		RabbitMQTimeout:        5 * time.Second,
		DatabaseTimeout:        5 * time.Second,
		IncidentTimeout:        5 * time.Second,
		CheckInterval:          30 * time.Second,
		RabbitMQURL:            "amqp://localhost:5672",
		RabbitMQQueue:          "uptime_checks",
		DatabaseDSN:            "postgres://user:password@localhost/uptimedb?sslmode=disable",
		IncidentManagerAddress: "localhost:50052",
		ShutdownTimeout:        30 * time.Second,
		MaxShutdownWait:        10 * time.Second,
	}
}

// Validate проверяет конфигурацию
func (c *Config) Validate() error {
	if c.RabbitMQTimeout <= 0 {
		return fmt.Errorf("rabbitmq timeout must be positive")
	}
	if c.DatabaseTimeout <= 0 {
		return fmt.Errorf("database timeout must be positive")
	}
	if c.IncidentTimeout <= 0 {
		return fmt.Errorf("incident timeout must be positive")
	}
	if c.CheckInterval <= 0 {
		return fmt.Errorf("check interval must be positive")
	}
	if c.RabbitMQURL == "" {
		return fmt.Errorf("rabbitmq url is required")
	}
	if c.DatabaseDSN == "" {
		return fmt.Errorf("database dsn is required")
	}
	if c.IncidentManagerAddress == "" {
		return fmt.Errorf("incident manager address is required")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	if c.MaxShutdownWait <= 0 {
		return fmt.Errorf("max shutdown wait must be positive")
	}
	return nil
}

// Service представляет сервис health check
type Service struct {
	config   *Config
	logger   *logging.UptimeLogger
	checkers []HealthChecker
	results  map[string]*CheckResult
	mu       sync.RWMutex

	// Компоненты
	db             *sql.DB
	rabbitMQ       *rabbitmq.Producer
	incidentClient client.IncidentClient

	// Graceful shutdown
	shutdownChan chan struct{}
	shutdownOnce sync.Once
	wg           sync.WaitGroup
}

// NewService создает новый сервис health check
func NewService(config *Config, logger *logging.UptimeLogger) (*Service, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	service := &Service{
		config:       config,
		logger:       logger.WithComponent("health-checker"),
		checkers:     make([]HealthChecker, 0),
		results:      make(map[string]*CheckResult),
		shutdownChan: make(chan struct{}),
	}

	// Инициализируем checkers
	service.initCheckers()

	return service, nil
}

// initCheckers инициализирует проверщики здоровья
func (s *Service) initCheckers() {
	// RabbitMQ checker
	rabbitLogger, _ := logger.NewLogger("development", "info", "rabbitmq-health", false)
	s.checkers = append(s.checkers, &RabbitMQChecker{
		url:     s.config.RabbitMQURL,
		queue:   s.config.RabbitMQQueue,
		timeout: s.config.RabbitMQTimeout,
		logger:  &rabbitLogger,
	})

	// Database checker
	dbLogger, _ := logger.NewLogger("development", "info", "database-health", false)
	s.checkers = append(s.checkers, &DatabaseChecker{
		dsn:     s.config.DatabaseDSN,
		timeout: s.config.DatabaseTimeout,
		logger:  &dbLogger,
	})

	// Incident Manager checker
	incidentLogger, _ := logger.NewLogger("development", "info", "incident-manager-health", false)
	incidentHandler := grpcBase.NewBaseHandler(incidentLogger)
	s.checkers = append(s.checkers, &IncidentManagerChecker{
		address: s.config.IncidentManagerAddress,
		timeout: s.config.IncidentTimeout,
		logger:  incidentLogger,
		handler: incidentHandler,
	})
}

// Start запускает сервис health check
func (s *Service) Start(ctx context.Context) error {
	s.logger.GetBaseLogger().Info("Starting health check service")

	// Запускаем периодические проверки
	s.wg.Add(1)
	go s.runPeriodicChecks(ctx)

	s.logger.GetBaseLogger().Info("Health check service started",
		logger.Int("checkers_count", len(s.checkers)),
		logger.String("check_interval", s.config.CheckInterval.String()))

	return nil
}

// Stop останавливает сервис с graceful shutdown
func (s *Service) Stop(ctx context.Context) error {
	s.shutdownOnce.Do(func() {
		close(s.shutdownChan)
	})

	s.logger.GetBaseLogger().Info("Stopping health check service")

	// Создаем контекст с таймаутом
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	// Ждем завершения
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.GetBaseLogger().Info("Health check service stopped gracefully")
	case <-shutdownCtx.Done():
		s.logger.GetBaseLogger().Warn("Health check service shutdown timeout reached")
	}

	return nil
}

// CheckAll выполняет все проверки здоровья
func (s *Service) CheckAll(ctx context.Context) map[string]*CheckResult {
	results := make(map[string]*CheckResult)

	for _, checker := range s.checkers {
		result := checker.Check(ctx)
		results[checker.Name()] = result

		// Обновляем кэш результатов
		s.mu.Lock()
		s.results[checker.Name()] = result
		s.mu.Unlock()
	}

	return results
}

// GetStatus возвращает общий статус здоровья
func (s *Service) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.results) == 0 {
		return StatusUnknown
	}

	hasUnhealthy := false
	hasDegraded := false

	for _, result := range s.results {
		switch result.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}

	return StatusHealthy
}

// GetResults возвращает последние результаты проверок
func (s *Service) GetResults() map[string]*CheckResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Создаем копию результатов
	results := make(map[string]*CheckResult)
	for k, v := range s.results {
		results[k] = v
	}

	return results
}

// runPeriodicChecks запускает периодические проверки
func (s *Service) runPeriodicChecks(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	// Выполняем первую проверку немедленно
	s.performChecks(ctx)

	for {
		select {
		case <-ticker.C:
			s.performChecks(ctx)
		case <-s.shutdownChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// performChecks выполняет все проверки
func (s *Service) performChecks(ctx context.Context) {
	s.logger.GetBaseLogger().Debug("Performing health checks")

	results := s.CheckAll(ctx)

	// Логируем результаты
	for name, result := range results {
		logFunc := s.logger.GetBaseLogger().Info
		if result.Status == StatusUnhealthy {
			logFunc = s.logger.GetBaseLogger().Error
		} else if result.Status == StatusDegraded {
			logFunc = s.logger.GetBaseLogger().Warn
		}

		logFunc("Health check result",
			logger.String("component", name),
			logger.String("status", string(result.Status)),
			logger.String("message", result.Message),
			logger.String("duration", result.Duration.String()))
	}

	// Обновляем общую статистику
	status := s.GetStatus()
	s.logger.GetBaseLogger().Info("Overall health status",
		logger.String("status", string(status)),
		logger.Int("healthy_checks", s.countChecksByStatus(results, StatusHealthy)),
		logger.Int("degraded_checks", s.countChecksByStatus(results, StatusDegraded)),
		logger.Int("unhealthy_checks", s.countChecksByStatus(results, StatusUnhealthy)))
}

// countChecksByStatus подсчитывает количество проверок по статусу
func (s *Service) countChecksByStatus(results map[string]*CheckResult, status Status) int {
	count := 0
	for _, result := range results {
		if result.Status == status {
			count++
		}
	}
	return count
}

// RabbitMQChecker проверяет здоровье RabbitMQ
type RabbitMQChecker struct {
	url     string
	queue   string
	timeout time.Duration
	logger  *logger.Logger
}

func (c *RabbitMQChecker) Name() string {
	return "rabbitmq"
}

func (c *RabbitMQChecker) Check(ctx context.Context) *CheckResult {
	start := time.Now()

	result := &CheckResult{
		Component: c.Name(),
		Timestamp: start,
	}

	// Создаем контекст с таймаутом
	_, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Пытаемся подключиться к RabbitMQ
	config := rabbitmq.NewConfig()
	config.URL = c.url
	conn, err := rabbitmq.Connect(ctx, config)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to connect to RabbitMQ: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer conn.Close()

	producer := rabbitmq.NewProducer(conn, config)

	// Проверяем доступность очереди
	err = producer.Publish(ctx, []byte("health-check"))
	if err != nil {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("RabbitMQ connected but queue unavailable: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Status = StatusHealthy
	result.Message = "RabbitMQ is healthy"
	result.Duration = time.Since(start)
	result.Details = map[string]interface{}{
		"url":   c.url,
		"queue": c.queue,
	}

	return result
}

// DatabaseChecker проверяет здоровье базы данных
type DatabaseChecker struct {
	dsn     string
	timeout time.Duration
	logger  *logger.Logger
}

func (c *DatabaseChecker) Name() string {
	return "database"
}

func (c *DatabaseChecker) Check(ctx context.Context) *CheckResult {
	start := time.Now()

	result := &CheckResult{
		Component: c.Name(),
		Timestamp: start,
	}

	// Создаем контекст с таймаутом
	_, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Пытаемся подключиться к базе данных
	db, err := sql.Open("postgres", c.dsn)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to connect to database: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer db.Close()

	// Выполняем простой запрос
	var version string
	err = db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("Database connected but query failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Status = StatusHealthy
	result.Message = "Database is healthy"
	result.Duration = time.Since(start)
	result.Details = map[string]interface{}{
		"version": version,
		"dsn":     c.dsn,
	}

	return result
}

// grpcConnecter реализует connection.Connecter для gRPC
type grpcConnecter struct {
	address string
	timeout time.Duration
	conn    *grpc.ClientConn
}

func (g *grpcConnecter) Connect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, g.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
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

// IncidentManagerChecker проверяет здоровье Incident Manager
type IncidentManagerChecker struct {
	address string
	timeout time.Duration
	logger  logger.Logger
	handler *grpcBase.BaseHandler
}

func (c *IncidentManagerChecker) Name() string {
	return "incident-manager"
}

func (c *IncidentManagerChecker) Check(ctx context.Context) *CheckResult {
	start := time.Now()

	result := &CheckResult{
		Component: c.Name(),
		Timestamp: start,
	}

	// Создаем контекст с таймаутом
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Создаем gRPC connecter
	connecter := &grpcConnecter{
		address: c.address,
		timeout: c.timeout,
	}

	// Используем pkg/connection для retry логики
	retryConfig := connection.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     2 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}

	err := connection.ConnectWithRetry(checkCtx, connecter, retryConfig)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to connect to Incident Manager: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer connecter.Close()

	// Проверяем health service
	healthClient := grpc_health_v1.NewHealthClient(connecter.conn)

	req := &grpc_health_v1.HealthCheckRequest{
		Service: "incident.service",
	}

	resp, err := healthClient.Check(checkCtx, req)
	if err != nil {
		c.handler.LogError(ctx, err, "Health check failed", c.Name())
		
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("Incident Manager connected but health check failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	status := StatusHealthy
	switch resp.Status {
	case grpc_health_v1.HealthCheckResponse_SERVING:
		status = StatusHealthy
	case grpc_health_v1.HealthCheckResponse_NOT_SERVING:
		status = StatusDegraded
	case grpc_health_v1.HealthCheckResponse_SERVICE_UNKNOWN:
		status = StatusUnhealthy
	}

	result.Status = status
	result.Message = fmt.Sprintf("Incident Manager status: %s", resp.Status.String())
	result.Duration = time.Since(start)
	result.Details = map[string]interface{}{
		"address":     c.address,
		"grpc_status": resp.Status.String(),
	}

	return result
}
