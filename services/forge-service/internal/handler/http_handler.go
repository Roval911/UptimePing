package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/forge-service/internal/domain"
	"UptimePingPlatform/services/forge-service/internal/service"
)

// HTTPHandler обрабатывает HTTP запросы для Forge Service
type HTTPHandler struct {
	logger            pkglogger.Logger
	codeGenerator     *service.CodeGenerator
	protoParser      *service.ProtoParser
	interactiveConfig *domain.InteractiveConfig
}

// NewHTTPHandler создает новый HTTP обработчик
func NewHTTPHandler(logger pkglogger.Logger, codeGenerator *service.CodeGenerator, protoParser *service.ProtoParser) *HTTPHandler {
	return &HTTPHandler{
		logger:            logger,
		codeGenerator:     codeGenerator,
		protoParser:      protoParser,
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
		"status":  "success",
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
		"status":  "success",
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
		"status":  "success",
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

	// Получаем список сервисов от парсера
	services, err := h.getServicesFromParser()
	if err != nil {
		h.logger.Error("Failed to get services from parser", pkglogger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to get services: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Retrieved services from parser", 
		pkglogger.Int("count", len(services)))

	checkersPath := "generated/checkers"
	if err := h.codeGenerator.GenerateGRPCCheckers(services, checkersPath); err != nil {
		h.logger.Error("Failed to generate checkers", pkglogger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to generate checkers: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"message":  "gRPC checkers generated successfully",
		"path":     checkersPath,
		"services": len(services),
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

// getServicesFromParser получает список сервисов от парсера и конвертирует их в domain.Service
func (h *HTTPHandler) getServicesFromParser() ([]domain.Service, error) {
	if h.protoParser == nil {
		return nil, fmt.Errorf("proto parser is not initialized")
	}

	// Получаем сервисы от парсера
	serviceInfos := h.protoParser.GetServices()
	services := make([]domain.Service, 0, len(serviceInfos))

	for _, serviceInfo := range serviceInfos {
		// Конвертируем ServiceInfo в domain.Service
		service := domain.Service{
			Name:    serviceInfo.Name,
			Package: serviceInfo.Package,
			Host:    h.getServiceHost(serviceInfo.Name),
			Port:    h.getServicePort(serviceInfo.Name),
			Methods: make([]domain.Method, 0, len(serviceInfo.Methods)),
		}

		// Конвертируем методы
		for _, methodInfo := range serviceInfo.Methods {
			method := domain.Method{
				Name:    methodInfo.Name,
				Timeout: h.getMethodTimeout(serviceInfo.Name, methodInfo.Name),
				Enabled: h.isMethodEnabled(serviceInfo.Name, methodInfo.Name),
			}
			service.Methods = append(service.Methods, method)
		}

		services = append(services, service)
	}

	h.logger.Info("Converted services from parser", 
		pkglogger.Int("total_services", len(services)))

	return services, nil
}

// getServiceHost получает хост для сервиса из конфигурации
func (h *HTTPHandler) getServiceHost(serviceName string) string {
	// Ищем конфигурацию сервиса в interactiveConfig
	if h.interactiveConfig != nil && h.interactiveConfig.Services != nil {
		if serviceConfig, exists := h.interactiveConfig.Services[serviceName]; exists {
			if serviceConfig.Host != "" {
				return serviceConfig.Host
			}
		}
	}
	
	// Значение по умолчанию
	return "localhost"
}

// getServicePort получает порт для сервиса из конфигурации
func (h *HTTPHandler) getServicePort(serviceName string) int {
	// Ищем конфигурацию сервиса в interactiveConfig
	if h.interactiveConfig != nil && h.interactiveConfig.Services != nil {
		if serviceConfig, exists := h.interactiveConfig.Services[serviceName]; exists {
			if serviceConfig.Port > 0 {
				return serviceConfig.Port
			}
		}
	}
	
	// Значение по умолчанию для gRPC
	return 50051
}

// getMethodTimeout получает таймаут для метода из конфигурации
func (h *HTTPHandler) getMethodTimeout(serviceName, methodName string) string {
	// Ищем конфигурацию сервиса в interactiveConfig
	if h.interactiveConfig != nil && h.interactiveConfig.Services != nil {
		if serviceConfig, exists := h.interactiveConfig.Services[serviceName]; exists {
			if serviceConfig.DefaultTimeout != "" {
				return serviceConfig.DefaultTimeout
			}
		}
	}
	
	// Значение по умолчанию
	return "30s"
}

// isMethodEnabled проверяет включен ли метод из конфигурации
func (h *HTTPHandler) isMethodEnabled(serviceName, methodName string) bool {
	// Ищем конфигурацию сервиса в interactiveConfig
	if h.interactiveConfig != nil && h.interactiveConfig.Services != nil {
		if serviceConfig, exists := h.interactiveConfig.Services[serviceName]; exists {
			// Если есть список отключенных методов, проверяем
			if len(serviceConfig.DisabledMethods) > 0 {
				for _, disabledMethod := range serviceConfig.DisabledMethods {
					if disabledMethod == methodName {
						return false
					}
				}
			}
			// Если есть список включенных методов, проверяем
			if len(serviceConfig.EnabledMethods) > 0 {
				for _, enabledMethod := range serviceConfig.EnabledMethods {
					if enabledMethod == methodName {
						return true
					}
				}
				return false // Метод не в списке включенных
			}
		}
	}
	
	// По умолчанию все методы включены
	return true
}
