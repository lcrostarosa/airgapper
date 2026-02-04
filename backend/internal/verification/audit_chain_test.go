package verification

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditChain_Record(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "audit-chain-test")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Generate test keys
	pubKey, privKey, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "failed to generate keys")
	hostKeyID := crypto.KeyID(pubKey)

	// Create audit chain with signing enabled
	chain, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	require.NoError(t, err, "failed to create audit chain")

	// Record some entries
	entry1, err := chain.Record("CREATE", "/repo/data/abc123", "created file", true, "")
	require.NoError(t, err, "failed to record entry 1")

	assert.Equal(t, uint64(1), entry1.Sequence, "expected sequence 1")
	assert.Equal(t, "CREATE", entry1.Operation, "expected operation CREATE")
	assert.NotEmpty(t, entry1.HostSignature, "expected host signature to be set")

	// Record another entry
	entry2, err := chain.Record("DELETE", "/repo/data/def456", "deleted file", true, "")
	require.NoError(t, err, "failed to record entry 2")

	assert.Equal(t, uint64(2), entry2.Sequence, "expected sequence 2")
	assert.Equal(t, entry1.ContentHash, entry2.PreviousHash, "entry2 should reference entry1's hash")
}

func TestAuditChain_Verify(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-chain-verify-test")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tempDir) }()

	pubKey, privKey, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "failed to generate keys")
	hostKeyID := crypto.KeyID(pubKey)

	chain, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	require.NoError(t, err, "failed to create audit chain")

	// Record multiple entries
	for i := 0; i < 5; i++ {
		_, err := chain.Record("CREATE", "/test/path", "test", true, "")
		require.NoError(t, err, "failed to record entry %d", i)
	}

	// Verify the chain
	result, err := chain.Verify()
	require.NoError(t, err, "verification failed with error")

	assert.True(t, result.Valid, "expected chain to be valid, got errors: %v", result.Errors)
	assert.Equal(t, 5, result.TotalEntries, "expected 5 entries")
	assert.Equal(t, 5, result.SignedEntries, "expected 5 signed entries")
}

func TestAuditChain_Persistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-chain-persist-test")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tempDir) }()

	pubKey, privKey, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "failed to generate keys")
	hostKeyID := crypto.KeyID(pubKey)

	// Create chain and record entries
	chain1, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	require.NoError(t, err, "failed to create audit chain")

	for i := 0; i < 3; i++ {
		_, err := chain1.Record("CREATE", "/test", "test", true, "")
		require.NoError(t, err, "failed to record")
	}

	lastHash1 := chain1.GetLatestHash()
	seq1 := chain1.GetSequence()

	// Create new chain from same directory (simulating restart)
	chain2, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	require.NoError(t, err, "failed to reload audit chain")

	assert.Equal(t, seq1, chain2.GetSequence(), "sequence mismatch")
	assert.Equal(t, lastHash1, chain2.GetLatestHash(), "last hash mismatch after reload")

	// Continue recording
	entry4, err := chain2.Record("DELETE", "/test", "test", true, "")
	require.NoError(t, err, "failed to record after reload")

	assert.Equal(t, uint64(4), entry4.Sequence, "expected sequence 4")
	assert.Equal(t, lastHash1, entry4.PreviousHash, "entry should reference last hash from before reload")
}

func TestAuditChain_GetEntries(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-chain-get-test")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tempDir) }()

	chain, err := NewAuditChain(tempDir, "test", nil, nil, false)
	require.NoError(t, err, "failed to create audit chain")

	// Record entries with different operations
	operations := []string{"CREATE", "DELETE", "CREATE", "POLICY_SET", "CREATE"}
	for _, op := range operations {
		_, err := chain.Record(op, "/test", "test", true, "")
		require.NoError(t, err, "failed to record")
	}

	// Get all entries
	all := chain.GetEntries(10, 0, "")
	assert.Len(t, all, 5, "expected 5 entries")

	// Filter by operation
	creates := chain.GetEntries(10, 0, "CREATE")
	assert.Len(t, creates, 3, "expected 3 CREATE entries")

	// Test limit
	limited := chain.GetEntries(2, 0, "")
	assert.Len(t, limited, 2, "expected 2 entries with limit")
}

func TestAuditChain_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-chain-export-test")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tempDir) }()

	pubKey, privKey, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "failed to generate keys")
	hostKeyID := crypto.KeyID(pubKey)

	chain, err := NewAuditChain(tempDir, hostKeyID, privKey, pubKey, true)
	require.NoError(t, err, "failed to create audit chain")

	// Record some entries
	for i := 0; i < 3; i++ {
		_, err = chain.Record("CREATE", "/test", "test", true, "")
		require.NoError(t, err, "failed to record entry")
	}

	// Export
	data, err := chain.Export()
	require.NoError(t, err, "export failed")

	assert.NotEmpty(t, data, "export returned empty data")

	// Verify exported data is valid JSON
	exportPath := filepath.Join(tempDir, "export.json")
	err = os.WriteFile(exportPath, data, 0644)
	require.NoError(t, err, "failed to write export")
}
