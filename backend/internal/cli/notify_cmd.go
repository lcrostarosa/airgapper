package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/emergency"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Configure notification providers and events",
	Long:  `Configure push notifications for backup events and alerts.`,
}

var notifyAddCmd = &cobra.Command{
	Use:   "add <provider>",
	Short: "Add a notification provider",
	Long: `Add a notification provider for alerts.

Supported providers:
  pushover   - Pushover push notifications
  ntfy       - ntfy.sh notifications
  webhook    - Generic HTTP webhooks
  email      - SMTP email notifications
  slack      - Slack webhooks
  discord    - Discord webhooks`,
	Args: cobra.ExactArgs(1),
	RunE: runners.Config().Wrap(runNotifyAdd),
}

var notifyRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a notification provider",
	Args:  cobra.ExactArgs(1),
	RunE:  runners.Config().Wrap(runNotifyRemove),
}

var notifyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured notification providers",
	RunE:  runners.Config().Wrap(runNotifyList),
}

var notifyTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Send a test notification",
	RunE:  runners.Config().Wrap(runNotifyTest),
}

func init() {
	// Add provider flags
	f := notifyAddCmd.Flags()
	f.String("api-token", "", "API token (for pushover)")
	f.String("user-key", "", "User key (for pushover)")
	f.String("server", "", "Server URL (for ntfy)")
	f.String("topic", "", "Topic (for ntfy)")
	f.String("auth-token", "", "Auth token (for ntfy)")
	f.String("url", "", "Webhook URL")
	f.String("method", "POST", "HTTP method (for webhook)")
	f.String("webhook-url", "", "Webhook URL (for slack/discord)")
	f.String("channel", "", "Channel (for slack)")
	f.String("smtp-host", "", "SMTP host (for email)")
	f.String("smtp-port", "587", "SMTP port (for email)")
	f.String("from", "", "From address (for email)")
	f.String("to", "", "To address (for email)")
	f.String("username", "", "Username (for email)")
	f.String("password", "", "Password (for email)")
	f.String("priority", "normal", "Notification priority (low, normal, high, urgent)")
	f.Bool("dry-run", false, "Preview changes without applying")

	notifyCmd.AddCommand(notifyAddCmd)
	notifyCmd.AddCommand(notifyRemoveCmd)
	notifyCmd.AddCommand(notifyListCmd)
	notifyCmd.AddCommand(notifyTestCmd)
	rootCmd.AddCommand(notifyCmd)
}

func runNotifyAdd(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	providerType := args[0]
	flags := runner.Flags(cmd)
	dryRun := flags.Bool("dry-run")
	priority := flags.String("priority")

	// Build settings from flags
	settings := make(map[string]string)
	settingKeys := []string{
		"api-token", "user-key", "server", "topic", "auth-token",
		"url", "method", "webhook-url", "channel", "smtp-host",
		"smtp-port", "from", "to", "username", "password",
	}

	for _, key := range settingKeys {
		val := flags.String(key)
		if val != "" {
			storageKey := strings.ReplaceAll(key, "-", "_")
			settings[storageKey] = val
		}
	}

	if err := flags.Err(); err != nil {
		return err
	}

	e := ctx.Config.EnsureEmergency()
	if e.Notify == nil {
		e.Notify = &emergency.NotifyConfig{
			Enabled:   true,
			Providers: make(map[string]emergency.Provider),
		}
	}

	providerID := fmt.Sprintf("%s-%d", providerType, len(e.Notify.Providers)+1)
	provider := emergency.Provider{
		Type:     providerType,
		Enabled:  true,
		Settings: settings,
		Priority: priority,
	}

	if dryRun {
		logging.Info("Dry-run: Would add notification provider",
			logging.String("id", providerID),
			logging.String("type", providerType),
			logging.String("priority", priority))
		return nil
	}

	e.Notify.AddProvider(providerID, provider)

	if err := ctx.SaveConfig(); err != nil {
		return err
	}

	logging.Info("Added notification provider",
		logging.String("id", providerID),
		logging.String("type", providerType))
	return nil
}

func runNotifyRemove(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	providerID := args[0]
	notify := ctx.Config.Emergency.GetNotify()

	if !notify.HasProviders() {
		return fmt.Errorf("no notification providers configured")
	}

	notify.RemoveProvider(providerID)

	if err := ctx.SaveConfig(); err != nil {
		return err
	}

	logging.Info("Removed notification provider", logging.String("id", providerID))
	return nil
}

func runNotifyList(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	notify := ctx.Config.Emergency.GetNotify()
	if !notify.HasProviders() {
		logging.Info("No notification providers configured")
		logging.Info("Add a provider with:")
		logging.Info("  airgapper notify add pushover --api-token xxx --user-key yyy")
		logging.Info("  airgapper notify add ntfy --server https://ntfy.sh --topic mybackups")
		return nil
	}

	logging.Info("Notification providers")
	for id, provider := range notify.Providers {
		status := "enabled"
		if !provider.Enabled {
			status = "disabled"
		}
		logging.Info("Provider",
			logging.String("id", id),
			logging.String("type", provider.Type),
			logging.String("status", status),
			logging.String("priority", provider.Priority))
	}
	return nil
}

func runNotifyTest(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	notify := ctx.Config.Emergency.GetNotify()
	if !notify.HasProviders() {
		return fmt.Errorf("no notification providers configured")
	}

	logging.Info("Sending test notification",
		logging.Int("providers", notify.ProviderCount()))
	logging.Warn("Note: Full notification delivery requires the notification service")
	logging.Info("Test messages will be sent when 'airgapper serve' is running")
	return nil
}
