package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/metrics-service/internal/collector"

	metricsv1 "UptimePingPlatform/gen/proto/api/metrics/v1"
)

// MetricsHandler реализует gRPC обработчики для MetricsService
type MetricsHandler struct {
	*grpcBase.BaseHandler
	metricsv1.UnimplementedMetricsServiceServer
	collector *collector.MetricsCollector
	validator  *validation.Validator
}

// NewMetricsHandler создает новый экземпляр MetricsHandler
func NewMetricsHandler(collector *collector.MetricsCollector, logger logger.Logger) *MetricsHandler {
	return &MetricsHandler{
		BaseHandler: grpcBase.NewBaseHandler(logger),
		collector:   collector,
		validator:   validation.NewValidator(),
	}
}

// CollectMetrics собирает метрики со всех сервисов
func (h *MetricsHandler) CollectMetrics(ctx context.Context, req *metricsv1.CollectMetricsRequest) (*metricsv1.CollectMetricsResponse, error) {
	h.LogOperationStart(ctx, "CollectMetrics", map[string]interface{}{
		"service_name": req.ServiceName,
		"tenant_id":    req.TenantId,
		"start_time":   req.StartTime,
		"end_time":     req.EndTime,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "CollectMetrics", map[string]string{
		"service_name": req.ServiceName,
		"tenant_id":    req.TenantId,
	}); err != nil {
		return nil, err
	}

	// Валидация service_name
	if err := h.validator.ValidateStringLength(req.ServiceName, "service_name", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "CollectMetrics", req.ServiceName)
	}

	// Валидация tenant_id
	if err := h.validator.ValidateStringLength(req.TenantId, "tenant_id", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "CollectMetrics", req.ServiceName)
	}

	// Валидация времени если указано
	var startTime, endTime *time.Time
	if req.StartTime != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return nil, h.LogError(ctx, err, "CollectMetrics", req.ServiceName)
		}
		startTime = &t
	}

	if req.EndTime != "" {
		t, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return nil, h.LogError(ctx, err, "CollectMetrics", req.ServiceName)
		}
		endTime = &t
	}

	// Собираем метрики
	metricsCount, err := h.collector.CollectMetrics(ctx, req.ServiceName, req.TenantId, startTime, endTime)
	if err != nil {
		h.LogError(ctx, err, "CollectMetrics", req.ServiceName)
		return nil, status.Errorf(codes.Internal, "failed to collect metrics: %v", err)
	}

	response := &metricsv1.CollectMetricsResponse{
		Success:      true,
		MetricsCount: int32(metricsCount),
		CollectedAt:  time.Now().Format(time.RFC3339),
	}

	h.LogOperationSuccess(ctx, "CollectMetrics", map[string]interface{}{
		"service_name":  req.ServiceName,
		"metrics_count": metricsCount,
	})

	return response, nil
}

// ExportMetrics экспортирует метрики в указанный формат
func (h *MetricsHandler) ExportMetrics(req *metricsv1.ExportMetricsRequest, stream metricsv1.MetricsService_ExportMetricsServer) error {
	ctx := stream.Context()
	
	h.LogOperationStart(ctx, "ExportMetrics", map[string]interface{}{
		"format":       req.Format,
		"service_name": req.ServiceName,
		"tenant_id":    req.TenantId,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "ExportMetrics", map[string]string{
		"format":       req.Format,
		"service_name": req.ServiceName,
		"tenant_id":    req.TenantId,
	}); err != nil {
		return err
	}

	// Валидация формата
	if err := h.validator.ValidateEnum(req.Format, []string{"prometheus", "json", "csv"}, "format"); err != nil {
		return h.LogError(ctx, err, "ExportMetrics", req.ServiceName)
	}

	// Валидация service_name
	if err := h.validator.ValidateStringLength(req.ServiceName, "service_name", 1, 100); err != nil {
		return h.LogError(ctx, err, "ExportMetrics", req.ServiceName)
	}

	// Валидация tenant_id
	if err := h.validator.ValidateStringLength(req.TenantId, "tenant_id", 1, 100); err != nil {
		return h.LogError(ctx, err, "ExportMetrics", req.ServiceName)
	}

	// Получаем данные для экспорта
	data, contentType, err := h.collector.ExportMetrics(ctx, req.Format, req.ServiceName, req.TenantId)
	if err != nil {
		h.LogError(ctx, err, "ExportMetrics", req.ServiceName)
		return status.Errorf(codes.Internal, "failed to export metrics: %v", err)
	}

	// Разбиваем данные на чанки для потоковой передачи
	chunkSize := 1024 * 1024 // 1MB chunks
	chunkNumber := int32(0)

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]
		chunkNumber++

		response := &metricsv1.ExportMetricsResponse{
			Data:         chunk,
			ContentType:  contentType,
			ChunkNumber:  chunkNumber,
		}

		if err := stream.Send(response); err != nil {
			h.LogError(ctx, err, "ExportMetrics", req.ServiceName)
			return status.Errorf(codes.Internal, "failed to send chunk: %v", err)
		}
	}

	h.LogOperationSuccess(ctx, "ExportMetrics", map[string]interface{}{
		"service_name":  req.ServiceName,
		"format":        req.Format,
		"total_chunks":  chunkNumber,
		"data_size":     len(data),
	})

	return nil
}

// GetMetrics возвращает текущие значения метрик
func (h *MetricsHandler) GetMetrics(ctx context.Context, req *metricsv1.GetMetricsRequest) (*metricsv1.GetMetricsResponse, error) {
	h.LogOperationStart(ctx, "GetMetrics", map[string]interface{}{
		"metric_names": req.MetricNames,
		"service_name": req.ServiceName,
		"tenant_id":    req.TenantId,
		"start_time":   req.StartTime,
		"end_time":     req.EndTime,
	})

	// Валидация service_name и tenant_id
	if req.ServiceName != "" {
		if err := h.validator.ValidateStringLength(req.ServiceName, "service_name", 1, 100); err != nil {
			return nil, h.LogError(ctx, err, "GetMetrics", req.ServiceName)
		}
	}

	if req.TenantId != "" {
		if err := h.validator.ValidateStringLength(req.TenantId, "tenant_id", 1, 100); err != nil {
			return nil, h.LogError(ctx, err, "GetMetrics", req.ServiceName)
		}
	}

	// Валидация времени если указано
	var startTime, endTime *time.Time
	if req.StartTime != "" {
		t, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return nil, h.LogError(ctx, err, "GetMetrics", req.ServiceName)
		}
		startTime = &t
	}

	if req.EndTime != "" {
		t, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return nil, h.LogError(ctx, err, "GetMetrics", req.ServiceName)
		}
		endTime = &t
	}

	// Получаем метрики
	metrics, err := h.collector.GetMetrics(ctx, req.MetricNames, req.ServiceName, req.TenantId, startTime, endTime)
	if err != nil {
		h.LogError(ctx, err, "GetMetrics", req.ServiceName)
		return nil, status.Errorf(codes.Internal, "failed to get metrics: %v", err)
	}

	// Конвертируем в protobuf формат
	protoMetrics := make(map[string]*metricsv1.MetricValue)
	for name, value := range metrics {
		protoMetrics[name] = &metricsv1.MetricValue{
			Value:     value.Value,
			Timestamp: value.Timestamp,
			Labels:    value.Labels,
		}
	}

	response := &metricsv1.GetMetricsResponse{
		Metrics: protoMetrics,
	}

	h.LogOperationSuccess(ctx, "GetMetrics", map[string]interface{}{
		"service_name": req.ServiceName,
		"metrics_count": len(protoMetrics),
	})

	return response, nil
}
