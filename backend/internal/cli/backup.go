package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/restic"
)

var backupCmd = &cobra.Command{
	Use:   "backup [paths...]",
	Short: "Create a backup (owner only)",
	Long:  `Create a new backup of the specified paths to the restic repository.`,
	Example: `  airgapper backup ~/Documents ~/Photos
  airgapper backup /home/alice/important`,
	Args: cobra.MinimumNArgs(1),
	RunE: runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)
}

func runBackup(cmd *cobra.Command, args []string) error {
	if err := RequireOwner(); err != nil {
		return err
	}

	if cfg.Password == "" {
		return fmt.Errorf("no password found - this config may be corrupted")
	}

	fmt.Print("\n=== Creating Backup ===\n\n")
	fmt.Printf("Repository: %s\n", cfg.RepoURL)
	fmt.Printf("Paths: %s\n\n", strings.Join(args, ", "))

	if !restic.IsInstalled() {
		return fmt.Errorf("restic is not installed")
	}

	client := restic.NewClient(cfg.RepoURL, cfg.Password)
	if err := client.Backup(args, []string{"airgapper"}); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Record activity for dead man's switch
	cfg.RecordActivity()

	fmt.Println("Backup complete!")
	return nil
}

var snapshotsCmd = &cobra.Command{
	Use:   "snapshots",
	Short: "List snapshots (requires password)",
	Long:  `List all backup snapshots in the repository.`,
	RunE:  runSnapshots,
}

func init() {
	rootCmd.AddCommand(snapshotsCmd)
}

func runSnapshots(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	if !cfg.IsOwner() {
		fmt.Print("\n=== Snapshots ===\n\n")
		fmt.Printf("Repository: %s\n\n", cfg.RepoURL)
		fmt.Println("Warning: As a backup host, you cannot list snapshots.")
		fmt.Println("   The data is encrypted and you don't have the key.")
		return nil
	}

	if cfg.Password == "" {
		return fmt.Errorf("no password found")
	}

	fmt.Print("\n=== Snapshots ===\n\n")
	fmt.Printf("Repository: %s\n\n", cfg.RepoURL)

	client := restic.NewClient(cfg.RepoURL, cfg.Password)
	output, err := client.Snapshots()
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	fmt.Println(output)
	return nil
}
