package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	cliConfig "UptimePingPlatform/services/cli-service/internal/config"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export [format]",
	Short: "Экспорт конфигурации",
	Long: `Экспортирует текущую конфигурацию CLI в различных форматах.

Поддерживаемые форматы:
- json: JSON формат
- yaml: YAML формат  
- env: Переменные окружения
- docker: Docker Compose переменные

Примеры:
  uptimeping export json
  uptimeping export yaml
  uptimeping export env > .env
  uptimeping export docker > docker-compose.env`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleExport(cmd, args)
	},
}

var (
	outputFile string
	contextName string
	include    []string
	exclude    []string
)

func init() {
	rootCmd.AddCommand(exportCmd)
	
	exportCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Файл для вывода (по умолчанию stdout)")
	exportCmd.Flags().StringVarP(&contextName, "context", "x", "default", "Контекст для экспорта")
	exportCmd.Flags().StringSliceVar(&include, "include", []string{}, "Включить только указанные секции")
	exportCmd.Flags().StringSliceVar(&exclude, "exclude", []string{}, "Исключить указанные секции")
}

func handleExport(cmd *cobra.Command, args []string) error {
	// Определяем формат
	format := "yaml"
	if len(args) > 0 {
		format = args[0]
	}

	// Загружаем конфигурацию
	config, err := cliConfig.LoadConfig("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Фильтруем конфигурацию
	filteredConfig := filterConfig(config, include, exclude)

	// Экспортируем в нужном формате
	var output string
	switch strings.ToLower(format) {
	case "json":
		output, err = exportConfigJSON(filteredConfig)
	case "yaml":
		output, err = exportConfigYAML(filteredConfig)
	case "env":
		output, err = exportConfigEnv(filteredConfig)
	case "docker":
		output, err = exportConfigDocker(filteredConfig)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to export config: %w", err)
	}

	// Выводим результат
	if outputFile != "" {
		return writeToFile(outputFile, []byte(output))
	}

	fmt.Print(output)
	return nil
}

// filterConfig фильтрует конфигурацию
func filterConfig(config *cliConfig.Config, include, exclude []string) map[string]interface{} {
	result := make(map[string]interface{})

	// Конвертируем конфигурацию в map
	configMap := configToMap(config)

	// Если указаны include, включаем только их
	if len(include) > 0 {
		for _, section := range include {
			if value, exists := configMap[section]; exists {
				result[section] = value
			}
		}
		return result
	}

	// Если указаны exclude, исключаем их
	if len(exclude) > 0 {
		for key, value := range configMap {
			shouldExclude := false
			for _, excl := range exclude {
				if key == excl {
					shouldExclude = true
					break
				}
			}
			if !shouldExclude {
				result[key] = value
			}
		}
		return result
	}

	// Возвращаем всю конфигурацию
	return configMap
}

// configToMap конвертирует конфигурацию в map
func configToMap(config *cliConfig.Config) map[string]interface{} {
	result := make(map[string]interface{})

	// API настройки
	if config.API.BaseURL != "" {
		result["api"] = map[string]interface{}{
			"base_url": config.API.BaseURL,
			"timeout":  config.API.Timeout,
		}
	}

	// gRPC настройки
	if config.GRPC.SchedulerAddress != "" {
		result["grpc"] = map[string]interface{}{
			"scheduler_address": config.GRPC.SchedulerAddress,
			"core_address":      config.GRPC.CoreAddress,
			"auth_address":       config.GRPC.AuthAddress,
			"use_grpc":          config.GRPC.UseGRPC,
			"timeout":           config.GRPC.Timeout,
		}
	}

	// Auth настройки
	if config.Auth.AccessSecret != "" {
		result["auth"] = map[string]interface{}{
			"token_expiry":       config.Auth.TokenExpiry,
			"refresh_threshold":  config.Auth.RefreshThreshold,
			"access_secret":      config.Auth.AccessSecret,
			"refresh_secret":     config.Auth.RefreshSecret,
		}
	}

	// Output настройки
	if config.Output.Format != "" {
		result["output"] = map[string]interface{}{
			"format": config.Output.Format,
			"colors": config.Output.Colors,
		}
	}

	// Текущий тенант
	result["current_tenant"] = config.CurrentTenant

	return result
}

// exportConfigJSON экспортирует конфигурацию в JSON
func exportConfigJSON(config map[string]interface{}) (string, error) {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// exportConfigYAML экспортирует конфигурацию в YAML
func exportConfigYAML(config map[string]interface{}) (string, error) {
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return string(data), nil
}

// exportConfigEnv экспортирует конфигурацию в переменные окружения
func exportConfigEnv(config map[string]interface{}) (string, error) {
	var lines []string

	flattenConfig(config, "UPTIMEPING", func(key, value string) {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	})

	return strings.Join(lines, "\n"), nil
}

// exportConfigDocker экспортирует конфигурацию для Docker Compose
func exportConfigDocker(config map[string]interface{}) (string, error) {
	var lines []string

	flattenConfig(config, "UPTIMEPING", func(key, value string) {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	})

	return strings.Join(lines, "\n"), nil
}

// flattenConfig рекурсивно преобразует вложенную структуру в плоские ключи
func flattenConfig(config map[string]interface{}, prefix string, fn func(key, value string)) {
	for key, value := range config {
		fullKey := fmt.Sprintf("%s_%s", prefix, strings.ToUpper(key))

		switch v := value.(type) {
		case map[string]interface{}:
			flattenConfig(v, fullKey, fn)
		case string:
			fn(fullKey, fmt.Sprintf(`"%s"`, v))
		case int, int32, int64:
			fn(fullKey, fmt.Sprintf("%d", v))
		case bool:
			fn(fullKey, fmt.Sprintf("%t", v))
		case float64:
			fn(fullKey, fmt.Sprintf("%f", v))
		default:
			fn(fullKey, fmt.Sprintf(`"%v"`, v))
		}
	}
}

// writeToFile записывает данные в файл
func writeToFile(filename string, data []byte) error {
	// Создаем директорию если не существует
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Записываем файл
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
