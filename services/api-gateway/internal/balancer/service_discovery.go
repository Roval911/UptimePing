package balancer

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

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
	return &MockHealthChecker{address: address}
}

// NewGrpcHealthChecker создает новый gRPC health checker
func NewGrpcHealthChecker(address string, log logger.Logger) InstanceHealthChecker {
	// В реальной реализации здесь бы создавался gRPC HealthChecker
	// Пока используем мок для совместимости
	return &MockHealthChecker{address: address}
}

// MockHealthChecker мок для health checker (для тестов)
type MockHealthChecker struct {
	address string
}

func (m *MockHealthChecker) IsHealthy() bool {
	return true
}

func (m *MockHealthChecker) Address() string {
	return m.address
}

func (m *MockHealthChecker) LastSeen() time.Time {
	return time.Now()
}

func (m *MockHealthChecker) Close() error {
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
}

// NewStaticServiceDiscovery создает новый StaticServiceDiscovery
func NewStaticServiceDiscovery() *StaticServiceDiscovery {
	return &StaticServiceDiscovery{
		instances: make(map[string][]*Instance),
	}
}

// Register регистрирует инстансы для сервиса
func (s *StaticServiceDiscovery) Register(serviceName string, addresses []string, weights []int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	instances := make([]*Instance, 0, len(addresses))
	for i, address := range addresses {
		weight := 1
		if i < len(weights) {
			weight = weights[i]
		}
		healthChecker := NewInstanceHealthChecker(address, nil) // TODO: передать реальный логгер
		instance := NewInstance(address, healthChecker, weight)
		instances = append(instances, instance)
	}
	s.instances[serviceName] = instances
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

	return activeInstances, nil
}

// Watch отслеживает изменения в списке инстансов
func (s *StaticServiceDiscovery) Watch(ctx context.Context, serviceName string, callback func([]*Instance)) error {
	// В статическом режиме инстансы не изменяются динамически,
	// но мы можем отслеживать изменения в их состоянии (например, доступность)
	go func() {
		ticker := time.NewTicker(10 * time.Second) // Проверяем состояние каждые 10 секунд
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if instances, err := s.GetInstances(ctx, serviceName); err == nil {
					callback(instances)
				}
			}
		}
	}()

	return nil
}
