package client

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"UptimePingPlatform/gen/go/proto/api/scheduler/v1"
	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/logger"
)

// SchedulerClient интерфейс для клиента планировщика
type SchedulerClient interface {
	ScheduleCheck(ctx context.Context, checkID, cronExpression string) (*v1.Schedule, error)
	UnscheduleCheck(ctx context.Context, checkID string) (*v1.UnscheduleCheckResponse, error)
	GetSchedule(ctx context.Context, checkID string) (*v1.Schedule, error)
	ListSchedules(ctx context.Context, pageSize, pageToken int32, filter string) (*v1.ListSchedulesResponse, error)
	Close() error
}

// GrpcSchedulerClient реализация SchedulerClient с использованием gRPC
type GrpcSchedulerClient struct {
	conn   *grpc.ClientConn
	logger logger.Logger
}

// NewGrpcSchedulerClient создает новый экземпляр GrpcSchedulerClient
func NewGrpcSchedulerClient(cfg *config.Config, log logger.Logger) (*GrpcSchedulerClient, error) {
	// Настройка keepalive
	keepAliveParams := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             2 * time.Second,
		PermitWithoutStream: true,
	}

	// Подключение с опциями
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepAliveParams),
	}

	// Формируем адрес сервиса - преобразуем порт в строку
	address := cfg.SchedulerService.Host + ":" + strconv.Itoa(cfg.SchedulerService.Port)

	log.Info("Connecting to scheduler service",
		logger.String("address", address),
		logger.String("host", cfg.SchedulerService.Host),
		logger.Int("port", cfg.SchedulerService.Port),
	)

	// Создание connection
	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		log.Error("Failed to connect to scheduler service",
			logger.String("error", err.Error()),
			logger.String("address", address),
		)
		return nil, err
	}

	log.Info("Successfully connected to scheduler service",
		logger.String("address", address),
	)

	return &GrpcSchedulerClient{
		conn:   conn,
		logger: log,
	}, nil
}

// ScheduleCheck планирует проверку
func (c *GrpcSchedulerClient) ScheduleCheck(ctx context.Context, checkID, cronExpression string) (*v1.Schedule, error) {
	// Создаем клиент
	client := v1.NewSchedulerServiceClient(c.conn)

	// Создаем запрос согласно proto файлу
	request := &v1.ScheduleCheckRequest{
		CheckId:        checkID,
		CronExpression: cronExpression,
	}

	// Выполняем запрос
	response, err := client.ScheduleCheck(ctx, request)
	if err != nil {
		c.logger.Error("Failed to schedule check",
			logger.String("error", err.Error()),
			logger.String("check_id", checkID),
			logger.String("cron_expression", cronExpression),
		)
		return nil, err
	}

	c.logger.Debug("Successfully scheduled check",
		logger.String("check_id", response.CheckId),
		logger.String("cron_expression", response.CronExpression),
		logger.Bool("is_active", response.IsActive),
	)

	return response, nil
}

// UnscheduleCheck отменяет расписание проверки
func (c *GrpcSchedulerClient) UnscheduleCheck(ctx context.Context, checkID string) (*v1.UnscheduleCheckResponse, error) {
	// Создаем клиент
	client := v1.NewSchedulerServiceClient(c.conn)

	// Создаем запрос согласно proto файлу
	request := &v1.UnscheduleCheckRequest{
		CheckId: checkID,
	}

	// Выполняем запрос
	response, err := client.UnscheduleCheck(ctx, request)
	if err != nil {
		c.logger.Error("Failed to unschedule check",
			logger.String("error", err.Error()),
			logger.String("check_id", checkID),
		)
		return nil, err
	}

	c.logger.Debug("Successfully unscheduled check",
		logger.String("check_id", checkID),
		logger.Bool("success", response.Success),
	)

	return response, nil
}

// GetSchedule получает расписание проверки
func (c *GrpcSchedulerClient) GetSchedule(ctx context.Context, checkID string) (*v1.Schedule, error) {
	// Создаем клиент
	client := v1.NewSchedulerServiceClient(c.conn)

	// Создаем запрос согласно proto файлу
	request := &v1.GetScheduleRequest{
		CheckId: checkID,
	}

	// Выполняем запрос
	response, err := client.GetSchedule(ctx, request)
	if err != nil {
		c.logger.Error("Failed to get schedule",
			logger.String("error", err.Error()),
			logger.String("check_id", checkID),
		)
		return nil, err
	}

	c.logger.Debug("Successfully got schedule",
		logger.String("check_id", response.CheckId),
		logger.String("cron_expression", response.CronExpression),
		logger.String("next_run", response.NextRun),
	)

	return response, nil
}

// ListSchedules получает список расписаний
func (c *GrpcSchedulerClient) ListSchedules(ctx context.Context, pageSize, pageToken int32, filter string) (*v1.ListSchedulesResponse, error) {
	// Создаем клиент
	client := v1.NewSchedulerServiceClient(c.conn)

	// Создаем запрос согласно proto файлу
	request := &v1.ListSchedulesRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
	}

	// Выполняем запрос
	response, err := client.ListSchedules(ctx, request)
	if err != nil {
		c.logger.Error("Failed to list schedules",
			logger.String("error", err.Error()),
			logger.Int32("page_size", pageSize),
			logger.Int32("page_token", pageToken),
			logger.String("filter", filter),
		)
		return nil, err
	}

	c.logger.Debug("Successfully listed schedules",
		logger.Int("count", len(response.Schedules)),
		logger.Int32("next_page_token", response.NextPageToken),
	)

	return response, nil
}

// Close закрывает соединение
func (c *GrpcSchedulerClient) Close() error {
	if c.conn != nil {
		c.logger.Info("Closing scheduler client connection")
		return c.conn.Close()
	}
	return nil
}
