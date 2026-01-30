package cmd

import (
	"fmt"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/restic"
	"github.com/spf13/cobra"
)

var snapshotsCmd = &cobra.Command{
	Use:   "snapshots",
	Short: "List snapshots (requires password)",
	Long: `List all snapshots in the backup repository.

Only the data owner can list snapshots, as they hold the password.`,
	RunE: runSnapshots,
}

func init() {
	rootCmd.AddCommand(snapshotsCmd)
}

func runSnapshots(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsOwner() {
		fmt.Println("📋 Snapshots")
		fmt.Printf("Repository: %s\n\n", cfg.RepoURL)
		fmt.Println("⚠️  As a backup host, you cannot list snapshots.")
		fmt.Println("   The data is encrypted and you don't have the key.")
		return nil
	}

	if cfg.Password == "" {
		return fmt.Errorf("no password found")
	}

	fmt.Println("📋 Snapshots")
	fmt.Printf("Repository: %s\n\n", cfg.RepoURL)

	client := restic.NewClient(cfg.RepoURL, cfg.Password)
	output, err := client.Snapshots()
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	fmt.Println(output)
	return nil
}
