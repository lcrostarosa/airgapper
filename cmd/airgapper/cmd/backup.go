package cmd

import (
	"fmt"
	"strings"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/restic"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup <paths...>",
	Short: "Create a backup (owner only)",
	Long: `Create a backup of the specified paths.

Only the data owner can create backups, as they hold the full password.

Examples:
  airgapper backup ~/Documents
  airgapper backup ~/Documents ~/Pictures ~/Code`,
	Args: cobra.MinimumNArgs(1),
	RunE: runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)
}

func runBackup(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsOwner() {
		return fmt.Errorf("only the data owner can create backups (you are: %s)", cfg.Role)
	}

	if cfg.Password == "" {
		return fmt.Errorf("no password found - this config may be corrupted")
	}

	fmt.Println("📦 Creating Backup")
	fmt.Println("==================")
	fmt.Printf("Repository: %s\n", cfg.RepoURL)
	fmt.Printf("Paths: %s\n\n", strings.Join(args, ", "))

	client := restic.NewClient(cfg.RepoURL, cfg.Password)

	// Check if restic is installed
	if !restic.IsInstalled() {
		return fmt.Errorf("restic is not installed")
	}

	// Run backup
	if err := client.Backup(args, []string{"airgapper"}); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	fmt.Println("\n✅ Backup complete!")

	return nil
}
