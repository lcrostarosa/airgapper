package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// Witness defines the interface for external verification services.
// A witness stores cryptographic checkpoints independently of the host,
// allowing detection of chain tampering even if the host is compromised.
type Witness interface {
	// SubmitCheckpoint sends a checkpoint to the witness service.
	SubmitCheckpoint(checkpoint *WitnessCheckpoint) (*WitnessReceipt, error)

	// VerifyCheckpoint retrieves and verifies a previously submitted checkpoint.
	VerifyCheckpoint(id string) (*WitnessVerification, error)

	// Ping checks if the witness service is available.
	Ping() error

	// Name returns the witness provider name.
	Name() string
}

// WitnessCheckpoint represents a point-in-time snapshot of verification state.
// Signed by both owner and host, stored by an independent witness.
type WitnessCheckpoint struct {
	ID                 string    `json:"id"`
	CreatedAt          time.Time `json:"created_at"`

	// Audit chain state
	AuditChainSequence uint64 `json:"audit_chain_sequence"`
	AuditChainHash     string `json:"audit_chain_hash"`

	// Manifest state (optional, for owner-side verification)
	ManifestMerkleRoot string `json:"manifest_merkle_root,omitempty"`
	SnapshotCount      int    `json:"snapshot_count,omitempty"`

	// Storage stats
	TotalBytes int64 `json:"total_bytes,omitempty"`
	FileCount  int   `json:"file_count,omitempty"`

	// Signatures
	OwnerKeyID     string `json:"owner_key_id,omitempty"`
	OwnerSignature string `json:"owner_signature,omitempty"`
	HostKeyID      string `json:"host_key_id"`
	HostSignature  string `json:"host_signature"`
}

// WitnessReceipt is returned after successfully submitting a checkpoint.
type WitnessReceipt struct {
	CheckpointID   string    `json:"checkpoint_id"`
	WitnessName    string    `json:"witness_name"`
	ReceivedAt     time.Time `json:"received_at"`
	WitnessHash    string    `json:"witness_hash"`    // Hash computed by witness
	WitnessProof   string    `json:"witness_proof"`   // Proof of inclusion (merkle proof, etc.)
	StorageURL     string    `json:"storage_url,omitempty"` // Where checkpoint is stored
}

// WitnessVerification is returned when verifying a checkpoint.
type WitnessVerification struct {
	CheckpointID  string             `json:"checkpoint_id"`
	VerifiedAt    time.Time          `json:"verified_at"`
	Valid         bool               `json:"valid"`
	Checkpoint    *WitnessCheckpoint `json:"checkpoint,omitempty"`
	WitnessName   string             `json:"witness_name"`
	StoredHash    string             `json:"stored_hash"`
	ComputedHash  string             `json:"computed_hash"`
	Errors        []string           `json:"errors,omitempty"`
}

// CreateCheckpoint creates a new checkpoint for witness submission.
func CreateCheckpoint(
	auditChainSequence uint64,
	auditChainHash string,
	manifestMerkleRoot string,
	snapshotCount int,
	totalBytes int64,
	fileCount int,
	hostKeyID string,
	hostPrivateKey []byte,
) (*WitnessCheckpoint, error) {
	checkpoint := &WitnessCheckpoint{
		ID:                 generateCheckpointID(),
		CreatedAt:          time.Now(),
		AuditChainSequence: auditChainSequence,
		AuditChainHash:     auditChainHash,
		ManifestMerkleRoot: manifestMerkleRoot,
		SnapshotCount:      snapshotCount,
		TotalBytes:         totalBytes,
		FileCount:          fileCount,
		HostKeyID:          hostKeyID,
	}

	// Sign with host key
	hash, err := computeCheckpointHash(checkpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to compute checkpoint hash: %w", err)
	}

	sig, err := crypto.Sign(hostPrivateKey, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign checkpoint: %w", err)
	}

	checkpoint.HostSignature = hex.EncodeToString(sig)

	return checkpoint, nil
}

// generateCheckpointID creates a unique checkpoint ID.
func generateCheckpointID() string {
	data := fmt.Sprintf("%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "chk-" + hex.EncodeToString(hash[:8])
}

// computeCheckpointHash creates a deterministic hash of checkpoint content.
func computeCheckpointHash(cp *WitnessCheckpoint) ([]byte, error) {
	hashData := struct {
		ID                 string `json:"id"`
		CreatedAt          int64  `json:"created_at"`
		AuditChainSequence uint64 `json:"audit_chain_sequence"`
		AuditChainHash     string `json:"audit_chain_hash"`
		ManifestMerkleRoot string `json:"manifest_merkle_root"`
		SnapshotCount      int    `json:"snapshot_count"`
		TotalBytes         int64  `json:"total_bytes"`
		FileCount          int    `json:"file_count"`
		HostKeyID          string `json:"host_key_id"`
		OwnerKeyID         string `json:"owner_key_id"`
	}{
		ID:                 cp.ID,
		CreatedAt:          cp.CreatedAt.Unix(),
		AuditChainSequence: cp.AuditChainSequence,
		AuditChainHash:     cp.AuditChainHash,
		ManifestMerkleRoot: cp.ManifestMerkleRoot,
		SnapshotCount:      cp.SnapshotCount,
		TotalBytes:         cp.TotalBytes,
		FileCount:          cp.FileCount,
		HostKeyID:          cp.HostKeyID,
		OwnerKeyID:         cp.OwnerKeyID,
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// AddOwnerSignature adds the owner's signature to a checkpoint.
func (cp *WitnessCheckpoint) AddOwnerSignature(ownerKeyID string, ownerPrivateKey []byte) error {
	cp.OwnerKeyID = ownerKeyID

	hash, err := computeCheckpointHash(cp)
	if err != nil {
		return fmt.Errorf("failed to compute checkpoint hash: %w", err)
	}

	sig, err := crypto.Sign(ownerPrivateKey, hash)
	if err != nil {
		return fmt.Errorf("failed to sign checkpoint: %w", err)
	}

	cp.OwnerSignature = hex.EncodeToString(sig)
	return nil
}

// VerifyHostSignature verifies the host's signature on a checkpoint.
func (cp *WitnessCheckpoint) VerifyHostSignature(hostPublicKey []byte) error {
	if cp.HostSignature == "" {
		return errors.New("no host signature")
	}

	hash, err := computeCheckpointHash(cp)
	if err != nil {
		return fmt.Errorf("failed to compute checkpoint hash: %w", err)
	}

	sig, err := hex.DecodeString(cp.HostSignature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !crypto.Verify(hostPublicKey, hash, sig) {
		return errors.New("host signature verification failed")
	}

	return nil
}

// VerifyOwnerSignature verifies the owner's signature on a checkpoint.
func (cp *WitnessCheckpoint) VerifyOwnerSignature(ownerPublicKey []byte) error {
	if cp.OwnerSignature == "" {
		return errors.New("no owner signature")
	}

	hash, err := computeCheckpointHash(cp)
	if err != nil {
		return fmt.Errorf("failed to compute checkpoint hash: %w", err)
	}

	sig, err := hex.DecodeString(cp.OwnerSignature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !crypto.Verify(ownerPublicKey, hash, sig) {
		return errors.New("owner signature verification failed")
	}

	return nil
}

// WitnessManager coordinates multiple witness providers.
type WitnessManager struct {
	witnesses []Witness
	autoSubmit bool
}

// NewWitnessManager creates a witness manager with the given providers.
func NewWitnessManager(witnesses []Witness, autoSubmit bool) *WitnessManager {
	return &WitnessManager{
		witnesses:  witnesses,
		autoSubmit: autoSubmit,
	}
}

// AddWitness adds a witness provider.
func (wm *WitnessManager) AddWitness(w Witness) {
	wm.witnesses = append(wm.witnesses, w)
}

// SubmitToAll submits a checkpoint to all configured witnesses.
func (wm *WitnessManager) SubmitToAll(checkpoint *WitnessCheckpoint) ([]*WitnessReceipt, []error) {
	var receipts []*WitnessReceipt
	var errs []error

	for _, w := range wm.witnesses {
		receipt, err := w.SubmitCheckpoint(checkpoint)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", w.Name(), err))
		} else {
			receipts = append(receipts, receipt)
		}
	}

	return receipts, errs
}

// VerifyFromAll verifies a checkpoint against all witnesses.
func (wm *WitnessManager) VerifyFromAll(checkpointID string) ([]*WitnessVerification, []error) {
	var verifications []*WitnessVerification
	var errs []error

	for _, w := range wm.witnesses {
		verification, err := w.VerifyCheckpoint(checkpointID)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", w.Name(), err))
		} else {
			verifications = append(verifications, verification)
		}
	}

	return verifications, errs
}

// PingAll checks connectivity to all witnesses.
func (wm *WitnessManager) PingAll() map[string]error {
	results := make(map[string]error)

	for _, w := range wm.witnesses {
		results[w.Name()] = w.Ping()
	}

	return results
}

// GetWitnesses returns all configured witnesses.
func (wm *WitnessManager) GetWitnesses() []Witness {
	return wm.witnesses
}
