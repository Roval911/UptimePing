package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"UptimePingPlatform/pkg/errors"
	// notificationv1 "UptimePingPlatform/proto/notification/v1"
)

var notificationCmd = &cobra.Command{
	Use:   "notification",
	Short: "Управление уведомлениями",
	Long: `Команды для управления уведомлениями:
управление каналами уведомлений и тестирование отправки.`,
}

// notificationChannelsCmd represents the notification channels command
var notificationChannelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Управление каналами уведомлений",
	Long:  `Команды для управления каналами уведомлений.`,
}

// notificationChannelsAddCmd represents the notification channels add command
var notificationChannelsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Добавить канал уведомлений",
	Long:  `Добавляет новый канал уведомлений с указанными параметрами.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleNotificationChannelsAdd(cmd, args)
	},
}

// notificationChannelsRemoveCmd represents the notification channels remove command
var notificationChannelsRemoveCmd = &cobra.Command{
	Use:   "remove [channel-id]",
	Short: "Удалить канал уведомлений",
	Long:  `Удаляет указанный канал уведомлений.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleNotificationChannelsRemove(cmd, args)
	},
}

// notificationChannelsListCmd represents the notification channels list command
var notificationChannelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать список каналов уведомлений",
	Long:  `Отображает все настроенные каналы уведомлений.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleNotificationChannelsList(cmd, args)
	},
}

// notificationTestCmd represents the notification test command
var notificationTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Тестировать уведомление",
	Long:  `Отправляет тестовое уведомление через указанный канал.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleNotificationTest(cmd, args)
	},
}

func init() {
	notificationCmd.AddCommand(notificationChannelsCmd)
	notificationCmd.AddCommand(notificationTestCmd)

	notificationChannelsCmd.AddCommand(notificationChannelsAddCmd)
	notificationChannelsCmd.AddCommand(notificationChannelsRemoveCmd)
	notificationChannelsCmd.AddCommand(notificationChannelsListCmd)

	// Notification channels add flags
	notificationChannelsAddCmd.Flags().StringP("name", "n", "", "название канала")
	notificationChannelsAddCmd.Flags().StringP("type", "t", "", "тип канала (email, slack, telegram, webhook, sms)")
	notificationChannelsAddCmd.Flags().StringP("address", "a", "", "адрес канала")
	notificationChannelsAddCmd.Flags().StringP("config", "c", "", "конфигурация канала (JSON)")
	notificationChannelsAddCmd.Flags().BoolP("enabled", "e", true, "включить канал")

	// Notification test flags
	notificationTestCmd.Flags().StringP("channel", "c", "", "ID канала для теста")
	notificationTestCmd.Flags().StringP("message", "m", "Test notification from UptimePing CLI", "текст сообщения")
	notificationTestCmd.Flags().StringP("title", "t", "Test Notification", "заголовок уведомления")
	notificationTestCmd.Flags().StringP("severity", "s", "info", "важность (info, warning, error, critical)")
}

// getNotificationClient creates a gRPC client for notification service
func getNotificationClient() (*MockNotificationClient, *grpc.ClientConn, error) {
	return getMockNotificationClient()
}

func handleNotificationChannelsAdd(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	channelType, _ := cmd.Flags().GetString("type")
	address, _ := cmd.Flags().GetString("address")
	config, _ := cmd.Flags().GetString("config")
	enabled, _ := cmd.Flags().GetBool("enabled")

	if name == "" {
		return errors.New(errors.ErrValidation, "channel name is required")
	}

	if channelType == "" {
		return errors.New(errors.ErrValidation, "channel type is required")
	}

	if address == "" {
		return errors.New(errors.ErrValidation, "channel address is required")
	}

	client, conn, err := getNotificationClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Address string `json:"address"`
		Config  string `json:"config"`
		Enabled bool   `json:"enabled"`
	}{
		Name:    name,
		Type:    channelType,
		Address: address,
		Config:  config,
		Enabled: enabled,
	}

	resp, err := client.CreateChannel(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	channelResp := resp.(*CreateChannelResponse)

	fmt.Printf("✅ Notification channel '%s' created successfully\n", name)
	fmt.Printf("Channel ID: %s\n", channelResp.ChannelId)
	if viper.GetBool("verbose") {
		fmt.Printf("Type: %s\n", channelType)
		fmt.Printf("Address: %s\n", address)
		fmt.Printf("Enabled: %t\n", enabled)
	}

	return nil
}

func handleNotificationChannelsRemove(cmd *cobra.Command, args []string) error {
	channelID := args[0]

	client, conn, err := getNotificationClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		ChannelId string `json:"channel_id"`
	}{
		ChannelId: channelID,
	}

	_, err = client.DeleteChannel(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	fmt.Printf("✅ Notification channel '%s' removed successfully\n", channelID)
	return nil
}

func handleNotificationChannelsList(cmd *cobra.Command, args []string) error {
	client, conn, err := getNotificationClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct{}{}
	resp, err := client.ListChannels(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	channelsResp := resp.(*ListChannelsResponse)

	if len(channelsResp.Channels) == 0 {
		fmt.Println("No notification channels found")
		return nil
	}

	outputFormat := viper.GetString("output")
	switch outputFormat {
	case "json":
		fmt.Println("[")
		for i, channel := range channelsResp.Channels {
			if i > 0 {
				fmt.Println(",")
			}
			fmt.Printf(`  {"id": "%s", "name": "%s", "type": "%s", "address": "%s", "enabled": %t}`,
				channel.ChannelId, channel.Name, channel.Type, channel.Address, channel.Enabled)
		}
		fmt.Println("\n]")
	default:
		fmt.Printf("Notification Channels (%d total):\n", len(channelsResp.Channels))
		fmt.Printf("%-20s %-20s %-12s %-30s %-10s\n", "ID", "Name", "Type", "Address", "Enabled")
		fmt.Println("--------------------------------------------------------------------------------")

		for _, channel := range channelsResp.Channels {
			id := channel.ChannelId
			if len(id) > 18 {
				id = id[:15] + "..."
			}

			name := channel.Name
			if len(name) > 18 {
				name = name[:15] + "..."
			}

			address := channel.Address
			if len(address) > 28 {
				address = address[:25] + "..."
			}

			enabled := "No"
			if channel.Enabled {
				enabled = "Yes"
			}

			fmt.Printf("%-20s %-20s %-12s %-30s %-10s\n",
				id, name, channel.Type, address, enabled)
		}
	}

	return nil
}

func handleNotificationTest(cmd *cobra.Command, args []string) error {
	channelID, _ := cmd.Flags().GetString("channel")
	message, _ := cmd.Flags().GetString("message")
	title, _ := cmd.Flags().GetString("title")
	severity, _ := cmd.Flags().GetString("severity")

	if channelID == "" {
		return errors.New(errors.ErrValidation, "channel ID is required")
	}

	client, conn, err := getNotificationClient()
	if err != nil {
		return handleError(err, cmd)
	}
	if conn != nil {
		defer conn.Close()
	}

	ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
	defer cancel()

	req := &struct {
		ChannelId string `json:"channel_id"`
		Title     string `json:"title"`
		Message   string `json:"message"`
		Severity  string `json:"severity"`
		Test      bool   `json:"test"`
	}{
		ChannelId: channelID,
		Title:     title,
		Message:   message,
		Severity:  severity,
		Test:      true,
	}

	resp, err := client.SendNotification(ctx, req)
	if err != nil {
		return handleError(err, cmd)
	}

	notificationResp := resp.(*SendNotificationResponse)

	fmt.Printf("✅ Test notification sent successfully\n")
	fmt.Printf("Notification ID: %s\n", notificationResp.NotificationId)
	fmt.Printf("Status: %s\n", notificationResp.Status)

	if viper.GetBool("verbose") {
		fmt.Printf("Sent at: %s\n", notificationResp.SentAt.Format(time.RFC3339))
		fmt.Printf("Channel: %s\n", channelID)
		fmt.Printf("Title: %s\n", title)
		fmt.Printf("Message: %s\n", message)
		fmt.Printf("Severity: %s\n", severity)
	}

	return nil
}
