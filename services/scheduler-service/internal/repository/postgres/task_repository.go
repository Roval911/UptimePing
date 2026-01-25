package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/services/scheduler-service/internal/domain"
	"UptimePingPlatform/services/scheduler-service/internal/repository"
)

// TaskRepository реализация TaskRepository в PostgreSQL
type TaskRepository struct {
	pool *pgxpool.Pool
}

// NewTaskRepository создает новый экземпляр TaskRepository
func NewTaskRepository(pool *pgxpool.Pool) repository.TaskRepository {
	return &TaskRepository{
		pool: pool,
	}
}

// CreateTask создает задачу
func (r *TaskRepository) CreateTask(ctx context.Context, task *domain.Task) error {
	query := `
		INSERT INTO tasks (id, check_id, tenant_id, scheduled_time, priority, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query,
		task.ID,
		task.CheckID,
		task.TenantID,
		task.ScheduledAt,
		task.Priority,
		domain.TaskStatusPending,
		task.CreatedAt,
	)

	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to create task").
			WithDetails(fmt.Sprintf("task_id: %s, check_id: %s", task.ID, task.CheckID)).
			WithContext(ctx)
	}

	return nil
}

// GetTaskByID получает задачу по ID
func (r *TaskRepository) GetTaskByID(ctx context.Context, taskID string) (*domain.Task, error) {
	query := `
		SELECT id, check_id, tenant_id, scheduled_time, priority, status, created_at
		FROM tasks
		WHERE id = $1
	`

	var task domain.Task
	var status string

	err := r.pool.QueryRow(ctx, query, taskID).Scan(
		&task.ID,
		&task.CheckID,
		&task.TenantID,
		&task.ScheduledAt,
		&task.Priority,
		&status,
		&task.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrNotFound, "task not found").
				WithDetails(fmt.Sprintf("task_id: %s", taskID)).
				WithContext(ctx)
		}
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get task").
			WithDetails(fmt.Sprintf("task_id: %s", taskID)).
			WithContext(ctx)
	}

	return &task, nil
}

// GetPendingTasks получает список ожидающих задач
func (r *TaskRepository) GetPendingTasks(ctx context.Context, limit int) ([]*domain.Task, error) {
	query := `
		SELECT id, check_id, tenant_id, scheduled_time, priority, status, created_at
		FROM tasks
		WHERE status = $1
		ORDER BY priority DESC, scheduled_time ASC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, domain.TaskStatusPending, limit)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get pending tasks").
			WithContext(ctx)
	}
	defer rows.Close()

	var tasks []*domain.Task
	for rows.Next() {
		var task domain.Task
		var status string

		err := rows.Scan(
			&task.ID,
			&task.CheckID,
			&task.TenantID,
			&task.ScheduledAt,
			&task.Priority,
			&status,
			&task.CreatedAt,
		)

		if err != nil {
			return nil, errors.Wrap(err, errors.ErrInternal, "failed to scan task").
				WithContext(ctx)
		}

		tasks = append(tasks, &task)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to iterate tasks").
			WithContext(ctx)
	}

	return tasks, nil
}

// UpdateTaskStatus обновляет статус задачи
func (r *TaskRepository) UpdateTaskStatus(ctx context.Context, taskID string, status domain.TaskStatus) error {
	query := `
		UPDATE tasks
		SET status = $1
		WHERE id = $2
	`

	result, err := r.pool.Exec(ctx, query, status, taskID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to update task status").
			WithDetails(fmt.Sprintf("task_id: %s, status: %s", taskID, status)).
			WithContext(ctx)
	}

	if result.RowsAffected() == 0 {
		return errors.New(errors.ErrNotFound, "task not found").
			WithDetails(fmt.Sprintf("task_id: %s", taskID)).
			WithContext(ctx)
	}

	return nil
}

// SaveTaskResult сохраняет результат выполнения задачи
func (r *TaskRepository) SaveTaskResult(ctx context.Context, result *domain.TaskResult) error {
	query := `
		INSERT INTO task_results (task_id, check_id, status, error_message, duration_ms, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.pool.Exec(ctx, query,
		result.TaskID,
		result.CheckID,
		result.Status,
		result.ErrorMessage,
		result.Duration,
		result.CompletedAt,
	)

	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to save task result").
			WithDetails(fmt.Sprintf("task_id: %s, check_id: %s", result.TaskID, result.CheckID)).
			WithContext(ctx)
	}

	return nil
}

// generateTaskID генерирует ID для задачи
func generateTaskID() string {
	return uuid.New().String()
}
