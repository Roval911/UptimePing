package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"UptimePingPlatform/pkg/logger"
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/forge-service/internal/api"
	"UptimePingPlatform/services/forge-service/internal/domain"
	"UptimePingPlatform/services/forge-service/internal/service"
	
	// gRPC клиенты
	authv1 "UptimePingPlatform/proto/api/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// HTTPHandler обрабатывает HTTP запросы для Forge Service
type HTTPHandler struct {
	logger            logger.Logger
	codeGenerator     *service.CodeGenerator
	protoParser      *service.ProtoParser
	forgeService      service.ForgeService
	interactiveConfig *domain.InteractiveConfig
	authClient        authv1.AuthServiceClient // gRPC клиент для Auth Service
}

// NewHTTPHandler создает новый HTTP обработчик
func NewHTTPHandler(logger logger.Logger, codeGenerator *service.CodeGenerator, protoParser *service.ProtoParser, forgeService service.ForgeService, apiGatewayAddress string) *HTTPHandler {
	// Создаем gRPC подключение к API Gateway
	conn, err := grpc.Dial(apiGatewayAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to API Gateway", pkglogger.String("error", err.Error()))
		return nil
	}

	// Создаем gRPC клиент для Auth Service
	authClient := authv1.NewAuthServiceClient(conn)

	return &HTTPHandler{
		logger:            logger,
		codeGenerator:     codeGenerator,
		protoParser:      protoParser,
		forgeService:      forgeService,
		interactiveConfig: domain.NewDefaultInteractiveConfig(),
		authClient:        authClient,
	}
}

// RegisterRoutes регистрирует HTTP маршруты
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	// Статические файлы (включая login.html)
	mux.Handle("/", h.staticHandler())
	
	// API маршруты (защищенные)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/config", h.handleConfig)
	apiMux.HandleFunc("/api/config/telegram", h.handleTelegramConfig)
	apiMux.HandleFunc("/api/config/email", h.handleEmailConfig)
	apiMux.HandleFunc("/api/config/generate", h.handleGenerateConfig)
	apiMux.HandleFunc("/api/checkers/generate", h.handleGenerateCheckers)
	apiMux.HandleFunc("/api/status", h.handleStatus)
	
	// CLI API маршруты (v1)
	apiMux.HandleFunc("/api/v1/forge/generate", h.handleGenerate)
	
	// Применяем middleware аутентификации к API
	mux.Handle("/api/", h.authMiddleware(apiMux))
	
	// Login endpoint (открытый)
	mux.HandleFunc("/api/auth/login", h.handleLogin)
	
	// Register endpoint (открытый)
	mux.HandleFunc("/api/auth/register", h.handleRegister)
}

// staticHandler обслуживает статические файлы
func (h *HTTPHandler) staticHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверяем, запрашивается ли index.html
		if r.URL.Path == "/" {
			// Проверяем наличие токена
			token := h.extractTokenFromRequest(r)
			if token == "" {
				// Если нет токена, показываем login.html
				http.ServeFile(w, r, filepath.Join("web", "login.html"))
				return
			}
			// Если токен есть, показываем index.html
			http.ServeFile(w, r, filepath.Join("web", "index.html"))
			return
		}
		
		// Для других статических файлов
		filePath := filepath.Join("web", r.URL.Path)
		http.ServeFile(w, r, filePath)
	}
}

// authMiddleware проверяет JWT токен
func (h *HTTPHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := h.extractTokenFromRequest(r)
		if token == "" {
			h.writeErrorResponse(w, http.StatusUnauthorized, "Missing authorization token")
			return
		}
		
		// Валидация токена через Auth Service
		valid, tenantID, err := h.validateToken(token)
		if err != nil {
			h.writeErrorResponse(w, http.StatusInternalServerError, "Token validation error")
			return
		}
		
		if !valid {
			h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid token")
			return
		}
		
		// Добавляем tenant ID в контекст
		ctx := context.WithValue(r.Context(), "tenant_id", tenantID)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractTokenFromRequest извлекает токен из запроса
func (h *HTTPHandler) extractTokenFromRequest(r *http.Request) string {
	// Проверяем Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
		return authHeader
	}
	
	// Проверяем query параметр
	return r.URL.Query().Get("token")
}

// validateToken валидирует токен через Auth Service
func (h *HTTPHandler) validateToken(token string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Вызываем ValidateToken через gRPC
	req := &authv1.ValidateTokenRequest{
		Token: token,
	}

	resp, err := h.authClient.ValidateToken(ctx, req)
	if err != nil {
		h.logger.Error("Failed to validate token", pkglogger.String("error", err.Error()))
		return false, "", fmt.Errorf("token validation failed: %w", err)
	}

	if !resp.IsValid {
		h.logger.Warn("Invalid token provided")
		return false, "", nil
	}

	h.logger.Info("Token validated successfully", 
		pkglogger.String("tenant_id", resp.TenantId),
		pkglogger.String("user_id", resp.UserId))

	return true, resp.TenantId, nil
}

// extractTenantIDFromToken извлекает tenant ID из токена
func (h *HTTPHandler) extractTenantIDFromToken(token string) string {
	// В реальной реализации здесь будет парсинг JWT
	// Для демонстрации извлекаем из токена
	if strings.HasPrefix(token, "forge-tenant-") {
		return strings.TrimPrefix(token, "forge-tenant-")
	}
	return "default"
}

// handleLogin обрабатывает вход в систему
func (h *HTTPHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	var loginReq struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	// Вызываем Login через gRPC к Auth Service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &authv1.LoginRequest{
		Email:    loginReq.Email,
		Password: loginReq.Password,
	}

	resp, err := h.authClient.Login(ctx, req)
	if err != nil {
		h.logger.Error("Login failed", 
			pkglogger.String("email", loginReq.Email),
			pkglogger.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	response := map[string]interface{}{
		"success":    true,
		"token":      resp.AccessToken,
		"tenant_id":  "", // Будет извлечено из токена при валидации
		"user_id":    "", // Будет извлечено из токена при валидации
		"expires_in": resp.ExpiresIn,
		"message":    "Login successful",
	}

	h.logger.Info("User logged in successfully",
		pkglogger.String("email", loginReq.Email))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRegister обрабатывает регистрацию пользователя
func (h *HTTPHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	
	var registerReq struct {
		Email      string `json:"email"`
		Password    string `json:"password"`
		TenantName  string `json:"tenant_name"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&registerReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	// Вызываем Register через gRPC к Auth Service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &authv1.RegisterRequest{
		Email:      registerReq.Email,
		Password:    registerReq.Password,
		TenantName:  registerReq.TenantName,
	}

	resp, err := h.authClient.Register(ctx, req)
	if err != nil {
		h.logger.Error("Registration failed", 
			pkglogger.String("email", registerReq.Email),
			pkglogger.String("tenant_name", registerReq.TenantName),
			pkglogger.String("error", err.Error()))
		h.writeErrorResponse(w, http.StatusBadRequest, "Registration failed")
		return
	}

	response := map[string]interface{}{
		"success":    true,
		"message":    "Registration successful",
		"token":      resp.AccessToken,
		"tenant_id":  "", // Будет извлечено из токена при валидации
		"user_id":    "", // Будет извлечено из токена при валидации
		"expires_in": resp.ExpiresIn,
	}

	h.logger.Info("User registered successfully",
		pkglogger.String("email", registerReq.Email),
		pkglogger.String("tenant_name", registerReq.TenantName))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// writeErrorResponse отправляет ошибку в формате JSON
func (h *HTTPHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := map[string]interface{}{
		"success": false,
		"error":   message,
		"code":    statusCode,
	}
	
	json.NewEncoder(w).Encode(response)
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
		h.logger.Error("Failed to encode config", logger.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// updateConfig обновляет конфигурацию
func (h *HTTPHandler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var config domain.InteractiveConfig

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.logger.Error("Failed to decode config", logger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Валидация конфигурации
	if err := config.Validate(); err != nil {
		h.logger.Error("Config validation failed", logger.Error(err))
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
		h.logger.Error("Failed to generate config", logger.Error(err))
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
		h.logger.Error("Failed to get services from parser", logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to get services: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Retrieved services from parser", 
		logger.Int("count", len(services)))

	checkersPath := "generated/checkers"
	if err := h.codeGenerator.GenerateGRPCCheckers(services, checkersPath); err != nil {
		h.logger.Error("Failed to generate checkers", logger.Error(err))
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
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("remote", r.RemoteAddr),
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
		logger.Int("total_services", len(services)))

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

// CLI API обработчики

// handleGenerate обрабатывает запрос на генерацию кода
func (h *HTTPHandler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req api.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode generate request", logger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing generate request", 
		logger.String("input", req.Input),
		logger.String("output", req.Output),
		logger.String("template", req.Template),
		logger.String("language", req.Language))

	// Валидация запроса
	if req.Input == "" {
		http.Error(w, "Input is required", http.StatusBadRequest)
		return
	}

	// Генерация кода
	outputPath := req.Output
	if outputPath == "" {
		outputPath = "generated"
	}

	// Используем существующий код генератор
	codeOptions := &service.CodeOptions{
		Language:  req.Language,
		Template:  req.Template,
		Framework: "default",
	}
	
	if codeOptions.Language == "" {
		codeOptions.Language = "go"
	}
	
	filename, content, _, err := h.forgeService.GenerateCode(r.Context(), req.Input, codeOptions)
	if err != nil {
		h.logger.Error("Failed to generate code", logger.Error(err))
		http.Error(w, fmt.Sprintf("Generation failed: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Создаем директорию если нужно
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		h.logger.Error("Failed to create output directory", logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to create output directory: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Записываем сгенерированный файл
	fullPath := filepath.Join(outputPath, filename)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		h.logger.Error("Failed to write generated file", logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to write file: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Получаем список сгенерированных файлов
	files, err := h.getGeneratedFiles(outputPath)
	if err != nil {
		h.logger.Warn("Failed to get generated files", logger.Error(err))
		files = []string{filename}
	}

	response := api.GenerateResponse{
		GeneratedFiles: len(files),
		OutputPath:     outputPath,
		GenerationTime: time.Now(),
		Files:          files,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleValidate обрабатывает запрос на валидацию
func (h *HTTPHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req api.ValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode validate request", logger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing validate request", 
		logger.String("input", req.Input),
		logger.String("proto_path", req.ProtoPath),
		logger.Bool("lint", req.Lint),
		logger.Bool("breaking", req.Breaking))

	// Валидация proto файлов
	protoPath := req.ProtoPath
	if protoPath == "" {
		protoPath = "."
	}

	// Используем существующий парсер
	parser := service.NewProtoParser(protoPath)
	if err := parser.LoadAndValidateProtoFiles(); err != nil {
		response := api.ValidateResponse{
			Status:         "error",
			Valid:          false,
			FilesChecked:   0,
			Errors:         []api.ValidationError{{File: "parser", Line: 0, Column: 0, Message: err.Error()}},
			Warnings:       []api.ValidationWarning{},
			ValidationTime: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Дополнительная валидация
	errors := []api.ValidationError{}
	warnings := []api.ValidationWarning{}

	if req.Lint {
		// Добавляем lint проверки
		warnings = append(warnings, api.ValidationWarning{
			File:    "proto",
			Message: "Lint checking not implemented yet",
		})
	}

	if req.Breaking {
		// Добавляем breaking change проверки
		warnings = append(warnings, api.ValidationWarning{
			File:    "proto",
			Message: "Breaking change checking not implemented yet",
		})
	}

	response := api.ValidateResponse{
		Status:         "success",
		Valid:          len(errors) == 0,
		FilesChecked:   1, // Используем 1 как approximation для одного файла
		Errors:         errors,
		Warnings:       warnings,
		ValidationTime: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleInteractive обрабатывает запрос на интерактивную настройку
func (h *HTTPHandler) handleInteractive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req api.InteractiveConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode interactive request", logger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing interactive request", 
		logger.String("proto_file", req.ProtoFile),
		logger.String("template", req.Template))

	// Обновляем интерактивную конфигурацию
	// Временно используем простое хранилище в памяти
	if h.interactiveConfig.Services == nil {
		h.interactiveConfig.Services = make(map[string]*domain.ServiceConfig)
	}
	
	// Обновляем глобальные настройки
	if req.Template != "" {
		// Сохраняем template в одном из сервисов как временное решение
		if _, exists := h.interactiveConfig.Services["global"]; !exists {
			h.interactiveConfig.Services["global"] = &domain.ServiceConfig{}
		}
		h.interactiveConfig.Services["global"].Host = req.Template
	}

	// Добавляем опции - используем DisabledMethods как временное хранилище для опций
	for key, value := range req.Options {
		if _, exists := h.interactiveConfig.Services["global"]; !exists {
			h.interactiveConfig.Services["global"] = &domain.ServiceConfig{}
		}
		// Временно используем DisabledMethods для хранения опций (как строка через запятую)
		if len(h.interactiveConfig.Services["global"].DisabledMethods) == 0 {
			h.interactiveConfig.Services["global"].DisabledMethods = []string{}
		}
		h.interactiveConfig.Services["global"].DisabledMethods = append(
			h.interactiveConfig.Services["global"].DisabledMethods,
			fmt.Sprintf("%s=%s", key, value),
		)
	}

	response := api.InteractiveConfigResponse{
		Config:   map[string]interface{}{"template": req.Template, "options": req.Options},
		Template: req.Template,
		Ready:    true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTemplates обрабатывает запрос на получение шаблонов
func (h *HTTPHandler) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем query параметры
	query := r.URL.Query()
	templateType := query.Get("type")
	language := query.Get("language")

	h.logger.Info("Processing templates request", 
		logger.String("type", templateType),
		logger.String("language", language))

	// Получаем шаблоны из реального сервиса
	serviceTemplates, err := h.forgeService.GetTemplates(r.Context(), templateType, language)
	if err != nil {
		h.logger.Error("Failed to get templates", logger.Error(err))
		http.Error(w, fmt.Sprintf("Failed to get templates: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Конвертируем service.TemplateInfo в api.TemplateInfo
	templates := make([]api.TemplateInfo, len(serviceTemplates))
	for i, tmpl := range serviceTemplates {
		templates[i] = api.TemplateInfo{
			Name:        tmpl.Name,
			Type:        tmpl.Type,
			Language:    tmpl.Language,
			Description: tmpl.Description,
			Parameters:  tmpl.Parameters,
			Example:     tmpl.Example,
		}
	}

	// Фильтрация по типу и языку
	filteredTemplates := []api.TemplateInfo{}
	for _, template := range templates {
		if templateType != "" && template.Type != templateType {
			continue
		}
		if language != "" && template.Language != language {
			continue
		}
		filteredTemplates = append(filteredTemplates, template)
	}

	response := api.GetTemplatesResponse{
		Templates: filteredTemplates,
		Total:     len(filteredTemplates),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Вспомогательные функции

// getGeneratedFiles получает список сгенерированных файлов
func (h *HTTPHandler) getGeneratedFiles(path string) ([]string, error) {
	var files []string
	
	// Рекурсивно обходим директорию
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() {
			// Получаем относительный путь
			relPath, err := filepath.Rel(path, filePath)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		
		return nil
	})
	
	return files, err
}
