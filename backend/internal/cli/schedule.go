package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Configure backup schedule",
	Long:  `View or configure the automatic backup schedule.`,
	Example: `  # View current schedule
  airgapper schedule

  # Set daily backups
  airgapper schedule --set daily ~/Documents ~/Photos

  # Set hourly backups
  airgapper schedule --set hourly /important/data

  # Set custom cron schedule (2am daily)
  airgapper schedule --set "0 2 * * *" ~/Documents

  # Clear schedule
  airgapper schedule --clear`,
	RunE: runners.Owner().Wrap(runSchedule),
}

func init() {
	f := scheduleCmd.Flags()
	f.String("set", "", "Set schedule (daily, hourly, weekly, or cron expression)")
	f.Bool("clear", false, "Clear the current schedule")
	rootCmd.AddCommand(scheduleCmd)
}

func runSchedule(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	flags := runner.Flags(cmd)
	clear := flags.Bool("clear")
	setSchedule := flags.String("set")
	if err := flags.Err(); err != nil {
		return err
	}

	if clear {
		return clearSchedule(ctx)
	}

	if setSchedule != "" {
		return setBackupSchedule(ctx, setSchedule, args)
	}

	return showSchedule(ctx)
}

func clearSchedule(ctx *runner.CommandContext) error {
	ctx.Config.BackupSchedule = ""
	ctx.Config.BackupPaths = nil
	if err := ctx.SaveConfig(); err != nil {
		return err
	}
	logging.Info("Schedule cleared")
	return nil
}

func setBackupSchedule(ctx *runner.CommandContext, scheduleExpr string, paths []string) error {
	sched, err := scheduler.ParseSchedule(scheduleExpr)
	if err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	ctx.Config.BackupSchedule = scheduleExpr
	if len(paths) > 0 {
		ctx.Config.BackupPaths = paths
	}

	if err := ctx.SaveConfig(); err != nil {
		return err
	}

	nextRun := sched.NextRun(time.Now())
	logging.Info("Schedule configured",
		logging.String("schedule", ctx.Config.BackupSchedule),
		logging.String("paths", strings.Join(ctx.Config.BackupPaths, ", ")),
		logging.String("nextRun", nextRun.Format("2006-01-02 15:04:05")),
		logging.String("in", scheduler.FormatDuration(time.Until(nextRun))))

	logging.Info("To start scheduled backups, run: airgapper serve")
	return nil
}

func showSchedule(ctx *runner.CommandContext) error {
	logging.Info("Backup schedule")

	if ctx.Config.BackupSchedule == "" {
		logging.Info("No schedule configured")
		logging.Info("Set a schedule with: airgapper schedule --set daily ~/Documents")
		return nil
	}

	logging.Info("Current schedule",
		logging.String("schedule", ctx.Config.BackupSchedule),
		logging.String("paths", strings.Join(ctx.Config.BackupPaths, ", ")))

	sched, err := scheduler.ParseSchedule(ctx.Config.BackupSchedule)
	if err == nil {
		nextRun := sched.NextRun(time.Now())
		logging.Infof("Next run: %s (in %s)", nextRun.Format("2006-01-02 15:04:05"), scheduler.FormatDuration(time.Until(nextRun)))
	}

	return nil
}
