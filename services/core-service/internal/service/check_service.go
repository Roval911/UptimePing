package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/services/core-service/internal/service/checker"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
)

// CheckService предоставляет бизнес-логику для выполнения проверок
type CheckService struct {
	logger      logger.Logger
	checkerFactory checker.CheckerFactory
	// TODO: Добавить зависимости для БД, Redis, Incident Manager
	// repository      repository.CheckResultRepository
	// redisClient     redis.Client
	// incidentManager incident.Manager
}

// NewCheckService создает новый экземпляр CheckService
func NewCheckService(
	log logger.Logger,
	factory checker.CheckerFactory,
) *CheckService {
	return &CheckService{
		logger:         log,
		checkerFactory: factory,
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
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ProcessTask обрабатывает задачу проверки
func (cs *CheckService) ProcessTask(ctx context.Context, message []byte) error {
	cs.logger.Info("Starting task processing",
		logger.Field{zap.String("message_size", fmt.Sprintf("%d", len(message)))},
	)

	// Десериализация сообщения из RabbitMQ
	taskMessage, err := cs.deserializeMessage(message)
	if err != nil {
		cs.logger.Error("Failed to deserialize message",
			logger.Field{zap.Error(err)},
		)
		return errors.Wrap(err, errors.ErrValidation, "failed to deserialize message")
	}

	cs.logger.Info("Task deserialized successfully",
		logger.Field{zap.String("check_id", taskMessage.CheckID)},
		logger.Field{zap.String("execution_id", taskMessage.ExecutionID)},
		logger.Field{zap.String("type", taskMessage.Type)},
		logger.Field{zap.String("target", taskMessage.Target)},
	)

	// Создание доменной модели Task
	task := cs.createTask(taskMessage)

	// Определение типа проверки и получение checker'а
	checker, err := cs.checkerFactory.CreateChecker(domain.TaskType(task.Type))
	if err != nil {
		cs.logger.Error("Failed to create checker",
			logger.Field{zap.String("type", task.Type)},
			logger.Field{zap.Error(err)},
		)
		return errors.Wrap(err, errors.ErrInternal, "failed to create checker")
	}

	cs.logger.Debug("Checker created successfully",
		logger.Field{zap.String("type", task.Type)},
	)

	// Вызов соответствующего checker'а
	result, err := cs.executeCheck(ctx, checker, task)
	if err != nil {
		cs.logger.Error("Check execution failed",
			logger.Field{zap.String("check_id", task.CheckID)},
			logger.Field{zap.Error(err)},
		)
		return errors.Wrap(err, errors.ErrInternal, "check execution failed")
	}

	cs.logger.Info("Check executed successfully",
		logger.Field{zap.String("check_id", task.CheckID)},
		logger.Field{zap.Bool("success", result.Success)},
		logger.Field{zap.Int64("duration_ms", result.DurationMs)},
	)

	// Сохранение результата в БД
	if err := cs.saveResult(ctx, result); err != nil {
		cs.logger.Error("Failed to save result to database",
			logger.Field{zap.String("check_id", task.CheckID)},
			logger.Field{zap.Error(err)},
		)
		// Не прерываем обработку, так как результат важен
	}

	// Кеширование результата в Redis (TTL 5 минут)
	if err := cs.cacheResult(ctx, result); err != nil {
		cs.logger.Warn("Failed to cache result in Redis",
			logger.Field{zap.String("check_id", task.CheckID)},
			logger.Field{zap.Error(err)},
		)
		// Не прерываем обработку, так как кеширование не критично
	}

	// Если проверка неудачна → отправка в Incident Manager
	if !result.Success {
		if err := cs.sendToIncidentManager(ctx, result); err != nil {
			cs.logger.Error("Failed to send to incident manager",
				logger.Field{zap.String("check_id", task.CheckID)},
				logger.Field{zap.Error(err)},
			)
			// Не прерываем обработку, так как это уведомление
		}
	}

	// ACK сообщения (в RabbitMQ это будет делать consumer после успешной обработки)
	cs.logger.Info("Task processing completed successfully",
		logger.Field{zap.String("check_id", task.CheckID)},
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
func (cs *CheckService) executeCheck(ctx context.Context, checker checker.Checker, task *domain.Task) (*domain.CheckResult, error) {
	cs.logger.Debug("Executing check",
		logger.Field{zap.String("check_id", task.CheckID)},
		logger.Field{zap.String("type", task.Type)},
	)

	// Выполнение проверки
	result, err := checker.Execute(task)
	if err != nil {
		cs.logger.Error("Check execution failed",
			logger.Field{zap.String("check_id", task.CheckID)},
			logger.Field{zap.Error(err)},
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
		logger.Field{zap.String("check_id", result.CheckID)},
	)

	// TODO: Реализовать сохранение в БД через repository
	// err := cs.repository.Save(ctx, result)
	// if err != nil {
	//     return errors.Wrap(err, errors.ErrInternal, "failed to save result to database")
	// }

	cs.logger.Debug("Result saved to database (mock)")
	return nil
}

// cacheResult кеширует результат в Redis с TTL 5 минут
func (cs *CheckService) cacheResult(ctx context.Context, result *domain.CheckResult) error {
	cs.logger.Debug("Caching result in Redis",
		logger.Field{zap.String("check_id", result.CheckID)},
	)

	// TODO: Реализовать кеширование в Redis
	// key := fmt.Sprintf("check_result:%s", result.CheckID)
	// data, err := json.Marshal(result)
	// if err != nil {
	//     return errors.Wrap(err, errors.ErrInternal, "failed to marshal result for caching")
	// }
	// 
	// err = cs.redisClient.Set(ctx, key, data, 5*time.Minute).Err()
	// if err != nil {
	//     return errors.Wrap(err, errors.ErrInternal, "failed to cache result in Redis")
	// }

	cs.logger.Debug("Result cached in Redis (mock)")
	return nil
}

// sendToIncidentManager отправляет инцидент в Incident Manager
func (cs *CheckService) sendToIncidentManager(ctx context.Context, result *domain.CheckResult) error {
	cs.logger.Info("Sending incident to incident manager",
		logger.Field{zap.String("check_id", result.CheckID)},
		logger.Field{zap.String("error", result.Error)},
	)

	// TODO: Реализовать отправку в Incident Manager
	// incident := incident.Incident{
	//     CheckID:     result.CheckID,
	//     ExecutionID: result.ExecutionID,
	//     Error:       result.Error,
	//     StatusCode:  result.StatusCode,
	//     CreatedAt:   result.CheckedAt,
	// }
	// 
	// err := cs.incidentManager.Create(ctx, incident)
	// if err != nil {
	//     return errors.Wrap(err, errors.ErrInternal, "failed to send incident")
	// }

	cs.logger.Info("Incident sent to incident manager (mock)")
	return nil
}

// GetCachedResult получает кешированный результат из Redis
func (cs *CheckService) GetCachedResult(ctx context.Context, checkID string) (*domain.CheckResult, error) {
	cs.logger.Debug("Getting cached result from Redis",
		logger.Field{zap.String("check_id", checkID)},
	)

	// TODO: Реализовать получение из Redis
	// key := fmt.Sprintf("check_result:%s", checkID)
	// data, err := cs.redisClient.Get(ctx, key).Result()
	// if err != nil {
	//     if err == redis.Nil {
	//         return nil, nil // Не найдено в кеше
	//     }
	//     return nil, errors.Wrap(err, errors.ErrInternal, "failed to get cached result")
	// }
	// 
	// var result domain.CheckResult
	// err = json.Unmarshal([]byte(data), &result)
	// if err != nil {
	//     return nil, errors.Wrap(err, errors.ErrInternal, "failed to unmarshal cached result")
	// }

	cs.logger.Debug("No cached result found (mock)")
	return nil, nil
}
