package balancer

import (
	"sync"
	"sync/atomic"
	"time"
)

// RoundRobin реализует стратегию round-robin балансировки нагрузки
type RoundRobin struct {
	index uint64
}

// NewRoundRobin создает новый RoundRobin балансировщик
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{
		index: 0,
	}
}

// Select выбирает инстанс из списка доступных
func (r *RoundRobin) Select(instances []*Instance) *Instance {
	if len(instances) == 0 {
		return nil
	}

	// Получаем текущий индекс и увеличиваем его
	idx := atomic.AddUint64(&r.index, 1) - 1
	// Выбираем инстанс по индексу (с учетом размера списка)
	return instances[idx%uint64(len(instances))]
}

// Instance представляет инстанс сервиса
type Instance struct {
	Address    string
	Health     *HealthChecker
	Weight     int // Вес инстанса (для взвешенного round-robin)
	StartedAt  time.Time
	LastActive time.Time
	mu         sync.RWMutex
}

// NewInstance создает новый инстанс
func NewInstance(address string, health *HealthChecker, weight int) *Instance {
	return &Instance{
		Address:   address,
		Health:    health,
		Weight:    weight,
		StartedAt: time.Now(),
	}
}

// IsActive возвращает true, если инстанс активен
func (i *Instance) IsActive() bool {
	return i.Health.IsHealthy()
}

// UpdateLastActive обновляет время последней активности
func (i *Instance) UpdateLastActive() {
	i.mu.Lock()
	i.LastActive = time.Now()
	i.mu.Unlock()
}

// GetLastActive возвращает время последней активности
func (i *Instance) GetLastActive() time.Time {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.LastActive
}
