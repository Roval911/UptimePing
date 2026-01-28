package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "UptimePingPlatform/proto/api/auth/v1"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/metrics"
)

// AuthGRPCClient представляет gRPC клиент для взаимодействия с Auth Service
type AuthGRPCClient struct {
	client   authv1.AuthServiceClient
	conn     *grpc.ClientConn
	logger   logger.Logger
	baseURL  string
	metrics  *metrics.Metrics
}

// NewAuthGRPCClient создает новый gRPC клиент для Auth Service
func NewAuthGRPCClient(authAddr string, log logger.Logger) (*AuthGRPCClient, error) {
	client := &AuthGRPCClient{
		logger:  log,
		baseURL: authAddr,
		metrics: metrics.NewMetrics("cli-auth-client"),
	}

	if authAddr != "" {
		conn, err := grpc.Dial(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("ошибка подключения к Auth Service: %w", err)
		}
		client.conn = conn
		client.client = authv1.NewAuthServiceClient(conn)

		client.logger.Info("подключено к Auth Service", 
			logger.String("address", authAddr))
	}

	return client, nil
}

// Close закрывает gRPC соединение
func (c *AuthGRPCClient) Close() error {
	if c.conn != nil {
		c.logger.Info("закрытие соединения с Auth Service")
		return c.conn.Close()
	}
	return nil
}

// LoginRequest представляет запрос на вход
type LoginRequest struct {
	Email    string
	Password string
}

// LoginResponse представляет ответ на вход
type LoginResponse struct {
	Success      bool   `json:"success"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	User         User   `json:"user"`
}

// User представляет информацию о пользователе
type User struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	TenantID   string `json:"tenant_id"`
	TenantName string `json:"tenant_name"`
}

// Login выполняет вход пользователя через gRPC
func (c *AuthGRPCClient) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	if c.client == nil {
		return nil, fmt.Errorf("Auth Service gRPC клиент не доступен")
	}

	start := time.Now()
	c.logger.Info("выполнение входа через gRPC", 
		logger.String("email", req.Email))

	protoReq := &authv1.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	protoResp, err := c.client.Login(ctx, protoReq)
	duration := time.Since(start)
	
	if err != nil {
		c.logger.Error("ошибка входа через gRPC", 
			logger.Error(err),
			logger.Duration("duration", duration))
		c.metrics.ErrorsCount.WithLabelValues("Login", "error", "grpc_error").Inc()
		c.metrics.RequestDuration.WithLabelValues("Login", "error").Observe(duration.Seconds())
		return nil, fmt.Errorf("ошибка gRPC вызова Login: %w", err)
	}

	response := &LoginResponse{
		Success:      true, // Если ответ получен, значит успех
		AccessToken:  protoResp.AccessToken,
		RefreshToken: protoResp.RefreshToken,
		TokenType:    "Bearer",
		User: User{
			ID:         "", // UserId не возвращается в TokenPair
			Email:      req.Email,
			TenantID:   "", // TenantId не возвращается в TokenPair
			TenantName: "", // TenantName не возвращается в TokenPair
		},
	}

	c.logger.Info("вход выполнен через gRPC", 
		logger.String("email", response.User.Email),
		logger.String("tenant", response.User.TenantName),
		logger.Duration("duration", duration))
	
	c.metrics.RequestCount.WithLabelValues("Login", "success").Inc()
	c.metrics.RequestDuration.WithLabelValues("Login", "success").Observe(duration.Seconds())

	return response, nil
}

// LogoutRequest представляет запрос на выход
type LogoutRequest struct {
	AccessToken string
}

// Logout выполняет выход пользователя через gRPC
func (c *AuthGRPCClient) Logout(ctx context.Context, req *LogoutRequest) error {
	if c.client == nil {
		return fmt.Errorf("Auth Service gRPC клиент не доступен")
	}

	start := time.Now()
	c.logger.Info("выполнение выхода через gRPC")

	protoReq := &authv1.LogoutRequest{
		UserId:       c.extractUserIDFromToken(req.AccessToken),
		RefreshToken: req.AccessToken, // Используем access token как refresh token для простоты
	}

	_, err := c.client.Logout(ctx, protoReq)
	duration := time.Since(start)
	
	if err != nil {
		c.logger.Error("ошибка выхода через gRPC", 
			logger.Error(err),
			logger.Duration("duration", duration))
		c.metrics.ErrorsCount.WithLabelValues("Logout", "error", "grpc_error").Inc()
		c.metrics.RequestDuration.WithLabelValues("Logout", "error").Observe(duration.Seconds())
		return fmt.Errorf("ошибка gRPC вызова Logout: %w", err)
	}

	c.logger.Info("выход выполнен через gRPC", logger.Duration("duration", duration))
	c.metrics.RequestCount.WithLabelValues("Logout", "success").Inc()
	c.metrics.RequestDuration.WithLabelValues("Logout", "success").Observe(duration.Seconds())
	
	return nil
}

// RefreshTokenRequest представляет запрос на обновление токена
type RefreshTokenRequest struct {
	RefreshToken string
}

// RefreshTokenResponse представляет ответ на обновление токена
type RefreshTokenResponse struct {
	Success      bool   `json:"success"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// RefreshToken обновляет токен через gRPC
func (c *AuthGRPCClient) RefreshToken(ctx context.Context, req *RefreshTokenRequest) (*RefreshTokenResponse, error) {
	if c.client == nil {
		return nil, fmt.Errorf("Auth Service gRPC клиент не доступен")
	}

	start := time.Now()
	c.logger.Info("обновление токена через gRPC")

	protoReq := &authv1.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	}

	protoResp, err := c.client.RefreshToken(ctx, protoReq)
	duration := time.Since(start)
	
	if err != nil {
		c.logger.Error("ошибка обновления токена через gRPC", 
			logger.Error(err),
			logger.Duration("duration", duration))
		c.metrics.ErrorsCount.WithLabelValues("RefreshToken", "error", "grpc_error").Inc()
		c.metrics.RequestDuration.WithLabelValues("RefreshToken", "error").Observe(duration.Seconds())
		return nil, fmt.Errorf("ошибка gRPC вызова RefreshToken: %w", err)
	}

	response := &RefreshTokenResponse{
		Success:      true, // Если ответ получен, значит успех
		AccessToken:  protoResp.AccessToken,
		RefreshToken: protoResp.RefreshToken,
		TokenType:    "Bearer",
	}

	c.logger.Info("токен обновлен через gRPC", logger.Duration("duration", duration))
	c.metrics.RequestCount.WithLabelValues("RefreshToken", "success").Inc()
	c.metrics.RequestDuration.WithLabelValues("RefreshToken", "success").Observe(duration.Seconds())
	
	return response, nil
}

// RegisterRequest представляет запрос на регистрацию
type RegisterRequest struct {
	Email      string
	Password    string
	TenantName  string
}

// RegisterResponse представляет ответ на регистрацию
type RegisterResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	User    User   `json:"user"`
}

// Register выполняет регистрацию пользователя через gRPC
func (c *AuthGRPCClient) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	if c.client == nil {
		return nil, fmt.Errorf("Auth Service gRPC клиент не доступен")
	}

	start := time.Now()
	c.logger.Info("выполнение регистрации через gRPC", 
		logger.String("email", req.Email),
		logger.String("tenant_name", req.TenantName))

	protoReq := &authv1.RegisterRequest{
		Email:      req.Email,
		Password:    req.Password,
		TenantName:  req.TenantName,
	}

	_, err := c.client.Register(ctx, protoReq)
	duration := time.Since(start)
	
	if err != nil {
		c.logger.Error("ошибка регистрации через gRPC", 
			logger.Error(err),
			logger.Duration("duration", duration))
		c.metrics.ErrorsCount.WithLabelValues("Register", "error", "grpc_error").Inc()
		c.metrics.RequestDuration.WithLabelValues("Register", "error").Observe(duration.Seconds())
		return nil, fmt.Errorf("ошибка gRPC вызова Register: %w", err)
	}

	response := &RegisterResponse{
		Success: true, // Если ответ получен, значит успех
		Message: "Регистрация выполнена успешно",
		User: User{
			ID:         "", // UserId не возвращается в TokenPair
			Email:      req.Email,
			TenantID:   "", // TenantId не возвращается в TokenPair
			TenantName: "", // TenantName не возвращается в TokenPair
		},
	}

	c.logger.Info("регистрация выполнена через gRPC", 
		logger.String("email", response.User.Email),
		logger.String("tenant_name", response.User.TenantName),
		logger.Duration("duration", duration))
	
	c.metrics.RequestCount.WithLabelValues("Register", "success").Inc()
	c.metrics.RequestDuration.WithLabelValues("Register", "success").Observe(duration.Seconds())

	return response, nil
}

// ValidateTokenRequest представляет запрос на валидацию токена
type ValidateTokenRequest struct {
	AccessToken string
}

// ValidateTokenResponse представляет ответ на валидацию токена
type ValidateTokenResponse struct {
	Valid bool   `json:"valid"`
	User  User   `json:"user"`
}

// ValidateToken валидирует токен через gRPC
func (c *AuthGRPCClient) ValidateToken(ctx context.Context, req *ValidateTokenRequest) (*ValidateTokenResponse, error) {
	if c.client == nil {
		return nil, fmt.Errorf("Auth Service gRPC клиент не доступен")
	}

	start := time.Now()
	c.logger.Debug("валидация токена через gRPC")

	protoReq := &authv1.ValidateTokenRequest{
		Token: req.AccessToken,
	}

	protoResp, err := c.client.ValidateToken(ctx, protoReq)
	duration := time.Since(start)
	
	if err != nil {
		c.logger.Error("ошибка валидации токена через gRPC", 
			logger.Error(err),
			logger.Duration("duration", duration))
		c.metrics.ErrorsCount.WithLabelValues("ValidateToken", "error", "grpc_error").Inc()
		c.metrics.RequestDuration.WithLabelValues("ValidateToken", "error").Observe(duration.Seconds())
		return nil, fmt.Errorf("ошибка gRPC вызова ValidateToken: %w", err)
	}

	response := &ValidateTokenResponse{
		Valid: protoResp.IsValid,
		User: User{
			ID:         protoResp.UserId,
			Email:      protoResp.Email,
			TenantID:   protoResp.TenantId,
			TenantName: "", // TenantName не возвращается в ValidateTokenResponse
		},
	}

	c.logger.Debug("токен валидирован через gRPC", 
		logger.Bool("valid", response.Valid),
		logger.Duration("duration", duration))
	
	c.metrics.RequestCount.WithLabelValues("ValidateToken", "success").Inc()
	c.metrics.RequestDuration.WithLabelValues("ValidateToken", "success").Observe(duration.Seconds())

	return response, nil
}

// extractUserIDFromToken извлекает user ID из JWT токена
func (c *AuthGRPCClient) extractUserIDFromToken(token string) string {
	// Удаляем префикс "Bearer " если он есть
	if strings.HasPrefix(token, "Bearer ") {
		token = token[7:]
	}

	// Парсим токен без валидации подписи (для извлечения claims)
	parsedToken, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		c.logger.Debug("Failed to parse JWT token", logger.String("error", err.Error()))
		return "system"
	}

	// Извлекаем claims
	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
		// Проверяем различные возможные поля для user ID
		if userID, exists := claims["user_id"]; exists {
			if uid, ok := userID.(string); ok {
				return uid
			}
		}
		if userID, exists := claims["sub"]; exists {
			if uid, ok := userID.(string); ok {
				return uid
			}
		}
		if userID, exists := claims["id"]; exists {
			if uid, ok := userID.(string); ok {
				return uid
			}
		}
		
		// Проверяем срок действия токена
		if exp, exists := claims["exp"]; exists {
			if expFloat, ok := exp.(float64); ok {
				if time.Unix(int64(expFloat), 0).Before(time.Now()) {
					c.logger.Debug("Token has expired")
					return "system"
				}
			}
		}
	}

	c.logger.Debug("No user ID found in JWT token")
	return "system"
}
