package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	schedulerv1 "UptimePingPlatform/proto/api/scheduler/v1"
	corev1 "UptimePingPlatform/proto/api/core/v1"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
)

// parseTime парсит время из RFC3339 формата
func parseTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Now()
	}
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t
	}
	return time.Now()
}

// GRPCClient представляет gRPC клиент для взаимодействия с сервисами
type GRPCClient struct {
	schedulerClient schedulerv1.SchedulerServiceClient
	coreClient     corev1.CoreServiceClient
	schedulerConn  *grpc.ClientConn
	coreConn       *grpc.ClientConn
	logger         logger.Logger
	metrics        *metrics.Metrics
}

// NewGRPCClient создает новый gRPC клиент
func NewGRPCClient(schedulerAddr, coreAddr string, log logger.Logger) (*GRPCClient, error) {
	client := &GRPCClient{
		logger:  log,
		metrics: metrics.NewMetrics("cli-grpc-client"),
	}

	// Подключение к Scheduler Service
	if schedulerAddr != "" {
		schedulerConn, err := grpc.Dial(schedulerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("ошибка подключения к Scheduler Service: %w", err)
		}
		client.schedulerConn = schedulerConn
		client.schedulerClient = schedulerv1.NewSchedulerServiceClient(schedulerConn)
		log.Info("подключено к Scheduler Service", logger.String("address", schedulerAddr))
	}

	// Подключение к Core Service
	if coreAddr != "" {
		coreConn, err := grpc.Dial(coreAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("ошибка подключения к Core Service: %w", err)
		}
		client.coreConn = coreConn
		client.coreClient = corev1.NewCoreServiceClient(coreConn)
		log.Info("подключено к Core Service", logger.String("address", coreAddr))
	}

	return client, nil
}

// Close закрывает все соединения
func (c *GRPCClient) Close() error {
	c.logger.Info("закрытие gRPC соединений")
	
	var errors []error
	
	if c.schedulerConn != nil {
		if err := c.schedulerConn.Close(); err != nil {
			errors = append(errors, fmt.Errorf("ошибка закрытия соединения с Scheduler Service: %w", err))
		}
	}
	
	if c.coreConn != nil {
		if err := c.coreConn.Close(); err != nil {
			errors = append(errors, fmt.Errorf("ошибка закрытия соединения с Core Service: %w", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("ошибки при закрытии соединений: %v", errors)
	}
	
	return nil
}

// CreateCheck создает новую проверку через Scheduler Service
func (c *GRPCClient) CreateCheck(ctx context.Context, req *CheckCreateRequest) (*Check, error) {
	if c.schedulerClient == nil {
		return nil, fmt.Errorf("Scheduler Service не доступен")
	}

	start := time.Now()
	c.logger.Info("создание проверки через gRPC", 
		logger.String("name", req.Name),
		logger.String("type", req.Type))

	// Конвертация в protobuf сообщение
	protoReq := &schedulerv1.CreateCheckRequest{
		TenantId: c.extractTenantIDFromContext(ctx),
		Name:     req.Name,
		Type:     req.Type,
		Target:   req.Target,
		Interval: int32(req.Interval),
		Timeout:  int32(req.Timeout),
		Status:   "active",
		Priority: 1,
		Tags:     req.Tags,
		Config:   req.Metadata,
	}

	// Вызов gRPC метода
	protoResp, err := c.schedulerClient.CreateCheck(ctx, protoReq)
	duration := time.Since(start)
	
	if err != nil {
		c.logger.Error("ошибка создания проверки через gRPC", 
			logger.Error(err),
			logger.Duration("duration", duration))
		c.metrics.ErrorsCount.WithLabelValues("CreateCheck", "error", "grpc_error").Inc()
		c.metrics.RequestDuration.WithLabelValues("CreateCheck", "error").Observe(duration.Seconds())
		return nil, fmt.Errorf("ошибка gRPC вызова CreateCheck: %w", err)
	}

	// Конвертация ответа
	check := &Check{
		ID:        protoResp.Id,
		Name:      protoResp.Name,
		Type:      protoResp.Type,
		Target:    protoResp.Target,
		Interval:  int(protoResp.Interval),
		Timeout:   int(protoResp.Timeout),
		Enabled:   protoResp.Status == "active",
		Tags:      protoResp.Tags,
		Metadata:  protoResp.Config,
		CreatedAt: parseTime(protoResp.CreatedAt),
		UpdatedAt: parseTime(protoResp.UpdatedAt),
	}

	c.logger.Info("проверка создана через gRPC", 
		logger.String("check_id", check.ID),
		logger.Duration("duration", duration))
	
	c.metrics.RequestCount.WithLabelValues("CreateCheck", "success").Inc()
	c.metrics.RequestDuration.WithLabelValues("CreateCheck", "success").Observe(duration.Seconds())

	return check, nil
}

// GetCheck получает проверку по ID через Scheduler Service
func (c *GRPCClient) GetCheck(ctx context.Context, checkID string) (*Check, error) {
	if c.schedulerClient == nil {
		return nil, fmt.Errorf("Scheduler Service не доступен")
	}

	c.logger.Info("получение проверки через gRPC", logger.String("check_id", checkID))

	protoReq := &schedulerv1.GetCheckRequest{
		CheckId: checkID,
	}

	protoResp, err := c.schedulerClient.GetCheck(ctx, protoReq)
	if err != nil {
		c.logger.Error("ошибка получения проверки через gRPC", logger.Error(err))
		return nil, fmt.Errorf("ошибка gRPC вызова GetCheck: %w", err)
	}

	check := &Check{
		ID:        protoResp.Id,
		Name:      protoResp.Name,
		Type:      protoResp.Type,
		Target:    protoResp.Target,
		Interval:  int(protoResp.Interval),
		Timeout:   int(protoResp.Timeout),
		Enabled:   protoResp.Status == "active",
		Tags:      protoResp.Tags,
		Metadata:  protoResp.Config,
		CreatedAt: parseTime(protoResp.CreatedAt),
		UpdatedAt: parseTime(protoResp.UpdatedAt),
	}

	c.logger.Info("проверка получена через gRPC", logger.String("check_id", checkID))
	return check, nil
}

// UpdateCheck обновляет проверку через Scheduler Service
func (c *GRPCClient) UpdateCheck(ctx context.Context, checkID string, req *CheckUpdateRequest) (*Check, error) {
	if c.schedulerClient == nil {
		return nil, fmt.Errorf("Scheduler Service не доступен")
	}

	c.logger.Info("обновление проверки через gRPC", logger.String("check_id", checkID))

	protoReq := &schedulerv1.UpdateCheckRequest{
		CheckId: checkID,
		Name:     "",
		Type:     "",
		Target:   "",
		Interval: 0,
		Timeout:  0,
		Status:   "",
		Priority: 0,
		Tags:     []string{},
		Config:   map[string]string{},
	}

	if req.Name != nil {
		protoReq.Name = *req.Name
	}
	if req.Type != nil {
		protoReq.Type = *req.Type
	}
	if req.Target != nil {
		protoReq.Target = *req.Target
	}
	if req.Interval != nil {
		protoReq.Interval = int32(*req.Interval)
	}
	if req.Timeout != nil {
		protoReq.Timeout = int32(*req.Timeout)
	}
	if req.Enabled != nil {
		if *req.Enabled {
			protoReq.Status = "active"
		} else {
			protoReq.Status = "inactive"
		}
	}
	if len(req.Tags) > 0 {
		protoReq.Tags = req.Tags
	}
	if len(req.Metadata) > 0 {
		protoReq.Config = req.Metadata
	}

	protoResp, err := c.schedulerClient.UpdateCheck(ctx, protoReq)
	if err != nil {
		c.logger.Error("ошибка обновления проверки через gRPC", logger.Error(err))
		return nil, fmt.Errorf("ошибка gRPC вызова UpdateCheck: %w", err)
	}

	check := &Check{
		ID:        protoResp.Id,
		Name:      protoResp.Name,
		Type:      protoResp.Type,
		Target:    protoResp.Target,
		Interval:  int(protoResp.Interval),
		Timeout:   int(protoResp.Timeout),
		Enabled:   protoResp.Status == "active",
		Tags:      protoResp.Tags,
		Metadata:  protoResp.Config,
		CreatedAt: parseTime(protoResp.CreatedAt),
		UpdatedAt: parseTime(protoResp.UpdatedAt),
	}

	c.logger.Info("проверка обновлена через gRPC", logger.String("check_id", checkID))
	return check, nil
}

// ListChecks получает список проверок через Scheduler Service
func (c *GRPCClient) ListChecks(ctx context.Context, tags []string, filters map[string]interface{}, page, pageSize int) (*CheckListResponse, error) {
	if c.schedulerClient == nil {
		return nil, fmt.Errorf("Scheduler Service не доступен")
	}

	// Извлекаем enabled из filters
	var enabled *bool
	if enabledVal, ok := filters["enabled"]; ok {
		if enabledBool, ok := enabledVal.(bool); ok {
			enabled = &enabledBool
		}
	}

	c.logger.Info("получение списка проверок через gRPC", 
		logger.String("tags", strings.Join(tags, ",")),
		logger.Bool("enabled_filter", enabled != nil))

	protoReq := &schedulerv1.ListChecksRequest{
		TenantId:  c.extractTenantIDFromContext(ctx),
		PageSize:  int32(pageSize),
		PageToken: int32(page),
		Filter:    c.buildFilterFromTagsAndStatus(tags, enabled),
	}

	protoResp, err := c.schedulerClient.ListChecks(ctx, protoReq)
	if err != nil {
		c.logger.Error("ошибка получения списка проверок через gRPC", logger.Error(err))
		return nil, fmt.Errorf("ошибка gRPC вызова ListChecks: %w", err)
	}

	checks := make([]Check, len(protoResp.Checks))
	for i, protoCheck := range protoResp.Checks {
		checks[i] = Check{
			ID:        protoCheck.Id,
			Name:      protoCheck.Name,
			Type:      protoCheck.Type,
			Target:    protoCheck.Target,
			Interval:  int(protoCheck.Interval),
			Timeout:   int(protoCheck.Timeout),
			Enabled:   protoCheck.Status == "active",
			Tags:      protoCheck.Tags,
			Metadata:  protoCheck.Config,
			CreatedAt: parseTime(protoCheck.CreatedAt),
			UpdatedAt: parseTime(protoCheck.UpdatedAt),
		}
	}

	response := &CheckListResponse{
		Checks: checks,
		Total:  c.calculateTotalCount(protoResp),
	}

	c.logger.Info("список проверок получен через gRPC", 
		logger.Int("total", response.Total),
		logger.Int("returned", len(response.Checks)))

	return response, nil
}

// RunCheck запускает проверку через Core Service
func (c *GRPCClient) RunCheck(ctx context.Context, checkID string) (*CheckRunResponse, error) {
	if c.coreClient == nil {
		return nil, fmt.Errorf("Core Service не доступен")
	}

	c.logger.Info("запуск проверки через gRPC", logger.String("check_id", checkID))

	protoReq := &corev1.ExecuteCheckRequest{
		CheckId: checkID,
	}

	protoResp, err := c.coreClient.ExecuteCheck(ctx, protoReq)
	if err != nil {
		c.logger.Error("ошибка запуска проверки через gRPC", logger.Error(err))
		return nil, fmt.Errorf("ошибка gRPC вызова ExecuteCheck: %w", err)
	}

	response := &CheckRunResponse{
		ExecutionID: protoResp.ExecutionId,
		Status:      "started",
		Message:     "Проверка запущена",
		StartedAt:   parseTime(protoResp.CheckedAt),
	}

	if protoResp.Success {
		response.Status = "success"
		response.Message = "Проверка выполнена успешно"
	} else {
		response.Status = "failed"
		response.Message = protoResp.Error
	}

	c.logger.Info("проверка запущена через gRPC", 
		logger.String("check_id", checkID),
		logger.String("execution_id", response.ExecutionID))

	return response, nil
}

// GetCheckStatus получает статус проверки через Core Service
func (c *GRPCClient) GetCheckStatus(ctx context.Context, checkID string) (*CheckStatusResponse, error) {
	if c.coreClient == nil {
		return nil, fmt.Errorf("Core Service не доступен")
	}

	c.logger.Info("получение статуса проверки через gRPC", logger.String("check_id", checkID))

	protoReq := &corev1.GetCheckStatusRequest{
		CheckId: checkID,
	}

	protoResp, err := c.coreClient.GetCheckStatus(ctx, protoReq)
	if err != nil {
		c.logger.Error("ошибка получения статуса проверки через gRPC", logger.Error(err))
		return nil, fmt.Errorf("ошибка gRPC вызова GetCheckStatus: %w", err)
	}

	response := &CheckStatusResponse{
		CheckID:     checkID,
		Status:      "success",
		LastRun:     parseTime(protoResp.LastCheckedAt),
		NextRun:     c.calculateNextRunFromSchedule(checkID),
		LastStatus:  "success",
		LastMessage: "",
		IsRunning:   false,
	}

	if !protoResp.IsHealthy {
		response.Status = "failed"
		response.LastStatus = "failed"
		response.LastMessage = "Проверка не прошла"
	}

	c.logger.Info("статус проверки получен через gRPC", 
		logger.String("check_id", checkID),
		logger.String("status", response.Status))

	return response, nil
}

// GetCheckHistory получает историю выполнения проверки через Core Service
func (c *GRPCClient) GetCheckHistory(ctx context.Context, checkID string, page, pageSize int) (*CheckHistoryResponse, error) {
	if c.coreClient == nil {
		return nil, fmt.Errorf("Core Service не доступен")
	}

	c.logger.Info("получение истории проверки через gRPC", 
		logger.String("check_id", checkID),
		logger.Int("page", page),
		logger.Int("page_size", pageSize))

	protoReq := &corev1.GetCheckHistoryRequest{
		CheckId: checkID,
		Limit:   int32(pageSize),
	}

	protoResp, err := c.coreClient.GetCheckHistory(ctx, protoReq)
	if err != nil {
		c.logger.Error("ошибка получения истории проверки через gRPC", logger.Error(err))
		return nil, fmt.Errorf("ошибка gRPC вызова GetCheckHistory: %w", err)
	}

	executions := make([]CheckExecution, len(protoResp.Results))
	for i, protoExec := range protoResp.Results {
		status := "success"
		if !protoExec.Success {
			status = "failed"
			if protoExec.Error != "" {
				status = "error"
			}
		}

		executions[i] = CheckExecution{
			ExecutionID: protoExec.ExecutionId,
			CheckID:     checkID,
			Status:      status,
			Message:     protoExec.Error,
			Duration:    int(protoExec.DurationMs),
			StartedAt:   parseTime(protoExec.CheckedAt),
			CompletedAt: parseTime(protoExec.CheckedAt).Add(time.Duration(protoExec.DurationMs) * time.Millisecond),
		}
	}

	response := &CheckHistoryResponse{
		Executions: executions,
		Total:      c.calculateHistoryTotal(protoResp),
		Page:       page,
		PageSize:   pageSize,
	}

	c.logger.Info("история проверок получена через gRPC", 
		logger.String("check_id", checkID),
		logger.Int("total", response.Total),
		logger.Int("returned", len(response.Executions)))

	return response, nil
}

// extractTenantIDFromContext извлекает tenant ID из контекста
func (c *GRPCClient) extractTenantIDFromContext(ctx context.Context) string {
	// Извлекаем из контекстных значений
	if tenantID := ctx.Value("tenant_id"); tenantID != nil {
		if tid, ok := tenantID.(string); ok {
			return tid
		}
	}
	
	// Извлекаем из JWT токена в контексте
	if token := ctx.Value("access_token"); token != nil {
		if tokenStr, ok := token.(string); ok {
			return c.extractTenantIDFromToken(tokenStr)
		}
	}
	
	// Возвращаем tenant по умолчанию
	return "default-tenant"
}

// extractTenantIDFromToken извлекает tenant ID из JWT токена
func (c *GRPCClient) extractTenantIDFromToken(token string) string {
	// Удаляем префикс "Bearer " если он есть
	if strings.HasPrefix(token, "Bearer ") {
		token = token[7:]
	}

	// Парсим токен без валидации подписи
	parsedToken, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return "default-tenant"
	}

	// Извлекаем claims
	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
		// Проверяем различные возможные поля для tenant ID
		if tenantID, exists := claims["tenant_id"]; exists {
			if tid, ok := tenantID.(string); ok {
				return tid
			}
		}
		if tenantID, exists := claims["tenant"]; exists {
			if tid, ok := tenantID.(string); ok {
				return tid
			}
		}
	}

	return "default-tenant"
}

// buildFilterFromTagsAndStatus строит фильтр из тегов и статуса
func (c *GRPCClient) buildFilterFromTagsAndStatus(tags []string, enabled *bool) string {
	if len(tags) == 0 && enabled == nil {
		return ""
	}
	
	var filterParts []string
	if len(tags) > 0 {
		filterParts = append(filterParts, "tags:"+strings.Join(tags, ","))
	}
	if enabled != nil {
		status := "inactive"
		if *enabled {
			status = "active"
		}
		filterParts = append(filterParts, "status:"+status)
	}
	
	return strings.Join(filterParts, ";")
}

// calculateTotalCount вычисляет общее количество элементов
func (c *GRPCClient) calculateTotalCount(protoResp *schedulerv1.ListChecksResponse) int {
	// TotalCount поле отсутствует в protobuf, возвращаем количество элементов
	return len(protoResp.Checks)
}

// calculateNextRunFromSchedule вычисляет время следующего запуска
func (c *GRPCClient) calculateNextRunFromSchedule(checkID string) time.Time {
	// Получаем расписание из Scheduler Service и вычисляем следующее время
	// Для простоты возвращаем время через 1 час
	return time.Now().Add(1 * time.Hour)
}

// calculateHistoryTotal вычисляет общее количество записей в истории
func (c *GRPCClient) calculateHistoryTotal(protoResp *corev1.GetCheckHistoryResponse) int {
	// Проверяем наличие поля TotalCount в ответе
	// TotalCount поле отсутствует в protobuf, возвращаем количество элементов
	return len(protoResp.Results)
}
