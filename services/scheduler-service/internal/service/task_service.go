package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/rabbitmq"
	"UptimePingPlatform/services/scheduler-service/internal/domain"
	"UptimePingPlatform/services/scheduler-service/internal/repository"
)

// TaskService предоставляет бизнес-логику для управления задачами
type TaskService struct {
	checkRepo     repository.CheckRepository
	taskRepo      repository.TaskRepository
	lockRepo      repository.LockRepository
	schedulerRepo repository.SchedulerRepository
	rabbitMQ      *rabbitmq.Producer
	cronScheduler *cron.Cron
	logger        logger.Logger
	workerID      string
}

// NewTaskService создает новый экземпляр TaskService
func NewTaskService(
	checkRepo repository.CheckRepository,
	taskRepo repository.TaskRepository,
	lockRepo repository.LockRepository,
	schedulerRepo repository.SchedulerRepository,
	rabbitMQ *rabbitmq.Producer,
	logger logger.Logger,
) *TaskService {
	return &TaskService{
		checkRepo:     checkRepo,
		taskRepo:      taskRepo,
		lockRepo:      lockRepo,
		schedulerRepo: schedulerRepo,
		rabbitMQ:      rabbitMQ,
		cronScheduler: cron.New(cron.WithSeconds()), // Поддержка секунд
		logger:        logger,
		workerID:      fmt.Sprintf("worker-%s", uuid.New().String()[:8]),
	}
}

// ExecuteCronTask выполняет cron задачу для проверки
func (s *TaskService) ExecuteCronTask(ctx context.Context, checkID string) error {
	s.logger.Debug("Starting cron task execution",
		logger.String("check_id", checkID),
		logger.String("worker_id", s.workerID),
	)

	// 1. Получение распределенной блокировки (Redis) по check_id
	lockTTL := 5 * time.Minute // Блокировка на 5 минут
	lockInfo, err := s.lockRepo.TryLock(ctx, checkID, s.workerID, lockTTL)
	if err != nil {
		if customErr, ok := err.(*errors.Error); ok && customErr.Code == errors.ErrConflict {
			// Блокировка уже получена другим worker
			s.logger.Debug("Task already locked by another worker",
				logger.String("check_id", checkID),
				logger.String("worker_id", s.workerID),
			)
			return nil
		}
		return errors.Wrap(err, errors.ErrInternal, "failed to acquire lock").
			WithDetails(fmt.Sprintf("check_id: %s, worker_id: %s", checkID, s.workerID)).
			WithContext(ctx)
	}

	defer func() {
		// 7. Освобождение блокировки
		if releaseErr := s.lockRepo.ReleaseLock(ctx, checkID, s.workerID); releaseErr != nil {
			s.logger.Error("Failed to release lock",
				logger.String("check_id", checkID),
				logger.String("worker_id", s.workerID),
				logger.String("error", releaseErr.Error()),
			)
		}
	}()

	s.logger.Debug("Lock acquired successfully",
		logger.String("check_id", checkID),
		logger.String("worker_id", s.workerID),
		logger.String("locked_at", lockInfo.LockedAt.Format(time.RFC3339)),
		logger.String("expires_at", lockInfo.ExpiresAt.Format(time.RFC3339)),
	)

	// 2. Если блокировка получена: Получение конфигурации проверки из БД
	check, err := s.checkRepo.GetByID(ctx, checkID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to get check configuration").
			WithDetails(fmt.Sprintf("check_id: %s", checkID)).
			WithContext(ctx)
	}

	// Проверяем, что проверка все еще активна
	if check.Status != domain.CheckStatusActive {
		s.logger.Info("Check is no longer active, skipping execution",
			logger.String("check_id", checkID),
			logger.String("status", string(check.Status)),
		)
		return nil
	}

	now := time.Now()

	// 3. Создание задачи (check_id, tenant_id, scheduled_time, priority)
	task := domain.NewTaskForExecution(checkID, check.TenantID, now, check.Priority)
	task.ID = s.generateTaskID()

	// 4. Отправка задачи в RabbitMQ очередь check_tasks
	if err := s.sendTaskToRabbitMQ(ctx, task); err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to send task to RabbitMQ").
			WithDetails(fmt.Sprintf("task_id: %s, check_id: %s", task.ID, checkID)).
			WithContext(ctx)
	}

	// 5. Обновление last_run и next_run в БД
	if err := s.updateCheckRunTimes(ctx, check, now); err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to update check run times").
			WithDetails(fmt.Sprintf("check_id: %s", checkID)).
			WithContext(ctx)
	}

	// Сохраняем задачу в БД для отслеживания
	if err := s.taskRepo.CreateTask(ctx, task); err != nil {
		s.logger.Error("Failed to save task to database",
			logger.String("task_id", task.ID),
			logger.String("check_id", checkID),
			logger.String("error", err.Error()),
		)
		// Не прерываем выполнение, так как задача уже отправлена в RabbitMQ
	}

	s.logger.Info("Cron task executed successfully",
		logger.String("check_id", checkID),
		logger.String("task_id", task.ID),
		logger.String("worker_id", s.workerID),
		logger.String("executed_at", now.Format(time.RFC3339)),
	)

	return nil
}

// sendTaskToRabbitMQ отправляет задачу в RabbitMQ
func (s *TaskService) sendTaskToRabbitMQ(ctx context.Context, task *domain.Task) error {
	// Если RabbitMQ не настроен (например, в тестах), просто логируем
	if s.rabbitMQ == nil {
		s.logger.Info("RabbitMQ not configured, skipping task send",
			logger.String("task_id", task.ID),
			logger.String("check_id", task.CheckID),
			logger.String("tenant_id", task.TenantID),
			logger.String("queue", "check_tasks"),
			logger.String("scheduled_time", task.ScheduledAt.Format(time.RFC3339)),
			logger.Int("priority", int(task.Priority)),
		)
		return nil
	}

	// Сериализуем задачу в JSON
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to serialize task to JSON").
			WithDetails(fmt.Sprintf("task_id: %s", task.ID)).
			WithContext(ctx)
	}

	// Отправляем задачу в RabbitMQ очередь
	if err := s.rabbitMQ.Publish(ctx, taskJSON, rabbitmq.WithRoutingKey("check_tasks")); err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to publish task to RabbitMQ").
			WithDetails(fmt.Sprintf("task_id: %s, queue: check_tasks", task.ID)).
			WithContext(ctx)
	}

	s.logger.Info("Task sent to RabbitMQ successfully",
		logger.String("task_id", task.ID),
		logger.String("check_id", task.CheckID),
		logger.String("tenant_id", task.TenantID),
		logger.String("queue", "check_tasks"),
		logger.String("scheduled_time", task.ScheduledAt.Format(time.RFC3339)),
		logger.Int("priority", int(task.Priority)),
	)

	return nil
}

// updateCheckRunTimes обновляет время последнего и следующего запуска проверки
func (s *TaskService) updateCheckRunTimes(ctx context.Context, check *domain.Check, executedAt time.Time) error {
	// Обновляем last_run
	check.LastRunAt = &executedAt

	// Обновляем next_run
	check.UpdateNextRun()

	// Обновляем updated_at
	check.UpdatedAt = time.Now()

	// Сохраняем в БД
	if err := s.checkRepo.Update(ctx, check); err != nil {
		return err
	}

	// Обновляем в планировщике
	if err := s.schedulerRepo.UpdateCheck(ctx, check); err != nil {
		s.logger.Warn("Failed to update check in scheduler",
			logger.String("check_id", check.ID),
			logger.String("error", err.Error()),
		)
	}

	return nil
}

// generateTaskID генерирует ID для задачи
func (s *TaskService) generateTaskID() string {
	return fmt.Sprintf("task_%s_%s", uuid.New().String()[:8], time.Now().Format("20060102150405"))
}

// LoadActiveChecksOnStartup загружает активные проверки при старте
func (s *TaskService) LoadActiveChecksOnStartup(ctx context.Context) error {
	s.logger.Info("Loading active checks on startup")

	checks, err := s.checkRepo.GetActiveChecks(ctx)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to load active checks").
			WithContext(ctx)
	}

	s.logger.Info("Loaded active checks",
		logger.Int("count", len(checks)),
	)

	// Создаем cron задачи для каждой проверки
	for _, check := range checks {
		if err := s.scheduleCronTask(ctx, check); err != nil {
			s.logger.Error("Failed to schedule cron task for check",
				logger.String("check_id", check.ID),
				logger.String("error", err.Error()),
			)
			continue
		}
	}

	s.logger.Info("Scheduled cron tasks for active checks",
		logger.Int("scheduled_count", len(checks)),
	)

	return nil
}

// scheduleCronTask создает cron задачу для проверки
func (s *TaskService) scheduleCronTask(ctx context.Context, check *domain.Check) error {
	var nextRunStr string
	if check.NextRunAt != nil {
		nextRunStr = check.NextRunAt.Format(time.RFC3339)
	} else {
		nextRunStr = "not set"
	}

	s.logger.Debug("Scheduling cron task for check",
		logger.String("check_id", check.ID),
		logger.String("target", check.Target),
		logger.String("next_run", nextRunStr),
	)

	// Если next_run не установлен, пропускаем
	if check.NextRunAt == nil {
		s.logger.Warn("Skipping cron task scheduling - next_run is not set",
			logger.String("check_id", check.ID),
		)
		return nil
	}

	// Создаем cron выражение на основе интервала
	cronExpr, err := s.generateCronExpression(check)
	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to generate cron expression").
			WithDetails(fmt.Sprintf("check_id: %s, interval: %d", check.ID, check.Interval)).
			WithContext(ctx)
	}

	// Добавляем задачу в cron планировщик
	_, err = s.cronScheduler.AddFunc(cronExpr, func() {
		// Создаем новый контекст для выполнения задачи
		taskCtx := context.Background()
		if err := s.ExecuteCronTask(taskCtx, check.ID); err != nil {
			s.logger.Error("Failed to execute scheduled cron task",
				logger.String("check_id", check.ID),
				logger.String("error", err.Error()),
			)
		}
	})

	if err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to add cron job").
			WithDetails(fmt.Sprintf("check_id: %s, cron_expr: %s", check.ID, cronExpr)).
			WithContext(ctx)
	}

	s.logger.Info("Cron task scheduled successfully",
		logger.String("check_id", check.ID),
		logger.String("cron_expression", cronExpr),
		logger.String("next_run", nextRunStr),
	)

	return nil
}

// GetStats возвращает статистику сервиса
func (s *TaskService) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"worker_id":      s.workerID,
		"service":        "task_service",
		"cron_entries":   s.cronScheduler.Entries(),
		"rabbitmq_connected": s.rabbitMQ != nil,
	}

	// Добавляем информацию о cron задачах
	entries := s.cronScheduler.Entries()
	stats["active_cron_jobs"] = len(entries)

	return stats
}

// generateCronExpression генерирует cron выражение на основе интервала проверки
func (s *TaskService) generateCronExpression(check *domain.Check) (string, error) {
	if check.Interval <= 0 {
		return "", fmt.Errorf("invalid interval: %d", check.Interval)
	}

	// Для простоты используем формат: каждые N секунд
	// В реальном проекте можно добавить более сложную логику
	if check.Interval < 60 {
		// Для интервалов меньше минуты
		return fmt.Sprintf("*/%d * * * * *", check.Interval), nil
	} else if check.Interval < 3600 {
		// Для интервалов меньше часа
		minutes := check.Interval / 60
		return fmt.Sprintf("0 */%d * * * *", minutes), nil
	} else {
		// Для интервалов больше часа
		hours := check.Interval / 3600
		return fmt.Sprintf("0 0 */%d * * *", hours), nil
	}
}

// Start запускает cron планировщик
func (s *TaskService) Start() {
	s.cronScheduler.Start()
	s.logger.Info("Cron scheduler started",
		logger.String("worker_id", s.workerID),
	)
}

// Stop останавливает cron планировщик
func (s *TaskService) Stop() {
	ctx := s.cronScheduler.Stop()
	select {
	case <-ctx.Done():
		s.logger.Info("Cron scheduler stopped gracefully",
			logger.String("worker_id", s.workerID),
		)
	case <-time.After(10 * time.Second):
		s.logger.Warn("Cron scheduler stop timeout",
			logger.String("worker_id", s.workerID),
		)
	}
}