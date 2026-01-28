package main

import (
	"context"
	"fmt"
	"os"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/cli-service/internal/client"
)

func main() {
	// Создаем логгер
	logger, err := logger.NewLogger("dev", "info", "cli-service", false)
	if err != nil {
		fmt.Printf("ошибка создания логгера: %v\n", err)
		os.Exit(1)
	}

	// Адреса сервисов (можно вынести в конфигурацию)
	schedulerAddr := "localhost:50051" // Scheduler Service gRPC порт
	coreAddr := "localhost:50052"     // Core Service gRPC порт

	// Создаем клиент с gRPC
	configClient, err := client.NewConfigClientWithGRPC(
		"http://localhost:8080", // Base URL для HTTP fallback
		schedulerAddr,           // Scheduler Service адрес
		coreAddr,               // Core Service адрес
		logger,
	)
	if err != nil {
		fmt.Printf("ошибка создания клиента: %v\n", err)
		os.Exit(1)
	}
	defer configClient.Close()

	ctx := context.Background()

	// Пример создания проверки
	fmt.Println("=== Создание проверки через gRPC ===")
	createReq := &client.CheckCreateRequest{
		Name:     "Google Homepage",
		Type:     "http",
		Target:   "https://google.com",
		Interval: 60,
		Timeout:  10,
		Tags:     []string{"production", "web"},
		Metadata: map[string]string{
			"created_by": "cli-example",
		},
	}

	check, err := configClient.CreateCheck(ctx, createReq)
	if err != nil {
		fmt.Printf("ошибка создания проверки: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Проверка создана: ID=%s, Name=%s\n", check.ID, check.Name)

	// Пример получения проверки
	fmt.Println("\n=== Получение проверки через gRPC ===")
	retrievedCheck, err := configClient.GetCheck(ctx, check.ID)
	if err != nil {
		fmt.Printf("ошибка получения проверки: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Проверка получена: ID=%s, Type=%s, Target=%s\n", 
		retrievedCheck.ID, retrievedCheck.Type, retrievedCheck.Target)

	// Пример запуска проверки
	fmt.Println("\n=== Запуск проверки через gRPC ===")
	runResp, err := configClient.RunCheck(ctx, check.ID)
	if err != nil {
		fmt.Printf("ошибка запуска проверки: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Проверка запущена: ExecutionID=%s, Status=%s\n", 
		runResp.ExecutionID, runResp.Status)

	// Пример получения статуса
	fmt.Println("\n=== Получение статуса через gRPC ===")
	statusResp, err := configClient.GetCheckStatus(ctx, check.ID)
	if err != nil {
		fmt.Printf("ошибка получения статуса: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Статус проверки: Status=%s, IsRunning=%t\n", 
		statusResp.Status, statusResp.IsRunning)

	// Пример получения истории
	fmt.Println("\n=== Получение истории через gRPC ===")
	historyResp, err := configClient.GetCheckHistory(ctx, check.ID, 1, 10)
	if err != nil {
		fmt.Printf("ошибка получения истории: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ История получена: Total=%d, Executions=%d\n", 
		historyResp.Total, len(historyResp.Executions))

	for i, execution := range historyResp.Executions {
		fmt.Printf("  %d. %s - %s (%dms)\n", 
			i+1, execution.ExecutionID, execution.Status, execution.Duration)
	}

	// Пример списка проверок
	fmt.Println("\n=== Список проверок через gRPC ===")
	listResp, err := configClient.ListChecks(ctx, []string{"production"}, nil, 1, 20)
	if err != nil {
		fmt.Printf("ошибка получения списка: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Список получен: Total=%d, Checks=%d\n", 
		listResp.Total, len(listResp.Checks))

	for i, check := range listResp.Checks {
		fmt.Printf("  %d. %s - %s (%s)\n", 
			i+1, check.ID, check.Name, check.Type)
	}

	fmt.Println("\n=== Все операции выполнены успешно! ===")
}
