package balancer

import (
	"sync/atomic"
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
