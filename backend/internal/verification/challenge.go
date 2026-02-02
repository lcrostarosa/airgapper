package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// FileChallenge represents a single file to verify.
type FileChallenge struct {
	Path         string `json:"path"`
	ExpectedHash string `json:"expected_hash,omitempty"` // Owner's expected hash (optional)
}

// Challenge represents a verification challenge from owner to host.
type Challenge struct {
	ID             string          `json:"id"`
	OwnerKeyID     string          `json:"owner_key_id"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
	Requests       []FileChallenge `json:"requests"`
	OwnerSignature string          `json:"owner_signature"`
}

// FileProof represents the host's proof for a single file.
type FileProof struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Hash   string `json:"hash,omitempty"`   // SHA256 of file contents
	Size   int64  `json:"size,omitempty"`   // File size in bytes
	Error  string `json:"error,omitempty"`  // Error if file couldn't be read
}

// ChallengeResponse is the host's response to a challenge.
type ChallengeResponse struct {
	ChallengeID   string      `json:"challenge_id"`
	HostKeyID     string      `json:"host_key_id"`
	RespondedAt   time.Time   `json:"responded_at"`
	Proofs        []FileProof `json:"proofs"`
	HostSignature string      `json:"host_signature"`
}

// ChallengeVerificationResult contains the owner's verification of a response.
type ChallengeVerificationResult struct {
	ChallengeID    string    `json:"challenge_id"`
	VerifiedAt     time.Time `json:"verified_at"`
	Valid          bool      `json:"valid"`
	TotalFiles     int       `json:"total_files"`
	ExistingFiles  int       `json:"existing_files"`
	MissingFiles   int       `json:"missing_files"`
	HashMatches    int       `json:"hash_matches"`
	HashMismatches int       `json:"hash_mismatches"`
	Errors         []string  `json:"errors,omitempty"`
	MissingPaths   []string  `json:"missing_paths,omitempty"`
	MismatchPaths  []string  `json:"mismatch_paths,omitempty"`
}

// ChallengeManager manages challenge-response operations.
type ChallengeManager struct {
	basePath       string
	storagePath    string
	hostPrivateKey []byte
	hostPublicKey  []byte
	hostKeyID      string
	ownerPublicKey []byte
	expiryMinutes  int

	mu         sync.RWMutex
	challenges map[string]*Challenge
	responses  map[string]*ChallengeResponse
}

// NewChallengeManager creates a new challenge manager for host-side operations.
func NewChallengeManager(basePath, storagePath string, hostPrivateKey, hostPublicKey, ownerPublicKey []byte, hostKeyID string, expiryMinutes int) (*ChallengeManager, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create challenge directory: %w", err)
	}

	if expiryMinutes <= 0 {
		expiryMinutes = 60
	}

	cm := &ChallengeManager{
		basePath:       basePath,
		storagePath:    storagePath,
		hostPrivateKey: hostPrivateKey,
		hostPublicKey:  hostPublicKey,
		hostKeyID:      hostKeyID,
		ownerPublicKey: ownerPublicKey,
		expiryMinutes:  expiryMinutes,
		challenges:     make(map[string]*Challenge),
		responses:      make(map[string]*ChallengeResponse),
	}

	if err := cm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load challenges: %w", err)
	}

	return cm, nil
}

// challengesPath returns the path to the challenges file.
func (cm *ChallengeManager) challengesPath() string {
	return filepath.Join(cm.basePath, "challenges.json")
}

// responsesPath returns the path to the responses file.
func (cm *ChallengeManager) responsesPath() string {
	return filepath.Join(cm.basePath, "challenge-responses.json")
}

// load reads challenges and responses from disk.
func (cm *ChallengeManager) load() error {
	// Load challenges
	data, err := os.ReadFile(cm.challengesPath())
	if err == nil {
		var challenges []*Challenge
		if err := json.Unmarshal(data, &challenges); err == nil {
			for _, c := range challenges {
				cm.challenges[c.ID] = c
			}
		}
	}

	// Load responses
	respData, err := os.ReadFile(cm.responsesPath())
	if err == nil {
		var responses []*ChallengeResponse
		if err := json.Unmarshal(respData, &responses); err == nil {
			for _, r := range responses {
				cm.responses[r.ChallengeID] = r
			}
		}
	}

	return nil
}

// save writes challenges and responses to disk.
func (cm *ChallengeManager) save() error {
	// Save challenges
	challenges := make([]*Challenge, 0, len(cm.challenges))
	for _, c := range cm.challenges {
		challenges = append(challenges, c)
	}

	data, err := json.MarshalIndent(challenges, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal challenges: %w", err)
	}

	if err := os.WriteFile(cm.challengesPath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write challenges: %w", err)
	}

	// Save responses
	responses := make([]*ChallengeResponse, 0, len(cm.responses))
	for _, r := range cm.responses {
		responses = append(responses, r)
	}

	respData, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal responses: %w", err)
	}

	if err := os.WriteFile(cm.responsesPath(), respData, 0600); err != nil {
		return fmt.Errorf("failed to write responses: %w", err)
	}

	return nil
}

// CreateChallenge creates a new challenge (owner side).
func CreateChallenge(ownerPrivateKey []byte, ownerKeyID string, requests []FileChallenge, expiryMinutes int) (*Challenge, error) {
	now := time.Now()

	if expiryMinutes <= 0 {
		expiryMinutes = 60
	}

	challenge := &Challenge{
		ID:         generateChallengeID(),
		OwnerKeyID: ownerKeyID,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(expiryMinutes) * time.Minute),
		Requests:   requests,
	}

	// Sign the challenge
	hash, err := computeChallengeHash(challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to compute challenge hash: %w", err)
	}

	sig, err := crypto.Sign(ownerPrivateKey, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign challenge: %w", err)
	}

	challenge.OwnerSignature = hex.EncodeToString(sig)

	return challenge, nil
}

// generateChallengeID creates a unique challenge ID.
func generateChallengeID() string {
	data := fmt.Sprintf("%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "chl-" + hex.EncodeToString(hash[:8])
}

// computeChallengeHash creates a deterministic hash of challenge content.
func computeChallengeHash(challenge *Challenge) ([]byte, error) {
	// Sort requests for canonical ordering
	sortedRequests := make([]FileChallenge, len(challenge.Requests))
	copy(sortedRequests, challenge.Requests)
	sort.Slice(sortedRequests, func(i, j int) bool {
		return sortedRequests[i].Path < sortedRequests[j].Path
	})

	hashData := struct {
		ID         string          `json:"id"`
		OwnerKeyID string          `json:"owner_key_id"`
		CreatedAt  int64           `json:"created_at"`
		ExpiresAt  int64           `json:"expires_at"`
		Requests   []FileChallenge `json:"requests"`
	}{
		ID:         challenge.ID,
		OwnerKeyID: challenge.OwnerKeyID,
		CreatedAt:  challenge.CreatedAt.Unix(),
		ExpiresAt:  challenge.ExpiresAt.Unix(),
		Requests:   sortedRequests,
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// ReceiveChallenge accepts a challenge from the owner (host side).
func (cm *ChallengeManager) ReceiveChallenge(challenge *Challenge) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Verify owner signature
	if err := cm.verifyChallenge(challenge); err != nil {
		return fmt.Errorf("challenge verification failed: %w", err)
	}

	// Check expiry
	if time.Now().After(challenge.ExpiresAt) {
		return errors.New("challenge has expired")
	}

	// Check if already exists
	if _, exists := cm.challenges[challenge.ID]; exists {
		return fmt.Errorf("challenge %s already received", challenge.ID)
	}

	cm.challenges[challenge.ID] = challenge

	return cm.save()
}

// verifyChallenge verifies the owner signature on a challenge.
func (cm *ChallengeManager) verifyChallenge(challenge *Challenge) error {
	if cm.ownerPublicKey == nil {
		return errors.New("owner public key not configured")
	}

	if challenge.OwnerSignature == "" {
		return errors.New("challenge not signed")
	}

	hash, err := computeChallengeHash(challenge)
	if err != nil {
		return fmt.Errorf("failed to compute challenge hash: %w", err)
	}

	sig, err := hex.DecodeString(challenge.OwnerSignature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !crypto.Verify(cm.ownerPublicKey, hash, sig) {
		return errors.New("signature verification failed")
	}

	return nil
}

// RespondToChallenge generates a response to a challenge (host side).
func (cm *ChallengeManager) RespondToChallenge(challengeID string) (*ChallengeResponse, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	challenge, exists := cm.challenges[challengeID]
	if !exists {
		return nil, fmt.Errorf("challenge %s not found", challengeID)
	}

	// Check expiry
	if time.Now().After(challenge.ExpiresAt) {
		return nil, errors.New("challenge has expired")
	}

	// Generate proofs for each requested file
	proofs := make([]FileProof, len(challenge.Requests))
	for i, req := range challenge.Requests {
		proofs[i] = cm.generateProof(req.Path)
	}

	response := &ChallengeResponse{
		ChallengeID: challengeID,
		HostKeyID:   cm.hostKeyID,
		RespondedAt: time.Now(),
		Proofs:      proofs,
	}

	// Sign the response
	if cm.hostPrivateKey != nil {
		hash, err := computeResponseHash(response)
		if err != nil {
			return nil, fmt.Errorf("failed to compute response hash: %w", err)
		}

		sig, err := crypto.Sign(cm.hostPrivateKey, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to sign response: %w", err)
		}

		response.HostSignature = hex.EncodeToString(sig)
	}

	cm.responses[challengeID] = response

	if err := cm.save(); err != nil {
		return nil, err
	}

	return response, nil
}

// generateProof creates a proof for a single file.
func (cm *ChallengeManager) generateProof(path string) FileProof {
	proof := FileProof{Path: path}

	// Resolve path relative to storage
	fullPath := path
	if cm.storagePath != "" {
		fullPath = filepath.Join(cm.storagePath, path)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			proof.Exists = false
		} else {
			proof.Error = err.Error()
		}
		return proof
	}

	proof.Exists = true
	proof.Size = info.Size()

	// Compute hash
	file, err := os.Open(fullPath)
	if err != nil {
		proof.Error = fmt.Sprintf("failed to open: %v", err)
		return proof
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		proof.Error = fmt.Sprintf("failed to hash: %v", err)
		return proof
	}

	proof.Hash = hex.EncodeToString(hasher.Sum(nil))

	return proof
}

// computeResponseHash creates a deterministic hash of response content.
func computeResponseHash(response *ChallengeResponse) ([]byte, error) {
	// Sort proofs for canonical ordering
	sortedProofs := make([]FileProof, len(response.Proofs))
	copy(sortedProofs, response.Proofs)
	sort.Slice(sortedProofs, func(i, j int) bool {
		return sortedProofs[i].Path < sortedProofs[j].Path
	})

	hashData := struct {
		ChallengeID string      `json:"challenge_id"`
		HostKeyID   string      `json:"host_key_id"`
		RespondedAt int64       `json:"responded_at"`
		Proofs      []FileProof `json:"proofs"`
	}{
		ChallengeID: response.ChallengeID,
		HostKeyID:   response.HostKeyID,
		RespondedAt: response.RespondedAt.Unix(),
		Proofs:      sortedProofs,
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// VerifyResponse verifies a challenge response (owner side).
func VerifyResponse(challenge *Challenge, response *ChallengeResponse, hostPublicKey []byte) (*ChallengeVerificationResult, error) {
	result := &ChallengeVerificationResult{
		ChallengeID: challenge.ID,
		VerifiedAt:  time.Now(),
		TotalFiles:  len(challenge.Requests),
	}

	// Verify host signature
	if response.HostSignature != "" && hostPublicKey != nil {
		hash, err := computeResponseHash(response)
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

	// Build proof map for lookup
	proofMap := make(map[string]FileProof)
	for _, p := range response.Proofs {
		proofMap[p.Path] = p
	}

	// Verify each requested file
	for _, req := range challenge.Requests {
		proof, found := proofMap[req.Path]
		if !found {
			result.MissingFiles++
			result.MissingPaths = append(result.MissingPaths, req.Path)
			continue
		}

		if !proof.Exists {
			result.MissingFiles++
			result.MissingPaths = append(result.MissingPaths, req.Path)
			continue
		}

		result.ExistingFiles++

		// Check hash if expected hash was provided
		if req.ExpectedHash != "" {
			if proof.Hash == req.ExpectedHash {
				result.HashMatches++
			} else {
				result.HashMismatches++
				result.MismatchPaths = append(result.MismatchPaths, req.Path)
			}
		}
	}

	// Determine overall validity
	result.Valid = result.MissingFiles == 0 && result.HashMismatches == 0 && len(result.Errors) == 0

	return result, nil
}

// GetChallenge retrieves a challenge by ID.
func (cm *ChallengeManager) GetChallenge(id string) *Challenge {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.challenges[id]
}

// GetResponse retrieves a response by challenge ID.
func (cm *ChallengeManager) GetResponse(challengeID string) *ChallengeResponse {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.responses[challengeID]
}

// ListChallenges returns all challenges.
func (cm *ChallengeManager) ListChallenges(pendingOnly bool) []*Challenge {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	now := time.Now()
	var result []*Challenge

	for _, c := range cm.challenges {
		if pendingOnly {
			// Pending = not expired and no response yet
			if now.After(c.ExpiresAt) {
				continue
			}
			if _, responded := cm.responses[c.ID]; responded {
				continue
			}
		}
		result = append(result, c)
	}

	return result
}

// CleanupExpired removes expired challenges and their responses.
func (cm *ChallengeManager) CleanupExpired() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, c := range cm.challenges {
		if now.After(c.ExpiresAt) {
			delete(cm.challenges, id)
			delete(cm.responses, id)
			removed++
		}
	}

	if removed > 0 {
		cm.save()
	}

	return removed
}
