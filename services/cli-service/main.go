package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"UptimePingPlatform/pkg/config"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/cli-service/cmd"
)

func main() {
	// Initialize configuration
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.NewLogger(
		cfg.Logger.Level,
		cfg.Logger.Format,
		"cli-service",
		false, // enableLoki
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Create context with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Received shutdown signal")
		cancel()
	}()

	// Execute CLI command
	if err := cmd.Execute(ctx, cfg, log); err != nil {
		log.Error("CLI execution failed", logger.Error(err))
		os.Exit(1)
	}
}
