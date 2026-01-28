package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"UptimePingPlatform/pkg/errors"
	// incidentv1 "UptimePingPlatform/proto/incident/v1"
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

	// Incidents acknowledge flags
	incidentsAcknowledgeCmd.Flags().StringP("message", "m", "", "сообщение подтверждения")

	// Incidents resolve flags
	incidentsResolveCmd.Flags().StringP("message", "m", "", "сообщение разрешения")
}

// getIncidentClient creates a gRPC client for incident service
func getIncidentClient() (*MockIncidentClient, *grpc.ClientConn, error) {
	return getMockIncidentClient()
}

func handleIncidentsList(cmd *cobra.Command, args []string) error {
	status, _ := cmd.Flags().GetString("status")
	severity, _ := cmd.Flags().GetString("severity")
	tenant, _ := cmd.Flags().GetString("tenant")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	limit, _ := cmd.Flags().GetInt("limit")

	client, conn, err := getIncidentClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		Status   string     `json:"status"`
		Severity string     `json:"severity"`
		TenantId string     `json:"tenant_id"`
		Limit    int32      `json:"limit"`
		From     *time.Time `json:"from,omitempty"`
		To       *time.Time `json:"to,omitempty"`
	}{
		Status:   status,
		Severity: severity,
		TenantId: tenant,
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

	incidentsResp := resp.(*ListIncidentsResponse)

	if len(incidentsResp.Incidents) == 0 {
		fmt.Println("No incidents found")
		return nil
	}

	outputFormat := viper.GetString("output")
	switch outputFormat {
	case "json":
		fmt.Println("[")
		for i, incident := range incidentsResp.Incidents {
			if i > 0 {
				fmt.Println(",")
			}
			fmt.Printf(`  {"id": "%s", "title": "%s", "status": "%s", "severity": "%s", "created_at": "%s"}`,
				incident.IncidentId, incident.Title, incident.Status, incident.Severity,
				incident.CreatedAt.Format(time.RFC3339))
		}
		fmt.Println("\n]")
	default:
		fmt.Printf("Incidents (%d total):\n", len(incidentsResp.Incidents))
		fmt.Printf("%-20s %-30s %-12s %-10s %-20s\n", "ID", "Title", "Status", "Severity", "Created")
		fmt.Println("----------------------------------------------------------------------------------------")

		for _, incident := range incidentsResp.Incidents {
			id := incident.IncidentId
			if len(id) > 18 {
				id = id[:15] + "..."
			}

			title := incident.Title
			if len(title) > 28 {
				title = title[:25] + "..."
			}

			created := incident.CreatedAt.Format("2006-01-02 15:04:05")

			fmt.Printf("%-20s %-30s %-12s %-10s %-20s\n",
				id, title, incident.Status, incident.Severity, created)
		}
	}

	return nil
}

func handleIncidentsGet(cmd *cobra.Command, args []string) error {
	incidentID := args[0]

	client, conn, err := getIncidentClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		IncidentId string `json:"incident_id"`
	}{
		IncidentId: incidentID,
	}

	resp, err := client.GetIncident(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	incidentResp := resp.(*GetIncidentResponse)

	fmt.Printf("Incident Details:\n")
	fmt.Printf("ID: %s\n", incidentResp.IncidentId)
	fmt.Printf("Title: %s\n", incidentResp.Title)
	fmt.Printf("Description: %s\n", incidentResp.Description)
	fmt.Printf("Status: %s\n", incidentResp.Status)
	fmt.Printf("Severity: %s\n", incidentResp.Severity)
	fmt.Printf("Tenant: %s\n", incidentResp.TenantId)
	fmt.Printf("Check ID: %s\n", incidentResp.CheckId)
	fmt.Printf("Created: %s\n", incidentResp.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", incidentResp.UpdatedAt.Format(time.RFC3339))

	if incidentResp.AcknowledgedAt != nil {
		fmt.Printf("Acknowledged: %s\n", incidentResp.AcknowledgedAt.Format(time.RFC3339))
		fmt.Printf("Acknowledged By: %s\n", incidentResp.AcknowledgedBy)
	}

	if incidentResp.ResolvedAt != nil {
		fmt.Printf("Resolved: %s\n", incidentResp.ResolvedAt.Format(time.RFC3339))
		fmt.Printf("Resolved By: %s\n", incidentResp.ResolvedBy)
	}

	if viper.GetBool("verbose") {
		fmt.Printf("\nEvents:\n")
		for _, event := range incidentResp.Events {
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

	client, conn, err := getIncidentClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		IncidentId string `json:"incident_id"`
		Message    string `json:"message"`
	}{
		IncidentId: incidentID,
		Message:    message,
	}

	resp, err := client.AcknowledgeIncident(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	ackResp := resp.(*AcknowledgeIncidentResponse)

	fmt.Printf("✅ Incident '%s' acknowledged successfully\n", incidentID)
	if viper.GetBool("verbose") {
		fmt.Printf("Acknowledged at: %s\n", ackResp.AcknowledgedAt.Format(time.RFC3339))
		fmt.Printf("Acknowledged by: %s\n", ackResp.AcknowledgedBy)
	}

	return nil
}

func handleIncidentsResolve(cmd *cobra.Command, args []string) error {
	incidentID := args[0]
	message, _ := cmd.Flags().GetString("message")

	client, conn, err := getIncidentClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		IncidentId string `json:"incident_id"`
		Message    string `json:"message"`
	}{
		IncidentId: incidentID,
		Message:    message,
	}

	resp, err := client.ResolveIncident(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	resolveResp := resp.(*ResolveIncidentResponse)

	fmt.Printf("✅ Incident '%s' resolved successfully\n", incidentID)
	if viper.GetBool("verbose") {
		fmt.Printf("Resolved at: %s\n", resolveResp.ResolvedAt.Format(time.RFC3339))
		fmt.Printf("Resolved by: %s\n", resolveResp.ResolvedBy)
	}

	return nil
}
