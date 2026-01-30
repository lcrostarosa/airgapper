// Package cmd provides the Cobra command structure for Airgapper
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const version = "0.4.0"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "airgapper",
	Short: "Consensus-based encrypted backup system",
	Long: `Airgapper - Consensus-based encrypted backup

A backup system that splits encryption keys using Shamir's Secret Sharing,
requiring multiple parties to consent before data can be restored.

WORKFLOW (Owner - Alice):
  1. airgapper init --name alice --repo rest:http://bob-nas:8000/alice-backup
  2. Give the displayed shares to your peers
  3. airgapper schedule --set "daily" ~/Documents ~/Pictures
  4. airgapper serve --addr :8080  (runs API + scheduled backups)
  5. When you need to restore:
     airgapper request --snapshot latest --reason "laptop died"
  6. Wait for peer approval, then:
     airgapper restore --request <id> --target /restore/path

WORKFLOW (Host - Bob):
  1. Start restic-rest-server --append-only
  2. airgapper join --name bob --repo rest:http://localhost:8000/alice-backup --share <hex> --index 2
  3. airgapper serve --addr :8080
  4. When Alice requests restore:
     airgapper pending
     airgapper approve <request-id>`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
}
