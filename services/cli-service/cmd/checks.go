package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/cli-service/internal/auth"
	"UptimePingPlatform/services/cli-service/internal/client"
	config "UptimePingPlatform/services/cli-service/internal/config"
)

var checksCmd = &cobra.Command{
	Use:   "checks",
	Short: "Управление проверками",
	Long: `Команды для управления проверками доступности:
запуск, проверка статуса, просмотр истории и списка проверок.`,
}

var checksRunCmd = &cobra.Command{
	Use:   "run [check-id]",
	Short: "Запустить проверку",
	Long:  `Запускает проверку с указанным ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleChecksRun,
}

var checksStatusCmd = &cobra.Command{
	Use:   "status [check-id]",
	Short: "Проверить статус проверки",
	Long:  `Проверяет текущий статус указанной проверки.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleChecksStatus,
}

var checksHistoryCmd = &cobra.Command{
	Use:   "history [check-id]",
	Short: "Показать историю проверок",
	Long:  `Отображает историю выполнения указанной проверки.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleChecksHistory,
}

var checksListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать список проверок",
	Long:  `Отображает все доступные проверки с возможностью фильтрации.`,
	RunE:  handleChecksList,
}

func init() {
	checksCmd.AddCommand(checksRunCmd)
	checksCmd.AddCommand(checksStatusCmd)
	checksCmd.AddCommand(checksHistoryCmd)
	checksCmd.AddCommand(checksListCmd)

	// Checks history flags
	checksHistoryCmd.Flags().IntP("limit", "l", 50, "лимит записей")
	checksHistoryCmd.Flags().IntP("page", "p", 1, "номер страницы")
	checksHistoryCmd.Flags().StringP("format", "f", "table", "формат вывода (table, json)")

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

func handleChecksRun(cmd *cobra.Command, args []string) error {
	checkID := args[0]

	// Load configuration
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := config.LoadConfig(configPath)
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

	// Create logger
	log, err := logger.NewLogger("dev", "info", "cli-service", false)
	if err != nil {
		return fmt.Errorf("ошибка создания логгера: %w", err)
	}

	// Create config client
	var configClient *client.ConfigClient
	if cfg.GRPC.UseGRPC {
		configClient, err = client.NewConfigClientWithGRPC(
			cfg.API.BaseURL,
			cfg.GRPC.SchedulerAddress,
			cfg.GRPC.CoreAddress,
			log,
		)
		if err != nil {
			return fmt.Errorf("ошибка создания gRPC клиента: %w", err)
		}
		defer configClient.Close()
	} else {
		configClient = client.NewConfigClient(cfg.API.BaseURL, log)
	}

	// Run check
	response, err := configClient.RunCheck(ctx, checkID)
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

func handleChecksStatus(cmd *cobra.Command, args []string) error {
	checkID := args[0]

	// Load configuration
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := config.LoadConfig(configPath)
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

	// Create logger
	log, err := logger.NewLogger("dev", "info", "cli-service", false)
	if err != nil {
		return fmt.Errorf("ошибка создания логгера: %w", err)
	}

	// Create config client
	var configClient *client.ConfigClient
	if cfg.GRPC.UseGRPC {
		configClient, err = client.NewConfigClientWithGRPC(
			cfg.API.BaseURL,
			cfg.GRPC.SchedulerAddress,
			cfg.GRPC.CoreAddress,
			log,
		)
		if err != nil {
			return fmt.Errorf("ошибка создания gRPC клиента: %w", err)
		}
		defer configClient.Close()
	} else {
		configClient = client.NewConfigClient(cfg.API.BaseURL, log)
	}

	// Get check status
	response, err := configClient.GetCheckStatus(ctx, checkID)
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

func handleChecksHistory(cmd *cobra.Command, args []string) error {
	checkID := args[0]

	limit, _ := cmd.Flags().GetInt("limit")
	page, _ := cmd.Flags().GetInt("page")
	format, _ := cmd.Flags().GetString("format")

	// Load configuration
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := config.LoadConfig(configPath)
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

	// Create logger
	log, err := logger.NewLogger("dev", "info", "cli-service", false)
	if err != nil {
		return fmt.Errorf("ошибка создания логгера: %w", err)
	}

	// Create config client
	var configClient *client.ConfigClient
	if cfg.GRPC.UseGRPC {
		configClient, err = client.NewConfigClientWithGRPC(
			cfg.API.BaseURL,
			cfg.GRPC.SchedulerAddress,
			cfg.GRPC.CoreAddress,
			log,
		)
		if err != nil {
			return fmt.Errorf("ошибка создания gRPC клиента: %w", err)
		}
		defer configClient.Close()
	} else {
		configClient = client.NewConfigClient(cfg.API.BaseURL, log)
	}

	// Get check history
	response, err := configClient.GetCheckHistory(ctx, checkID, page, limit)
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
				execution.ExecutionID,
				execution.Status,
				execution.Message,
				execution.Duration,
				execution.StartedAt.Format(time.RFC3339),
				execution.CompletedAt.Format(time.RFC3339))
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

func handleChecksList(cmd *cobra.Command, args []string) error {
	tags, _ := cmd.Flags().GetStringSlice("tags")
	enabled, _ := cmd.Flags().GetBool("enabled")
	page, _ := cmd.Flags().GetInt("page")
	limit, _ := cmd.Flags().GetInt("limit")
	format, _ := cmd.Flags().GetString("format")

	var enabledPtr *bool
	if cmd.Flags().Changed("enabled") {
		enabledPtr = &enabled
	}

	// Load configuration
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := config.LoadConfig(configPath)
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

	// Create logger
	log, err := logger.NewLogger("dev", "info", "cli-service", false)
	if err != nil {
		return fmt.Errorf("ошибка создания логгера: %w", err)
	}

	// Create config client
	var configClient *client.ConfigClient
	if cfg.GRPC.UseGRPC {
		configClient, err = client.NewConfigClientWithGRPC(
			cfg.API.BaseURL,
			cfg.GRPC.SchedulerAddress,
			cfg.GRPC.CoreAddress,
			log,
		)
		if err != nil {
			return fmt.Errorf("ошибка создания gRPC клиента: %w", err)
		}
		defer configClient.Close()
	} else {
		configClient = client.NewConfigClient(cfg.API.BaseURL, log)
	}

	// Get checks list
	response, err := configClient.ListChecks(ctx, tags, enabledPtr, page, limit)
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
			fmt.Printf(`  {"id": "%s", "name": "%s", "type": "%s", "target": "%s", "interval": %d, "timeout": %d, "enabled": %t, "tags": [%s], "created_at": "%s"}`,
				check.ID,
				check.Name,
				check.Type,
				check.Target,
				check.Interval,
				check.Timeout,
				check.Enabled,
				strings.Join(check.Tags, ", "),
				check.CreatedAt.Format(time.RFC3339))
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

			tags := strings.Join(check.Tags, ", ")
			if tags == "" {
				tags = "-"
			}

			fmt.Printf("%-20s %-25s %-10s %-30s %-10s %-10s %s\n", id, name, check.Type, target, interval, timeout, tags)
		}
	}

	fmt.Printf("\n📊 Всего проверок: %d\n", response.Total)
	fmt.Printf("📄 Страница: %d из %d\n", page, (response.Total+limit-1)/limit)

	return nil
}
