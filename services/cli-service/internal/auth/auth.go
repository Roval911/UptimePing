package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/cli-service/internal/client"
	"UptimePingPlatform/services/cli-service/internal/config"
	"UptimePingPlatform/services/cli-service/internal/store"
)

// AuthManager управляет аутентификацией
type AuthManager struct {
	config     *config.Config
	tokenStore *store.TokenStore
	logger     logger.Logger
	validator  *validation.Validator
	metrics    *metrics.Metrics
	authClient *client.AuthGRPCClient
	useGRPC    bool
}

// NewAuthManager создает новый менеджер аутентификации
func NewAuthManager(cfg *config.Config) (*AuthManager, error) {
	// Создаем логгер
	log, err := logger.NewLogger("dev", "info", "cli-service", false)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal, "ошибка создания логгера")
	}

	// Создаем метрики
	metrics := metrics.NewMetrics("cli-service")

	tokenStore, err := store.NewTokenStore()
	if err != nil {
		log.Error("ошибка создания хранилища токенов", logger.Error(err))
		return nil, errors.Wrap(err, errors.ErrInternal, "ошибка создания хранилища токенов")
	}

	// Определяем, использовать ли gRPC
	useGRPC := cfg.GRPC.UseGRPC
	var authClient *client.AuthGRPCClient

	if useGRPC {
		// Создаем gRPC клиент для Auth Service
		authClient, err = client.NewAuthGRPCClient(cfg.GRPC.AuthAddress, log)
		if err != nil {
			log.Error("ошибка создания gRPC клиента для Auth Service", logger.Error(err))
			return nil, errors.Wrap(err, errors.ErrInternal, "ошибка создания gRPC клиента для Auth Service")
		}
		log.Info("gRPC клиент для Auth Service создан", 
			logger.String("address", cfg.GRPC.AuthAddress))
	} else {
		log.Info("используется mock режим для Auth Service")
	}

	log.Info("AuthManager создан успешно", 
		logger.String("api_url", cfg.API.BaseURL),
		logger.Int("token_expiry", cfg.Auth.TokenExpiry),
		logger.Bool("use_grpc", useGRPC))

	return &AuthManager{
		config:     cfg,
		tokenStore: tokenStore,
		logger:     log,
		validator:  &validation.Validator{},
		metrics:    metrics,
		authClient: authClient,
		useGRPC:    useGRPC,
	}, nil
}

// Close закрывает соединения
func (am *AuthManager) Close() error {
	am.logger.Info("закрытие AuthManager")
	
	// Закрываем gRPC клиент если используется
	if am.authClient != nil {
		if err := am.authClient.Close(); err != nil {
			am.logger.Error("ошибка закрытия gRPC клиента", logger.Error(err))
			return err
		}
	}
	
	return nil
}

// LoginInput представляет ввод для логина
type LoginInput struct {
	Email    string
	Password string
}

// GetLoginInput получает ввод для логина интерактивно
func GetLoginInput() (*LoginInput, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrValidation, "ошибка чтения email")
	}
	email = strings.TrimSpace(email)

	if email == "" {
		return nil, errors.New(errors.ErrValidation, "email не может быть пустым")
	}

	// Валидация email с использованием pkg/validation
	validator := &validation.Validator{}
	if err := validator.ValidateStringLength("email", email, 5, 100); err != nil {
		return nil, errors.Wrap(err, errors.ErrValidation, "некорректная длина email")
	}

	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return nil, errors.New(errors.ErrValidation, "некорректный формат email")
	}

	fmt.Print("Пароль: ")
	password, err := reader.ReadString('\n')
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrValidation, "ошибка чтения пароля")
	}
	password = strings.TrimSpace(password)

	if password == "" {
		return nil, errors.New(errors.ErrValidation, "пароль не может быть пустым")
	}

	// Валидация пароля
	if err := validator.ValidateStringLength("password", password, 8, 128); err != nil {
		return nil, errors.Wrap(err, errors.ErrValidation, "пароль должен содержать от 8 до 128 символов")
	}

	return &LoginInput{
		Email:    email,
		Password: password,
	}, nil
}

// Login выполняет вход пользователя
func (am *AuthManager) Login(ctx context.Context, input *LoginInput) error {
	// Записываем метрику начала операции
	am.metrics.RequestCount.WithLabelValues("login", "start", "").Inc()
	start := time.Now()

	am.logger.Info("попытка входа пользователя", 
		logger.String("email", input.Email))

	// Валидация входных данных
	if err := am.validator.ValidateRequiredFields(map[string]interface{}{
		"email":    input.Email,
		"password": input.Password,
	}, map[string]string{}); err != nil {
		am.metrics.ErrorsCount.WithLabelValues("login", "validation_error", "").Inc()
		am.metrics.RequestDuration.WithLabelValues("login", "validation_error").Observe(time.Since(start).Seconds())
		am.logger.Error("ошибка валидации данных входа", logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "некорректные данные входа")
	}

	// Используем gRPC если доступно
	if am.useGRPC && am.authClient != nil {
		// Вызываем Auth Service API через gRPC
		req := &client.LoginRequest{
			Email:    input.Email,
			Password: input.Password,
		}

		resp, err := am.authClient.Login(ctx, req)
		if err != nil {
			am.metrics.ErrorsCount.WithLabelValues("login", "grpc_error", "").Inc()
			am.metrics.RequestDuration.WithLabelValues("login", "grpc_error").Observe(time.Since(start).Seconds())
			am.logger.Error("ошибка входа через gRPC", logger.Error(err), logger.String("email", input.Email))
			return errors.Wrap(err, errors.ErrUnauthorized, "ошибка входа через gRPC")
		}

		if !resp.Success {
			am.metrics.ErrorsCount.WithLabelValues("login", "auth_failed", "").Inc()
			am.metrics.RequestDuration.WithLabelValues("login", "auth_failed").Observe(time.Since(start).Seconds())
			am.logger.Warn("неудачная попытка входа через gRPC", logger.String("message", "неудачная аутентификация"), logger.String("email", input.Email))
			return errors.New(errors.ErrUnauthorized, "неудачная аутентификация")
		}

		// Рассчитываем время истечения токена
		expiresAt := time.Now().Add(time.Duration(am.config.Auth.TokenExpiry) * time.Second)

		// Создаем информацию о токенах
		tokenInfo := &store.TokenInfo{
			AccessToken:  resp.AccessToken,
			RefreshToken: resp.RefreshToken,
			TokenType:   resp.TokenType,
			ExpiresAt:    expiresAt,
			UserID:      resp.User.ID,
			Email:       resp.User.Email,
			TenantID:    resp.User.TenantID,
			TenantName:  resp.User.TenantName,
		}

		// Сохраняем токены
		if err := am.tokenStore.SaveTokens(tokenInfo); err != nil {
			am.metrics.ErrorsCount.WithLabelValues("login", "token_storage_error", "").Inc()
			am.metrics.RequestDuration.WithLabelValues("login", "token_storage_error").Observe(time.Since(start).Seconds())
			am.logger.Error("ошибка сохранения токенов", logger.Error(err))
			return errors.Wrap(err, errors.ErrInternal, "ошибка сохранения токенов")
		}

		// Обновляем конфигурацию
		am.config.SetCurrentTenant(resp.User.TenantID)
		if err := am.config.Save(); err != nil {
			am.metrics.ErrorsCount.WithLabelValues("login", "config_save_error", "").Inc()
			am.metrics.RequestDuration.WithLabelValues("login", "config_save_error").Observe(time.Since(start).Seconds())
			am.logger.Error("ошибка сохранения конфигурации", logger.Error(err))
			return errors.Wrap(err, errors.ErrInternal, "ошибка сохранения конфигурации")
		}

		// Записываем успешную метрику
		am.metrics.RequestCount.WithLabelValues("login", "success", "").Inc()
		am.metrics.RequestDuration.WithLabelValues("login", "success").Observe(time.Since(start).Seconds())

		am.logger.Info("вход выполнен успешно через gRPC", 
			logger.String("email", resp.User.Email),
			logger.String("tenant", resp.User.TenantName),
			logger.String("expires_at", expiresAt.Format(time.RFC3339)))

		fmt.Printf("✅ Вход выполнен успешно!\n")
		fmt.Printf("👤 Пользователь: %s\n", resp.User.Email)
		fmt.Printf("🏢 Тенант: %s\n", resp.User.TenantName)
		fmt.Printf("⏰ Токен истекает: %s\n", expiresAt.Format("2006-01-02 15:04:05"))

		return nil
	}

	// Mock успешного ответа для демонстрации
	mockResp := struct {
		Success      bool   `json:"success"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		User         struct {
			ID         string `json:"id"`
			Email      string `json:"email"`
			TenantId   string `json:"tenant_id"`
			TenantName string `json:"tenant_name"`
		} `json:"user"`
	}{
		Success:      true,
		AccessToken:  "mock-access-token-" + input.Email,
		RefreshToken: "mock-refresh-token-" + input.Email,
		TokenType:    "Bearer",
		User: struct {
			ID         string `json:"id"`
			Email      string `json:"email"`
			TenantId   string `json:"tenant_id"`
			TenantName string `json:"tenant_name"`
		}{
			ID:         "user-123",
			Email:      input.Email,
			TenantId:   "tenant-456",
			TenantName: "Demo Tenant",
		},
	}

	// Рассчитываем время истечения токена
	expiresAt := time.Now().Add(time.Duration(am.config.Auth.TokenExpiry) * time.Second)

	// Создаем информацию о токенах
	tokenInfo := &store.TokenInfo{
		AccessToken:  mockResp.AccessToken,
		RefreshToken: mockResp.RefreshToken,
		TokenType:   mockResp.TokenType,
		ExpiresAt:    expiresAt,
		UserID:      mockResp.User.ID,
		Email:       mockResp.User.Email,
		TenantID:    mockResp.User.TenantId,
		TenantName:  mockResp.User.TenantName,
	}

	// Сохраняем токены
	if err := am.tokenStore.SaveTokens(tokenInfo); err != nil {
		am.metrics.ErrorsCount.WithLabelValues("login", "token_storage_error").Inc()
		am.metrics.RequestDuration.WithLabelValues("login", "token_storage_error").Observe(time.Since(start).Seconds())
		am.logger.Error("ошибка сохранения токенов", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка сохранения токенов")
	}

	// Обновляем конфигурацию
	am.config.SetCurrentTenant(mockResp.User.TenantId)
	if err := am.config.Save(); err != nil {
		am.metrics.ErrorsCount.WithLabelValues("login", "config_save_error").Inc()
		am.metrics.RequestDuration.WithLabelValues("login", "config_save_error").Observe(time.Since(start).Seconds())
		am.logger.Error("ошибка сохранения конфигурации", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка сохранения конфигурации")
	}

	// Записываем успешную метрику
	am.metrics.RequestCount.WithLabelValues("login", "success", "").Inc()
	am.metrics.RequestDuration.WithLabelValues("login", "success").Observe(time.Since(start).Seconds())

	am.logger.Info("вход выполнен успешно", 
		logger.String("email", mockResp.User.Email),
		logger.String("tenant", mockResp.User.TenantName),
		logger.String("expires_at", expiresAt.Format(time.RFC3339)))

	fmt.Printf("✅ Вход выполнен успешно!\n")
	fmt.Printf("👤 Пользователь: %s\n", mockResp.User.Email)
	fmt.Printf("🏢 Тенант: %s\n", mockResp.User.TenantName)
	fmt.Printf("⏰ Токен истекает: %s\n", expiresAt.Format("2006-01-02 15:04:05"))

	return nil
}

// Logout выполняет выход пользователя
func (am *AuthManager) Logout(ctx context.Context) error {
	am.logger.Info("попытка выхода пользователя")

	if !am.tokenStore.HasTokens() {
		am.logger.Warn("попытка выхода неавторизованного пользователя")
		return errors.New(errors.ErrUnauthorized, "пользователь не авторизован")
	}

	// Получаем токен
	accessToken, err := am.tokenStore.GetAccessToken()
	if err != nil {
		am.logger.Error("ошибка получения токена", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка получения токена")
	}

	// Используем gRPC если доступно
	if am.useGRPC && am.authClient != nil {
		// Вызываем Auth Service API через gRPC
		req := &client.LogoutRequest{
			AccessToken: accessToken,
		}

		err = am.authClient.Logout(ctx, req)
		if err != nil {
			am.logger.Error("ошибка логаута через gRPC", logger.Error(err))
			return errors.Wrap(err, errors.ErrInternal, "ошибка логаута через gRPC")
		}
	}

	// Удаляем локальные токены
	if err := am.tokenStore.ClearTokens(); err != nil {
		am.logger.Error("ошибка удаления токенов", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка удаления токенов")
	}

	// Очищаем текущий тенант в конфигурации
	am.config.SetCurrentTenant("")
	if err := am.config.Save(); err != nil {
		am.logger.Error("ошибка сохранения конфигурации", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка сохранения конфигурации")
	}

	am.logger.Info("выход выполнен успешно")
	fmt.Printf("✅ Выход выполнен успешно!\n")

	return nil
}

// IsAuthenticated проверяет, авторизован ли пользователь
func (am *AuthManager) IsAuthenticated() bool {
	if !am.tokenStore.HasTokens() {
		am.logger.Debug("пользователь не авторизован - отсутствуют токены")
		return false
	}

	// Проверяем, не истек ли токен
	expired, err := am.tokenStore.IsTokenExpired()
	if err != nil {
		am.logger.Error("ошибка проверки срока действия токена", logger.Error(err))
		return false
	}

	if expired {
		am.logger.Warn("токен истек")
		return false
	}

	am.logger.Debug("пользователь авторизован")
	return true
}

// RefreshToken обновляет токен
func (am *AuthManager) RefreshToken(ctx context.Context) error {
	am.logger.Info("попытка обновления токена")

	if !am.tokenStore.HasTokens() {
		am.logger.Warn("отсутствуют токены для обновления")
		return errors.New(errors.ErrUnauthorized, "отсутствуют токены для обновления")
	}

	refreshToken, err := am.tokenStore.GetRefreshToken()
	if err != nil {
		am.logger.Error("ошибка получения refresh токена", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка получения refresh токена")
	}

	_ = refreshToken // Используем для подавления ошибки неиспользуемой переменной

	// Используем gRPC если доступно
	if am.useGRPC && am.authClient != nil {
		// Вызываем Auth Service API через gRPC
		req := &client.RefreshTokenRequest{
			RefreshToken: refreshToken,
		}

		resp, err := am.authClient.RefreshToken(ctx, req)
		if err != nil {
			am.logger.Error("ошибка обновления токена через gRPC", logger.Error(err))
			return errors.Wrap(err, errors.ErrUnauthorized, "ошибка обновления токена через gRPC")
		}

		if !resp.Success {
			am.logger.Warn("неудачное обновление токена через gRPC", logger.String("message", "неудачное обновление"))
			return errors.New(errors.ErrUnauthorized, "неудачное обновление")
		}

		// Рассчитываем новое время истечения
		expiresAt := time.Now().Add(time.Duration(am.config.Auth.TokenExpiry) * time.Second)

		// Обновляем токены
		if err := am.tokenStore.UpdateTokens(resp.AccessToken, resp.RefreshToken, expiresAt); err != nil {
			am.logger.Error("ошибка обновления токенов", logger.Error(err))
			return errors.Wrap(err, errors.ErrInternal, "ошибка обновления токенов")
		}

		am.logger.Info("токен успешно обновлен через gRPC", logger.String("expires_at", expiresAt.Format(time.RFC3339)))
		fmt.Printf("✅ Токен успешно обновлен!\n")
		fmt.Printf("⏰ Новый токен истекает: %s\n", expiresAt.Format("2006-01-02 15:04:05"))

		return nil
	}

	// Mock успешного ответа
	mockResp := struct {
		Success      bool   `json:"success"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}{
		Success:      true,
		AccessToken:  "new-mock-access-token",
		RefreshToken: "new-mock-refresh-token",
	}

	// Рассчитываем новое время истечения
	expiresAt := time.Now().Add(time.Duration(am.config.Auth.TokenExpiry) * time.Second)

	// Обновляем токены
	if err := am.tokenStore.UpdateTokens(mockResp.AccessToken, mockResp.RefreshToken, expiresAt); err != nil {
		am.logger.Error("ошибка обновления токенов", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка обновления токенов")
	}

	am.logger.Info("токен успешно обновлен", logger.String("expires_at", expiresAt.Format(time.RFC3339)))
	fmt.Printf("✅ Токен успешно обновлен!\n")
	fmt.Printf("⏰ Новый токен истекает: %s\n", expiresAt.Format("2006-01-02 15:04:05"))

	return nil
}

// EnsureValidToken обеспечивает наличие валидного токена
func (am *AuthManager) EnsureValidToken(ctx context.Context) error {
	if !am.IsAuthenticated() {
		am.logger.Warn("пользователь не авторизован")
		return errors.New(errors.ErrUnauthorized, "пользователь не авторизован. Выполните 'uptimeping auth login'")
	}

	// Проверяем, нужно ли обновить токен
	threshold := time.Duration(am.config.Auth.RefreshThreshold) * time.Second
	shouldRefresh, err := am.tokenStore.ShouldRefreshToken(threshold)
	if err != nil {
		am.logger.Error("ошибка проверки необходимости обновления токена", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка проверки токена")
	}

	if shouldRefresh {
		am.logger.Info("токен скоро истекает, обновляем...")
		if err := am.RefreshToken(ctx); err != nil {
			am.logger.Error("ошибка обновления токена", logger.Error(err))
			return errors.Wrap(err, errors.ErrInternal, "ошибка обновления токена")
		}
	}

	return nil
}

// GetAuthToken возвращает токен для API запросов
func (am *AuthManager) GetAuthToken() (string, error) {
	if !am.IsAuthenticated() {
		am.logger.Warn("попытка получения токена неавторизованным пользователем")
		return "", errors.New(errors.ErrUnauthorized, "пользователь не авторизован")
	}

	accessToken, err := am.tokenStore.GetAccessToken()
	if err != nil {
		am.logger.Error("ошибка получения access токена", logger.Error(err))
		return "", errors.Wrap(err, errors.ErrInternal, "ошибка получения access токена")
	}

	am.logger.Debug("access токен успешно получен")
	return accessToken, nil
}

// GetUserInfo возвращает информацию о текущем пользователе
func (am *AuthManager) GetUserInfo() (string, string, string, string, error) {
	if !am.IsAuthenticated() {
		am.logger.Warn("попытка получения информации о неавторизованном пользователе")
		return "", "", "", "", errors.New(errors.ErrUnauthorized, "пользователь не авторизован")
	}

	userID, email, err := am.tokenStore.GetUserInfo()
	if err != nil {
		am.logger.Error("ошибка получения информации о пользователе", logger.Error(err))
		return "", "", "", "", errors.Wrap(err, errors.ErrInternal, "ошибка получения информации о пользователе")
	}

	tenantID, tenantName, err := am.tokenStore.GetCurrentTenant()
	if err != nil {
		am.logger.Error("ошибка получения информации о тенанте", logger.Error(err))
		return "", "", "", "", errors.Wrap(err, errors.ErrInternal, "ошибка получения информации о тенанте")
	}

	am.logger.Debug("информация о пользователе успешно получена",
		logger.String("user_id", userID),
		logger.String("email", email),
		logger.String("tenant_id", tenantID))

	return userID, email, tenantID, tenantName, nil
}

// RegisterInput представляет ввод для регистрации
type RegisterInput struct {
	Email       string
	Password    string
	TenantName  string
}

// GetRegisterInput получает ввод для регистрации интерактивно
func GetRegisterInput() (*RegisterInput, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения email: %w", err)
	}
	email = strings.TrimSpace(email)

	if email == "" {
		return nil, fmt.Errorf("email не может быть пустым")
	}

	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return nil, fmt.Errorf("некорректный формат email")
	}

	fmt.Print("Пароль (минимум 8 символов): ")
	password, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения пароля: %w", err)
	}
	password = strings.TrimSpace(password)

	if len(password) < 8 {
		return nil, fmt.Errorf("пароль должен содержать минимум 8 символов")
	}

	fmt.Print("Название тенанта: ")
	tenantName, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения названия тенанта: %w", err)
	}
	tenantName = strings.TrimSpace(tenantName)

	if tenantName == "" {
		return nil, fmt.Errorf("название тенанта не может быть пустым")
	}

	return &RegisterInput{
		Email:      email,
		Password:   password,
		TenantName: tenantName,
	}, nil
}

// Register выполняет регистрацию пользователя
func (am *AuthManager) Register(ctx context.Context, input *RegisterInput) error {
	am.logger.Info("попытка регистрации пользователя", 
		logger.String("email", input.Email),
		logger.String("tenant_name", input.TenantName))

	// Валидация входных данных
	if err := am.validator.ValidateRequiredFields(map[string]interface{}{
		"email":       input.Email,
		"password":    input.Password,
		"tenant_name": input.TenantName,
	}, map[string]string{}); err != nil {
		am.logger.Error("ошибка валидации данных регистрации", logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "некорректные данные регистрации")
	}

	// Валидация полей
	if err := am.validator.ValidateStringLength("email", input.Email, 5, 100); err != nil {
		am.logger.Error("некорректная длина email", logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "email должен содержать от 5 до 100 символов")
	}

	if err := am.validator.ValidateStringLength("password", input.Password, 8, 128); err != nil {
		am.logger.Error("некорректная длина пароля", logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "пароль должен содержать от 8 до 128 символов")
	}

	if err := am.validator.ValidateStringLength("tenant_name", input.TenantName, 2, 50); err != nil {
		am.logger.Error("некорректная длина названия тенанта", logger.Error(err))
		return errors.Wrap(err, errors.ErrValidation, "название тенанта должно содержать от 2 до 50 символов")
	}

	// Используем gRPC если доступно
	if am.useGRPC && am.authClient != nil {
		// Вызываем Auth Service API через gRPC
		req := &client.RegisterRequest{
			Email:      input.Email,
			Password:    input.Password,
			TenantName:  input.TenantName,
		}

		resp, err := am.authClient.Register(ctx, req)
		if err != nil {
			am.logger.Error("ошибка регистрации через gRPC", logger.Error(err), logger.String("email", input.Email))
			return errors.Wrap(err, errors.ErrInternal, "ошибка регистрации через gRPC")
		}

		if !resp.Success {
			am.logger.Warn("неудачная попытка регистрации через gRPC", logger.String("message", resp.Message), logger.String("email", input.Email))
			return errors.New(errors.ErrConflict, resp.Message)
		}

		am.logger.Info("регистрация выполнена успешно через gRPC", 
			logger.String("email", resp.User.Email),
			logger.String("tenant_name", resp.User.TenantName))

		fmt.Printf("✅ Регистрация выполнена успешно!\n")
		fmt.Printf("👤 Пользователь: %s\n", resp.User.Email)
		fmt.Printf("🏢 Тенант: %s\n", resp.User.TenantName)
		fmt.Printf("💡 Теперь выполните 'uptimeping auth login' для входа\n")

		return nil
	}

	// Mock успешного ответа
	am.logger.Info("регистрация выполнена успешно", 
		logger.String("email", input.Email),
		logger.String("tenant_name", input.TenantName))

	fmt.Printf("✅ Регистрация выполнена успешно!\n")
	fmt.Printf("👤 Пользователь: %s\n", input.Email)
	fmt.Printf("🏢 Тенант: %s\n", input.TenantName)
	fmt.Printf("💡 Теперь выполните 'uptimeping auth login' для входа\n")

	return nil
}

// Status показывает статус аутентификации
func (am *AuthManager) Status() error {
	am.logger.Debug("проверка статуса аутентификации")

	if !am.tokenStore.HasTokens() {
		am.logger.Info("пользователь не авторизован")
		fmt.Printf("❌ Не авторизован\n")
		return nil
	}

	userID, email, tenantID, tenantName, err := am.GetUserInfo()
	if err != nil {
		am.logger.Error("ошибка получения информации о пользователе", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка получения информации о пользователе")
	}

	tokens, err := am.tokenStore.LoadTokens()
	if err != nil {
		am.logger.Error("ошибка загрузки токенов", logger.Error(err))
		return errors.Wrap(err, errors.ErrInternal, "ошибка загрузки токенов")
	}

	expired := time.Now().After(tokens.ExpiresAt)

	if expired {
		am.logger.Warn("токен истек", 
			logger.String("user_id", userID),
			logger.String("email", email))
		fmt.Printf("❌ Токен истек\n")
		fmt.Printf("👤 Пользователь: %s\n", email)
		fmt.Printf("🏢 Тенант: %s (%s)\n", tenantName, tenantID)
		fmt.Printf("💡 Выполните 'uptimeping auth login' для обновления\n")
	} else {
		am.logger.Info("пользователь авторизован",
			logger.String("user_id", userID),
			logger.String("email", email),
			logger.String("tenant_id", tenantID),
			logger.String("expires_at", tokens.ExpiresAt.Format(time.RFC3339)))
		fmt.Printf("✅ Авторизован\n")
		fmt.Printf("👤 Пользователь: %s (ID: %s)\n", email, userID)
		fmt.Printf("🏢 Тенант: %s (%s)\n", tenantName, tenantID)
		fmt.Printf("⏰ Токен истекает: %s\n", tokens.ExpiresAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("🔑 Тип токена: %s\n", tokens.TokenType)
	}

	return nil
}
