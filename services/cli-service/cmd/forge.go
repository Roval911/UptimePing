package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	cliClient "UptimePingPlatform/services/cli-service/internal/client"
	cliConfig "UptimePingPlatform/services/cli-service/internal/config"
)

var forgeCmd = &cobra.Command{
	Use:   "forge",
	Short: "Управление Forge сервисом",
	Long: `Команды для управления Forge сервисом:
генерация кода и валидация protobuf файлов.`,
}

// forgeGenerateCmd represents the forge generate command
var forgeGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Сгенерировать код",
	Long:  `Генерирует код на основе protobuf файлов или конфигурации.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleForgeGenerate(cmd, args)
	},
}

// forgeValidateCmd represents the forge validate command
var forgeValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Валидировать protobuf файлы",
	Long:  `Проверяет валидность protobuf файлов и их синтаксис.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleForgeValidate(cmd, args)
	},
}

// forgeInteractiveCmd represents the forge interactive command
var forgeInteractiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Интерактивная настройка параметров проверки",
	Long:  `Запускает интерактивный режим для настройки параметров проверки на основе protobuf файла.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleForgeInteractive(cmd, args)
	},
}

// forgeTemplatesCmd represents the forge templates command
var forgeTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Показать доступные шаблоны",
	Long:  `Отображает список доступных шаблонов для генерации кода.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleForgeTemplates(cmd, args)
	},
}

func init() {
	forgeCmd.AddCommand(forgeGenerateCmd)
	forgeCmd.AddCommand(forgeValidateCmd)
	forgeCmd.AddCommand(forgeInteractiveCmd)
	forgeCmd.AddCommand(forgeTemplatesCmd)

	// Forge generate flags
	forgeGenerateCmd.Flags().StringP("input", "i", "", "входной файл или директория")
	forgeGenerateCmd.Flags().StringP("output", "o", "", "выходная директория")
	forgeGenerateCmd.Flags().StringP("template", "t", "", "шаблон для генерации")
	forgeGenerateCmd.Flags().StringP("language", "l", "go", "язык генерации (go, java, python, typescript)")
	forgeGenerateCmd.Flags().BoolP("watch", "w", false, "следить за изменениями")
	forgeGenerateCmd.Flags().StringP("config", "c", "", "файл конфигурации")

	// Forge validate flags
	forgeValidateCmd.Flags().StringP("input", "i", "", "входной файл или директория")
	forgeValidateCmd.Flags().StringP("proto-path", "p", "", "путь к protobuf файлам")
	forgeValidateCmd.Flags().BoolP("lint", "l", true, "проверять стиль кода")
	forgeValidateCmd.Flags().BoolP("breaking", "b", true, "проверять обратно-совместимость")

	// Forge interactive flags
	forgeInteractiveCmd.Flags().StringP("proto", "p", "", "protobuf файл для анализа")
	forgeInteractiveCmd.Flags().StringP("template", "t", "", "шаблон для настройки")

	// Forge templates flags
	forgeTemplatesCmd.Flags().StringP("type", "t", "", "тип шаблонов (http, grpc, tcp)")
	forgeTemplatesCmd.Flags().StringP("language", "l", "", "язык шаблонов (go, java, python)")
}

// getForgeClient создает клиент для работы с Forge сервисом
func getForgeClient() (cliClient.ForgeClientInterface, error) {
	// Создаем логгер
	log, err := logger.NewLogger("dev", "info", "cli-service", false)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания логгера: %w", err)
	}

	// Загружаем реальную конфигурацию из файла или переменных окружения
	config, err := cliConfig.LoadConfig("")
	if err != nil {
		log.Warn("не удалось загрузить конфигурацию, используем значения по умолчанию", logger.Error(err))
		config = cliConfig.DefaultConfig()
	}

	baseURL := config.API.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080" // Значение по умолчанию
	}

	return cliClient.NewForgeClient(baseURL, log), nil
}

func handleForgeGenerate(cmd *cobra.Command, args []string) error {
	input, _ := cmd.Flags().GetString("input")
	output, _ := cmd.Flags().GetString("output")
	template, _ := cmd.Flags().GetString("template")
	language, _ := cmd.Flags().GetString("language")
	watch, _ := cmd.Flags().GetBool("watch")
	config, _ := cmd.Flags().GetString("config")

	if input == "" {
		return errors.New(errors.ErrValidation, "input file or directory is required")
	}

	if output == "" {
		return errors.New(errors.ErrValidation, "output directory is required")
	}

	client, err := getForgeClient()
	if err != nil {
		return handleError(err, cmd)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(rootCtx, 300*time.Second) // 5 minutes timeout
	defer cancel()

	req := &cliClient.GenerateRequest{
		Input:    input,
		Output:   output,
		Template: template,
		Language: language,
		Watch:    watch,
		Config:   config,
	}

	resp, err := client.Generate(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	fmt.Printf("✅ Code generation completed successfully\n")
	fmt.Printf("Generated files: %d\n", resp.GeneratedFiles)
	fmt.Printf("Output directory: %s\n", resp.OutputPath)

	if viper.GetBool("verbose") {
		fmt.Printf("Generation time: %v\n", resp.GenerationTime.Format(time.RFC3339))
		for _, file := range resp.Files {
			fmt.Printf("  - %s\n", file)
		}
	}

	if watch {
		fmt.Println("👀 Watching for changes... Press Ctrl+C to stop")
		// В реальной реализации здесь будет настройка отслеживания изменений
		select {
		case <-ctx.Done():
			fmt.Println("Stopped watching for changes")
		}
	}

	return nil
}

func handleForgeValidate(cmd *cobra.Command, args []string) error {
	input, _ := cmd.Flags().GetString("input")
	protoPath, _ := cmd.Flags().GetString("proto-path")
	lint, _ := cmd.Flags().GetBool("lint")
	breaking, _ := cmd.Flags().GetBool("breaking")

	if input == "" {
		return errors.New(errors.ErrValidation, "input file or directory is required")
	}

	client, err := getForgeClient()
	if err != nil {
		return handleError(err, cmd)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(rootCtx, 120*time.Second) // 2 minutes timeout
	defer cancel()

	req := &cliClient.ValidateRequest{
		Input:     input,
		ProtoPath: protoPath,
		Lint:      lint,
		Breaking:  breaking,
	}

	resp, err := client.Validate(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	fmt.Printf("📋 Validation completed\n")
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Files checked: %d\n", resp.FilesChecked)

	if resp.Valid {
		fmt.Printf("✅ All files are valid\n")
	} else {
		fmt.Printf("❌ Validation failed with %d errors\n", len(resp.Errors))
		for _, validationError := range resp.Errors {
			fmt.Printf("  - %s: %s\n", validationError.File, validationError.Message)
			if viper.GetBool("verbose") {
				fmt.Printf("    Line: %d, Column: %d\n", validationError.Line, validationError.Column)
			}
		}
	}

	if len(resp.Warnings) > 0 {
		fmt.Printf("⚠️  %d warnings found\n", len(resp.Warnings))
		for _, warning := range resp.Warnings {
			fmt.Printf("  - %s: %s\n", warning.File, warning.Message)
		}
	}

	if viper.GetBool("verbose") {
		fmt.Printf("Validation time: %v\n", resp.ValidationTime.Format(time.RFC3339))
		if protoPath != "" {
			fmt.Printf("Proto path: %s\n", protoPath)
		}
	}

	return nil
}

func handleForgeInteractive(cmd *cobra.Command, args []string) error {
	proto, _ := cmd.Flags().GetString("proto")
	template, _ := cmd.Flags().GetString("template")

	if proto == "" {
		return errors.New(errors.ErrValidation, "proto file is required")
	}

	client, err := getForgeClient()
	if err != nil {
		return handleError(err, cmd)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(rootCtx, 60*time.Second)
	defer cancel()

	req := &cliClient.InteractiveConfigRequest{
		ProtoFile: proto,
		Template:  template,
		Options:   make(map[string]string),
	}

	resp, err := client.InteractiveConfig(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	fmt.Printf("🔧 Interactive configuration completed\n")
	fmt.Printf("Template: %s\n", resp.Template)
	fmt.Printf("Ready: %t\n", resp.Ready)

	if viper.GetBool("verbose") {
		fmt.Printf("Configuration:\n")
		for key, value := range resp.Config {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	if resp.Ready {
		fmt.Printf("✅ Configuration is ready to use\n")
	} else {
		fmt.Printf("⚠️  Configuration needs additional setup\n")
	}

	return nil
}

func handleForgeTemplates(cmd *cobra.Command, args []string) error {
	templateType, _ := cmd.Flags().GetString("type")
	language, _ := cmd.Flags().GetString("language")

	client, err := getForgeClient()
	if err != nil {
		return handleError(err, cmd)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &cliClient.GetTemplatesRequest{
		Type:     templateType,
		Language: language,
	}

	resp, err := client.GetTemplates(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	if len(resp.Templates) == 0 {
		fmt.Printf("📭 No templates found")
		if templateType != "" {
			fmt.Printf(" for type '%s'", templateType)
		}
		if language != "" {
			fmt.Printf(" for language '%s'", language)
		}
		fmt.Printf("\n")
		return nil
	}

	fmt.Printf("📋 Available Templates (%d total):\n", resp.Total)
	fmt.Printf("%-20s %-15s %-15s %s\n", "Name", "Type", "Language", "Description")
	fmt.Println(strings.Repeat("-", 80))

	for _, template := range resp.Templates {
		name := template.Name
		if len(name) > 18 {
			name = name[:15] + "..."
		}

		description := template.Description
		if len(description) > 40 {
			description = description[:37] + "..."
		}

		fmt.Printf("%-20s %-15s %-15s %s\n", name, template.Type, template.Language, description)

		if viper.GetBool("verbose") {
			fmt.Printf("  Parameters:\n")
			for paramName, paramDesc := range template.Parameters {
				fmt.Printf("    %s: %s\n", paramName, paramDesc)
			}
			fmt.Printf("  Example:\n    %s\n\n", template.Example)
		}
	}

	return nil
}

// Helper function to check if path is directory
func isDirectory(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// Helper function to get all proto files in directory
func getProtoFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".proto" {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}
