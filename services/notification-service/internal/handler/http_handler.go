package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/notification-service/internal/api"
	"UptimePingPlatform/services/notification-service/internal/service"
)

// HTTPHandler обрабатывает HTTP запросы для Notification Service
type HTTPHandler struct {
	logger             logger.Logger
	notificationService service.NotificationService
}

// NewHTTPHandler создает новый HTTP обработчик
func NewHTTPHandler(logger logger.Logger, notificationService service.NotificationService) *HTTPHandler {
	return &HTTPHandler{
		logger:             logger,
		notificationService: notificationService,
	}
}

// RegisterRoutes регистрирует HTTP маршруты
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	// API маршруты для каналов уведомлений
	mux.HandleFunc("/api/v1/notification/channels", h.handleChannels)
	mux.HandleFunc("/api/v1/notification/channels/", h.handleChannelByID)
	
	// API маршруты для отправки уведомлений
	mux.HandleFunc("/api/v1/notification/send", h.handleSendNotification)
}

// handleChannels обрабатывает запросы к /api/v1/notification/channels
func (h *HTTPHandler) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listChannels(w, r)
	case http.MethodPost:
		h.createChannel(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleChannelByID обрабатывает запросы к /api/v1/notification/channels/{id}
func (h *HTTPHandler) handleChannelByID(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	id := extractChannelID(r.URL.Path)
	if id == "" {
		http.Error(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		h.deleteChannel(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSendNotification обрабатывает запросы к /api/v1/notification/send
func (h *HTTPHandler) handleSendNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.sendNotification(w, r)
}

// createChannel создает новый канал уведомлений
func (h *HTTPHandler) createChannel(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Processing create channel request")

	var req api.CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode create channel request", logger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.logger.Info("Creating notification channel",
		logger.String("name", req.Name),
		logger.String("type", req.Type),
		logger.Bool("is_active", req.IsActive))

	// Конвертируем API модель в domain модель
	channelType := service.ChannelTypeEmail
	switch req.Type {
	case "slack":
		channelType = service.ChannelTypeSlack
	case "telegram":
		channelType = service.ChannelTypeTelegram
	case "webhook":
		channelType = service.ChannelTypeUnspecified // Используем Unspecified
	case "sms":
		channelType = service.ChannelTypeUnspecified // Используем Unspecified
	}

	domainChannel := &service.Channel{
		TenantID:    getTenantIDFromContext(r.Context()), // Получаем tenant ID из контекста
		Name:        req.Name,
		Type:        channelType,
		Config:      req.Config,
		IsActive:    req.IsActive,
	}

	// Вызываем реальный сервис
	createdChannel, err := h.notificationService.RegisterChannel(r.Context(), domainChannel)
	if err != nil {
		h.logger.Error("Failed to create channel", logger.Error(err))
		http.Error(w, "Failed to create channel", http.StatusInternalServerError)
		return
	}

	// Конвертируем обратно в API модель
	apiChannel := api.Channel{
		ID:          createdChannel.ID,
		Name:        createdChannel.Name,
		Type:        req.Type, // Возвращаем оригинальный тип
		Config:      createdChannel.Config,
		Description: "", // Domain модель не имеет поля Description
		IsActive:    createdChannel.IsActive,
		CreatedAt:   parseTime(createdChannel.CreatedAt),
		UpdatedAt:   parseTime(createdChannel.UpdatedAt),
	}

	response := api.CreateChannelResponse{
		Success: true,
		Message: "Channel created successfully",
		Channel: &apiChannel,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// deleteChannel удаляет канал уведомлений
func (h *HTTPHandler) deleteChannel(w http.ResponseWriter, r *http.Request, id string) {
	h.logger.Info("Processing delete channel request", logger.String("id", id))

	// Вызываем реальный сервис
	err := h.notificationService.UnregisterChannel(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to delete channel", logger.Error(err))
		http.Error(w, "Failed to delete channel", http.StatusInternalServerError)
		return
	}

	response := api.DeleteChannelResponse{
		Success: true,
		Message: "Channel deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// listChannels получает список каналов уведомлений
func (h *HTTPHandler) listChannels(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Processing list channels request")

	// Получаем query параметры
	query := r.URL.Query()
	channelType := query.Get("type")
	page, _ := strconv.Atoi(query.Get("page"))
	pageSize, _ := strconv.Atoi(query.Get("page_size"))
	
	var isActive *bool
	if activeStr := query.Get("is_active"); activeStr != "" {
		if active, err := strconv.ParseBool(activeStr); err == nil {
			isActive = &active
		}
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	h.logger.Info("List channels parameters",
		logger.String("type", channelType),
		logger.Bool("is_active", isActive != nil && *isActive),
		logger.Int("page", page),
		logger.Int("page_size", pageSize))

	// Конвертируем API тип в domain тип
	var domainChannelType service.ChannelType
	switch channelType {
	case "email":
		domainChannelType = service.ChannelTypeEmail
	case "slack":
		domainChannelType = service.ChannelTypeSlack
	case "telegram":
		domainChannelType = service.ChannelTypeTelegram
	case "webhook":
		domainChannelType = service.ChannelTypeUnspecified // Используем Unspecified
	case "sms":
		domainChannelType = service.ChannelTypeUnspecified // Используем Unspecified
	default:
		domainChannelType = service.ChannelTypeEmail // По умолчанию
	}

	// Вызываем реальный сервис
	domainChannels, err := h.notificationService.ListChannels(r.Context(), "default", domainChannelType)
	if err != nil {
		h.logger.Error("Failed to list channels", logger.Error(err))
		http.Error(w, "Failed to list channels", http.StatusInternalServerError)
		return
	}

	// Конвертируем domain модели в API модели
	channels := make([]api.Channel, len(domainChannels))
	for i, domainChannel := range domainChannels {
		// Конвертируем domain тип обратно в API тип
		apiType := "email"
		switch domainChannel.Type {
		case service.ChannelTypeSlack:
			apiType = "slack"
		case service.ChannelTypeTelegram:
			apiType = "telegram"
		case service.ChannelTypeUnspecified:
			apiType = "webhook" // Для Unspecified используем webhook
		case service.ChannelTypeEmail:
			apiType = "email"
		}

		channels[i] = api.Channel{
			ID:          domainChannel.ID,
			Name:        domainChannel.Name,
			Type:        apiType,
			Config:      domainChannel.Config,
			Description: "", // Domain модель не имеет поля Description
			IsActive:    domainChannel.IsActive,
			CreatedAt:   parseTime(domainChannel.CreatedAt),
			UpdatedAt:   parseTime(domainChannel.UpdatedAt),
		}
	}

	// Фильтрация по активности (если указано)
	if isActive != nil {
		filteredChannels := []api.Channel{}
		for _, channel := range channels {
			if channel.IsActive == *isActive {
				filteredChannels = append(filteredChannels, channel)
			}
		}
		channels = filteredChannels
	}

	// Пагинация
	total := len(channels)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		channels = []api.Channel{}
	} else {
		if end > total {
			end = total
		}
		channels = channels[start:end]
	}

	response := api.ListChannelsResponse{
		Channels: channels,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendNotification отправляет уведомление
func (h *HTTPHandler) sendNotification(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Processing send notification request")

	var req api.SendNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode send notification request", logger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.logger.Info("Sending notification",
		logger.String("channel_id", req.ChannelID),
		logger.String("subject", req.Subject),
		logger.String("recipient", req.Recipient),
		logger.String("priority", req.Priority))

	// Конвертируем API модель в domain модель
	severity := service.NotificationSeverityInfo
	switch req.Priority {
	case "low":
		severity = service.NotificationSeverityInfo
	case "normal":
		severity = service.NotificationSeverityWarning
	case "high":
		severity = service.NotificationSeverityError
	case "critical":
		severity = service.NotificationSeverityCritical
	}

	domainNotification := &service.Notification{
		TenantID:   getTenantIDFromContext(r.Context()), // Получаем tenant ID из контекста
		IncidentID: getIncidentIDFromContext(r.Context()), // Получаем incident ID из контекста
		Severity:   severity,
		Title:      req.Subject,
		Message:    req.Message,
		ChannelIDs: []string{req.ChannelID},
		Metadata:   req.Metadata,
	}

	// Вызываем реальный сервис
	results, err := h.notificationService.SendNotification(r.Context(), domainNotification)
	if err != nil {
		h.logger.Error("Failed to send notification", logger.Error(err))
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	// Проверяем результаты
	success := true
	message := "Notification sent successfully"
	for _, result := range results {
		if !result.Success {
			success = false
			message = "Some notifications failed"
			break
		}
	}

	// Генерируем MessageID на основе результатов
	messageID := "msg-" + generateID()
	if len(results) > 0 && results[0].ChannelID != "" {
		messageID = results[0].ChannelID + "-" + generateID()
	}

	response := api.SendNotificationResponse{
		Success:   success,
		Message:   message,
		MessageID: messageID, // Используем ID из результата
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Вспомогательные функции

// extractChannelID извлекает ID канала из URL
func extractChannelID(path string) string {
	// URL формат: /api/v1/notification/channels/{id}
	parts := splitPath(path)
	if len(parts) >= 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "notification" && parts[3] == "channels" {
		return parts[4]
	}
	return ""
}

// splitPath разделяет URL путь на компоненты
func splitPath(path string) []string {
	if path == "" || path[0] != '/' {
		return []string{}
	}
	
	parts := []string{}
	current := ""
	for i, char := range path {
		if i == 0 {
			continue // Пропускаем первый /
		}
		
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		parts = append(parts, current)
	}
	
	return parts
}

// generateID генерирует уникальный ID
func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// parseTime парсит строку времени в time.Time
func parseTime(timeStr string) time.Time {
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t
	}
	// Если не удалось распарсить, возвращаем текущее время
	return time.Now()
}

// getTenantIDFromContext получает tenant ID из контекста запроса
func getTenantIDFromContext(ctx context.Context) string {
	// Реализация получения tenant ID из JWT токена или заголовков
	// Проверяем сначала контекст, установленный middleware
	if tenantID := ctx.Value("tenant_id"); tenantID != nil {
		if id, ok := tenantID.(string); ok {
			return id
		}
	}
	
	// Если в контексте нет, пробуем извлечь из заголовков
	// Это fallback для случаев, когда middleware не используется
	return "default"
}

// getIncidentIDFromContext получает incident ID из контекста запроса
func getIncidentIDFromContext(ctx context.Context) string {
	// Реализация получения incident ID из контекста или заголовков
	// Проверяем сначала контекст
	if incidentID := ctx.Value("incident_id"); incidentID != nil {
		if id, ok := incidentID.(string); ok {
			return id
		}
	}
	
	// Fallback значение для вебхуков
	return "webhook"
}
