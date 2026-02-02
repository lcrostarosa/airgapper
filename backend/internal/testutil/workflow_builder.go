package testutil

import (
	"testing"
)

// WorkflowFixture represents a complete multi-party workflow setup
type WorkflowFixture struct {
	// SSS is the SSS fixture (for SSS mode)
	SSS *SSSFixture
	// Owner is the owner's key fixture
	Owner *CryptoKeyFixture
	// Hosts are the host key fixtures
	Hosts []*CryptoKeyFixture
	// AllKeys are all participant keys (owner + hosts)
	AllKeys *KeyHoldersFixture
	// OwnerConfig is the owner's config fixture
	OwnerConfig *ConfigFixture
	// Request is a pre-built restore request fixture
	Request *RestoreRequestFixture
	// Threshold is the consensus/SSS threshold (k)
	Threshold int
	// TotalParties is the total number of parties (n)
	TotalParties int
}

// WorkflowFixtureBuilder constructs complete workflow fixtures
type WorkflowFixtureBuilder struct {
	t            *testing.T
	mode         string // "sss" or "consensus"
	ownerName    string
	hostNames    []string
	threshold    int
	totalParties int
	secretSize   int
	opts         []FixtureOption
}

// NewWorkflowFixture starts building a workflow fixture
func NewWorkflowFixture(t *testing.T) *WorkflowFixtureBuilder {
	return &WorkflowFixtureBuilder{
		t:            t,
		mode:         "sss",
		ownerName:    "alice",
		hostNames:    []string{"bob"},
		threshold:    2,
		totalParties: 2,
		secretSize:   32,
	}
}

// With2of2SSS configures a 2-of-2 SSS workflow
func (b *WorkflowFixtureBuilder) With2of2SSS(owner, host string) *WorkflowFixtureBuilder {
	b.mode = "sss"
	b.ownerName = owner
	b.hostNames = []string{host}
	b.threshold = 2
	b.totalParties = 2
	return b
}

// With2of3SSS configures a 2-of-3 SSS workflow
func (b *WorkflowFixtureBuilder) With2of3SSS(owner string, hosts ...string) *WorkflowFixtureBuilder {
	b.mode = "sss"
	b.ownerName = owner
	b.hostNames = hosts
	b.threshold = 2
	b.totalParties = 1 + len(hosts)
	return b
}

// WithThresholdSSS configures a k-of-n SSS workflow
func (b *WorkflowFixtureBuilder) WithThresholdSSS(k, n int, owner string, hosts ...string) *WorkflowFixtureBuilder {
	b.mode = "sss"
	b.ownerName = owner
	b.hostNames = hosts
	b.threshold = k
	b.totalParties = n
	return b
}

// WithConsensus configures a consensus-mode workflow
func (b *WorkflowFixtureBuilder) WithConsensus(threshold int, owner string, hosts ...string) *WorkflowFixtureBuilder {
	b.mode = "consensus"
	b.ownerName = owner
	b.hostNames = hosts
	b.threshold = threshold
	b.totalParties = 1 + len(hosts)
	return b
}

// WithSecretSize sets the secret byte size for SSS mode
func (b *WorkflowFixtureBuilder) WithSecretSize(size int) *WorkflowFixtureBuilder {
	b.secretSize = size
	return b
}

// WithSeed sets deterministic seeding
func (b *WorkflowFixtureBuilder) WithSeed(seed int64) *WorkflowFixtureBuilder {
	b.opts = append(b.opts, WithSeed(seed))
	return b
}

// Build creates the workflow fixture
func (b *WorkflowFixtureBuilder) Build() (*WorkflowFixture, error) {
	fixture := &WorkflowFixture{
		Threshold:    b.threshold,
		TotalParties: b.totalParties,
	}

	// Create all participant keys
	allNames := append([]string{b.ownerName}, b.hostNames...)
	allKeys, err := NewKeyHoldersFixture(allNames...)
	if err != nil {
		return nil, err
	}
	fixture.AllKeys = allKeys
	fixture.Owner = allKeys.Get(b.ownerName)
	fixture.Hosts = make([]*CryptoKeyFixture, len(b.hostNames))
	for i, name := range b.hostNames {
		fixture.Hosts[i] = allKeys.Get(name)
	}

	// Create SSS fixture if in SSS mode
	if b.mode == "sss" {
		sss, err := NewSSSFixture().
			WithHexPassword(b.secretSize).
			WithThreshold(b.threshold, b.totalParties).
			Build()
		if err != nil {
			return nil, err
		}
		fixture.SSS = sss
	}

	// Create config fixture
	cfgBuilder := NewConfigFixture(b.t).
		AsOwner().
		WithName(b.ownerName).
		WithPeer(b.hostNames[0], "localhost:8081")

	if b.mode == "consensus" {
		cfgBuilder = cfgBuilder.
			WithConsensusMode(b.threshold, b.totalParties).
			WithKeyHolders(allKeys.Holders...)
	} else if fixture.SSS != nil {
		cfgBuilder = cfgBuilder.
			WithPassword(string(fixture.SSS.Secret)).
			WithSSSShare(fixture.SSS.Shares[0].Data, int(fixture.SSS.Shares[0].Index))
	}

	ownerCfg, err := cfgBuilder.Build()
	if err != nil {
		return nil, err
	}
	fixture.OwnerConfig = ownerCfg

	// Create restore request fixture
	fixture.Request = NewRestoreRequestFixture()
	fixture.Request.Requester = b.ownerName

	return fixture, nil
}

// MustBuild creates the fixture or fails the test
func (b *WorkflowFixtureBuilder) MustBuild() *WorkflowFixture {
	f, err := b.Build()
	if err != nil {
		b.t.Fatalf("Failed to build workflow fixture: %v", err)
	}
	return f
}

// SimulateRestore simulates a complete restore workflow in SSS mode.
// Returns the reconstructed secret or error.
func (w *WorkflowFixture) SimulateRestore() ([]byte, error) {
	if w.SSS == nil {
		return nil, nil // Not in SSS mode
	}

	// Combine all shares (in a real workflow, these would be collected from parties)
	indices := make([]int, w.Threshold)
	for i := 0; i < w.Threshold; i++ {
		indices[i] = i
	}

	return w.SSS.Combine(indices...)
}

// SimulateConsensusApproval simulates collecting enough signatures.
// Returns signatures from the first `threshold` parties.
func (w *WorkflowFixture) SimulateConsensusApproval() ([][]byte, error) {
	sigs := make([][]byte, w.Threshold)

	for i := 0; i < w.Threshold; i++ {
		key := w.AllKeys.GetByIndex(i)
		sig, err := w.Request.Sign(key)
		if err != nil {
			return nil, err
		}
		sigs[i] = sig
	}

	return sigs, nil
}

// VerifySignatures verifies that all provided signatures are valid
func (w *WorkflowFixture) VerifySignatures(sigs [][]byte) (int, error) {
	validCount := 0

	for i, sig := range sigs {
		if i >= len(w.AllKeys.Holders) {
			break
		}
		key := w.AllKeys.GetByIndex(i)
		valid, err := w.Request.Verify(key, sig)
		if err != nil {
			return validCount, err
		}
		if valid {
			validCount++
		}
	}

	return validCount, nil
}
