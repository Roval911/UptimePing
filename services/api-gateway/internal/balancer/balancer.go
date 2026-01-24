package balancer

import (
	"context"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"

	"UptimePingPlatform/pkg/logger"
)

// Name имя балансировщика
const Name = "uptimeping"

// Builder создает новый балансировщик
type Builder struct {
	log logger.Logger
}

// NewBuilder создает новый Builder
func NewBuilder(log logger.Logger) balancer.Builder {
	return &Builder{log: log}
}

// Build создает новый балансировщик
func (b *Builder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	bal := &Balancer{
		cc:         cc,
		log:        b.log,
		scStates:   make(map[balancer.SubConn]connectivity.State),
		subConns:   make(map[resolver.Address]balancer.SubConn),
		scToAddr:   make(map[balancer.SubConn]resolver.Address),
		connsCount: make(map[balancer.SubConn]int),
		instances:  make(map[string]*Instance),
		picker:     base.NewErrPicker(balancer.ErrNoSubConnAvailable),
	}

	bal.log.Info("Balancer created")
	return bal
}

// Name возвращает имя балансировщика
func (b *Builder) Name() string {
	return Name
}

// Balancer реализует интерфейс балансировки нагрузки
type Balancer struct {
	cc  balancer.ClientConn
	log logger.Logger
	mu  sync.RWMutex

	// Состояние SubConn
	scStates map[balancer.SubConn]connectivity.State
	// Маппинг адресов на SubConn
	subConns map[resolver.Address]balancer.SubConn
	// Обратный маппинг SubConn на адреса
	scToAddr map[balancer.SubConn]resolver.Address
	// Количество активных соединений для каждого SubConn
	connsCount map[balancer.SubConn]int
	// Инстансы для health checking
	instances map[string]*Instance
	// Текущий picker
	picker balancer.Picker

	ctx    context.Context
	cancel context.CancelFunc
}

// UpdateClientConnState обновляет состояние соединения клиента
func (b *Balancer) UpdateClientConnState(s balancer.ClientConnState) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.log.Debug("UpdateClientConnState called",
		logger.Int("address_count", len(s.ResolverState.Addresses)))

	// Обновляем SubConn на основе новых адресов
	addrsSet := make(map[resolver.Address]struct{})
	for _, addr := range s.ResolverState.Addresses {
		addrsSet[addr] = struct{}{}

		// Если SubConn для этого адреса еще не существует, создаем его
		if _, ok := b.subConns[addr]; !ok {
			sc, err := b.cc.NewSubConn([]resolver.Address{addr}, balancer.NewSubConnOptions{})
			if err != nil {
				b.log.Error("Failed to create new SubConn",
					logger.String("address", addr.Addr),
					logger.String("error", err.Error()))
				continue
			}

			b.subConns[addr] = sc
			b.scToAddr[sc] = addr
			b.scStates[sc] = connectivity.Idle
			b.connsCount[sc] = 0

			// Создаем инстанс для health checking
			healthChecker := NewHealthChecker(addr.Addr, b.log)
			instance := NewInstance(addr.Addr, healthChecker, 0)
			b.instances[addr.Addr] = instance

			// Запускаем контекст для health checking если он еще не запущен
			if b.ctx == nil {
				b.ctx, b.cancel = context.WithCancel(context.Background())
			}

			// Запускаем health checking
			go func(addr string, instance *Instance) {
				if err := instance.Health.Start(b.ctx); err != nil {
					b.log.Error("Failed to start health check",
						logger.String("address", addr),
						logger.String("error", err.Error()))
				}
			}(addr.Addr, instance)

			sc.Connect()
			b.log.Debug("Created new SubConn", logger.String("address", addr.Addr))
		}
	}

	// Удаляем SubConn для адресов, которых больше нет
	for addr, sc := range b.subConns {
		if _, ok := addrsSet[addr]; !ok {
			b.cc.RemoveSubConn(sc)
			delete(b.subConns, addr)
			delete(b.scToAddr, sc)
			delete(b.scStates, sc)
			delete(b.connsCount, sc)

			// Закрываем health checker для этого инстанса
			if instance, ok := b.instances[addr.Addr]; ok {
				instance.Health.Close()
				delete(b.instances, addr.Addr)
			}

			b.log.Debug("Removed SubConn", logger.String("address", addr.Addr))
		}
	}

	// Обновляем picker
	b.updatePicker()
	return nil
}

// ResolverError обрабатывает ошибки резолвера
func (b *Balancer) ResolverError(err error) {
	b.log.Error("Resolver error", logger.String("error", err.Error()))
}

// UpdateSubConnState обновляет состояние подсоединения
func (b *Balancer) UpdateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	b.mu.Lock()
	defer b.mu.Unlock()

	oldState, ok := b.scStates[sc]
	if !ok {
		b.log.Warn("UpdateSubConnState called for unknown SubConn")
		return
	}

	b.scStates[sc] = state.ConnectivityState
	b.log.Debug("SubConn state changed",
		logger.String("address", b.scToAddr[sc].Addr),
		logger.String("old_state", oldState.String()),
		logger.String("new_state", state.ConnectivityState.String()))

	// Обновляем состояние инстанса на основе health checking
	if addr, ok := b.scToAddr[sc]; ok {
		if instance, ok := b.instances[addr.Addr]; ok {
			// Если состояние изменилось на Ready и health checker здоров, помечаем как активный
			if state.ConnectivityState == connectivity.Ready && instance.Health.IsHealthy() {
				// Используем метод SetActive из Instance
				instance.SetActive(true)
			} else {
				instance.SetActive(false)
			}
		}
	}

	// Обновляем picker
	b.updatePicker()
}

// ExitIdle вызывается, когда клиент хочет выйти из idle состояния
func (b *Balancer) ExitIdle() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.log.Debug("ExitIdle called")

	// Пытаемся переподключить все idle соединения
	for sc, state := range b.scStates {
		if state == connectivity.Idle || state == connectivity.TransientFailure {
			sc.Connect()
			b.log.Debug("Reconnecting SubConn", logger.String("address", b.scToAddr[sc].Addr))
		}
	}
}

// Close закрывает балансировщик
func (b *Balancer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.log.Info("Balancer closing")

	if b.cancel != nil {
		b.cancel()
	}

	// Закрываем все health checkers
	for _, instance := range b.instances {
		instance.Health.Close()
	}

	b.instances = nil
	b.subConns = nil
	b.scToAddr = nil
	b.scStates = nil
	b.connsCount = nil
}

// updatePicker обновляет picker на основе текущего состояния
func (b *Balancer) updatePicker() {
	// Собираем доступные SubConn
	var readyScs []balancer.SubConn
	for sc, state := range b.scStates {
		if state == connectivity.Ready {
			// Проверяем health инстанса
			if addr, ok := b.scToAddr[sc]; ok {
				if instance, ok := b.instances[addr.Addr]; ok && instance.IsActive() {
					readyScs = append(readyScs, sc)
				}
			}
		}
	}

	if len(readyScs) > 0 {
		// Используем round-robin стратегию
		b.picker = &rrPicker{
			subConns: readyScs,
			next:     0,
			mu:       sync.Mutex{},
			log:      b.log,
		}
		b.log.Debug("Picker updated with ready connections", logger.Int("ready_count", len(readyScs)))
	} else {
		// Нет доступных соединений
		b.picker = base.NewErrPicker(balancer.ErrNoSubConnAvailable)
		b.log.Warn("No ready connections available")
	}

	b.cc.UpdateState(balancer.State{
		ConnectivityState: b.determineOverallState(),
		Picker:            b.picker,
	})
}

// determineOverallState определяет общее состояние балансировщика
func (b *Balancer) determineOverallState() connectivity.State {
	hasReady := false
	hasConnecting := false
	hasIdle := false

	for _, state := range b.scStates {
		switch state {
		case connectivity.Ready:
			hasReady = true
		case connectivity.Connecting:
			hasConnecting = true
		case connectivity.Idle:
			hasIdle = true
		}
	}

	if hasReady {
		return connectivity.Ready
	} else if hasConnecting {
		return connectivity.Connecting
	} else if hasIdle {
		return connectivity.Idle
	}
	return connectivity.TransientFailure
}

// Register регистрирует балансировщик в gRPC
func Register(log logger.Logger) {
	balancer.Register(NewBuilder(log))
}

// SetLogger устанавливает логгер
func (b *Builder) SetLogger(log logger.Logger) {
	b.log = log
}

// rrPicker реализует round-robin стратегию выбора SubConn
type rrPicker struct {
	subConns []balancer.SubConn
	next     int
	mu       sync.Mutex
	log      logger.Logger
}

// Pick выбирает SubConn для запроса
func (p *rrPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.subConns) == 0 {
		p.log.Error("No subconns available in picker")
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	sc := p.subConns[p.next]
	p.next = (p.next + 1) % len(p.subConns)

	p.log.Debug("Picked SubConn", logger.Int("index", p.next))

	return balancer.PickResult{
		SubConn: sc,
		Done: func(info balancer.DoneInfo) {
			// Здесь можно добавить логику обработки завершения запроса
			// Например, отслеживание ошибок или обновление метрик
			if info.Err != nil {
				p.log.Debug("Request completed with error",
					logger.String("error", info.Err.Error()))
			}
		},
	}, nil
}
