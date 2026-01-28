package main

import (
	"context"
	"fmt"
	"os"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/cli-service/cmd"
)

func main() {
	// Создаем конфигурацию по умолчанию
	cfg, err := config.LoadConfig("")
	if err != nil {
		cfg = &config.Config{
			Server: config.ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
			Logger: config.LoggerConfig{
				Level:  "info",
				Format: "console",
			},
			Environment: "dev",
		}
	}

	// Создаем логгер
	log, err := logger.NewLogger(cfg.Logger.Level, cfg.Logger.Format, "cli-service", false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания логгера: %v\n", err)
		os.Exit(1)
	}

	// Выполняем команду
	ctx := context.Background()
	if err := cmd.Execute(ctx, cfg, log); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
}
