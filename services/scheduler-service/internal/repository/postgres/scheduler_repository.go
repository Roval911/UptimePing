package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/services/scheduler-service/internal/domain"
	"UptimePingPlatform/services/scheduler-service/internal/repository"
)

// SchedulerRepository реализация репозитория для планировщика
// Использует PostgreSQL для хранения и Redis для очереди задач
type SchedulerRepository struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

// NewSchedulerRepository создает новый экземпляр SchedulerRepository
func NewSchedulerRepository(pool *pgxpool.Pool, redisClient *redis.Client) repository.SchedulerRepository {
	return &SchedulerRepository{
		pool:  pool,
		redis: redisClient,
	}
}

// AddCheck добавляет проверку в планировщик
func (r *SchedulerRepository) AddCheck(ctx context.Context, check *domain.Check) error {
	// Сохраняем в Redis очередь для планировщика
	scheduledCheck := ScheduledCheck{
		ID:        check.ID,
		TenantID:  check.TenantID,
		Name:      check.Name,
		Target:    check.Target,
		Type:      check.Type,
		Interval:  check.Interval,
		Timeout:   check.Timeout,
		Priority:  check.Priority,
		Config:    check.Config,
		NextRunAt: check.NextRunAt,
	}

	// Сериализуем в JSON
	data, err := json.Marshal(scheduledCheck)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to marshal check for scheduler").
			WithDetails(fmt.Sprintf("check_id: %s", check.ID)).
			WithContext(ctx)
	}

	// Добавляем в Redis sorted set с временем выполнения как score
	var score float64
	if check.NextRunAt != nil {
		score = float64(check.NextRunAt.Unix())
	} else {
		score = float64(time.Now().Unix())
	}
	err = r.redis.ZAdd(ctx, "scheduler:checks", &redis.Z{
		Score:  score,
		Member: data,
	}).Err()

	if err != nil {
		var nextRunStr string
		if check.NextRunAt != nil {
			nextRunStr = check.NextRunAt.Format(time.RFC3339)
		} else {
			nextRunStr = "nil"
		}
		return errors.Wrap(err, errors.ErrInternal, "failed to add check to scheduler queue").
			WithDetails(fmt.Sprintf("check_id: %s, next_run: %s", check.ID, nextRunStr)).
			WithContext(ctx)
	}

	return nil
}

// RemoveCheck удаляет проверку из планировщика
func (r *SchedulerRepository) RemoveCheck(ctx context.Context, checkID string) error {
	// Получаем все проверки из очереди
	members, err := r.redis.ZRange(ctx, "scheduler:checks", 0, -1).Result()
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to get scheduler checks").
			WithContext(ctx)
	}

	// Ищем и удаляем нужную проверку
	for _, member := range members {
		var scheduledCheck ScheduledCheck
		if err := json.Unmarshal([]byte(member), &scheduledCheck); err != nil {
			continue // Пропускаем некорректные данные
		}

		if scheduledCheck.ID == checkID {
			// Удаляем из Redis
			err = r.redis.ZRem(ctx, "scheduler:checks", member).Err()
			if err != nil {
				return errors.Wrap(err, errors.ErrInternal, "failed to remove check from scheduler queue").
					WithDetails(fmt.Sprintf("check_id: %s", checkID)).
					WithContext(ctx)
			}
			return nil
		}
	}

	// Если проверка не найдена в очереди, это не ошибка - возможно она еще не была добавлена
	return nil
}

// UpdateCheck обновляет проверку в планировщике
func (r *SchedulerRepository) UpdateCheck(ctx context.Context, check *domain.Check) error {
	// Сначала удаляем старую версию
	if err := r.RemoveCheck(ctx, check.ID); err != nil {
		return err
	}

	// Добавляем обновленную версию
	return r.AddCheck(ctx, check)
}

// GetScheduledChecks возвращает список запланированных проверок
func (r *SchedulerRepository) GetScheduledChecks(ctx context.Context) ([]*domain.Check, error) {
	// Получаем проверки, которые нужно выполнить (с временем <= текущего)
	now := float64(time.Now().Unix())
	members, err := r.redis.ZRangeByScore(ctx, "scheduler:checks", &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%f", now),
	}).Result()

	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get scheduled checks").
			WithContext(ctx)
	}

	var checks []*domain.Check
	for _, member := range members {
		var scheduledCheck ScheduledCheck
		if err := json.Unmarshal([]byte(member), &scheduledCheck); err != nil {
			continue // Пропускаем некорректные данные
		}

		// Конвертируем в domain.Check
		check := &domain.Check{
			ID:        scheduledCheck.ID,
			TenantID:  scheduledCheck.TenantID,
			Name:      scheduledCheck.Name,
			Target:    scheduledCheck.Target,
			Type:      scheduledCheck.Type,
			Interval:  scheduledCheck.Interval,
			Timeout:   scheduledCheck.Timeout,
			Status:    domain.CheckStatusActive, // В планировщике только активные
			Priority:  scheduledCheck.Priority,
			Config:    scheduledCheck.Config,
			NextRunAt: scheduledCheck.NextRunAt,
		}

		checks = append(checks, check)
	}

	return checks, nil
}

// ScheduledCheck представляет проверку в очереди планировщика
type ScheduledCheck struct {
	ID        string                 `json:"id"`
	TenantID  string                 `json:"tenant_id"`
	Name      string                 `json:"name"`
	Target    string                 `json:"target"`
	Type      domain.CheckType       `json:"type"`
	Interval  int                    `json:"interval"`
	Timeout   int                    `json:"timeout"`
	Priority  domain.Priority        `json:"priority"`
	Config    map[string]interface{} `json:"config"`
	NextRunAt *time.Time             `json:"next_run"`
}

// Create создает новое расписание
func (r *SchedulerRepository) Create(ctx context.Context, schedule *domain.Schedule) (*domain.Schedule, error) {
	query := `
		INSERT INTO schedules (check_id, cron_expression, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, next_run, last_run`

	now := time.Now()
	err := r.pool.QueryRow(ctx, query,
		schedule.CheckID,
		schedule.CronExpression,
		schedule.IsActive,
		now,
		now,
	).Scan(&schedule.ID, &schedule.NextRun, &schedule.LastRun)

	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	schedule.CreatedAt = now
	schedule.UpdatedAt = now
	return schedule, nil
}

// DeleteByCheckID удаляет расписание по ID проверки
func (r *SchedulerRepository) DeleteByCheckID(ctx context.Context, checkID string) error {
	query := `DELETE FROM schedules WHERE check_id = $1`

	_, err := r.pool.Exec(ctx, query, checkID)
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	return nil
}

// GetByCheckID получает расписание по ID проверки
func (r *SchedulerRepository) GetByCheckID(ctx context.Context, checkID string) (*domain.Schedule, error) {
	query := `
		SELECT id, check_id, cron_expression, next_run, last_run, is_active, created_at, updated_at
		FROM schedules
		WHERE check_id = $1`

	schedule := &domain.Schedule{}
	err := r.pool.QueryRow(ctx, query, checkID).Scan(
		&schedule.ID,
		&schedule.CheckID,
		&schedule.CronExpression,
		&schedule.NextRun,
		&schedule.LastRun,
		&schedule.IsActive,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("schedule not found: %w", err)
	}

	return schedule, nil
}

// List получает список расписаний с пагинацией
func (r *SchedulerRepository) List(ctx context.Context, pageSize int, pageToken string, filter string) ([]*domain.Schedule, error) {
	query := `
		SELECT id, check_id, cron_expression, next_run, last_run, is_active, created_at, updated_at
		FROM schedules
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	offset := 0
	if pageToken != "" {
		fmt.Sscanf(pageToken, "%d", &offset)
	}

	rows, err := r.pool.Query(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedules: %w", err)
	}
	defer rows.Close()

	var schedules []*domain.Schedule
	for rows.Next() {
		schedule := &domain.Schedule{}
		err := rows.Scan(
			&schedule.ID,
			&schedule.CheckID,
			&schedule.CronExpression,
			&schedule.NextRun,
			&schedule.LastRun,
			&schedule.IsActive,
			&schedule.CreatedAt,
			&schedule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// Count возвращает общее количество расписаний
func (r *SchedulerRepository) Count(ctx context.Context, filter string) (int, error) {
	query := `SELECT COUNT(*) FROM schedules`

	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count schedules: %w", err)
	}

	return count, nil
}

// Ping проверяет подключение к базе данных
func (r *SchedulerRepository) Ping(ctx context.Context) (interface{}, error) {
	err := r.pool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return "pong", nil
}
