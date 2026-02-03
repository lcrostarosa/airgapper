package cli

import (
	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/emergency"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// --- Notify Events Subcommand ---

var notifyEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Configure which events trigger notifications",
	RunE:  runners.Config().Wrap(runNotifyEvents),
}

func init() {
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

	notifyCmd.AddCommand(notifyEventsCmd)
}

func runNotifyEvents(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	e := ctx.Config.EnsureEmergency()
	if e.Notify == nil {
		e.Notify = &emergency.NotifyConfig{
			Enabled:   true,
			Providers: make(map[string]emergency.Provider),
		}
	}

	flags := runner.Flags(cmd)
	all := flags.Bool("all")
	none := flags.Bool("none")

	// If no flags, show current config
	if !all && !none && !flags.Changed("backup-started") && !flags.Changed("backup-completed") {
		events := e.Notify.Events
		logging.Info("Notification events",
			logging.Bool("backupStarted", events.BackupStarted),
			logging.Bool("backupCompleted", events.BackupCompleted),
			logging.Bool("backupFailed", events.BackupFailed),
			logging.Bool("restoreRequested", events.RestoreRequested),
			logging.Bool("restoreApproved", events.RestoreApproved),
			logging.Bool("restoreDenied", events.RestoreDenied),
			logging.Bool("deletionRequested", events.DeletionRequested),
			logging.Bool("deletionApproved", events.DeletionApproved),
			logging.Bool("consensusReceived", events.ConsensusReceived),
			logging.Bool("emergencyTriggered", events.EmergencyTriggered),
			logging.Bool("deadManWarning", events.DeadManWarning),
			logging.Bool("heartbeatMissed", events.HeartbeatMissed))
		return nil
	}

	if all {
		e.Notify.EnableAllEvents()
	} else if none {
		e.Notify.DisableAllEvents()
	} else {
		// Set individual events
		if flags.Bool("backup-started") {
			e.Notify.Events.BackupStarted = true
		}
		if flags.Bool("backup-completed") {
			e.Notify.Events.BackupCompleted = true
		}
		if flags.Bool("backup-failed") {
			e.Notify.Events.BackupFailed = true
		}
		if flags.Bool("restore-requested") {
			e.Notify.Events.RestoreRequested = true
		}
		if flags.Bool("restore-approved") {
			e.Notify.Events.RestoreApproved = true
		}
		if flags.Bool("restore-denied") {
			e.Notify.Events.RestoreDenied = true
		}
		if flags.Bool("emergency-triggered") {
			e.Notify.Events.EmergencyTriggered = true
		}
	}

	if err := ctx.SaveConfig(); err != nil {
		return err
	}

	logging.Info("Event notification settings updated")
	return nil
}
