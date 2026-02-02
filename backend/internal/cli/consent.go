package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// --- Request Command ---

var requestCmd = &cobra.Command{
	Use:   "request",
	Short: "Request restore approval from peer(s)",
	Long:  `Create a new restore request that must be approved by your peer(s).`,
	Example: `  airgapper request --snapshot latest --reason "Need to recover deleted files"
  airgapper request --snapshot abc123 --reason "Testing restore" --peer http://bob:8081`,
	RunE: runRequest,
}

func init() {
	f := requestCmd.Flags()
	f.String("snapshot", "latest", "Snapshot ID to restore")
	f.String("reason", "", "Reason for restore (required)")
	f.String("peer", "", "Peer address to notify")
	requestCmd.MarkFlagRequired("reason")
	rootCmd.AddCommand(requestCmd)
}

func runRequest(cmd *cobra.Command, args []string) error {
	if err := RequireOwner(); err != nil {
		return err
	}

	snapshotID, _ := cmd.Flags().GetString("snapshot")
	reason, _ := cmd.Flags().GetString("reason")
	peerAddr, _ := cmd.Flags().GetString("peer")

	mgr := consent.NewManager(cfg.ConfigDir)
	req, err := mgr.CreateRequest(cfg.Name, snapshotID, reason, nil)
	if err != nil {
		return err
	}

	printHeader("Restore Request Created")
	printInfo("Request ID: %s", req.ID)
	printInfo("Snapshot:   %s", req.SnapshotID)
	printInfo("Reason:     %s", req.Reason)
	printInfo("Expires:    %s", req.ExpiresAt.Format("2006-01-02 15:04:05"))

	// Notify peer if address provided
	if peerAddr == "" && cfg.Peer != nil && cfg.Peer.Address != "" {
		peerAddr = cfg.Peer.Address
	}

	if peerAddr != "" {
		notifyPeer(peerAddr, req)
	}

	fmt.Println()
	printInfo("Waiting for peer approval...")
	printInfo("Share request ID with your peer: %s", req.ID)
	fmt.Println()
	printInfo("Once approved, run:")
	printInfo("  airgapper restore --request %s --target /restore/path", req.ID)

	return nil
}

func notifyPeer(peerAddr string, req *consent.RestoreRequest) {
	printInfo("\nNotifying peer at %s...", peerAddr)

	reqBody := map[string]interface{}{
		"id":          req.ID,
		"requester":   req.Requester,
		"snapshot_id": req.SnapshotID,
		"reason":      req.Reason,
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(peerAddr+"/api/requests", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		printWarning("Could not notify peer: %v", err)
		printInfo("   Share the request ID manually.")
		return
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		printSuccess("Peer notified!")
	} else {
		printWarning("Peer returned status %d", resp.StatusCode)
	}
}

// --- Pending Command ---

var pendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "List pending restore requests",
	Long:  `Show all restore requests waiting for approval.`,
	RunE:  runPending,
}

func init() {
	rootCmd.AddCommand(pendingCmd)
}

func runPending(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	mgr := consent.NewManager(cfg.ConfigDir)
	requests, err := mgr.ListPending()
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		printInfo("No pending restore requests.")
		return nil
	}

	printHeader("Pending Restore Requests")
	for _, req := range requests {
		fmt.Printf("\nID: %s\n", req.ID)
		fmt.Printf("  From:     %s\n", req.Requester)
		fmt.Printf("  Snapshot: %s\n", req.SnapshotID)
		fmt.Printf("  Reason:   %s\n", req.Reason)
		fmt.Printf("  Expires:  %s\n", req.ExpiresAt.Format("2006-01-02 15:04"))
	}

	fmt.Println()
	printInfo("To approve: airgapper approve <request-id>")
	printInfo("To deny:    airgapper deny <request-id>")

	return nil
}

// --- Approve Command ---

var approveCmd = &cobra.Command{
	Use:   "approve <request-id>",
	Short: "Approve a restore request (sign or release share)",
	Long:  `Approve a pending restore request by signing it or releasing your key share.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runApprove,
}

func init() {
	rootCmd.AddCommand(approveCmd)
}

func runApprove(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	requestID := args[0]
	mgr := consent.NewManager(cfg.ConfigDir)

	if cfg.UsesConsensusMode() || cfg.PrivateKey != nil {
		return approveConsensus(mgr, requestID)
	}

	return approveSSS(mgr, requestID)
}

func approveSSS(mgr *consent.Manager, requestID string) error {
	share, shareIndex, err := cfg.LoadShare()
	if err != nil {
		return fmt.Errorf("failed to load share: %w", err)
	}

	printInfo("Approving request %s...", requestID)
	printInfo("Releasing key share (index %d)...", shareIndex)

	if err := mgr.Approve(requestID, cfg.Name, share); err != nil {
		return err
	}

	printSuccess("Request approved!")
	printInfo("Your key share has been released.")
	printInfo("The requester can now restore their data.")

	return nil
}

func approveConsensus(mgr *consent.Manager, requestID string) error {
	if cfg.PrivateKey == nil {
		return fmt.Errorf("no private key found - cannot sign")
	}

	req, err := mgr.GetRequest(requestID)
	if err != nil {
		return err
	}

	keyID := crypto.KeyID(cfg.PublicKey)
	printInfo("Signing request %s...", requestID)
	printInfo("Your Key ID: %s", keyID)

	signature, err := crypto.SignRestoreRequest(
		cfg.PrivateKey,
		req.ID,
		req.Requester,
		req.SnapshotID,
		req.Reason,
		keyID,
		req.Paths,
		req.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	if err := mgr.AddSignature(requestID, keyID, cfg.Name, signature); err != nil {
		return err
	}

	current, required, _ := mgr.GetApprovalProgress(requestID)

	printSuccess("Request signed!")
	printInfo("Approvals: %d of %d required", current, required)

	if current >= required {
		printSuccess("Request is now fully approved!")
		printInfo("The requester can now restore their data.")
	} else {
		printInfo("Waiting for %d more approval(s)...", required-current)
	}

	return nil
}

// --- Deny Command ---

var denyCmd = &cobra.Command{
	Use:   "deny <request-id>",
	Short: "Deny a restore request",
	Long:  `Deny a pending restore request.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDeny,
}

func init() {
	rootCmd.AddCommand(denyCmd)
}

func runDeny(cmd *cobra.Command, args []string) error {
	if err := RequireConfig(); err != nil {
		return err
	}

	requestID := args[0]
	mgr := consent.NewManager(cfg.ConfigDir)

	if err := mgr.Deny(requestID, cfg.Name); err != nil {
		return err
	}

	printInfo("Request denied.")
	return nil
}
