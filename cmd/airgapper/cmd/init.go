package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/restic"
	"github.com/lcrostarosa/airgapper/internal/sss"
	"github.com/spf13/cobra"
)

var (
	initName      string
	initRepo      string
	initThreshold int
	initShares    int
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize as data owner (creates repo, splits key)",
	Long: `Initialize a new Airgapper backup as the data owner.

This command:
  1. Generates a secure random password for the backup repository
  2. Splits the password using Shamir's Secret Sharing (N-of-M threshold)
  3. Initializes a restic repository with the password
  4. Outputs shares to distribute to backup hosts

Examples:
  # Initialize with 2-of-2 sharing (both parties required)
  airgapper init --name alice --repo /tmp/backup-repo

  # Initialize with 2-of-3 sharing (any 2 of 3 parties can restore)
  airgapper init --name alice --repo rest:http://bob:8000/backup --threshold 2 --shares 3

  # Initialize with 3-of-5 sharing
  airgapper init --name alice --repo rest:http://server:8000/backup -t 3 -n 5`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&initName, "name", "n", "", "Your name/identifier (required)")
	initCmd.Flags().StringVarP(&initRepo, "repo", "r", "", "Restic repository URL (required)")
	initCmd.Flags().IntVarP(&initThreshold, "threshold", "t", 2, "Number of shares required to restore (k)")
	initCmd.Flags().IntVarP(&initShares, "shares", "s", 2, "Total number of shares to create (n)")

	_ = initCmd.MarkFlagRequired("name")
	_ = initCmd.MarkFlagRequired("repo")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Validate threshold and shares
	if initThreshold < 2 {
		return fmt.Errorf("threshold must be at least 2")
	}
	if initShares < initThreshold {
		return fmt.Errorf("shares must be >= threshold")
	}
	if initShares > 255 {
		return fmt.Errorf("shares must be <= 255")
	}

	// Check restic is installed
	if !restic.IsInstalled() {
		return fmt.Errorf("restic is not installed - please install it first: https://restic.net")
	}

	// Check if already initialized
	if config.Exists("") {
		return fmt.Errorf("already initialized. Remove ~/.airgapper to reinitialize")
	}

	fmt.Println("🔐 Airgapper Initialization (Data Owner)")
	fmt.Println("=========================================")
	fmt.Printf("Name:      %s\n", initName)
	fmt.Printf("Repo:      %s\n", initRepo)
	fmt.Printf("Threshold: %d-of-%d\n\n", initThreshold, initShares)

	// Generate random repo password (64 hex chars = 32 bytes entropy)
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}
	password := hex.EncodeToString(passwordBytes)

	fmt.Println("1. Generated secure repository password")

	// Split password using Shamir's Secret Sharing
	shares, err := sss.Split([]byte(password), initThreshold, initShares)
	if err != nil {
		return fmt.Errorf("failed to split password: %w", err)
	}

	fmt.Printf("2. Split password into %d shares (%d-of-%d required for restore)\n", initShares, initThreshold, initShares)

	// Initialize restic repo
	fmt.Println("3. Initializing restic repository...")
	client := restic.NewClient(initRepo, password)
	if err := client.Init(); err != nil {
		return fmt.Errorf("failed to init repo: %w", err)
	}

	fmt.Println("4. Repository initialized successfully")

	// Create peer shares configuration
	peerShares := make([]config.PeerShare, len(shares)-1)
	for i := 1; i < len(shares); i++ {
		peerShares[i-1] = config.PeerShare{
			Index: shares[i].Index,
			// Data is nil - we don't store peer shares locally
		}
	}

	// Save config with our share AND the full password (for backup)
	cfg := &config.Config{
		Name:       initName,
		Role:       config.RoleOwner,
		RepoURL:    initRepo,
		Password:   password, // Owner keeps full password for backups
		LocalShare: shares[0].Data,
		ShareIndex: shares[0].Index,
		Threshold:  initThreshold,
		TotalShares: initShares,
		PeerShares: peerShares,
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("5. Configuration saved to ~/.airgapper/")

	// Output the peer shares (they need these)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("⚠️  IMPORTANT: Distribute these shares to your backup hosts:")
	fmt.Println(strings.Repeat("=", 70))

	for i := 1; i < len(shares); i++ {
		peerShare := hex.EncodeToString(shares[i].Data)
		fmt.Println()
		fmt.Printf("  Share %d:\n", i)
		fmt.Printf("    Data:   %s\n", peerShare)
		fmt.Printf("    Index:  %d\n", shares[i].Index)
		fmt.Println()
		fmt.Printf("    Peer should run:\n")
		fmt.Printf("    airgapper join --name <their-name> --repo '%s' \\\n", initRepo)
		fmt.Printf("      --share %s --index %d\n", peerShare, shares[i].Index)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))

	fmt.Println("\n✅ Initialization complete!")
	fmt.Println("\nNext steps:")
	fmt.Printf("  1. Distribute the %d shares above to your backup hosts\n", initShares-1)
	fmt.Println("  2. Configure backup schedule: airgapper schedule --set daily ~/Documents")
	fmt.Println("  3. Run: airgapper backup <paths>  (or start server for scheduled backups)")

	return nil
}
