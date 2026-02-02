package cli

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/emergency"
	"github.com/lcrostarosa/airgapper/backend/internal/sss"
)

// --- Heartbeat Command ---

var heartbeatCmd = &cobra.Command{
	Use:   "heartbeat",
	Short: "Record proof of life (resets dead man's switch timer)",
	Long: `Record activity to reset the dead man's switch timer.

This command updates your last activity timestamp, preventing
the dead man's switch from triggering.`,
	RunE: runHeartbeat,
}

func init() {
	rootCmd.AddCommand(heartbeatCmd)
}

func runHeartbeat(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	dms := cfg.Emergency.GetDeadManSwitch()
	if !dms.IsEnabled() {
		PrintInfo("Dead man's switch is not enabled.")
		PrintInfo("\nTo enable, reinitialize with:")
		PrintInfo("  airgapper init --dead-man-switch 180d ...")
		return nil
	}

	if err := cfg.RecordActivity(); err != nil {
		return fmt.Errorf("failed to record activity: %w", err)
	}

	PrintSuccess("Heartbeat recorded!")
	PrintInfo("Last activity: %s", dms.LastActivity.Format("2006-01-02 15:04:05"))
	PrintInfo("Inactivity threshold: %d days", dms.InactivityDays)
	PrintInfo("Days until trigger: %d", dms.DaysUntilTrigger())

	return nil
}

// --- Export Share Command ---

var exportShareCmd = &cobra.Command{
	Use:   "export-share",
	Short: "Re-export a specific key share (for lost custodian shares)",
	Long: `Re-export a specific key share for distribution to custodians.

Use this when a custodian has lost their share and needs a new copy.`,
	Example: `  airgapper export-share --index 3`,
	RunE:    runExportShare,
}

func init() {
	exportShareCmd.Flags().Int("index", 0, "Share index to export (required)")
	exportShareCmd.MarkFlagRequired("index")
	rootCmd.AddCommand(exportShareCmd)
}

func runExportShare(cmd *cobra.Command, args []string) error {
	if err := RequireOwner(); err != nil {
		return err
	}

	if cfg.Password == "" {
		return fmt.Errorf("no password found - cannot regenerate shares")
	}

	shareIndex, _ := cmd.Flags().GetInt("index")

	k := cfg.GetRecoveryThreshold()
	n := cfg.GetRecoveryTotalShares()

	if shareIndex > n || shareIndex < 1 {
		return fmt.Errorf("share index %d is out of range (1-%d)", shareIndex, n)
	}

	// Regenerate shares
	shares, err := sss.Split([]byte(cfg.Password), k, n)
	if err != nil {
		return fmt.Errorf("failed to regenerate shares: %w", err)
	}

	// Find requested share
	var targetShare *sss.Share
	for i := range shares {
		if shares[i].Index == byte(shareIndex) {
			targetShare = &shares[i]
			break
		}
	}

	if targetShare == nil {
		return fmt.Errorf("share index %d not found", shareIndex)
	}

	PrintHeader(fmt.Sprintf("Exporting Share %d", shareIndex))
	fmt.Println()
	PrintInfo("Share: %s", hex.EncodeToString(targetShare.Data))
	PrintInfo("Index: %d", targetShare.Index)
	PrintInfo("Repo:  %s", cfg.RepoURL)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	PrintWarning("This share is part of a %d-of-%d scheme.", k, n)
	PrintInfo("   Any %d shares can decrypt your backups - store securely!", k)

	return nil
}

// --- Override Command (parent) ---

var overrideCmd = &cobra.Command{
	Use:   "override",
	Short: "Manage emergency override keys",
	Long:  `Configure and manage emergency override keys for bypassing normal consensus requirements.`,
}

var overrideSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate a new override key",
	Long: `Generate a new emergency override key.

The key will be displayed once and must be stored securely.
It cannot be recovered if lost.`,
	RunE: runOverrideSetup,
}

var overrideAllowCmd = &cobra.Command{
	Use:   "allow <type>",
	Short: "Allow a specific override type",
	Long: `Allow a specific type of emergency override.

Available types:
  restore-without-consensus  - Restore without peer approval
  delete-without-consensus   - Delete without peer approval
  bypass-retention           - Bypass retention period
  bypass-dead-man-switch     - Bypass dead man's switch
  force-unlock               - Force unlock any operation`,
	Args: cobra.ExactArgs(1),
	RunE: runOverrideAllow,
}

var overrideDenyCmd = &cobra.Command{
	Use:   "deny <type>",
	Short: "Deny a specific override type",
	Args:  cobra.ExactArgs(1),
	RunE:  runOverrideDeny,
}

var overrideListCmd = &cobra.Command{
	Use:   "list",
	Short: "List allowed override types",
	RunE:  runOverrideList,
}

var overrideAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View override audit log",
	RunE:  runOverrideAudit,
}

func init() {
	overrideCmd.AddCommand(overrideSetupCmd)
	overrideCmd.AddCommand(overrideAllowCmd)
	overrideCmd.AddCommand(overrideDenyCmd)
	overrideCmd.AddCommand(overrideListCmd)
	overrideCmd.AddCommand(overrideAuditCmd)
	rootCmd.AddCommand(overrideCmd)
}

func runOverrideSetup(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	if cfg.Emergency.GetOverride().HasKey() {
		PrintWarning("Override key already configured!")
		PrintInfo("   To reset, remove ~/.airgapper/config.json and reinitialize.")
		return nil
	}

	e := cfg.EnsureEmergency()
	if e.Override == nil {
		e.WithOverrides()
	}

	key, err := e.Override.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate override key: %w", err)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	PrintHeader("Override Key Generated")
	fmt.Println()
	PrintInfo("Override Key: %s", key)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	PrintWarning("IMPORTANT: Store this key securely!")
	PrintInfo("   - This key is shown ONCE and cannot be recovered")
	PrintInfo("   - Store in a safe deposit box, with a lawyer, etc.")
	PrintInfo("   - Anyone with this key can bypass security controls")
	fmt.Println()
	PrintInfo("Use with:")
	PrintInfo("  airgapper restore --override-key %s --reason \"...\"", key)

	return nil
}

func runOverrideAllow(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	overrideType := args[0]

	e := cfg.EnsureEmergency()
	if e.Override == nil {
		e.WithOverrides()
	}

	e.Override.AllowType(emergency.OverrideType(overrideType))

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	PrintSuccess("Override type '%s' is now allowed", overrideType)
	return nil
}

func runOverrideDeny(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	overrideType := args[0]

	override := cfg.Emergency.GetOverride()
	if override == nil {
		PrintInfo("No override configuration found")
		return nil
	}

	override.DenyType(emergency.OverrideType(overrideType))

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	PrintSuccess("Override type '%s' is now denied", overrideType)
	return nil
}

func runOverrideList(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	o := cfg.Emergency.GetOverride()
	if !o.IsEnabled() {
		PrintInfo("Override system is not configured.")
		fmt.Println()
		PrintInfo("To enable, run:")
		PrintInfo("  airgapper override setup")
		return nil
	}

	PrintHeader("Override Configuration")
	PrintInfo("Enabled: %v", o.Enabled)
	PrintInfo("Key configured: %v", o.HasKey())
	PrintInfo("Require reason: %v", o.RequireReason)
	PrintInfo("Notify on override: %v", o.NotifyOnUse)
	fmt.Println()
	PrintInfo("Allowed override types:")
	if len(o.AllowedTypes) == 0 {
		PrintInfo("  (none)")
	} else {
		for _, ot := range o.AllowedTypes {
			PrintInfo("  ✅ %s", ot)
		}
	}
	return nil
}

func runOverrideAudit(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	auditPath := cfg.ConfigDir + "/override-audit.log"
	data, err := os.ReadFile(auditPath)
	if err != nil {
		if os.IsNotExist(err) {
			PrintInfo("No override audit log found (no overrides have been used).")
			return nil
		}
		return err
	}

	PrintHeader("Override Audit Log")
	fmt.Println(string(data))
	return nil
}

// --- Notify Command (parent) ---

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
	RunE: runNotifyAdd,
}

var notifyRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a notification provider",
	Args:  cobra.ExactArgs(1),
	RunE:  runNotifyRemove,
}

var notifyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured notification providers",
	RunE:  runNotifyList,
}

var notifyTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Send a test notification",
	RunE:  runNotifyTest,
}

var notifyEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Configure which events trigger notifications",
	RunE:  runNotifyEvents,
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

	// Event flags
	ef := notifyEventsCmd.Flags()
	ef.Bool("all", false, "Enable all events")
	ef.Bool("none", false, "Disable all events")
	ef.Bool("backup-started", false, "Notify on backup start")
	ef.Bool("backup-completed", false, "Notify on backup completion")
	ef.Bool("backup-failed", false, "Notify on backup failure")
	ef.Bool("restore-requested", false, "Notify on restore request")
	ef.Bool("restore-approved", false, "Notify on restore approval")
	ef.Bool("restore-denied", false, "Notify on restore denial")
	ef.Bool("emergency-triggered", false, "Notify on emergency trigger")

	notifyCmd.AddCommand(notifyAddCmd)
	notifyCmd.AddCommand(notifyRemoveCmd)
	notifyCmd.AddCommand(notifyListCmd)
	notifyCmd.AddCommand(notifyTestCmd)
	notifyCmd.AddCommand(notifyEventsCmd)
	rootCmd.AddCommand(notifyCmd)
}

func runNotifyAdd(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	providerType := args[0]
	f := cmd.Flags()
	dryRun, _ := f.GetBool("dry-run")
	priority, _ := f.GetString("priority")

	// Build settings from flags
	settings := make(map[string]string)
	settingKeys := []string{
		"api-token", "user-key", "server", "topic", "auth-token",
		"url", "method", "webhook-url", "channel", "smtp-host",
		"smtp-port", "from", "to", "username", "password",
	}

	for _, key := range settingKeys {
		val, _ := f.GetString(key)
		if val != "" {
			storageKey := strings.ReplaceAll(key, "-", "_")
			settings[storageKey] = val
		}
	}

	e := cfg.EnsureEmergency()
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
		PrintInfo("Dry-run: Would add notification provider:")
		PrintInfo("  ID: %s", providerID)
		PrintInfo("  Type: %s", providerType)
		PrintInfo("  Priority: %s", priority)
		return nil
	}

	e.Notify.AddProvider(providerID, provider)

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	PrintSuccess("Added notification provider: %s (%s)", providerID, providerType)
	return nil
}

func runNotifyRemove(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	providerID := args[0]
	notify := cfg.Emergency.GetNotify()

	if !notify.HasProviders() {
		return fmt.Errorf("no notification providers configured")
	}

	notify.RemoveProvider(providerID)

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	PrintSuccess("Removed notification provider: %s", providerID)
	return nil
}

func runNotifyList(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	notify := cfg.Emergency.GetNotify()
	if !notify.HasProviders() {
		PrintInfo("No notification providers configured.")
		fmt.Println()
		PrintInfo("Add a provider with:")
		PrintInfo("  airgapper notify add pushover --api-token xxx --user-key yyy")
		PrintInfo("  airgapper notify add ntfy --server https://ntfy.sh --topic mybackups")
		return nil
	}

	PrintHeader("Notification Providers")
	for id, provider := range notify.Providers {
		status := "✅"
		if !provider.Enabled {
			status = "❌"
		}
		PrintInfo("%s %s (%s) - Priority: %s", status, id, provider.Type, provider.Priority)
	}
	return nil
}

func runNotifyTest(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	notify := cfg.Emergency.GetNotify()
	if !notify.HasProviders() {
		return fmt.Errorf("no notification providers configured")
	}

	PrintInfo("Sending test notification...")
	PrintInfo("Configured providers: %d", notify.ProviderCount())
	fmt.Println()
	PrintWarning("Note: Full notification delivery requires the notification service.")
	PrintInfo("   Test messages will be sent when 'airgapper serve' is running.")
	return nil
}

func runNotifyEvents(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	e := cfg.EnsureEmergency()
	if e.Notify == nil {
		e.Notify = &emergency.NotifyConfig{
			Enabled:   true,
			Providers: make(map[string]emergency.Provider),
		}
	}

	f := cmd.Flags()
	all, _ := f.GetBool("all")
	none, _ := f.GetBool("none")

	// If no flags, show current config
	if !all && !none && !f.Changed("backup-started") && !f.Changed("backup-completed") {
		events := e.Notify.Events
		PrintHeader("Notification Events")
		PrintInfo("Backup Started:      %v", events.BackupStarted)
		PrintInfo("Backup Completed:    %v", events.BackupCompleted)
		PrintInfo("Backup Failed:       %v", events.BackupFailed)
		PrintInfo("Restore Requested:   %v", events.RestoreRequested)
		PrintInfo("Restore Approved:    %v", events.RestoreApproved)
		PrintInfo("Restore Denied:      %v", events.RestoreDenied)
		PrintInfo("Deletion Requested:  %v", events.DeletionRequested)
		PrintInfo("Deletion Approved:   %v", events.DeletionApproved)
		PrintInfo("Consensus Received:  %v", events.ConsensusReceived)
		PrintInfo("Emergency Triggered: %v", events.EmergencyTriggered)
		PrintInfo("Dead Man Warning:    %v", events.DeadManWarning)
		PrintInfo("Heartbeat Missed:    %v", events.HeartbeatMissed)
		return nil
	}

	if all {
		e.Notify.EnableAllEvents()
	} else if none {
		e.Notify.DisableAllEvents()
	} else {
		// Set individual events
		if v, _ := f.GetBool("backup-started"); v {
			e.Notify.Events.BackupStarted = true
		}
		if v, _ := f.GetBool("backup-completed"); v {
			e.Notify.Events.BackupCompleted = true
		}
		if v, _ := f.GetBool("backup-failed"); v {
			e.Notify.Events.BackupFailed = true
		}
		if v, _ := f.GetBool("restore-requested"); v {
			e.Notify.Events.RestoreRequested = true
		}
		if v, _ := f.GetBool("restore-approved"); v {
			e.Notify.Events.RestoreApproved = true
		}
		if v, _ := f.GetBool("restore-denied"); v {
			e.Notify.Events.RestoreDenied = true
		}
		if v, _ := f.GetBool("emergency-triggered"); v {
			e.Notify.Events.EmergencyTriggered = true
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	PrintSuccess("Event notification settings updated")
	return nil
}
