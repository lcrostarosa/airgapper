package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

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
	RunE: runSchedule,
}

func init() {
	f := scheduleCmd.Flags()
	f.String("set", "", "Set schedule (daily, hourly, weekly, or cron expression)")
	f.Bool("clear", false, "Clear the current schedule")
	rootCmd.AddCommand(scheduleCmd)
}

func runSchedule(cmd *cobra.Command, args []string) error {
	if err := RequireOwner(); err != nil {
		return err
	}

	clear, _ := cmd.Flags().GetBool("clear")
	if clear {
		return clearSchedule()
	}

	setSchedule, _ := cmd.Flags().GetString("set")
	if setSchedule != "" {
		return setBackupSchedule(setSchedule, args)
	}

	return showSchedule()
}

func clearSchedule() error {
	cfg.BackupSchedule = ""
	cfg.BackupPaths = nil
	if err := cfg.Save(); err != nil {
		return err
	}
	printSuccess("Schedule cleared.")
	return nil
}

func setBackupSchedule(scheduleExpr string, paths []string) error {
	sched, err := scheduler.ParseSchedule(scheduleExpr)
	if err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	cfg.BackupSchedule = scheduleExpr
	if len(paths) > 0 {
		cfg.BackupPaths = paths
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	printSuccess("Schedule configured!")
	printInfo("Schedule: %s", cfg.BackupSchedule)
	if len(cfg.BackupPaths) > 0 {
		printInfo("Paths:    %s", strings.Join(cfg.BackupPaths, ", "))
	}

	nextRun := sched.NextRun(time.Now())
	printInfo("Next run: %s (in %s)", nextRun.Format("2006-01-02 15:04:05"), scheduler.FormatDuration(time.Until(nextRun)))
	fmt.Println()
	printInfo("To start scheduled backups, run:")
	printInfo("  airgapper serve")
	return nil
}

func showSchedule() error {
	printHeader("Backup Schedule")

	if cfg.BackupSchedule == "" {
		printInfo("No schedule configured.")
		fmt.Println()
		printInfo("Set a schedule with:")
		printInfo("  airgapper schedule --set daily ~/Documents")
		return nil
	}

	printInfo("Schedule: %s", cfg.BackupSchedule)
	if len(cfg.BackupPaths) > 0 {
		printInfo("Paths:    %s", strings.Join(cfg.BackupPaths, ", "))
	} else {
		printInfo("Paths:    (none configured)")
	}

	sched, err := scheduler.ParseSchedule(cfg.BackupSchedule)
	if err == nil {
		nextRun := sched.NextRun(time.Now())
		printInfo("Next run: %s (in %s)", nextRun.Format("2006-01-02 15:04:05"), scheduler.FormatDuration(time.Until(nextRun)))
	}

	return nil
}
