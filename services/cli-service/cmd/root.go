package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	pkgconfig "UptimePingPlatform/pkg/config"
	pkgerrors "UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/cli-service/internal/config"
)

var (
	cfg       *config.Config
	appLogger logger.Logger
	rootCtx   context.Context
	pkgCfg    *pkgconfig.Config
)

// Execute executes the root command
func Execute(ctx context.Context, config *config.Config, logger logger.Logger, pkgConfig *pkgconfig.Config) error {
	rootCtx = ctx
	cfg = config
	appLogger = logger
	pkgCfg = pkgConfig

	return rootCmd.Execute()
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "uptimeping",
	Short: "UptimePing CLI - Управление платформой мониторинга",
	Long: `UptimePing CLI - мощный инструмент командной строки для управления 
платформой мониторинга доступности сервисов.

Поддерживает управление аутентификацией, проверками, инцидентами,
уведомлениями и конфигурацией системы.`,
	Version: "1.0.0",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize viper for config file support
		initConfig()

		// Set up logging
		setupLogging()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default is $HOME/.uptimeping.yaml)")
	rootCmd.PersistentFlags().StringP("server", "s", "localhost:8080", "server address")
	rootCmd.PersistentFlags().StringP("output", "o", "table", "output format (table, json, yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().Bool("debug", false, "debug mode")

	// Bind flags to viper
	viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))

	// Add subcommands
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(checksCmd)
	rootCmd.AddCommand(incidentsCmd)
	rootCmd.AddCommand(notificationCmd)
	rootCmd.AddCommand(forgeCmd)
	rootCmd.AddCommand(completionCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile := viper.GetString("config"); cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".uptimeping" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".uptimeping")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}

// setupLogging configures logging based on flags and config
func setupLogging() {
	// Simple logging setup - no complex initialization
	debug := viper.GetBool("debug")
	if debug {
		fmt.Println("Debug mode enabled")
	}
}

// handleError handles errors consistently across commands
func handleError(err error, cmd *cobra.Command) error {
	if err == nil {
		return nil
	}

	// Convert to our error type if possible
	var appErr *pkgerrors.Error
	if !errors.As(err, &appErr) {
		appErr = pkgerrors.New(pkgerrors.ErrInternal, err.Error())
	}

	// Log the error
	if appLogger != nil {
		appLogger.Error("Command failed",
			logger.String("command", cmd.Name()),
			logger.Error(appErr))
	}

	// Return formatted error
	return fmt.Errorf("%s: %s", cmd.Name(), appErr.GetUserMessage())
}
