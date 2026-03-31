package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/metrics-service/internal/domain"
	"github.com/lib/pq"
)

// MetricsRepository PostgreSQL репозиторий для работы с метриками
type MetricsRepository struct {
	db     *sql.DB
	logger logger.Logger
}

// NewMetricsRepository создает новый PostgreSQL репозиторий метрик
func NewMetricsRepository(db *sql.DB, logger logger.Logger) *MetricsRepository {
	return &MetricsRepository{
		db:     db,
		logger: logger,
	}
}

// SaveMetrics сохраняет агрегированные метрики
func (r *MetricsRepository) SaveMetrics(ctx context.Context, metrics *domain.AggregatedMetrics) error {
	query := `
		INSERT INTO metrics (id, tenant_id, check_id, metric_type, value, timestamp, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			value = EXCLUDED.value,
			timestamp = EXCLUDED.timestamp,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
	`

	metadataJSON, _ := json.Marshal(metrics.Metadata)
	_, err := r.db.ExecContext(ctx, query,
		metrics.ID,
		metrics.TenantID,
		metrics.CheckID,
		metrics.MetricType,
		metrics.Value,
		metrics.Timestamp,
		metadataJSON,
	)

	if err != nil {
		r.logger.Error("Failed to save metrics",
			logger.String("metrics_id", metrics.ID),
			logger.String("tenant_id", metrics.TenantID),
			logger.String("check_id", metrics.CheckID),
			logger.Error(err),
		)
		return err
	}

	r.logger.Info("Metrics saved successfully",
		logger.String("metrics_id", metrics.ID),
		logger.String("tenant_id", metrics.TenantID),
		logger.String("check_id", metrics.CheckID),
	)

	return nil
}

// GetMetricsByTimeRange получает метрики за период времени
func (r *MetricsRepository) GetMetricsByTimeRange(ctx context.Context, tenantID, checkID string, startTime, endTime time.Time) ([]*domain.AggregatedMetrics, error) {
	query := `
		SELECT id, tenant_id, check_id, metric_type, value, timestamp, metadata
		FROM metrics
		WHERE tenant_id = $1 AND check_id = $2 AND timestamp BETWEEN $3 AND $4
		ORDER BY timestamp DESC
		LIMIT 1000
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID, checkID, startTime, endTime)
	if err != nil {
		r.logger.Error("Failed to get metrics by time range",
			logger.String("tenant_id", tenantID),
			logger.String("check_id", checkID),
			logger.Error(err),
		)
		return nil, err
	}
	defer rows.Close()

	var metrics []*domain.AggregatedMetrics
	for rows.Next() {
		var metric domain.AggregatedMetrics
		var metadataJSON []byte
		err := rows.Scan(
			&metric.ID,
			&metric.TenantID,
			&metric.CheckID,
			&metric.MetricType,
			&metric.Value,
			&metric.Timestamp,
			&metadataJSON,
		)
		if err != nil {
			r.logger.Error("Failed to scan metrics row",
				logger.Error(err),
			)
			return nil, err
		}

		// Десериализация JSON метаданных
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &metric.Metadata); err != nil {
				r.logger.Error("Failed to unmarshal metrics metadata",
					logger.String("metrics_id", metric.ID),
					logger.Error(err),
				)
			}
		}

		metrics = append(metrics, &metric)
	}

	return metrics, nil
}

// GetMetricsByType получает метрики определенного типа
func (r *MetricsRepository) GetMetricsByType(ctx context.Context, tenantID, metricType string, limit int) ([]*domain.AggregatedMetrics, error) {
	query := `
		SELECT id, tenant_id, check_id, metric_type, value, timestamp, metadata
		FROM metrics
		WHERE tenant_id = $1 AND metric_type = $2
		ORDER BY timestamp DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID, metricType, limit)
	if err != nil {
		r.logger.Error("Failed to get metrics by type",
			logger.String("tenant_id", tenantID),
			logger.String("metric_type", metricType),
			logger.Error(err),
		)
		return nil, err
	}
	defer rows.Close()

	var metrics []*domain.AggregatedMetrics
	for rows.Next() {
		var metric domain.AggregatedMetrics
		var metadataJSON []byte
		err := rows.Scan(
			&metric.ID,
			&metric.TenantID,
			&metric.CheckID,
			&metric.MetricType,
			&metric.Value,
			&metric.Timestamp,
			&metadataJSON,
		)
		if err != nil {
			r.logger.Error("Failed to scan metrics row",
				logger.Error(err),
			)
			return nil, err
		}

		// Десериализация JSON метаданных
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &metric.Metadata); err != nil {
				r.logger.Error("Failed to unmarshal metrics metadata",
					logger.String("metrics_id", metric.ID),
					logger.Error(err),
				)
			}
		}

		metrics = append(metrics, &metric)
	}

	return metrics, nil
}

// DeleteOldMetrics удаляет старые метрики (для очистки)
func (r *MetricsRepository) DeleteOldMetrics(ctx context.Context, olderThan time.Time) error {
	query := `DELETE FROM metrics WHERE timestamp < $1`

	result, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		r.logger.Error("Failed to delete old metrics",
			logger.Time("older_than", olderThan),
			logger.Error(err),
		)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	r.logger.Info("Old metrics deleted successfully",
		logger.Int64("rows_affected", rowsAffected),
		logger.Time("older_than", olderThan),
	)

	return nil
}
