package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/services/core-service/internal/repository"
	"UptimePingPlatform/services/core-service/internal/service/checker"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	pkg_redis "UptimePingPlatform/pkg/redis"
)

// CheckService предоставляет бизнес-логику для выполнения проверок
type CheckService struct {
	logger          logger.Logger
	checkerFactory  checker.CheckerFactory
	repository      repository.CheckResultRepository
	redisClient     *pkg_redis.Client
	incidentManager IncidentManager
}

// NewCheckService создает новый экземпляр CheckService
func NewCheckService(
	log logger.Logger,
	factory checker.CheckerFactory,
	repository repository.CheckResultRepository,
	redisClient *pkg_redis.Client,
	incidentManager IncidentManager,
) *CheckService {
	return &CheckService{
		logger:          log,
		checkerFactory:  factory,
		repository:      repository,
		redisClient:     redisClient,
		incidentManager: incidentManager,
	}
}

// TaskMessage представляет сообщение из RabbitMQ
type TaskMessage struct {
	CheckID      string                 `json:"check_id"`
	ExecutionID  string                 `json:"execution_id"`
	Target       string                 `json:"target"`
	Type         string                 `json:"type"`
	Config       map[string]interface{} `json:"config"`
	ScheduledAt  time.Time              `json:"scheduled_at"`
	TenantID     string                 `json:"tenant_id"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ProcessTask обрабатывает задачу проверки
func (cs *CheckService) ProcessTask(ctx context.Context, message []byte) error {
	cs.logger.Info("Starting task processing",
		logger.String("message_size", fmt.Sprintf("%d", len(message))),
	)

	// Десериализация сообщения из RabbitMQ
	taskMessage, err := cs.deserializeMessage(message)
	if err != nil {
		cs.logger.Error("Failed to deserialize message",
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrValidation, "failed to deserialize message")
	}

	cs.logger.Info("Task deserialized successfully",
		logger.String("check_id", taskMessage.CheckID),
		logger.String("execution_id", taskMessage.ExecutionID),
		logger.String("type", taskMessage.Type),
		logger.String("target", taskMessage.Target),
		logger.String("tenant_id", taskMessage.TenantID),
	)

	// Создание доменной модели Task
	task := cs.createTask(taskMessage)

	// Определение типа проверки и получение checker'а
	checker, err := cs.checkerFactory.CreateChecker(domain.TaskType(task.Type))
	if err != nil {
		cs.logger.Error("Failed to create checker",
			logger.String("type", task.Type),
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to create checker")
	}

	cs.logger.Debug("Checker created successfully",
		logger.String("type", task.Type),
	)

	// Вызов соответствующего checker'а
	result, err := cs.executeCheck(ctx, checker, task, taskMessage.TenantID)
	if err != nil {
		cs.logger.Error("Check execution failed",
			logger.String("check_id", task.CheckID),
			logger.String("tenant_id", taskMessage.TenantID),
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "check execution failed")
	}

	cs.logger.Info("Check executed successfully",
		logger.String("check_id", task.CheckID),
		logger.Bool("success", result.Success),
		logger.Int64("duration_ms", result.DurationMs),
	)

	// Сохранение результата в БД
	if err := cs.saveResult(ctx, result); err != nil {
		cs.logger.Error("Failed to save result to database",
			logger.String("check_id", task.CheckID),
			logger.Error(err),
		)
		// Не прерываем обработку, так как результат важен
	}

	// Кеширование результата в Redis (TTL 5 минут)
	if err := cs.cacheResult(ctx, result); err != nil {
		cs.logger.Warn("Failed to cache result in Redis",
			logger.String("check_id", task.CheckID),
			logger.Error(err),
		)
		// Не прерываем обработку, так как кеширование не критично
	}

	// Если проверка неудачна → отправка в Incident Manager
	if !result.Success {
		if err := cs.sendToIncidentManager(ctx, result, taskMessage.TenantID); err != nil {
			cs.logger.Error("Failed to send to incident manager",
				logger.String("check_id", task.CheckID),
				logger.String("tenant_id", taskMessage.TenantID),
				logger.Error(err),
			)
			// Не прерываем обработку, так как это уведомление
		}
	}

	// ACK сообщения (в RabbitMQ это будет делать consumer после успешной обработки)
	cs.logger.Info("Task processing completed successfully",
		logger.String("check_id", task.CheckID),
	)

	return nil
}

// deserializeMessage десериализует сообщение из RabbitMQ
func (cs *CheckService) deserializeMessage(message []byte) (*TaskMessage, error) {
	var taskMessage TaskMessage
	if err := json.Unmarshal(message, &taskMessage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task message: %w", err)
	}

	// Валидация обязательных полей
	if taskMessage.CheckID == "" {
		return nil, fmt.Errorf("check_id is required")
	}
	if taskMessage.ExecutionID == "" {
		return nil, fmt.Errorf("execution_id is required")
	}
	if taskMessage.Target == "" {
		return nil, fmt.Errorf("target is required")
	}
	if taskMessage.Type == "" {
		return nil, fmt.Errorf("type is required")
	}

	return &taskMessage, nil
}

// createTask создает доменную модель Task из TaskMessage
func (cs *CheckService) createTask(message *TaskMessage) *domain.Task {
	return domain.NewTask(
		message.CheckID,
		message.Target,
		message.Type,
		message.ExecutionID,
		message.ScheduledAt,
		message.Config,
	)
}

// executeCheck выполняет проверку
func (cs *CheckService) executeCheck(ctx context.Context, checker checker.Checker, task *domain.Task, tenantID string) (*domain.CheckResult, error) {
	cs.logger.Debug("Executing check",
		logger.String("check_id", task.CheckID),
		logger.String("type", task.Type),
		logger.String("tenant_id", tenantID),
	)

	// Выполнение проверки
	result, err := checker.Execute(task)
	if err != nil {
		cs.logger.Error("Check execution failed",
			logger.String("check_id", task.CheckID),
			logger.Error(err),
		)
		return nil, err
	}

	// Добавление метаданных
	if result.Metadata == nil {
		result.Metadata = make(map[string]string)
	}
	result.Metadata["processed_at"] = time.Now().UTC().Format(time.RFC3339)
	result.Metadata["service"] = "core-service"

	return result, nil
}

// saveResult сохраняет результат в БД
func (cs *CheckService) saveResult(ctx context.Context, result *domain.CheckResult) error {
	cs.logger.Debug("Saving result to database",
		logger.String("check_id", result.CheckID),
	)

	if cs.repository == nil {
		cs.logger.Warn("Repository is not initialized, skipping database save")
		return nil
	}

	err := cs.repository.Save(ctx, result)
	if err != nil {
		cs.logger.Error("Failed to save result to database",
			logger.String("check_id", result.CheckID),
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to save result to database")
	}

	cs.logger.Debug("Result saved to database successfully",
		logger.String("check_id", result.CheckID),
	)

	return nil
}

// cacheResult кеширует результат в Redis с TTL 5 минут
func (cs *CheckService) cacheResult(ctx context.Context, result *domain.CheckResult) error {
	cs.logger.Debug("Caching result in Redis",
		logger.String("check_id", result.CheckID),
	)

	if cs.redisClient == nil {
		cs.logger.Warn("Redis client is not initialized, skipping cache")
		return nil
	}

	key := fmt.Sprintf("check_result:%s", result.CheckID)
	data, err := json.Marshal(result)
	if err != nil {
		cs.logger.Error("Failed to marshal result for caching",
			logger.String("check_id", result.CheckID),
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to marshal result for caching")
	}

	// Устанавливаем в Redis с TTL 5 минут
	err = cs.redisClient.Client.Set(ctx, key, data, 5*time.Minute).Err()
	if err != nil {
		cs.logger.Error("Failed to cache result in Redis",
			logger.String("check_id", result.CheckID),
			logger.String("key", key),
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to cache result in Redis")
	}

	cs.logger.Debug("Result cached in Redis successfully",
		logger.String("check_id", result.CheckID),
		logger.String("key", key),
	)

	return nil
}

// sendToIncidentManager отправляет инцидент в Incident Manager
func (cs *CheckService) sendToIncidentManager(ctx context.Context, result *domain.CheckResult, tenantID string) error {
	cs.logger.Info("Sending incident to incident manager",
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", tenantID),
		logger.String("error", result.Error),
	)

	if cs.incidentManager == nil {
		cs.logger.Warn("Incident manager is not initialized, skipping incident creation")
		return nil
	}

	// Создаем инцидент из результата проверки
	incident := CreateIncidentFromCheckResult(result, tenantID)

	// Отправляем в Incident Manager
	createdIncident, err := cs.incidentManager.CreateIncident(ctx, incident)
	if err != nil {
		cs.logger.Error("Failed to send incident to incident manager",
			logger.String("check_id", result.CheckID),
			logger.String("tenant_id", tenantID),
			logger.Error(err),
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to send incident")
	}

	cs.logger.Info("Incident sent to incident manager successfully",
		logger.String("check_id", result.CheckID),
		logger.String("tenant_id", tenantID),
		logger.String("incident_id", createdIncident.ID),
		logger.String("incident_status", string(createdIncident.Status)),
		logger.String("incident_severity", string(createdIncident.Severity)),
	)

	return nil
}

// GetCachedResult получает кешированный результат из Redis
func (cs *CheckService) GetCachedResult(ctx context.Context, checkID string) (*domain.CheckResult, error) {
	cs.logger.Debug("Getting cached result from Redis",
		logger.String("check_id", checkID),
	)

	if cs.redisClient == nil {
		cs.logger.Warn("Redis client is not initialized, returning nil")
		return nil, nil
	}

	key := fmt.Sprintf("check_result:%s", checkID)
	data, err := cs.redisClient.Client.Get(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			cs.logger.Debug("No cached result found",
				logger.String("check_id", checkID),
				logger.String("key", key),
			)
			return nil, nil // Не найдено в кеше
		}
		cs.logger.Error("Failed to get cached result",
			logger.String("check_id", checkID),
			logger.String("key", key),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to get cached result")
	}

	var result domain.CheckResult
	err = json.Unmarshal([]byte(data), &result)
	if err != nil {
		cs.logger.Error("Failed to unmarshal cached result",
			logger.String("check_id", checkID),
			logger.String("key", key),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to unmarshal cached result")
	}

	cs.logger.Debug("Cached result retrieved successfully",
		logger.String("check_id", checkID),
		logger.String("key", key),
	)

	return &result, nil
}
