// Airgapper - Consensus-based encrypted backup system
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/api"
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
	"github.com/lcrostarosa/airgapper/backend/internal/sss"
)

const version = "0.3.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "join":
		err = cmdJoin(args)
	case "backup":
		err = cmdBackup(args)
	case "snapshots":
		err = cmdSnapshots(args)
	case "request":
		err = cmdRequest(args)
	case "pending":
		err = cmdPending(args)
	case "approve":
		err = cmdApprove(args)
	case "deny":
		err = cmdDeny(args)
	case "restore":
		err = cmdRestore(args)
	case "status":
		err = cmdStatus(args)
	case "schedule":
		err = cmdSchedule(args)
	case "serve":
		err = cmdServe(args)
	case "version":
		fmt.Printf("airgapper %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`Airgapper - Consensus-based encrypted backup

USAGE:
  airgapper <command> [options]

COMMANDS:
  init        Initialize as data owner (creates repo, splits key)
  join        Join as backup host (receive key share from owner)
  backup      Create a backup (owner only)
  snapshots   List snapshots (requires password)
  request     Request restore approval from peer
  pending     List pending restore requests
  approve     Approve a restore request (releases key share)
  deny        Deny a restore request
  restore     Restore from a snapshot (requires approval)
  status      Show status
  schedule    Configure backup schedule
  serve       Run HTTP API server (with optional scheduled backups)
  version     Show version
  help        Show this help

WORKFLOW (Owner - Alice):
  1. airgapper init --name alice --repo rest:http://bob-nas:8000/alice-backup
  2. Give the displayed share to your peer
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
     airgapper approve <request-id>

SCHEDULE EXAMPLES:
  airgapper schedule --set "daily" ~/Documents    # Daily at 2 AM
  airgapper schedule --set "hourly" ~/Documents   # Every hour
  airgapper schedule --set "every 4h" ~/Documents # Every 4 hours
  airgapper schedule --set "0 3 * * *" ~/Documents # Cron: 3 AM daily
  airgapper schedule --show                        # Show current schedule
  airgapper schedule --clear                       # Remove schedule

EXAMPLES:
  # Initialize with local repo (testing)
  airgapper init --name alice --repo /tmp/backup-repo

  # Initialize with remote REST server
  airgapper init --name alice --repo rest:http://192.168.1.50:8000/mybackup

  # Join as host
  airgapper join --name bob --repo rest:http://192.168.1.50:8000/mybackup \
    --share abc123... --index 2

  # Backup multiple paths
  airgapper backup ~/Documents ~/Pictures

  # Run server with scheduled backups
  airgapper serve --schedule "daily" --paths ~/Documents,~/Pictures
`)
}

func cmdInit(args []string) error {
	// Check restic is installed
	if !restic.IsInstalled() {
		return fmt.Errorf("restic is not installed - please install it first: https://restic.net")
	}

	// Parse args
	var name, repoURL string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--repo", "-r":
			if i+1 < len(args) {
				repoURL = args[i+1]
				i++
			}
		}
	}

	if name == "" {
		return fmt.Errorf("--name is required")
	}
	if repoURL == "" {
		return fmt.Errorf("--repo is required")
	}

	// Check if already initialized
	if config.Exists("") {
		return fmt.Errorf("already initialized. Remove ~/.airgapper to reinitialize")
	}

	fmt.Println("🔐 Airgapper Initialization (Data Owner)")
	fmt.Println("=========================================")
	fmt.Printf("Name: %s\n", name)
	fmt.Printf("Repo: %s\n\n", repoURL)

	// Generate random repo password (64 hex chars = 32 bytes entropy)
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}
	password := hex.EncodeToString(passwordBytes)

	fmt.Println("1. Generated secure repository password")

	// Split password using 2-of-2 Shamir's Secret Sharing
	shares, err := sss.Split([]byte(password), 2, 2)
	if err != nil {
		return fmt.Errorf("failed to split password: %w", err)
	}

	fmt.Println("2. Split password into 2 shares (2-of-2 required for restore)")

	// Initialize restic repo
	fmt.Println("3. Initializing restic repository...")
	client := restic.NewClient(repoURL, password)
	if err := client.Init(); err != nil {
		return fmt.Errorf("failed to init repo: %w", err)
	}

	fmt.Println("4. Repository initialized successfully")

	// Save config with our share AND the full password (for backup)
	cfg := &config.Config{
		Name:       name,
		Role:       config.RoleOwner,
		RepoURL:    repoURL,
		Password:   password, // Owner keeps full password for backups
		LocalShare: shares[0].Data,
		ShareIndex: shares[0].Index,
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("5. Configuration saved to ~/.airgapper/")

	// Output the peer's share (they need this)
	peerShare := hex.EncodeToString(shares[1].Data)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("⚠️  IMPORTANT: Share this with your backup host (Bob):")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Printf("  Share:   %s\n", peerShare)
	fmt.Printf("  Index:   %d\n", shares[1].Index)
	fmt.Printf("  Repo:    %s\n", repoURL)
	fmt.Println()
	fmt.Println("They should run:")
	fmt.Printf("  airgapper join --name <their-name> --repo '%s' \\\n", repoURL)
	fmt.Printf("    --share %s --index %d\n", peerShare, shares[1].Index)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))

	fmt.Println("\n✅ Initialization complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Give the share above to your backup host")
	fmt.Println("  2. Configure backup schedule: airgapper schedule --set daily ~/Documents")
	fmt.Println("  3. Run: airgapper backup <paths>  (or start server for scheduled backups)")

	return nil
}

func cmdJoin(args []string) error {
	// Parse args
	var name, repoURL, shareHex string
	var shareIndex int
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--repo", "-r":
			if i+1 < len(args) {
				repoURL = args[i+1]
				i++
			}
		case "--share", "-s":
			if i+1 < len(args) {
				shareHex = args[i+1]
				i++
			}
		case "--index", "-i":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &shareIndex)
				i++
			}
		}
	}

	if name == "" {
		return fmt.Errorf("--name is required")
	}
	if repoURL == "" {
		return fmt.Errorf("--repo is required")
	}
	if shareHex == "" {
		return fmt.Errorf("--share is required (hex-encoded share from owner)")
	}
	if shareIndex == 0 {
		return fmt.Errorf("--index is required (share index, usually 2)")
	}

	// Check if already initialized
	if config.Exists("") {
		return fmt.Errorf("already initialized. Remove ~/.airgapper to reinitialize")
	}

	// Decode share
	share, err := hex.DecodeString(shareHex)
	if err != nil {
		return fmt.Errorf("invalid share (must be hex): %w", err)
	}

	fmt.Println("🔐 Airgapper Join (Backup Host)")
	fmt.Println("================================")
	fmt.Printf("Name:  %s\n", name)
	fmt.Printf("Repo:  %s\n", repoURL)
	fmt.Printf("Share: %d bytes, index %d\n\n", len(share), shareIndex)

	// Save config
	cfg := &config.Config{
		Name:       name,
		Role:       config.RoleHost,
		RepoURL:    repoURL,
		LocalShare: share,
		ShareIndex: byte(shareIndex),
		// Note: Host does NOT have the password, only the share
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✅ Joined as backup host!")
	fmt.Println()
	fmt.Println("You are now a key holder for this backup repository.")
	fmt.Println("When the owner requests a restore, you'll need to approve it.")
	fmt.Println()
	fmt.Println("Commands available to you:")
	fmt.Println("  airgapper pending  - List pending restore requests")
	fmt.Println("  airgapper approve  - Approve a restore request")
	fmt.Println("  airgapper deny     - Deny a restore request")
	fmt.Println("  airgapper serve    - Run HTTP API for remote management")

	return nil
}

func cmdBackup(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: airgapper backup <path> [paths...]")
	}

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

func cmdSnapshots(args []string) error {
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

func cmdRequest(args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsOwner() {
		return fmt.Errorf("only the data owner can request restores")
	}

	var snapshotID, reason, peerAddr string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--snapshot", "-s":
			if i+1 < len(args) {
				snapshotID = args[i+1]
				i++
			}
		case "--reason", "-m":
			if i+1 < len(args) {
				reason = args[i+1]
				i++
			}
		case "--peer", "-p":
			if i+1 < len(args) {
				peerAddr = args[i+1]
				i++
			}
		}
	}

	if snapshotID == "" {
		snapshotID = "latest"
	}
	if reason == "" {
		return fmt.Errorf("--reason is required (explain why you need to restore)")
	}

	mgr := consent.NewManager(cfg.ConfigDir)
	req, err := mgr.CreateRequest(cfg.Name, snapshotID, reason, nil)
	if err != nil {
		return err
	}

	fmt.Println("📤 Restore Request Created")
	fmt.Println("==========================")
	fmt.Printf("Request ID: %s\n", req.ID)
	fmt.Printf("Snapshot:   %s\n", req.SnapshotID)
	fmt.Printf("Reason:     %s\n", req.Reason)
	fmt.Printf("Expires:    %s\n", req.ExpiresAt.Format("2006-01-02 15:04:05"))

	// If peer address provided, try to notify them
	if peerAddr != "" || (cfg.Peer != nil && cfg.Peer.Address != "") {
		addr := peerAddr
		if addr == "" {
			addr = cfg.Peer.Address
		}
		fmt.Printf("\nNotifying peer at %s...\n", addr)

		// Send request to peer's API
		reqBody := map[string]interface{}{
			"id":          req.ID,
			"requester":   req.Requester,
			"snapshot_id": req.SnapshotID,
			"reason":      req.Reason,
		}
		jsonBody, _ := json.Marshal(reqBody)

		resp, err := http.Post(addr+"/api/requests", "application/json", bytes.NewReader(jsonBody))
		if err != nil {
			fmt.Printf("⚠️  Could not notify peer: %v\n", err)
			fmt.Println("   Share the request ID manually.")
		} else {
			resp.Body.Close()
			if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
				fmt.Println("✅ Peer notified!")
			} else {
				fmt.Printf("⚠️  Peer returned status %d\n", resp.StatusCode)
			}
		}
	}

	fmt.Println("\n⏳ Waiting for peer approval...")
	fmt.Println("Share request ID with your peer: " + req.ID)
	fmt.Println("\nOnce approved, run:")
	fmt.Printf("  airgapper restore --request %s --target /restore/path\n", req.ID)

	return nil
}

func cmdPending(args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	mgr := consent.NewManager(cfg.ConfigDir)
	requests, err := mgr.ListPending()
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		fmt.Println("No pending restore requests.")
		return nil
	}

	fmt.Println("📋 Pending Restore Requests")
	fmt.Println("===========================")
	for _, req := range requests {
		fmt.Printf("\nID: %s\n", req.ID)
		fmt.Printf("  From:     %s\n", req.Requester)
		fmt.Printf("  Snapshot: %s\n", req.SnapshotID)
		fmt.Printf("  Reason:   %s\n", req.Reason)
		fmt.Printf("  Expires:  %s\n", req.ExpiresAt.Format("2006-01-02 15:04"))
	}

	fmt.Println("\nTo approve: airgapper approve <request-id>")
	fmt.Println("To deny:    airgapper deny <request-id>")

	return nil
}

func cmdApprove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: airgapper approve <request-id>")
	}

	requestID := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	mgr := consent.NewManager(cfg.ConfigDir)

	// Load our share
	share, shareIndex, err := cfg.LoadShare()
	if err != nil {
		return fmt.Errorf("failed to load share: %w", err)
	}

	fmt.Printf("Approving request %s...\n", requestID)
	fmt.Printf("Releasing key share (index %d)...\n", shareIndex)

	// Approve and attach our share
	if err := mgr.Approve(requestID, cfg.Name, share); err != nil {
		return err
	}

	fmt.Println("\n✅ Request approved!")
	fmt.Println("Your key share has been released.")
	fmt.Println("The requester can now restore their data.")

	return nil
}

func cmdDeny(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: airgapper deny <request-id>")
	}

	requestID := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	mgr := consent.NewManager(cfg.ConfigDir)

	if err := mgr.Deny(requestID, cfg.Name); err != nil {
		return err
	}

	fmt.Println("❌ Request denied.")

	return nil
}

func cmdRestore(args []string) error {
	var requestID, target string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request", "-r":
			if i+1 < len(args) {
				requestID = args[i+1]
				i++
			}
		case "--target", "-t":
			if i+1 < len(args) {
				target = args[i+1]
				i++
			}
		}
	}

	if requestID == "" {
		return fmt.Errorf("--request is required")
	}
	if target == "" {
		return fmt.Errorf("--target is required")
	}

	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsOwner() {
		return fmt.Errorf("only the data owner can restore data")
	}

	mgr := consent.NewManager(cfg.ConfigDir)
	req, err := mgr.GetRequest(requestID)
	if err != nil {
		return err
	}

	if req.Status != consent.StatusApproved {
		return fmt.Errorf("request is not approved (status: %s)", req.Status)
	}

	if req.ShareData == nil {
		return fmt.Errorf("approved request missing share data - peer approval may have failed")
	}

	// Reconstruct password from both shares
	localShare, localIndex, err := cfg.LoadShare()
	if err != nil {
		return err
	}

	// Determine peer's index (if we're 1, peer is 2; if we're 2, peer is 1)
	peerIndex := byte(1)
	if localIndex == 1 {
		peerIndex = 2
	}

	shares := []sss.Share{
		{Index: localIndex, Data: localShare},
		{Index: peerIndex, Data: req.ShareData},
	}

	fmt.Println("🔓 Reconstructing password from key shares...")
	password, err := sss.Combine(shares)
	if err != nil {
		return fmt.Errorf("failed to reconstruct password: %w", err)
	}

	fmt.Println("✅ Password reconstructed successfully")
	fmt.Println()
	fmt.Println("📥 Starting restore...")
	fmt.Printf("Snapshot: %s\n", req.SnapshotID)
	fmt.Printf("Target:   %s\n\n", target)

	client := restic.NewClient(cfg.RepoURL, string(password))
	if err := client.Restore(req.SnapshotID, target); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	fmt.Printf("\n✅ Restore complete! Files restored to: %s\n", target)

	return nil
}

func cmdStatus(args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		// Not initialized
		fmt.Println("Airgapper Status: Not initialized")
		fmt.Println()
		fmt.Println("To get started:")
		fmt.Println("  As data owner:  airgapper init --name <name> --repo <url>")
		fmt.Println("  As backup host: airgapper join --name <name> --repo <url> --share <hex> --index <n>")
		return nil
	}

	fmt.Println("📊 Airgapper Status")
	fmt.Println("===================")
	fmt.Printf("Name:       %s\n", cfg.Name)
	fmt.Printf("Role:       %s\n", cfg.Role)
	fmt.Printf("Repository: %s\n", cfg.RepoURL)

	if cfg.LocalShare != nil {
		fmt.Printf("Key Share:  Index %d (%d bytes)\n", cfg.ShareIndex, len(cfg.LocalShare))
	} else {
		fmt.Println("Key Share:  Not configured")
	}

	if cfg.IsOwner() {
		if cfg.Password != "" {
			fmt.Println("Password:   ✅ Stored (can backup)")
		} else {
			fmt.Println("Password:   ❌ Missing")
		}
	}

	if cfg.Peer != nil {
		fmt.Printf("Peer:       %s", cfg.Peer.Name)
		if cfg.Peer.Address != "" {
			fmt.Printf(" (%s)", cfg.Peer.Address)
		}
		fmt.Println()
	} else {
		fmt.Println("Peer:       Not configured")
	}

	// Show schedule if configured
	if cfg.BackupSchedule != "" {
		fmt.Printf("Schedule:   %s\n", cfg.BackupSchedule)
		if len(cfg.BackupPaths) > 0 {
			fmt.Printf("Paths:      %s\n", strings.Join(cfg.BackupPaths, ", "))
		}
	} else {
		fmt.Println("Schedule:   Not configured")
	}

	// Check restic
	if restic.IsInstalled() {
		ver, _ := restic.Version()
		fmt.Printf("Restic:     %s\n", ver)
	} else {
		fmt.Println("Restic:     ❌ Not installed")
	}

	// Check pending requests
	mgr := consent.NewManager(cfg.ConfigDir)
	pending, _ := mgr.ListPending()
	fmt.Printf("Pending:    %d restore request(s)\n", len(pending))

	return nil
}

func cmdSchedule(args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsOwner() {
		return fmt.Errorf("only the data owner can configure backup schedule")
	}

	var showSchedule, clearSchedule bool
	var setSchedule string
	var paths []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--show":
			showSchedule = true
		case "--clear":
			clearSchedule = true
		case "--set":
			if i+1 < len(args) {
				setSchedule = args[i+1]
				i++
			}
		default:
			// Assume it's a path
			if !strings.HasPrefix(args[i], "-") {
				paths = append(paths, args[i])
			}
		}
	}

	// Default to showing schedule
	if !clearSchedule && setSchedule == "" {
		showSchedule = true
	}

	if showSchedule {
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

	if clearSchedule {
		cfg.BackupSchedule = ""
		cfg.BackupPaths = nil
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Println("✅ Schedule cleared.")
		return nil
	}

	if setSchedule != "" {
		// Validate schedule
		sched, err := scheduler.ParseSchedule(setSchedule)
		if err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}

		cfg.BackupSchedule = setSchedule
		if len(paths) > 0 {
			cfg.BackupPaths = paths
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

func cmdServe(args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	// Parse args
	addr := ":8080"
	var scheduleOverride, pathsOverride string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr", "-a":
			if i+1 < len(args) {
				addr = args[i+1]
				i++
			}
		case "--schedule", "-s":
			if i+1 < len(args) {
				scheduleOverride = args[i+1]
				i++
			}
		case "--paths", "-p":
			if i+1 < len(args) {
				pathsOverride = args[i+1]
				i++
			}
		}
	}

	// Update config with listen address
	cfg.ListenAddr = addr

	fmt.Println("🌐 Airgapper Server")
	fmt.Println("===================")
	fmt.Printf("Name: %s\n", cfg.Name)
	fmt.Printf("Role: %s\n", cfg.Role)
	fmt.Printf("API:  http://localhost%s\n\n", addr)

	fmt.Println("Endpoints:")
	fmt.Println("  GET  /health               - Health check")
	fmt.Println("  GET  /api/status           - System status")
	fmt.Println("  GET  /api/requests         - List pending requests")
	fmt.Println("  POST /api/requests         - Create restore request")
	fmt.Println("  GET  /api/requests/{id}    - Get request details")
	fmt.Println("  POST /api/requests/{id}/approve - Approve request")
	fmt.Println("  POST /api/requests/{id}/deny    - Deny request")
	fmt.Println("  GET  /api/schedule         - Get schedule info")
	fmt.Println("  POST /api/schedule         - Update schedule")
	fmt.Println()

	server := api.NewServer(cfg, addr)

	// Set up scheduler if owner and schedule is configured
	var sched *scheduler.Scheduler
	if cfg.IsOwner() {
		scheduleExpr := cfg.BackupSchedule
		backupPaths := cfg.BackupPaths

		// Override from command line
		if scheduleOverride != "" {
			scheduleExpr = scheduleOverride
		}
		if pathsOverride != "" {
			backupPaths = strings.Split(pathsOverride, ",")
		}

		if scheduleExpr != "" && len(backupPaths) > 0 {
			parsedSched, err := scheduler.ParseSchedule(scheduleExpr)
			if err != nil {
				return fmt.Errorf("invalid schedule: %w", err)
			}

			// Create backup function
			backupFunc := func() error {
				client := restic.NewClient(cfg.RepoURL, cfg.Password)
				return client.Backup(backupPaths, []string{"airgapper", "scheduled"})
			}

			sched = scheduler.NewScheduler(parsedSched, backupFunc)
			server.SetScheduler(sched)

			fmt.Println("📅 Scheduled Backups:")
			fmt.Printf("  Schedule: %s\n", scheduleExpr)
			fmt.Printf("  Paths:    %s\n", strings.Join(backupPaths, ", "))
			nextRun := parsedSched.NextRun(time.Now())
			fmt.Printf("  Next:     %s\n", nextRun.Format("2006-01-02 15:04:05"))
			fmt.Println()

			sched.Start()
		} else if cfg.IsOwner() {
			if scheduleExpr == "" {
				fmt.Println("📅 No backup schedule configured.")
				fmt.Println("   Configure with: airgapper schedule --set daily ~/Documents")
			} else if len(backupPaths) == 0 {
				fmt.Println("📅 Schedule configured but no paths specified.")
				fmt.Println("   Add paths with: airgapper schedule --set daily ~/Documents")
			}
			fmt.Println()
		}
	}

	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Handle shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-stop
	fmt.Println("\nShutting down...")

	// Stop scheduler
	if sched != nil {
		sched.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	fmt.Println("Server stopped.")
	return nil
}
