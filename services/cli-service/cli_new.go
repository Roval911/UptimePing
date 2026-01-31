package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"
)

// TokenInfo хранит информацию о токенах
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TenantID     string    `json:"tenant_id"`
	TenantName   string    `json:"tenant_name"`
	Email        string    `json:"email"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// RedisTokenStore хранит токены в Redis
type RedisTokenStore struct {
	client *redis.Client
	prefix string
}

// NewRedisTokenStore создает новое хранилище токенов в Redis
func NewRedisTokenStore() (*RedisTokenStore, error) {
	redisAddr := "localhost:6379"
	// Проверяем, запущены ли мы в Docker
	if os.Getenv("ENVIRONMENT") == "dev" && os.Getenv("REDIS_ADDR") == "" {
		// В Docker контейнере Redis доступен по имени сервиса
		redisAddr = "redis:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к Redis: %w", err)
	}

	return &RedisTokenStore{
		client: rdb,
		prefix: "uptimeping:cli:tokens:",
	}, nil
}

// SaveTokens сохраняет токены в Redis
func (rts *RedisTokenStore) SaveTokens(tokenInfo *TokenInfo) error {
	ctx := context.Background()

	data, err := json.Marshal(tokenInfo)
	if err != nil {
		return fmt.Errorf("ошибка сериализации токенов: %w", err)
	}

	key := rts.prefix + "current"

	// Сохраняем с TTL равным времени жизни токена
	ttl := time.Until(tokenInfo.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Hour // по умолчанию 1 час
	}

	err = rts.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("ошибка сохранения токенов в Redis: %w", err)
	}

	fmt.Printf("💾 Токен сохранен в Redis с TTL: %v\n", ttl)
	return nil
}

// LoadTokens загружает токены из Redis
func (rts *RedisTokenStore) LoadTokens() (*TokenInfo, error) {
	ctx := context.Background()

	key := rts.prefix + "current"
	data, err := rts.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("токены не найдены")
		}
		return nil, fmt.Errorf("ошибка загрузки токенов из Redis: %w", err)
	}

	var tokenInfo TokenInfo
	err = json.Unmarshal([]byte(data), &tokenInfo)
	if err != nil {
		return nil, fmt.Errorf("ошибка десериализации токенов: %w", err)
	}

	return &tokenInfo, nil
}

// ClearTokens удаляет токены из Redis
func (rts *RedisTokenStore) ClearTokens() error {
	ctx := context.Background()

	key := rts.prefix + "current"
	err := rts.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("ошибка удаления токенов из Redis: %w", err)
	}

	return nil
}

// Close закрывает соединение с Redis
func (rts *RedisTokenStore) Close() error {
	return rts.client.Close()
}

// HTTPClient HTTP клиент для работы с API Gateway
type HTTPClient struct {
	baseURL    string
	client     *http.Client
	tokenStore *RedisTokenStore
}

// NewHTTPClient создает новый HTTP клиент
func NewHTTPClient(baseURL string, tokenStore *RedisTokenStore) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		tokenStore: tokenStore,
	}
}

// makeRequest выполняет HTTP запрос с авторизацией
func (c *HTTPClient) makeRequest(method, endpoint string, body interface{}, requireAuth bool) (*http.Response, error) {
	ctx := context.Background()

	var req *http.Request
	var err error
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("ошибка кодирования тела запроса: %w", err)
		}
		req, err = http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, strings.NewReader(string(jsonBody)))
		if err != nil {
			return nil, fmt.Errorf("ошибка создания запроса: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("ошибка создания запроса: %w", err)
		}
	}

	// Добавляем заголовок авторизации только если требуется
	if requireAuth {
		tokenInfo, err := c.tokenStore.LoadTokens()
		if err != nil {
			return nil, fmt.Errorf("токен авторизации не найден: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tokenInfo.AccessToken)
	}

	req.Header.Set("User-Agent", "UptimePing-CLI/2.0")

	return c.client.Do(req)
}

// Глобальные переменные
var (
	tokenStore *RedisTokenStore
	httpClient *HTTPClient
)

// Глобальные флаги
var (
	emailFlag         string
	passwordFlag      string
	tenantFlag        string
	checkNameFlag     string
	checkTypeFlag     string
	checkURLFlag      string
	checkIntervalFlag int
	checkTimeoutFlag  int
)

func main() {
	// Инициализация Redis хранилища
	var err error
	tokenStore, err = NewRedisTokenStore()
	if err != nil {
		fmt.Printf("❌ Ошибка инициализации Redis: %v\n", err)
		os.Exit(1)
	}
	defer tokenStore.Close()

	// Инициализация HTTP клиента
	apiURL := "http://localhost:8080"
	// Проверяем, запущены ли мы в Docker
	if os.Getenv("ENVIRONMENT") == "dev" {
		// В Docker контейнере API Gateway доступен по имени сервиса
		apiURL = "http://api-gateway:8080"
	}
	httpClient = NewHTTPClient(apiURL, tokenStore)

	// Корневая команда
	rootCmd := &cobra.Command{
		Use:   "cli",
		Short: "UptimePing CLI",
		Long:  "UptimePing CLI - инструмент для управления мониторингом доступности сервисов",
	}

	// Auth команды
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Управление аутентификацией",
		Long:  "Команды для входа, выхода, регистрации и проверки статуса",
	}

	// Auth register
	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Зарегистрировать нового пользователя",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("🔄 Регистрация пользователя: %s\n", emailFlag)
			fmt.Printf("💾 Используется Redis хранилище токенов\n")

			// Регистрация через API Gateway
			body := map[string]interface{}{
				"email":       emailFlag,
				"password":    passwordFlag,
				"tenant_name": tenantFlag,
			}

			resp, err := httpClient.makeRequest("POST", "/api/v1/auth/register", body, false)
			if err != nil {
				fmt.Printf("❌ Ошибка регистрации: %v\n", err)
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusCreated {
				// Парсим ответ и сохраняем токен
				var tokenResponse map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&tokenResponse)

				// API Gateway возвращает токены прямо в корне
				if tokenResponse["access_token"] != nil {
					tokenInfo := &TokenInfo{
						AccessToken:  tokenResponse["access_token"].(string),
						RefreshToken: tokenResponse["refresh_token"].(string),
						Email:        emailFlag,
						ExpiresAt:    time.Now().Add(time.Hour), // TODO: получить из ответа
					}

					// Добавляем tenant_id если он есть в ответе
					if tenantID, ok := tokenResponse["tenant_id"].(string); ok {
						tokenInfo.TenantID = tenantID
					}

					err = tokenStore.SaveTokens(tokenInfo)
					if err != nil {
						fmt.Printf("⚠️  Предупреждение: не удалось сохранить токен: %v\n", err)
					} else {
						fmt.Printf("💾 Токен сохранен в Redis\n")
					}
				}
			}

			fmt.Printf("✅ Регистрация успешна!\n")
			fmt.Printf("👤 Пользователь: %s\n", emailFlag)
			if tenantFlag != "" {
				fmt.Printf("🏢 Тенант: %s\n", tenantFlag)
			}
		},
	}
	registerCmd.Flags().StringVar(&emailFlag, "email", "", "Email адрес")
	registerCmd.Flags().StringVar(&passwordFlag, "password", "", "Пароль")
	registerCmd.Flags().StringVar(&tenantFlag, "tenant", "", "Имя тенанта")
	registerCmd.MarkFlagRequired("email")
	registerCmd.MarkFlagRequired("password")

	// Auth login
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Войти в систему",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("🔄 Вход пользователя: %s\n", emailFlag)
			fmt.Printf("💾 Используется Redis хранилище токенов\n")

			body := map[string]interface{}{
				"email":    emailFlag,
				"password": passwordFlag,
			}

			resp, err := httpClient.makeRequest("POST", "/api/v1/auth/login", body, false)
			if err != nil {
				fmt.Printf("❌ Ошибка входа: %v\n", err)
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var response map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&response)

				// API Gateway возвращает токены прямо в корне
				if response["access_token"] != nil {
					tokenInfo := &TokenInfo{
						AccessToken:  response["access_token"].(string),
						RefreshToken: response["refresh_token"].(string),
						Email:        emailFlag,
						ExpiresAt:    time.Now().Add(time.Hour), // TODO: получить из ответа
					}

					// Добавляем tenant_id если он есть в ответе
					if tenantID, ok := response["tenant_id"].(string); ok {
						tokenInfo.TenantID = tenantID
					}

					err = tokenStore.SaveTokens(tokenInfo)
					if err != nil {
						fmt.Printf("⚠️  Предупреждение: не удалось сохранить токен: %v\n", err)
					}
				}
			}

			fmt.Printf("✅ Вход выполнен успешно!\n")
			fmt.Printf("👤 Пользователь: %s\n", emailFlag)
		},
	}
	loginCmd.Flags().StringVar(&emailFlag, "email", "", "Email адрес")
	loginCmd.Flags().StringVar(&passwordFlag, "password", "", "Пароль")
	loginCmd.MarkFlagRequired("email")
	loginCmd.MarkFlagRequired("password")

	// Auth status
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Проверить статус аутентификации",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("💾 Проверка Redis хранилища токенов\n")

			tokenInfo, err := tokenStore.LoadTokens()
			if err != nil {
				fmt.Printf("❌ Пользователь не авторизован: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✅ Пользователь авторизован\n")
			fmt.Printf("👤 Email: %s\n", tokenInfo.Email)
			fmt.Printf("🏢 Тенант: %s\n", tokenInfo.TenantName)
			fmt.Printf("⏰ Токен истекает: %s\n", tokenInfo.ExpiresAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("⏳ TTL: %v\n", time.Until(tokenInfo.ExpiresAt))
		},
	}

	// Auth logout
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Выйти из системы",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("💾 Используется Redis хранилище токенов\n")

			err := tokenStore.ClearTokens()
			if err != nil {
				fmt.Printf("⚠️  Warning: failed to clear tokens from Redis: %v\n", err)
			}

			fmt.Printf("✅ Выход выполнен успешно!\n")
			fmt.Printf("💾 Токены удалены из Redis\n")
		},
	}

	// Добавляем auth команды
	authCmd.AddCommand(registerCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(statusCmd)
	authCmd.AddCommand(logoutCmd)

	// Checks команды
	checksCmd := &cobra.Command{
		Use:   "checks",
		Short: "Управление проверками",
		Long:  "Команды для управления проверками мониторинга",
	}

	// Checks list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Получить список проверок",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("🔄 Получение списка проверок...\n")
			fmt.Printf("💾 Используется Redis хранилище токенов\n")

			resp, err := httpClient.makeRequest("GET", "/api/v1/checks", nil, true)
			if err != nil {
				fmt.Printf("❌ Ошибка получения списка проверок: %v\n", err)
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var response map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&response)

				if checks, ok := response["checks"].([]interface{}); ok && len(checks) > 0 {
					fmt.Printf("✅ Найдено %d проверок:\n\n", len(checks))
					for i, check := range checks {
						if checkMap, ok := check.(map[string]interface{}); ok {
							fmt.Printf("%d. 📋 %s\n", i+1, checkMap["name"])
							fmt.Printf("   ID: %s\n", checkMap["id"])
							fmt.Printf("   Тип: %s\n", checkMap["type"])
							fmt.Printf("   Цель: %s\n", checkMap["target"])
							fmt.Printf("   Статус: %s\n", checkMap["status"])
							fmt.Println()
						}
					}
				} else {
					fmt.Printf("📭 Проверки не найдены\n")
				}
			} else {
				fmt.Printf("❌ Ошибка: сервер вернул статус %d\n", resp.StatusCode)
			}
		},
	}

	// Checks create
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Создать новую проверку",
		Run: func(cmd *cobra.Command, args []string) {
			if checkNameFlag == "" {
				fmt.Printf("❌ Ошибка: --name обязателен\n")
				os.Exit(1)
			}
			if checkTypeFlag == "" {
				fmt.Printf("❌ Ошибка: --type обязателен (http, tcp, icmp, grpc)\n")
				os.Exit(1)
			}
			if checkURLFlag == "" {
				fmt.Printf("❌ Ошибка: --url обязателен\n")
				os.Exit(1)
			}

			fmt.Printf("🔄 Создание новой проверки...\n")
			fmt.Printf("💾 Используется Redis хранилище токенов\n")

			check := map[string]interface{}{
				"name":     checkNameFlag,
				"type":     checkTypeFlag,
				"url":      checkURLFlag, // Исправлено: url вместо target
				"interval": checkIntervalFlag,
				"timeout":  checkTimeoutFlag,
				"enabled":  true,
			}

			resp, err := httpClient.makeRequest("POST", "/api/v1/checks", check, true)
			if err != nil {
				fmt.Printf("❌ Ошибка создания проверки: %v\n", err)
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusCreated {
				var response map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&response)

				fmt.Printf("✅ Проверка создана успешно!\n")
				fmt.Printf("📋 Название: %s\n", response["name"])
				fmt.Printf("🆔 ID: %s\n", response["id"])
				fmt.Printf("🌐 Тип: %s\n", response["check_type"])
				fmt.Printf("🎯 Цель: %s\n", response["url"])
				fmt.Printf("⏱️ Интервал: %v сек\n", response["interval"])
				fmt.Printf("⏳️ Таймаут: %v сек\n", response["timeout"])
				fmt.Printf("✅ Статус: %s\n", response["status"])
			} else {
				fmt.Printf("❌ Ошибка: сервер вернул статус %d\n", resp.StatusCode)
			}
		},
	}
	createCmd.Flags().StringVar(&checkNameFlag, "name", "", "Название проверки")
	createCmd.Flags().StringVar(&checkTypeFlag, "type", "", "Тип проверки (http, tcp, icmp, grpc)")
	createCmd.Flags().StringVar(&checkURLFlag, "url", "", "URL для проверки")
	createCmd.Flags().IntVar(&checkIntervalFlag, "interval", 60, "Интервал в секундах")
	createCmd.Flags().IntVar(&checkTimeoutFlag, "timeout", 10, "Таймаут в секундах")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("type")
	createCmd.MarkFlagRequired("url")

	// Checks get
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Получить информацию о проверке",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			checkID := args[0]
			fmt.Printf("🔄 Получение информации о проверке %s...\n", checkID)

			resp, err := httpClient.makeRequest("GET", "/api/v1/checks/"+checkID, nil, true)
			if err != nil {
				fmt.Printf("❌ Ошибка получения проверки: %v\n", err)
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var response map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&response)

				if data, ok := response["data"].(map[string]interface{}); ok {
					fmt.Printf("✅ Информация о проверке:\n")
					fmt.Printf("📋 Название: %s\n", data["name"])
					fmt.Printf("🆔 ID: %s\n", data["id"])
					fmt.Printf("🌐 Тип: %s\n", data["type"])
					fmt.Printf("🎯 Цель: %s\n", data["target"])
					fmt.Printf("⏱️ Интервал: %v сек\n", data["interval"])
					fmt.Printf("⏳️ Таймаут: %v сек\n", data["timeout"])
					fmt.Printf("✅ Статус: %s\n", data["status"])
					fmt.Printf("📅 Создана: %s\n", data["created_at"])
				}
			} else {
				fmt.Printf("❌ Ошибка: сервер вернул статус %d\n", resp.StatusCode)
			}
		},
	}

	// Checks delete
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Удалить проверку",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			checkID := args[0]
			fmt.Printf("🔄 Удаление проверки %s...\n", checkID)

			resp, err := httpClient.makeRequest("DELETE", "/api/v1/checks/"+checkID, nil, true)
			if err != nil {
				fmt.Printf("❌ Ошибка удаления проверки: %v\n", err)
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
				fmt.Printf("✅ Проверка %s удалена успешно!\n", checkID)
			} else {
				fmt.Printf("❌ Ошибка: сервер вернул статус %d\n", resp.StatusCode)
			}
		},
	}

	// Добавляем checks команды
	checksCmd.AddCommand(listCmd)
	checksCmd.AddCommand(createCmd)
	checksCmd.AddCommand(getCmd)
	checksCmd.AddCommand(deleteCmd)

	// Добавляем все команды в root
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(checksCmd)

	// Запускаем
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		os.Exit(1)
	}
}
