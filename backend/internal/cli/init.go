package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/emergency"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
	"github.com/lcrostarosa/airgapper/backend/internal/sss"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize as data owner (creates repo, splits key)",
	Long: `Initialize a new Airgapper vault as the data owner.

This creates a restic repository and splits the encryption password
using Shamir's Secret Sharing. You keep one share, and give the
other to your backup host.`,
	Example: `  # Standard 2-of-2 initialization
  airgapper init --name alice --repo rest:http://bob-nas:8000/backup

  # With recovery shares (2-of-4 scheme)
  airgapper init --name alice --repo rest:http://bob-nas:8000/backup \
    --recovery-shares 4 --recovery-threshold 2 \
    --custodian "Lawyer" --custodian "Family"

  # With consensus mode (m-of-n key holders)
  airgapper init --name alice --repo rest:http://bob-nas:8000/backup \
    --threshold 2 --holders 3`,
	RunE: runInit,
}

func init() {
	f := initCmd.Flags()

	// Required
	f.StringP("name", "n", "", "Your name/identifier")
	f.StringP("repo", "r", "", "Restic repository URL")
	initCmd.MarkFlagRequired("name")
	initCmd.MarkFlagRequired("repo")

	// SSS mode options
	f.Int("recovery-shares", 2, "Total shares to create")
	f.Int("recovery-threshold", 2, "Shares needed to restore")
	f.StringSlice("custodian", nil, "Custodian name (can specify multiple)")

	// Consensus mode options
	f.Int("threshold", 0, "Approval threshold (enables consensus mode)")
	f.Int("holders", 0, "Total key holders")

	// Emergency options
	f.String("dead-man-switch", "", "Days of inactivity before trigger (e.g., 180d)")
	f.StringSlice("escalation-contact", nil, "Escalation contact (can specify multiple)")
	f.Bool("enable-overrides", false, "Enable emergency override keys")

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if !restic.IsInstalled() {
		return fmt.Errorf("restic is not installed - please install it first: https://restic.net")
	}

	name, _ := cmd.Flags().GetString("name")
	repoURL, _ := cmd.Flags().GetString("repo")

	if config.Exists("") {
		return fmt.Errorf("already initialized. Remove ~/.airgapper to reinitialize")
	}

	threshold, _ := cmd.Flags().GetInt("threshold")
	holders, _ := cmd.Flags().GetInt("holders")

	if threshold > 0 || holders > 0 {
		return initConsensus(cmd, name, repoURL)
	}

	return initSSS(cmd, name, repoURL)
}

func initSSS(cmd *cobra.Command, name, repoURL string) error {
	f := cmd.Flags()
	recoveryShares, _ := f.GetInt("recovery-shares")
	recoveryThreshold, _ := f.GetInt("recovery-threshold")
	custodians, _ := f.GetStringSlice("custodian")
	deadManSwitch, _ := f.GetString("dead-man-switch")
	enableOverrides, _ := f.GetBool("enable-overrides")
	escalationContacts, _ := f.GetStringSlice("escalation-contact")

	deadManDays := parseDays(deadManSwitch)

	if recoveryThreshold > recoveryShares {
		return fmt.Errorf("recovery threshold (%d) cannot exceed total shares (%d)", recoveryThreshold, recoveryShares)
	}

	PrintHeader("Airgapper Initialization (Data Owner) - SSS Mode")
	PrintInfo("Name: %s", name)
	PrintInfo("Repo: %s", repoURL)
	if recoveryShares > 2 {
		PrintInfo("Recovery: %d-of-%d shares", recoveryThreshold, recoveryShares)
	}
	fmt.Println()

	// Generate password
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}
	password := hex.EncodeToString(passwordBytes)
	PrintInfo("1. Generated secure repository password")

	// Split using SSS
	shares, err := sss.Split([]byte(password), recoveryThreshold, recoveryShares)
	if err != nil {
		return fmt.Errorf("failed to split password: %w", err)
	}
	PrintInfo("2. Split password into %d shares (%d-of-%d required)", recoveryShares, recoveryThreshold, recoveryShares)

	// Initialize restic repo
	PrintInfo("3. Initializing restic repository...")
	client := restic.NewClient(repoURL, password)
	if err := client.Init(); err != nil {
		return fmt.Errorf("failed to init repo: %w", err)
	}
	PrintInfo("4. Repository initialized successfully")

	// Build config
	newCfg := &config.Config{
		Name:       name,
		Role:       config.RoleOwner,
		RepoURL:    repoURL,
		Password:   password,
		LocalShare: shares[0].Data,
		ShareIndex: shares[0].Index,
	}

	// Configure emergency features
	if recoveryShares > 2 || deadManDays > 0 || enableOverrides {
		newCfg.Emergency = emergency.NewConfig()

		if recoveryShares > 2 {
			newCfg.Emergency.WithRecovery(recoveryThreshold, recoveryShares, custodians)
		}

		if deadManDays > 0 {
			newCfg.Emergency.WithDeadManSwitch(deadManDays, escalationContacts)
		}

		if enableOverrides {
			newCfg.Emergency.WithOverrides()
		}
	}

	if err := newCfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	PrintInfo("5. Configuration saved to ~/.airgapper/")

	// Output shares
	printShareInfo(shares, repoURL, recoveryThreshold, recoveryShares, custodians)

	if newCfg.Emergency != nil {
		printEmergencyFeatures(newCfg.Emergency)
	}

	PrintSuccess("Initialization complete!")
	printInitNextSteps(recoveryShares > 2)

	return nil
}

func initConsensus(cmd *cobra.Command, name, repoURL string) error {
	f := cmd.Flags()
	threshold, _ := f.GetInt("threshold")
	holders, _ := f.GetInt("holders")

	if threshold < 1 {
		threshold = 1
	}
	if holders < threshold {
		holders = threshold
	}

	PrintHeader("Airgapper Initialization (Data Owner) - Consensus Mode")
	PrintInfo("Name:      %s", name)
	PrintInfo("Repo:      %s", repoURL)
	PrintInfo("Consensus: %d-of-%d", threshold, holders)
	fmt.Println()

	// Generate password
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}
	password := hex.EncodeToString(passwordBytes)
	PrintInfo("1. Generated secure repository password")

	// Generate Ed25519 key pair
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}
	keyID := crypto.KeyID(pubKey)
	PrintInfo("2. Generated Ed25519 key pair")
	PrintInfo("   Your Key ID: %s", keyID)

	// Initialize restic repo
	PrintInfo("3. Initializing restic repository...")
	client := restic.NewClient(repoURL, password)
	if err := client.Init(); err != nil {
		return fmt.Errorf("failed to init repo: %w", err)
	}
	PrintInfo("4. Repository initialized successfully")

	// Build config
	ownerHolder := config.KeyHolder{
		ID:        keyID,
		Name:      name,
		PublicKey: pubKey,
		JoinedAt:  time.Now(),
		IsOwner:   true,
	}

	newCfg := &config.Config{
		Name:       name,
		Role:       config.RoleOwner,
		RepoURL:    repoURL,
		Password:   password,
		PublicKey:  pubKey,
		PrivateKey: privKey,
		Consensus: &config.ConsensusConfig{
			Threshold:       threshold,
			TotalKeys:       holders,
			KeyHolders:      []config.KeyHolder{ownerHolder},
			RequireApproval: threshold > 1 || holders > 1,
		},
	}

	if err := newCfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	PrintInfo("5. Configuration saved to ~/.airgapper/")

	if holders > 1 {
		PrintDivider()
		PrintWarning("IMPORTANT: Invite other key holders to join")
		PrintInfo("You need %d more key holder(s) to reach %d-of-%d consensus.", holders-1, threshold, holders)
		fmt.Println()
		PrintInfo("They should run:")
		PrintInfo("  airgapper join --name <their-name> --repo '%s' --consensus", repoURL)
		PrintDivider()
	}

	PrintSuccess("Initialization complete!")
	return nil
}

func printShareInfo(shares []sss.Share, repoURL string, k, n int, custodians []string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	PrintWarning("IMPORTANT: Share this with your backup host (Bob):")
	PrintDivider()

	peerShare := hex.EncodeToString(shares[1].Data)
	PrintInfo("  Share:   %s", peerShare)
	PrintInfo("  Index:   %d", shares[1].Index)
	PrintInfo("  Repo:    %s", repoURL)
	fmt.Println()
	PrintInfo("They should run:")
	PrintInfo("  airgapper join --name <their-name> --repo '%s' \\", repoURL)
	PrintInfo("    --share %s --index %d", peerShare, shares[1].Index)

	if n > 2 {
		fmt.Println()
		PrintDivider()
		PrintInfo("RECOVERY CUSTODIAN SHARES:")
		PrintDivider()
		PrintInfo("These shares can be used to recover if you lose access to share 1.")
		PrintInfo("Any %d of %d shares can reconstruct the password.", k, n)
		fmt.Println()

		for i := 2; i < n; i++ {
			custName := fmt.Sprintf("Custodian %d", i-1)
			if len(custodians) > i-2 {
				custName = custodians[i-2]
			}
			custShare := hex.EncodeToString(shares[i].Data)
			PrintInfo("Share %d (%s):", shares[i].Index, custName)
			PrintInfo("  %s", custShare)
			fmt.Println()
		}
		PrintWarning("Store these shares securely! They can decrypt your backups!")
	}

	fmt.Println(strings.Repeat("=", 70))
}

func printEmergencyFeatures(e *emergency.Config) {
	fmt.Println()
	PrintInfo("Emergency Features Enabled:")
	if e.Recovery != nil && e.Recovery.Enabled {
		PrintInfo("  • Recovery shares: %d-of-%d", e.Recovery.Threshold, e.Recovery.TotalShares)
	}
	if e.DeadManSwitch != nil && e.DeadManSwitch.Enabled {
		PrintInfo("  • Dead man's switch: %d days", e.DeadManSwitch.InactivityDays)
	}
	if e.Override != nil && e.Override.Enabled {
		PrintInfo("  • Override key: run 'airgapper override setup' to generate")
	}
}

func printInitNextSteps(hasRecovery bool) {
	fmt.Println()
	PrintInfo("Next steps:")
	PrintInfo("  1. Give the host share above to your backup host")
	if hasRecovery {
		PrintInfo("  2. Securely distribute custodian shares to trusted parties")
	}
	PrintInfo("  3. Configure backup schedule: airgapper schedule --set daily ~/Documents")
	PrintInfo("  4. Run: airgapper backup <paths>  (or start server for scheduled backups)")
}

func parseDays(s string) int {
	if s == "" {
		return 0
	}
	s = strings.TrimSuffix(s, "d")
	var days int
	fmt.Sscanf(s, "%d", &days)
	return days
}
