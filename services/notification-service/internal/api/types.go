package api

import "time"

// CreateChannelRequest представляет запрос на создание канала уведомлений
type CreateChannelRequest struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`        // email, slack, telegram, webhook, sms
	Config      map[string]string `json:"config"`
	Description string            `json:"description"`
	IsActive    bool              `json:"is_active"`
}

// Channel представляет канал уведомлений
type Channel struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Config      map[string]string `json:"config"`
	Description string            `json:"description"`
	IsActive    bool              `json:"is_active"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CreateChannelResponse представляет ответ на создание канала
type CreateChannelResponse struct {
	Success bool    `json:"success"`
	Message string  `json:"message"`
	Channel *Channel `json:"channel"`
}

// DeleteChannelRequest представляет запрос на удаление канала
type DeleteChannelRequest struct {
	ID string `json:"id"`
}

// DeleteChannelResponse представляет ответ на удаление канала
type DeleteChannelResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ListChannelsRequest представляет запрос на получение списка каналов
type ListChannelsRequest struct {
	Type     string `json:"type"`     // фильтр по типу
	IsActive *bool  `json:"is_active"` // фильтр по активности
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// ListChannelsResponse представляет ответ со списком каналов
type ListChannelsResponse struct {
	Channels []Channel `json:"channels"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// SendNotificationRequest представляет запрос на отправку уведомления
type SendNotificationRequest struct {
	ChannelID string            `json:"channel_id"`
	Subject   string            `json:"subject"`
	Message   string            `json:"message"`
	Recipient string            `json:"recipient"`
	Priority  string            `json:"priority"` // low, normal, high, critical
	Metadata  map[string]string `json:"metadata"`
}

// SendNotificationResponse представляет ответ на отправку уведомления
type SendNotificationResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	MessageID string    `json:"message_id"`
	Timestamp time.Time `json:"timestamp"`
}
