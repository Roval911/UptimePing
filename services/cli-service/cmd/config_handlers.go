package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/cli-service/internal/auth"
	"UptimePingPlatform/services/cli-service/internal/client"
	cliConfig "UptimePingPlatform/services/cli-service/internal/config"
)

func handleConfigCreate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	checkType, _ := cmd.Flags().GetString("type")
	target, _ := cmd.Flags().GetString("target")
	interval, _ := cmd.Flags().GetInt("interval")
	timeout, _ := cmd.Flags().GetInt("timeout")
	tags, _ := cmd.Flags().GetStringSlice("tags")

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

	// Create logger
	log, err := logger.NewLogger("dev", "info", "cli-service", false)
	if err != nil {
		return fmt.Errorf("ошибка создания логгера: %w", err)
	}

	// Validate input
	validator := &validation.Validator{}
	if err := validator.ValidateRequiredFields(map[string]interface{}{
		"name":   name,
		"type":   checkType,
		"target": target,
	}, map[string]string{}); err != nil {
		return errors.Wrap(err, errors.ErrValidation, "обязательные поля: name, type, target")
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

	// Create check request
	req := &client.CheckCreateRequest{
		Name:     name,
		Type:     checkType,
		Target:   target,
		Interval: interval,
		Timeout:  timeout,
		Tags:     tags,
		Metadata: map[string]string{
			"created_by": "cli",
		},
	}

	// Create check
	check, err := configClient.CreateCheck(ctx, req)
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
	fmt.Printf("🏷️ Теги: %s\n", strings.Join(check.Tags, ", "))
	fmt.Printf("📅 Создана: %s\n", check.CreatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

func handleConfigGet(cmd *cobra.Command, args []string) error {
	checkID := args[0]
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

	// Get check
	check, err := configClient.GetCheck(ctx, checkID)
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
		fmt.Printf("  \"tags\": [%s],\n", strings.Join(check.Tags, ", "))
		fmt.Printf("  \"metadata\": {\n")
		for k, v := range check.Metadata {
			fmt.Printf("    \"%s\": \"%s\",\n", k, v)
		}
		fmt.Printf("  },\n")
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
		fmt.Printf("🏷️ Теги: %s\n", strings.Join(check.Tags, ", "))
		fmt.Printf("📅 Создана: %s\n", check.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("🔄 Обновлена: %s\n", check.UpdatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("📋 Метаданные:\n")
		for k, v := range check.Metadata {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	return nil
}

func handleConfigUpdate(cmd *cobra.Command, args []string) error {
	checkID := args[0]
	name, _ := cmd.Flags().GetString("name")
	checkType, _ := cmd.Flags().GetString("type")
	target, _ := cmd.Flags().GetString("target")
	interval, _ := cmd.Flags().GetInt("interval")
	timeout, _ := cmd.Flags().GetInt("timeout")
	enabled, _ := cmd.Flags().GetBool("enabled")
	tags, _ := cmd.Flags().GetStringSlice("tags")

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

	// Create update request
	req := &client.CheckUpdateRequest{}
	if name != "" {
		req.Name = &name
	}
	if checkType != "" {
		req.Type = &checkType
	}
	if target != "" {
		req.Target = &target
	}
	if interval > 0 {
		req.Interval = &interval
	}
	if timeout > 0 {
		req.Timeout = &timeout
	}
	if cmd.Flags().Changed("enabled") {
		req.Enabled = &enabled
	}
	if len(tags) > 0 {
		req.Tags = tags
	}
	req.Metadata = map[string]string{
		"updated_by": "cli",
	}

	// Update check
	check, err := configClient.UpdateCheck(ctx, checkID, req)
	if err != nil {
		return fmt.Errorf("ошибка обновления проверки: %w", err)
	}

	fmt.Printf("✅ Проверка обновлена успешно!\n")
	fmt.Printf("🔍 ID: %s\n", check.ID)
	fmt.Printf("📝 Название: %s\n", check.Name)
	fmt.Printf("🔧 Тип: %s\n", check.Type)
	fmt.Printf("🎯 Цель: %s\n", check.Target)
	fmt.Printf("⏱️ Интервал: %d секунд\n", check.Interval)
	fmt.Printf("⏰ Таймаут: %d секунд\n", check.Timeout)
	fmt.Printf("🏷️ Статус: %t\n", check.Enabled)
	fmt.Printf("🏷️ Теги: %s\n", strings.Join(check.Tags, ", "))
	fmt.Printf("🔄 Обновлена: %s\n", check.UpdatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

func handleConfigList(cmd *cobra.Command, args []string) error {
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
