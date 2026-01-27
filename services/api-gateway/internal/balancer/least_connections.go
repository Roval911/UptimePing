package balancer

import (
	"sync"

	"UptimePingPlatform/pkg/logger"
)

// LeastConnections реализует стратегию least-connections балансировки нагрузки
type LeastConnections struct {
	mu  sync.Mutex
	log logger.Logger
}

// NewLeastConnections создает новый LeastConnections балансировщик
func NewLeastConnections(log logger.Logger) *LeastConnections {
	return &LeastConnections{
		log: log,
	}
}

// Select выбирает инстанс с наименьшим количеством активных соединений
func (l *LeastConnections) Select(instances []*Instance) *Instance {
	if len(instances) == 0 {
		l.log.Warn("No instances available for least connections selection")
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

	if selected != nil {
		l.log.Debug("Selected instance with least connections",
			logger.String("address", selected.Address),
			logger.Int64("connections", minConnections))
	} else {
		l.log.Warn("No active instances found")
	}

	return selected
}
