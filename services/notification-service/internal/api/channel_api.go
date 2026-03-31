package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"UptimePingPlatform/services/notification-service/internal/models"
)

// ChannelAPIRequest структура для запросов к каналам
type ChannelAPIRequest struct {
	TenantID string                 `json:"tenant_id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"` // email, telegram, slack
	Config   map[string]interface{} `json:"config"`
	IsActive bool                   `json:"is_active"`
}

// ChannelAPIResponse структура для ответов о каналах
type ChannelAPIResponse struct {
	ID        string                 `json:"id"`
	TenantID  string                 `json:"tenant_id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Config    map[string]interface{} `json:"config"`
	IsActive  bool                   `json:"is_active"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// ChannelListResponse структура для списка каналов
type ChannelListResponse struct {
	Channels []ChannelAPIResponse `json:"channels"`
	Total    int                  `json:"total"`
}

// UpdateChannelRequest структура для обновления канала
type UpdateChannelRequest struct {
	Name     *string                `json:"name,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
	IsActive *bool                  `json:"is_active,omitempty"`
}

// ValidationError структура для ошибок валидации
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorResponse структура для ошибок
type ErrorResponse struct {
	Error       string            `json:"error"`
	Message     string            `json:"message"`
	Validations []ValidationError `json:"validations,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// SuccessResponse структура для успешных ответов
type SuccessResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Helper функции для API

// WriteJSONResponse пишет JSON ответ
func WriteJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("Failed to encode JSON response: %v\n", err)
	}
}

// WriteErrorResponse пишет ошибку
func WriteErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	response := ErrorResponse{
		Error:     http.StatusText(statusCode),
		Message:   message,
		Timestamp: time.Now(),
	}

	if err != nil {
		// В логах можно показать детальную ошибку, но в ответе только общее сообщение
		fmt.Printf("API error: %s - %v\n", message, err)
	}

	WriteJSONResponse(w, statusCode, response)
}

// WriteSuccessResponse пишет успешный ответ
func WriteSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	response := SuccessResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}

	WriteJSONResponse(w, http.StatusOK, response)
}

// ValidateChannelRequest валидирует запрос канала
func ValidateChannelRequest(req *CreateChannelRequest) []ValidationError {
	var validations []ValidationError

	if req.Name == "" {
		validations = append(validations, ValidationError{
			Field:   "name",
			Message: "Channel name is required",
		})
	}

	if req.Type == "" {
		validations = append(validations, ValidationError{
			Field:   "type",
			Message: "Channel type is required",
		})
	} else {
		// Проверка допустимых типов
		validTypes := map[string]bool{
			"email":    true,
			"telegram": true,
			"slack":    true,
		}

		if !validTypes[req.Type] {
			validations = append(validations, ValidationError{
				Field:   "type",
				Message: "Invalid channel type. Must be one of: email, telegram, slack",
			})
		}
	}

	if req.Config == nil {
		validations = append(validations, ValidationError{
			Field:   "config",
			Message: "Channel configuration is required",
		})
	} else {
		// Валидация конфигурации в зависимости от типа
		if req.Type == "email" {
			validations = append(validations, validateEmailConfig(req.Config)...)
		} else if req.Type == "telegram" {
			validations = append(validations, validateTelegramConfig(req.Config)...)
		}
	}

	return validations
}

// validateEmailConfig валидирует конфигурацию email
func validateEmailConfig(config map[string]interface{}) []ValidationError {
	var validations []ValidationError

	requiredFields := []string{"smtp_host", "smtp_port", "smtp_user", "smtp_password", "from_email", "to_emails"}

	for _, field := range requiredFields {
		if _, ok := config[field]; !ok || config[field] == "" {
			validations = append(validations, ValidationError{
				Field:   "config." + field,
				Message: field + " is required for email channel",
			})
		}
	}

	// Валидация порта
	if port, ok := config["smtp_port"]; ok {
		if portInt, ok := port.(float64); ok {
			if portInt < 1 || portInt > 65535 {
				validations = append(validations, ValidationError{
					Field:   "config.smtp_port",
					Message: "SMTP port must be between 1 and 65535",
				})
			}
		} else {
			validations = append(validations, ValidationError{
				Field:   "config.smtp_port",
				Message: "SMTP port must be a number",
			})
		}
	}

	return validations
}

// validateTelegramConfig валидирует конфигурацию telegram
func validateTelegramConfig(config map[string]interface{}) []ValidationError {
	var validations []ValidationError

	requiredFields := []string{"bot_token", "chat_id"}

	for _, field := range requiredFields {
		if _, ok := config[field]; !ok || config[field] == "" {
			validations = append(validations, ValidationError{
				Field:   "config." + field,
				Message: field + " is required for telegram channel",
			})
		}
	}

	return validations
}

// ConvertModelToResponse конвертирует модель в API response
func ConvertModelToResponse(model *models.ChannelConfig) ChannelAPIResponse {
	return ChannelAPIResponse{
		ID:        model.ID,
		TenantID:  model.TenantID,
		Name:      model.Name,
		Type:      model.Type,
		Config:    model.Config,
		IsActive:  model.IsActive,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
