package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

var (
	// Version is set at build time
	Version = "0.4.0"

	// App state
	cfg    *config.Config
	cfgErr error
)

// rootCmd is the base command
var rootCmd = &cobra.Command{
	Use:   "airgapper",
	Short: "Consensus-based encrypted backup",
	Long: `Airgapper wraps restic and uses Shamir's Secret Sharing to split
backup encryption keys between parties. Neither the data owner nor
the backup host can decrypt/restore backups alone - both must agree.`,
}

// Execute runs the CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// SetVersion sets the version string
func SetVersion(v string) {
	Version = v
	rootCmd.Version = v
}

func init() {
	cobra.OnInitialize(initLogging, initConfig)
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func initLogging() {
	// Initialize logging with defaults; can be reconfigured later based on flags
	logging.InitDefault()
}

func initConfig() {
	cfg, cfgErr = config.Load("")
}

// Config returns the loaded config (may be nil)
func Config() *config.Config {
	return cfg
}

// RequireConfig returns an error if config is not loaded
func RequireConfig() error {
	if cfgErr != nil {
		return cfgErr
	}
	if cfg == nil {
		return fmt.Errorf("airgapper not initialized - run 'airgapper init' first")
	}
	return nil
}

// RequireOwner returns an error if not the owner role
func RequireOwner() error {
	if err := RequireConfig(); err != nil {
		return err
	}
	if !cfg.IsOwner() {
		return fmt.Errorf("only the data owner can run this command (you are: %s)", cfg.Role)
	}
	return nil
}

// RequireHost returns an error if not the host role
func RequireHost() error {
	if err := RequireConfig(); err != nil {
		return err
	}
	if !cfg.IsHost() {
		return fmt.Errorf("only the backup host can run this command (you are: %s)", cfg.Role)
	}
	return nil
}
