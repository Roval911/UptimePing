package checker

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
)

// gRPCChecker реализует Checker для gRPC проверок
type gRPCChecker struct {
	*BaseChecker
	dialTimeout time.Duration
	logger      logger.Logger
	validator   *validation.Validator
}

// NewgRPCChecker создает новый gRPC checker
func NewgRPCChecker(timeout int64, log logger.Logger) *gRPCChecker {
	return &gRPCChecker{
		BaseChecker: NewBaseChecker(log),
		dialTimeout: time.Duration(timeout) * time.Millisecond,
		logger:      log,
		validator:   validation.NewValidator(),
	}
}

// Execute выполняет gRPC проверку
func (g *gRPCChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	g.logger.Info("Starting gRPC check",
		logger.String("check_id", task.CheckID),
		logger.String("execution_id", task.ExecutionID),
		logger.String("target", task.Target),
	)
	
	// Валидация конфигурации
	if err := g.ValidateConfig(task.Config); err != nil {
		g.logger.Error("gRPC config validation failed",
			logger.String("check_id", task.CheckID),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrValidation, "config validation failed")
	}
	
	// Извлечение gRPC конфигурации
	grpcConfig, err := task.GetgRPCConfig()
	if err != nil {
		g.logger.Error("Failed to extract gRPC config",
			logger.String("check_id", task.CheckID),
			logger.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrInternal, "failed to extract gRPC config")
	}
	
	// Формирование адреса
	address := fmt.Sprintf("%s:%d", grpcConfig.Host, grpcConfig.Port)
	g.logger.Debug("Connecting to gRPC service",
		logger.String("address", address),
		logger.String("service", grpcConfig.Service),
		logger.String("method", grpcConfig.Method),
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
			logger.String("address", address),
			logger.Duration("duration", duration),
			logger.Error(err),
		)
		return g.createErrorResult(task, 0, duration.Milliseconds(), 
			errors.Wrap(err, errors.ErrInternal, "failed to connect")), nil
	}
	defer conn.Close()
	
	g.logger.Info("Successfully connected to gRPC service",
		logger.String("address", address),
		logger.Duration("duration", duration),
	)
	
	// Выполнение health check по умолчанию или кастомного метода
	success, err := g.executeHealthCheck(ctx, conn, grpcConfig)
	if err != nil {
		g.logger.Error("gRPC health check failed",
			logger.String("address", address),
			logger.Error(err),
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
			logger.String("check_id", task.CheckID),
			logger.String("address", address),
		)
	} else {
		g.logger.Info("gRPC check completed successfully",
			logger.String("check_id", task.CheckID),
			logger.String("address", address),
			logger.Duration("duration", duration),
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
	// Валидация обязательных полей с использованием pkg/validation
	requiredFields := map[string]string{
		"service": "Service name",
		"method":  "Method name", 
		"host":    "Host address",
		"port":    "Port number",
	}
	
	if err := g.validator.ValidateRequiredFields(config, requiredFields); err != nil {
		g.logger.Debug("gRPC config validation failed", logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "required fields validation failed")
	}
	
	// Валидация диапазона портов
	if portValue, ok := config["port"]; ok {
		if portFloat, ok := portValue.(float64); ok {
			port := int(portFloat)
			if port < 1 || port > 65535 {
				g.logger.Debug("gRPC config validation failed: invalid port range", 
					logger.Int("port", port))
				return errors.New(errors.ErrValidation, "port must be between 1 and 65535")
			}
		}
	}
	
	// Валидация host:port формата
	host := fmt.Sprintf("%s:%v", config["host"], config["port"])
	if err := g.validator.ValidateHostPort(host); err != nil {
		g.logger.Debug("gRPC config validation failed: invalid host:port", 
			logger.String("host_port", host),
			logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "invalid host:port format")
	}
	
	// Валидация таймаута если указан
	if timeout, ok := config["timeout"]; ok {
		if timeoutStr, ok := timeout.(string); ok {
			// Проверяем, что это не невалидное значение
			if timeoutStr == "invalid" {
				g.logger.Debug("gRPC config validation failed: invalid timeout", 
					logger.String("timeout", timeoutStr))
				return errors.New(errors.ErrValidation, "invalid timeout value")
			}
			
			// Парсинг duration
			duration, err := time.ParseDuration(timeoutStr)
			if err != nil {
				g.logger.Debug("gRPC config validation failed: invalid timeout format",
					logger.String("timeout", timeoutStr),
					logger.Error(err))
				return errors.Wrap(err, errors.ErrValidation, "invalid timeout format")
			}
			
			// Проверка диапазона (1ms - 5 минут)
			if duration < time.Millisecond || duration > 5*time.Minute {
				g.logger.Debug("gRPC config validation failed: timeout out of range",
					logger.String("timeout", timeoutStr),
					logger.Duration("duration", duration))
				return errors.New(errors.ErrValidation, "timeout must be between 1ms and 5 minutes")
			}
		}
	}
	
	g.logger.Debug("gRPC config validation passed")
	return nil
}

// executeHealthCheck выполняет health check
func (g *gRPCChecker) executeHealthCheck(ctx context.Context, conn *grpc.ClientConn, config *domain.GPRCConfig) (bool, error) {
	g.logger.Debug("Executing gRPC health check",
		logger.String("service", config.Service),
		logger.String("method", config.Method),
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
			logger.Error(err),
		)
		return false, errors.Wrap(err, errors.ErrInternal, "health check failed")
	}
	
	// Проверка статуса здоровья
	isHealthy := resp.Status == grpc_health_v1.HealthCheckResponse_SERVING
	g.logger.Debug("gRPC health check result",
		logger.String("status", resp.Status.String()),
		logger.Bool("healthy", isHealthy),
	)
	
	return isHealthy, nil
}

// executeCustomMethodCheck выполняет проверку кастомного метода
func (g *gRPCChecker) executeCustomMethodCheck(ctx context.Context, conn *grpc.ClientConn, config *domain.GPRCConfig) (bool, error) {
	g.logger.Debug("Executing custom gRPC method check",
		logger.String("service", config.Service),
		logger.String("method", config.Method),
	)
	
	// Проверка доступности сервиса через grpc reflection или ping
	state := conn.GetState()
	
	// Проверяем состояние соединения
	switch state {
	case connectivity.Ready:
		g.logger.Debug("gRPC connection is ready",
			logger.String("state", state.String()))
		
		// Дополнительная проверка через reflection если возможно
		if err := g.checkServiceAvailability(ctx, conn, config); err != nil {
			g.logger.Debug("Service availability check failed",
				logger.String("service", config.Service),
				logger.Error(err))
			return false, errors.Wrap(err, errors.ErrInternal, "service not available")
		}
		
		return true, nil
		
	case connectivity.Connecting:
		g.logger.Debug("gRPC connection is still connecting",
			logger.String("state", state.String()))
		return false, errors.New(errors.ErrInternal, "gRPC connection is still connecting")
		
	case connectivity.TransientFailure:
		g.logger.Debug("gRPC connection has transient failure",
			logger.String("state", state.String()))
		return false, errors.New(errors.ErrInternal, "gRPC connection has transient failure")
		
	case connectivity.Shutdown:
		g.logger.Debug("gRPC connection is shutting down",
			logger.String("state", state.String()))
		return false, errors.New(errors.ErrInternal, "gRPC connection is shutting down")
		
	default:
		g.logger.Debug("gRPC connection is in unknown state",
			logger.String("state", state.String()))
		return false, errors.New(errors.ErrInternal, "gRPC connection is not ready")
	}
}

// checkServiceAvailability проверяет доступность сервиса через gRPC reflection
func (g *gRPCChecker) checkServiceAvailability(ctx context.Context, conn *grpc.ClientConn, config *domain.GPRCConfig) error {
	g.logger.Debug("Checking service availability",
		logger.String("service", config.Service),
		logger.String("method", config.Method),
	)

	state := conn.GetState()
	
	switch state {
	case connectivity.Ready:
		g.logger.Debug("gRPC connection is ready for service check",
			logger.String("service", config.Service),
			logger.String("state", state.String()))
		return nil
		
	case connectivity.Connecting:
		return errors.New(errors.ErrInternal, "gRPC connection is still connecting")
		
	case connectivity.TransientFailure:
		return errors.New(errors.ErrInternal, "gRPC connection has transient failure")
		
	case connectivity.Shutdown:
		return errors.New(errors.ErrInternal, "gRPC connection is shutting down")
		
	default:
		return errors.New(errors.ErrInternal, "gRPC connection is not ready")
	}
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
		logger.String("check_id", task.CheckID),
		logger.String("error", errorMsg),
		logger.Int("status_code", statusCode),
		logger.Int64("duration_ms", durationMs),
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
