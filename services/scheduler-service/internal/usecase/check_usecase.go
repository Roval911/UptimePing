package usecase

import (
	"context"
	"fmt"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/scheduler-service/internal/domain"
	"UptimePingPlatform/services/scheduler-service/internal/repository"
	"github.com/google/uuid"
)

// CheckUseCase реализует бизнес-логику для управления проверками
type CheckUseCase struct {
	checkRepo     repository.CheckRepository
	schedulerRepo repository.SchedulerRepository
	logger        logger.Logger
}

// NewCheckUseCase создает новый экземпляр CheckUseCase
func NewCheckUseCase(
	checkRepo repository.CheckRepository,
	schedulerRepo repository.SchedulerRepository,
	logger logger.Logger,
) *CheckUseCase {
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

	// Генерация check_id (UUID)
	checkID := uuid.New().String()
	check.ID = checkID

	// Установка временных меток
	now := time.Now()
	check.CreatedAt = now
	check.UpdatedAt = now

	// Установка времени следующего запуска для активных проверок
	if check.Enabled {
		check.UpdateNextRun()
	}

	// Сохранение в БД
	if err := uc.checkRepo.Create(ctx, check); err != nil {
		return nil, fmt.Errorf("failed to create check: %w", err)
	}

	// Если enabled = true → добавление в планировщик
	if check.Enabled {
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

	uc.logger.Info("Check created successfully",
		logger.CtxField(ctx),
		logger.String("check_id", checkID),
		logger.String("tenant_id", tenantID),
		logger.String("name", check.Name),
		logger.String("type", string(check.Type)),
		logger.Bool("enabled", check.Enabled),
	)

	return check, nil
}

// UpdateCheck обновляет существующую проверку
func (uc *CheckUseCase) UpdateCheck(ctx context.Context, checkID string, check *domain.Check) error {
	// Получаем существующую проверку
	existingCheck, err := uc.checkRepo.GetByID(ctx, checkID)
	if err != nil {
		return fmt.Errorf("failed to get existing check: %w", err)
	}

	// Устанавливаем ID для обновляемой проверки
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
	if check.Enabled {
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
	if check.Enabled {
		if err := uc.schedulerRepo.AddCheck(ctx, check); err != nil {
			return fmt.Errorf("check updated but failed to add to scheduler: %w", err)
		}
	}

	uc.logger.Info("Check updated successfully",
		logger.CtxField(ctx),
		logger.String("check_id", checkID),
		logger.String("name", check.Name),
		logger.Bool("enabled", check.Enabled),
	)

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
	if check.Enabled {
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

	uc.logger.Info("Check deleted successfully",
		logger.CtxField(ctx),
		logger.String("check_id", checkID),
		logger.String("tenant_id", check.TenantID),
	)

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

// ListChecks возвращает список проверок для tenant
func (uc *CheckUseCase) ListChecks(ctx context.Context, tenantID string, pageSize int, pageToken string) ([]*domain.Check, error) {
	checks, err := uc.checkRepo.List(ctx, tenantID, pageSize, pageToken)
	if err != nil {
		return nil, fmt.Errorf("failed to list checks: %w", err)
	}

	return checks, nil
}

// GetActiveChecks возвращает список активных проверок
func (uc *CheckUseCase) GetActiveChecks(ctx context.Context) ([]*domain.Check, error) {
	checks, err := uc.checkRepo.GetActiveChecks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active checks: %w", err)
	}

	return checks, nil
}

// GetActiveChecksByTenant возвращает список активных проверок для tenant
func (uc *CheckUseCase) GetActiveChecksByTenant(ctx context.Context, tenantID string) ([]*domain.Check, error) {
	checks, err := uc.checkRepo.GetActiveChecksByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active checks by tenant: %w", err)
	}

	return checks, nil
}

// validateCheckConfigForUpdate выполняет валидацию конфигурации проверки для обновления
func (uc *CheckUseCase) validateCheckConfigForUpdate(check *domain.Check) error {
	// Базовая валидация с ID (так как он уже установлен)
	if check.ID == "" {
		return fmt.Errorf("check id is required")
	}

	return uc.validateCheckConfig(check)
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

	return uc.validateCheckConfig(check)
}

// validateCheckConfig выполняет полную валидацию конфигурации проверки
func (uc *CheckUseCase) validateCheckConfig(check *domain.Check) error {
	// Базовая валидация
	if err := check.Validate(); err != nil {
		return err
	}

	// Валидация интервала (от 5 секунд до 24 часов)
	if check.Interval < 5 || check.Interval > 86400 {
		return fmt.Errorf("interval must be between 5 seconds and 24 hours")
	}

	if check.Timeout < 1 || check.Timeout > 300 {
		return fmt.Errorf("timeout must be between 1 second and 5 minutes")
	}

	// Дополнительная валидация конфигурации в зависимости от типа
	if err := uc.validateTypeSpecificConfig(check); err != nil {
		return fmt.Errorf("type-specific validation failed: %w", err)
	}

	return nil
}

// validateTypeSpecificConfig выполняет валидацию конфигурации в зависимости от типа проверки
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

// validateHTTPConfig выполняет валидацию конфигурации для HTTP проверок
func (uc *CheckUseCase) validateHTTPConfig(check *domain.Check) error {
	// HTTP специфическая валидация
	if check.Config == nil {
		return nil
	}

	// Проверка метода, если указан
	if method, ok := check.Config["method"]; ok {
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

	// Проверка кодов ответа, если указаны
	if expectedCodes, ok := check.Config["expected_codes"]; ok {
		// Валидация формата expected_codes
		// TODO: реализовать валидацию списка кодов
		_ = expectedCodes
	}

	return nil
}

// validateGRPCConfig выполняет валидацию конфигурации для gRPC проверок
func (uc *CheckUseCase) validateGRPCConfig(check *domain.Check) error {
	// gRPC специфическая валидация
	if check.Config == nil {
		return nil
	}

	// Проверка сервиса, если указан
	if service, ok := check.Config["service"]; ok {
		if service == "" {
			return fmt.Errorf("gRPC service cannot be empty")
		}
	}

	// Проверка метода, если указан
	if method, ok := check.Config["method"]; ok {
		if method == "" {
			return fmt.Errorf("gRPC method cannot be empty")
		}
	}

	return nil
}

// validateGraphQLConfig выполняет валидацию конфигурации для GraphQL проверок
func (uc *CheckUseCase) validateGraphQLConfig(check *domain.Check) error {
	// GraphQL специфическая валидация
	if check.Config == nil {
		return nil
	}

	// Проверка query, если указан
	if query, ok := check.Config["query"]; ok {
		if query == "" {
			return fmt.Errorf("GraphQL query cannot be empty")
		}
		// TODO: добавить валидацию синтаксиса GraphQL
	}

	return nil
}

// validateTCPConfig выполняет валидацию конфигурации для TCP проверок
func (uc *CheckUseCase) validateTCPConfig(check *domain.Check) error {
	// TCP специфическая валидация
	if check.Config == nil {
		return nil
	}

	// Проверка порта, если указан
	if port, ok := check.Config["port"]; ok {
		// Валидация формата порта
		// TODO: добавить валидацию порта
		_ = port
	}

	return nil
}
