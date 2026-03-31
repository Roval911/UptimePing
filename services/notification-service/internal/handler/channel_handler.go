package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/api"
	"UptimePingPlatform/services/notification-service/internal/service"
)

// ChannelHandler обрабатывает HTTP запросы для управления каналами
type ChannelHandler struct {
	channelService service.NotificationService
	logger         logger.Logger
}

// NewChannelHandler создает новый ChannelHandler
func NewChannelHandler(channelService service.NotificationService, logger logger.Logger) *ChannelHandler {
	return &ChannelHandler{
		channelService: channelService,
		logger:         logger,
	}
}

// RegisterRoutes регистрирует маршруты для управления каналами
func (h *ChannelHandler) RegisterRoutes(mux *http.ServeMux) {
	// Маршруты для управления каналами
	mux.HandleFunc("/api/v1/notification/channels", h.handleChannels)
	mux.HandleFunc("/api/v1/notification/channels/", h.handleChannelByID)
}

// handleChannels обрабатывает запросы к списку каналов (GET - получить все, POST - создать)
func (h *ChannelHandler) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listChannels(w, r)
	case http.MethodPost:
		h.createChannel(w, r)
	default:
		api.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
	}
}

// handleChannelByID обрабатывает запросы к конкретному каналу
func (h *ChannelHandler) handleChannelByID(w http.ResponseWriter, r *http.Request) {
	// Извлекаем channel_id из URL
	channelID := r.URL.Path[len("/api/v1/channels/"):]
	if channelID == "" {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Channel ID is required", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getChannel(w, r, channelID)
	case http.MethodPut:
		h.updateChannel(w, r, channelID)
	case http.MethodDelete:
		h.deleteChannel(w, r, channelID)
	default:
		api.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
	}
}

// listChannels получает список всех каналов тенанта
func (h *ChannelHandler) listChannels(w http.ResponseWriter, r *http.Request) {
	// Получаем tenant_id из query параметров или заголовков
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-ID")
	}

	if tenantID == "" {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Tenant ID is required", nil)
		return
	}

	// Получаем опциональный фильтр по типу
	channelType := r.URL.Query().Get("type")

	// Получаем каналы из сервиса
	channels, err := h.channelService.ListChannels(r.Context(), tenantID, service.ChannelTypeUnspecified)
	if err != nil {
		h.logger.Error("Failed to list channels", logger.Error(err))
		api.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to list channels", err)
		return
	}

	// Фильтрация по типу если указан
	if channelType != "" {
		filtered := make([]*service.Channel, 0)
		for _, channel := range channels {
			if string(channel.Type) == channelType {
				filtered = append(filtered, channel)
			}
		}
		channels = filtered
	}

	// Конвертируем в API response
	response := api.ChannelListResponse{
		Channels: make([]api.ChannelAPIResponse, 0, len(channels)),
		Total:    len(channels),
	}

	for _, channel := range channels {
		apiResponse := api.ChannelAPIResponse{
			ID:        channel.ID,
			TenantID:  channel.TenantID,
			Name:      channel.Name,
			Type:      string(channel.Type),
			Config:    convertStringMapToInterface(channel.Config),
			IsActive:  channel.IsActive,
			CreatedAt: parseTime(channel.CreatedAt),
			UpdatedAt: parseTime(channel.UpdatedAt),
		}
		response.Channels = append(response.Channels, apiResponse)
	}

	api.WriteSuccessResponse(w, "Channels retrieved successfully", response)
}

// createChannel создает новый канал
func (h *ChannelHandler) createChannel(w http.ResponseWriter, r *http.Request) {
	// Получаем tenant_id
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = r.URL.Query().Get("tenant_id")
	}

	if tenantID == "" {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Tenant ID is required", nil)
		return
	}

	// Парсим запрос
	var req api.CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Валидируем запрос
	validations := api.ValidateChannelRequest(&req)
	if len(validations) > 0 {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Validation failed", nil)
		// Можно добавить детали валидации в ответ
		return
	}

	// Создаем канал
	channel := &service.Channel{
		TenantID: tenantID,
		Name:     req.Name,
		Type:     convertStringToChannelType(req.Type),
		Config:   convertInterfaceToStringMap(req.Config),
		IsActive: true,
	}

	createdChannel, err := h.channelService.RegisterChannel(r.Context(), channel)
	if err != nil {
		h.logger.Error("Failed to create channel", logger.Error(err))
		api.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to create channel", err)
		return
	}

	// Конвертируем в API response
	response := api.ChannelAPIResponse{
		ID:        createdChannel.ID,
		TenantID:  createdChannel.TenantID,
		Name:      createdChannel.Name,
		Type:      string(createdChannel.Type),
		Config:    convertStringMapToInterface(createdChannel.Config),
		IsActive:  createdChannel.IsActive,
		CreatedAt: parseTime(createdChannel.CreatedAt),
		UpdatedAt: parseTime(createdChannel.UpdatedAt),
	}

	api.WriteSuccessResponse(w, "Channel created successfully", response)
}

// getChannel получает канал по ID
func (h *ChannelHandler) getChannel(w http.ResponseWriter, r *http.Request, channelID string) {
	// Получаем tenant_id
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = r.URL.Query().Get("tenant_id")
	}

	if tenantID == "" {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Tenant ID is required", nil)
		return
	}

	// Получаем все каналы и ищем нужный
	channels, err := h.channelService.ListChannels(r.Context(), tenantID, service.ChannelTypeUnspecified)
	if err != nil {
		h.logger.Error("Failed to get channels", logger.Error(err))
		api.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to get channel", err)
		return
	}

	// Ищем канал по ID
	var foundChannel *service.Channel
	for _, channel := range channels {
		if channel.ID == channelID {
			foundChannel = channel
			break
		}
	}

	if foundChannel == nil {
		api.WriteErrorResponse(w, http.StatusNotFound, "Channel not found", nil)
		return
	}

	// Конвертируем в API response
	response := api.ChannelAPIResponse{
		ID:        foundChannel.ID,
		TenantID:  foundChannel.TenantID,
		Name:      foundChannel.Name,
		Type:      string(foundChannel.Type),
		Config:    convertStringMapToInterface(foundChannel.Config),
		IsActive:  foundChannel.IsActive,
		CreatedAt: parseTime(foundChannel.CreatedAt),
		UpdatedAt: parseTime(foundChannel.UpdatedAt),
	}

	api.WriteSuccessResponse(w, "Channel retrieved successfully", response)
}

// updateChannel обновляет канал
func (h *ChannelHandler) updateChannel(w http.ResponseWriter, r *http.Request, channelID string) {
	// Получаем tenant_id
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Missing X-Tenant-ID header", nil)
		return
	}

	// Декодируем тело запроса
	var req api.CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Валидация обязательных полей
	if req.Name == "" {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Channel name is required", nil)
		return
	}

	// Конвертируем в domain модель
	channel := &service.Channel{
		ID:       channelID,
		TenantID: tenantID,
		Name:     req.Name,
		Type:     convertStringToChannelType(req.Type),
		Config:   convertInterfaceToStringMap(req.Config),
		IsActive: true, // По умолчанию активен
	}

	// Обновляем канал
	err := h.channelService.UpdateChannel(r.Context(), channel)
	if err != nil {
		h.logger.Error("Failed to update channel",
			logger.String("channel_id", channelID),
			logger.String("tenant_id", tenantID),
			logger.Error(err))
		api.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to update channel", err)
		return
	}

	h.logger.Info("Channel updated successfully",
		logger.String("channel_id", channelID),
		logger.String("tenant_id", tenantID))

	// Конвертируем в API модель
	apiType := convertChannelTypeToAPI(channel.Type)
	apiResponse := api.ChannelAPIResponse{
		ID:        channel.ID,
		TenantID:  channel.TenantID,
		Name:      channel.Name,
		Type:      apiType,
		Config:    convertStringMapToInterface(channel.Config),
		IsActive:  channel.IsActive,
		CreatedAt: parseTime(channel.CreatedAt),
		UpdatedAt: parseTime(channel.UpdatedAt),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiResponse)
}

// deleteChannel удаляет канал
func (h *ChannelHandler) deleteChannel(w http.ResponseWriter, r *http.Request, channelID string) {
	// Получаем tenant_id
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = r.URL.Query().Get("tenant_id")
	}

	if tenantID == "" {
		api.WriteErrorResponse(w, http.StatusBadRequest, "Tenant ID is required", nil)
		return
	}

	// Удаляем канал
	err := h.channelService.UnregisterChannel(r.Context(), channelID)
	if err != nil {
		h.logger.Error("Failed to delete channel", logger.Error(err))
		api.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to delete channel", err)
		return
	}

	api.WriteSuccessResponse(w, "Channel deleted successfully", nil)
}

// Вспомогательные функции

// convertStringMapToInterface конвертирует map[string]string в map[string]interface{}
func convertStringMapToInterface(config map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range config {
		result[k] = v
	}
	return result
}

// convertInterfaceToStringMap конвертирует map[string]interface{} в map[string]string
func convertInterfaceToStringMap(config map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range config {
		if str, ok := v.(string); ok {
			result[k] = str
		} else {
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// parseTime парсит время из строки
func parseTime(timeStr string) time.Time {
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t
	}
	return time.Time{}
}

// convertStringToChannelType конвертирует строку в service.ChannelType
func convertStringToChannelType(channelType string) service.ChannelType {
	switch strings.ToLower(channelType) {
	case "email":
		return service.ChannelTypeEmail
	case "telegram":
		return service.ChannelTypeTelegram
	case "slack":
		return service.ChannelTypeSlack
	case "webhook":
		return service.ChannelTypeUnspecified
	case "sms":
		return service.ChannelTypeUnspecified
	default:
		return service.ChannelTypeEmail // По умолчанию
	}
}

// convertChannelTypeToAPI конвертирует service.ChannelType в строку API
func convertChannelTypeToAPI(channelType service.ChannelType) string {
	switch channelType {
	case service.ChannelTypeEmail:
		return "email"
	case service.ChannelTypeTelegram:
		return "telegram"
	case service.ChannelTypeSlack:
		return "slack"
	case service.ChannelTypeUnspecified:
		return "webhook" // Для неуказанного типа используем webhook
	default:
		return "email" // По умолчанию
	}
}
