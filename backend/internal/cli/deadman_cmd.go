package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// --- Heartbeat Command ---

var heartbeatCmd = &cobra.Command{
	Use:   "heartbeat",
	Short: "Record proof of life (resets dead man's switch timer)",
	Long: `Record activity to reset the dead man's switch timer.

This command updates your last activity timestamp, preventing
the dead man's switch from triggering.`,
	RunE: runners.Config().Wrap(runHeartbeat),
}

func init() {
	rootCmd.AddCommand(heartbeatCmd)
}

func runHeartbeat(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	dms := ctx.Config.Emergency.GetDeadManSwitch()
	if !dms.IsEnabled() {
		logging.Info("Dead man's switch is not enabled")
		logging.Info("To enable, reinitialize with: airgapper init --dead-man-switch 180d ...")
		return nil
	}

	ctx.Config.Emergency.GetDeadManSwitch().RecordActivity()
	if err := ctx.SaveConfig(); err != nil {
		return fmt.Errorf("failed to record activity: %w", err)
	}

	logging.Info("Heartbeat recorded",
		logging.String("lastActivity", dms.LastActivity.Format("2006-01-02 15:04:05")),
		logging.Int("inactivityThreshold", dms.InactivityDays),
		logging.Int("daysUntilTrigger", dms.DaysUntilTrigger()))

	return nil
}
