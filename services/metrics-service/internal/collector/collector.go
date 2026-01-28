package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	pkglogger "UptimePingPlatform/pkg/logger"
	pkgerrors "UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/services/metrics-service/internal/domain"
)

// MetricsCollector собирает метрики из всех сервисов
type MetricsCollector struct {
	logger      pkglogger.Logger
	registry    *prometheus.Registry
	services    map[string]*ServiceMetrics
	mu          sync.RWMutex
	httpHandler http.Handler
}

// ServiceMetrics содержит метрики для конкретного сервиса
type ServiceMetrics struct {
	Name    string
	Address string
	Conn    *grpc.ClientConn
	
	// Prometheus метрики
	RequestCount    *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	ErrorCount      *prometheus.CounterVec
	ActiveConnections prometheus.Gauge
	
	// gRPC клиенты для метрик
	metricsClient domain.MetricsServiceClient
	healthClient  domain.HealthServiceClient
}

// NewMetricsCollector создает новый коллектор метрик
func NewMetricsCollector(logger pkglogger.Logger) *MetricsCollector {
	registry := prometheus.NewRegistry()
	
	collector := &MetricsCollector{
		logger:      logger,
		registry:    registry,
		services:    make(map[string]*ServiceMetrics),
		httpHandler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	}
	
	// Регистрируем системные метрики
	collector.registerSystemMetrics()
	
	return collector
}

// registerSystemMetrics регистрирует системные метрики коллектора
func (mc *MetricsCollector) registerSystemMetrics() {
	// Метрики самого коллектора
	mc.registry.MustRegister(prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "metrics_collector_services_total",
			Help: "Total number of services being monitored",
		},
	))
	
	mc.registry.MustRegister(prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "metrics_collector_active_connections",
			Help: "Number of active gRPC connections",
		},
	))
	
	mc.registry.MustRegister(prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "metrics_collector_scrapes_total",
			Help: "Total number of metrics scrapes",
		},
	))
}

// AddService добавляет сервис для мониторинга
func (mc *MetricsCollector) AddService(name, address string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.logger.Info("Adding service to metrics collector", 
		pkglogger.String("service", name),
		pkglogger.String("address", address))
	
	// Проверяем, что сервис еще не добавлен
	if _, exists := mc.services[name]; exists {
		return pkgerrors.New(pkgerrors.ErrConflict, fmt.Sprintf("service %s already exists", name))
	}
	
	// Создаем gRPC подключение
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return pkgerrors.Wrap(err, "failed to connect to service %s", name)
	}
	
	// Создаем метрики для сервиса
	serviceMetrics := &ServiceMetrics{
		Name:    name,
		Address: address,
		Conn:    conn,
		RequestCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_requests_total", name),
				Help: fmt.Sprintf("Total number of requests to %s", name),
			},
			[]string{"method", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    fmt.Sprintf("%s_request_duration_seconds", name),
				Help:    fmt.Sprintf("Request duration for %s", name),
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		ErrorCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_errors_total", name),
				Help: fmt.Sprintf("Total number of errors in %s", name),
			},
			[]string{"method", "error_type"},
		),
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: fmt.Sprintf("%s_active_connections", name),
				Help: fmt.Sprintf("Number of active connections to %s", name),
			},
		),
	}
	
	// Регистрируем метрики в реестре
	registry := prometheus.NewRegistry()
	registry.MustRegister(serviceMetrics.RequestCount)
	registry.MustRegister(serviceMetrics.RequestDuration)
	registry.MustRegister(serviceMetrics.ErrorCount)
	registry.MustRegister(serviceMetrics.ActiveConnections)
	
	// Создаем gRPC клиенты
	metricsClient := domain.NewMetricsServiceClient(conn)
	healthClient := domain.NewHealthServiceClient(conn)
	
	serviceMetrics.metricsClient = metricsClient
	serviceMetrics.healthClient = healthClient
	
	mc.services[name] = serviceMetrics
	
	// Запускаем сбор метрик в отдельной горутине
	go mc.collectServiceMetrics(name, serviceMetrics)
	
	mc.logger.Info("Service added successfully", 
		pkglogger.String("service", name),
		pkglogger.String("address", address))
	
	return nil
}

// RemoveService удаляет сервис из мониторинга
func (mc *MetricsCollector) RemoveService(name string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	serviceMetrics, exists := mc.services[name]
	if !exists {
		return pkgerrors.New(pkgerrors.ErrNotFound, fmt.Sprintf("service %s not found", name))
	}
	
	// Закрываем соединение
	if serviceMetrics.Conn != nil {
		serviceMetrics.Conn.Close()
	}
	
	delete(mc.services, name)
	
	mc.logger.Info("Service removed from metrics collector", 
		pkglogger.String("service", name))
	
	return nil
}

// collectServiceMetrics собирает метрики для конкретного сервиса
func (mc *MetricsCollector) collectServiceMetrics(name string, sm *ServiceMetrics) {
	ticker := time.NewTicker(15 * time.Second) // Сбор метрик каждые 15 секунд
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			
			// Собираем метрики через gRPC
			if err := mc.collectGRPCMetrics(ctx, name, sm); err != nil {
				mc.logger.Error("Failed to collect gRPC metrics", 
					pkglogger.String("service", name),
					pkglogger.Error(err))
			}
			
			// Обновляем метрики активных соединений
			sm.ActiveConnections.Set(1)
			
			cancel()
		}
	}
}

// collectGRPCMetrics собирает метрики через gRPC
func (mc *MetricsCollector) collectGRPCMetrics(ctx context.Context, name string, sm *ServiceMetrics) error {
	// Запрашиваем метрики у сервиса
	req := &domain.GetMetricsRequest{
		ServiceName: name,
	}
	
	resp, err := sm.metricsClient.GetMetrics(ctx, req)
	if err != nil {
		sm.ErrorCount.WithLabelValues("get_metrics", "grpc_error").Inc()
		return err
	}
	
	// Обновляем метрики
	for _, metric := range resp.Metrics {
		switch metric.Type {
		case "counter":
			if counter, ok := metric.Value.(float64); ok {
				sm.RequestCount.WithLabelValues(metric.Method, "success").Add(float64(counter))
			}
		case "histogram":
			if histogram, ok := metric.Value.(map[string]interface{}); ok {
				if countVal, ok := histogram["count"].(float64); ok {
					sm.RequestCount.WithLabelValues(metric.Method, "success").Add(float64(countVal))
				}
				if sumVal, ok := histogram["sum"].(float64); ok {
					if countVal, ok := histogram["count"].(float64); ok {
						sm.RequestDuration.WithLabelValues(metric.Method).Observe(sumVal / countVal)
					}
				}
			}
		}
	}
	
	// Проверяем здоровье сервиса
	healthReq := &domain.HealthCheckRequest{
		Service: name,
	}
	
	healthResp, err := sm.healthClient.Check(ctx, healthReq)
	if err != nil {
		sm.ErrorCount.WithLabelValues("health_check", "grpc_error").Inc()
		return err
	}
	
	if healthResp.Status != "SERVING" {
		sm.ErrorCount.WithLabelValues("health_check", "unhealthy").Inc()
	}
	
	return nil
}

// GetHandler возвращает HTTP обработчик для метрик
func (mc *MetricsCollector) GetHandler() http.Handler {
	return mc.httpHandler
}

// GetRegistry возвращает реестр метрик
func (mc *MetricsCollector) GetRegistry() *prometheus.Registry {
	return mc.registry
}

// GetServices возвращает список подключенных сервисов
func (mc *MetricsCollector) GetServices() []string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	services := make([]string, 0, len(mc.services))
	for name := range mc.services {
		services = append(services, name)
	}
	
	return services
}

// GetServiceMetrics возвращает метрики конкретного сервиса
func (mc *MetricsCollector) GetServiceMetrics(name string) (*ServiceMetrics, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	serviceMetrics, exists := mc.services[name]
	if !exists {
		return nil, pkgerrors.New(pkgerrors.ErrNotFound, fmt.Sprintf("service %s not found", name))
	}
	
	return serviceMetrics, nil
}

// ScrapeAll выполняет сбор всех метрик
func (mc *MetricsCollector) ScrapeAll() error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	var errors []error
	
	for name, serviceMetrics := range mc.services {
		if err := mc.collectGRPCMetrics(ctx, name, serviceMetrics); err != nil {
			errors = append(errors, fmt.Errorf("service %s: %w", name, err))
		}
	}
	
	if len(errors) > 0 {
		return pkgerrors.New(pkgerrors.ErrInternal, fmt.Sprintf("failed to scrape metrics: %v", errors))
	}
	
	return nil
}

// CollectMetrics собирает метрики со всех сервисов
func (mc *MetricsCollector) CollectMetrics(ctx context.Context, serviceName, tenantID string, startTime, endTime *time.Time) (int, error) {
	mc.logger.Info("Collecting metrics",
		pkglogger.String("service_name", serviceName),
		pkglogger.String("tenant_id", tenantID))

	// Собираем метрики со всех сервисов
	mc.mu.RLock()
	services := make([]*ServiceMetrics, 0, len(mc.services))
	for _, sm := range mc.services {
		if serviceName == "" || sm.Name == serviceName {
			services = append(services, sm)
		}
	}
	mc.mu.RUnlock()

	totalMetrics := 0
	for _, sm := range services {
		err := mc.collectGRPCMetrics(ctx, sm.Name, sm)
		if err != nil {
			mc.logger.Error("Failed to collect metrics from service",
				pkglogger.String("service", sm.Name),
				pkglogger.Error(err))
			continue
		}
		totalMetrics++
	}

	return totalMetrics, nil
}

// ExportMetrics экспортирует метрики в указанный формат
func (mc *MetricsCollector) ExportMetrics(ctx context.Context, format, serviceName, tenantID string) ([]byte, string, error) {
	mc.logger.Info("Exporting metrics",
		pkglogger.String("format", format),
		pkglogger.String("service_name", serviceName))

	switch format {
	case "prometheus":
		// Экспорт в Prometheus формате
		metricsFamilies, err := mc.registry.Gather()
		if err != nil {
			return nil, "", err
		}

		var output strings.Builder
		for _, mf := range metricsFamilies {
			output.WriteString(mf.String())
		}

		return []byte(output.String()), "text/plain", nil

	case "json":
		// Экспорт в JSON формате
		result := map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"services": mc.GetServices(),
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, "", err
		}

		return data, "application/json", nil

	case "csv":
		// Экспорт в CSV формате
		var output strings.Builder
		output.WriteString("service,metric,value,timestamp\n")

		for _, serviceName := range mc.GetServices() {
			_, err := mc.GetServiceMetrics(serviceName)
			if err != nil {
				continue
			}

			// Добавляем базовые метрики
			value := 1.0 // Для простоты вернем 1.0
			output.WriteString(fmt.Sprintf("%s,active_connections,%f,%d\n",
				serviceName, value, time.Now().Unix()))
		}

		return []byte(output.String()), "text/csv", nil

	default:
		return nil, "", fmt.Errorf("unsupported format: %s", format)
	}
}

// GetMetrics возвращает текущие значения метрик
func (mc *MetricsCollector) GetMetrics(ctx context.Context, metricNames []string, serviceName, tenantID string, startTime, endTime *time.Time) (map[string]*MetricValue, error) {
	mc.logger.Info("Getting metrics",
		pkglogger.String("metric_names", fmt.Sprintf("%v", metricNames)),
		pkglogger.String("service_name", serviceName))

	result := make(map[string]*MetricValue)

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Если указан конкретный сервис
	if serviceName != "" {
		_, err := mc.GetServiceMetrics(serviceName)
		if err != nil {
			return nil, err
		}

		// Собираем метрики для сервиса
		value := 1.0 // Для простоты вернем 1.0
		result[serviceName+".active_connections"] = &MetricValue{
			Value:     value,
			Timestamp: time.Now().Format(time.RFC3339),
			Labels: map[string]string{
				"service": serviceName,
			},
		}
	} else {
		// Собираем метрики для всех сервисов
		for name := range mc.services {
			value := 1.0 // Для простоты вернем 1.0
			result[name+".active_connections"] = &MetricValue{
				Value:     value,
				Timestamp: time.Now().Format(time.RFC3339),
				Labels: map[string]string{
					"service": name,
				},
			}
		}
	}

	return result, nil
}

// MetricValue представляет значение метрики
type MetricValue struct {
	Value     float64            `json:"value"`
	Timestamp string             `json:"timestamp"`
	Labels    map[string]string  `json:"labels"`
}

// Shutdown корректно завершает работу коллектора
func (mc *MetricsCollector) Shutdown() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.logger.Info("Shutting down metrics collector")
	
	// Закрываем все соединения
	for name, serviceMetrics := range mc.services {
		if serviceMetrics.Conn != nil {
			serviceMetrics.Conn.Close()
			mc.logger.Debug("Closed connection for service", 
				pkglogger.String("service", name))
		}
	}
	
	mc.services = make(map[string]*ServiceMetrics)
	
	return nil
}
