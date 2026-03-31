package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/vektah/gqlparser/ast"
	"github.com/vektah/gqlparser/parser"

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
	// Валидация конфигурации проверки
	if err := uc.validateCheckConfigForCreate(check); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Проверяем, что ID уже сгенерирован на уровне handler
	if check.ID == "" {
		return nil, fmt.Errorf("check ID must be generated at handler level")
	}

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
				logger.String("check_id", check.ID),
				logger.String("tenant_id", tenantID),
				logger.Error(err),
			)
			return check, fmt.Errorf("check created but failed to add to scheduler: %w", err)
		}
	}

	uc.logger.Info("Check created successfully",
		logger.CtxField(ctx),
		logger.String("check_id", check.ID),
		logger.String("tenant_id", tenantID),
		logger.String("name", check.Name),
		logger.String("type", string(check.Type)),
		logger.Bool("enabled", check.Enabled),
	)

	return check, nil
}

// UpdateCheck обновляет существующую проверку
func (uc *CheckUseCase) UpdateCheck(ctx context.Context, checkID string, check *domain.Check) error {
	// Получаем существующую проверку для сохранения tenant_id
	existingCheck, err := uc.checkRepo.GetByID(ctx, checkID)
	if err != nil {
		return fmt.Errorf("failed to get existing check: %w", err)
	}

	// Устанавливаем ID и tenant_id для обновляемой проверки
	check.ID = checkID
	check.TenantID = existingCheck.TenantID // Сохраняем существующий tenant_id

	// Валидация конфигурации проверки
	if err := uc.validateCheckConfigForUpdate(check); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Обновляем в репозитории
	if err := uc.checkRepo.Update(ctx, check); err != nil {
		return fmt.Errorf("failed to update check: %w", err)
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

// ListSchedules возвращает список расписаний для tenant
func (uc *CheckUseCase) ListSchedules(ctx context.Context, tenantID string, pageSize int, pageToken string) ([]*domain.Schedule, error) {
	// Получаем все проверки для tenant
	checks, err := uc.checkRepo.List(ctx, tenantID, pageSize, pageToken)
	if err != nil {
		return nil, fmt.Errorf("failed to list checks for schedules: %w", err)
	}

	// Получаем расписания для каждой проверки
	var schedules []*domain.Schedule
	for _, check := range checks {
		schedule, err := uc.schedulerRepo.GetByCheckID(ctx, check.ID)
		if err != nil {
			// Если расписание не найдено, пропускаем
			continue
		}
		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// CreateSchedule создает новое расписание для проверки
func (uc *CheckUseCase) CreateSchedule(ctx context.Context, checkID, cronExpression string) (*domain.Schedule, error) {
	// Валидация UUID
	if err := validateUUID(checkID); err != nil {
		return nil, fmt.Errorf("invalid check ID: %w", err)
	}

	// Валидация cron выражения
	if err := validateCronExpression(cronExpression); err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	// Проверяем существование проверки
	_, err := uc.checkRepo.GetByID(ctx, checkID)
	if err != nil {
		return nil, fmt.Errorf("check not found: %w", err)
	}

	// Рассчитываем следующее время выполнения
	nextRunAt, err := calculateNextRun(cronExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate next run: %w", err)
	}

	// Создаем расписание
	schedule := &domain.Schedule{
		ID:             uuid.New().String(),
		CheckID:        checkID,
		CronExpression: cronExpression,
		NextRunAt:      &nextRunAt,
		LastRunAt:      nil,
		Status:         "active",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// ОТЛАДКА: Логируем перед вызовом репозитория
	uc.logger.Info("DEBUG: Before repository Create",
		logger.String("check_id", checkID),
		logger.String("cron_expression", cronExpression),
		logger.String("schedule_cron_expression", schedule.CronExpression))

	// Сохраняем в репозиторий
	createdSchedule, err := uc.schedulerRepo.Create(ctx, schedule)
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	uc.logger.Info("Schedule created",
		logger.String("schedule_id", createdSchedule.ID),
		logger.String("check_id", checkID),
		logger.String("cron_expression", cronExpression),
		logger.String("next_run_at", createdSchedule.NextRunAt.Format(time.RFC3339)))

	return createdSchedule, nil
}

// GetScheduleByCheckID получает расписание по ID проверки
func (uc *CheckUseCase) GetScheduleByCheckID(ctx context.Context, checkID string) (*domain.Schedule, error) {
	// Валидация UUID
	if err := validateUUID(checkID); err != nil {
		return nil, fmt.Errorf("invalid check ID: %w", err)
	}

	schedule, err := uc.schedulerRepo.GetByCheckID(ctx, checkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	return schedule, nil
}

// UpdateSchedule обновляет расписание
func (uc *CheckUseCase) UpdateSchedule(ctx context.Context, checkID, cronExpression string) (*domain.Schedule, error) {
	// Валидация UUID
	if err := validateUUID(checkID); err != nil {
		return nil, fmt.Errorf("invalid check ID: %w", err)
	}

	// Валидация cron выражения
	if err := validateCronExpression(cronExpression); err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	// Рассчитываем новое время выполнения
	nextRunAt, err := calculateNextRun(cronExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate next run: %w", err)
	}

	// Получаем существующее расписание
	schedule, err := uc.schedulerRepo.GetByCheckID(ctx, checkID)
	if err != nil {
		return nil, fmt.Errorf("schedule not found: %w", err)
	}

	// ✅ ИСПРАВЛЕНИЕ: Обновляем расписание
	schedule.CronExpression = cronExpression
	schedule.NextRunAt = &nextRunAt
	schedule.UpdatedAt = time.Now()

	if err := uc.schedulerRepo.Update(ctx, schedule); err != nil {
		return nil, fmt.Errorf("failed to update schedule: %w", err)
	}

	uc.logger.Info("Schedule updated",
		logger.String("schedule_id", schedule.ID),
		logger.String("check_id", checkID),
		logger.String("cron_expression", cronExpression),
		logger.String("next_run_at", nextRunAt.Format(time.RFC3339)))

	return schedule, nil
}

// DeleteSchedule удаляет расписание
func (uc *CheckUseCase) DeleteSchedule(ctx context.Context, checkID string) error {
	// Валидация UUID
	if err := validateUUID(checkID); err != nil {
		return fmt.Errorf("invalid check ID: %w", err)
	}

	// Удаляем расписание
	if err := uc.schedulerRepo.DeleteByCheckID(ctx, checkID); err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	uc.logger.Info("Schedule deleted",
		logger.String("check_id", checkID))

	return nil
}

// validateUUID валидирует UUID формат
func validateUUID(id string) error {
	if len(id) != 36 {
		return fmt.Errorf("invalid UUID format")
	}
	return nil
}

// validateCronExpression валидирует cron выражение
func validateCronExpression(expr string) error {
	if expr == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}

	// Пытаемся распарсить cron выражение с помощью библиотеки
	_, err := cron.ParseStandard(expr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// Дополнительная валидация формата
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Errorf("cron expression must have exactly 5 fields (minute hour day month weekday), got %d", len(fields))
	}

	// Валидация каждого поля
	for i, field := range fields {
		if err := validateCronField(field, i); err != nil {
			return fmt.Errorf("invalid field %d (%s): %w", i, field, err)
		}
	}

	return nil
}

// validateCronField валидирует отдельное поле cron выражения
func validateCronField(field string, fieldIndex int) error {
	// Специальная валидация для каждого поля
	switch fieldIndex {
	case 0: // minute (0-59)
		return validateCronFieldRange(field, 0, 59, "minute")
	case 1: // hour (0-23)
		return validateCronFieldRange(field, 0, 23, "hour")
	case 2: // day (1-31)
		return validateCronFieldRange(field, 1, 31, "day")
	case 3: // month (1-12)
		return validateCronFieldRange(field, 1, 12, "month")
	case 4: // weekday (0-7, где 0 и 7 = воскресенье)
		return validateCronFieldRange(field, 0, 7, "weekday")
	}

	return fmt.Errorf("invalid field index: %d", fieldIndex)
}

// validateCronFieldRange валидирует поле с учетом диапазона значений
func validateCronFieldRange(field string, min, max int, fieldName string) error {
	// Проверка на wildcard
	if field == "*" {
		return nil
	}

	// Разделение на несколько выражений через запятую
	parts := strings.Split(field, ",")
	for _, part := range parts {
		if err := validateCronPart(part, min, max, fieldName); err != nil {
			return err
		}
	}

	return nil
}

// validateCronPart валидирует часть cron выражения
func validateCronPart(part string, min, max int, fieldName string) error {
	// Проверка на диапазон (например, 1-5)
	if strings.Contains(part, "-") {
		return validateCronRange(part, min, max, fieldName)
	}

	// Проверка на шаг (например, */5 или 1-10/2)
	if strings.Contains(part, "/") {
		return validateCronStep(part, min, max, fieldName)
	}

	// Проверка на простое число
	if part == "*" {
		return nil
	}

	// Валидация числового значения
	num, err := strconv.Atoi(part)
	if err != nil {
		return fmt.Errorf("invalid %s value '%s': must be a number", fieldName, part)
	}

	if num < min || num > max {
		return fmt.Errorf("invalid %s value '%d': must be between %d and %d", fieldName, num, min, max)
	}

	return nil
}

// validateCronRange валидирует диапазон в cron выражении
func validateCronRange(part string, min, max int, fieldName string) error {
	rangeParts := strings.Split(part, "/")
	if len(rangeParts) > 2 {
		return fmt.Errorf("invalid %s range format '%s': too many '/'", fieldName, part)
	}

	// Проверка диапазона (например, 1-5)
	rangeStr := rangeParts[0]
	if !strings.Contains(rangeStr, "-") {
		return fmt.Errorf("invalid %s range format '%s': expected 'start-end'", fieldName, part)
	}

	startEnd := strings.Split(rangeStr, "-")
	if len(startEnd) != 2 {
		return fmt.Errorf("invalid %s range format '%s': expected 'start-end'", fieldName, part)
	}

	start, err := strconv.Atoi(strings.TrimSpace(startEnd[0]))
	if err != nil {
		return fmt.Errorf("invalid %s range start '%s': must be a number", fieldName, startEnd[0])
	}

	end, err := strconv.Atoi(strings.TrimSpace(startEnd[1]))
	if err != nil {
		return fmt.Errorf("invalid %s range end '%s': must be a number", fieldName, startEnd[1])
	}

	if start < min || start > max {
		return fmt.Errorf("invalid %s range start '%d': must be between %d and %d", fieldName, start, min, max)
	}

	if end < min || end > max {
		return fmt.Errorf("invalid %s range end '%d': must be between %d and %d", fieldName, end, min, max)
	}

	if start > end {
		return fmt.Errorf("invalid %s range: start (%d) must be less than or equal to end (%d)", fieldName, start, end)
	}

	// Если есть шаг, валидируем его
	if len(rangeParts) == 2 {
		step, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
		if err != nil {
			return fmt.Errorf("invalid %s step '%s': must be a number", fieldName, rangeParts[1])
		}
		if step <= 0 {
			return fmt.Errorf("invalid %s step '%d': must be positive", fieldName, step)
		}
	}

	return nil
}

// validateCronStep валидирует шаг в cron выражении
func validateCronStep(part string, min, max int, fieldName string) error {
	stepParts := strings.Split(part, "/")
	if len(stepParts) != 2 {
		return fmt.Errorf("invalid %s step format '%s': expected 'base/step'", fieldName, part)
	}

	base := stepParts[0]
	stepStr := stepParts[1]

	// Валидация шага
	step, err := strconv.Atoi(strings.TrimSpace(stepStr))
	if err != nil {
		return fmt.Errorf("invalid %s step '%s': must be a number", fieldName, stepStr)
	}
	if step <= 0 {
		return fmt.Errorf("invalid %s step '%d': must be positive", fieldName, step)
	}

	// Валидация базовой части
	if base != "*" {
		return validateCronPart(base, min, max, fieldName)
	}

	return nil
}

// calculateNextRun рассчитывает следующее время выполнения
func calculateNextRun(cronExpression string) (time.Time, error) {
	// Создаем парсер cron с использованием UTC времени
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	// Парсим cron выражение
	schedule, err := parser.Parse(cronExpression)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse cron expression '%s': %w", cronExpression, err)
	}

	// Рассчитываем следующее время выполнения от текущего момента
	nextRun := schedule.Next(time.Now())

	// Добавляем небольшую проверку на случай, если следующее выполнение слишком далеко в будущем
	maxFuture := time.Now().Add(365 * 24 * time.Hour) // максимум 1 год вперед
	if nextRun.After(maxFuture) {
		return time.Time{}, fmt.Errorf("next run time is too far in the future: %s", nextRun.Format(time.RFC3339))
	}

	return nextRun, nil
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

	// Валидация tenant_id
	if check.TenantID == "" {
		return fmt.Errorf("tenant id is required")
	}

	// Для обновления не требуем обязательные поля
	// Поля могут быть пустыми при частичном обновлении

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
		return fmt.Errorf("check interval must be between 5 seconds and 24 hours")
	}

	// Валидация таймаута (от 1 секунды до 5 минут)
	if check.Timeout < 1 || check.Timeout > 300 {
		return fmt.Errorf("check timeout must be between 1 second and 5 minutes")
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
	// Если тип не предоставлен, пропускаем валидацию
	if check.Type == "" {
		return nil
	}

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
		switch codes := expectedCodes.(type) {
		case string:
			// Формат: "200,201,202" или "200-299"
			if err := validateHTTPCodes(codes); err != nil {
				return fmt.Errorf("invalid expected_codes format: %w", err)
			}
		case []interface{}:
			// Формат: ["200", "201", "202"]
			for _, code := range codes {
				if codeStr, ok := code.(string); ok {
					if err := validateHTTPCode(codeStr); err != nil {
						return fmt.Errorf("invalid expected_code '%s': %w", codeStr, err)
					}
				} else {
					return fmt.Errorf("expected_code must be string, got %T", code)
				}
			}
		default:
			return fmt.Errorf("expected_codes must be string or array, got %T", expectedCodes)
		}
	}

	return nil
}

// validateHTTPCode валидирует один HTTP код
func validateHTTPCode(code string) error {
	codeNum, err := strconv.Atoi(code)
	if err != nil {
		return fmt.Errorf("must be a number")
	}

	if codeNum < 100 || codeNum > 599 {
		return fmt.Errorf("must be between 100 and 599")
	}

	return nil
}

// validateHTTPCodes валидирует строку с HTTP кодами
func validateHTTPCodes(codes string) error {
	if strings.Contains(codes, "-") {
		// Формат диапазона: "200-299"
		parts := strings.Split(codes, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid range format, expected 'start-end'")
		}

		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return fmt.Errorf("invalid start range: %w", err)
		}

		end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return fmt.Errorf("invalid end range: %w", err)
		}

		if start < 100 || start > 599 || end < 100 || end > 599 {
			return fmt.Errorf("range must be between 100 and 599")
		}

		if start > end {
			return fmt.Errorf("start range must be less than or equal to end range")
		}
	} else {
		// Формат списка: "200,201,202"
		codeList := strings.Split(codes, ",")
		for _, code := range codeList {
			if err := validateHTTPCode(strings.TrimSpace(code)); err != nil {
				return fmt.Errorf("invalid code '%s': %w", strings.TrimSpace(code), err)
			}
		}
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
		if queryStr, ok := query.(string); ok {
			if queryStr == "" {
				return fmt.Errorf("GraphQL query cannot be empty")
			}

			// Валидация синтаксиса GraphQL
			if err := validateGraphQLQuery(queryStr); err != nil {
				return fmt.Errorf("invalid GraphQL query syntax: %w", err)
			}
		} else {
			return fmt.Errorf("GraphQL query must be a string")
		}
	}

	return nil
}

// validateGraphQLQuery валидирует синтаксис GraphQL запроса
func validateGraphQLQuery(query string) error {
	// Создаем исходный код GraphQL
	source := &ast.Source{
		Input: query,
	}

	// Парсим GraphQL запрос
	_, err := parser.ParseQuery(source)
	if err != nil {
		return fmt.Errorf("GraphQL parsing failed: %w", err)
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
		switch portVal := port.(type) {
		case string:
			portNum, err := strconv.Atoi(portVal)
			if err != nil {
				return fmt.Errorf("port must be a number, got string '%s'", portVal)
			}
			if portNum < 1 || portNum > 65535 {
				return fmt.Errorf("port must be between 1 and 65535, got %d", portNum)
			}
		case float64:
			// JSON парсер может дать float64 для чисел
			if portNum := int(portVal); float64(portNum) != portVal {
				return fmt.Errorf("port must be an integer, got float %.0f", portVal)
			} else if portNum < 1 || portNum > 65535 {
				return fmt.Errorf("port must be between 1 and 65535, got %d", portNum)
			}
		case int:
			if portNum := portVal; portNum < 1 || portNum > 65535 {
				return fmt.Errorf("port must be between 1 and 65535, got %d", portNum)
			}
		default:
			return fmt.Errorf("port must be a number, got %T", port)
		}
	}

	return nil
}
