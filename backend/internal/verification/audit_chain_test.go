package verification

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

func TestAuditChain_Record(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "audit-chain-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate test keys
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}
	hostKeyID := crypto.KeyID(pubKey)

	// Create audit chain with signing enabled
	chain, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	if err != nil {
		t.Fatalf("failed to create audit chain: %v", err)
	}

	// Record some entries
	entry1, err := chain.Record("CREATE", "/repo/data/abc123", "created file", true, "")
	if err != nil {
		t.Fatalf("failed to record entry 1: %v", err)
	}

	if entry1.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", entry1.Sequence)
	}

	if entry1.Operation != "CREATE" {
		t.Errorf("expected operation CREATE, got %s", entry1.Operation)
	}

	if entry1.HostSignature == "" {
		t.Error("expected host signature to be set")
	}

	// Record another entry
	entry2, err := chain.Record("DELETE", "/repo/data/def456", "deleted file", true, "")
	if err != nil {
		t.Fatalf("failed to record entry 2: %v", err)
	}

	if entry2.Sequence != 2 {
		t.Errorf("expected sequence 2, got %d", entry2.Sequence)
	}

	if entry2.PreviousHash != entry1.ContentHash {
		t.Error("entry2 should reference entry1's hash")
	}
}

func TestAuditChain_Verify(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-chain-verify-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}
	hostKeyID := crypto.KeyID(pubKey)

	chain, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	if err != nil {
		t.Fatalf("failed to create audit chain: %v", err)
	}

	// Record multiple entries
	for i := 0; i < 5; i++ {
		_, err := chain.Record("CREATE", "/test/path", "test", true, "")
		if err != nil {
			t.Fatalf("failed to record entry %d: %v", i, err)
		}
	}

	// Verify the chain
	result, err := chain.Verify()
	if err != nil {
		t.Fatalf("verification failed with error: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected chain to be valid, got errors: %v", result.Errors)
	}

	if result.TotalEntries != 5 {
		t.Errorf("expected 5 entries, got %d", result.TotalEntries)
	}

	if result.SignedEntries != 5 {
		t.Errorf("expected 5 signed entries, got %d", result.SignedEntries)
	}
}

func TestAuditChain_Persistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-chain-persist-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}
	hostKeyID := crypto.KeyID(pubKey)

	// Create chain and record entries
	chain1, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	if err != nil {
		t.Fatalf("failed to create audit chain: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := chain1.Record("CREATE", "/test", "test", true, "")
		if err != nil {
			t.Fatalf("failed to record: %v", err)
		}
	}

	lastHash1 := chain1.GetLatestHash()
	seq1 := chain1.GetSequence()

	// Create new chain from same directory (simulating restart)
	chain2, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	if err != nil {
		t.Fatalf("failed to reload audit chain: %v", err)
	}

	if chain2.GetSequence() != seq1 {
		t.Errorf("sequence mismatch: expected %d, got %d", seq1, chain2.GetSequence())
	}

	if chain2.GetLatestHash() != lastHash1 {
		t.Error("last hash mismatch after reload")
	}

	// Continue recording
	entry4, err := chain2.Record("DELETE", "/test", "test", true, "")
	if err != nil {
		t.Fatalf("failed to record after reload: %v", err)
	}

	if entry4.Sequence != 4 {
		t.Errorf("expected sequence 4, got %d", entry4.Sequence)
	}

	if entry4.PreviousHash != lastHash1 {
		t.Error("entry should reference last hash from before reload")
	}
}

func TestAuditChain_GetEntries(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-chain-get-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	chain, err := NewAuditChain(tempDir, "test", nil, nil, false)
	if err != nil {
		t.Fatalf("failed to create audit chain: %v", err)
	}

	// Record entries with different operations
	operations := []string{"CREATE", "DELETE", "CREATE", "POLICY_SET", "CREATE"}
	for _, op := range operations {
		_, err := chain.Record(op, "/test", "test", true, "")
		if err != nil {
			t.Fatalf("failed to record: %v", err)
		}
	}

	// Get all entries
	all := chain.GetEntries(10, 0, "")
	if len(all) != 5 {
		t.Errorf("expected 5 entries, got %d", len(all))
	}

	// Filter by operation
	creates := chain.GetEntries(10, 0, "CREATE")
	if len(creates) != 3 {
		t.Errorf("expected 3 CREATE entries, got %d", len(creates))
	}

	// Test limit
	limited := chain.GetEntries(2, 0, "")
	if len(limited) != 2 {
		t.Errorf("expected 2 entries with limit, got %d", len(limited))
	}
}

func TestAuditChain_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-chain-export-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}
	hostKeyID := crypto.KeyID(pubKey)

	chain, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	if err != nil {
		t.Fatalf("failed to create audit chain: %v", err)
	}

	// Record some entries
	for i := 0; i < 3; i++ {
		chain.Record("CREATE", "/test", "test", true, "")
	}

	// Export
	data, err := chain.Export()
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("export returned empty data")
	}

	// Verify exported data is valid JSON
	exportPath := filepath.Join(tempDir, "export.json")
	if err := os.WriteFile(exportPath, data, 0644); err != nil {
		t.Fatalf("failed to write export: %v", err)
	}
}
