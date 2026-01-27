package balancer

import (
	"sync/atomic"

	"UptimePingPlatform/pkg/logger"
)

// RoundRobin реализует стратегию round-robin балансировки нагрузки
type RoundRobin struct {
	index uint64
	log   logger.Logger
}

// NewRoundRobin создает новый RoundRobin балансировщик
func NewRoundRobin(log logger.Logger) *RoundRobin {
	return &RoundRobin{
		log: log,
	}
}

// Select выбирает инстанс из списка доступных
func (r *RoundRobin) Select(instances []*Instance) *Instance {
	if len(instances) == 0 {
		r.log.Warn("No instances available for round-robin selection")
		return nil
	}

	// Получаем текущий индекс и увеличиваем его
	idx := atomic.AddUint64(&r.index, 1) - 1
	// Выбираем инстанс по индексу (с учетом размера списка)
	selected := instances[idx%uint64(len(instances))]

	r.log.Debug("Selected instance using round-robin",
		logger.String("address", selected.Address),
		logger.Int("index", int(idx%uint64(len(instances)))),
		logger.Int("total_instances", len(instances)))

	return selected
}
