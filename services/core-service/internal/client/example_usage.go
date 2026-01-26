package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"UptimePingPlatform/gen/go/proto/api/incident/v1"
	"UptimePingPlatform/services/core-service/internal/domain"
)

// ExampleUsage демонстрирует использование IncidentClient
func ExampleUsage() {
	// Создаем конфигурацию клиента
	config := &Config{
		Address:         "localhost:50052",
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        10 * time.Second,
		RetryMultiplier: 2.0,
		RetryJitter:     0.1,
		EnableLogging:   true,
	}

	// Создаем клиент
	client, err := NewIncidentClient(config)
	if err != nil {
		log.Fatalf("Failed to create incident client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Пример 1: Создание инцидента при неудачной проверке
	checkResult := &domain.CheckResult{
		ID:          "result123",
		CheckID:     "check456",
		ExecutionID: "exec789",
		Success:     false,
		DurationMs:  1500,
		StatusCode:  500,
		Error:       "Internal server error: database connection failed",
		CheckedAt:   time.Now(),
		Metadata: map[string]string{
			"region":   "us-west-2",
			"service":  "api-gateway",
			"version":  "v1.2.3",
		},
	}

	incident, err := client.CreateIncident(ctx, checkResult, "tenant123")
	if err != nil {
		log.Printf("Failed to create incident: %v", err)
	} else {
		log.Printf("Created incident: ID=%s, Severity=%s", incident.Id, incident.Severity)
	}

	// Пример 2: Получение деталей инцидента
	if incident != nil {
		incidentDetails, err := client.GetIncident(ctx, incident.Id)
		if err != nil {
			log.Printf("Failed to get incident details: %v", err)
		} else {
			log.Printf("Incident details: %+v", incidentDetails)
		}
	}

	// Пример 3: Обновление статуса инцидента
	if incident != nil {
		updatedIncident, err := client.UpdateIncident(
			ctx,
			incident.Id,
			v1.IncidentStatus_INCIDENT_STATUS_ACKNOWLEDGED,
			v1.IncidentSeverity_INCIDENT_SEVERITY_ERROR,
		)
		if err != nil {
			log.Printf("Failed to update incident: %v", err)
		} else {
			log.Printf("Updated incident status: %s", updatedIncident.Status)
		}
	}

	// Пример 4: Получение списка инцидентов
	incidents, nextPageToken, err := client.ListIncidents(
		ctx,
		"tenant123",
		v1.IncidentStatus_INCIDENT_STATUS_OPEN,
		v1.IncidentSeverity_INCIDENT_SEVERITY_UNSPECIFIED,
		10,
		0,
	)
	if err != nil {
		log.Printf("Failed to list incidents: %v", err)
	} else {
		log.Printf("Found %d incidents, next page token: %d", len(incidents), nextPageToken)
		for _, inc := range incidents {
			log.Printf("- Incident %s: %s", inc.Id, inc.ErrorMessage)
		}
	}

	// Пример 5: Закрытие инцидента после восстановления
	if incident != nil {
		err := client.ResolveIncident(ctx, incident.Id)
		if err != nil {
			log.Printf("Failed to resolve incident: %v", err)
		} else {
			log.Printf("Resolved incident: %s", incident.Id)
		}
	}

	// Пример 6: Получение статистики клиента
	stats := client.GetStats()
	fmt.Printf("Client Statistics:\n")
	fmt.Printf("  Total Calls: %d\n", stats.CallsTotal)
	fmt.Printf("  Successful Calls: %d\n", stats.CallsSuccessful)
	fmt.Printf("  Failed Calls: %d\n", stats.CallsFailed)
	fmt.Printf("  Incidents Created: %d\n", stats.IncidentsCreated)
	fmt.Printf("  Incidents Updated: %d\n", stats.IncidentsUpdated)
	fmt.Printf("  Incidents Resolved: %d\n", stats.IncidentsResolved)
	fmt.Printf("  Total Retries: %d\n", stats.RetriesTotal)
	fmt.Printf("  Average Response Time: %v\n", stats.AverageResponseTime)
	if stats.LastError != "" {
		fmt.Printf("  Last Error: %s\n", stats.LastError)
	}
}

// ExampleWithRetry демонстрирует работу retry логики
func ExampleWithRetry() {
	// Конфигурация с агрессивной retry политикой для демонстрации
	config := &Config{
		Address:         "unreachable-host:50052", // Недоступный хост
		Timeout:         2 * time.Second,
		MaxRetries:      5,
		InitialDelay:    50 * time.Millisecond,
		MaxDelay:        1 * time.Second,
		RetryMultiplier: 2.0,
		RetryJitter:     0.1,
		EnableLogging:   true,
	}

	client, err := NewIncidentClient(config)
	if err != nil {
		log.Printf("Expected connection failure: %v", err)
		return
	}
	defer client.Close()

	ctx := context.Background()
	
	// Попытка создать инцидент с недоступным сервером
	checkResult := &domain.CheckResult{
		CheckID: "check123",
		Success: false,
		Error:   "Test error",
	}

	start := time.Now()
	_, err = client.CreateIncident(ctx, checkResult, "tenant123")
	duration := time.Since(start)

	if err != nil {
		log.Printf("Expected failure after retries: %v", err)
		log.Printf("Total time with retries: %v", duration)
	}

	// Статистика покажет количество retry попыток
	stats := client.GetStats()
	log.Printf("Total retries attempted: %d", stats.RetriesTotal)
}

// ExampleBatchProcessing демонстрирует обработку множественных результатов проверок
func ExampleBatchProcessing() {
	client, err := NewIncidentClient(DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Симуляция множественных результатов проверок
	checkResults := []*domain.CheckResult{
		{
			CheckID:     "check1",
			Success:     false,
			StatusCode:  500,
			Error:       "Database connection failed",
			CheckedAt:   time.Now().Add(-5 * time.Minute),
		},
		{
			CheckID:     "check2",
			Success:     false,
			StatusCode:  404,
			Error:       "Endpoint not found",
			CheckedAt:   time.Now().Add(-3 * time.Minute),
		},
		{
			CheckID:     "check3",
			Success:     true,
			StatusCode:  200,
			CheckedAt:   time.Now().Add(-1 * time.Minute),
		},
		{
			CheckID:     "check4",
			Success:     false,
			StatusCode:  503,
			Error:       "Service unavailable",
			CheckedAt:   time.Now(),
		},
	}

	tenantID := "tenant456"

	// Обработка результатов с созданием инцидентов для неудачных проверок
	for i, result := range checkResults {
		if !result.Success {
			incident, err := client.CreateIncident(ctx, result, tenantID)
			if err != nil {
				log.Printf("Failed to create incident for check %s: %v", result.CheckID, err)
				continue
			}
			
			log.Printf("Created incident %d: ID=%s, CheckID=%s, Severity=%s",
				i+1, incident.Id, result.CheckID, incident.Severity)
		} else {
			log.Printf("Check %s passed, no incident needed", result.CheckID)
		}
	}

	// Получение финальной статистики
	stats := client.GetStats()
	log.Printf("Batch processing completed:")
	log.Printf("  Processed %d check results", len(checkResults))
	log.Printf("  Created %d incidents", stats.IncidentsCreated)
	log.Printf("  Success rate: %.2f%%", float64(stats.CallsSuccessful)/float64(stats.CallsTotal)*100)
}

// ExampleErrorHandling демонстрирует различные сценарии обработки ошибок
func ExampleErrorHandling() {
	client, err := NewIncidentClient(DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Сценарий 1: Валидация входных данных
	_, err = client.CreateIncident(ctx, nil, "")
	if err != nil {
		log.Printf("Validation error (expected): %v", err)
	}

	// Сценарий 2: Работа с несуществующим инцидентом
	_, err = client.GetIncident(ctx, "non-existent-id")
	if err != nil {
		log.Printf("Get non-existent incident error: %v", err)
	}

	// Сценарий 3: Обновление несуществующего инцидента
	_, err = client.UpdateIncident(
		ctx,
		"non-existent-id",
		v1.IncidentStatus_INCIDENT_STATUS_RESOLVED,
		v1.IncidentSeverity_INCIDENT_SEVERITY_WARNING,
	)
	if err != nil {
		log.Printf("Update non-existent incident error: %v", err)
	}

	// Сценарий 4: Закрытие несуществующего инцидента
	err = client.ResolveIncident(ctx, "non-existent-id")
	if err != nil {
		log.Printf("Resolve non-existent incident error: %v", err)
	}

	// Анализ статистики для понимания паттернов ошибок
	stats := client.GetStats()
	log.Printf("Error handling statistics:")
	log.Printf("  Failed calls: %d", stats.CallsFailed)
	log.Printf("  Last error: %s", stats.LastError)
	if stats.CallsTotal > 0 {
		log.Printf("  Error rate: %.2f%%", float64(stats.CallsFailed)/float64(stats.CallsTotal)*100)
	}
}
