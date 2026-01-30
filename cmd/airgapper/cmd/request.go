package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/consent"
	"github.com/spf13/cobra"
)

var (
	requestSnapshot string
	requestReason   string
	requestPeer     string
)

var requestCmd = &cobra.Command{
	Use:   "request",
	Short: "Request restore approval from peer",
	Long: `Request restore approval from your backup hosts.

Before restoring data, you need approval from enough peers to meet the threshold.
For a 2-of-3 setup, you need 1 other peer to approve (you have 1 share already).

Examples:
  airgapper request --reason "laptop died" --snapshot latest
  airgapper request --reason "accidental deletion" --peer http://bob:8080`,
	RunE: runRequest,
}

func init() {
	rootCmd.AddCommand(requestCmd)

	requestCmd.Flags().StringVarP(&requestSnapshot, "snapshot", "s", "latest", "Snapshot ID to restore")
	requestCmd.Flags().StringVarP(&requestReason, "reason", "m", "", "Reason for restore request (required)")
	requestCmd.Flags().StringVarP(&requestPeer, "peer", "p", "", "Peer address to notify")

	_ = requestCmd.MarkFlagRequired("reason")
}

func runRequest(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsOwner() {
		return fmt.Errorf("only the data owner can request restores")
	}

	mgr := consent.NewManager(cfg.ConfigDir)
	req, err := mgr.CreateRequest(cfg.Name, requestSnapshot, requestReason, nil)
	if err != nil {
		return err
	}

	fmt.Println("📤 Restore Request Created")
	fmt.Println("==========================")
	fmt.Printf("Request ID: %s\n", req.ID)
	fmt.Printf("Snapshot:   %s\n", req.SnapshotID)
	fmt.Printf("Reason:     %s\n", req.Reason)
	fmt.Printf("Expires:    %s\n", req.ExpiresAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Threshold:  %d shares needed\n", cfg.Threshold)

	// If peer address provided, try to notify them
	if requestPeer != "" || (cfg.Peer != nil && cfg.Peer.Address != "") {
		addr := requestPeer
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
	fmt.Println("Share request ID with your peers: " + req.ID)
	fmt.Println("\nOnce approved, run:")
	fmt.Printf("  airgapper restore --request %s --target /restore/path\n", req.ID)

	return nil
}
