package repository

import (
	"context"
	"time"

	"UptimePingPlatform/services/core-service/internal/domain"
)

// CheckResultRepository определяет интерфейс для работы с результатами проверок
type CheckResultRepository interface {
	// Save сохраняет результат проверки в БД
	Save(ctx context.Context, result *domain.CheckResult) error
	
	// GetByID получает результат по ID
	GetByID(ctx context.Context, id string) (*domain.CheckResult, error)
	
	// GetByCheckID получает результаты для конкретной проверки
	GetByCheckID(ctx context.Context, checkID string, limit int) ([]*domain.CheckResult, error)
	
	// GetLatestByCheckID получает последний результат для проверки
	GetLatestByCheckID(ctx context.Context, checkID string) (*domain.CheckResult, error)
	
	// GetByTimeRange получает результаты за период времени
	GetByTimeRange(ctx context.Context, startTime, endTime time.Time, limit int) ([]*domain.CheckResult, error)
	
	// GetFailedChecks получает все неудачные проверки за период
	GetFailedChecks(ctx context.Context, startTime, endTime time.Time, limit int) ([]*domain.CheckResult, error)
	
	// DeleteOldResults удаляет старые результаты
	DeleteOldResults(ctx context.Context, olderThan time.Time) error
	
	// GetStats получает статистику по результатам
	GetStats(ctx context.Context, startTime, endTime time.Time) (*ResultStats, error)
}

// ResultStats статистика по результатам проверок
type ResultStats struct {
	TotalChecks   int64 `json:"total_checks"`
	SuccessfulChecks int64 `json:"successful_checks"`
	FailedChecks  int64 `json:"failed_checks"`
	UnknownChecks int64 `json:"unknown_checks"`
	AvgResponseTime float64 `json:"avg_response_time"`
	UptimePercent   float64 `json:"uptime_percent"`
}
