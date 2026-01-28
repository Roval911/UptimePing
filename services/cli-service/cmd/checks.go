package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"UptimePingPlatform/pkg/errors"
	// corev1 "UptimePingPlatform/proto/core/v1"
)

var checksCmd = &cobra.Command{
	Use:   "checks",
	Short: "Управление проверками",
	Long: `Команды для управления проверками доступности:
запуск, проверка статуса, просмотр истории и списка проверок.`,
}

// checksRunCmd represents the checks run command
var checksRunCmd = &cobra.Command{
	Use:   "run [check-id]",
	Short: "Запустить проверку",
	Long:  `Запускает проверку с указанным ID или создает новую проверку.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleChecksRun(cmd, args)
	},
}

// checksStatusCmd represents the checks status command
var checksStatusCmd = &cobra.Command{
	Use:   "status [check-id]",
	Short: "Проверить статус проверки",
	Long:  `Проверяет текущий статус указанной проверки.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleChecksStatus(cmd, args)
	},
}

// checksHistoryCmd represents the checks history command
var checksHistoryCmd = &cobra.Command{
	Use:   "history [check-id]",
	Short: "Показать историю проверок",
	Long:  `Отображает историю выполнения указанной проверки.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleChecksHistory(cmd, args)
	},
}

// checksListCmd represents the checks list command
var checksListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать список проверок",
	Long:  `Отображает все доступные проверки.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleChecksList(cmd, args)
	},
}

func init() {
	checksCmd.AddCommand(checksRunCmd)
	checksCmd.AddCommand(checksStatusCmd)
	checksCmd.AddCommand(checksHistoryCmd)
	checksCmd.AddCommand(checksListCmd)

	// Checks run flags
	checksRunCmd.Flags().StringP("url", "u", "", "URL для проверки")
	checksRunCmd.Flags().StringP("method", "d", "GET", "HTTP метод")
	checksRunCmd.Flags().StringP("type", "y", "http", "тип проверки (http, tcp, grpc, graphql)")
	checksRunCmd.Flags().StringP("interval", "i", "1m", "интервал проверки")
	checksRunCmd.Flags().IntP("timeout", "m", 30, "таймаут в секундах")
	checksRunCmd.Flags().StringP("name", "n", "", "название проверки")
	checksRunCmd.Flags().StringP("tenant", "e", "", "ID тенанта")

	// Checks history flags
	checksHistoryCmd.Flags().IntP("limit", "l", 50, "лимит записей")
	checksHistoryCmd.Flags().StringP("from", "f", "", "начальная дата (RFC3339)")
	checksHistoryCmd.Flags().StringP("to", "o", "", "конечная дата (RFC3339)")

	// Checks list flags
	checksListCmd.Flags().StringP("status", "a", "", "фильтр по статусу")
	checksListCmd.Flags().StringP("type", "y", "", "фильтр по типу")
	checksListCmd.Flags().StringP("tenant", "n", "", "фильтр по тенанту")
}

// getCoreClient creates a gRPC client for core service
func getCoreClient() (*MockCoreClient, *grpc.ClientConn, error) {
	return getMockCoreClient()
}

func handleChecksRun(cmd *cobra.Command, args []string) error {
	var checkID string
	if len(args) > 0 {
		checkID = args[0]
	}

	client, conn, err := getCoreClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 60*time.Second)
	defer cancel()

	if checkID != "" {
		// Run existing check
		req := &struct {
			CheckId string `json:"check_id"`
		}{
			CheckId: checkID,
		}

		resp, err := client.ExecuteCheck(ctx, req)
		if err != nil {
			return handleError(err, cmd)
		}

		checkResp := resp.(*ExecuteCheckResponse)

		fmt.Printf("✅ Check '%s' executed successfully\n", checkID)
		fmt.Printf("Status: %s\n", checkResp.Status)
		fmt.Printf("Response Time: %dms\n", checkResp.ResponseTime)
		if checkResp.Message != "" {
			fmt.Printf("Message: %s\n", checkResp.Message)
		}
	} else {
		// Create and run new check
		url, _ := cmd.Flags().GetString("url")
		interval, _ := cmd.Flags().GetString("interval")
		timeout, _ := cmd.Flags().GetInt("timeout")
		name, _ := cmd.Flags().GetString("name")
		tenant, _ := cmd.Flags().GetString("tenant")

		// Get the flag value by the actual flag name, not shorthand
		method, _ := cmd.Flags().GetString("method")
		checkType, _ := cmd.Flags().GetString("type")

		if url == "" {
			return errors.New(errors.ErrValidation, "URL is required for new check")
		}

		// Create check configuration
		checkConfig := &struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Url      string `json:"url"`
			Method   string `json:"method"`
			Interval string `json:"interval"`
			Timeout  int32  `json:"timeout"`
			TenantId string `json:"tenant_id"`
		}{
			Name:     name,
			Type:     checkType,
			Url:      url,
			Method:   method,
			Interval: interval,
			Timeout:  int32(timeout),
			TenantId: tenant,
		}

		req := &struct {
			Config interface{} `json:"config"`
		}{
			Config: checkConfig,
		}

		resp, err := client.ExecuteCheck(ctx, req)
		if err != nil {
			return handleError(err, cmd)
		}

		checkResp := resp.(*ExecuteCheckResponse)

		fmt.Printf("✅ Check executed successfully\n")
		fmt.Printf("Check ID: %s\n", checkResp.CheckId)
		fmt.Printf("Status: %s\n", checkResp.Status)
		fmt.Printf("Response Time: %dms\n", checkResp.ResponseTime)
		if checkResp.Message != "" {
			fmt.Printf("Message: %s\n", checkResp.Message)
		}
	}

	return nil
}

func handleChecksStatus(cmd *cobra.Command, args []string) error {
	var checkID string
	if len(args) > 0 {
		checkID = args[0]
	} else {
		return errors.New(errors.ErrValidation, "check ID is required")
	}

	client, conn, err := getCoreClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		CheckId string `json:"check_id"`
	}{
		CheckId: checkID,
	}

	resp, err := client.GetCheckStatus(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	statusResp := resp.(*GetCheckStatusResponse)

	fmt.Printf("Check Status: %s\n", statusResp.CheckId)
	fmt.Printf("Name: %s\n", statusResp.Name)
	fmt.Printf("Type: %s\n", statusResp.Type)
	fmt.Printf("Status: %s\n", statusResp.Status)
	fmt.Printf("Last Check: %s\n", statusResp.LastCheck.Format(time.RFC3339))
	fmt.Printf("Next Check: %s\n", statusResp.NextCheck.Format(time.RFC3339))
	fmt.Printf("Success Rate: %.2f%%\n", statusResp.SuccessRate)
	fmt.Printf("Total Checks: %d\n", statusResp.TotalChecks)
	fmt.Printf("Failed Checks: %d\n", statusResp.FailedChecks)

	if viper.GetBool("verbose") {
		fmt.Printf("URL: %s\n", statusResp.Url)
		fmt.Printf("Interval: %s\n", statusResp.Interval)
		fmt.Printf("Timeout: %ds\n", statusResp.Timeout)
		fmt.Printf("Tenant: %s\n", statusResp.TenantId)
	}

	return nil
}

func handleChecksHistory(cmd *cobra.Command, args []string) error {
	var checkID string
	if len(args) > 0 {
		checkID = args[0]
	} else {
		return errors.New(errors.ErrValidation, "check ID is required")
	}

	limit, _ := cmd.Flags().GetInt("limit")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")

	client, conn, err := getCoreClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		CheckId string `json:"check_id"`
		Limit   int32  `json:"limit"`
		From    *time.Time `json:"from,omitempty"`
		To      *time.Time `json:"to,omitempty"`
	}{
		CheckId: checkID,
		Limit:   int32(limit),
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

	resp, err := client.GetCheckHistory(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	historyResp := resp.(*GetCheckHistoryResponse)

	if len(historyResp.Results) == 0 {
		fmt.Println("No history found for this check")
		return nil
	}

	outputFormat := viper.GetString("output")
	switch outputFormat {
	case "json":
		fmt.Println("[")
		for i, result := range historyResp.Results {
			if i > 0 {
				fmt.Println(",")
			}
			fmt.Printf(`  {"timestamp": "%s", "status": "%s", "response_time": %d, "message": "%s"}`,
				result.Timestamp.Format(time.RFC3339),
				result.Status,
				result.ResponseTime,
				result.Message)
		}
		fmt.Println("\n]")
	default:
		fmt.Printf("Check History for %s:\n", checkID)
		fmt.Printf("%-20s %-10s %-15s %s\n", "Timestamp", "Status", "Response Time", "Message")
		fmt.Println("----------------------------------------------------------------")
		
		for _, result := range historyResp.Results {
			timestamp := result.Timestamp.Format("2006-01-02 15:04:05")
			status := result.Status
			responseTime := fmt.Sprintf("%dms", result.ResponseTime)
			message := result.Message
			
			if len(message) > 50 {
				message = message[:47] + "..."
			}
			
			fmt.Printf("%-20s %-10s %-15s %s\n", timestamp, status, responseTime, message)
		}
	}

	fmt.Printf("\nTotal: %d results\n", len(historyResp.Results))
	return nil
}

func handleChecksList(cmd *cobra.Command, args []string) error {
	status, _ := cmd.Flags().GetString("status")
	checkType, _ := cmd.Flags().GetString("type")
	tenant, _ := cmd.Flags().GetString("tenant")

	client, conn, err := getCoreClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		Status  string `json:"status"`
		Type    string `json:"type"`
		TenantId string `json:"tenant_id"`
	}{
		Status:  status,
		Type:    checkType,
		TenantId: tenant,
	}

	resp, err := client.ListChecks(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	checksResp := resp.(*ListChecksResponse)

	if len(checksResp.Checks) == 0 {
		fmt.Println("No checks found")
		return nil
	}

	outputFormat := viper.GetString("output")
	switch outputFormat {
	case "json":
		fmt.Println("[")
		for i, check := range checksResp.Checks {
			if i > 0 {
				fmt.Println(",")
			}
			fmt.Printf(`  {"id": "%s", "name": "%s", "type": "%s", "status": "%s", "url": "%s"}`,
				check.CheckId, check.Name, check.Type, check.Status, check.Url)
		}
		fmt.Println("\n]")
	default:
		fmt.Printf("Checks (%d total):\n", len(checksResp.Checks))
		fmt.Printf("%-20s %-15s %-10s %-15s %s\n", "ID", "Name", "Type", "Status", "URL")
		fmt.Println("--------------------------------------------------------------------------------")
		
		for _, check := range checksResp.Checks {
			id := check.CheckId
			if len(id) > 18 {
				id = id[:15] + "..."
			}
			
			name := check.Name
			if len(name) > 13 {
				name = name[:10] + "..."
			}
			
			status := check.Status
			url := check.Url
			if len(url) > 30 {
				url = url[:27] + "..."
			}
			
			fmt.Printf("%-20s %-15s %-10s %-15s %s\n", id, name, check.Type, status, url)
		}
	}

	return nil
}
