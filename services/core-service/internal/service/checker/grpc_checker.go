package checker

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"go.uber.org/zap"
	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
)

// gRPCChecker реализует Checker для gRPC проверок
type gRPCChecker struct {
	*BaseChecker
	dialTimeout time.Duration
	logger      logger.Logger
}

// NewgRPCChecker создает новый gRPC checker
func NewgRPCChecker(timeout int64, log logger.Logger) *gRPCChecker {
	return &gRPCChecker{
		BaseChecker: NewBaseChecker(timeout),
		dialTimeout: time.Duration(timeout) * time.Millisecond,
		logger:      log,
	}
}

// Execute выполняет gRPC проверку
func (g *gRPCChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	g.logger.Info("Starting gRPC check",
		logger.Field{zap.String("check_id", task.CheckID)},
		logger.Field{zap.String("execution_id", task.ExecutionID)},
		logger.Field{zap.String("target", task.Target)},
	)
	
	// Валидация конфигурации
	if err := g.ValidateConfig(task.Config); err != nil {
		g.logger.Error("gRPC config validation failed",
			logger.Field{zap.String("check_id", task.CheckID)},
			logger.Field{zap.Error(err)},
		)
		return nil, errors.Wrap(err, errors.ErrValidation, "config validation failed")
	}
	
	// Извлечение gRPC конфигурации
	grpcConfig, err := task.GetgRPCConfig()
	if err != nil {
		g.logger.Error("Failed to extract gRPC config",
			logger.Field{zap.String("check_id", task.CheckID)},
			logger.Field{zap.Error(err)},
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to extract gRPC config")
	}
	
	// Формирование адреса
	address := fmt.Sprintf("%s:%d", grpcConfig.Host, grpcConfig.Port)
	g.logger.Debug("Connecting to gRPC service",
		logger.Field{zap.String("address", address)},
		logger.Field{zap.String("service", grpcConfig.Service)},
		logger.Field{zap.String("method", grpcConfig.Method)},
	)
	
	// Создание контекста с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), grpcConfig.Timeout)
	defer cancel()
	
	// Добавление метаданных в контекст
	if len(grpcConfig.Metadata) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, metadata.New(grpcConfig.Metadata))
	}
	
	// Установка соединения
	startTime := time.Now()
	conn, err := grpc.DialContext(ctx, address, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	duration := time.Since(startTime)
	
	if err != nil {
		g.logger.Error("Failed to connect to gRPC service",
			logger.Field{zap.String("address", address)},
			logger.Field{zap.Duration("duration", duration)},
			logger.Field{zap.Error(err)},
		)
		return g.createErrorResult(task, 0, duration.Milliseconds(), 
			errors.Wrap(err, errors.ErrInternal, "failed to connect")), nil
	}
	defer conn.Close()
	
	g.logger.Info("Successfully connected to gRPC service",
		logger.Field{zap.String("address", address)},
		logger.Field{zap.Duration("duration", duration)},
	)
	
	// Выполнение health check по умолчанию или кастомного метода
	success, err := g.executeHealthCheck(ctx, conn, grpcConfig)
	if err != nil {
		g.logger.Error("gRPC health check failed",
			logger.Field{zap.String("address", address)},
			logger.Field{zap.Error(err)},
		)
		return g.createErrorResult(task, 0, duration.Milliseconds(), err), nil
	}
	
	// Формирование результата
	result := &domain.CheckResult{
		CheckID:      task.CheckID,
		ExecutionID:  task.ExecutionID,
		Success:      success,
		DurationMs:   duration.Milliseconds(),
		StatusCode:   200, // gRPC не имеет HTTP статус кодов, используем 200 для успеха
		CheckedAt:    time.Now().UTC(),
		Metadata:     make(map[string]string),
	}
	
	// Добавление метаданных
	result.Metadata["address"] = address
	result.Metadata["service"] = grpcConfig.Service
	result.Metadata["method"] = grpcConfig.Method
	
	if !success {
		result.Error = "gRPC health check failed"
		g.logger.Warn("gRPC health check returned unhealthy status",
			logger.Field{zap.String("check_id", task.CheckID)},
			logger.Field{zap.String("address", address)},
		)
	} else {
		g.logger.Info("gRPC check completed successfully",
			logger.Field{zap.String("check_id", task.CheckID)},
			logger.Field{zap.String("address", address)},
			logger.Field{zap.Duration("duration", duration)},
		)
	}
	
	return result, nil
}

// GetType возвращает тип checker'а
func (g *gRPCChecker) GetType() domain.TaskType {
	return domain.TaskTypeGRPC
}

// ValidateConfig валидирует gRPC конфигурацию
func (g *gRPCChecker) ValidateConfig(config map[string]interface{}) error {
	// Проверка обязательных полей
	if service, ok := config["service"]; !ok || service == "" {
		err := &ValidationError{Field: "service", Message: "required and cannot be empty"}
		g.logger.Debug("gRPC config validation failed: missing service", 
			logger.Field{zap.Error(err)})
		return err
	}
	
	if method, ok := config["method"]; !ok || method == "" {
		err := &ValidationError{Field: "method", Message: "required and cannot be empty"}
		g.logger.Debug("gRPC config validation failed: missing method", 
			logger.Field{zap.Error(err)})
		return err
	}
	
	if host, ok := config["host"]; !ok || host == "" {
		err := &ValidationError{Field: "host", Message: "required and cannot be empty"}
		g.logger.Debug("gRPC config validation failed: missing host", 
			logger.Field{zap.Error(err)})
		return err
	}
	
	if _, ok := config["port"]; !ok {
		err := &ValidationError{Field: "port", Message: "required"}
		g.logger.Debug("gRPC config validation failed: missing port", 
			logger.Field{zap.Error(err)})
		return err
	}
	
	// Валидация порта
	if portFloat, ok := config["port"].(float64); ok {
		if portFloat < 1 || portFloat > 65535 {
			err := &ValidationError{Field: "port", Message: "must be between 1 and 65535"}
			g.logger.Debug("gRPC config validation failed: invalid port range", 
				logger.Field{zap.Float64("port", portFloat)},
				logger.Field{zap.Error(err)})
			return err
		}
	} else {
		err := &ValidationError{Field: "port", Message: "must be a number"}
		g.logger.Debug("gRPC config validation failed: port not a number", 
			logger.Field{zap.Error(err)})
		return err
	}
	
	// Валидация таймаута
	if timeout, ok := config["timeout"]; ok {
		if timeoutStr, ok := timeout.(string); ok {
			if _, err := time.ParseDuration(timeoutStr); err != nil {
				err := &ValidationError{Field: "timeout", Message: "invalid duration format"}
				g.logger.Debug("gRPC config validation failed: invalid timeout format", 
					logger.Field{zap.String("timeout", timeoutStr)},
					logger.Field{zap.Error(err)})
				return err
			}
		}
	}
	
	g.logger.Debug("gRPC config validation passed")
	return nil
}

// executeHealthCheck выполняет health check
func (g *gRPCChecker) executeHealthCheck(ctx context.Context, conn *grpc.ClientConn, config *domain.GPRCConfig) (bool, error) {
	g.logger.Debug("Executing gRPC health check",
		logger.Field{zap.String("service", config.Service)},
		logger.Field{zap.String("method", config.Method)},
	)
	
	// Если это стандартный health check
	if config.Service == "grpc.health.v1.Health" && config.Method == "Check" {
		return g.executeStandardHealthCheck(ctx, conn)
	}
	
	// Для кастомных методов выполняем базовую проверку соединения
	return g.executeCustomMethodCheck(ctx, conn, config)
}

// executeStandardHealthCheck выполняет стандартный gRPC health check
func (g *gRPCChecker) executeStandardHealthCheck(ctx context.Context, conn *grpc.ClientConn) (bool, error) {
	g.logger.Debug("Executing standard gRPC health check")
	
	client := grpc_health_v1.NewHealthClient(conn)
	
	req := &grpc_health_v1.HealthCheckRequest{
		Service: "", // пустая строка для проверки сервиса по умолчанию
	}
	
	resp, err := client.Check(ctx, req)
	if err != nil {
		g.logger.Error("Standard gRPC health check failed",
			logger.Field{zap.Error(err)},
		)
		return false, errors.Wrap(err, errors.ErrInternal, "health check failed")
	}
	
	// Проверка статуса здоровья
	isHealthy := resp.Status == grpc_health_v1.HealthCheckResponse_SERVING
	g.logger.Debug("gRPC health check result",
		logger.Field{zap.String("status", resp.Status.String())},
		logger.Field{zap.Bool("healthy", isHealthy)},
	)
	
	return isHealthy, nil
}

// executeCustomMethodCheck выполняет проверку кастомного метода
func (g *gRPCChecker) executeCustomMethodCheck(ctx context.Context, conn *grpc.ClientConn, config *domain.GPRCConfig) (bool, error) {
	g.logger.Debug("Executing custom gRPC method check",
		logger.Field{zap.String("service", config.Service)},
		logger.Field{zap.String("method", config.Method)},
	)
	
	// Для кастомных методов просто проверяем, что соединение установлено
	// В реальной реализации здесь можно вызывать конкретные методы сервиса
	
	//todo Проверка доступности сервиса через grpc reflection или ping
	// Сейчас просто возвращаем true если соединение установлено
	state := conn.GetState().String()
	isReady := state == "READY"
	
	g.logger.Debug("Custom gRPC method check result",
		logger.Field{zap.String("connection_state", state)},
		logger.Field{zap.Bool("ready", isReady)},
	)
	
	return isReady, nil
}

// createErrorResult создает результат с ошибкой
func (g *gRPCChecker) createErrorResult(task *domain.Task, statusCode int, durationMs int64, err error) *domain.CheckResult {
	var errorMsg string
	if customErr, ok := err.(*errors.Error); ok {
		errorMsg = customErr.Error()
	} else {
		errorMsg = err.Error()
	}
	
	g.logger.Debug("Creating error result for gRPC check",
		logger.Field{zap.String("check_id", task.CheckID)},
		logger.Field{zap.String("error", errorMsg)},
		logger.Field{zap.Int("status_code", statusCode)},
		logger.Field{zap.Int64("duration_ms", durationMs)},
	)
	
	return &domain.CheckResult{
		CheckID:      task.CheckID,
		ExecutionID:  task.ExecutionID,
		Success:      false,
		DurationMs:   durationMs,
		StatusCode:   statusCode,
		Error:        errorMsg,
		ResponseBody: "",
		CheckedAt:    time.Now().UTC(),
		Metadata:     make(map[string]string),
	}
}

// SetDialTimeout устанавливает таймаут подключения
func (g *gRPCChecker) SetDialTimeout(timeout time.Duration) {
	g.dialTimeout = timeout
}

// GetDialTimeout возвращает таймаут подключения
func (g *gRPCChecker) GetDialTimeout() time.Duration {
	return g.dialTimeout
}
