package balancer

import (
	"sync"
)

// LeastConnections реализует стратегию least-connections балансировки нагрузки
type LeastConnections struct {
	connections map[*Instance]int
	mu          sync.RWMutex
}

// NewLeastConnections создает новый LeastConnections балансировщик
func NewLeastConnections() *LeastConnections {
	return &LeastConnections{
		connections: make(map[*Instance]int),
	}
}

// Select выбирает инстанс с наименьшим количеством активных соединений
func (l *LeastConnections) Select(instances []*Instance) *Instance {
	if len(instances) == 0 {
		return nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	var selected *Instance
	minConnections := -1

	// Проходим по всем доступным инстансам
	for _, instance := range instances {
		// Пропускаем нездоровые инстансы
		if !instance.IsActive() {
			continue
		}

		// Получаем количество активных соединений
		connections := l.connections[instance]

		// Выбираем инстанс с наименьшим количеством соединений
		if selected == nil || connections < minConnections {
			selected = instance
			minConnections = connections
		}
	}

	return selected
}

// Increment увеличивает количество соединений для инстанса
func (l *LeastConnections) Increment(instance *Instance) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.connections[instance]++
	instance.UpdateLastActive()
}

// Decrement уменьшает количество соединений для инстанса
func (l *LeastConnections) Decrement(instance *Instance) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.connections[instance] > 0 {
		l.connections[instance]--
	}
}

// GetConnections возвращает количество активных соединений для инстанса
func (l *LeastConnections) GetConnections(instance *Instance) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.connections[instance]
}

// GetStats возвращает статистику по всем инстансам
func (l *LeastConnections) GetStats() map[string]int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := make(map[string]int)
	for instance, connections := range l.connections {
		stats[instance.Address] = connections
	}
	return stats
}

// Cleanup удаляет записи для инстансов, которые больше не в списке
func (l *LeastConnections) Cleanup(activeInstances []*Instance) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Создаем множество активных инстансов
	activeSet := make(map[*Instance]bool)
	for _, instance := range activeInstances {
		activeSet[instance] = true
	}

	// Удаляем записи для неактивных инстансов
	for instance := range l.connections {
		if !activeSet[instance] {
			delete(l.connections, instance)
		}
	}
}
