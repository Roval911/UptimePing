package usecase

import (
	"context"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/scheduler-service/internal/repository"
	"UptimePingPlatform/services/scheduler-service/internal/service"
)

// SchedulerUseCase предоставляет бизнес-логику для управления планировщиком
type SchedulerUseCase struct {
	scheduler *service.Scheduler
	logger    logger.Logger
}

// NewSchedulerUseCase создает новый экземпляр SchedulerUseCase
func NewSchedulerUseCase(
	checkRepo repository.CheckRepository,
	taskRepo repository.TaskRepository,
	lockRepo repository.LockRepository,
	schedulerRepo repository.SchedulerRepository,
	logger logger.Logger,
) *SchedulerUseCase {
	// Создаем TaskService
	taskService := service.NewTaskService(checkRepo, taskRepo, lockRepo, schedulerRepo, nil, logger)

	// Создаем Scheduler
	scheduler := service.NewScheduler(taskService, logger)

	return &SchedulerUseCase{
		scheduler: scheduler,
		logger:    logger,
	}
}

// Start запускает планировщик
func (uc *SchedulerUseCase) Start(ctx context.Context) error {
	uc.logger.Info("Starting scheduler use case", logger.CtxField(ctx))

	return uc.scheduler.Start(ctx)
}

// Stop останавливает планировщик
func (uc *SchedulerUseCase) Stop(ctx context.Context) error {
	uc.logger.Info("Stopping scheduler use case", logger.CtxField(ctx))

	return uc.scheduler.Stop(ctx)
}

// ExecuteTask выполняет конкретную задачу
func (uc *SchedulerUseCase) ExecuteTask(ctx context.Context, checkID string) error {
	uc.logger.Info("Executing task via use case",
		logger.String("check_id", checkID),
		logger.CtxField(ctx),
	)

	// Получаем TaskService из планировщика
	taskService := uc.scheduler.GetTaskService()

	return taskService.ExecuteCronTask(ctx, checkID)
}

// GetStats возвращает статистику планировщика
func (uc *SchedulerUseCase) GetStats() map[string]interface{} {
	return uc.scheduler.GetStats()
}

// IsRunning проверяет, запущен ли планировщик
func (uc *SchedulerUseCase) IsRunning() bool {
	return uc.scheduler.IsRunning()
}
