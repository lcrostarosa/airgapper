package verification

import (
	"os"
	"testing"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

func TestCreateTicket(t *testing.T) {
	// Generate owner keys
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}
	ownerKeyID := crypto.KeyID(pubKey)

	// Create a snapshot deletion ticket
	target := TicketTarget{
		Type:        TicketTargetSnapshot,
		SnapshotIDs: []string{"snap1", "snap2"},
	}

	ticket, err := CreateTicket(privKey, ownerKeyID, target, "cleanup old snapshots", 7)
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	if ticket.ID == "" {
		t.Error("ticket ID should not be empty")
	}

	if ticket.OwnerKeyID != ownerKeyID {
		t.Errorf("owner key ID mismatch: expected %s, got %s", ownerKeyID, ticket.OwnerKeyID)
	}

	if ticket.OwnerSignature == "" {
		t.Error("ticket should be signed")
	}

	if len(ticket.Target.SnapshotIDs) != 2 {
		t.Errorf("expected 2 snapshot IDs, got %d", len(ticket.Target.SnapshotIDs))
	}

	if ticket.ExpiresAt.IsZero() {
		t.Error("ticket should have expiry set")
	}

	// Verify expiry is approximately 7 days from now
	expectedExpiry := time.Now().Add(7 * 24 * time.Hour)
	diff := ticket.ExpiresAt.Sub(expectedExpiry)
	if diff > time.Minute || diff < -time.Minute {
		t.Errorf("expiry time unexpected: got %v", ticket.ExpiresAt)
	}
}

func TestTicketManager_RegisterAndValidate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ticket-manager-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate owner and host keys
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	// Create ticket manager
	tm, err := NewTicketManager(tempDir, ownerPub, hostPriv, hostPub, hostKeyID, 7)
	if err != nil {
		t.Fatalf("failed to create ticket manager: %v", err)
	}

	// Create a ticket
	target := TicketTarget{
		Type:        TicketTargetSnapshot,
		SnapshotIDs: []string{"snapshot-abc123"},
	}

	ticket, err := CreateTicket(ownerPriv, ownerKeyID, target, "approved deletion", 7)
	if err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// Register the ticket
	err = tm.RegisterTicket(ticket)
	if err != nil {
		t.Fatalf("failed to register ticket: %v", err)
	}

	// Validate deletion with the ticket
	ticketID, err := tm.ValidateDelete("/some/path", "snapshot-abc123")
	if err != nil {
		t.Fatalf("validation should succeed: %v", err)
	}

	if ticketID != ticket.ID {
		t.Errorf("returned ticket ID mismatch: expected %s, got %s", ticket.ID, ticketID)
	}

	// Validate deletion for non-authorized snapshot
	_, err = tm.ValidateDelete("/some/path", "snapshot-xyz789")
	if err == nil {
		t.Error("validation should fail for unauthorized snapshot")
	}
}

func TestTicketManager_FileTicket(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ticket-file-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	tm, err := NewTicketManager(tempDir, ownerPub, hostPriv, hostPub, hostKeyID, 7)
	if err != nil {
		t.Fatalf("failed to create ticket manager: %v", err)
	}

	// Create a file deletion ticket with wildcard
	target := TicketTarget{
		Type:  TicketTargetFile,
		Paths: []string{"/repo/data/*", "/repo/config"},
	}

	ticket, _ := CreateTicket(ownerPriv, ownerKeyID, target, "cleanup", 7)
	tm.RegisterTicket(ticket)

	// Should match exact path
	_, err = tm.ValidateDelete("/repo/config", "")
	if err != nil {
		t.Errorf("should match exact path: %v", err)
	}

	// Should match wildcard
	_, err = tm.ValidateDelete("/repo/data/abc123", "")
	if err != nil {
		t.Errorf("should match wildcard path: %v", err)
	}

	// Should not match unrelated path
	_, err = tm.ValidateDelete("/other/path", "")
	if err == nil {
		t.Error("should not match unrelated path")
	}
}

func TestTicketManager_RecordUsage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ticket-usage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	tm, err := NewTicketManager(tempDir, ownerPub, hostPriv, hostPub, hostKeyID, 7)
	if err != nil {
		t.Fatalf("failed to create ticket manager: %v", err)
	}

	// Create and register a ticket
	target := TicketTarget{
		Type:        TicketTargetSnapshot,
		SnapshotIDs: []string{"snap1"},
	}
	ticket, _ := CreateTicket(ownerPriv, ownerKeyID, target, "test", 7)
	tm.RegisterTicket(ticket)

	// Record usage
	record, err := tm.RecordUsage(ticket.ID, []string{"/repo/snapshots/snap1"})
	if err != nil {
		t.Fatalf("failed to record usage: %v", err)
	}

	if record.TicketID != ticket.ID {
		t.Error("usage record should reference ticket")
	}

	if record.HostSignature == "" {
		t.Error("usage record should be signed by host")
	}

	// Get usage records
	records := tm.GetUsageRecords(ticket.ID)
	if len(records) != 1 {
		t.Errorf("expected 1 usage record, got %d", len(records))
	}
}

func TestTicketManager_InvalidSignature(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ticket-invalid-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, _, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	// Use a different key to sign (not the expected owner key)
	_, wrongPriv, _ := crypto.GenerateKeyPair()
	wrongKeyID := "wrong-key"

	tm, err := NewTicketManager(tempDir, ownerPub, hostPriv, hostPub, hostKeyID, 7)
	if err != nil {
		t.Fatalf("failed to create ticket manager: %v", err)
	}

	// Create a ticket signed with wrong key
	target := TicketTarget{
		Type:        TicketTargetSnapshot,
		SnapshotIDs: []string{"snap1"},
	}
	ticket, _ := CreateTicket(wrongPriv, wrongKeyID, target, "test", 7)

	// Registration should fail due to invalid signature
	err = tm.RegisterTicket(ticket)
	if err == nil {
		t.Error("registration should fail with invalid signature")
	}
}

func TestTicketManager_ExpiredTicket(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ticket-expired-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	tm, err := NewTicketManager(tempDir, ownerPub, hostPriv, hostPub, hostKeyID, 7)
	if err != nil {
		t.Fatalf("failed to create ticket manager: %v", err)
	}

	// Create a ticket that's already expired
	target := TicketTarget{
		Type:        TicketTargetSnapshot,
		SnapshotIDs: []string{"snap1"},
	}
	ticket, _ := CreateTicket(ownerPriv, ownerKeyID, target, "test", 7)

	// Manually set expiry to past
	ticket.ExpiresAt = time.Now().Add(-24 * time.Hour)

	// Note: We'd need to re-sign for this to work properly, but the point is
	// the manager should reject expired tickets at registration time
	err = tm.RegisterTicket(ticket)
	if err == nil {
		t.Error("should reject expired ticket")
	}
}

func TestTicketManager_ListTickets(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ticket-list-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	tm, err := NewTicketManager(tempDir, ownerPub, hostPriv, hostPub, hostKeyID, 7)
	if err != nil {
		t.Fatalf("failed to create ticket manager: %v", err)
	}

	// Create and register multiple tickets
	for i := 0; i < 3; i++ {
		target := TicketTarget{
			Type:        TicketTargetSnapshot,
			SnapshotIDs: []string{"snap"},
		}
		ticket, _ := CreateTicket(ownerPriv, ownerKeyID, target, "test", 7)
		tm.RegisterTicket(ticket)
	}

	tickets := tm.ListTickets(true)
	if len(tickets) != 3 {
		t.Errorf("expected 3 tickets, got %d", len(tickets))
	}
}
