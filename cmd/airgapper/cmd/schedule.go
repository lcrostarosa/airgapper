package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/scheduler"
	"github.com/spf13/cobra"
)

var (
	scheduleShow  bool
	scheduleClear bool
	scheduleSet   string
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule [paths...]",
	Short: "Configure backup schedule",
	Long: `Configure automatic backup scheduling.

Schedule formats:
  daily            Daily at 2 AM
  hourly           Every hour
  weekly           Weekly on Sunday at 2 AM
  every 4h         Every 4 hours
  every 30m        Every 30 minutes
  0 3 * * *        Cron format (3 AM daily)

Examples:
  airgapper schedule --show
  airgapper schedule --set daily ~/Documents
  airgapper schedule --set hourly ~/Documents ~/Pictures
  airgapper schedule --set "0 3 * * *" ~/Documents
  airgapper schedule --clear`,
	RunE: runSchedule,
}

func init() {
	rootCmd.AddCommand(scheduleCmd)

	scheduleCmd.Flags().BoolVar(&scheduleShow, "show", false, "Show current schedule")
	scheduleCmd.Flags().BoolVar(&scheduleClear, "clear", false, "Clear current schedule")
	scheduleCmd.Flags().StringVar(&scheduleSet, "set", "", "Set schedule (daily, hourly, cron expression)")
}

func runSchedule(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsOwner() {
		return fmt.Errorf("only the data owner can configure backup schedule")
	}

	// Default to showing schedule
	if !scheduleClear && scheduleSet == "" {
		scheduleShow = true
	}

	if scheduleShow {
		fmt.Println("📅 Backup Schedule")
		fmt.Println("==================")
		if cfg.BackupSchedule == "" {
			fmt.Println("No schedule configured.")
			fmt.Println()
			fmt.Println("Set a schedule with:")
			fmt.Println("  airgapper schedule --set daily ~/Documents")
			fmt.Println("  airgapper schedule --set hourly ~/Documents ~/Pictures")
			fmt.Println("  airgapper schedule --set \"0 3 * * *\" ~/Documents  # Cron: 3 AM daily")
			fmt.Println("  airgapper schedule --set \"every 4h\" ~/Documents   # Every 4 hours")
		} else {
			fmt.Printf("Schedule: %s\n", cfg.BackupSchedule)
			if len(cfg.BackupPaths) > 0 {
				fmt.Printf("Paths:    %s\n", strings.Join(cfg.BackupPaths, ", "))
			} else {
				fmt.Println("Paths:    (none configured)")
			}

			// Show next run time
			sched, err := scheduler.ParseSchedule(cfg.BackupSchedule)
			if err == nil {
				nextRun := sched.NextRun(time.Now())
				fmt.Printf("Next run: %s (in %s)\n", nextRun.Format("2006-01-02 15:04:05"), scheduler.FormatDuration(time.Until(nextRun)))
			}
		}
		return nil
	}

	if scheduleClear {
		cfg.BackupSchedule = ""
		cfg.BackupPaths = nil
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Println("✅ Schedule cleared.")
		return nil
	}

	if scheduleSet != "" {
		// Validate schedule
		sched, err := scheduler.ParseSchedule(scheduleSet)
		if err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}

		cfg.BackupSchedule = scheduleSet
		if len(args) > 0 {
			cfg.BackupPaths = args
		}

		if err := cfg.Save(); err != nil {
			return err
		}

		fmt.Println("✅ Schedule configured!")
		fmt.Printf("Schedule: %s\n", cfg.BackupSchedule)
		if len(cfg.BackupPaths) > 0 {
			fmt.Printf("Paths:    %s\n", strings.Join(cfg.BackupPaths, ", "))
		}

		nextRun := sched.NextRun(time.Now())
		fmt.Printf("Next run: %s (in %s)\n", nextRun.Format("2006-01-02 15:04:05"), scheduler.FormatDuration(time.Until(nextRun)))
		fmt.Println()
		fmt.Println("To start scheduled backups, run:")
		fmt.Println("  airgapper serve --addr :8080")
		return nil
	}

	return fmt.Errorf("usage: airgapper schedule [--show|--clear|--set <schedule>] [paths...]")
}
