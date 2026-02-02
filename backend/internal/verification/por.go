package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// PORChallenge represents a Proof of Retrievability challenge.
// Unlike simple existence checks, PoR proves the host can actually retrieve the data.
type PORChallenge struct {
	ID             string          `json:"id"`
	OwnerKeyID     string          `json:"owner_key_id"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
	Challenges     []BlockChallenge `json:"challenges"`
	OwnerSignature string          `json:"owner_signature"`
}

// BlockChallenge challenges specific blocks within files.
type BlockChallenge struct {
	FilePath    string `json:"file_path"`
	BlockIndex  int64  `json:"block_index"`   // Which block to prove
	BlockSize   int64  `json:"block_size"`    // Size of each block (e.g., 4KB)
	Nonce       string `json:"nonce"`         // Random nonce for this challenge
}

// PORResponse is the host's proof that data is retrievable.
type PORResponse struct {
	ChallengeID   string       `json:"challenge_id"`
	HostKeyID     string       `json:"host_key_id"`
	RespondedAt   time.Time    `json:"responded_at"`
	Proofs        []BlockProof `json:"proofs"`
	HostSignature string       `json:"host_signature"`
}

// BlockProof proves a specific block can be retrieved.
type BlockProof struct {
	FilePath     string `json:"file_path"`
	BlockIndex   int64  `json:"block_index"`
	BlockHash    string `json:"block_hash"`    // Hash of the actual block content
	CombinedHash string `json:"combined_hash"` // Hash(nonce || block_content) - proves fresh read
	Error        string `json:"error,omitempty"`
}

// PORVerificationResult contains the result of verifying a PoR response.
type PORVerificationResult struct {
	ChallengeID     string    `json:"challenge_id"`
	VerifiedAt      time.Time `json:"verified_at"`
	Valid           bool      `json:"valid"`
	TotalChallenges int       `json:"total_challenges"`
	ValidProofs     int       `json:"valid_proofs"`
	InvalidProofs   int       `json:"invalid_proofs"`
	MissingProofs   int       `json:"missing_proofs"`
	Errors          []string  `json:"errors,omitempty"`
	InvalidBlocks   []string  `json:"invalid_blocks,omitempty"`
}

// PORConfig configures Proof of Retrievability.
type PORConfig struct {
	Enabled           bool  `json:"enabled"`
	BlockSize         int64 `json:"block_size"`          // Block size in bytes (default: 4096)
	ChallengesPerFile int   `json:"challenges_per_file"` // Random blocks to challenge per file
	ExpiryMinutes     int   `json:"expiry_minutes"`      // Challenge validity
}

// DefaultPORConfig returns sensible defaults.
func DefaultPORConfig() *PORConfig {
	return &PORConfig{
		Enabled:           true,
		BlockSize:         4096, // 4KB blocks
		ChallengesPerFile: 5,    // Challenge 5 random blocks per file
		ExpiryMinutes:     60,
	}
}

// PORManager manages Proof of Retrievability operations.
type PORManager struct {
	basePath       string
	storagePath    string
	config         *PORConfig
	hostPrivateKey []byte
	hostPublicKey  []byte
	hostKeyID      string
	ownerPublicKey []byte

	mu         sync.RWMutex
	challenges map[string]*PORChallenge
	responses  map[string]*PORResponse
}

// NewPORManager creates a new PoR manager.
func NewPORManager(basePath, storagePath string, config *PORConfig, hostPrivateKey, hostPublicKey, ownerPublicKey []byte, hostKeyID string) (*PORManager, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	if config == nil {
		config = DefaultPORConfig()
	}

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create PoR directory: %w", err)
	}

	pm := &PORManager{
		basePath:       basePath,
		storagePath:    storagePath,
		config:         config,
		hostPrivateKey: hostPrivateKey,
		hostPublicKey:  hostPublicKey,
		hostKeyID:      hostKeyID,
		ownerPublicKey: ownerPublicKey,
		challenges:     make(map[string]*PORChallenge),
		responses:      make(map[string]*PORResponse),
	}

	if err := pm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load PoR state: %w", err)
	}

	return pm, nil
}

func (pm *PORManager) challengesPath() string {
	return filepath.Join(pm.basePath, "por-challenges.json")
}

func (pm *PORManager) responsesPath() string {
	return filepath.Join(pm.basePath, "por-responses.json")
}

func (pm *PORManager) load() error {
	// Load challenges
	data, err := os.ReadFile(pm.challengesPath())
	if err == nil {
		var challenges []*PORChallenge
		if json.Unmarshal(data, &challenges) == nil {
			for _, c := range challenges {
				pm.challenges[c.ID] = c
			}
		}
	}

	// Load responses
	respData, err := os.ReadFile(pm.responsesPath())
	if err == nil {
		var responses []*PORResponse
		if json.Unmarshal(respData, &responses) == nil {
			for _, r := range responses {
				pm.responses[r.ChallengeID] = r
			}
		}
	}

	return nil
}

func (pm *PORManager) save() error {
	// Save challenges
	challenges := make([]*PORChallenge, 0, len(pm.challenges))
	for _, c := range pm.challenges {
		challenges = append(challenges, c)
	}

	data, err := json.MarshalIndent(challenges, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(pm.challengesPath(), data, 0600); err != nil {
		return err
	}

	// Save responses
	responses := make([]*PORResponse, 0, len(pm.responses))
	for _, r := range pm.responses {
		responses = append(responses, r)
	}

	respData, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pm.responsesPath(), respData, 0600)
}

// CreatePORChallenge creates a new PoR challenge (owner side).
func CreatePORChallenge(ownerPrivateKey []byte, ownerKeyID string, files []FileBlockInfo, challengesPerFile int, blockSize int64, expiryMinutes int) (*PORChallenge, error) {
	now := time.Now()

	// Allow negative expiry for testing (creates already-expired challenge)
	if expiryMinutes == 0 {
		expiryMinutes = 60
	}
	if blockSize <= 0 {
		blockSize = 4096
	}
	if challengesPerFile <= 0 {
		challengesPerFile = 5
	}

	challenge := &PORChallenge{
		ID:         generatePORChallengeID(),
		OwnerKeyID: ownerKeyID,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(expiryMinutes) * time.Minute),
		Challenges: []BlockChallenge{},
	}

	// Generate block challenges for each file
	for _, file := range files {
		numBlocks := file.Size / blockSize
		if file.Size%blockSize != 0 {
			numBlocks++
		}

		if numBlocks == 0 {
			continue
		}

		// Challenge random blocks
		challengeCount := challengesPerFile
		if int64(challengeCount) > numBlocks {
			challengeCount = int(numBlocks)
		}

		// Generate random block indices
		indices := generateRandomIndices(numBlocks, challengeCount)
		for _, idx := range indices {
			nonce := make([]byte, 16)
			rand.Read(nonce)

			challenge.Challenges = append(challenge.Challenges, BlockChallenge{
				FilePath:   file.Path,
				BlockIndex: idx,
				BlockSize:  blockSize,
				Nonce:      hex.EncodeToString(nonce),
			})
		}
	}

	// Sign the challenge
	hash, err := computePORChallengeHash(challenge)
	if err != nil {
		return nil, err
	}

	sig, err := crypto.Sign(ownerPrivateKey, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign challenge: %w", err)
	}

	challenge.OwnerSignature = hex.EncodeToString(sig)

	return challenge, nil
}

// FileBlockInfo contains info needed to generate block challenges.
type FileBlockInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func generatePORChallengeID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "por-" + hex.EncodeToString(b)
}

func generateRandomIndices(max int64, count int) []int64 {
	indices := make(map[int64]bool)
	result := make([]int64, 0, count)

	for len(result) < count && len(result) < int(max) {
		b := make([]byte, 8)
		rand.Read(b)
		idx := int64(0)
		for i := 0; i < 8; i++ {
			idx = (idx << 8) | int64(b[i])
		}
		if idx < 0 {
			idx = -idx
		}
		idx = idx % max

		if !indices[idx] {
			indices[idx] = true
			result = append(result, idx)
		}
	}

	return result
}

func computePORChallengeHash(c *PORChallenge) ([]byte, error) {
	hashData := struct {
		ID         string           `json:"id"`
		OwnerKeyID string           `json:"owner_key_id"`
		CreatedAt  int64            `json:"created_at"`
		ExpiresAt  int64            `json:"expires_at"`
		Challenges []BlockChallenge `json:"challenges"`
	}{
		ID:         c.ID,
		OwnerKeyID: c.OwnerKeyID,
		CreatedAt:  c.CreatedAt.Unix(),
		ExpiresAt:  c.ExpiresAt.Unix(),
		Challenges: c.Challenges,
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// ReceivePORChallenge accepts a PoR challenge from the owner.
func (pm *PORManager) ReceivePORChallenge(challenge *PORChallenge) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Verify owner signature
	if err := pm.verifyPORChallenge(challenge); err != nil {
		return fmt.Errorf("challenge verification failed: %w", err)
	}

	// Check expiry
	if time.Now().After(challenge.ExpiresAt) {
		return errors.New("challenge has expired")
	}

	// Check if already exists
	if _, exists := pm.challenges[challenge.ID]; exists {
		return fmt.Errorf("challenge %s already received", challenge.ID)
	}

	pm.challenges[challenge.ID] = challenge

	return pm.save()
}

func (pm *PORManager) verifyPORChallenge(challenge *PORChallenge) error {
	if pm.ownerPublicKey == nil {
		return errors.New("owner public key not configured")
	}

	if challenge.OwnerSignature == "" {
		return errors.New("challenge not signed")
	}

	hash, err := computePORChallengeHash(challenge)
	if err != nil {
		return err
	}

	sig, err := hex.DecodeString(challenge.OwnerSignature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !crypto.Verify(pm.ownerPublicKey, hash, sig) {
		return errors.New("signature verification failed")
	}

	return nil
}

// RespondToPORChallenge generates proofs for a PoR challenge.
func (pm *PORManager) RespondToPORChallenge(challengeID string) (*PORResponse, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	challenge, exists := pm.challenges[challengeID]
	if !exists {
		return nil, fmt.Errorf("challenge %s not found", challengeID)
	}

	// Check expiry
	if time.Now().After(challenge.ExpiresAt) {
		return nil, errors.New("challenge has expired")
	}

	// Generate proofs for each block challenge
	proofs := make([]BlockProof, len(challenge.Challenges))
	for i, bc := range challenge.Challenges {
		proofs[i] = pm.generateBlockProof(bc)
	}

	response := &PORResponse{
		ChallengeID: challengeID,
		HostKeyID:   pm.hostKeyID,
		RespondedAt: time.Now(),
		Proofs:      proofs,
	}

	// Sign the response
	if pm.hostPrivateKey != nil {
		hash, err := computePORResponseHash(response)
		if err != nil {
			return nil, err
		}

		sig, err := crypto.Sign(pm.hostPrivateKey, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to sign response: %w", err)
		}

		response.HostSignature = hex.EncodeToString(sig)
	}

	pm.responses[challengeID] = response

	if err := pm.save(); err != nil {
		return nil, err
	}

	return response, nil
}

func (pm *PORManager) generateBlockProof(bc BlockChallenge) BlockProof {
	proof := BlockProof{
		FilePath:   bc.FilePath,
		BlockIndex: bc.BlockIndex,
	}

	// Resolve path
	fullPath := bc.FilePath
	if pm.storagePath != "" {
		fullPath = filepath.Join(pm.storagePath, bc.FilePath)
	}

	// Open file
	file, err := os.Open(fullPath)
	if err != nil {
		proof.Error = fmt.Sprintf("failed to open file: %v", err)
		return proof
	}
	defer file.Close()

	// Seek to block
	offset := bc.BlockIndex * bc.BlockSize
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		proof.Error = fmt.Sprintf("failed to seek: %v", err)
		return proof
	}

	// Read block
	block := make([]byte, bc.BlockSize)
	n, err := file.Read(block)
	if err != nil && err != io.EOF {
		proof.Error = fmt.Sprintf("failed to read block: %v", err)
		return proof
	}
	block = block[:n] // Trim to actual read size

	// Compute block hash
	blockHash := sha256.Sum256(block)
	proof.BlockHash = hex.EncodeToString(blockHash[:])

	// Compute combined hash (proves fresh read with nonce)
	nonce, _ := hex.DecodeString(bc.Nonce)
	combined := append(nonce, block...)
	combinedHash := sha256.Sum256(combined)
	proof.CombinedHash = hex.EncodeToString(combinedHash[:])

	return proof
}

func computePORResponseHash(r *PORResponse) ([]byte, error) {
	hashData := struct {
		ChallengeID string       `json:"challenge_id"`
		HostKeyID   string       `json:"host_key_id"`
		RespondedAt int64        `json:"responded_at"`
		Proofs      []BlockProof `json:"proofs"`
	}{
		ChallengeID: r.ChallengeID,
		HostKeyID:   r.HostKeyID,
		RespondedAt: r.RespondedAt.Unix(),
		Proofs:      r.Proofs,
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// VerifyPORResponse verifies a PoR response (owner side).
// The owner must have the original file data to verify block hashes.
func VerifyPORResponse(challenge *PORChallenge, response *PORResponse, hostPublicKey []byte, blockVerifier func(path string, blockIndex, blockSize int64, nonce, expectedCombinedHash string) bool) (*PORVerificationResult, error) {
	result := &PORVerificationResult{
		ChallengeID:     challenge.ID,
		VerifiedAt:      time.Now(),
		TotalChallenges: len(challenge.Challenges),
	}

	// Verify host signature
	if response.HostSignature != "" && hostPublicKey != nil {
		hash, err := computePORResponseHash(response)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to compute response hash: %v", err))
		} else {
			sig, err := hex.DecodeString(response.HostSignature)
			if err != nil {
				result.Errors = append(result.Errors, "invalid signature encoding")
			} else if !crypto.Verify(hostPublicKey, hash, sig) {
				result.Errors = append(result.Errors, "host signature verification failed")
			}
		}
	}

	// Build proof map
	proofMap := make(map[string]BlockProof)
	for _, p := range response.Proofs {
		key := fmt.Sprintf("%s:%d", p.FilePath, p.BlockIndex)
		proofMap[key] = p
	}

	// Verify each challenge
	for _, bc := range challenge.Challenges {
		key := fmt.Sprintf("%s:%d", bc.FilePath, bc.BlockIndex)
		proof, found := proofMap[key]

		if !found {
			result.MissingProofs++
			result.InvalidBlocks = append(result.InvalidBlocks, key)
			continue
		}

		if proof.Error != "" {
			result.InvalidProofs++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", key, proof.Error))
			continue
		}

		// If owner has block verifier, use it to verify combined hash
		if blockVerifier != nil {
			if blockVerifier(bc.FilePath, bc.BlockIndex, bc.BlockSize, bc.Nonce, proof.CombinedHash) {
				result.ValidProofs++
			} else {
				result.InvalidProofs++
				result.InvalidBlocks = append(result.InvalidBlocks, key)
			}
		} else {
			// Without verifier, we can only check that proof was provided
			result.ValidProofs++
		}
	}

	result.Valid = result.InvalidProofs == 0 && result.MissingProofs == 0 && len(result.Errors) == 0

	return result, nil
}

// GetChallenge retrieves a challenge by ID.
func (pm *PORManager) GetChallenge(id string) *PORChallenge {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.challenges[id]
}

// GetResponse retrieves a response by challenge ID.
func (pm *PORManager) GetResponse(challengeID string) *PORResponse {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.responses[challengeID]
}

// ListChallenges returns all challenges.
func (pm *PORManager) ListChallenges(pendingOnly bool) []*PORChallenge {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	now := time.Now()
	var result []*PORChallenge

	for _, c := range pm.challenges {
		if pendingOnly {
			if now.After(c.ExpiresAt) {
				continue
			}
			if _, responded := pm.responses[c.ID]; responded {
				continue
			}
		}
		result = append(result, c)
	}

	return result
}

// CleanupExpired removes expired challenges and responses.
func (pm *PORManager) CleanupExpired() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, c := range pm.challenges {
		if now.After(c.ExpiresAt) {
			delete(pm.challenges, id)
			delete(pm.responses, id)
			removed++
		}
	}

	if removed > 0 {
		pm.save()
	}

	return removed
}

// GetStats returns PoR statistics.
func (pm *PORManager) GetStats() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pending := 0
	responded := 0
	expired := 0
	now := time.Now()

	for id, c := range pm.challenges {
		if now.After(c.ExpiresAt) {
			expired++
		} else if _, ok := pm.responses[id]; ok {
			responded++
		} else {
			pending++
		}
	}

	return map[string]interface{}{
		"total_challenges": len(pm.challenges),
		"pending":          pending,
		"responded":        responded,
		"expired":          expired,
	}
}
