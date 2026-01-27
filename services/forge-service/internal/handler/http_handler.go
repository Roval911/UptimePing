package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"UptimePingPlatform/services/forge-service/internal/domain"
	"UptimePingPlatform/services/forge-service/internal/service"
	pkglogger "UptimePingPlatform/pkg/logger"
)

// HTTPHandler обрабатывает HTTP запросы для Forge Service
type HTTPHandler struct {
	logger         pkglogger.Logger
	codeGenerator  *service.CodeGenerator
	interactiveConfig *domain.InteractiveConfig
}

// NewHTTPHandler создает новый HTTP обработчик
func NewHTTPHandler(logger pkglogger.Logger, codeGenerator *service.CodeGenerator) *HTTPHandler {
	return &HTTPHandler{
		logger:         logger,
		codeGenerator:  codeGenerator,
		interactiveConfig: domain.NewDefaultInteractiveConfig(),
	}
}

// RegisterRoutes регистрирует HTTP маршруты
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	// API маршруты
	mux.HandleFunc("/api/config", h.handleConfig)
	mux.HandleFunc("/api/config/telegram", h.handleTelegramConfig)
	mux.HandleFunc("/api/config/email", h.handleEmailConfig)
	mux.HandleFunc("/api/config/generate", h.handleGenerateConfig)
	mux.HandleFunc("/api/checkers/generate", h.handleGenerateCheckers)
	mux.HandleFunc("/api/status", h.handleStatus)
}

// handleConfig обрабатывает получение и обновление конфигурации
func (h *HTTPHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r)
	case http.MethodPost:
		h.updateConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getConfig возвращает текущую конфигурацию
func (h *HTTPHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if err := json.NewEncoder(w).Encode(h.interactiveConfig); err != nil {
		h.logger.Error("Failed to encode config", pkglogger.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// updateConfig обновляет конфигурацию
func (h *HTTPHandler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var config domain.InteractiveConfig
	
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.logger.Error("Failed to decode config", pkglogger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Валидация конфигурации
	if err := config.Validate(); err != nil {
		h.logger.Error("Config validation failed", pkglogger.Error(err))
		http.Error(w, fmt.Sprintf("Validation failed: %s", err.Error()), http.StatusBadRequest)
		return
	}

	h.interactiveConfig = &config
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"message": "Configuration updated successfully",
	})
}

// handleTelegramConfig обрабатывает настройку Telegram
func (h *HTTPHandler) handleTelegramConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getTelegramConfig(w, r)
	case http.MethodPost:
		h.updateTelegramConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getTelegramConfig возвращает конфигурацию Telegram
func (h *HTTPHandler) getTelegramConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.interactiveConfig.Telegram)
}

// updateTelegramConfig обновляет конфигурацию Telegram
func (h *HTTPHandler) updateTelegramConfig(w http.ResponseWriter, r *http.Request) {
	var telegramConfig domain.TelegramConfig
	
	if err := json.NewDecoder(r.Body).Decode(&telegramConfig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.interactiveConfig.Telegram = telegramConfig
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"message": "Telegram configuration updated successfully",
	})
}

// handleEmailConfig обрабатывает настройку Email
func (h *HTTPHandler) handleEmailConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getEmailConfig(w, r)
	case http.MethodPost:
		h.updateEmailConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getEmailConfig возвращает конфигурацию Email
func (h *HTTPHandler) getEmailConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.interactiveConfig.Email)
}

// updateEmailConfig обновляет конфигурацию Email
func (h *HTTPHandler) updateEmailConfig(w http.ResponseWriter, r *http.Request) {
	var emailConfig domain.EmailConfig
	
	if err := json.NewDecoder(r.Body).Decode(&emailConfig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.interactiveConfig.Email = emailConfig
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"message": "Email configuration updated successfully",
	})
}

// handleGenerateConfig генерирует конфигурационные файлы
func (h *HTTPHandler) handleGenerateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.codeGenerator.GenerateInteractiveConfig(h.interactiveConfig); err != nil {
		h.logger.Error("Failed to generate config", pkglogger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to generate config: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Configuration files generated successfully",
	})
}

// handleGenerateCheckers генерирует gRPC checker'ы
func (h *HTTPHandler) handleGenerateCheckers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Здесь нужно получить список сервисов от парсера
	// Для упрощения примера, используем пустой список
	services := []domain.Service{}
	
	checkersPath := "generated/checkers"
	if err := h.codeGenerator.GenerateGRPCCheckers(services, checkersPath); err != nil {
		h.logger.Error("Failed to generate checkers", pkglogger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to generate checkers: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "gRPC checkers generated successfully",
		"path":    checkersPath,
	})
}

// handleStatus возвращает статус сервиса
func (h *HTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"service": "forge-service",
		"version": "1.0.0",
		"status":  "running",
		"config": map[string]interface{}{
			"telegram_enabled": h.interactiveConfig.Telegram.Enabled,
			"email_enabled":    h.interactiveConfig.Email.Enabled,
			"environment":      h.interactiveConfig.Environment,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Middleware для логирования запросов
func (h *HTTPHandler) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info("HTTP request",
			pkglogger.String("method", r.Method),
			pkglogger.String("path", r.URL.Path),
			pkglogger.String("remote", r.RemoteAddr),
		)
		
		next.ServeHTTP(w, r)
	})
}

// Middleware для CORS
func (h *HTTPHandler) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
