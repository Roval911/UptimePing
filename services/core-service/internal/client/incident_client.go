package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"UptimePingPlatform/gen/go/proto/api/incident/v1"
	"UptimePingPlatform/services/core-service/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// IncidentClient интерфейс для работы с Incident Manager
type IncidentClient interface {
	CreateIncident(ctx context.Context, result *domain.CheckResult, tenantID string) (*v1.Incident, error)
	UpdateIncident(ctx context.Context, incidentID string, status v1.IncidentStatus, severity v1.IncidentSeverity) (*v1.Incident, error)
	ResolveIncident(ctx context.Context, incidentID string) error
	GetIncident(ctx context.Context, incidentID string) (*v1.Incident, error)
	ListIncidents(ctx context.Context, tenantID string, status v1.IncidentStatus, severity v1.IncidentSeverity, pageSize, pageToken int32) ([]*v1.Incident, int32, error)
	Close() error
	GetStats() *ClientStats
}

// Config конфигурация клиента
type Config struct {
	// Адрес Incident Manager
	Address string
	
	// Таймауты
	Timeout time.Duration
	
	// Retry конфигурация
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	RetryMultiplier float64
	RetryJitter     float64
	
	// Размер буфера для retry
	RetryBufferSize int
	
	// Логирование
	EnableLogging bool
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *Config {
	return &Config{
		Address:         "localhost:50052",
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        10 * time.Second,
		RetryMultiplier: 2.0,
		RetryJitter:     0.1,
		RetryBufferSize: 1000,
		EnableLogging:   true,
	}
}

// Validate проверяет конфигурацию
func (c *Config) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("address is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative")
	}
	if c.InitialDelay <= 0 {
		return fmt.Errorf("initial delay must be positive")
	}
	if c.MaxDelay <= 0 {
		return fmt.Errorf("max delay must be positive")
	}
	if c.RetryMultiplier <= 1.0 {
		return fmt.Errorf("retry multiplier must be greater than 1.0")
	}
	if c.RetryJitter < 0 || c.RetryJitter > 1.0 {
		return fmt.Errorf("retry jitter must be between 0 and 1")
	}
	if c.RetryBufferSize <= 0 {
		return fmt.Errorf("retry buffer size must be positive")
	}
	return nil
}

// Merge сливает конфигурацию с другой
func (c *Config) Merge(other *Config) *Config {
	if other == nil {
		return c
	}
	
	result := *c
	
	if other.Address != "" {
		result.Address = other.Address
	}
	if other.Timeout > 0 {
		result.Timeout = other.Timeout
	}
	if other.MaxRetries >= 0 {
		result.MaxRetries = other.MaxRetries
	}
	if other.InitialDelay > 0 {
		result.InitialDelay = other.InitialDelay
	}
	if other.MaxDelay > 0 {
		result.MaxDelay = other.MaxDelay
	}
	if other.RetryMultiplier > 1.0 {
		result.RetryMultiplier = other.RetryMultiplier
	}
	if other.RetryJitter >= 0 {
		result.RetryJitter = other.RetryJitter
	}
	if other.RetryBufferSize > 0 {
		result.RetryBufferSize = other.RetryBufferSize
	}
	
	return &result
}

// ClientStats статистика клиента
type ClientStats struct {
	mu                    sync.RWMutex
	IncidentsCreated      int64
	IncidentsUpdated      int64
	IncidentsResolved     int64
	CallsTotal            int64
	CallsSuccessful       int64
	CallsFailed           int64
	RetriesTotal          int64
	LastCallTime          time.Time
	LastError             string
	AverageResponseTime   time.Duration
	totalResponseTime     time.Duration
	responseTimeCount     int64
}

// updateStats обновляет статистику
func (s *ClientStats) updateStats(success bool, responseTime time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.CallsTotal++
	s.LastCallTime = time.Now()
	
	if success {
		s.CallsSuccessful++
	} else {
		s.CallsFailed++
		if err != nil {
			s.LastError = err.Error()
		}
	}
	
	if responseTime > 0 {
		s.totalResponseTime += responseTime
		s.responseTimeCount++
		s.AverageResponseTime = s.totalResponseTime / time.Duration(s.responseTimeCount)
	}
}

// incrementCreated инкрементирует счетчик созданных инцидентов
func (s *ClientStats) incrementCreated() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IncidentsCreated++
}

// incrementUpdated инкрементирует счетчик обновленных инцидентов
func (s *ClientStats) incrementUpdated() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IncidentsUpdated++
}

// incrementResolved инкрементирует счетчик закрытых инцидентов
func (s *ClientStats) incrementResolved() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IncidentsResolved++
}

// incrementRetries инкрементирует счетчик retry
func (s *ClientStats) incrementRetries() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RetriesTotal++
}

// GetStats возвращает копию статистики
func (s *ClientStats) GetStats() *ClientStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return &ClientStats{
		IncidentsCreated:    s.IncidentsCreated,
		IncidentsUpdated:    s.IncidentsUpdated,
		IncidentsResolved:   s.IncidentsResolved,
		CallsTotal:          s.CallsTotal,
		CallsSuccessful:     s.CallsSuccessful,
		CallsFailed:         s.CallsFailed,
		RetriesTotal:        s.RetriesTotal,
		LastCallTime:        s.LastCallTime,
		LastError:           s.LastError,
		AverageResponseTime: s.AverageResponseTime,
	}
}

// incidentClient реализация IncidentClient
type incidentClient struct {
	config *Config
	conn   *grpc.ClientConn
	client v1.IncidentServiceClient
	stats  *ClientStats
	logger *log.Logger
	mu     sync.RWMutex
}

// NewIncidentClient создает новый клиент для Incident Manager
func NewIncidentClient(config *Config) (IncidentClient, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	client := &incidentClient{
		config: config,
		stats:  &ClientStats{},
		logger: log.New(log.Writer(), "[IncidentClient] ", log.LstdFlags),
	}
	
	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	
	return client, nil
}

// connect устанавливает соединение с gRPC сервером
func (c *incidentClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()
	
	conn, err := grpc.DialContext(ctx, c.config.Address, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	
	c.conn = conn
	c.client = v1.NewIncidentServiceClient(conn)
	
	if c.config.EnableLogging {
		c.logger.Printf("Connected to Incident Manager at %s", c.config.Address)
	}
	
	return nil
}

// calculateRetryDelay вычисляет задержку для retry с экспоненциальным backoff и jitter
func (c *incidentClient) calculateRetryDelay(attempt int) time.Duration {
	delay := time.Duration(float64(c.config.InitialDelay) * 
		pow(c.config.RetryMultiplier, float64(attempt)))
	
	if delay > c.config.MaxDelay {
		delay = c.config.MaxDelay
	}
	
	// Добавляем jitter
	jitter := time.Duration(float64(delay) * c.config.RetryJitter * (rand.Float64()*2 - 1))
	delay += jitter
	
	// Убеждаемся, что задержка не отрицательная и не превышает максимум
	if delay < 0 {
		delay = 0
	}
	if delay > c.config.MaxDelay {
		delay = c.config.MaxDelay
	}
	
	return delay
}

// pow быстрая реализация возведения в степень
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// generateErrorHash генерирует хеш ошибки для дедупликации
func (c *incidentClient) generateErrorHash(checkID, errorMessage string) string {
	data := fmt.Sprintf("%s:%s", checkID, errorMessage)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16]
}

// determineSeverity определяет серьезность инцидента на основе результата проверки
func (c *incidentClient) determineSeverity(result *domain.CheckResult) v1.IncidentSeverity {
	if result.Success {
		return v1.IncidentSeverity_INCIDENT_SEVERITY_WARNING
	}
	
	// Определяем серьезность на основе типа ошибки
	if result.StatusCode >= 500 {
		return v1.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL
	}
	if result.StatusCode >= 400 {
		return v1.IncidentSeverity_INCIDENT_SEVERITY_ERROR
	}
	
	return v1.IncidentSeverity_INCIDENT_SEVERITY_ERROR
}

// executeWithRetry выполняет функцию с retry логикой
func (c *incidentClient) executeWithRetry(ctx context.Context, fn func() error) error {
	// Проверяем, что клиент инициализирован
	if c.client == nil {
		return fmt.Errorf("gRPC client is not initialized")
	}
	
	var lastErr error
	
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			c.stats.incrementRetries()
			
			delay := c.calculateRetryDelay(attempt - 1)
			if c.config.EnableLogging {
				c.logger.Printf("Retry attempt %d/%d after %v", attempt, c.config.MaxRetries, delay)
			}
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		
		if err := fn(); err != nil {
			lastErr = err
			
			// Проверяем, нужно ли retry
			if !c.shouldRetry(err) || attempt == c.config.MaxRetries {
				break
			}
			
			continue
		}
		
		// Успешное выполнение
		return nil
	}
	
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// shouldRetry проверяет, нужно ли повторять операцию
func (c *incidentClient) shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	
	// Проверяем статус gRPC ошибки
	if grpcErr, ok := status.FromError(err); ok {
		switch grpcErr.Code() {
		case codes.DeadlineExceeded, codes.Unavailable, codes.Aborted:
			return true
		case codes.Internal, codes.Unknown:
			return true
		default:
			return false
		}
	}
	
	// Для других типов ошибок тоже пробуем retry
	return true
}

// CreateIncident создает новый инцидент
func (c *incidentClient) CreateIncident(ctx context.Context, result *domain.CheckResult, tenantID string) (*v1.Incident, error) {
	if result == nil {
		return nil, fmt.Errorf("check result is nil")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("tenant ID is required")
	}
	
	var incident *v1.Incident
	start := time.Now()
	
	err := c.executeWithRetry(ctx, func() error {
		req := &v1.CreateIncidentRequest{
			CheckId:      result.CheckID,
			TenantId:     tenantID,
			Severity:     c.determineSeverity(result),
			ErrorMessage: result.Error,
		}
		
		resp, err := c.client.CreateIncident(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to create incident: %w", err)
		}
		
		incident = resp
		return nil
	})
	
	responseTime := time.Since(start)
	c.stats.updateStats(err == nil, responseTime, err)
	
	if err != nil {
		return nil, err
	}
	
	c.stats.incrementCreated()
	
	if c.config.EnableLogging {
		c.logger.Printf("Created incident %s for check %s", incident.Id, result.CheckID)
	}
	
	return incident, nil
}

// UpdateIncident обновляет существующий инцидент
func (c *incidentClient) UpdateIncident(ctx context.Context, incidentID string, status v1.IncidentStatus, severity v1.IncidentSeverity) (*v1.Incident, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	
	var incident *v1.Incident
	start := time.Now()
	
	err := c.executeWithRetry(ctx, func() error {
		req := &v1.UpdateIncidentRequest{
			IncidentId: incidentID,
			Status:     status,
			Severity:   severity,
		}
		
		resp, err := c.client.UpdateIncident(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to update incident: %w", err)
		}
		
		incident = resp
		return nil
	})
	
	responseTime := time.Since(start)
	c.stats.updateStats(err == nil, responseTime, err)
	
	if err != nil {
		return nil, err
	}
	
	c.stats.incrementUpdated()
	
	if c.config.EnableLogging {
		c.logger.Printf("Updated incident %s", incidentID)
	}
	
	return incident, nil
}

// ResolveIncident закрывает инцидент
func (c *incidentClient) ResolveIncident(ctx context.Context, incidentID string) error {
	if incidentID == "" {
		return fmt.Errorf("incident ID is required")
	}
	
	start := time.Now()
	
	err := c.executeWithRetry(ctx, func() error {
		req := &v1.ResolveIncidentRequest{
			IncidentId: incidentID,
		}
		
		_, err := c.client.ResolveIncident(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to resolve incident: %w", err)
		}
		
		return nil
	})
	
	responseTime := time.Since(start)
	c.stats.updateStats(err == nil, responseTime, err)
	
	if err != nil {
		return err
	}
	
	c.stats.incrementResolved()
	
	if c.config.EnableLogging {
		c.logger.Printf("Resolved incident %s", incidentID)
	}
	
	return nil
}

// GetIncident возвращает детали инцидента
func (c *incidentClient) GetIncident(ctx context.Context, incidentID string) (*v1.Incident, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident ID is required")
	}
	
	var incident *v1.Incident
	start := time.Now()
	
	err := c.executeWithRetry(ctx, func() error {
		req := &v1.GetIncidentRequest{
			IncidentId: incidentID,
		}
		
		resp, err := c.client.GetIncident(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to get incident: %w", err)
		}
		
		incident = resp.Incident
		return nil
	})
	
	responseTime := time.Since(start)
	c.stats.updateStats(err == nil, responseTime, err)
	
	if err != nil {
		return nil, err
	}
	
	return incident, nil
}

// ListIncidents возвращает список инцидентов
func (c *incidentClient) ListIncidents(ctx context.Context, tenantID string, status v1.IncidentStatus, severity v1.IncidentSeverity, pageSize, pageToken int32) ([]*v1.Incident, int32, error) {
	var incidents []*v1.Incident
	var nextPageToken int32
	start := time.Now()
	
	err := c.executeWithRetry(ctx, func() error {
		req := &v1.ListIncidentsRequest{
			TenantId:   tenantID,
			Status:     status,
			Severity:   severity,
			PageSize:   pageSize,
			PageToken:  pageToken,
		}
		
		resp, err := c.client.ListIncidents(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to list incidents: %w", err)
		}
		
		incidents = resp.Incidents
		nextPageToken = resp.NextPageToken
		return nil
	})
	
	responseTime := time.Since(start)
	c.stats.updateStats(err == nil, responseTime, err)
	
	if err != nil {
		return nil, 0, err
	}
	
	return incidents, nextPageToken, nil
}

// Close закрывает соединение
func (c *incidentClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.client = nil
		
		if c.config.EnableLogging {
			c.logger.Printf("Disconnected from Incident Manager")
		}
		
		return err
	}
	
	return nil
}

// GetStats возвращает статистику клиента
func (c *incidentClient) GetStats() *ClientStats {
	return c.stats.GetStats()
}
