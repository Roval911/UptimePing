package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"UptimePingPlatform/pkg/logger"
	cliClient "UptimePingPlatform/services/cli-service/internal/client"
	cliConfig "UptimePingPlatform/services/cli-service/internal/config"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "test-uptimeping",
		Short: "Тестовая версия uptimeping CLI без аутентификации",
		Long:  `Тестовая CLI утилита для демонстрации функционала без необходимости аутентификации`,
	}

	// Добавляем команды config
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Управление конфигурацией проверок",
	}

	// Команда create
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Создать новую проверку",
		RunE:  handleCreate,
	}
	createCmd.Flags().StringP("name", "n", "", "название проверки")
	createCmd.Flags().StringP("type", "t", "http", "тип проверки")
	createCmd.Flags().StringP("target", "u", "", "цель проверки")
	createCmd.Flags().IntP("interval", "i", 60, "интервал в секундах")
	createCmd.Flags().IntP("timeout", "m", 10, "таймаут в секундах")
	createCmd.Flags().StringSliceP("tags", "g", []string{}, "теги")

	// Команда list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Показать список проверок",
		RunE:  handleList,
	}
	listCmd.Flags().StringSliceP("tags", "t", []string{}, "фильтр по тегам")
	listCmd.Flags().BoolP("enabled", "e", false, "фильтр по статусу")
	listCmd.Flags().IntP("page", "p", 1, "номер страницы")
	listCmd.Flags().IntP("limit", "l", 20, "лимит записей")
	listCmd.Flags().StringP("format", "f", "table", "формат вывода")

	// Команда get
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Получить проверку по ID",
		Args:  cobra.ExactArgs(1),
		RunE:  handleGet,
	}
	getCmd.Flags().StringP("format", "f", "yaml", "формат вывода")

	// Команда run
	runCmd := &cobra.Command{
		Use:   "run [id]",
		Short: "Запустить проверку",
		Args:  cobra.ExactArgs(1),
		RunE:  handleRun,
	}

	// Команда status
	statusCmd := &cobra.Command{
		Use:   "status [id]",
		Short: "Получить статус проверки",
		Args:  cobra.ExactArgs(1),
		RunE:  handleStatus,
	}

	// Команда history
	historyCmd := &cobra.Command{
		Use:   "history [id]",
		Short: "Получить историю проверок",
		Args:  cobra.ExactArgs(1),
		RunE:  handleHistory,
	}
	historyCmd.Flags().IntP("limit", "l", 50, "лимит записей")
	historyCmd.Flags().IntP("page", "p", 1, "номер страницы")
	historyCmd.Flags().StringP("format", "f", "table", "формат вывода")

	// Собираем команды
	configCmd.AddCommand(createCmd)
	configCmd.AddCommand(listCmd)
	configCmd.AddCommand(getCmd)

	checksCmd := &cobra.Command{
		Use:   "checks",
		Short: "Управление проверками",
	}
	checksCmd.AddCommand(runCmd)
	checksCmd.AddCommand(statusCmd)
	checksCmd.AddCommand(historyCmd)

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(checksCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func createTestClient() (*cliClient.ConfigClient, error) {
	// Создаем логгер
	log, err := logger.NewLogger("dev", "info", "test-cli", false)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания логгера: %w", err)
	}

	// Создаем тестовую конфигурацию с учетом переменных окружения
	config, err := cliConfig.LoadTestConfig()
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Создаем клиент без gRPC (используем HTTP fallback)
	configClient := cliClient.NewConfigClient(config.API.BaseURL, log)

	return configClient, nil
}

func handleCreate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	checkType, _ := cmd.Flags().GetString("type")
	target, _ := cmd.Flags().GetString("target")
	interval, _ := cmd.Flags().GetInt("interval")
	timeout, _ := cmd.Flags().GetInt("timeout")
	tags, _ := cmd.Flags().GetStringSlice("tags")

	client, err := createTestClient()
	if err != nil {
		return fmt.Errorf("ошибка создания клиента: %w", err)
	}

	req := &cliClient.CheckCreateRequest{
		Name:     name,
		Type:     checkType,
		Target:   target,
		Interval: interval,
		Timeout:  timeout,
		Tags:     tags,
		Metadata: map[string]string{
			"created_by": "test-cli",
		},
	}

	ctx := context.Background()
	check, err := client.CreateCheck(ctx, req)
	if err != nil {
		return fmt.Errorf("ошибка создания проверки: %w", err)
	}

	fmt.Printf("✅ Проверка создана успешно!\n")
	fmt.Printf("🔍 ID: %s\n", check.ID)
	fmt.Printf("📝 Название: %s\n", check.Name)
	fmt.Printf("🔧 Тип: %s\n", check.Type)
	fmt.Printf("🎯 Цель: %s\n", check.Target)
	fmt.Printf("⏱️ Интервал: %d секунд\n", check.Interval)
	fmt.Printf("⏰ Таймаут: %d секунд\n", check.Timeout)
	fmt.Printf("🏷️ Теги: %s\n", fmt.Sprintf("%v", check.Tags))

	return nil
}

func handleList(cmd *cobra.Command, args []string) error {
	tags, _ := cmd.Flags().GetStringSlice("tags")
	enabled, _ := cmd.Flags().GetBool("enabled")
	page, _ := cmd.Flags().GetInt("page")
	limit, _ := cmd.Flags().GetInt("limit")
	format, _ := cmd.Flags().GetString("format")

	client, err := createTestClient()
	if err != nil {
		return fmt.Errorf("ошибка создания клиента: %w", err)
	}

	var enabledPtr *bool
	if cmd.Flags().Changed("enabled") {
		enabledPtr = &enabled
	}

	ctx := context.Background()
	response, err := client.ListChecks(ctx, tags, enabledPtr, page, limit)
	if err != nil {
		return fmt.Errorf("ошибка получения списка проверок: %w", err)
	}

	if len(response.Checks) == 0 {
		fmt.Printf("📭 Проверки не найдены\n")
		return nil
	}

	switch format {
	case "json":
		fmt.Println("[")
		for i, check := range response.Checks {
			if i > 0 {
				fmt.Println(",")
			}
			fmt.Printf(`  {"id": "%s", "name": "%s", "type": "%s", "target": "%s", "interval": %d, "timeout": %d, "enabled": %t}`,
				check.ID, check.Name, check.Type, check.Target, check.Interval, check.Timeout, check.Enabled)
		}
		fmt.Println("\n]")
	default:
		fmt.Printf("📋 Список проверок (страница %d):\n", page)
		fmt.Printf("%-20s %-25s %-10s %-30s %-10s %-10s %s\n", "🔍 ID", "📝 Название", "🔧 Тип", "🎯 Цель", "⏱️ Интервал", "⏰ Таймаут", "🏷️ Теги")
		fmt.Println(strings.Repeat("-", 120))

		for _, check := range response.Checks {
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

			tagsStr := fmt.Sprintf("%v", check.Tags)
			if tagsStr == "[]" {
				tagsStr = "-"
			}

			fmt.Printf("%-20s %-25s %-10s %-30s %-10s %-10s %s\n", id, name, check.Type, target, interval, timeout, tagsStr)
		}
	}

	fmt.Printf("\n📊 Всего проверок: %d\n", response.Total)
	fmt.Printf("📄 Страница: %d из %d\n", page, (response.Total+limit-1)/limit)

	return nil
}

func handleGet(cmd *cobra.Command, args []string) error {
	checkID := args[0]
	format, _ := cmd.Flags().GetString("format")

	client, err := createTestClient()
	if err != nil {
		return fmt.Errorf("ошибка создания клиента: %w", err)
	}

	ctx := context.Background()
	check, err := client.GetCheck(ctx, checkID)
	if err != nil {
		return fmt.Errorf("ошибка получения проверки: %w", err)
	}

	switch format {
	case "json":
		fmt.Printf("{\n")
		fmt.Printf("  \"id\": \"%s\",\n", check.ID)
		fmt.Printf("  \"name\": \"%s\",\n", check.Name)
		fmt.Printf("  \"type\": \"%s\",\n", check.Type)
		fmt.Printf("  \"target\": \"%s\",\n", check.Target)
		fmt.Printf("  \"interval\": %d,\n", check.Interval)
		fmt.Printf("  \"timeout\": %d,\n", check.Timeout)
		fmt.Printf("  \"enabled\": %t,\n", check.Enabled)
		fmt.Printf("  \"tags\": %v,\n", check.Tags)
		fmt.Printf("  \"created_at\": \"%s\",\n", check.CreatedAt.Format(time.RFC3339))
		fmt.Printf("  \"updated_at\": \"%s\"\n", check.UpdatedAt.Format(time.RFC3339))
		fmt.Printf("}\n")
	default:
		fmt.Printf("📋 Конфигурация проверки: %s\n", checkID)
		fmt.Printf("🔍 ID: %s\n", check.ID)
		fmt.Printf("📝 Название: %s\n", check.Name)
		fmt.Printf("🔧 Тип: %s\n", check.Type)
		fmt.Printf("🎯 Цель: %s\n", check.Target)
		fmt.Printf("⏱️ Интервал: %d секунд\n", check.Interval)
		fmt.Printf("⏰ Таймаут: %d секунд\n", check.Timeout)
		fmt.Printf("🏷️ Статус: %t\n", check.Enabled)
		fmt.Printf("🏷️ Теги: %v\n", check.Tags)
		fmt.Printf("📅 Создана: %s\n", check.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("🔄 Обновлена: %s\n", check.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func handleRun(cmd *cobra.Command, args []string) error {
	checkID := args[0]

	client, err := createTestClient()
	if err != nil {
		return fmt.Errorf("ошибка создания клиента: %w", err)
	}

	ctx := context.Background()
	response, err := client.RunCheck(ctx, checkID)
	if err != nil {
		return fmt.Errorf("ошибка запуска проверки: %w", err)
	}

	fmt.Printf("✅ Проверка запущена!\n")
	fmt.Printf("🔍 ID проверки: %s\n", checkID)
	fmt.Printf("🆔 ID выполнения: %s\n", response.ExecutionID)
	fmt.Printf("📊 Статус: %s\n", response.Status)
	fmt.Printf("🕐 Время запуска: %s\n", response.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("💬 Сообщение: %s\n", response.Message)

	return nil
}

func handleStatus(cmd *cobra.Command, args []string) error {
	checkID := args[0]

	client, err := createTestClient()
	if err != nil {
		return fmt.Errorf("ошибка создания клиента: %w", err)
	}

	ctx := context.Background()
	response, err := client.GetCheckStatus(ctx, checkID)
	if err != nil {
		return fmt.Errorf("ошибка получения статуса: %w", err)
	}

	fmt.Printf("📊 Статус проверки: %s\n", checkID)
	fmt.Printf("🔍 ID: %s\n", response.CheckID)
	fmt.Printf("📈 Текущий статус: %s\n", response.Status)
	fmt.Printf("🕐 Последний запуск: %s\n", response.LastRun.Format("2006-01-02 15:04:05"))
	fmt.Printf("⏰ Следующий запуск: %s\n", response.NextRun.Format("2006-01-02 15:04:05"))
	fmt.Printf("📋 Последний статус: %s\n", response.LastStatus)
	fmt.Printf("💬 Последнее сообщение: %s\n", response.LastMessage)
	fmt.Printf("🔄 Выполняется: %t\n", response.IsRunning)

	return nil
}

func handleHistory(cmd *cobra.Command, args []string) error {
	checkID := args[0]
	limit, _ := cmd.Flags().GetInt("limit")
	page, _ := cmd.Flags().GetInt("page")
	format, _ := cmd.Flags().GetString("format")

	client, err := createTestClient()
	if err != nil {
		return fmt.Errorf("ошибка создания клиента: %w", err)
	}

	ctx := context.Background()
	response, err := client.GetCheckHistory(ctx, checkID, page, limit)
	if err != nil {
		return fmt.Errorf("ошибка получения истории: %w", err)
	}

	if len(response.Executions) == 0 {
		fmt.Printf("📭 История проверок для %s пуста\n", checkID)
		return nil
	}

	switch format {
	case "json":
		fmt.Println("[")
		for i, execution := range response.Executions {
			if i > 0 {
				fmt.Println(",")
			}
			fmt.Printf(`  {"execution_id": "%s", "status": "%s", "message": "%s", "duration": %d, "started_at": "%s", "completed_at": "%s"}`,
				execution.ExecutionID, execution.Status, execution.Message, execution.Duration,
				execution.StartedAt.Format(time.RFC3339), execution.CompletedAt.Format(time.RFC3339))
		}
		fmt.Println("\n]")
	default:
		fmt.Printf("📋 История проверок для %s (страница %d):\n", checkID, page)
		fmt.Printf("%-20s %-10s %-15s %s\n", "🕐 Время", "📊 Статус", "⏱️ Длительность", "💬 Сообщение")
		fmt.Println(strings.Repeat("-", 80))

		for _, execution := range response.Executions {
			timestamp := execution.StartedAt.Format("2006-01-02 15:04:05")
			status := execution.Status
			duration := fmt.Sprintf("%dms", execution.Duration)
			message := execution.Message

			if len(message) > 50 {
				message = message[:47] + "..."
			}

			// Добавляем эмодзи для статуса
			switch status {
			case "success":
				status = "✅ " + status
			case "failed":
				status = "❌ " + status
			case "timeout":
				status = "⏰ " + status
			default:
				status = "⏳️ " + status
			}

			fmt.Printf("%-20s %-15s %-15s %s\n", timestamp, status, duration, message)
		}
	}

	fmt.Printf("\n📊 Всего записей: %d\n", response.Total)
	fmt.Printf("📄 Страница: %d из %d\n", page, (response.Total+limit-1)/limit)

	return nil
}
