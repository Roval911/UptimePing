package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"UptimePingPlatform/pkg/errors"
	// forgev1 "UptimePingPlatform/proto/forge/v1"
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

func init() {
	forgeCmd.AddCommand(forgeGenerateCmd)
	forgeCmd.AddCommand(forgeValidateCmd)

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
}

// getForgeClient creates a gRPC client for forge service
func getForgeClient() (*MockForgeClient, *grpc.ClientConn, error) {
	return getMockForgeClient()
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

	// Check if input exists
	if _, err := os.Stat(input); os.IsNotExist(err) {
		return errors.New(errors.ErrNotFound, fmt.Sprintf("input path does not exist: %s", input))
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(output, 0755); err != nil {
		return errors.Wrap(err, errors.ErrInternal, "failed to create output directory")
	}

	client, conn, err := getForgeClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 300*time.Second) // 5 minutes timeout
	defer cancel()

	req := &struct {
		Input    string `json:"input"`
		Output   string `json:"output"`
		Template string `json:"template"`
		Language string `json:"language"`
		Watch    bool   `json:"watch"`
		Config   string `json:"config"`
	}{
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

	generateResp := resp.(*GenerateResponse)

	fmt.Printf("✅ Code generation completed successfully\n")
	fmt.Printf("Generated files: %d\n", generateResp.GeneratedFiles)
	fmt.Printf("Output directory: %s\n", generateResp.OutputPath)

	if viper.GetBool("verbose") {
		fmt.Printf("Generation time: %v\n", generateResp.GenerationTime.Format(time.RFC3339))
		for _, file := range generateResp.Files {
			fmt.Printf("  - %s\n", file)
		}
	}

	if watch {
		fmt.Println("👀 Watching for changes... Press Ctrl+C to stop")
		// In a real implementation, you would set up file watching here
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

	// Check if input exists
	if _, err := os.Stat(input); os.IsNotExist(err) {
		return errors.New(errors.ErrNotFound, fmt.Sprintf("input path does not exist: %s", input))
	}

	client, conn, err := getForgeClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 120*time.Second) // 2 minutes timeout
	defer cancel()

	req := &struct {
		Input    string `json:"input"`
		ProtoPath string `json:"proto_path"`
		Lint     bool   `json:"lint"`
		Breaking bool   `json:"breaking"`
	}{
		Input:    input,
		ProtoPath: protoPath,
		Lint:     lint,
		Breaking: breaking,
	}

	resp, err := client.Validate(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	validateResp := resp.(*ValidateResponse)

	fmt.Printf("📋 Validation completed\n")
	fmt.Printf("Status: %s\n", validateResp.Status)
	fmt.Printf("Files checked: %d\n", validateResp.FilesChecked)

	if validateResp.Valid {
		fmt.Printf("✅ All files are valid\n")
	} else {
		fmt.Printf("❌ Validation failed with %d errors\n", len(validateResp.Errors))
		for _, validationError := range validateResp.Errors {
			fmt.Printf("  - %s: %s\n", validationError.File, validationError.Message)
			if viper.GetBool("verbose") {
				fmt.Printf("    Line: %d, Column: %d\n", validationError.Line, validationError.Column)
			}
		}
	}

	if len(validateResp.Warnings) > 0 {
		fmt.Printf("⚠️  %d warnings found\n", len(validateResp.Warnings))
		for _, warning := range validateResp.Warnings {
			fmt.Printf("  - %s: %s\n", warning.File, warning.Message)
		}
	}

	if viper.GetBool("verbose") {
		fmt.Printf("Validation time: %v\n", validateResp.ValidationTime.Format(time.RFC3339))
		if protoPath != "" {
			fmt.Printf("Proto path: %s\n", protoPath)
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
