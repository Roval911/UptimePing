package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/scheduler-service/internal/domain"
	"UptimePingPlatform/services/scheduler-service/internal/repository"
)

// CheckUseCase предоставляет бизнес-логику для управления проверками
type CheckUseCase struct {
	checkRepo     repository.CheckRepository
	schedulerRepo repository.SchedulerRepository
	logger        logger.Logger
}

// NewCheckUseCase создает новый экземпляр CheckUseCase
func NewCheckUseCase(checkRepo repository.CheckRepository, schedulerRepo repository.SchedulerRepository, logger logger.Logger) *CheckUseCase {
	return &CheckUseCase{
		checkRepo:     checkRepo,
		schedulerRepo: schedulerRepo,
		logger:        logger,
	}
}

// CreateCheck создает новую проверку
func (uc *CheckUseCase) CreateCheck(ctx context.Context, tenantID string, check *domain.Check) (*domain.Check, error) {
	// Валидация конфигурации проверки (без ID, так как он будет сгенерирован)
	if err := uc.validateCheckConfigForCreate(check); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Установка tenant_id
	check.TenantID = tenantID

	// Генерация check_id (UUID)
	checkID := uuid.New().String()
	check.ID = checkID

	// Установка временных меток
	now := time.Now()
	check.CreatedAt = now
	check.UpdatedAt = now

	// Установка времени следующего запуска для активных проверок
	if check.Status == domain.CheckStatusActive {
		check.UpdateNextRun()
	}

	// Сохранение в БД
	if err := uc.checkRepo.Create(ctx, check); err != nil {
		return nil, fmt.Errorf("failed to create check: %w", err)
	}

	// Если status = active → добавление в планировщик
	if check.Status == domain.CheckStatusActive {
		if err := uc.schedulerRepo.AddCheck(ctx, check); err != nil {
			// Логируем ошибку, но не откатываем создание проверки
			uc.logger.Error("Failed to add check to scheduler",
				logger.CtxField(ctx),
				logger.String("check_id", checkID),
				logger.String("tenant_id", tenantID),
				logger.Error(err),
			)
			return check, fmt.Errorf("check created but failed to add to scheduler: %w", err)
		}
	}

	return check, nil
}

// UpdateCheck обновляет существующую проверку
func (uc *CheckUseCase) UpdateCheck(ctx context.Context, checkID string, check *domain.Check) error {
	// Получаем существующую проверку
	existingCheck, err := uc.checkRepo.GetByID(ctx, checkID)
	if err != nil {
		return fmt.Errorf("failed to get existing check: %w", err)
	}

	// Устанавливаем ID для валидации
	check.ID = checkID

	// Валидация конфигурации проверки
	if err := uc.validateCheckConfigForUpdate(check); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Сохраняем важные поля из существующей проверки
	check.TenantID = existingCheck.TenantID
	check.CreatedAt = existingCheck.CreatedAt
	check.UpdatedAt = time.Now()

	// Обновляем время следующего запуска для активных проверок
	if check.Status == domain.CheckStatusActive {
		check.UpdateNextRun()
	}

	// Сохранение в БД
	if err := uc.checkRepo.Update(ctx, check); err != nil {
		return fmt.Errorf("failed to update check: %w", err)
	}

	// Обновление в планировщике
	// Сначала удаляем старую версию
	if err := uc.schedulerRepo.RemoveCheck(ctx, checkID); err != nil {
		// Логируем ошибку, но продолжаем
		uc.logger.Warn("Failed to remove check from scheduler during update",
			logger.CtxField(ctx),
			logger.String("check_id", checkID),
			logger.Error(err),
		)
	}

	// Если проверка активна, добавляем обновленную версию
	if check.Status == domain.CheckStatusActive {
		if err := uc.schedulerRepo.AddCheck(ctx, check); err != nil {
			return fmt.Errorf("check updated but failed to add to scheduler: %w", err)
		}
	}

	return nil
}

// DeleteCheck удаляет проверку
func (uc *CheckUseCase) DeleteCheck(ctx context.Context, checkID string) error {
	// Получаем проверку для информации о статусе
	check, err := uc.checkRepo.GetByID(ctx, checkID)
	if err != nil {
		return fmt.Errorf("failed to get check: %w", err)
	}

	// Удаление из планировщика (если была активна)
	if check.Status == domain.CheckStatusActive {
		if err := uc.schedulerRepo.RemoveCheck(ctx, checkID); err != nil {
			// Логируем ошибку, но продолжаем удаление
			uc.logger.Warn("Failed to remove check from scheduler during deletion",
				logger.CtxField(ctx),
				logger.String("check_id", checkID),
				logger.String("tenant_id", check.TenantID),
				logger.Error(err),
			)
		}
	}

	// Удаление из БД
	if err := uc.checkRepo.Delete(ctx, checkID); err != nil {
		return fmt.Errorf("failed to delete check: %w", err)
	}

	return nil
}

// validateCheckConfigForUpdate выполняет валидацию конфигурации проверки для обновления
func (uc *CheckUseCase) validateCheckConfigForUpdate(check *domain.Check) error {
	// Базовая валидация с ID (так как он уже установлен)
	if check.ID == "" {
		return fmt.Errorf("check id is required")
	}
	if check.Name == "" {
		return fmt.Errorf("check name is required")
	}
	if check.Target == "" {
		return fmt.Errorf("check target is required")
	}

	// Валидация типа проверки
	switch check.Type {
	case domain.CheckTypeHTTP, domain.CheckTypeHTTPS, domain.CheckTypeGRPC, domain.CheckTypeGraphQL, domain.CheckTypeTCP:
		// Valid types
	default:
		return fmt.Errorf("invalid check type: %s", check.Type)
	}

	// Валидация интервала (от 5 секунд до 24 часов)
	if check.Interval < 5 || check.Interval > 86400 {
		return fmt.Errorf("interval must be between 5 seconds and 24 hours")
	}

	// Валидация таймаута (от 1 секунды до 5 минут)
	if check.Timeout < 1 || check.Timeout > 300 {
		return fmt.Errorf("timeout must be between 1 second and 5 minutes")
	}

	// Валидация статуса
	switch check.Status {
	case domain.CheckStatusActive, domain.CheckStatusPaused, domain.CheckStatusDisabled:
		// Valid statuses
	default:
		return fmt.Errorf("invalid check status: %s", check.Status)
	}

	// Валидация приоритета
	if check.Priority < domain.PriorityLow || check.Priority > domain.PriorityCritical {
		return fmt.Errorf("priority must be between %d and %d", domain.PriorityLow, domain.PriorityCritical)
	}

	// Дополнительная валидация конфигурации в зависимости от типа
	if err := uc.validateTypeSpecificConfig(check); err != nil {
		return fmt.Errorf("type-specific validation failed: %w", err)
	}

	return nil
}

// validateCheckConfigForCreate выполняет валидацию конфигурации проверки для создания
func (uc *CheckUseCase) validateCheckConfigForCreate(check *domain.Check) error {
	// Базовая валидация без ID (так как он будет сгенерирован)
	if check.Name == "" {
		return fmt.Errorf("check name is required")
	}
	if check.Target == "" {
		return fmt.Errorf("check target is required")
	}

	// Валидация типа проверки
	switch check.Type {
	case domain.CheckTypeHTTP, domain.CheckTypeHTTPS, domain.CheckTypeGRPC, domain.CheckTypeGraphQL, domain.CheckTypeTCP:
		// Valid types
	default:
		return fmt.Errorf("invalid check type: %s", check.Type)
	}

	// Валидация интервала (от 5 секунд до 24 часов)
	if check.Interval < 5 || check.Interval > 86400 {
		return fmt.Errorf("interval must be between 5 seconds and 24 hours")
	}

	// Валидация таймаута (от 1 секунды до 5 минут)
	if check.Timeout < 1 || check.Timeout > 300 {
		return fmt.Errorf("timeout must be between 1 second and 5 minutes")
	}

	// Валидация статуса
	switch check.Status {
	case domain.CheckStatusActive, domain.CheckStatusPaused, domain.CheckStatusDisabled:
		// Valid statuses
	default:
		return fmt.Errorf("invalid check status: %s", check.Status)
	}

	// Валидация приоритета
	if check.Priority < domain.PriorityLow || check.Priority > domain.PriorityCritical {
		return fmt.Errorf("priority must be between %d and %d", domain.PriorityLow, domain.PriorityCritical)
	}

	// Дополнительная валидация конфигурации в зависимости от типа
	if err := uc.validateTypeSpecificConfig(check); err != nil {
		return fmt.Errorf("type-specific validation failed: %w", err)
	}

	return nil
}

// validateCheckConfig выполняет полную валидацию конфигурации проверки
func (uc *CheckUseCase) validateCheckConfig(check *domain.Check) error {
	// Базовая валидация
	if err := check.Validate(); err != nil {
		return err
	}

	// Дополнительная валидация конфигурации в зависимости от типа
	if err := uc.validateTypeSpecificConfig(check); err != nil {
		return fmt.Errorf("type-specific validation failed: %w", err)
	}

	return nil
}

// validateTypeSpecificConfig выполняет валидацию специфичную для типа проверки
func (uc *CheckUseCase) validateTypeSpecificConfig(check *domain.Check) error {
	switch check.Type {
	case domain.CheckTypeHTTP, domain.CheckTypeHTTPS:
		return uc.validateHTTPConfig(check)
	case domain.CheckTypeGRPC:
		return uc.validateGRPCConfig(check)
	case domain.CheckTypeGraphQL:
		return uc.validateGraphQLConfig(check)
	case domain.CheckTypeTCP:
		return uc.validateTCPConfig(check)
	default:
		return fmt.Errorf("unsupported check type: %s", check.Type)
	}
}

// validateHTTPConfig валидирует конфигурацию HTTP/HTTPS проверки
func (uc *CheckUseCase) validateHTTPConfig(check *domain.Check) error {
	// Проверка URL формата для HTTP/HTTPS
	if check.Config == nil {
		return nil
	}

	// Проверка метода, если указан
	if method, ok := check.Config["method"].(string); ok {
		validMethods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"}
		valid := false
		for _, m := range validMethods {
			if method == m {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid HTTP method: %s", method)
		}
	}

	// Проверка ожидаемого статуса, если указан
	if expectedStatus, ok := check.Config["expected_status"].(float64); ok {
		status := int(expectedStatus)
		if status < 100 || status > 599 {
			return fmt.Errorf("invalid expected status code: %d", status)
		}
	}

	return nil
}

// validateGRPCConfig валидирует конфигурацию GRPC проверки
func (uc *CheckUseCase) validateGRPCConfig(check *domain.Check) error {
	if check.Config == nil {
		return nil
	}

	// Проверка сервиса, если указан
	if service, ok := check.Config["service"].(string); ok && service == "" {
		return fmt.Errorf("grpc service cannot be empty")
	}

	// Проверка метода, если указан
	if method, ok := check.Config["method"].(string); ok && method == "" {
		return fmt.Errorf("grpc method cannot be empty")
	}

	return nil
}

// validateGraphQLConfig валидирует конфигурацию GraphQL проверки
func (uc *CheckUseCase) validateGraphQLConfig(check *domain.Check) error {
	if check.Config == nil {
		return nil
	}

	// Проверка запроса, если указан
	if query, ok := check.Config["query"].(string); ok && query == "" {
		return fmt.Errorf("graphql query cannot be empty")
	}

	return nil
}

// GetCheck получает проверку по ID
func (uc *CheckUseCase) GetCheck(ctx context.Context, checkID string) (*domain.Check, error) {
	check, err := uc.checkRepo.GetByID(ctx, checkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get check: %w", err)
	}
	return check, nil
}

// ScheduleCheck создает расписание для проверки
func (uc *CheckUseCase) ScheduleCheck(ctx context.Context, schedule *domain.Schedule) (*domain.Schedule, error) {
	// Валидация расписания
	if schedule.CheckID == "" {
		return nil, fmt.Errorf("check_id is required")
	}
	if schedule.CronExpression == "" {
		return nil, fmt.Errorf("cron_expression is required")
	}

	// Установка времени создания
	now := time.Now()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now

	// Сохранение расписания
	createdSchedule, err := uc.schedulerRepo.Create(ctx, schedule)
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	return createdSchedule, nil
}

// UnscheduleCheck удаляет расписание проверки
func (uc *CheckUseCase) UnscheduleCheck(ctx context.Context, checkID string) error {
	err := uc.schedulerRepo.DeleteByCheckID(ctx, checkID)
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}
	return nil
}

// GetSchedule получает расписание по ID проверки
func (uc *CheckUseCase) GetSchedule(ctx context.Context, checkID string) (*domain.Schedule, error) {
	schedule, err := uc.schedulerRepo.GetByCheckID(ctx, checkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}
	return schedule, nil
}

// ListSchedulesParams параметры для списка расписаний
type ListSchedulesParams struct {
	PageSize  int
	PageToken string
	Filter    string
}

// ListSchedules получает список расписаний
func (uc *CheckUseCase) ListSchedules(ctx context.Context, params ListSchedulesParams) ([]*domain.Schedule, int, error) {
	schedules, err := uc.schedulerRepo.List(ctx, params.PageSize, params.PageToken, params.Filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list schedules: %w", err)
	}

	// Получаем общее количество
	total, err := uc.schedulerRepo.Count(ctx, params.Filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count schedules: %w", err)
	}

	return schedules, total, nil
}

// HealthCheck проверяет состояние сервиса
func (uc *CheckUseCase) HealthCheck(ctx context.Context) bool {
	// Простая проверка - пытаемся подключиться к репозиториям
	_, err := uc.checkRepo.Ping(ctx)
	if err != nil {
		uc.logger.Error("Health check failed for check repository", logger.Error(err))
		return false
	}

	_, err = uc.schedulerRepo.Ping(ctx)
	if err != nil {
		uc.logger.Error("Health check failed for scheduler repository", logger.Error(err))
		return false
	}

	return true
}

// validateTCPConfig валидирует конфигурацию TCP проверки
func (uc *CheckUseCase) validateTCPConfig(check *domain.Check) error {
	if check.Config == nil {
		return nil
	}

	// Проверка порта, если указан
	if port, ok := check.Config["port"].(float64); ok {
		p := int(port)
		if p < 1 || p > 65535 {
			return fmt.Errorf("invalid port number: %d", p)
		}
	}

	return nil
}
