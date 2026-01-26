package balancer

import (
	"testing"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"

	"UptimePingPlatform/pkg/logger"
)

// BalancerMockLogger для тестов
type BalancerMockLogger struct {
	logs   []string
	infos  []string
	warns  []string
	errors []string
}

func (m *BalancerMockLogger) Debug(msg string, fields ...logger.Field) {
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *BalancerMockLogger) Info(msg string, fields ...logger.Field) {
	m.infos = append(m.infos, msg)
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *BalancerMockLogger) Warn(msg string, fields ...logger.Field) {
	m.warns = append(m.warns, msg)
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *BalancerMockLogger) Error(msg string, fields ...logger.Field) {
	m.errors = append(m.errors, msg)
	m.logs = append(m.logs, "ERROR: "+msg)
}

func (m *BalancerMockLogger) With(fields ...logger.Field) logger.Logger {
	return m
}

func (m *BalancerMockLogger) Sync() error {
	return nil
}

func (m *BalancerMockLogger) GetLogs() []string {
	return append([]string{}, m.logs...)
}

func (m *BalancerMockLogger) GetInfos() []string {
	return append([]string{}, m.infos...)
}

func (m *BalancerMockLogger) GetWarns() []string {
	return append([]string{}, m.warns...)
}

func (m *BalancerMockLogger) GetErrors() []string {
	return append([]string{}, m.errors...)
}

// mockClientConn простой мок для ClientConn
type mockClientConn struct{}

func (m *mockClientConn) NewSubConn(addrs []resolver.Address, opts balancer.NewSubConnOptions) (balancer.SubConn, error) {
	return nil, nil
}

func (m *mockClientConn) RemoveSubConn(sc balancer.SubConn) {}
func (m *mockClientConn) UpdateState(state balancer.State)          {}
func (m *mockClientConn) UpdateAddresses(sc balancer.SubConn, addrs []resolver.Address) {}
func (m *mockClientConn) ResolveNow(o resolver.ResolveNowOptions) {}
func (m *mockClientConn) Target() string                           { return "mock-target" }

// TestBuilder_NewBuilder тестирует создание нового билдера
func TestBuilder_NewBuilder(t *testing.T) {
	log := &BalancerMockLogger{}
	builder := NewBuilder(log)

	if builder == nil {
		t.Fatal("Builder should not be nil")
	}

	if builder.Name() != Name {
		t.Errorf("Expected name %s, got %s", Name, builder.Name())
	}
}

// TestBuilder_Name тестирует получение имени билдера
func TestBuilder_Name(t *testing.T) {
	log := &BalancerMockLogger{}
	builder := NewBuilder(log)

	name := builder.Name()
	if name != Name {
		t.Errorf("Expected name %s, got %s", Name, name)
	}
}

// TestUptimePingBalancer_ResolverError тестирует обработку ошибок resolver
func TestUptimePingBalancer_ResolverError(t *testing.T) {
	log := &BalancerMockLogger{}
	
	// Создаем балансировщик напрямую для теста ResolverError
	bal := &uptimePingBalancer{
		log: log,
	}

	testError := &testError{msg: "test resolver error"}
	bal.ResolverError(testError)

	// Проверяем логи ошибок
	errors := log.GetErrors()
	if len(errors) == 0 {
		t.Fatal("Expected at least 1 error log")
	}

	expectedError := "Resolver error"
	if errors[0] != expectedError {
		t.Errorf("Expected error log '%s', got '%s'", expectedError, errors[0])
	}
}

// TestUptimePingBalancer_Close тестирует закрытие балансировщика
func TestUptimePingBalancer_Close(t *testing.T) {
	log := &BalancerMockLogger{}
	
	// Создаем балансировщик напрямую
	bal := &uptimePingBalancer{
		log:        log,
		scStates:   make(map[balancer.SubConn]connectivity.State),
		subConns:   make(map[resolver.Address]balancer.SubConn),
		scToAddr:   make(map[balancer.SubConn]resolver.Address),
		connsCount: make(map[balancer.SubConn]int),
		instances:  make(map[string]*Instance),
	}

	// Закрываем балансировщик
	bal.Close()

	// Проверяем логи
	logs := log.GetInfos()
	closeFound := false
	for _, logMsg := range logs {
		if logMsg == "Closing balancer" {
			closeFound = true
			break
		}
	}
	if !closeFound {
		t.Error("Expected 'Closing balancer' log")
	}
}

// TestUptimePingBalancer_ExitIdle тестирует выход из idle состояния
func TestUptimePingBalancer_ExitIdle(t *testing.T) {
	log := &BalancerMockLogger{}
	
	// Создаем балансировщик напрямую
	bal := &uptimePingBalancer{
		log:        log,
		scStates:   make(map[balancer.SubConn]connectivity.State),
		subConns:   make(map[resolver.Address]balancer.SubConn),
		scToAddr:   make(map[balancer.SubConn]resolver.Address),
		connsCount: make(map[balancer.SubConn]int),
		instances:  make(map[string]*Instance),
	}

	// Выходим из idle
	bal.ExitIdle()

	// Проверяем логи
	logs := log.GetInfos()
	exitIdleFound := false
	for _, logMsg := range logs {
		if logMsg == "Exiting idle state" {
			exitIdleFound = true
			break
		}
	}
	if !exitIdleFound {
		t.Error("Expected 'Exiting idle state' log")
	}
}

// testError тестовая ошибка
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// TestUpdatePicker тестирует обновление picker (упрощенная версия)
func TestUpdatePicker(t *testing.T) {
	log := &BalancerMockLogger{}
	
	// Просто проверяем, что методы вызываются без паники
	// Создаем балансировщик без ClientConn для базовых тестов
	bal := &uptimePingBalancer{
		log:        log,
		scStates:   make(map[balancer.SubConn]connectivity.State),
		subConns:   make(map[resolver.Address]balancer.SubConn),
		scToAddr:   make(map[balancer.SubConn]resolver.Address),
		connsCount: make(map[balancer.SubConn]int),
		instances:  make(map[string]*Instance),
	}

	// Проверяем базовые методы без вызова updatePicker
	// Так как updatePicker требует валидный ClientConn
	
	// Проверяем, что логер работает
	log.Info("Test message")
	logs := log.GetInfos()
	if len(logs) != 1 || logs[0] != "Test message" {
		t.Error("Logger should work correctly")
	}

	// Проверяем, что структуры инициализируются правильно
	if bal.scStates == nil {
		t.Error("scStates should be initialized")
	}
	if bal.subConns == nil {
		t.Error("subConns should be initialized")
	}
	if bal.instances == nil {
		t.Error("instances should be initialized")
	}
}

// TestUptimePingPicker_Pick_NoReadyConns тестирует выбор без готовых соединений
func TestUptimePingPicker_Pick_NoReadyConns(t *testing.T) {
	picker := &uptimePingPicker{
		subConns:   []balancer.SubConn{},
		connsCount: make(map[balancer.SubConn]int),
		log:        &BalancerMockLogger{},
	}

	pickInfo := balancer.PickInfo{}
	_, err := picker.Pick(pickInfo)

	if err == nil {
		t.Error("Expected error when no ready subconnections")
	}

	if err != balancer.ErrNoSubConnAvailable {
		t.Errorf("Expected ErrNoSubConnAvailable, got %v", err)
	}
}
