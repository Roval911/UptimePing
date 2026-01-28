package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	// authv1 "UptimePingPlatform/proto/auth/v1"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Управление аутентификацией",
	Long: `Команды для управления аутентификацией пользователей:
вход, выход, регистрация, управление API ключами и проверка статуса.`,
}

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login [email]",
	Short: "Войти в систему",
	Long: `Выполняет вход пользователя в систему по email и паролю.
Сохраняет токен аутентификации для последующих команд.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleLogin(cmd, args)
	},
}

// logoutCmd represents the logout command
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Выйти из системы",
	Long:  `Удаляет сохраненный токен аутентификации.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleLogout(cmd, args)
	},
}

// registerCmd represents the register command
var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Зарегистрировать нового пользователя",
	Long:  `Создает новую учетную запись пользователя в системе.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleRegister(cmd, args)
	},
}

// apiKeyCmd represents the api-key command
var apiKeyCmd = &cobra.Command{
	Use:   "api-key",
	Short: "Управление API ключами",
	Long:  `Команды для создания, просмотра и отзыва API ключей.`,
}

// apiKeyCreateCmd represents the api-key create command
var apiKeyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Создать новый API ключ",
	Long:  `Создает новый API ключ для аутентификации запросов.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleAPIKeyCreate(cmd, args)
	},
}

// apiKeyListCmd represents the api-key list command
var apiKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать список API ключей",
	Long:  `Отображает все API ключи текущего пользователя.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleAPIKeyList(cmd, args)
	},
}

// apiKeyRevokeCmd represents the api-key revoke command
var apiKeyRevokeCmd = &cobra.Command{
	Use:   "revoke [key-id]",
	Short: "Отозвать API ключ",
	Long:  `Отзывает указанный API ключ.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleAPIKeyRevoke(cmd, args)
	},
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Проверить статус аутентификации",
	Long:  `Проверяет текущий статус аутентификации и информацию о пользователе.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleAuthStatus(cmd, args)
	},
}

func init() {
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(registerCmd)
	authCmd.AddCommand(apiKeyCmd)
	authCmd.AddCommand(statusCmd)

	apiKeyCmd.AddCommand(apiKeyCreateCmd)
	apiKeyCmd.AddCommand(apiKeyListCmd)
	apiKeyCmd.AddCommand(apiKeyRevokeCmd)

	// Login flags
	loginCmd.Flags().StringP("password", "p", "", "пароль")
	loginCmd.Flags().StringP("tenant", "t", "", "имя тенанта")
	loginCmd.Flags().Bool("save-token", true, "сохранить токен")

	// Register flags
	registerCmd.Flags().StringP("email", "e", "", "email адрес")
	registerCmd.Flags().StringP("password", "p", "", "пароль")
	registerCmd.Flags().StringP("tenant", "t", "", "имя тенанта")
	registerCmd.Flags().StringP("name", "n", "", "имя пользователя")

	// API Key create flags
	apiKeyCreateCmd.Flags().StringP("name", "n", "", "название ключа")
	apiKeyCreateCmd.Flags().StringP("expires", "x", "", "срок действия (например: 24h, 7d, 30d)")
}

// getAuthClient creates a gRPC client for auth service
func getAuthClient() (*MockAuthClient, *grpc.ClientConn, error) {
	return getMockAuthClient()
}

// getAuthToken retrieves auth token from storage
func getAuthToken() (string, error) {
	// Try to get token from viper (config file)
	if token := viper.GetString("auth.token"); token != "" {
		return token, nil
	}

	// Try to get token from environment
	if token := os.Getenv("UPTIMEPING_TOKEN"); token != "" {
		return token, nil
	}

	return "", errors.New(errors.ErrUnauthorized, "authentication token not found")
}

// saveAuthToken saves auth token to storage
func saveAuthToken(token string) error {
	viper.Set("auth.token", token)
	
	// Save to config file if specified
	if configFile := viper.ConfigFileUsed(); configFile != "" {
		return viper.WriteConfig()
	}
	
	return nil
}

// clearAuthToken removes saved auth token
func clearAuthToken() error {
	viper.Set("auth.token", "")
	
	if configFile := viper.ConfigFileUsed(); configFile != "" {
		return viper.WriteConfig()
	}
	
	return nil
}

func handleLogin(cmd *cobra.Command, args []string) error {
	var email string
	if len(args) > 0 {
		email = args[0]
	} else {
		email, _ = cmd.Flags().GetString("email")
	}

	password, _ := cmd.Flags().GetString("password")
	tenant, _ := cmd.Flags().GetString("tenant")
	saveToken, _ := cmd.Flags().GetBool("save-token")

	if email == "" {
		return errors.New(errors.ErrValidation, "email is required")
	}

	if password == "" {
		fmt.Print("Enter password: ")
		var pass string
		fmt.Scanln(&pass)
		password = pass
	}

	client, conn, err := getAuthClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		TenantId string `json:"tenant_id"`
	}{
		Email:    email,
		Password: password,
		TenantId: tenant,
	}

	resp, err := client.Login(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	loginResp := resp.(*LoginResponse)

	if saveToken {
		if err := saveAuthToken(loginResp.Token); err != nil {
			log.Warn("Failed to save token", logger.Error(err))
		}
	}

	fmt.Printf("✅ Successfully logged in as %s\n", email)
	if viper.GetBool("verbose") {
		fmt.Printf("Token: %s...\n", loginResp.Token[:min(20, len(loginResp.Token))])
	}

	return nil
}

func handleLogout(cmd *cobra.Command, args []string) error {
	if err := clearAuthToken(); err != nil {
		log.Warn("Failed to clear token", logger.Error(err))
	}

	fmt.Println("✅ Successfully logged out")
	return nil
}

func handleRegister(cmd *cobra.Command, args []string) error {
	email, _ := cmd.Flags().GetString("email")
	password, _ := cmd.Flags().GetString("password")
	tenant, _ := cmd.Flags().GetString("tenant")
	name, _ := cmd.Flags().GetString("name")

	if email == "" {
		return errors.New(errors.ErrValidation, "email is required")
	}

	if password == "" {
		fmt.Print("Enter password: ")
		var pass string
		fmt.Scanln(&pass)
		password = pass
	}

	if tenant == "" {
		fmt.Print("Enter tenant name: ")
		var t string
		fmt.Scanln(&t)
		tenant = t
	}

	client, conn, err := getAuthClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		TenantId string `json:"tenant_id"`
		Name     string `json:"name"`
	}{
		Email:    email,
		Password: password,
		TenantId: tenant,
		Name:     name,
	}

	resp, err := client.Register(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	registerResp := resp.(*RegisterResponse)

	fmt.Printf("✅ Successfully registered user %s\n", email)
	if viper.GetBool("verbose") {
		fmt.Printf("User ID: %s\n", registerResp.UserId)
	}

	return nil
}

func handleAPIKeyCreate(cmd *cobra.Command, args []string) error {
	token, err := getAuthToken()
	if err != nil {
		return handleError(err, cmd)
	}

	name, _ := cmd.Flags().GetString("name")
	expires, _ := cmd.Flags().GetString("expires")

	client, conn, err := getAuthClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		Name string `json:"name"`
	}{
		Name: name,
	}

	if expires != "" {
		// For mock implementation, just log the expires value
		if viper.GetBool("verbose") {
			fmt.Printf("API key expires: %s\n", expires)
		}
	}

	// Add auth token to context
	ctx = context.WithValue(ctx, "token", token)

	resp, err := client.CreateAPIKey(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	apiKeyResp := resp.(*CreateAPIKeyResponse)

	fmt.Printf("✅ API key created successfully\n")
	fmt.Printf("Key ID: %s\n", apiKeyResp.KeyId)
	fmt.Printf("API Key: %s\n", apiKeyResp.ApiKey)
	fmt.Printf("Expires at: %s\n", apiKeyResp.ExpiresAt.Format(time.RFC3339))

	return nil
}

func handleAPIKeyList(cmd *cobra.Command, args []string) error {
	token, err := getAuthToken()
	if err != nil {
		return handleError(err, cmd)
	}

	client, conn, err := getAuthClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct{}{}
	ctx = context.WithValue(ctx, "token", token)

	resp, err := client.ListAPIKeys(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	keysResp := resp.(*ListAPIKeysResponse)

	if len(keysResp.Keys) == 0 {
		fmt.Println("No API keys found")
		return nil
	}

	fmt.Println("API Keys:")
	for range keysResp.Keys {
		// For mock implementation, just show placeholder
		fmt.Printf("- Key: %s (Active)\n", "mock-key")
	}

	return nil
}

func handleAPIKeyRevoke(cmd *cobra.Command, args []string) error {
	token, err := getAuthToken()
	if err != nil {
		return handleError(err, cmd)
	}

	keyID := args[0]

	client, conn, err := getAuthClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		KeyId string `json:"key_id"`
	}{
		KeyId: keyID,
	}
	ctx = context.WithValue(ctx, "token", token)

	_, err = client.RevokeAPIKey(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	fmt.Printf("✅ API key %s revoked successfully\n", keyID)
	return nil
}

func handleAuthStatus(cmd *cobra.Command, args []string) error {
	token, err := getAuthToken()
	if err != nil {
		fmt.Println("❌ Not authenticated")
		return nil
	}

	client, conn, err := getAuthClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		Token string `json:"token"`
	}{
		Token: token,
	}
	ctx = context.WithValue(ctx, "token", token)

	resp, err := client.ValidateToken(ctx, req)
	if err != nil {
		fmt.Println("❌ Invalid token")
		return nil
	}

	tokenResp := resp.(*ValidateTokenResponse)

	fmt.Println("✅ Authenticated")
	fmt.Printf("User ID: %s\n", tokenResp.UserId)
	fmt.Printf("Email: %s\n", tokenResp.Email)
	fmt.Printf("Tenant: %s\n", tokenResp.TenantId)
	if viper.GetBool("verbose") {
		fmt.Printf("Token expires: %s\n", tokenResp.ExpiresAt.Format(time.RFC3339))
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
