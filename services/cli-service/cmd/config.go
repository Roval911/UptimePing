package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	"UptimePingPlatform/pkg/errors"
	// configv1 "UptimePingPlatform/proto/config/v1"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Управление конфигурацией",
	Long: `Команды для управления конфигурацией системы:
просмотр, создание, обновление и удаление конфигураций.`,
}

// configGetCmd represents the config get command
var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Получить значение конфигурации",
	Long:  `Получает значение указанного ключа конфигурации или всю конфигурацию.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleConfigGet(cmd, args)
	},
}

// configCreateCmd represents the config create command
var configCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Создать новую конфигурацию",
	Long:  `Создает новую конфигурацию с указанными параметрами.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleConfigCreate(cmd, args)
	},
}

// configUpdateCmd represents the config update command
var configUpdateCmd = &cobra.Command{
	Use:   "update [key] [value]",
	Short: "Обновить значение конфигурации",
	Long:  `Обновляет значение указанного ключа конфигурации.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleConfigUpdate(cmd, args)
	},
}

// configDeleteCmd represents the config delete command
var configDeleteCmd = &cobra.Command{
	Use:   "delete [key]",
	Short: "Удалить конфигурацию",
	Long:  `Удаляет указанный ключ конфигурации.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleConfigDelete(cmd, args)
	},
}

// configListCmd represents the config list command
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать список конфигураций",
	Long:  `Отображает все доступные конфигурации.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleConfigList(cmd, args)
	},
}

// configInitCmd represents the config init command
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Инициализировать конфигурацию",
	Long:  `Создает файл конфигурации с настройками по умолчанию.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleConfigInit(cmd, args)
	},
}

// configViewCmd represents the config view command
var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Просмотреть текущую конфигурацию",
	Long:  `Отображает текущую конфигурацию в указанном формате.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleConfigView(cmd, args)
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configCreateCmd)
	configCmd.AddCommand(configUpdateCmd)
	configCmd.AddCommand(configDeleteCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configViewCmd)

	// Config create flags
	configCreateCmd.Flags().StringP("name", "n", "", "название конфигурации")
	configCreateCmd.Flags().StringP("description", "d", "", "описание конфигурации")
	configCreateCmd.Flags().StringP("format", "f", "yaml", "формат конфигурации (yaml, json)")

	// Config view flags
	configViewCmd.Flags().StringP("format", "f", "yaml", "формат вывода (yaml, json, table)")
	configViewCmd.Flags().BoolP("show-secrets", "s", false, "показать секретные данные")

	// Config init flags
	configInitCmd.Flags().StringP("path", "p", "", "путь для создания конфигурации")
	configInitCmd.Flags().BoolP("force", "f", false, "перезаписать существующий файл")
}

// getConfigClient creates a gRPC client for config service
func getConfigClient() (*MockConfigClient, *grpc.ClientConn, error) {
	return getMockConfigClient()
}

func handleConfigGet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// Show all configuration
		return showAllConfig(cmd)
	}

	key := args[0]
	
	// Get specific key from viper
	value := viper.Get(key)
	if value == nil {
		return errors.New(errors.ErrNotFound, fmt.Sprintf("config key '%s' not found", key))
	}

	outputFormat := viper.GetString("output")
	switch outputFormat {
	case "json":
		fmt.Printf(`{"%s": %v}`, key, value)
	default:
		fmt.Printf("%s: %v\n", key, value)
	}

	return nil
}

func handleConfigCreate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	format, _ := cmd.Flags().GetString("format")

	if name == "" {
		return errors.New(errors.ErrValidation, "name is required")
	}

	client, conn, err := getConfigClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Format      string `json:"format"`
	}{
		Name:        name,
		Description: description,
		Format:      format,
	}

	resp, err := client.CreateConfig(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	configResp := resp.(*CreateConfigResponse)

	fmt.Printf("✅ Configuration '%s' created successfully\n", name)
	fmt.Printf("Config ID: %s\n", configResp.ConfigId)
	if viper.GetBool("verbose") {
		fmt.Printf("Created at: %s\n", configResp.CreatedAt.Format(time.RFC3339))
	}

	return nil
}

func handleConfigUpdate(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// Update viper configuration
	viper.Set(key, value)

	// Save to config file if specified
	if configFile := viper.ConfigFileUsed(); configFile != "" {
		if err := viper.WriteConfig(); err != nil {
			return handleError(err, cmd)
		}
	}

	fmt.Printf("✅ Configuration key '%s' updated to '%s'\n", key, value)
	return nil
}

func handleConfigDelete(cmd *cobra.Command, args []string) error {
	key := args[0]

	// Check if key exists
	if !viper.IsSet(key) {
		return errors.New(errors.ErrNotFound, fmt.Sprintf("config key '%s' not found", key))
	}

	// Delete from viper
	viper.Set(key, nil)

	// Save to config file if specified
	if configFile := viper.ConfigFileUsed(); configFile != "" {
		if err := viper.WriteConfig(); err != nil {
			return handleError(err, cmd)
		}
	}

	fmt.Printf("✅ Configuration key '%s' deleted\n", key)
	return nil
}

func handleConfigList(cmd *cobra.Command, args []string) error {
	client, conn, err := getConfigClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct{}{}
	resp, err := client.ListConfigs(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	configsResp := resp.(*ListConfigsResponse)

	if len(configsResp.Configs) == 0 {
		fmt.Println("No configurations found")
		return nil
	}

	outputFormat := viper.GetString("output")
	switch outputFormat {
	case "json":
		fmt.Println("[")
		for i, config := range configsResp.Configs {
			if i > 0 {
				fmt.Println(",")
			}
			fmt.Printf(`  {"id": "%s", "name": "%s", "description": "%s", "format": "%s"}`,
				config.ConfigId, config.Name, config.Description, config.Format)
		}
		fmt.Println("\n]")
	default:
		fmt.Println("Configurations:")
		for _, config := range configsResp.Configs {
			fmt.Printf("  %s: %s (%s)\n", config.ConfigId, config.Name, config.Format)
			if config.Description != "" {
				fmt.Printf("    Description: %s\n", config.Description)
			}
		}
	}

	return nil
}

func handleConfigInit(cmd *cobra.Command, args []string) error {
	path, _ := cmd.Flags().GetString("path")
	force, _ := cmd.Flags().GetBool("force")

	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return handleError(err, cmd)
		}
		path = filepath.Join(home, ".uptimeping.yaml")
	}

	// Check if file exists
	if _, err := os.Stat(path); err == nil && !force {
		return errors.New(errors.ErrConflict, fmt.Sprintf("config file already exists: %s (use --force to overwrite)", path))
	}

	// Create default configuration
	defaultConfig := map[string]interface{}{
		"server":      "localhost:8080",
		"output":      "table",
		"verbose":     false,
		"debug":       false,
		"auth": map[string]interface{}{
			"token": "",
		},
		"services": map[string]interface{}{
			"auth": map[string]interface{}{
				"address": "localhost:50051",
			},
			"core": map[string]interface{}{
				"address": "localhost:50052",
			},
			"scheduler": map[string]interface{}{
				"address": "localhost:50053",
			},
		},
	}

	// Set default values in viper
	for key, value := range defaultConfig {
		viper.Set(key, value)
	}

	// Create config file
	if err := viper.WriteConfigAs(path); err != nil {
		return handleError(err, cmd)
	}

	fmt.Printf("✅ Configuration initialized at: %s\n", path)
	fmt.Println("You can now edit the file to customize your settings.")
	
	return nil
}

func handleConfigView(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	showSecrets, _ := cmd.Flags().GetBool("show-secrets")

	// Get current configuration
	allSettings := viper.AllSettings()

	// Filter secrets if not requested
	if !showSecrets {
		allSettings = filterSecrets(allSettings)
	}

	switch format {
	case "json":
		jsonData, err := json.MarshalIndent(&allSettings, "", "  ")
		if err != nil {
			return handleError(err, cmd)
		}
		fmt.Println(string(jsonData))
	case "table":
		fmt.Println("Current Configuration:")
		printConfigTable(allSettings)
	default: // yaml
		yamlData, err := yaml.Marshal(&allSettings)
		if err != nil {
			return handleError(err, cmd)
		}
		fmt.Println(string(yamlData))
	}

	return nil
}

func showAllConfig(cmd *cobra.Command) error {
	allSettings := viper.AllSettings()
	
	outputFormat := viper.GetString("output")
	switch outputFormat {
	case "json":
		jsonData, err := json.MarshalIndent(&allSettings, "", "  ")
		if err != nil {
			return handleError(err, cmd)
		}
		fmt.Println(string(jsonData))
	default:
		fmt.Println("Current Configuration:")
		printConfigTable(allSettings)
	}

	return nil
}

func printConfigTable(settings map[string]interface{}) {
	for key, value := range settings {
		fmt.Printf("  %-20s: %v\n", key, value)
	}
}

func filterSecrets(settings map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})
	
	for key, value := range settings {
		if containsSecret(key) {
			filtered[key] = "***HIDDEN***"
		} else if nested, ok := value.(map[string]interface{}); ok {
			filtered[key] = filterSecrets(nested)
		} else {
			filtered[key] = value
		}
	}
	
	return filtered
}

func containsSecret(key string) bool {
	secretKeys := []string{"password", "token", "secret", "key", "auth"}
	lowerKey := lower(key)
	
	for _, secret := range secretKeys {
		if contains(lowerKey, secret) {
			return true
		}
	}
	
	return false
}

func lower(s string) string {
	// Simple lowercase implementation
	result := make([]rune, len(s))
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + ('a' - 'A')
		} else {
			result[i] = r
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || 
		   len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
