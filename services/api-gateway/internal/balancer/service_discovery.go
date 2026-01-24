package balancer

import (
	"context"
	"sync"
	"time"

	"UptimePingPlatform/pkg/logger"
)

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
		healthChecker := NewHealthChecker(address, nil) // Логгер будет установлен позже
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
		
		var lastActiveCount int
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				instances, _ := s.GetInstances(ctx, serviceName)
				
				// Проверяем, изменилось ли количество активных инстансов
				// Это позволяет детектировать изменения в состоянии health check
				activeCount := len(instances)
				if activeCount != lastActiveCount {
					callback(instances)
					lastActiveCount = activeCount
				}
			}
		}
	}()
	return nil
}

// DynamicServiceDiscovery реализует динамическое обнаружение сервисов
// Пока пустая реализация, может быть расширена для работы с Consul, etcd и т.д.
type DynamicServiceDiscovery struct {
	// TODO: Реализовать интеграцию с реальными системами обнаружения сервисов
	// На данный момент в проекте нет интеграции с Consul, etcd или другими системами service discovery
	// Реализация отложена до добавления соответствующих зависимостей
}

// NewDynamicServiceDiscovery создает новый DynamicServiceDiscovery
func NewDynamicServiceDiscovery() *DynamicServiceDiscovery {
	return &DynamicServiceDiscovery{}
}

// GetInstances возвращает список инстансов для сервиса
func (d *DynamicServiceDiscovery) GetInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	// На данный момент в проекте нет интеграции с Consul, etcd или другими системами service discovery
	// Возвращаем пустой список, так как нет реальной реализации
	return []*Instance{}, nil
}

// Watch отслеживает изменения в списке инстансов
func (d *DynamicServiceDiscovery) Watch(ctx context.Context, serviceName string, callback func([]*Instance)) error {
	// На данный момент в проекте нет интеграции с Consul, etcd или другими системами service discovery
	// Имитируем постоянное наблюдение с пустым списком инстансов
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				callback([]*Instance{})
			}
		}
	}()
	return nil
}

// SetLogger устанавливает логгер для всех health checkers
func (s *StaticServiceDiscovery) SetLogger(log logger.Logger) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, instances := range s.instances {
		for _, instance := range instances {
			instance.Health.log = log
		}
	}
}