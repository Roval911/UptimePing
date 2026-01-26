package balancer

import (
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
	bal := &uptimePingBalancer{
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

// uptimePingBalancer реализует интерфейс балансировки нагрузки
type uptimePingBalancer struct {
	cc         balancer.ClientConn
	log        logger.Logger
	mu         sync.Mutex
	scStates   map[balancer.SubConn]connectivity.State
	subConns   map[resolver.Address]balancer.SubConn
	scToAddr   map[balancer.SubConn]resolver.Address
	connsCount map[balancer.SubConn]int
	instances  map[string]*Instance
	picker     balancer.Picker
}

// ClientConn возвращает клиентское соединение
func (b *uptimePingBalancer) ClientConn() balancer.ClientConn {
	return b.cc
}

// UpdateClientConnState обновляет состояние соединения
func (b *uptimePingBalancer) UpdateClientConnState(ccs balancer.ClientConnState) error {
	b.log.Info("Updating client connection state")
	b.mu.Lock()
	defer b.mu.Unlock()

	// Получаем список адресов из resolver
	addresses := ccs.ResolverState.Addresses
	if len(addresses) == 0 {
		b.log.Warn("No addresses available")
		return nil
	}

	// Создаем новые SubConn для новых адресов
	for _, addr := range addresses {
		if _, exists := b.subConns[addr]; !exists {
			sc, err := b.cc.NewSubConn([]resolver.Address{addr}, balancer.NewSubConnOptions{})
			if err != nil {
				b.log.Error("Failed to create subconnection", logger.String("address", addr.Addr), logger.Error(err))
				continue
			}
			b.subConns[addr] = sc
			b.scToAddr[sc] = addr
			b.scStates[sc] = connectivity.Idle
			b.connsCount[sc] = 0
			sc.Connect()
			b.log.Info("Created new subconnection", logger.String("address", addr.Addr))
		}
	}

	// Удаляем SubConn для адресов, которых больше нет
	for addr, sc := range b.subConns {
		found := false
		for _, resolverAddr := range addresses {
			if addr == resolverAddr {
				found = true
				break
			}
		}
		if !found {
			sc.Shutdown()
			delete(b.subConns, addr)
			delete(b.scToAddr, sc)
			delete(b.scStates, sc)
			delete(b.connsCount, sc)
			b.log.Info("Removed subconnection", logger.String("address", addr.Addr))
		}
	}

	b.updatePicker()
	return nil
}

// ResolverError обрабатывает ошибки resolver
func (b *uptimePingBalancer) ResolverError(err error) {
	b.log.Error("Resolver error", logger.Error(err))
}

// UpdateSubConnState обновляет состояние подсоединения
func (b *uptimePingBalancer) UpdateSubConnState(sc balancer.SubConn, scs balancer.SubConnState) {
	b.log.Info("Updating subconnection state", logger.String("state", scs.ConnectivityState.String()))
	b.mu.Lock()
	defer b.mu.Unlock()

	oldState, exists := b.scStates[sc]
	if !exists {
		b.log.Warn("Unknown subconnection")
		return
	}

	b.scStates[sc] = scs.ConnectivityState

	// Если состояние изменилось на Ready, обновляем picker
	if oldState != connectivity.Ready && scs.ConnectivityState == connectivity.Ready {
		b.log.Info("Subconnection is ready")
		b.updatePicker()
	} else if oldState == connectivity.Ready && scs.ConnectivityState != connectivity.Ready {
		b.log.Warn("Subconnection is not ready anymore", logger.String("state", scs.ConnectivityState.String()))
		b.updatePicker()
	}
}

// Close закрывает балансировщик
func (b *uptimePingBalancer) Close() {
	b.log.Info("Closing balancer")
	b.mu.Lock()
	defer b.mu.Unlock()

	// Закрываем все SubConn
	for _, sc := range b.subConns {
		sc.Shutdown()
	}

	// Очищаем все карты
	b.subConns = make(map[resolver.Address]balancer.SubConn)
	b.scToAddr = make(map[balancer.SubConn]resolver.Address)
	b.scStates = make(map[balancer.SubConn]connectivity.State)
	b.connsCount = make(map[balancer.SubConn]int)
	b.instances = make(map[string]*Instance)
}

// ExitIdle выходит из idle состояния
func (b *uptimePingBalancer) ExitIdle() {
	b.log.Info("Exiting idle state")
	b.mu.Lock()
	defer b.mu.Unlock()

	// Переподключаем все неактивные SubConn
	for sc, state := range b.scStates {
		if state == connectivity.Idle {
			sc.Connect()
			b.log.Info("Reconnecting idle subconnection")
		}
	}
}

// updatePicker обновляет стратегию выбора соединений
func (b *uptimePingBalancer) updatePicker() {
	// Собираем список готовых SubConn
	readySCs := make([]balancer.SubConn, 0)
	for sc, state := range b.scStates {
		if state == connectivity.Ready {
			readySCs = append(readySCs, sc)
		}
	}

	if len(readySCs) == 0 {
		b.picker = base.NewErrPicker(balancer.ErrNoSubConnAvailable)
		b.log.Warn("No ready subconnections available")
	} else {
		b.picker = &uptimePingPicker{
			subConns:   readySCs,
			connsCount: b.connsCount,
			log:        b.log,
		}
		b.log.Info("Picker updated", logger.Int("ready_connections", len(readySCs)))
	}

	b.cc.UpdateState(balancer.State{Picker: b.picker})
}

// uptimePingPicker реализует стратегию выбора соединений
type uptimePingPicker struct {
	subConns   []balancer.SubConn
	connsCount map[balancer.SubConn]int
	log        logger.Logger
	mu         sync.Mutex
}

// Pick выбирает SubConn для запроса
func (p *uptimePingPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.subConns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	// Простая round-robin стратегия
	// В будущем можно реализовать более сложные стратегии
	selected := p.subConns[0]

	// Перемещаем выбранный SubConn в конец для round-robin
	p.subConns = append(p.subConns[1:], selected)

	p.log.Debug("Picked subconnection")

	return balancer.PickResult{SubConn: selected}, nil
}
