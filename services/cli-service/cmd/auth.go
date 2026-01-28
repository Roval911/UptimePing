package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"UptimePingPlatform/services/cli-service/internal/auth"
	cliConfig "UptimePingPlatform/services/cli-service/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Управление аутентификацией",
	Long: `Команды для управления аутентификацией пользователей:
вход, выход, регистрация и проверка статуса.`,
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Войти в систему",
	Long:  "Выполнить вход пользователя с email и паролем",
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Выйти из системы",
	Long:  "Выполнить выход текущего пользователя",
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Зарегистрироваться",
	Long:  "Зарегистрировать нового пользователя",
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Проверить статус аутентификации",
	Long:  "Показать информацию о текущем пользователе",
}

func init() {
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(registerCmd)
	authCmd.AddCommand(statusCmd)

	// Login flags
	loginCmd.Flags().StringP("email", "e", "", "email адрес")
	loginCmd.Flags().StringP("password", "p", "", "пароль")

	// Register flags
	registerCmd.Flags().StringP("email", "e", "", "email адрес")
	registerCmd.Flags().StringP("password", "p", "", "пароль")
	registerCmd.Flags().StringP("tenant", "t", "", "имя тенанта")

	// Set run functions
	loginCmd.RunE = handleLogin
	logoutCmd.RunE = handleLogout
	registerCmd.RunE = handleRegister
	statusCmd.RunE = handleStatus
}

func GetAuthCmd() *cobra.Command {
	return authCmd
}

func handleLogin(cmd *cobra.Command, args []string) error {
	// Загрузка конфигурации CLI - используем внутреннюю систему
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	// Get login input from flags or interactively
	var loginInput *auth.LoginInput

	email, _ := cmd.Flags().GetString("email")
	password, _ := cmd.Flags().GetString("password")

	if email != "" && password != "" {
		// Use provided flags
		loginInput = &auth.LoginInput{
			Email:    email,
			Password: password,
		}
	} else {
		// Get input interactively
		loginInput, err = auth.GetLoginInput()
		if err != nil {
			return fmt.Errorf("ошибка получения данных для входа: %w", err)
		}
	}

	// Perform login
	ctx := context.Background()
	if err := authManager.Login(ctx, loginInput); err != nil {
		return fmt.Errorf("ошибка входа: %w", err)
	}

	return nil
}

func handleLogout(cmd *cobra.Command, args []string) error {
	// Загрузка конфигурации CLI - используем внутреннюю систему
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	// Perform logout
	ctx := context.Background()
	if err := authManager.Logout(ctx); err != nil {
		return fmt.Errorf("ошибка выхода: %w", err)
	}

	return nil
}

func handleRegister(cmd *cobra.Command, args []string) error {
	// Загрузка конфигурации CLI - используем внутреннюю систему
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	// Get register input from flags or interactively
	var registerInput *auth.RegisterInput

	email, _ := cmd.Flags().GetString("email")
	password, _ := cmd.Flags().GetString("password")
	tenantName, _ := cmd.Flags().GetString("tenant")

	if email != "" && password != "" && tenantName != "" {
		// Use provided flags
		registerInput = &auth.RegisterInput{
			Email:      email,
			Password:   password,
			TenantName: tenantName,
		}
	} else {
		// Get input interactively
		registerInput, err = auth.GetRegisterInput()
		if err != nil {
			return fmt.Errorf("ошибка получения данных для регистрации: %w", err)
		}
	}

	// Perform registration
	ctx := context.Background()
	if err := authManager.Register(ctx, registerInput); err != nil {
		return fmt.Errorf("ошибка регистрации: %w", err)
	}

	return nil
}

func handleStatus(cmd *cobra.Command, args []string) error {
	// Загрузка конфигурации CLI - используем внутреннюю систему
	configPath, err := cliConfig.GetConfigPath()
	if err != nil {
		return fmt.Errorf("ошибка получения пути конфигурации: %w", err)
	}

	cfg, err := cliConfig.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Create auth manager
	authManager, err := auth.NewAuthManager(cfg)
	if err != nil {
		return fmt.Errorf("ошибка создания менеджера аутентификации: %w", err)
	}
	defer authManager.Close()

	// Show status
	if err := authManager.Status(); err != nil {
		return fmt.Errorf("ошибка получения статуса: %w", err)
	}

	return nil
}
