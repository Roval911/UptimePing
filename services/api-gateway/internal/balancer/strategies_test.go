package balancer

import (
	"sync"
	"testing"

	"UptimePingPlatform/pkg/logger"
)

// nopLogger - простая реализация logger для тестов
type nopLogger struct{}

func (l *nopLogger) Debug(msg string, fields ...logger.Field)  {}
func (l *nopLogger) Info(msg string, fields ...logger.Field)   {}
func (l *nopLogger) Warn(msg string, fields ...logger.Field)   {}
func (l *nopLogger) Error(msg string, fields ...logger.Field)  {}
func (l *nopLogger) With(fields ...logger.Field) logger.Logger { return l }
func (l *nopLogger) Sync() error                               { return nil }

// TestRoundRobin_NewRoundRobin тестирует создание RoundRobin балансировщика
func TestRoundRobin_NewRoundRobin(t *testing.T) {
	nopLog := &nopLogger{}
	rr := NewRoundRobin(nopLog)

	if rr == nil {
		t.Fatal("RoundRobin should not be nil")
	}

	// Проверяем начальный индекс
	if rr.index != 0 {
		t.Errorf("Expected initial index 0, got %d", rr.index)
	}
}

// TestRoundRobin_Select тестирует выбор инстансов
func TestRoundRobin_Select(t *testing.T) {
	nopLog := &nopLogger{}
	rr := NewRoundRobin(nopLog)

	// Тест с пустым списком
	instances := []*Instance{}
	selected := rr.Select(instances)

	if selected != nil {
		t.Error("Select should return nil for empty instances")
	}

	// Тест с одним инстансом
	mockChecker := &MockHealthChecker{address: "localhost:50051"}
	singleInstance := []*Instance{
		NewInstance("localhost:50051", mockChecker, 1),
	}

	selected = rr.Select(singleInstance)
	if selected == nil {
		t.Error("Select should not return nil for non-empty instances")
	}

	if selected != singleInstance[0] {
		t.Error("Select should return the only available instance")
	}

	// Создаем новый RoundRobin для чистого теста нескольких инстансов
	nopLog2 := &nopLogger{}
	rr2 := NewRoundRobin(nopLog2)
	instances = []*Instance{
		NewInstance("localhost:50051", mockChecker, 1),
		NewInstance("localhost:50052", mockChecker, 1),
		NewInstance("localhost:50053", mockChecker, 1),
	}

	// Делаем несколько выборов для проверки round-robin
	selections := make([]*Instance, 9) // 3 цикла по 3 инстанса

	for i := 0; i < 9; i++ {
		selections[i] = rr2.Select(instances)
		if selections[i] == nil {
			t.Errorf("Selection %d should not be nil", i)
		}
	}

	// Проверяем round-robin последовательность
	expected := []int{0, 1, 2, 0, 1, 2, 0, 1, 2}
	for i, expectedIndex := range expected {
		if selections[i] != instances[expectedIndex] {
			t.Errorf("Selection %d: expected instance %d, got %v", i, expectedIndex, selections[i])
		}
	}
}

// TestRoundRobin_ConcurrentSelect тестирует конкурентный выбор
func TestRoundRobin_ConcurrentSelect(t *testing.T) {
	nopLog := &nopLogger{}
	rr := NewRoundRobin(nopLog)

	mockChecker := &MockHealthChecker{address: "localhost:50051"}
	instances := []*Instance{
		NewInstance("localhost:50051", mockChecker, 1),
		NewInstance("localhost:50052", mockChecker, 1),
		NewInstance("localhost:50053", mockChecker, 1),
		NewInstance("localhost:50054", mockChecker, 1),
	}

	const numGoroutines = 100
	const numSelections = 1000

	selectionCounts := make(map[string]int)
	var mu sync.Mutex

	// Запускаем несколько горутин для конкурентного выбора
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numSelections; j++ {
				selected := rr.Select(instances)
				if selected != nil {
					mu.Lock()
					selectionCounts[selected.Address]++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// Проверяем, что все инстансы были выбраны
	if len(selectionCounts) != 4 {
		t.Errorf("Expected all 4 instances to be selected, got %d", len(selectionCounts))
	}

	// Проверяем распределение (должно быть примерно равномерным)
	totalSelections := numGoroutines * numSelections
	expectedPerInstance := totalSelections / 4
	tolerance := expectedPerInstance / 10 // 10% tolerance

	for address, count := range selectionCounts {
		if count < expectedPerInstance-tolerance || count > expectedPerInstance+tolerance {
			t.Errorf("Instance %s selected %d times, expected around %d (±%d)",
				address, count, expectedPerInstance, tolerance)
		}
	}
}

// TestLeastConnections_NewLeastConnections тестирует создание LeastConnections балансировщика
func TestLeastConnections_NewLeastConnections(t *testing.T) {
	nopLog := &nopLogger{}
	lc := NewLeastConnections(nopLog)

	if lc == nil {
		t.Fatal("LeastConnections should not be nil")
	}
}

// TestLeastConnections_Select тестирует выбор инстанса с наименьшими соединениями
func TestLeastConnections_Select(t *testing.T) {
	nopLog := &nopLogger{}
	lc := NewLeastConnections(nopLog)

	// Тест с пустым списком
	instances := []*Instance{}
	selected := lc.Select(instances)

	if selected != nil {
		t.Error("Select should return nil for empty instances")
	}

	// Тест с одним инстансом
	mockChecker := &MockHealthChecker{address: "localhost:50051"}
	singleInstance := []*Instance{
		NewInstance("localhost:50051", mockChecker, 1),
	}

	selected = lc.Select(singleInstance)
	if selected == nil {
		t.Error("Select should not return nil for non-empty instances")
	}

	if selected != singleInstance[0] {
		t.Error("Select should return the only available instance")
	}

	// Тест с несколькими инстансами и разным количеством соединений
	instances = []*Instance{
		NewInstance("localhost:50051", mockChecker, 1),
		NewInstance("localhost:50052", mockChecker, 1),
		NewInstance("localhost:50053", mockChecker, 1),
	}

	// Устанавливаем разное количество соединений
	instances[0].IncrementConnections() // 1 соединение
	instances[0].IncrementConnections() // 2 соединения
	instances[1].IncrementConnections() // 1 соединение
	// instances[2] - 0 соединений

	// Должен быть выбран инстанс с 0 соединений
	selected = lc.Select(instances)
	if selected != instances[2] {
		t.Errorf("Expected instance with 0 connections, got instance with %d connections",
			selected.GetActiveConnections())
	}

	// Увеличиваем соединения у третьего инстанса
	instances[2].IncrementConnections() // 1 соединение
	instances[2].IncrementConnections() // 2 соединения

	// Теперь должен быть выбран инстанс с 1 соединением (второй)
	selected = lc.Select(instances)
	if selected != instances[1] {
		t.Errorf("Expected instance with 1 connection, got instance with %d connections",
			selected.GetActiveConnections())
	}
}

// TestLeastConnections_Select_InactiveInstances тестирует выбор только активных инстансов
func TestLeastConnections_Select_InactiveInstances(t *testing.T) {
	nopLog := &nopLogger{}
	lc := NewLeastConnections(nopLog)

	mockChecker := &MockHealthChecker{address: "localhost:50051"}
	instances := []*Instance{
		NewInstance("localhost:50051", mockChecker, 1),
		NewInstance("localhost:50052", mockChecker, 1),
		NewInstance("localhost:50053", mockChecker, 1),
	}

	// Деактивируем первый и третий инстансы
	instances[0].SetActive(false)
	instances[2].SetActive(false)

	// Должен быть выбран только активный инстанс (второй)
	selected := lc.Select(instances)
	if selected == nil {
		t.Error("Select should not return nil when there are active instances")
	}

	if selected != instances[1] {
		t.Error("Select should return the only active instance")
	}

	// Деактивируем все инстансы
	instances[1].SetActive(false)

	// Теперь должен вернуться nil
	selected = lc.Select(instances)
	if selected != nil {
		t.Error("Select should return nil when all instances are inactive")
	}
}

// TestLeastConnections_Select_EqualConnections тестирует выбор при равном количестве соединений
func TestLeastConnections_Select_EqualConnections(t *testing.T) {
	nopLog := &nopLogger{}
	lc := NewLeastConnections(nopLog)

	mockChecker := &MockHealthChecker{address: "localhost:50051"}
	instances := []*Instance{
		NewInstance("localhost:50051", mockChecker, 1),
		NewInstance("localhost:50052", mockChecker, 1),
		NewInstance("localhost:50053", mockChecker, 1),
	}

	// Все инстансы имеют одинаковое количество соединений (0)
	selected := lc.Select(instances)
	if selected == nil {
		t.Error("Select should not return nil")
	}

	// Должен быть выбран первый инстанс (первый с минимальным количеством)
	if selected != instances[0] {
		t.Error("Select should return the first instance when all have equal connections")
	}

	// Увеличиваем соединения у всех инстансов
	for _, instance := range instances {
		instance.IncrementConnections()
	}

	// Все еще должны иметь равное количество соединений
	selected = lc.Select(instances)
	if selected != instances[0] {
		t.Error("Select should return the first instance when all have equal connections")
	}
}

// TestLeastConnections_ConcurrentSelect тестирует конкурентный выбор
func TestLeastConnections_ConcurrentSelect(t *testing.T) {
	nopLog := &nopLogger{}
	lc := NewLeastConnections(nopLog)

	// Создаем уникальные health checker для каждого инстанса
	instances := []*Instance{
		NewInstance("localhost:50051", &MockHealthChecker{address: "localhost:50051"}, 1),
		NewInstance("localhost:50052", &MockHealthChecker{address: "localhost:50052"}, 1),
		NewInstance("localhost:50053", &MockHealthChecker{address: "localhost:50053"}, 1),
		NewInstance("localhost:50054", &MockHealthChecker{address: "localhost:50054"}, 1),
	}

	const numGoroutines = 10  // Уменьшаем для надежности
	const numSelections = 100 // Уменьшаем для надежности

	selectionCounts := make(map[string]int)
	var mu sync.Mutex

	// Запускаем несколько горутин для конкурентного выбора
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numSelections; j++ {
				selected := lc.Select(instances)
				if selected != nil {
					mu.Lock()
					selectionCounts[selected.Address]++
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	// Проверяем, что был выбран хотя бы один инстанс
	if len(selectionCounts) == 0 {
		t.Error("Expected at least 1 instance to be selected")
	}

	// Для least connections с одинаковым количеством соединений,
	// ожидаем что будет выбран первый инстанс чаще всего
	t.Logf("Selection counts:")
	for address, count := range selectionCounts {
		t.Logf("Instance %s selected %d times", address, count)
	}

	// Проверяем, что общее количество выборов правильное
	totalSelections := 0
	for _, count := range selectionCounts {
		totalSelections += count
	}
	expectedTotal := numGoroutines * numSelections
	if totalSelections != expectedTotal {
		t.Errorf("Expected total selections %d, got %d", expectedTotal, totalSelections)
	}
}

// BenchmarkRoundRobin_Select бенчмарк для RoundRobin выбора
func BenchmarkRoundRobin_Select(b *testing.B) {
	nopLog := &nopLogger{}
	rr := NewRoundRobin(nopLog)

	mockChecker := &MockHealthChecker{address: "localhost:50051"}
	instances := make([]*Instance, 100)
	for i := 0; i < 100; i++ {
		instances[i] = NewInstance("localhost:50051", mockChecker, 1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rr.Select(instances)
	}
}

// BenchmarkLeastConnections_Select бенчмарк для LeastConnections выбора
func BenchmarkLeastConnections_Select(b *testing.B) {
	nopLog := &nopLogger{}
	lc := NewLeastConnections(nopLog)

	mockChecker := &MockHealthChecker{address: "localhost:50051"}
	instances := make([]*Instance, 100)
	for i := 0; i < 100; i++ {
		instances[i] = NewInstance("localhost:50051", mockChecker, 1)
		// Устанавливаем разное количество соединений
		for j := 0; j < i%10; j++ {
			instances[i].IncrementConnections()
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lc.Select(instances)
	}
}
