package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
)

var backupCmd = &cobra.Command{
	Use:   "backup [paths...]",
	Short: "Create a backup (owner only)",
	Long:  `Create a new backup of the specified paths to the restic repository.`,
	Example: `  airgapper backup ~/Documents ~/Photos
  airgapper backup /home/alice/important`,
	Args: cobra.MinimumNArgs(1),
	RunE: runners.OwnerWithActivity().Use(runner.RequirePassword()).Wrap(runBackup),
}

func init() {
	rootCmd.AddCommand(backupCmd)
}

func runBackup(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	logging.Info("Creating backup",
		logging.String("repository", ctx.Config.RepoURL),
		logging.String("paths", strings.Join(args, ", ")))

	if !restic.IsInstalled() {
		return fmt.Errorf("restic is not installed")
	}

	client := restic.NewClient(ctx.Config.RepoURL, ctx.Config.Password)
	if err := client.Backup(args, []string{"airgapper"}); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	logging.Info("Backup complete")
	return nil
}

var snapshotsCmd = &cobra.Command{
	Use:   "snapshots",
	Short: "List snapshots (requires password)",
	Long:  `List all backup snapshots in the repository.`,
	RunE:  runners.Config().Wrap(runSnapshots),
}

func init() {
	rootCmd.AddCommand(snapshotsCmd)
}

func runSnapshots(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	if !ctx.Config.IsOwner() {
		logging.Info("Snapshots", logging.String("repository", ctx.Config.RepoURL))
		logging.Warn("As a backup host, you cannot list snapshots - the data is encrypted and you don't have the key")
		return nil
	}

	if ctx.Config.Password == "" {
		return fmt.Errorf("no password found")
	}

	logging.Info("Listing snapshots", logging.String("repository", ctx.Config.RepoURL))

	client := restic.NewClient(ctx.Config.RepoURL, ctx.Config.Password)
	output, err := client.Snapshots()
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	logging.Infof("Snapshots:\n%s", output)
	return nil
}
