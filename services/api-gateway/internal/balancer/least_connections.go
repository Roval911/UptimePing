package balancer

import (
	"sync"
)

// LeastConnections реализует стратегию least-connections балансировки нагрузки
type LeastConnections struct {
	mu sync.Mutex
}

// NewLeastConnections создает новый LeastConnections балансировщик
func NewLeastConnections() *LeastConnections {
	return &LeastConnections{}
}

// Select выбирает инстанс с наименьшим количеством активных соединений
func (l *LeastConnections) Select(instances []*Instance) *Instance {
	if len(instances) == 0 {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var selected *Instance
	minConnections := int64(^uint64(0) >> 1) // Максимальное значение int64

	for _, instance := range instances {
		if instance.IsActive() {
			connections := instance.GetActiveConnections()
			if connections < minConnections {
				minConnections = connections
				selected = instance
			}
		}
	}

	return selected
}
