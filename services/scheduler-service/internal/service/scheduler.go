package service

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	"UptimePingPlatform/pkg/logger"
)

// Scheduler отвечает за планирование и выполнение проверок
type Scheduler struct {
	taskService TaskServiceInterface
	cron        *cron.Cron
	logger      logger.Logger
	isRunning   bool
	entryIDs    map[string]cron.EntryID // Хранение ID cron задач
}

// NewScheduler создает новый экземпляр Scheduler
func NewScheduler(taskService TaskServiceInterface, logger logger.Logger) *Scheduler {
	return &Scheduler{
		taskService: taskService,
		cron:        cron.New(cron.WithSeconds()), // Поддержка секунд
		logger:      logger,
		isRunning:   false,
		entryIDs:    make(map[string]cron.EntryID),
	}
}

// Start запускает планировщик
func (s *Scheduler) Start(ctx context.Context) error {
	if s.isRunning {
		return nil
	}

	s.logger.Info("Starting scheduler", logger.CtxField(ctx))

	// Загрузка активных проверок при старте
	if err := s.taskService.LoadActiveChecksOnStartup(ctx); err != nil {
		s.logger.Error("Failed to load active checks on startup",
			logger.Error(err),
			logger.CtxField(ctx),
		)
		return err
	}

	// Запускаем TaskService cron планировщик
	s.taskService.Start()

	// Запускаем основной cron планировщик
	s.cron.Start()
	s.isRunning = true

	s.logger.Info("Scheduler started successfully", logger.CtxField(ctx))

	return nil
}

// Stop останавливает планировщик
func (s *Scheduler) Stop(ctx context.Context) error {
	if !s.isRunning {
		return nil
	}

	s.logger.Info("Stopping scheduler", logger.CtxField(ctx))

	// Останавливаем TaskService cron планировщик
	s.taskService.Stop()

	// Останавливаем основной cron планировщик с контекстом и таймаутом
	done := make(chan struct{})
	go func() {
		s.cron.Stop()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Scheduler stopped successfully", logger.CtxField(ctx))
	case <-time.After(30 * time.Second):
		s.logger.Warn("Scheduler stop timeout after 30 seconds", logger.CtxField(ctx))
	}

	s.isRunning = false
	return nil
}

// AddCheck добавляет проверку в планировщик
func (s *Scheduler) AddCheck(ctx context.Context, checkID string, nextRun time.Time) error {
	if !s.isRunning {
		return nil
	}

	// Создаем cron выражение для конкретного времени
	cronExpr := s.formatTimeToCron(nextRun)

	// Добавляем задачу в cron и сохраняем Entry ID
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.taskService.ExecuteCronTask(ctx, checkID)
	})

	if err != nil {
		s.logger.Error("Failed to add check to cron scheduler",
			logger.String("check_id", checkID),
			logger.String("cron_expr", cronExpr),
			logger.String("next_run", nextRun.Format(time.RFC3339)),
			logger.Error(err),
			logger.CtxField(ctx),
		)
		return err
	}

	// Сохраняем Entry ID для возможности удаления
	s.entryIDs[checkID] = entryID

	s.logger.Debug("Added check to cron scheduler",
		logger.String("check_id", checkID),
		logger.String("cron_expr", cronExpr),
		logger.String("next_run", nextRun.Format(time.RFC3339)),
		logger.String("entry_id", fmt.Sprintf("%v", entryID)),
		logger.CtxField(ctx),
	)

	return nil
}

// RemoveCheck удаляет проверку из планировщика
func (s *Scheduler) RemoveCheck(ctx context.Context, checkID string) error {
	// Проверяем, есть ли задача в планировщике
	entryID, exists := s.entryIDs[checkID]
	if !exists {
		s.logger.Debug("Check not found in scheduler",
			logger.String("check_id", checkID),
			logger.CtxField(ctx),
		)
		return nil // Задача не найдена - считаем успешным удалением
	}

	// Удаляем задачу из cron
	s.cron.Remove(entryID)
	
	// Удаляем ID из карты
	delete(s.entryIDs, checkID)

	s.logger.Debug("Removed check from cron scheduler",
		logger.String("check_id", checkID),
		logger.String("entry_id", fmt.Sprintf("%v", entryID)),
		logger.CtxField(ctx),
	)

	return nil
}

// UpdateCheck обновляет проверку в планировщике
func (s *Scheduler) UpdateCheck(ctx context.Context, checkID string, nextRun time.Time) error {
	// Сначала удаляем старую задачу (если возможно)
	s.RemoveCheck(ctx, checkID)

	// Затем добавляем новую
	return s.AddCheck(ctx, checkID, nextRun)
}

// formatTimeToCron форматирует время в cron выражение
func (s *Scheduler) formatTimeToCron(t time.Time) string {
	// Формат: секунда минута час день месяц день_недели
	// Для однократного выполнения используем точное время
	return fmt.Sprintf("%d %d %d %d %d *",
		t.Second(),
		t.Minute(),
		t.Hour(),
		t.Day(),
		int(t.Month()),
	)
}

// IsRunning проверяет, запущен ли планировщик
func (s *Scheduler) IsRunning() bool {
	return s.isRunning
}

// GetTaskService возвращает TaskService для внешнего доступа
func (s *Scheduler) GetTaskService() TaskServiceInterface {
	return s.taskService
}

// GetStats возвращает статистику планировщика
func (s *Scheduler) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"is_running":     s.isRunning,
		"cron_entries":   s.cron.Entries(),
		"active_checks":  len(s.entryIDs),
		"entry_ids":      s.entryIDs,
	}
}
