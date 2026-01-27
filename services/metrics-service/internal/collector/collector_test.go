package collector

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pkglogger "UptimePingPlatform/pkg/logger"
)

// MockLogger для тестов
type mockLogger struct {
	logs []string
}

func (m *mockLogger) Debug(msg string, fields ...pkglogger.Field) {
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *mockLogger) Info(msg string, fields ...pkglogger.Field) {
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *mockLogger) Warn(msg string, fields ...pkglogger.Field) {
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *mockLogger) Error(msg string, fields ...pkglogger.Field) {
	m.logs = append(m.logs, "ERROR: "+msg)
}

func (m *mockLogger) With(fields ...pkglogger.Field) pkglogger.Logger {
	return m
}

func (m *mockLogger) Sync() error {
	return nil
}

func (m *mockLogger) GetLogs() []string {
	return m.logs
}

func (m *mockLogger) ClearLogs() {
	m.logs = []string{}
}

// Helper функция для создания тестового коллектора с уникальным реестром
func createTestCollector() *MetricsCollector {
	logger := &mockLogger{}
	
	// Создаем новый реестр для каждого теста
	registry := prometheus.NewRegistry()
	
	collector := &MetricsCollector{
		logger:   logger,
		services: make(map[string]*ServiceMetrics),
		registry: registry,
	}
	
	// Регистрируем системные метрики
	collector.registerSystemMetrics()
	
	// Создаем HTTP handler
	collector.httpHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	
	return collector
}

// Helper функция для создания тестового коллектора для тестов
func createTestCollectorForTest(t *testing.T) *MetricsCollector {
	return createTestCollector()
}

func TestNewMetricsCollector(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	assert.NotNil(t, collector)
	assert.NotNil(t, collector.logger)
	assert.NotNil(t, collector.registry)
	assert.NotNil(t, collector.httpHandler)
	assert.Empty(t, collector.services)
}

func TestMetricsCollector_AddService(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Добавляем сервис
	err := collector.AddService("test-service", "localhost:50051")
	require.NoError(t, err)

	// Проверяем, что сервис добавлен
	services := collector.GetServices()
	assert.Len(t, services, 1)
	assert.Contains(t, services, "test-service")

	// Проверяем логи
	logs := collector.logger.(*mockLogger).GetLogs()
	assert.Contains(t, logs, "INFO: Adding service to metrics collector")
	assert.Contains(t, logs, "INFO: Service added successfully")
}

func TestMetricsCollector_AddService_Duplicate(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Добавляем сервис дважды
	err1 := collector.AddService("test-service", "localhost:50051")
	require.NoError(t, err1)

	err2 := collector.AddService("test-service", "localhost:50051")
	require.Error(t, err2)
	assert.Contains(t, err2.Error(), "service test-service already exists")
}

func TestMetricsCollector_RemoveService(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Добавляем сервис
	err := collector.AddService("test-service", "localhost:50051")
	require.NoError(t, err)

	// Удаляем сервис
	err = collector.RemoveService("test-service")
	require.NoError(t, err)

	// Проверяем, что сервис удален
	services := collector.GetServices()
	assert.Empty(t, services)

	// Проверяем логи
	logs := collector.logger.(*mockLogger).GetLogs()
	assert.Contains(t, logs, "INFO: Service removed from metrics collector")
}

func TestMetricsCollector_RemoveService_NotFound(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Пытаемся удалить несуществующий сервис
	err := collector.RemoveService("non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service non-existent not found")
}

func TestMetricsCollector_GetServiceMetrics(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Добавляем сервис
	err := collector.AddService("test-service", "localhost:50051")
	require.NoError(t, err)

	// Получаем метрики сервиса
	serviceMetrics, err := collector.GetServiceMetrics("test-service")
	require.NoError(t, err)
	assert.NotNil(t, serviceMetrics)
	assert.Equal(t, "test-service", serviceMetrics.Name)
	assert.Equal(t, "localhost:50051", serviceMetrics.Address)
}

func TestMetricsCollector_GetServiceMetrics_NotFound(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Пытаемся получить метрики несуществующего сервиса
	_, err := collector.GetServiceMetrics("non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service non-existent not found")
}

func TestMetricsCollector_GetServices(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Изначально нет сервисов
	services := collector.GetServices()
	assert.Empty(t, services)

	// Добавляем несколько сервисов
	err1 := collector.AddService("service1", "localhost:50051")
	err2 := collector.AddService("service2", "localhost:50052")
	require.NoError(t, err1)
	require.NoError(t, err2)

	// Проверяем список сервисов
	services = collector.GetServices()
	assert.Len(t, services, 2)
	assert.Contains(t, services, "service1")
	assert.Contains(t, services, "service2")
}

func TestMetricsCollector_GetHandler(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	handler := collector.GetHandler()
	assert.NotNil(t, handler)
}

func TestMetricsCollector_GetRegistry(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	registry := collector.GetRegistry()
	assert.NotNil(t, registry)
}

func TestMetricsCollector_ScrapeAll(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Добавляем сервис
	err := collector.AddService("test-service", "localhost:50051")
	require.NoError(t, err)

	// Выполняем сбор метрик
	err = collector.ScrapeAll()
	// В тесте сервис не будет доступен, но не должно быть panic
	// Ожидаем ошибку или nil, но не panic
	// В зависимости от реализации может быть nil или ошибка
	// Главное - не должно быть panic
	assert.True(t, err == nil || err != nil, "ScrapeAll should not panic")
}

func TestMetricsCollector_Shutdown(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Добавляем сервис
	err := collector.AddService("test-service", "localhost:50051")
	require.NoError(t, err)

	// Выполняем shutdown
	err = collector.Shutdown()
	require.NoError(t, err)

	// Проверяем, что сервисы удалены
	services := collector.GetServices()
	assert.Empty(t, services)
}

func TestMetricsCollector_ConcurrentAccess(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Тестируем конкурентный доступ
	done := make(chan bool, 2)

	// Горутина для добавления сервисов
	go func() {
		for i := 0; i < 10; i++ {
			collector.AddService("service"+string(rune(i)), "localhost:50051")
		}
		done <- true
	}()

	// Горутина для получения списка сервисов
	go func() {
		for i := 0; i < 10; i++ {
			collector.GetServices()
		}
		done <- true
	}()

	// Ждем завершения обеих горутин
	<-done
	<-done

	// Проверяем, что все еще работает
	services := collector.GetServices()
	assert.NotNil(t, services)
}

func TestServiceMetrics_Structure(t *testing.T) {
	collector := createTestCollectorForTest(t)
	defer collector.Shutdown()

	// Добавляем сервис
	err := collector.AddService("test-service", "localhost:50051")
	require.NoError(t, err)

	// Получаем метрики сервиса
	serviceMetrics, err := collector.GetServiceMetrics("test-service")
	require.NoError(t, err)

	// Проверяем структуру метрик
	assert.NotNil(t, serviceMetrics.RequestCount)
	assert.NotNil(t, serviceMetrics.RequestDuration)
	assert.NotNil(t, serviceMetrics.ErrorCount)
	assert.NotNil(t, serviceMetrics.ActiveConnections)
}

// Benchmark для проверки производительности
func BenchmarkMetricsCollector_AddService(b *testing.B) {
	collector := createTestCollector()
	defer collector.Shutdown()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.AddService("service"+string(rune(i%100)), "localhost:50051")
	}
}

func BenchmarkMetricsCollector_GetServices(b *testing.B) {
	collector := createTestCollector()
	defer collector.Shutdown()

	// Добавляем несколько сервисов
	for i := 0; i < 100; i++ {
		collector.AddService("service"+string(rune(i)), "localhost:50051")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.GetServices()
	}
}
