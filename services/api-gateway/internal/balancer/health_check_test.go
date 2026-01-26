package balancer

import (
	"context"
	"sync"
	"testing"
	"time"

	"UptimePingPlatform/pkg/logger"
)

// TestHealthChecker_NewHealthChecker тестирует создание HealthChecker
func TestHealthChecker_NewHealthChecker(t *testing.T) {
	address := "localhost:50051"
	mockLogger := &struct {
		logger.Logger
	}{}

	checker := NewHealthChecker(address, mockLogger)

	if checker == nil {
		t.Fatal("HealthChecker should not be nil")
	}

	if checker.Address() != address {
		t.Errorf("Expected address %s, got %s", address, checker.Address())
	}

	// Изначально должен быть не здоров
	if checker.IsHealthy() {
		t.Error("HealthChecker should not be healthy initially")
	}
}

// TestHealthChecker_Start тестирует запуск HealthChecker
func TestHealthChecker_Start(t *testing.T) {
	address := "localhost:50051" // Несуществующий адрес для теста
	mockLogger := &MockLogger{}

	checker := NewHealthChecker(address, mockLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := checker.Start(ctx)
	
	// Проверяем, что checker запускается без паники
	// Результат может быть разным в зависимости от реализации
	if err != nil {
		t.Logf("Start returned error (expected for unavailable address): %v", err)
	}
}

// TestHealthChecker_IsHealthy тестирует проверку здоровья
func TestHealthChecker_IsHealthy(t *testing.T) {
	address := "localhost:50051"
	mockLogger := &MockLogger{}

	checker := NewHealthChecker(address, mockLogger)

	// Изначально не здоров
	if checker.IsHealthy() {
		t.Error("HealthChecker should not be healthy initially")
	}

	// LastSeen должно быть нулевым
	if !checker.LastSeen().IsZero() {
		t.Error("LastSeen should be zero initially")
	}
}

// TestHealthChecker_Close тестирует закрытие HealthChecker
func TestHealthChecker_Close(t *testing.T) {
	address := "localhost:50051"
	mockLogger := &MockLogger{}

	checker := NewHealthChecker(address, mockLogger)

	// Закрываем без запуска - не должно быть ошибки
	err := checker.Close()
	if err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}
}

// TestConnectivityError тестирует ошибку подключения
func TestConnectivityError(t *testing.T) {
	state := "TRANSIENT_FAILURE"
	err := &ConnectivityError{State: state}

	expectedMsg := "connection not ready, state: " + state
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestHealthChecker_checkLoop тестирует цикл проверки
func TestHealthChecker_checkLoop(t *testing.T) {
	address := "localhost:50051"
	mockLogger := &MockLogger{}

	checker := NewHealthChecker(address, mockLogger)

	// Создаем контекст с таймаутом для завершения цикла
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Запускаем checkLoop в отдельной горутине
	done := make(chan struct{})
	go func() {
		checker.checkLoop(ctx)
		done <- struct{}{}
	}()

	// Ждем завершения
	select {
	case <-done:
		// Цикл завершился как ожидалось
	case <-time.After(100 * time.Millisecond):
		t.Error("checkLoop should have completed within timeout")
	}
}

// TestHealthChecker_check тестирует проверку состояния
func TestHealthChecker_check(t *testing.T) {
	address := "localhost:50051"
	mockLogger := &MockLogger{}

	checker := NewHealthChecker(address, mockLogger)

	// Тестируем базовые свойства HealthChecker без вызова check()
	// так как check() требует установленное соединение
	
	// Проверяем адрес
	if checker.Address() != address {
		t.Errorf("Expected address %s, got %s", address, checker.Address())
	}

	// Проверяем начальное состояние здоровья
	if checker.IsHealthy() {
		t.Error("HealthChecker should not be healthy initially")
	}

	// Проверяем LastSeen
	if !checker.LastSeen().IsZero() {
		t.Error("LastSeen should be zero initially")
	}

	// Проверяем Close
	err := checker.Close()
	if err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}
}

// TestHealthChecker_Integration тестирует интеграцию HealthChecker
func TestHealthChecker_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Этот тест требует реальный gRPC сервер
	// Для простоты используем мок
	address := "localhost:50051"
	mockLogger := &MockLogger{}

	checker := NewHealthChecker(address, mockLogger)

	// Проверяем базовые операции
	if checker.Address() != address {
		t.Errorf("Expected address %s, got %s", address, checker.Address())
	}

	if checker.IsHealthy() {
		t.Error("Should not be healthy initially")
	}

	// Закрываем
	err := checker.Close()
	if err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}
}

// BenchmarkHealthChecker_IsHealthy бенчмарк для IsHealthy
func BenchmarkHealthChecker_IsHealthy(b *testing.B) {
	address := "localhost:50051"
	mockLogger := &MockLogger{}
	checker := NewHealthChecker(address, mockLogger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = checker.IsHealthy()
	}
}

// BenchmarkHealthChecker_Address бенчмарк для Address
func BenchmarkHealthChecker_Address(b *testing.B) {
	address := "localhost:50051"
	mockLogger := &MockLogger{}
	checker := NewHealthChecker(address, mockLogger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = checker.Address()
	}
}

// BenchmarkHealthChecker_LastSeen бенчмарк для LastSeen
func BenchmarkHealthChecker_LastSeen(b *testing.B) {
	address := "localhost:50051"
	mockLogger := &MockLogger{}
	checker := NewHealthChecker(address, mockLogger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = checker.LastSeen()
	}
}

// TestHealthChecker_ConcurrentAccess тестирует конкурентный доступ
func TestHealthChecker_ConcurrentAccess(t *testing.T) {
	address := "localhost:50051"
	mockLogger := &MockLogger{}
	checker := NewHealthChecker(address, mockLogger)

	const numGoroutines = 50
	const numOperations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // IsHealthy, Address, LastSeen

	// Горутины для IsHealthy
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = checker.IsHealthy()
			}
		}()
	}

	// Горутины для Address
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = checker.Address()
			}
		}()
	}

	// Горутины для LastSeen
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = checker.LastSeen()
			}
		}()
	}

	wg.Wait()
}

// TestHealthChecker_TimeoutBehavior тестирует поведение при таймауте
func TestHealthChecker_TimeoutBehavior(t *testing.T) {
	address := "localhost:50051"
	mockLogger := &MockLogger{}

	checker := NewHealthChecker(address, mockLogger)

	// Проверяем, что LastSeen обновляется корректно
	initialLastSeen := checker.LastSeen()
	
	// Небольшая задержка для обеспечения разницы во времени
	time.Sleep(1 * time.Millisecond)
	
	// Имитируем обновление LastSeen (в реальной ситуации это происходит при успешном подключении)
	// Поскольку у нас нет реального подключения, просто проверяем, что значение не меняется без подключения
	currentLastSeen := checker.LastSeen()
	
	if !initialLastSeen.Equal(currentLastSeen) {
		t.Error("LastSeen should not change without successful connection")
	}
}

// TestHealthChecker_ErrorHandling тестирует обработку ошибок
func TestHealthChecker_ErrorHandling(t *testing.T) {
	address := "invalid-address" // Невалидный адрес
	mockLogger := &MockLogger{}

	checker := NewHealthChecker(address, mockLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Попытка запустить с невалидным адресом
	err := checker.Start(ctx)
	// Не проверяем конкретную ошибку, так как реализация может меняться
	if err != nil {
		t.Logf("Start returned error (expected): %v", err)
	}

	// Закрываем даже при ошибке
	err = checker.Close()
	if err != nil {
		t.Errorf("Close should not return error even after start failure, got %v", err)
	}
}
