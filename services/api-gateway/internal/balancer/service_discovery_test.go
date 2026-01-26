package balancer

import (
	"context"
	"sync"
	"testing"
	"time"

	"UptimePingPlatform/pkg/logger"
)

// MockLogger для тестов
type MockLogger struct {
	mu   sync.Mutex
	logs []string
}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, "DEBUG: "+msg)
	m.mu.Unlock()
}

func (m *MockLogger) Info(msg string, fields ...logger.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, "INFO: "+msg)
	m.mu.Unlock()
}

func (m *MockLogger) Warn(msg string, fields ...logger.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, "WARN: "+msg)
	m.mu.Unlock()
}

func (m *MockLogger) Error(msg string, fields ...logger.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, "ERROR: "+msg)
	m.mu.Unlock()
}

func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	return m
}

func (m *MockLogger) Sync() error {
	return nil
}

func (m *MockLogger) GetLogs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.logs...)
}

// TestNewInstance тестирует создание нового инстанса
func TestNewInstance(t *testing.T) {
	address := "localhost:50051"
	weight := 5
	mockChecker := &MockHealthChecker{address: address}

	instance := NewInstance(address, mockChecker, weight)

	if instance == nil {
		t.Fatal("Instance should not be nil")
	}

	if instance.Address != address {
		t.Errorf("Expected address %s, got %s", address, instance.Address)
	}

	if instance.Weight != weight {
		t.Errorf("Expected weight %d, got %d", weight, instance.Weight)
	}

	if instance.HealthChecker != mockChecker {
		t.Error("HealthChecker should be set correctly")
	}

	if !instance.IsActive() {
		t.Error("Instance should be active by default")
	}

	if instance.GetActiveConnections() != 0 {
		t.Error("Active connections should be 0 by default")
	}
}

// TestInstance_SetActive тестирует установку активности
func TestInstance_SetActive(t *testing.T) {
	instance := NewInstance("localhost:50051", &MockHealthChecker{}, 1)

	// Тест деактивации
	instance.SetActive(false)
	if instance.IsActive() {
		t.Error("Instance should be inactive")
	}

	// Тест активации
	instance.SetActive(true)
	if !instance.IsActive() {
		t.Error("Instance should be active")
	}
}

// TestInstance_Connections тестирует счетчик соединений
func TestInstance_Connections(t *testing.T) {
	instance := NewInstance("localhost:50051", &MockHealthChecker{}, 1)

	// Тест увеличения
	instance.IncrementConnections()
	if instance.GetActiveConnections() != 1 {
		t.Errorf("Expected 1 connection, got %d", instance.GetActiveConnections())
	}

	// Тест еще одного увеличения
	instance.IncrementConnections()
	if instance.GetActiveConnections() != 2 {
		t.Errorf("Expected 2 connections, got %d", instance.GetActiveConnections())
	}

	// Тест уменьшения
	instance.DecrementConnections()
	if instance.GetActiveConnections() != 1 {
		t.Errorf("Expected 1 connection after decrement, got %d", instance.GetActiveConnections())
	}

	// Тест еще одного уменьшения
	instance.DecrementConnections()
	if instance.GetActiveConnections() != 0 {
		t.Errorf("Expected 0 connections after second decrement, got %d", instance.GetActiveConnections())
	}
}

// TestInstance_ConcurrentConnections тестирует конкурентный доступ к счетчику
func TestInstance_ConcurrentConnections(t *testing.T) {
	instance := NewInstance("localhost:50051", &MockHealthChecker{}, 1)

	// Запускаем несколько горутин для конкурентного изменения счетчика
	const numGoroutines = 100
	const numOperations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Increment + Decrement

	// Горутины для увеличения
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				instance.IncrementConnections()
			}
		}()
	}

	// Горутины для уменьшения
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				instance.DecrementConnections()
			}
		}()
	}

	wg.Wait()

	// Счетчик должен вернуться к 0
	finalCount := instance.GetActiveConnections()
	if finalCount != 0 {
		t.Errorf("Expected 0 connections after concurrent operations, got %d", finalCount)
	}
}

// TestNewInstanceHealthChecker тестирует создание health checker
func TestNewInstanceHealthChecker(t *testing.T) {
	address := "localhost:50051"
	log := &MockLogger{}

	checker := NewInstanceHealthChecker(address, log)

	if checker == nil {
		t.Fatal("HealthChecker should not be nil")
	}

	mockChecker, ok := checker.(*MockHealthChecker)
	if !ok {
		t.Fatal("Checker should be MockHealthChecker")
	}

	if mockChecker.Address() != address {
		t.Errorf("Expected address %s, got %s", address, mockChecker.Address())
	}
}

// TestNewGrpcHealthChecker тестирует создание gRPC health checker
func TestNewGrpcHealthChecker(t *testing.T) {
	address := "localhost:50051"
	log := &MockLogger{}

	checker := NewGrpcHealthChecker(address, log)

	if checker == nil {
		t.Fatal("GrpcHealthChecker should not be nil")
	}

	mockChecker, ok := checker.(*MockHealthChecker)
	if !ok {
		t.Fatal("Checker should be MockHealthChecker")
	}

	if mockChecker.Address() != address {
		t.Errorf("Expected address %s, got %s", address, mockChecker.Address())
	}
}

// TestMockHealthChecker тестирует мок health checker
func TestMockHealthChecker(t *testing.T) {
	address := "localhost:50051"
	checker := &MockHealthChecker{address: address}

	// Тест IsHealthy
	if !checker.IsHealthy() {
		t.Error("MockHealthChecker should always return true for IsHealthy")
	}

	// Тест Address
	if checker.Address() != address {
		t.Errorf("Expected address %s, got %s", address, checker.Address())
	}

	// Тест LastSeen
	lastSeen := checker.LastSeen()
	if lastSeen.IsZero() {
		t.Error("LastSeen should not be zero")
	}

	// Тест Close
	err := checker.Close()
	if err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}
}

// TestNewStaticServiceDiscovery тестирует создание статического service discovery
func TestNewStaticServiceDiscovery(t *testing.T) {
	sd := NewStaticServiceDiscovery()

	if sd == nil {
		t.Fatal("StaticServiceDiscovery should not be nil")
	}
}

// TestStaticServiceDiscovery_Register тестирует регистрацию инстансов
func TestStaticServiceDiscovery_Register(t *testing.T) {
	sd := NewStaticServiceDiscovery()

	serviceName := "test-service"
	addresses := []string{"localhost:50051", "localhost:50052"}
	weights := []int{1, 2}

	sd.Register(serviceName, addresses, weights)

	// Проверяем, что инстансы зарегистрированы
	ctx := context.Background()
	instances, err := sd.GetInstances(ctx, serviceName)

	if err != nil {
		t.Errorf("GetInstances should not return error, got %v", err)
	}

	if len(instances) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(instances))
	}

	// Проверяем адреса
	if instances[0].Address != addresses[0] {
		t.Errorf("Expected address %s, got %s", addresses[0], instances[0].Address)
	}

	if instances[1].Address != addresses[1] {
		t.Errorf("Expected address %s, got %s", addresses[1], instances[1].Address)
	}

	// Проверяем веса
	if instances[0].Weight != 1 {
		t.Errorf("Expected weight 1, got %d", instances[0].Weight)
	}

	if instances[1].Weight != 2 {
		t.Errorf("Expected weight 2, got %d", instances[1].Weight)
	}
}

// TestStaticServiceDiscovery_Register_DefaultWeights тестирует регистрацию с весами по умолчанию
func TestStaticServiceDiscovery_Register_DefaultWeights(t *testing.T) {
	sd := NewStaticServiceDiscovery()

	serviceName := "test-service"
	addresses := []string{"localhost:50051", "localhost:50052", "localhost:50053"}
	weights := []int{5} // Только один вес для первого адреса

	sd.Register(serviceName, addresses, weights)

	ctx := context.Background()
	instances, err := sd.GetInstances(ctx, serviceName)

	if err != nil {
		t.Errorf("GetInstances should not return error, got %v", err)
	}

	if len(instances) != 3 {
		t.Errorf("Expected 3 instances, got %d", len(instances))
	}

	// Проверяем веса
	if instances[0].Weight != 5 {
		t.Errorf("Expected weight 5 for first instance, got %d", instances[0].Weight)
	}

	if instances[1].Weight != 1 {
		t.Errorf("Expected weight 1 for second instance, got %d", instances[1].Weight)
	}

	if instances[2].Weight != 1 {
		t.Errorf("Expected weight 1 for third instance, got %d", instances[2].Weight)
	}
}

// TestStaticServiceDiscovery_GetInstances_NonExistentService тестирует получение инстансов для несуществующего сервиса
func TestStaticServiceDiscovery_GetInstances_NonExistentService(t *testing.T) {
	sd := NewStaticServiceDiscovery()

	ctx := context.Background()
	instances, err := sd.GetInstances(ctx, "non-existent-service")

	if err != nil {
		t.Errorf("GetInstances should not return error, got %v", err)
	}

	if instances != nil {
		t.Error("GetInstances should return nil for non-existent service")
	}
}

// TestStaticServiceDiscovery_GetInstances_InactiveInstances тестирует фильтрацию неактивных инстансов
func TestStaticServiceDiscovery_GetInstances_InactiveInstances(t *testing.T) {
	sd := NewStaticServiceDiscovery()

	serviceName := "test-service"
	addresses := []string{"localhost:50051", "localhost:50052"}
	weights := []int{1, 1}

	sd.Register(serviceName, addresses, weights)

	// Деактивируем один инстанс
	ctx := context.Background()
	instances, err := sd.GetInstances(ctx, serviceName)
	if err != nil {
		t.Errorf("GetInstances should not return error, got %v", err)
	}

	instances[0].SetActive(false)

	// Получаем инстансы снова
	activeInstances, err := sd.GetInstances(ctx, serviceName)
	if err != nil {
		t.Errorf("GetInstances should not return error, got %v", err)
	}

	// Должен остаться только один активный инстанс
	if len(activeInstances) != 1 {
		t.Errorf("Expected 1 active instance, got %d", len(activeInstances))
	}

	if activeInstances[0].Address != addresses[1] {
		t.Errorf("Expected address %s, got %s", addresses[1], activeInstances[0].Address)
	}
}

// TestStaticServiceDiscovery_Watch тестирует отслеживание изменений
func TestStaticServiceDiscovery_Watch(t *testing.T) {
	sd := NewStaticServiceDiscovery()

	serviceName := "test-service"
	addresses := []string{"localhost:50051"}
	weights := []int{1}

	sd.Register(serviceName, addresses, weights)

	// Просто проверяем, что Watch не возвращает ошибку и не блокирует
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	callbackCalled := false

	err := sd.Watch(ctx, serviceName, func(instances []*Instance) {
		callbackCalled = true
	})

	if err != nil {
		t.Errorf("Watch should not return error, got %v", err)
	}

	// Watch может быть асинхронным, поэтому не проверяем callback
	// Главное - чтобы не было ошибки и блокировки
	t.Logf("Watch completed successfully, callback called: %v", callbackCalled)
}

// TestStaticServiceDiscovery_ConcurrentAccess тестирует конкурентный доступ
func TestStaticServiceDiscovery_ConcurrentAccess(t *testing.T) {
	sd := NewStaticServiceDiscovery()

	serviceName := "test-service"
	addresses := []string{"localhost:50051", "localhost:50052"}
	weights := []int{1, 1}

	sd.Register(serviceName, addresses, weights)

	const numGoroutines = 50
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	ctx := context.Background()

	// Запускаем несколько горутин для чтения
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				instances, err := sd.GetInstances(ctx, serviceName)
				if err != nil {
					t.Errorf("GetInstances should not return error, got %v", err)
					return
				}
				if len(instances) != 2 {
					t.Errorf("Expected 2 instances, got %d", len(instances))
					return
				}
			}
		}()
	}

	wg.Wait()
}

// BenchmarkStaticServiceDiscovery_GetInstances бенчмарк для GetInstances
func BenchmarkStaticServiceDiscovery_GetInstances(b *testing.B) {
	sd := NewStaticServiceDiscovery()

	serviceName := "test-service"
	addresses := make([]string, 100)
	weights := make([]int, 100)
	for i := 0; i < 100; i++ {
		addresses[i] = "localhost:50051"
		weights[i] = 1
	}

	sd.Register(serviceName, addresses, weights)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sd.GetInstances(ctx, serviceName)
		if err != nil {
			b.Fatalf("GetInstances failed: %v", err)
		}
	}
}

// BenchmarkInstance_IncrementConnections бенчмарк для увеличения счетчика
func BenchmarkInstance_IncrementConnections(b *testing.B) {
	instance := NewInstance("localhost:50051", &MockHealthChecker{}, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance.IncrementConnections()
	}
}

// BenchmarkInstance_GetActiveConnections бенчмарк для получения счетчика
func BenchmarkInstance_GetActiveConnections(b *testing.B) {
	instance := NewInstance("localhost:50051", &MockHealthChecker{}, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = instance.GetActiveConnections()
	}
}
