package balancer

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"

	"UptimePingPlatform/pkg/logger"
)

// Instance представляет инстанс сервиса
type Instance struct {
	Address       string
	Weight        int
	HealthChecker InstanceHealthChecker
	mu            sync.RWMutex
	active        bool
	connections   int64
}

// InstanceHealthChecker интерфейс для проверки здоровья инстанса
type InstanceHealthChecker interface {
	IsHealthy() bool
	Address() string
	LastSeen() time.Time
	Close() error
}

// NewInstance создает новый инстанс
func NewInstance(address string, healthChecker InstanceHealthChecker, weight int) *Instance {
	return &Instance{
		Address:       address,
		Weight:        weight,
		HealthChecker: healthChecker,
		active:        true,
	}
}

// IsActive возвращает true если инстанс активен
func (i *Instance) IsActive() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.active
}

// SetActive устанавливает активность инстанса
func (i *Instance) SetActive(active bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.active = active
}

// GetActiveConnections возвращает количество активных соединений
func (i *Instance) GetActiveConnections() int64 {
	return atomic.LoadInt64(&i.connections)
}

// IncrementConnections увеличивает счетчик соединений
func (i *Instance) IncrementConnections() {
	atomic.AddInt64(&i.connections, 1)
}

// DecrementConnections уменьшает счетчик соединений
func (i *Instance) DecrementConnections() {
	atomic.AddInt64(&i.connections, -1)
}

// NewInstanceHealthChecker создает новый health checker
func NewInstanceHealthChecker(address string, log logger.Logger) InstanceHealthChecker {
	return &MockHealthChecker{address: address, logger: log}
}

// NewGrpcHealthChecker создает новый gRPC health checker
func NewGrpcHealthChecker(address string, log logger.Logger) InstanceHealthChecker {
	return &GrpcHealthChecker{
		address: address,
		logger:  log,
	}
}

// GrpcHealthChecker реализует проверку здоровья через gRPC Health Checking Protocol
type GrpcHealthChecker struct {
	address   string
	logger    logger.Logger
	client    grpc_health_v1.HealthClient
	conn      *grpc.ClientConn
	mu        sync.RWMutex
	lastSeen  time.Time
	isHealthy bool
}

func (g *GrpcHealthChecker) IsHealthy() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Если клиент не инициализирован, пытаемся подключиться
	if g.client == nil {
		if err := g.connect(); err != nil {
			if g.logger != nil {
				g.logger.Error("Failed to connect to gRPC health service",
					logger.String("address", g.address),
					logger.Error(err))
			}
			g.isHealthy = false
			return false
		}
	}

	// Проверяем здоровье сервиса
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := g.client.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: "", // Проверяем здоровье всего сервиса
	})

	if err != nil {
		if g.logger != nil {
			g.logger.Error("Health check failed",
				logger.String("address", g.address),
				logger.Error(err))
		}
		g.isHealthy = false
		return false
	}

	g.lastSeen = time.Now()
	g.isHealthy = resp.Status == grpc_health_v1.HealthCheckResponse_SERVING

	if g.logger != nil {
		g.logger.Debug("Health check completed",
			logger.String("address", g.address),
			logger.String("status", resp.Status.String()),
			logger.Bool("healthy", g.isHealthy))
	}

	return g.isHealthy
}

func (g *GrpcHealthChecker) Address() string {
	return g.address
}

func (g *GrpcHealthChecker) LastSeen() time.Time {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.lastSeen
}

func (g *GrpcHealthChecker) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		err := g.conn.Close()
		g.conn = nil
		g.client = nil
		
		if g.logger != nil {
			g.logger.Info("gRPC health checker closed",
				logger.String("address", g.address),
				logger.Error(err))
		}
		return err
	}

	if g.logger != nil {
		g.logger.Info("gRPC health checker closed (no connection)",
			logger.String("address", g.address))
	}
	return nil
}

// connect устанавливает соединение с gRPC сервисом
func (g *GrpcHealthChecker) connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, g.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}

	g.conn = conn
	g.client = grpc_health_v1.NewHealthClient(conn)

	if g.logger != nil {
		g.logger.Info("Connected to gRPC health service",
			logger.String("address", g.address))
	}

	return nil
}

// MockHealthChecker мок для health checker (для тестов)
type MockHealthChecker struct {
	address string
	logger  logger.Logger
}

func (m *MockHealthChecker) IsHealthy() bool {
	if m.logger != nil {
		m.logger.Debug("Checking instance health",
			logger.String("address", m.address),
			logger.Bool("healthy", true))
	}
	return true
}

func (m *MockHealthChecker) Address() string {
	return m.address
}

func (m *MockHealthChecker) LastSeen() time.Time {
	if m.logger != nil {
		m.logger.Debug("Getting last seen time",
			logger.String("address", m.address))
	}
	return time.Now()
}

func (m *MockHealthChecker) Close() error {
	if m.logger != nil {
		m.logger.Info("Closing health checker",
			logger.String("address", m.address))
	}
	return nil
}

// ServiceDiscovery представляет механизм обнаружения сервисов
type ServiceDiscovery interface {
	// GetInstances возвращает список доступных инстансов сервиса
	GetInstances(ctx context.Context, serviceName string) ([]*Instance, error)
	// Watch отслеживает изменения в списке инстансов
	Watch(ctx context.Context, serviceName string, callback func([]*Instance)) error
}

// StaticServiceDiscovery реализует статическое обнаружение сервисов
// Инстансы задаются вручную и не изменяются
type StaticServiceDiscovery struct {
	instances map[string][]*Instance
	mu        sync.RWMutex
	logger    logger.Logger
}

// NewStaticServiceDiscovery создает новый StaticServiceDiscovery
func NewStaticServiceDiscovery(logger logger.Logger) *StaticServiceDiscovery {
	return &StaticServiceDiscovery{
		instances: make(map[string][]*Instance),
		logger:    logger,
	}
}

// Register регистрирует инстансы для сервиса
func (s *StaticServiceDiscovery) Register(serviceName string, addresses []string, weights []int) {
	s.RegisterWithHealthChecker(serviceName, addresses, weights, false)
}

// RegisterWithHealthChecker регистрирует инстансы с указанием типа health checker
func (s *StaticServiceDiscovery) RegisterWithHealthChecker(serviceName string, addresses []string, weights []int, useGrpcHealth bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Registering service instances",
		logger.String("service", serviceName),
		logger.Int("address_count", len(addresses)),
		logger.Bool("grpc_health", useGrpcHealth))

	instances := make([]*Instance, 0, len(addresses))
	for i, address := range addresses {
		weight := 1
		if i < len(weights) {
			weight = weights[i]
		}

		s.logger.Debug("Creating instance",
			logger.String("service", serviceName),
			logger.String("address", address),
			logger.Int("weight", weight),
			logger.Bool("grpc_health", useGrpcHealth))

		var healthChecker InstanceHealthChecker
		if useGrpcHealth {
			healthChecker = NewGrpcHealthChecker(address, s.logger)
		} else {
			healthChecker = NewInstanceHealthChecker(address, s.logger)
		}
		
		instance := NewInstance(address, healthChecker, weight)
		instances = append(instances, instance)
	}
	s.instances[serviceName] = instances

	s.logger.Info("Service instances registered successfully",
		logger.String("service", serviceName),
		logger.Int("total_instances", len(instances)))
}

// GetInstances возвращает список инстансов для сервиса
func (s *StaticServiceDiscovery) GetInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instances, exists := s.instances[serviceName]
	if !exists {
		return nil, nil
	}

	// Фильтруем только активные инстансы
	activeInstances := make([]*Instance, 0, len(instances))
	for _, instance := range instances {
		if instance.IsActive() {
			activeInstances = append(activeInstances, instance)
		}
	}

	s.logger.Debug("Retrieved service instances",
		logger.String("service", serviceName),
		logger.Int("total_instances", len(instances)),
		logger.Int("active_instances", len(activeInstances)))

	return activeInstances, nil
}

// Watch отслеживает изменения в списке инстансов
func (s *StaticServiceDiscovery) Watch(ctx context.Context, serviceName string, callback func([]*Instance)) error {
	// В статическом режиме инстансы не изменяются динамически,
	// но мы можем отслеживать изменения в их состоянии (например, доступность)
	s.logger.Info("Starting to watch service instances",
		logger.String("service", serviceName))

	go func() {
		ticker := time.NewTicker(10 * time.Second) // Проверяем состояние каждые 10 секунд
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("Stopping to watch service instances",
					logger.String("service", serviceName))
				return
			case <-ticker.C:
				if instances, err := s.GetInstances(ctx, serviceName); err == nil {
					s.logger.Debug("Service instances updated",
						logger.String("service", serviceName),
						logger.Int("count", len(instances)))
					callback(instances)
				} else {
					s.logger.Error("Failed to get service instances",
						logger.String("service", serviceName),
						logger.Error(err))
				}
			}
		}
	}()

	return nil
}

// Close закрывает все health checker соединения
func (s *StaticServiceDiscovery) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Closing service discovery")

	for serviceName, instances := range s.instances {
		for _, instance := range instances {
			if err := instance.HealthChecker.Close(); err != nil {
				s.logger.Error("Failed to close health checker",
					logger.String("service", serviceName),
					logger.String("address", instance.Address),
					logger.Error(err))
			}
		}
	}

	s.logger.Info("Service discovery closed successfully")
	return nil
}
