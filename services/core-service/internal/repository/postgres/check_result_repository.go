package postgres

import (
	"context"
	"database/sql"
	"time"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/services/core-service/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CheckResultRepository реализация репозитория для PostgreSQL
type CheckResultRepository struct {
	pool   *pgxpool.Pool
	logger logger.Logger
}

// NewCheckResultRepository создает новый репозиторий
func NewCheckResultRepository(pool *pgxpool.Pool, logger logger.Logger) repository.CheckResultRepository {
	return &CheckResultRepository{
		pool:   pool,
		logger: logger,
	}
}

// Save сохраняет результат проверки в БД
func (r *CheckResultRepository) Save(ctx context.Context, result *domain.CheckResult) error {
	r.logger.Debug("Saving check result to database",
		logger.String("check_id", result.CheckID),
		logger.String("execution_id", result.ExecutionID),
	)

	query := `
		INSERT INTO check_results (
			id, check_id, status, response_time, response_code, 
			response_body, error_message, location, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			response_time = EXCLUDED.response_time,
			response_code = EXCLUDED.response_code,
			response_body = EXCLUDED.response_body,
			error_message = EXCLUDED.error_message,
			location = EXCLUDED.location,
			created_at = EXCLUDED.created_at
	`

	// Конвертация статуса
	status := "unknown"
	if result.Success {
		status = "up"
	} else {
		status = "down"
	}

	_, err := r.pool.Exec(ctx, query,
		result.CheckID, // Используем CheckID как ID для простоты
		result.CheckID,
		status,
		float64(result.DurationMs)/1000.0, // Конвертация в секунды
		result.StatusCode,
		result.ResponseBody,
		result.Error,
		result.CheckID, // location = check_id
		result.CheckedAt,
	)

	if err != nil {
		r.logger.Error("Failed to save check result",
			logger.String("check_id", result.CheckID),
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to save check result")
	}

	r.logger.Debug("Check result saved successfully",
		logger.String("check_id", result.CheckID),
	)

	return nil
}

// GetByID получает результат по ID
func (r *CheckResultRepository) GetByID(ctx context.Context, id string) (*domain.CheckResult, error) {
	r.logger.Debug("Getting check result by ID",
		logger.String("id", id),
	)

	query := `
		SELECT id, check_id, status, response_time, response_code, 
			   response_body, error_message, location, created_at
		FROM check_results 
		WHERE id = $1
	`

	var (
		checkID       string
		status        string
		responseTime  float64
		responseCode  sql.NullInt32
		responseBody  sql.NullString
		errorMessage  sql.NullString
		location      string
		createdAt     time.Time
	)

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&id,
		&checkID,
		&status,
		&responseTime,
		&responseCode,
		&responseBody,
		&errorMessage,
		&location,
		&createdAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.ErrNotFound, "check result not found")
		}
		r.logger.Error("Failed to get check result by ID",
			logger.String("id", id),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get check result")
	}

	// Конвертация в доменную модель
	result := &domain.CheckResult{
		CheckID:     checkID,
		DurationMs:  int64(responseTime * 1000), // Конвертация в миллисекунды
		CheckedAt:   createdAt,
		Success:     status == "up",
		Metadata:    make(map[string]string),
	}

	if responseCode.Valid {
		result.StatusCode = int(responseCode.Int32)
	}

	if responseBody.Valid {
		result.ResponseBody = responseBody.String
	}

	if errorMessage.Valid {
		result.Error = errorMessage.String
	}

	return result, nil
}

// GetByCheckID получает результаты для конкретной проверки
func (r *CheckResultRepository) GetByCheckID(ctx context.Context, checkID string, limit int) ([]*domain.CheckResult, error) {
	r.logger.Debug("Getting check results by check ID",
		logger.String("check_id", checkID),
		logger.Int("limit", limit),
	)

	query := `
		SELECT id, check_id, status, response_time, response_code, 
			   response_body, error_message, location, created_at
		FROM check_results 
		WHERE check_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, checkID, limit)
	if err != nil {
		r.logger.Error("Failed to query check results",
			logger.String("check_id", checkID),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to query check results")
	}
	defer rows.Close()

	var results []*domain.CheckResult
	for rows.Next() {
		var (
			id            string
			checkID       string
			status        string
			responseTime  float64
			responseCode  sql.NullInt32
			responseBody  sql.NullString
			errorMessage  sql.NullString
			location      string
			createdAt     time.Time
		)

		if err := rows.Scan(
			&id,
			&checkID,
			&status,
			&responseTime,
			&responseCode,
			&responseBody,
			&errorMessage,
			&location,
			&createdAt,
		); err != nil {
			r.logger.Error("Failed to scan check result row",
				logger.Error(err),
			)
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan check result")
		}

		result := &domain.CheckResult{
			CheckID:     checkID,
			DurationMs:  int64(responseTime * 1000),
			CheckedAt:   createdAt,
			Success:     status == "up",
			Metadata:    make(map[string]string),
		}

		if responseCode.Valid {
			result.StatusCode = int(responseCode.Int32)
		}

		if responseBody.Valid {
			result.ResponseBody = responseBody.String
		}

		if errorMessage.Valid {
			result.Error = errorMessage.String
		}

		results = append(results, result)
	}

	return results, nil
}

// GetLatestByCheckID получает последний результат для проверки
func (r *CheckResultRepository) GetLatestByCheckID(ctx context.Context, checkID string) (*domain.CheckResult, error) {
	results, err := r.GetByCheckID(ctx, checkID, 1)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, errors.New(errors.ErrNotFound, "no results found for check")
	}

	return results[0], nil
}

// GetByTimeRange получает результаты за период времени
func (r *CheckResultRepository) GetByTimeRange(ctx context.Context, startTime, endTime time.Time, limit int) ([]*domain.CheckResult, error) {
	r.logger.Debug("Getting check results by time range",
			logger.String("start_time", startTime.String()),
			logger.String("end_time", endTime.String()),
			logger.Int("limit", limit),
		)

	query := `
		SELECT id, check_id, status, response_time, response_code, 
			   response_body, error_message, location, created_at
		FROM check_results 
		WHERE created_at BETWEEN $1 AND $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, startTime, endTime, limit)
	if err != nil {
		r.logger.Error("Failed to query check results by time range",
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to query check results")
	}
	defer rows.Close()

	var results []*domain.CheckResult
	for rows.Next() {
		var (
			id            string
			checkID       string
			status        string
			responseTime  float64
			responseCode  sql.NullInt32
			responseBody  sql.NullString
			errorMessage  sql.NullString
			location      string
			createdAt     time.Time
		)

		if err := rows.Scan(
			&id,
			&checkID,
			&status,
			&responseTime,
			&responseCode,
			&responseBody,
			&errorMessage,
			&location,
			&createdAt,
		); err != nil {
			r.logger.Error("Failed to scan check result row",
				logger.Error(err),
			)
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan check result")
		}

		result := &domain.CheckResult{
			CheckID:     checkID,
			DurationMs:  int64(responseTime * 1000),
			CheckedAt:   createdAt,
			Success:     status == "up",
			Metadata:    make(map[string]string),
		}

		if responseCode.Valid {
			result.StatusCode = int(responseCode.Int32)
		}

		if responseBody.Valid {
			result.ResponseBody = responseBody.String
		}

		if errorMessage.Valid {
			result.Error = errorMessage.String
		}

		results = append(results, result)
	}

	return results, nil
}

// GetFailedChecks получает все неудачные проверки за период
func (r *CheckResultRepository) GetFailedChecks(ctx context.Context, startTime, endTime time.Time, limit int) ([]*domain.CheckResult, error) {
	r.logger.Debug("Getting failed check results",
			logger.String("start_time", startTime.String()),
			logger.String("end_time", endTime.String()),
			logger.Int("limit", limit),
		)

	query := `
		SELECT id, check_id, status, response_time, response_code, 
			   response_body, error_message, location, created_at
		FROM check_results 
		WHERE created_at BETWEEN $1 AND $2 AND status = 'down'
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, startTime, endTime, limit)
	if err != nil {
		r.logger.Error("Failed to query failed check results",
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to query failed check results")
	}
	defer rows.Close()

	var results []*domain.CheckResult
	for rows.Next() {
		var (
			id            string
			checkID       string
			status        string
			responseTime  float64
			responseCode  sql.NullInt32
			responseBody  sql.NullString
			errorMessage  sql.NullString
			location      string
			createdAt     time.Time
		)

		if err := rows.Scan(
			&id,
			&checkID,
			&status,
			&responseTime,
			&responseCode,
			&responseBody,
			&errorMessage,
			&location,
			&createdAt,
		); err != nil {
			r.logger.Error("Failed to scan check result row",
				logger.Error(err),
			)
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan check result")
		}

		result := &domain.CheckResult{
			CheckID:     checkID,
			DurationMs:  int64(responseTime * 1000),
			CheckedAt:   createdAt,
			Success:     false, // Всегда false для failed checks
			Metadata:    make(map[string]string),
		}

		if responseCode.Valid {
			result.StatusCode = int(responseCode.Int32)
		}

		if responseBody.Valid {
			result.ResponseBody = responseBody.String
		}

		if errorMessage.Valid {
			result.Error = errorMessage.String
		}

		results = append(results, result)
	}

	return results, nil
}

// DeleteOldResults удаляет старые результаты
func (r *CheckResultRepository) DeleteOldResults(ctx context.Context, olderThan time.Time) error {
	r.logger.Debug("Deleting old check results",
		logger.String("older_than", olderThan.String()),
	)

	query := `DELETE FROM check_results WHERE created_at < $1`

	cmdTag, err := r.pool.Exec(ctx, query, olderThan)
	if err != nil {
		r.logger.Error("Failed to delete old check results",
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to delete old check results")
	}

	r.logger.Info("Old check results deleted",
		logger.Int64("deleted_count", cmdTag.RowsAffected()),
		logger.String("older_than", olderThan.String()),
	)

	return nil
}

// GetStats получает статистику по результатам
func (r *CheckResultRepository) GetStats(ctx context.Context, startTime, endTime time.Time) (*repository.ResultStats, error) {
	r.logger.Debug("Getting check result statistics",
		logger.String("start_time", startTime.String()),
		logger.String("end_time", endTime.String()),
	)

	query := `
		SELECT 
			COUNT(*) as total_checks,
			COUNT(CASE WHEN status = 'up' THEN 1 END) as successful_checks,
			COUNT(CASE WHEN status = 'down' THEN 1 END) as failed_checks,
			COUNT(CASE WHEN status = 'unknown' THEN 1 END) as unknown_checks,
			AVG(response_time) as avg_response_time
		FROM check_results 
		WHERE created_at BETWEEN $1 AND $2
	`

	var (
		totalChecks       int64
		successfulChecks  int64
		failedChecks      int64
		unknownChecks     int64
		avgResponseTime   sql.NullFloat64
	)

	err := r.pool.QueryRow(ctx, query, startTime, endTime).Scan(
		&totalChecks,
		&successfulChecks,
		&failedChecks,
		&unknownChecks,
		&avgResponseTime,
	)

	if err != nil {
		r.logger.Error("Failed to get check result statistics",
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get check result statistics")
	}

	stats := &repository.ResultStats{
		TotalChecks:       totalChecks,
		SuccessfulChecks:  successfulChecks,
		FailedChecks:      failedChecks,
		UnknownChecks:     unknownChecks,
	}

	if avgResponseTime.Valid {
		stats.AvgResponseTime = avgResponseTime.Float64
	}

	// Расчет uptime процента
	if totalChecks > 0 {
		stats.UptimePercent = float64(successfulChecks) / float64(totalChecks) * 100
	}

	return stats, nil
}
