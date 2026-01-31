package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/cli-service/internal/auth"
	"UptimePingPlatform/services/cli-service/internal/client"
	cliConfig "UptimePingPlatform/services/cli-service/internal/config"
)

var checksCmd = &cobra.Command{
	Use:   "checks",
	Short: "Управление проверками",
	Long: `Команды для управления проверками доступности:
запуск, проверка статуса, просмотр истории и списка проверок.`,
}

var checksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Создать новую проверку",
	Long: `Создает новую проверку доступности с указанными параметрами.
Поддерживаются HTTP, TCP, ICMP, gRPC и GraphQL проверки.`,
	RunE: handleChecksCreate,
}

var checksGetCmd = &cobra.Command{
	Use:   "get [check-id]",
	Short: "Получить детали проверки",
	Long:  `Отображает детальную информацию о проверке по ее ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleChecksGet,
}

var checksUpdateCmd = &cobra.Command{
	Use:   "update [check-id]",
	Short: "Обновить проверку",
	Long:  `Обновляет параметры существующей проверки.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleChecksUpdate,
}

var checksEnableCmd = &cobra.Command{
	Use:   "enable [check-id]",
	Short: "Включить проверку",
	Long:  `Включает выполнение проверки по расписанию.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleChecksEnable,
}

var checksDisableCmd = &cobra.Command{
	Use:   "disable [check-id]",
	Short: "Отключить проверку",
	Long:  `Отключает выполнение проверки по расписанию.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleChecksDisable,
}

var checksDeleteCmd = &cobra.Command{
	Use:   "delete [check-id]",
	Short: "Удалить проверку",
	Long:  `Удаляет проверку и все связанные с ней данные.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleChecksDelete,
}

var checksListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать список проверок",
	Long:  `Отображает все доступные проверки с возможностью фильтрации.`,
	RunE:  handleChecksList,
}

func init() {
	checksCmd.AddCommand(checksCreateCmd)
	checksCmd.AddCommand(checksGetCmd)
	checksCmd.AddCommand(checksUpdateCmd)
	checksCmd.AddCommand(checksEnableCmd)
	checksCmd.AddCommand(checksDisableCmd)
	checksCmd.AddCommand(checksDeleteCmd)
	checksCmd.AddCommand(checksListCmd)

	// Checks create flags
	checksCreateCmd.Flags().StringP("name", "n", "", "название проверки (обязательно)")
	checksCreateCmd.Flags().StringP("url", "u", "", "URL для проверки (обязательно для HTTP/HTTPS)")
	checksCreateCmd.Flags().StringP("type", "t", "http", "тип проверки (http, https, tcp, icmp, grpc, graphql)")
	checksCreateCmd.Flags().IntP("interval", "i", 60, "интервал проверки в секундах")
	checksCreateCmd.Flags().IntP("timeout", "m", 10, "таймаут в секундах")
	checksCreateCmd.Flags().StringSliceP("tags", "g", []string{}, "теги для проверки")
	checksCreateCmd.Flags().BoolP("enabled", "e", true, "включить проверку")

	// Checks update flags
	checksUpdateCmd.Flags().StringP("name", "n", "", "новое название проверки")
	checksUpdateCmd.Flags().StringP("url", "u", "", "новый URL для проверки")
	checksUpdateCmd.Flags().IntP("interval", "i", 0, "новый интервал проверки в секундах")
	checksUpdateCmd.Flags().IntP("timeout", "m", 0, "новый таймаут в секундах")
	checksUpdateCmd.Flags().StringSliceP("tags", "g", []string{}, "новые теги для проверки")
	checksUpdateCmd.Flags().BoolP("enabled", "e", false, "включить/отключить проверку")

	// Checks list flags
	checksListCmd.Flags().StringSliceP("tags", "t", []string{}, "фильтр по тегам")
	checksListCmd.Flags().BoolP("enabled", "e", false, "фильтр по статусу (enabled/disabled)")
	checksListCmd.Flags().IntP("page", "p", 1, "номер страницы")
	checksListCmd.Flags().IntP("limit", "l", 20, "лимит записей на странице")
	checksListCmd.Flags().StringP("format", "f", "table", "формат вывода (table, json)")
}

func GetChecksCmd() *cobra.Command {
	return checksCmd
}

func handleChecksList(cmd *cobra.Command, args []string) error {
	page, _ := cmd.Flags().GetInt("page")
	limit, _ := cmd.Flags().GetInt("limit")
	format, _ := cmd.Flags().GetString("format")

	// Load configuration
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager and ensure valid token
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	ctx := context.Background()
	if err := authManager.EnsureValidToken(ctx); err != nil {
		return fmt.Errorf("ошибка аутентификации: %w", err)
	}

	// Create checks client instead of config client
	checksClient := client.NewChecksClient(cfg.API.BaseURL, authManager.GetTokenStore())
	defer checksClient.Close()

	// Get checks list
	checks, err := checksClient.ListChecks(ctx)
	if err != nil {
		return fmt.Errorf("ошибка получения списка проверок: %w", err)
	}

	if len(checks) == 0 {
		fmt.Printf("📭 Проверки не найдены\n")
		return nil
	}

	switch format {
	case "json":
		fmt.Println("[")
		for i, check := range checks {
			if i > 0 {
				fmt.Println(",")
			}
			fmt.Printf(`  {"id": "%s", "name": "%s", "type": "%s", "target": "%s", "interval": %d, "timeout": %d, "enabled": %t, "tags": [%s], "created_at": "%s"}`,
				check.ID,
				check.Name,
				check.Type,
				check.Target,
				check.Interval,
				check.Timeout,
				check.Enabled,
				strings.Join(check.Tags, ", "),
				check.CreatedAt)
		}
		fmt.Println("\n]")
	default:
		fmt.Printf("📋 Список проверок (страница %d):\n", page)
		fmt.Printf("%-20s %-25s %-10s %-30s %-10s %-10s %s\n", "🔍 ID", "📝 Название", "🔧 Тип", "🎯 Цель", "⏱️ Интервал", "⏰ Таймаут", "🏷️ Теги")
		fmt.Println(strings.Repeat("-", 120))

		for _, check := range checks {
			id := check.ID
			if len(id) > 18 {
				id = id[:15] + "..."
			}

			name := check.Name
			if len(name) > 23 {
				name = name[:20] + "..."
			}

			target := check.Target
			if len(target) > 28 {
				target = target[:25] + "..."
			}

			interval := fmt.Sprintf("%ds", check.Interval)
			timeout := fmt.Sprintf("%ds", check.Timeout)

			tags := strings.Join(check.Tags, ", ")
			if tags == "" {
				tags = "-"
			}

			fmt.Printf("%-20s %-25s %-10s %-30s %-10s %-10s %s\n", id, name, check.Type, target, interval, timeout, tags)
		}
	}

	fmt.Printf("\n📊 Всего проверок: %d\n", len(checks))
	fmt.Printf("📄 Страница: %d из %d\n", page, (len(checks)+limit-1)/limit)

	return nil
}

// handleChecksCreate обрабатывает создание новой проверки
func handleChecksCreate(cmd *cobra.Command, args []string) error {
	// Загрузка конфигурации CLI
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager and ensure valid token
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	// Get token
	ctx := context.Background()
	if err := authManager.EnsureValidToken(ctx); err != nil {
		return fmt.Errorf("ошибка проверки токена: %w", err)
	}

	token := authManager.GetTokenStore().GetAccessToken()

	// Добавляем токен в контекст
	ctx = context.WithValue(ctx, "access_token", token)

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	url, _ := cmd.Flags().GetString("url")
	checkType, _ := cmd.Flags().GetString("type")
	interval, _ := cmd.Flags().GetInt("interval")
	timeout, _ := cmd.Flags().GetInt("timeout")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	enabled, _ := cmd.Flags().GetBool("enabled")

	// Validate required fields
	if name == "" {
		return fmt.Errorf("флаг --name обязателен")
	}

	if checkType == "http" || checkType == "https" {
		if url == "" {
			return fmt.Errorf("флаг --url обязателен для HTTP/HTTPS проверок")
		}
	}

	// Create checks client
	checksClient := client.NewChecksClient(cfg.API.BaseURL, authManager.GetTokenStore())
	defer checksClient.Close()

	// Create check request
	request := &client.Check{
		Name:     name,
		Type:     checkType,
		Target:   url,
		Interval: interval,
		Timeout:  timeout,
		Tags:     tags,
		Metadata: map[string]interface{}{
			"enabled": fmt.Sprintf("%t", enabled),
		},
	}

	// Create check
	response, err := checksClient.CreateCheck(ctx, request)
	if err != nil {
		return fmt.Errorf("ошибка создания проверки: %w", err)
	}

	// Display result
	fmt.Printf("✅ Проверка создана успешно!\n")
	fmt.Printf("📝 ID: %s\n", response.ID)
	fmt.Printf("🔗 URL: %s\n", response.Target)
	fmt.Printf("⏱️ Интервал: %d секунд\n", response.Interval)
	fmt.Printf("⏰ Таймаут: %d секунд\n", response.Timeout)
	if len(response.Tags) > 0 {
		fmt.Printf("🏷️ Теги: %s\n", strings.Join(response.Tags, ", "))
	}
	fmt.Printf("🔧 Статус: ")
	if response.Enabled {
		fmt.Printf("Включена\n")
	} else {
		fmt.Printf("Отключена\n")
	}

	return nil
}

// handleChecksGet обрабатывает получение деталей проверки
func handleChecksGet(cmd *cobra.Command, args []string) error {
	checkID := args[0]

	// Валидация UUID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(checkID, "check_id"); err != nil {
		return fmt.Errorf("невалидный ID проверки: %w", err)
	}

	// Загрузка конфигурации CLI
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager and ensure valid token
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	// Get token
	ctx := context.Background()
	if err := authManager.EnsureValidToken(ctx); err != nil {
		return fmt.Errorf("ошибка проверки токена: %w", err)
	}

	token := authManager.GetTokenStore().GetAccessToken()

	// Добавляем токен в контекст
	ctx = context.WithValue(ctx, "access_token", token)

	// Create checks client instead of config client
	checksClient := client.NewChecksClient(cfg.API.BaseURL, authManager.GetTokenStore())
	defer checksClient.Close()

	// Get check
	check, err := checksClient.GetCheck(ctx, checkID)
	if err != nil {
		return fmt.Errorf("ошибка получения проверки: %w", err)
	}

	// Display result
	fmt.Printf("✅ Детали проверки:\n\n")
	fmt.Printf("📝 ID: %s\n", check.ID)
	fmt.Printf("🔗 Название: %s\n", check.Name)
	fmt.Printf("🌐 Тип: %s\n", check.Type)
	fmt.Printf("🎯 Цель: %s\n", check.Target)
	fmt.Printf("⏱️ Интервал: %d секунд\n", check.Interval)
	fmt.Printf("⏰ Таймаут: %d секунд\n", check.Timeout)

	if len(check.Tags) > 0 {
		fmt.Printf("🏷️ Теги: %s\n", strings.Join(check.Tags, ", "))
	}

	fmt.Printf("🔧 Статус: ")
	if check.Enabled {
		fmt.Printf("Включена\n")
	} else {
		fmt.Printf("Отключена\n")
	}

	if check.CreatedAt != "" {
		// Пробуем распарсить как Unix timestamp
		if timestamp, err := strconv.ParseInt(check.CreatedAt, 10, 64); err == nil {
			parsedTime := time.Unix(timestamp, 0)
			fmt.Printf("📅 Создана: %s\n", parsedTime.Format("2006-01-02 15:04:05"))
		} else if parsedTime, err := time.Parse(time.RFC3339, check.CreatedAt); err == nil {
			fmt.Printf("📅 Создана: %s\n", parsedTime.Format("2006-01-02 15:04:05"))
		}
	}

	if check.UpdatedAt != "" {
		// Пробуем распарсить как Unix timestamp
		if timestamp, err := strconv.ParseInt(check.UpdatedAt, 10, 64); err == nil {
			parsedTime := time.Unix(timestamp, 0)
			fmt.Printf("🔄 Обновлена: %s\n", parsedTime.Format("2006-01-02 15:04:05"))
		} else if parsedTime, err := time.Parse(time.RFC3339, check.UpdatedAt); err == nil {
			fmt.Printf("🔄 Обновлена: %s\n", parsedTime.Format("2006-01-02 15:04:05"))
		}
	}

	return nil
}

// handleChecksUpdate обрабатывает обновление проверки
func handleChecksUpdate(cmd *cobra.Command, args []string) error {
	checkID := args[0]

	// Валидация UUID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(checkID, "check_id"); err != nil {
		return fmt.Errorf("невалидный ID проверки: %w", err)
	}

	// Загрузка конфигурации CLI
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager and ensure valid token
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	// Get token
	ctx := context.Background()
	if err := authManager.EnsureValidToken(ctx); err != nil {
		return fmt.Errorf("ошибка проверки токена: %w", err)
	}

	token := authManager.GetTokenStore().GetAccessToken()

	// Добавляем токен в контекст
	ctx = context.WithValue(ctx, "access_token", token)

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	url, _ := cmd.Flags().GetString("url")
	interval, _ := cmd.Flags().GetInt("interval")
	timeout, _ := cmd.Flags().GetInt("timeout")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	enabled, _ := cmd.Flags().GetBool("enabled")

	// Проверяем, что хотя бы один флаг установлен
	if name == "" && url == "" && interval == 0 && timeout == 0 && len(tags) == 0 && !cmd.Flags().Changed("enabled") {
		return fmt.Errorf("необходимо указать хотя бы один параметр для обновления")
	}

	// Create checks client
	checksClient := client.NewChecksClient(cfg.API.BaseURL, authManager.GetTokenStore())
	defer checksClient.Close()

	// Create update request
	request := &client.Check{
		Metadata: map[string]interface{}{},
	}

	// Устанавливаем только те поля, которые были изменены
	if name != "" {
		request.Name = name
	}
	if url != "" {
		request.Target = url
	}
	if interval > 0 {
		request.Interval = interval
	}
	if timeout > 0 {
		request.Timeout = timeout
	}
	if len(tags) > 0 {
		request.Tags = tags
	}
	if cmd.Flags().Changed("enabled") {
		request.Enabled = enabled
		request.Metadata["enabled"] = fmt.Sprintf("%t", enabled)
	}

	// Update check
	response, err := checksClient.UpdateCheck(ctx, checkID, request)
	if err != nil {
		return fmt.Errorf("ошибка обновления проверки: %w", err)
	}

	// Display result
	fmt.Printf("✅ Проверка обновлена успешно!\n")
	fmt.Printf("📝 ID: %s\n", response.ID)
	fmt.Printf("🔗 Название: %s\n", response.Name)
	fmt.Printf("🎯 Цель: %s\n", response.Target)
	fmt.Printf("⏱️ Интервал: %d секунд\n", response.Interval)
	fmt.Printf("⏰ Таймаут: %d секунд\n", response.Timeout)

	if len(response.Tags) > 0 {
		fmt.Printf("🏷️ Теги: %s\n", strings.Join(response.Tags, ", "))
	}

	fmt.Printf("🔧 Статус: ")
	if response.Enabled {
		fmt.Printf("Включена\n")
	} else {
		fmt.Printf("Отключена\n")
	}

	return nil
}

// handleChecksEnable обрабатывает включение проверки
func handleChecksEnable(cmd *cobra.Command, args []string) error {
	return handleChecksToggle(cmd, args, true)
}

// handleChecksDisable обрабатывает отключение проверки
func handleChecksDisable(cmd *cobra.Command, args []string) error {
	return handleChecksToggle(cmd, args, false)
}

// handleChecksToggle обрабатывает включение/отключение проверки
func handleChecksToggle(cmd *cobra.Command, args []string, enabled bool) error {
	checkID := args[0]

	// Валидация UUID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(checkID, "check_id"); err != nil {
		return fmt.Errorf("невалидный ID проверки: %w", err)
	}

	// Загрузка конфигурации CLI
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager and ensure valid token
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	// Get token
	ctx := context.Background()
	if err := authManager.EnsureValidToken(ctx); err != nil {
		return fmt.Errorf("ошибка проверки токена: %w", err)
	}

	token := authManager.GetTokenStore().GetAccessToken()

	// Добавляем токен в контекст
	ctx = context.WithValue(ctx, "access_token", token)

	// Create checks client
	checksClient := client.NewChecksClient(cfg.API.BaseURL, authManager.GetTokenStore())
	defer checksClient.Close()

	// Create update request
	request := &client.Check{
		Enabled: enabled,
		Metadata: map[string]interface{}{
			"enabled": fmt.Sprintf("%t", enabled),
		},
	}

	// Update check
	response, err := checksClient.UpdateCheck(ctx, checkID, request)
	if err != nil {
		return fmt.Errorf("ошибка %s проверки: %w", func() string {
			if enabled {
				return "включения"
			}
			return "отключения"
		}(), err)
	}

	// Display result
	action := "отключена"
	if enabled {
		action = "включена"
	}

	fmt.Printf("✅ Проверка %s успешно!\n", action)
	fmt.Printf("📝 ID: %s\n", response.ID)
	fmt.Printf("🔗 Название: %s\n", response.Name)
	fmt.Printf("🔧 Статус: %s\n", action)

	return nil
}

// handleChecksDelete обрабатывает удаление проверки
func handleChecksDelete(cmd *cobra.Command, args []string) error {
	checkID := args[0]

	// Валидация UUID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(checkID, "check_id"); err != nil {
		return fmt.Errorf("невалидный ID проверки: %w", err)
	}

	// Загрузка конфигурации CLI
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager and ensure valid token
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	// Get token
	ctx := context.Background()
	if err := authManager.EnsureValidToken(ctx); err != nil {
		return fmt.Errorf("ошибка проверки токена: %w", err)
	}

	token := authManager.GetTokenStore().GetAccessToken()

	// Добавляем токен в контекст
	ctx = context.WithValue(ctx, "access_token", token)

	// Create checks client
	checksClient := client.NewChecksClient(cfg.API.BaseURL, authManager.GetTokenStore())
	defer checksClient.Close()

	// Delete check
	err = checksClient.DeleteCheck(ctx, checkID)
	if err != nil {
		return fmt.Errorf("ошибка удаления проверки: %w", err)
	}

	// Display result
	fmt.Printf("✅ Проверка удалена успешно!\n")
	fmt.Printf("📝 ID: %s\n", checkID)
	fmt.Printf("🗑️ Все связанные данные также удалены\n")

	return nil
}
