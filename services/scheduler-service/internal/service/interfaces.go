package service

import (
	"context"
)

// TaskServiceInterface определяет интерфейс для TaskService
type TaskServiceInterface interface {
	// LoadActiveChecksOnStartup загружает активные проверки при старте
	LoadActiveChecksOnStartup(ctx context.Context) error
	
	// ExecuteCronTask выполняет задачу по расписанию
	ExecuteCronTask(ctx context.Context, checkID string) error
	
	// GetStats возвращает статистику сервиса
	GetStats() map[string]interface{}
	
	// Start запускает cron планировщик
	Start()
	
	// Stop останавливает cron планировщик
	Stop()
}
