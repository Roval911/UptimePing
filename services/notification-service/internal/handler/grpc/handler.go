package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcBase "UptimePingPlatform/pkg/grpc"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/notification-service/internal/service"

	notificationv1 "UptimePingPlatform/gen/proto/api/notification/v1"
)

// NotificationHandler реализует gRPC обработчики для NotificationService
type NotificationHandler struct {
	*grpcBase.BaseHandler
	notificationv1.UnimplementedNotificationServiceServer
	notificationService service.NotificationService
	validator           *validation.Validator
}

// NewNotificationHandler создает новый экземпляр NotificationHandler
func NewNotificationHandler(notificationService service.NotificationService, logger logger.Logger) *NotificationHandler {
	return &NotificationHandler{
		BaseHandler:       grpcBase.NewBaseHandler(logger),
		notificationService: notificationService,
		validator:         validation.NewValidator(),
	}
}

// SendNotification отправляет уведомление через указанные каналы
func (h *NotificationHandler) SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	h.LogOperationStart(ctx, "SendNotification", map[string]interface{}{
		"tenant_id":    req.TenantId,
		"incident_id":  req.IncidentId,
		"severity":     req.Severity,
		"title":        req.Title,
		"channel_ids":  req.ChannelIds,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "SendNotification", map[string]string{
		"tenant_id":   req.TenantId,
		"incident_id": req.IncidentId,
		"title":       req.Title,
		"message":     req.Message,
	}); err != nil {
		return nil, err
	}

	// Валидация tenant_id
	if err := h.validator.ValidateStringLength(req.TenantId, "tenant_id", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "SendNotification", req.TenantId)
	}

	// Валидация incident_id
	if err := h.validator.ValidateStringLength(req.IncidentId, "incident_id", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "SendNotification", req.IncidentId)
	}

	// Валидация title
	if err := h.validator.ValidateStringLength(req.Title, "title", 1, 200); err != nil {
		return nil, h.LogError(ctx, err, "SendNotification", req.Title)
	}

	// Валидация message
	if err := h.validator.ValidateStringLength(req.Message, "message", 1, 1000); err != nil {
		return nil, h.LogError(ctx, err, "SendNotification", req.Message)
	}

	// Валидация severity
	if req.Severity < notificationv1.NotificationSeverity_NOTIFICATION_SEVERITY_INFO ||
		req.Severity > notificationv1.NotificationSeverity_NOTIFICATION_SEVERITY_CRITICAL {
		return nil, h.LogError(ctx, status.Errorf(codes.InvalidArgument, "invalid severity value"), "SendNotification", req.TenantId)
	}

	// Конвертация в доменную модель
	notification := &service.Notification{
		TenantID:   req.TenantId,
		IncidentID: req.IncidentId,
		Severity:   service.NotificationSeverity(req.Severity),
		Title:      req.Title,
		Message:    req.Message,
		ChannelIDs: req.ChannelIds,
		Metadata:   req.Metadata,
	}

	// Отправка уведомления
	results, err := h.notificationService.SendNotification(ctx, notification)
	if err != nil {
		h.LogError(ctx, err, "SendNotification", req.TenantId)
		return nil, status.Errorf(codes.Internal, "failed to send notification: %v", err)
	}

	// Конвертация результатов в protobuf
	protoResults := make([]*notificationv1.SendResult, len(results))
	for i, result := range results {
		protoResults[i] = &notificationv1.SendResult{
			ChannelId: result.ChannelID,
			Success:   result.Success,
			Error:     result.Error,
		}
	}

	response := &notificationv1.SendNotificationResponse{
		Success: true,
		Results: protoResults,
	}

	h.LogOperationSuccess(ctx, "SendNotification", map[string]interface{}{
		"tenant_id":     req.TenantId,
		"incident_id":  req.IncidentId,
		"channels_sent": len(protoResults),
	})

	return response, nil
}

// RegisterChannel регистрирует новый канал уведомлений
func (h *NotificationHandler) RegisterChannel(ctx context.Context, req *notificationv1.RegisterChannelRequest) (*notificationv1.Channel, error) {
	h.LogOperationStart(ctx, "RegisterChannel", map[string]interface{}{
		"tenant_id": req.TenantId,
		"type":      req.Type,
		"name":      req.Name,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "RegisterChannel", map[string]string{
		"tenant_id": req.TenantId,
		"type":      string(req.Type),
		"name":      req.Name,
	}); err != nil {
		return nil, err
	}

	// Валидация tenant_id
	if err := h.validator.ValidateStringLength(req.TenantId, "tenant_id", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "RegisterChannel", req.TenantId)
	}

	// Валидация name
	if err := h.validator.ValidateStringLength(req.Name, "name", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "RegisterChannel", req.Name)
	}

	// Валидация типа канала
	if req.Type < notificationv1.ChannelType_CHANNEL_TYPE_TELEGRAM ||
		req.Type > notificationv1.ChannelType_CHANNEL_TYPE_EMAIL {
		return nil, h.LogError(ctx, status.Errorf(codes.InvalidArgument, "invalid channel type"), "RegisterChannel", req.TenantId)
	}

	// Конвертация в доменную модель
	channel := &service.Channel{
		TenantID: req.TenantId,
		Type:     service.ChannelType(req.Type),
		Name:     req.Name,
		Config:   req.Config,
		IsActive: true,
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	// Регистрация канала
	registeredChannel, err := h.notificationService.RegisterChannel(ctx, channel)
	if err != nil {
		h.LogError(ctx, err, "RegisterChannel", req.TenantId)
		return nil, status.Errorf(codes.Internal, "failed to register channel: %v", err)
	}

	// Конвертация в protobuf
	response := &notificationv1.Channel{
		Id:        registeredChannel.ID,
		TenantId:  registeredChannel.TenantID,
		Type:      notificationv1.ChannelType(registeredChannel.Type),
		Name:      registeredChannel.Name,
		Config:    registeredChannel.Config,
		IsActive:  registeredChannel.IsActive,
		CreatedAt: registeredChannel.CreatedAt,
		UpdatedAt: registeredChannel.UpdatedAt,
	}

	h.LogOperationSuccess(ctx, "RegisterChannel", map[string]interface{}{
		"tenant_id":   req.TenantId,
		"channel_id": response.Id,
		"channel_type": response.Type,
	})

	return response, nil
}

// UnregisterChannel удаляет канал уведомлений
func (h *NotificationHandler) UnregisterChannel(ctx context.Context, req *notificationv1.UnregisterChannelRequest) (*notificationv1.UnregisterChannelResponse, error) {
	h.LogOperationStart(ctx, "UnregisterChannel", map[string]interface{}{
		"channel_id": req.ChannelId,
	})

	// Валидация обязательных полей
	if err := h.ValidateRequiredFields(ctx, "UnregisterChannel", map[string]string{
		"channel_id": req.ChannelId,
	}); err != nil {
		return nil, err
	}

	// Валидация channel_id
	if err := h.validator.ValidateStringLength(req.ChannelId, "channel_id", 1, 100); err != nil {
		return nil, h.LogError(ctx, err, "UnregisterChannel", req.ChannelId)
	}

	// Удаление канала
	err := h.notificationService.UnregisterChannel(ctx, req.ChannelId)
	if err != nil {
		h.LogError(ctx, err, "UnregisterChannel", req.ChannelId)
		return nil, status.Errorf(codes.Internal, "failed to unregister channel: %v", err)
	}

	response := &notificationv1.UnregisterChannelResponse{
		Success: true,
	}

	h.LogOperationSuccess(ctx, "UnregisterChannel", map[string]interface{}{
		"channel_id": req.ChannelId,
	})

	return response, nil
}

// ListChannels возвращает список каналов уведомлений
func (h *NotificationHandler) ListChannels(ctx context.Context, req *notificationv1.ListChannelsRequest) (*notificationv1.ListChannelsResponse, error) {
	h.LogOperationStart(ctx, "ListChannels", map[string]interface{}{
		"tenant_id": req.TenantId,
		"type":      req.Type,
	})

	// Валидация tenant_id
	if req.TenantId != "" {
		if err := h.validator.ValidateStringLength(req.TenantId, "tenant_id", 1, 100); err != nil {
			return nil, h.LogError(ctx, err, "ListChannels", req.TenantId)
		}
	}

	// Конвертация в доменную модель
	channelType := service.ChannelType(req.Type)

	// Получение списка каналов
	channels, err := h.notificationService.ListChannels(ctx, req.TenantId, channelType)
	if err != nil {
		h.LogError(ctx, err, "ListChannels", req.TenantId)
		return nil, status.Errorf(codes.Internal, "failed to list channels: %v", err)
	}

	// Конвертация в protobuf
	protoChannels := make([]*notificationv1.Channel, len(channels))
	for i, channel := range channels {
		protoChannels[i] = &notificationv1.Channel{
			Id:        channel.ID,
			TenantId:  channel.TenantID,
			Type:      notificationv1.ChannelType(channel.Type),
			Name:      channel.Name,
			Config:    channel.Config,
			IsActive:  channel.IsActive,
			CreatedAt: channel.CreatedAt,
			UpdatedAt: channel.UpdatedAt,
		}
	}

	response := &notificationv1.ListChannelsResponse{
		Channels: protoChannels,
	}

	h.LogOperationSuccess(ctx, "ListChannels", map[string]interface{}{
		"tenant_id":     req.TenantId,
		"channels_count": len(protoChannels),
	})

	return response, nil
}
