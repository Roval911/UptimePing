package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	cliConfig "UptimePingPlatform/services/cli-service/internal/config"
)

// contextCmd represents the context command
var contextCmd = &cobra.Command{
	Use:   "context [name]",
	Short: "Управление контекстами (окружениями)",
	Long: `Управляет контекстами для переключения между разными окружениями.

Доступные команды:
  context list              - Показать список контекстов
  context current            - Показать текущий контекст
  context set <name>         - Установить контекст
  context create <name>      - Создать новый контекст
  context delete <name>      - Удалить контекст
  context show <name>        - Показать детали контекста

Примеры:
  uptimeping context list
  uptimeping context set production
  uptimeping context create staging
  uptimeping context delete test`,
}

var contextDir string

func init() {
	rootCmd.AddCommand(contextCmd)
	
	homeDir, _ := os.UserHomeDir()
	contextDir = filepath.Join(homeDir, ".uptimeping", "contexts")
	
	// Добавляем подкоманды
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextCurrentCmd)
	contextCmd.AddCommand(contextSetCmd)
	contextCmd.AddCommand(contextCreateCmd)
	contextCmd.AddCommand(contextDeleteCmd)
	contextCmd.AddCommand(contextShowCmd)
}

// ContextInfo представляет информацию о контексте
type ContextInfo struct {
	Name    string                 `json:"name" yaml:"name"`
	API     *APIConfig             `json:"api,omitempty" yaml:"api,omitempty"`
	GRPC    *GRPCConfig            `json:"grpc,omitempty" yaml:"grpc,omitempty"`
	Auth    *AuthConfig            `json:"auth,omitempty" yaml:"auth,omitempty"`
	Output  *OutputConfig          `json:"output,omitempty" yaml:"output,omitempty"`
	Current bool                   `json:"current" yaml:"current"`
}

// APIConfig представляет конфигурацию API
type APIConfig struct {
	BaseURL string `json:"base_url" yaml:"base_url"`
	Timeout int    `json:"timeout" yaml:"timeout"`
}

// GRPCConfig представляет конфигурацию gRPC
type GRPCConfig struct {
	SchedulerAddress string `json:"scheduler_address" yaml:"scheduler_address"`
	CoreAddress      string `json:"core_address" yaml:"core_address"`
	AuthAddress       string `json:"auth_address" yaml:"auth_address"`
	UseGRPC          bool   `json:"use_grpc" yaml:"use_grpc"`
	Timeout          int    `json:"timeout" yaml:"timeout"`
}

// AuthConfig представляет конфигурацию аутентификации
type AuthConfig struct {
	TokenExpiry      int    `json:"token_expiry" yaml:"token_expiry"`
	RefreshThreshold int    `json:"refresh_threshold" yaml:"refresh_threshold"`
	AccessSecret     string `json:"access_secret,omitempty" yaml:"access_secret,omitempty"`
	RefreshSecret    string `json:"refresh_secret,omitempty" yaml:"refresh_secret,omitempty"`
}

// OutputConfig представляет конфигурацию вывода
type OutputConfig struct {
	Format string `json:"format" yaml:"format"`
	Colors bool   `json:"colors" yaml:"colors"`
}

// contextListCmd - показать список контекстов
var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать список контекстов",
	RunE:  listContexts,
}

// contextCurrentCmd - показать текущий контекст
var contextCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Показать текущий контекст",
	RunE:  showCurrentContext,
}

// contextSetCmd - установить контекст
var contextSetCmd = &cobra.Command{
	Use:   "set [name]",
	Short: "Установить контекст",
	Args:  cobra.ExactArgs(1),
	RunE:  setContext,
}

// contextCreateCmd - создать контекст
var contextCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Создать новый контекст",
	Args:  cobra.ExactArgs(1),
	RunE:  createContext,
}

// contextDeleteCmd - удалить контекст
var contextDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Удалить контекст",
	Args:  cobra.ExactArgs(1),
	RunE:  deleteContext,
}

// contextShowCmd - показать детали контекста
var contextShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Показать детали контекста",
	Args:  cobra.ExactArgs(1),
	RunE:  showContext,
}

// listContexts выводит список всех контекстов
func listContexts(cmd *cobra.Command, args []string) error {
	contexts, err := loadAllContexts()
	if err != nil {
		return fmt.Errorf("failed to load contexts: %w", err)
	}

	currentContext, err := getCurrentContext()
	if err != nil {
		return fmt.Errorf("failed to get current context: %w", err)
	}

	if len(contexts) == 0 {
		fmt.Println("No contexts found. Use 'uptimeping context create <name>' to create one.")
		return nil
	}

	fmt.Println("Available contexts:")
	for _, context := range contexts {
		marker := " "
		if context.Name == currentContext {
			marker = "*"
		}
		fmt.Printf("  %s %s\n", marker, context.Name)
	}

	return nil
}

// showCurrentContext показывает текущий контекст
func showCurrentContext(cmd *cobra.Command, args []string) error {
	currentContext, err := getCurrentContext()
	if err != nil {
		return fmt.Errorf("failed to get current context: %w", err)
	}

	if currentContext == "" {
		fmt.Println("No current context set. Use 'uptimeping context set <name>' to set one.")
		return nil
	}

	fmt.Printf("Current context: %s\n", currentContext)
	return nil
}

// setContext устанавливает контекст
func setContext(cmd *cobra.Command, args []string) error {
	contextName := args[0]

	// Проверяем, что контекст существует
	contextFile := filepath.Join(contextDir, contextName+".yaml")
	if _, err := os.Stat(contextFile); os.IsNotExist(err) {
		return fmt.Errorf("context '%s' does not exist", contextName)
	}

	// Устанавливаем текущий контекст
	homeDir, _ := os.UserHomeDir()
	currentContextFile := filepath.Join(homeDir, ".uptimeping", "current_context")

	if err := os.MkdirAll(filepath.Dir(currentContextFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(currentContextFile, []byte(contextName), 0644); err != nil {
		return fmt.Errorf("failed to write current context: %w", err)
	}

	fmt.Printf("Context set to: %s\n", contextName)
	return nil
}

// createContext создает новый контекст
func createContext(cmd *cobra.Command, args []string) error {
	contextName := args[0]

	// Проверяем, что контекст еще не существует
	contextFile := filepath.Join(contextDir, contextName+".yaml")
	if _, err := os.Stat(contextFile); !os.IsNotExist(err) {
		return fmt.Errorf("context '%s' already exists", contextName)
	}

	// Загружаем текущую конфигурацию как основу
	config, err := cliConfig.LoadConfig("")
	if err != nil {
		return fmt.Errorf("failed to load current config: %w", err)
	}

	// Создаем информацию о контексте
	contextInfo := &ContextInfo{
		Name: contextName,
		API: &APIConfig{
			BaseURL: config.API.BaseURL,
			Timeout: config.API.Timeout,
		},
		GRPC: &GRPCConfig{
			SchedulerAddress: config.GRPC.SchedulerAddress,
			CoreAddress:      config.GRPC.CoreAddress,
			AuthAddress:       config.GRPC.AuthAddress,
			UseGRPC:          config.GRPC.UseGRPC,
			Timeout:          config.GRPC.Timeout,
		},
		Auth: &AuthConfig{
			TokenExpiry:      config.Auth.TokenExpiry,
			RefreshThreshold: config.Auth.RefreshThreshold,
			// Не экспортируем секреты
		},
		Output: &OutputConfig{
			Format: config.Output.Format,
			Colors: config.Output.Colors,
		},
	}

	// Сохраняем контекст
	if err := saveContext(contextInfo); err != nil {
		return fmt.Errorf("failed to save context: %w", err)
	}

	fmt.Printf("Context '%s' created successfully\n", contextName)
	fmt.Printf("Use 'uptimeping context set %s' to switch to it\n", contextName)
	return nil
}

// deleteContext удаляет контекст
func deleteContext(cmd *cobra.Command, args []string) error {
	contextName := args[0]

	// Проверяем, что контекст существует
	contextFile := filepath.Join(contextDir, contextName+".yaml")
	if _, err := os.Stat(contextFile); os.IsNotExist(err) {
		return fmt.Errorf("context '%s' does not exist", contextName)
	}

	// Проверяем, что это не текущий контекст
	currentContext, err := getCurrentContext()
	if err != nil {
		return fmt.Errorf("failed to get current context: %w", err)
	}

	if currentContext == contextName {
		return fmt.Errorf("cannot delete current context '%s'. Switch to another context first", contextName)
	}

	// Удаляем файл контекста
	if err := os.Remove(contextFile); err != nil {
		return fmt.Errorf("failed to delete context: %w", err)
	}

	fmt.Printf("Context '%s' deleted successfully\n", contextName)
	return nil
}

// showContext показывает детали контекста
func showContext(cmd *cobra.Command, args []string) error {
	contextName := args[0]

	contextInfo, err := loadContext(contextName)
	if err != nil {
		return fmt.Errorf("failed to load context: %w", err)
	}

	// Определяем текущий контекст
	currentContext, _ := getCurrentContext()
	contextInfo.Current = (contextName == currentContext)

	// Выводим детали
	data, err := yaml.Marshal(contextInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	fmt.Printf("Context: %s\n", contextName)
	if contextInfo.Current {
		fmt.Println("(current)")
	}
	fmt.Println(string(data))

	return nil
}

// loadAllContexts загружает все контексты
func loadAllContexts() ([]*ContextInfo, error) {
	if _, err := os.Stat(contextDir); os.IsNotExist(err) {
		return []*ContextInfo{}, nil
	}

	files, err := os.ReadDir(contextDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read contexts directory: %w", err)
	}

	var contexts []*ContextInfo
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".yaml") {
			contextName := strings.TrimSuffix(file.Name(), ".yaml")
			context, err := loadContext(contextName)
			if err != nil {
				continue // Пропускаем некорректные файлы
			}
			contexts = append(contexts, context)
		}
	}

	return contexts, nil
}

// loadContext загружает контекст по имени
func loadContext(name string) (*ContextInfo, error) {
	contextFile := filepath.Join(contextDir, name+".yaml")
	
	data, err := os.ReadFile(contextFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read context file: %w", err)
	}

	var context ContextInfo
	if err := yaml.Unmarshal(data, &context); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context: %w", err)
	}

	context.Name = name
	return &context, nil
}

// saveContext сохраняет контекст
func saveContext(context *ContextInfo) error {
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return fmt.Errorf("failed to create contexts directory: %w", err)
	}

	contextFile := filepath.Join(contextDir, context.Name+".yaml")
	
	data, err := yaml.Marshal(context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	if err := os.WriteFile(contextFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}

	return nil
}

// getCurrentContext получает текущий контекст
func getCurrentContext() (string, error) {
	homeDir, _ := os.UserHomeDir()
	currentContextFile := filepath.Join(homeDir, ".uptimeping", "current_context")

	data, err := os.ReadFile(currentContextFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read current context file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}
