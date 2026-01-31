package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	cliConfig "UptimePingPlatform/services/cli-service/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Управление конфигурацией",
	Long: `Команды для управления конфигурацией системы:
просмотр, создание, обновление и удаление конфигураций.`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Инициализировать конфигурацию",
	Long:  "Создать файл конфигурации с настройками по умолчанию",
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Просмотреть конфигурацию",
	Long:  "Показать текущую конфигурацию",
}

var configCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Создать новую проверку",
	Long:  `Создает новую проверку доступности с указанными параметрами.`,
	RunE:  handleConfigCreate,
}

var configGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Получить конфигурацию проверки",
	Long:  `Получает детальную конфигурацию проверки по ее ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleConfigGet,
}

var configUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Обновить проверку",
	Long:  `Обновляет существующую проверку с указанными параметрами.`,
	Args:  cobra.ExactArgs(1),
	RunE:  handleConfigUpdate,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Список проверок",
	Long:  `Отображает список всех проверок с возможностью фильтрации.`,
	RunE:  handleConfigList,
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configCreateCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configUpdateCmd)
	configCmd.AddCommand(configListCmd)

	// Config init flags
	configInitCmd.Flags().StringP("path", "p", "", "путь для создания конфигурации")
	configInitCmd.Flags().BoolP("force", "f", false, "перезаписать существующий файл")

	// Config view flags
	configViewCmd.Flags().StringP("format", "f", "yaml", "формат вывода (yaml, json)")
	configViewCmd.Flags().BoolP("show-secrets", "x", false, "показать секретные данные")

	// Config create flags
	configCreateCmd.Flags().StringP("name", "n", "", "название проверки")
	configCreateCmd.Flags().StringP("type", "y", "http", "тип проверки (http, tcp, ping, grpc, graphql)")
	configCreateCmd.Flags().StringP("target", "t", "", "цель проверки")
	configCreateCmd.Flags().IntP("interval", "i", 60, "интервал в секундах")
	configCreateCmd.Flags().IntP("timeout", "m", 10, "таймаут в секундах")
	configCreateCmd.Flags().StringSliceP("tags", "g", []string{}, "теги")

	// Config get flags
	configGetCmd.Flags().StringP("format", "f", "yaml", "формат вывода (yaml, json)")

	// Config update flags
	configUpdateCmd.Flags().StringP("name", "n", "", "новое название")
	configUpdateCmd.Flags().StringP("type", "y", "", "новый тип")
	configUpdateCmd.Flags().StringP("target", "t", "", "новая цель")
	configUpdateCmd.Flags().IntP("interval", "i", 0, "новый интервал в секундах")
	configUpdateCmd.Flags().IntP("timeout", "m", 0, "новый таймаут в секундах")
	configUpdateCmd.Flags().BoolP("enabled", "e", false, "статус проверки")
	configUpdateCmd.Flags().StringSliceP("tags", "g", []string{}, "новые теги")

	// Config list flags
	configListCmd.Flags().StringSliceP("tags", "t", []string{}, "фильтр по тегам")
	configListCmd.Flags().BoolP("enabled", "e", false, "фильтр по статусу")
	configListCmd.Flags().IntP("page", "p", 1, "номер страницы")
	configListCmd.Flags().IntP("limit", "l", 20, "лимит записей на странице")
	configListCmd.Flags().StringP("format", "f", "table", "формат вывода (table, json)")

	// Set run functions
	configInitCmd.RunE = handleConfigInit
	configViewCmd.RunE = handleConfigView
	configCreateCmd.RunE = handleConfigCreate
	configGetCmd.RunE = handleConfigGet
	configUpdateCmd.RunE = handleConfigUpdate
	configListCmd.RunE = handleConfigList
}

func GetConfigCmd() *cobra.Command {
	return configCmd
}

func handleConfigInit(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	path, _ := cmd.Flags().GetString("path")

	// Initialize configuration using internal config
	if path != "" {
		// Use custom path
		configPath := path
		if !force {
			if _, err := os.Stat(configPath); err == nil {
				return fmt.Errorf("файл конфигурации уже существует. Используйте --force для перезаписи")
			}
		}

		cfg := cliConfig.DefaultConfig()
		cfg.Path = configPath
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("ошибка сохранения конфигурации: %w", err)
		}

		fmt.Printf("✅ Конфигурация успешно инициализирована!\n")
		fmt.Printf("📁 Файл конфигурации: %s\n", configPath)
	} else {
		// Use default path
		configPath, err := cliConfig.GetConfigPath()
		if err != nil {
			return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
		}

		if !force {
			if _, err := os.Stat(configPath); err == nil {
				return fmt.Errorf("файл конфигурации уже существует. Используйте --force для перезаписи")
			}
		}

		_, err = cliConfig.InitConfig()
		if err != nil {
			return fmt.Errorf("ошибка инициализации конфигурации: %w", err)
		}

		fmt.Printf("✅ Конфигурация успешно инициализирована!\n")
		fmt.Printf("📁 Файл конфигурации: %s\n", configPath)
	}

	fmt.Printf("💡 Отредактируйте файл для изменения настроек\n")
	return nil
}

func handleConfigView(cmd *cobra.Command, args []string) error {
	// Загрузка конфигурации CLI - используем внутреннюю систему
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	format, _ := cmd.Flags().GetString("format")
	showSecrets, _ := cmd.Flags().GetBool("show-secrets")

	switch format {
	case "json":
		// Convert to JSON - simplified implementation
		fmt.Printf("{\n")
		fmt.Printf("  \"api\": {\n")
		fmt.Printf("    \"base_url\": \"%s\"\n", cfg.API.BaseURL)
		fmt.Printf("  },\n")
		fmt.Printf("  \"auth\": {\n")
		fmt.Printf("    \"token_expiry\": %d,\n", cfg.Auth.TokenExpiry)
		fmt.Printf("    \"refresh_threshold\": %d\n", cfg.Auth.RefreshThreshold)
		fmt.Printf("  }\n")
		fmt.Printf("}\n")
	case "yaml":
		// Convert to YAML - simplified implementation
		fmt.Printf("api:\n")
		fmt.Printf("  base_url: %s\n", cfg.API.BaseURL)
		fmt.Printf("auth:\n")
		fmt.Printf("  token_expiry: %d\n", cfg.Auth.TokenExpiry)
		fmt.Printf("  refresh_threshold: %d\n", cfg.Auth.RefreshThreshold)
	default:
		// Table format
		fmt.Printf("📋 Текущая конфигурация:\n")
		fmt.Printf("🔗 API Base URL: %s\n", cfg.API.BaseURL)
		fmt.Printf("🔐 Token Expiry: %d секунд\n", cfg.Auth.TokenExpiry)
		fmt.Printf("🔄 Refresh Threshold: %d секунд\n", cfg.Auth.RefreshThreshold)
		if showSecrets {
			fmt.Printf("🔑 Encryption Key: %s\n", "********")
		}
	}

	return nil
}
