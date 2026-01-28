package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"UptimePingPlatform/services/cli-service/cmd"
)

var rootCmd = &cobra.Command{
	Use:   "uptimeping",
	Short: "UptimePing CLI",
	Long:  `UptimePing CLI - инструмент командной строки для мониторинга доступности сервисов.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Показать версию",
	Long:  "Показать информацию о версии CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("UptimePing CLI v1.0.0\n")
		fmt.Printf("Built for UptimePing Platform\n")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cmd.GetConfigCmd())
	rootCmd.AddCommand(cmd.GetAuthCmd())
	rootCmd.AddCommand(cmd.GetChecksCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
}
