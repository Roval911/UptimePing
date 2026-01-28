package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	cliClient "UptimePingPlatform/services/cli-service/internal/client"
	cliConfig "UptimePingPlatform/services/cli-service/internal/config"
	"UptimePingPlatform/services/cli-service/internal/output"
)

var incidentsCmd = &cobra.Command{
	Use:   "incidents",
	Short: "Управление инцидентами",
	Long: `Команды для управления инцидентами:
просмотр списка, получение деталей, подтверждение и разрешение инцидентов.`,
}

// incidentsListCmd represents the incidents list command
var incidentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать список инцидентов",
	Long:  `Отображает список инцидентов с возможностью фильтрации.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleIncidentsList(cmd, args)
	},
}

func init() {
	incidentsCmd.AddCommand(incidentsListCmd)
	incidentsCmd.AddCommand(incidentsGetCmd)
	incidentsCmd.AddCommand(incidentsAcknowledgeCmd)
	incidentsCmd.AddCommand(incidentsResolveCmd)

	// Incidents list flags
	incidentsListCmd.Flags().StringP("status", "a", "", "фильтр по статусу (open, acknowledged, resolved)")
	incidentsListCmd.Flags().StringP("severity", "S", "", "фильтр по важности (low, medium, high, critical)")
	incidentsListCmd.Flags().StringP("tenant", "n", "", "фильтр по тенанту")
	incidentsListCmd.Flags().StringP("from", "f", "", "начальная дата (RFC3339)")
	incidentsListCmd.Flags().StringP("to", "e", "", "конечная дата (RFC3339)")
	incidentsListCmd.Flags().IntP("limit", "l", 50, "лимит записей")
	
	// Флаги формата вывода
	incidentsListCmd.Flags().StringP("format", "o", "", "Формат вывода (table, json, yaml)")
	incidentsListCmd.Flags().BoolP("pretty", "p", true, "Pretty JSON/YAML вывод")
	incidentsListCmd.Flags().BoolP("colors", "c", true, "Цветной вывод")

	// Incidents acknowledge flags
	incidentsAcknowledgeCmd.Flags().StringP("message", "m", "", "сообщение подтверждения")

	// Incidents resolve flags
	incidentsResolveCmd.Flags().StringP("message", "m", "", "сообщение разрешения")
}

// incidentsGetCmd represents the incidents get command
var incidentsGetCmd = &cobra.Command{
	Use:   "get [incident-id]",
	Short: "Получить детали инцидента",
	Long:  `Отображает подробную информацию об указанном инциденте.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleIncidentsGet(cmd, args)
	},
}

// incidentsAcknowledgeCmd represents the incidents acknowledge command
var incidentsAcknowledgeCmd = &cobra.Command{
	Use:   "acknowledge [incident-id]",
	Short: "Подтвердить инцидент",
	Long:  `Подтверждает указанный инцидент, отмечая его как acknowledged.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleIncidentsAcknowledge(cmd, args)
	},
}

// incidentsResolveCmd represents the incidents resolve command
var incidentsResolveCmd = &cobra.Command{
	Use:   "resolve [incident-id]",
	Short: "Разрешить инцидент",
	Long:  `Закрывает указанный инцидент, отмечая его как разрешенный.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleIncidentsResolve(cmd, args)
	},
}

// getIncidentClient создает клиент для работы с инцидентами
func getIncidentClient() (cliClient.IncidentClientInterface, error) {
	// Создаем логгер
	log, err := logger.NewLogger("dev", "info", "cli-service", false)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания логгера: %w", err)
	}

	// Загружаем реальную конфигурацию из файла или переменных окружения
	config, err := cliConfig.LoadConfig("")
	if err != nil {
		log.Warn("не удалось загрузить конфигурацию, используем значения по умолчанию", logger.Error(err))
		config = cliConfig.DefaultConfig()
	}

	baseURL := config.API.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080" // Значение по умолчанию
	}

	return cliClient.NewIncidentClient(baseURL, log), nil
}

func handleIncidentsList(cmd *cobra.Command, args []string) error {
	status, _ := cmd.Flags().GetString("status")
	severity, _ := cmd.Flags().GetString("severity")
	tenant, _ := cmd.Flags().GetString("tenant")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	limit, _ := cmd.Flags().GetInt("limit")
	
	// Получаем флаги формата
	formatStr, _ := cmd.Flags().GetString("format")
	pretty, _ := cmd.Flags().GetBool("pretty")
	useColors, _ := cmd.Flags().GetBool("colors")

	client, err := getIncidentClient()
	if err != nil {
		return handleError(err, cmd)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &cliClient.ListIncidentsRequest{
		Status:   status,
		Severity: severity,
		TenantID: tenant,
		Limit:    int32(limit),
	}

	if from != "" {
		if fromTime, err := time.Parse(time.RFC3339, from); err == nil {
			req.From = &fromTime
		} else {
			return errors.New(errors.ErrValidation, "invalid from date format, use RFC3339")
		}
	}

	if to != "" {
		if toTime, err := time.Parse(time.RFC3339, to); err == nil {
			req.To = &toTime
		} else {
			return errors.New(errors.ErrValidation, "invalid to date format, use RFC3339")
		}
	}

	resp, err := client.ListIncidents(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	if len(resp.Incidents) == 0 {
		// Используем новый форматировщик
		var format output.FormatType = output.FormatTable
		if formatStr != "" {
			switch formatStr {
			case "json":
				format = output.FormatJSON
			case "yaml":
				format = output.FormatYAML
			}
		}
		
		formatter := output.GetFormatter(format, pretty, useColors)
		emptyOutput, _ := formatter.Format("No incidents found")
		fmt.Println(emptyOutput)
		return nil
	}

	// Определяем формат вывода
	var format output.FormatType = output.FormatTable
	if formatStr != "" {
		switch formatStr {
		case "json":
			format = output.FormatJSON
		case "yaml":
			format = output.FormatYAML
		}
	}

	switch format {
	case output.FormatJSON:
		// Конвертируем []client.IncidentInfo в []interface{}
		incidents := make([]interface{}, len(resp.Incidents))
		for i, incident := range resp.Incidents {
			incidents[i] = incident
		}
		jsonOutput := output.CreateIncidentsJSONResponse(incidents, len(resp.Incidents), 1, len(resp.Incidents))
		output.PrintJSON(jsonOutput, pretty)
	case output.FormatYAML:
		// Конвертируем []client.IncidentInfo в []interface{}
		incidents := make([]interface{}, len(resp.Incidents))
		for i, incident := range resp.Incidents {
			incidents[i] = incident
		}
		yamlOutput := output.CreateIncidentsYAMLResponse(incidents, len(resp.Incidents), 1, len(resp.Incidents))
		output.PrintYAML(yamlOutput)
	default:
		// Конвертируем []client.IncidentInfo в []interface{}
		incidents := make([]interface{}, len(resp.Incidents))
		for i, incident := range resp.Incidents {
			incidents[i] = incident
		}
		// Табличный формат
		table := output.CreateIncidentsTable(incidents, useColors)
		output.PrintTable(table)
	}

	return nil
}

func handleIncidentsGet(cmd *cobra.Command, args []string) error {
	incidentID := args[0]

	client, err := getIncidentClient()
	if err != nil {
		return handleError(err, cmd)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &cliClient.GetIncidentRequest{
		IncidentID: incidentID,
	}

	resp, err := client.GetIncident(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	fmt.Printf("Incident Details:\n")
	fmt.Printf("ID: %s\n", resp.IncidentID)
	fmt.Printf("Title: %s\n", resp.Title)
	fmt.Printf("Description: %s\n", resp.Description)
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Severity: %s\n", resp.Severity)
	fmt.Printf("Tenant: %s\n", resp.TenantID)
	fmt.Printf("Check ID: %s\n", resp.CheckID)
	fmt.Printf("Created: %s\n", resp.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", resp.UpdatedAt.Format(time.RFC3339))

	if resp.AcknowledgedAt != nil {
		fmt.Printf("Acknowledged: %s\n", resp.AcknowledgedAt.Format(time.RFC3339))
		fmt.Printf("Acknowledged By: %s\n", resp.AcknowledgedBy)
	}

	if resp.ResolvedAt != nil {
		fmt.Printf("Resolved: %s\n", resp.ResolvedAt.Format(time.RFC3339))
		fmt.Printf("Resolved By: %s\n", resp.ResolvedBy)
	}

	if viper.GetBool("verbose") {
		fmt.Printf("\nEvents:\n")
		for _, event := range resp.Events {
			fmt.Printf("  %s: %s - %s\n",
				event.Timestamp.Format("2006-01-02 15:04:05"),
				event.Type, event.Message)
		}
	}

	return nil
}

func handleIncidentsAcknowledge(cmd *cobra.Command, args []string) error {
	incidentID := args[0]
	message, _ := cmd.Flags().GetString("message")

	client, err := getIncidentClient()
	if err != nil {
		return handleError(err, cmd)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &cliClient.AcknowledgeIncidentRequest{
		IncidentID: incidentID,
		Message:    message,
	}

	resp, err := client.AcknowledgeIncident(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	fmt.Printf("✅ Incident '%s' acknowledged successfully\n", incidentID)
	if viper.GetBool("verbose") {
		fmt.Printf("Acknowledged at: %s\n", resp.AcknowledgedAt.Format(time.RFC3339))
		fmt.Printf("Acknowledged by: %s\n", resp.AcknowledgedBy)
	}

	return nil
}

func handleIncidentsResolve(cmd *cobra.Command, args []string) error {
	incidentID := args[0]
	message, _ := cmd.Flags().GetString("message")

	client, err := getIncidentClient()
	if err != nil {
		return handleError(err, cmd)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &cliClient.ResolveIncidentRequest{
		IncidentID: incidentID,
		Message:    message,
	}

	resp, err := client.ResolveIncident(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	fmt.Printf("✅ Incident '%s' resolved successfully\n", incidentID)
	if viper.GetBool("verbose") {
		fmt.Printf("Resolved at: %s\n", resp.ResolvedAt.Format(time.RFC3339))
		fmt.Printf("Resolved by: %s\n", resp.ResolvedBy)
	}

	return nil
}
