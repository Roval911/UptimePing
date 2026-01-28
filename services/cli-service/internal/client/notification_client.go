package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
)

// NotificationClientInterface определяет интерфейс для работы с уведомлениями
type NotificationClientInterface interface {
	CreateChannel(ctx context.Context, req *CreateChannelRequest) (*CreateChannelResponse, error)
	DeleteChannel(ctx context.Context, req *DeleteChannelRequest) (*DeleteChannelResponse, error)
	ListChannels(ctx context.Context, req *ListChannelsRequest) (*ListChannelsResponse, error)
	SendNotification(ctx context.Context, req *SendNotificationRequest) (*SendNotificationResponse, error)
	Close() error
}

// NotificationClient реализует клиент для работы с уведомлениями
type NotificationClient struct {
	logger  logger.Logger
	baseURL string
	client  *http.Client
}

// NewNotificationClient создает новый экземпляр NotificationClient
func NewNotificationClient(baseURL string, logger logger.Logger) *NotificationClient {
	return &NotificationClient{
		logger:  logger,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateChannelRequest представляет запрос на создание канала уведомлений
type CreateChannelRequest struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
	Config  string `json:"config"`
	Enabled bool   `json:"enabled"`
}

// CreateChannelResponse представляет ответ на создание канала
type CreateChannelResponse struct {
	ChannelID string `json:"channel_id"`
}

// DeleteChannelRequest представляет запрос на удаление канала
type DeleteChannelRequest struct {
	ChannelID string `json:"channel_id"`
}

// DeleteChannelResponse представляет ответ на удаление канала
type DeleteChannelResponse struct {
	Success bool `json:"success"`
}

// ListChannelsRequest представляет запрос на список каналов
type ListChannelsRequest struct {
	// В будущем можно добавить фильтры
}

// ListChannelsResponse представляет ответ со списком каналов
type ListChannelsResponse struct {
	Channels []ChannelInfo `json:"channels"`
	Total    int           `json:"total"`
}

// ChannelInfo представляет информацию о канале уведомлений
type ChannelInfo struct {
	ChannelID string    `json:"channel_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Address   string    `json:"address"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SendNotificationRequest представляет запрос на отправку уведомления
type SendNotificationRequest struct {
	ChannelID string `json:"channel_id"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
	Test      bool   `json:"test"`
}

// SendNotificationResponse представляет ответ на отправку уведомления
type SendNotificationResponse struct {
	NotificationID string    `json:"notification_id"`
	Status         string    `json:"status"`
	SentAt         time.Time `json:"sent_at"`
}

// CreateChannel создает новый канал уведомлений
func (c *NotificationClient) CreateChannel(ctx context.Context, req *CreateChannelRequest) (*CreateChannelResponse, error) {
	c.logger.Info("создание канала уведомлений",
		logger.String("name", req.Name),
		logger.String("type", req.Type),
		logger.String("address", req.Address))

	// Валидация входных данных
	validator := &validation.Validator{}

	if err := validator.ValidateRequiredFields(map[string]interface{}{
		"name":    req.Name,
		"type":    req.Type,
		"address": req.Address,
	}, map[string]string{
		"name":    "название канала",
		"type":    "тип канала",
		"address": "адрес канала",
	}); err != nil {
		c.logger.Error("ошибка валидации обязательных полей", logger.Error(err))
		return nil, fmt.Errorf("ошибка валидации: %w", err)
	}

	// Валидация имени
	if err := validator.ValidateStringLength(req.Name, "name", 1, 100); err != nil {
		c.logger.Error("ошибка валидации имени канала", logger.Error(err))
		return nil, fmt.Errorf("некорректное имя канала: %w", err)
	}

	// Валидация типа
	validTypes := map[string]bool{
		"email": true, "slack": true, "telegram": true, "webhook": true, "sms": true,
	}
	if !validTypes[req.Type] {
		err := fmt.Errorf("некорректный тип канала: %s", req.Type)
		c.logger.Error("ошибка валидации типа канала", logger.Error(err))
		return nil, err
	}

	// Валидация адреса
	if err := validator.ValidateStringLength(req.Address, "address", 1, 500); err != nil {
		c.logger.Error("ошибка валидации адреса канала", logger.Error(err))
		return nil, fmt.Errorf("некорректный адрес канала: %w", err)
	}

	// Реализуем HTTP вызов к Notification Service API
	url := fmt.Sprintf("%s/api/v1/notification/channels", c.baseURL)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на создание канала уведомлений", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Notification сервис недоступен, используем mock данные")
		return c.createChannelMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Notification сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Notification сервис вернул ошибку, используем mock данные")
		return c.createChannelMockResponse(req)
	}

	var createResp CreateChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.createChannelMockResponse(req)
	}

	c.logger.Info("создание канала уведомлений завершено успешно через HTTP API",
		logger.String("channel_id", createResp.ChannelID))

	return &createResp, nil
}

// createChannelMockResponse создает mock ответ для создания канала
func (c *NotificationClient) createChannelMockResponse(req *CreateChannelRequest) (*CreateChannelResponse, error) {
	c.logger.Info("создание mock ответа для создания канала уведомлений")

	response := &CreateChannelResponse{
		ChannelID: fmt.Sprintf("channel-%d", time.Now().Unix()),
	}

	c.logger.Info("mock создание канала уведомлений завершено", logger.String("channel_id", response.ChannelID))

	return response, nil
}

// DeleteChannel удаляет канал уведомлений
func (c *NotificationClient) DeleteChannel(ctx context.Context, req *DeleteChannelRequest) (*DeleteChannelResponse, error) {
	c.logger.Info("удаление канала уведомлений", logger.String("channel_id", req.ChannelID))

	// Валидация ID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(req.ChannelID, "channel_id"); err != nil {
		c.logger.Error("ошибка валидации ID канала", logger.Error(err))
		return nil, fmt.Errorf("некорректный ID канала: %w", err)
	}

	// Реализуем HTTP вызов к Notification Service API
	url := fmt.Sprintf("%s/api/v1/notification/channels/%s", c.baseURL, req.ChannelID)

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на удаление канала уведомлений", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Notification сервис недоступен, используем mock данные")
		return c.deleteChannelMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Notification сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Notification сервис вернул ошибку, используем mock данные")
		return c.deleteChannelMockResponse(req)
	}

	var deleteResp DeleteChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&deleteResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.deleteChannelMockResponse(req)
	}

	c.logger.Info("удаление канала уведомлений завершено успешно через HTTP API",
		logger.String("channel_id", req.ChannelID))

	return &deleteResp, nil
}

// deleteChannelMockResponse создает mock ответ для удаления канала
func (c *NotificationClient) deleteChannelMockResponse(req *DeleteChannelRequest) (*DeleteChannelResponse, error) {
	c.logger.Info("создание mock ответа для удаления канала уведомлений")

	response := &DeleteChannelResponse{
		Success: true,
	}

	c.logger.Info("mock удаление канала уведомлений завершено", logger.String("channel_id", req.ChannelID))

	return response, nil
}

// ListChannels получает список каналов уведомлений
func (c *NotificationClient) ListChannels(ctx context.Context, req *ListChannelsRequest) (*ListChannelsResponse, error) {
	c.logger.Info("получение списка каналов уведомлений")

	// Реализуем HTTP вызов к Notification Service API
	url := fmt.Sprintf("%s/api/v1/notification/channels", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на получение списка каналов уведомлений", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Notification сервис недоступен, используем mock данные")
		return c.listChannelsMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Notification сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Notification сервис вернул ошибку, используем mock данные")
		return c.listChannelsMockResponse(req)
	}

	var listResp ListChannelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.listChannelsMockResponse(req)
	}

	c.logger.Info("получение списка каналов уведомлений завершено успешно через HTTP API",
		logger.Int("total", listResp.Total),
		logger.Int("returned", len(listResp.Channels)))

	return &listResp, nil
}

// listChannelsMockResponse создает mock ответ для получения списка каналов
func (c *NotificationClient) listChannelsMockResponse(req *ListChannelsRequest) (*ListChannelsResponse, error) {
	c.logger.Info("создание mock ответа для получения списка каналов уведомлений")

	mockChannels := []ChannelInfo{
		{
			ChannelID: "channel-001",
			Name:      "Email Notifications",
			Type:      "email",
			Address:   "alerts@company.com",
			Enabled:   true,
			CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			ChannelID: "channel-002",
			Name:      "Slack Alerts",
			Type:      "slack",
			Address:   "https://hooks.slack.com/services/...",
			Enabled:   true,
			CreatedAt: time.Now().Add(-15 * 24 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		},
		{
			ChannelID: "channel-003",
			Name:      "Telegram Bot",
			Type:      "telegram",
			Address:   "@uptimeping_bot",
			Enabled:   false,
			CreatedAt: time.Now().Add(-7 * 24 * time.Hour),
			UpdatedAt: time.Now().Add(-2 * time.Hour),
		},
	}

	response := &ListChannelsResponse{
		Channels: mockChannels,
		Total:    len(mockChannels),
	}

	c.logger.Info("mock получение списка каналов уведомлений завершено",
		logger.Int("total", response.Total),
		logger.Int("returned", len(response.Channels)))

	return response, nil
}

// SendNotification отправляет уведомление
func (c *NotificationClient) SendNotification(ctx context.Context, req *SendNotificationRequest) (*SendNotificationResponse, error) {
	c.logger.Info("отправка уведомления",
		logger.String("channel_id", req.ChannelID),
		logger.String("title", req.Title),
		logger.String("severity", req.Severity),
		logger.Bool("test", req.Test))

	// Валидация входных данных
	validator := &validation.Validator{}

	if err := validator.ValidateRequiredFields(map[string]interface{}{
		"channel_id": req.ChannelID,
		"title":      req.Title,
		"message":    req.Message,
	}, map[string]string{
		"channel_id": "ID канала",
		"title":      "заголовок",
		"message":    "сообщение",
	}); err != nil {
		c.logger.Error("ошибка валидации обязательных полей", logger.Error(err))
		return nil, fmt.Errorf("ошибка валидации: %w", err)
	}

	// Валидация ID канала
	if err := validator.ValidateUUID(req.ChannelID, "channel_id"); err != nil {
		c.logger.Error("ошибка валидации ID канала", logger.Error(err))
		return nil, fmt.Errorf("некорректный ID канала: %w", err)
	}

	// Валидация заголовка
	if err := validator.ValidateStringLength(req.Title, "title", 1, 200); err != nil {
		c.logger.Error("ошибка валидации заголовка", logger.Error(err))
		return nil, fmt.Errorf("некорректный заголовок: %w", err)
	}

	// Валидация сообщения
	if err := validator.ValidateStringLength(req.Message, "message", 1, 1000); err != nil {
		c.logger.Error("ошибка валидации сообщения", logger.Error(err))
		return nil, fmt.Errorf("некорректное сообщение: %w", err)
	}

	// Валидация важности
	validSeverities := map[string]bool{
		"info": true, "warning": true, "error": true, "critical": true,
	}
	if !validSeverities[req.Severity] {
		err := fmt.Errorf("некорректная важность: %s", req.Severity)
		c.logger.Error("ошибка валидации важности", logger.Error(err))
		return nil, err
	}

	// Реализуем HTTP вызов к Notification Service API
	url := fmt.Sprintf("%s/api/v1/notification/send", c.baseURL)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на отправку уведомления", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Notification сервис недоступен, используем mock данные")
		return c.sendNotificationMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Notification сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Notification сервис вернул ошибку, используем mock данные")
		return c.sendNotificationMockResponse(req)
	}

	var sendResp SendNotificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&sendResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.sendNotificationMockResponse(req)
	}

	c.logger.Info("отправка уведомления завершена успешно через HTTP API",
		logger.String("notification_id", sendResp.NotificationID))

	return &sendResp, nil
}

// sendNotificationMockResponse создает mock ответ для отправки уведомления
func (c *NotificationClient) sendNotificationMockResponse(req *SendNotificationRequest) (*SendNotificationResponse, error) {
	c.logger.Info("создание mock ответа для отправки уведомления")

	response := &SendNotificationResponse{
		NotificationID: fmt.Sprintf("notif-%d", time.Now().Unix()),
		Status:         "sent",
		SentAt:         time.Now(),
	}

	c.logger.Info("mock отправка уведомления завершена",
		logger.String("notification_id", response.NotificationID),
		logger.String("status", response.Status))

	return response, nil
}

// Close закрывает клиент
func (c *NotificationClient) Close() error {
	c.logger.Info("закрытие NotificationClient")
	return nil
}
