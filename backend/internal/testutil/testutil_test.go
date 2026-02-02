package testutil

import (
	"bytes"
	"testing"
)

func TestGetTestSeed(t *testing.T) {
	seed := GetTestSeed(t)
	if seed == 0 {
		t.Error("seed should not be zero")
	}
}

func TestHashData(t *testing.T) {
	data := []byte("test data")
	hash1 := HashData(data)
	hash2 := HashData(data)

	if hash1 != hash2 {
		t.Error("same data should produce same hash")
	}

	differentData := []byte("different data")
	hash3 := HashData(differentData)
	if hash1 == hash3 {
		t.Error("different data should produce different hash")
	}
}

func TestHashHex(t *testing.T) {
	data := []byte("test data")
	hexHash := HashHex(data)

	if len(hexHash) != 64 {
		t.Errorf("SHA256 hex should be 64 chars, got %d", len(hexHash))
	}
}

func TestCompareHashes(t *testing.T) {
	data := []byte("test data")
	hash1 := HashData(data)
	hash2 := HashData(data)

	if !CompareHashes(hash1, hash2) {
		t.Error("identical hashes should compare equal")
	}

	hash3 := HashData([]byte("other"))
	if CompareHashes(hash1, hash3) {
		t.Error("different hashes should not compare equal")
	}
}

func TestValidateHash(t *testing.T) {
	data := []byte("test data")
	hash := HashData(data)

	if !ValidateHash(data, hash) {
		t.Error("data should validate against its own hash")
	}

	if ValidateHash([]byte("wrong data"), hash) {
		t.Error("wrong data should not validate")
	}
}

func TestPasswordFixture(t *testing.T) {
	pf := NewPasswordFixture()

	if len(pf.Raw) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(pf.Raw))
	}

	if len(pf.Hex) != 64 {
		t.Errorf("expected 64 char hex, got %d", len(pf.Hex))
	}

	if !pf.ValidateHash(pf.Bytes()) {
		t.Error("password should validate its own hash")
	}
}

func TestPasswordFixtureWithSeed(t *testing.T) {
	seed := int64(12345)
	pf1 := NewPasswordFixture(WithSeed(seed))
	pf2 := NewPasswordFixture(WithSeed(seed))

	if !bytes.Equal(pf1.Raw, pf2.Raw) {
		t.Error("same seed should produce same password")
	}
}

func TestDataFixture(t *testing.T) {
	df := NewDataFixture(100)

	if df.Size != 100 {
		t.Errorf("expected size 100, got %d", df.Size)
	}

	if len(df.Data) != 100 {
		t.Errorf("expected 100 bytes, got %d", len(df.Data))
	}

	if !df.ValidateHash(df.Data) {
		t.Error("data should validate its own hash")
	}

	if !df.ValidateContent(df.Data) {
		t.Error("data should match itself")
	}
}

func TestDataFixtureFromBytes(t *testing.T) {
	original := []byte("specific test content")
	df := NewDataFixtureFromBytes(original)

	if !bytes.Equal(df.Data, original) {
		t.Error("data should match original")
	}

	if !df.ValidateHash(original) {
		t.Error("should validate original data")
	}
}

func TestSSSFixture(t *testing.T) {
	sss, err := NewSSSFixture().
		WithRandomSecret(32).
		WithThreshold(2, 2).
		Build()

	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(sss.Shares) != 2 {
		t.Errorf("expected 2 shares, got %d", len(sss.Shares))
	}

	// Test reconstruction
	err = sss.ValidateReconstruction(0, 1)
	if err != nil {
		t.Errorf("reconstruction failed: %v", err)
	}
}

func TestSSSFixtureThresholdSchemes(t *testing.T) {
	schemes := []struct {
		k, n int
	}{
		{2, 2},
		{2, 3},
		{3, 5},
	}

	for _, scheme := range schemes {
		t.Run("", func(t *testing.T) {
			sss, err := NewSSSFixture().
				WithRandomSecret(32).
				WithThreshold(scheme.k, scheme.n).
				Build()

			if err != nil {
				t.Fatalf("build failed for %d-of-%d: %v", scheme.k, scheme.n, err)
			}

			if len(sss.Shares) != scheme.n {
				t.Errorf("expected %d shares, got %d", scheme.n, len(sss.Shares))
			}

			// Test all valid combinations
			combos := sss.AllCombinations()
			for _, combo := range combos {
				err := sss.ValidateReconstruction(combo...)
				if err != nil {
					t.Errorf("reconstruction failed for combo %v: %v", combo, err)
				}
			}
		})
	}
}

func TestSSSFixtureTamperedShare(t *testing.T) {
	sss, _ := NewSSSFixture().
		WithSecret([]byte("test-secret")).
		WithThreshold(2, 2).
		Build()

	// Reconstruct with tampered share
	reconstructed, err := sss.CombineWithTamperedShare(0, 1)
	if err != nil {
		t.Fatalf("combine failed: %v", err)
	}

	// Hash should NOT match
	if sss.ValidateReconstruction(0, 1) == nil {
		// The tampered version should be different
		if bytes.Equal(reconstructed, sss.Secret) {
			t.Error("tampered reconstruction should differ from original")
		}
	}
}

func TestCryptoKeyFixture(t *testing.T) {
	key, err := NewCryptoKeyFixture("alice")
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	if key.Name != "alice" {
		t.Errorf("expected name 'alice', got %s", key.Name)
	}

	if key.KeyID == "" {
		t.Error("KeyID should not be empty")
	}

	// Test sign/verify
	message := []byte("test message")
	sig, err := key.Sign(message)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	if !key.Verify(message, sig) {
		t.Error("verification should succeed")
	}

	if key.Verify([]byte("different message"), sig) {
		t.Error("verification should fail for different message")
	}
}

func TestCryptoKeyFixtureRoundTrip(t *testing.T) {
	key := MustNewCryptoKeyFixture("test")

	err := key.EncodeDecodeRoundTrip()
	if err != nil {
		t.Errorf("round trip failed: %v", err)
	}
}

func TestKeyHoldersFixture(t *testing.T) {
	kf, err := NewKeyHoldersFixture("alice", "bob", "charlie")
	if err != nil {
		t.Fatalf("failed to create key holders: %v", err)
	}

	if len(kf.Holders) != 3 {
		t.Errorf("expected 3 holders, got %d", len(kf.Holders))
	}

	alice := kf.Get("alice")
	if alice == nil || alice.Name != "alice" {
		t.Error("should be able to get alice by name")
	}

	bob := kf.GetByIndex(1)
	if bob == nil || bob.Name != "bob" {
		t.Error("should be able to get bob by index")
	}

	if len(kf.PublicKeys()) != 3 {
		t.Error("should return 3 public keys")
	}

	if len(kf.KeyIDs()) != 3 {
		t.Error("should return 3 key IDs")
	}
}

func TestRestoreRequestFixture(t *testing.T) {
	key := MustNewCryptoKeyFixture("alice")
	req := NewRestoreRequestFixture()

	// Sign the request
	sig, err := req.Sign(key)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	// Verify
	valid, err := req.Verify(key, sig)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !valid {
		t.Error("signature should be valid")
	}

	// Test tampered request - verification should fail
	tamperedReq := req.WithTamperedReason("malicious reason")
	valid, _ = tamperedReq.Verify(key, sig)
	if valid {
		t.Error("tampered request should fail verification")
	}
}

func TestRepositoryFixture(t *testing.T) {
	repo := NewRepositoryFixture(t).
		WithRepoName("myrepo").
		WithDataFileCount(3).
		MustBuild()

	if repo.RepoName != "myrepo" {
		t.Errorf("expected repo name 'myrepo', got %s", repo.RepoName)
	}

	if len(repo.DataFiles) != 3 {
		t.Errorf("expected 3 data files, got %d", len(repo.DataFiles))
	}

	// Test corruption
	err := repo.CorruptDataFile(0)
	if err != nil {
		t.Errorf("corrupt failed: %v", err)
	}
}

func TestConfigFixture(t *testing.T) {
	cfg := NewConfigFixture(t).
		AsOwner().
		WithName("test-owner").
		WithRepoURL("rest:http://test:8000/").
		WithPassword("testpassword").
		MustBuild()

	if cfg.Config.Name != "test-owner" {
		t.Errorf("expected name 'test-owner', got %s", cfg.Config.Name)
	}

	if !cfg.Config.IsOwner() {
		t.Error("should be owner")
	}
}

func TestConfigFixtureConsensus(t *testing.T) {
	holders := MustNewKeyHoldersFixture("alice", "bob")

	cfg := NewConfigFixture(t).
		AsOwner().
		WithName("alice").
		WithConsensusMode(2, 2).
		WithKeyHolders(holders.Holders...).
		MustBuild()

	if !cfg.Config.UsesConsensusMode() {
		t.Error("should use consensus mode")
	}

	if cfg.Config.Consensus.Threshold != 2 {
		t.Errorf("expected threshold 2, got %d", cfg.Config.Consensus.Threshold)
	}
}

func TestWorkflowFixture(t *testing.T) {
	wf := NewWorkflowFixture(t).
		With2of2SSS("alice", "bob").
		MustBuild()

	if wf.Owner.Name != "alice" {
		t.Errorf("expected owner 'alice', got %s", wf.Owner.Name)
	}

	if len(wf.Hosts) != 1 || wf.Hosts[0].Name != "bob" {
		t.Error("expected host 'bob'")
	}

	if wf.SSS == nil {
		t.Error("SSS fixture should be created")
	}

	// Simulate restore
	secret, err := wf.SimulateRestore()
	if err != nil {
		t.Fatalf("restore simulation failed: %v", err)
	}

	if !bytes.Equal(secret, wf.SSS.Secret) {
		t.Error("restored secret should match original")
	}
}

func TestWorkflowFixtureConsensus(t *testing.T) {
	wf := NewWorkflowFixture(t).
		WithConsensus(2, "alice", "bob", "charlie").
		MustBuild()

	if wf.SSS != nil {
		t.Error("SSS should be nil in consensus mode")
	}

	if wf.Threshold != 2 {
		t.Errorf("expected threshold 2, got %d", wf.Threshold)
	}

	if wf.TotalParties != 3 {
		t.Errorf("expected 3 parties, got %d", wf.TotalParties)
	}

	// Simulate consensus approval
	sigs, err := wf.SimulateConsensusApproval()
	if err != nil {
		t.Fatalf("consensus simulation failed: %v", err)
	}

	if len(sigs) != 2 {
		t.Errorf("expected 2 signatures, got %d", len(sigs))
	}

	// Verify signatures
	validCount, err := wf.VerifySignatures(sigs)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if validCount != 2 {
		t.Errorf("expected 2 valid signatures, got %d", validCount)
	}
}

func TestCombinations(t *testing.T) {
	// Test 2-of-3
	combos := combinations(3, 2)
	expected := [][]int{{0, 1}, {0, 2}, {1, 2}}

	if len(combos) != len(expected) {
		t.Fatalf("expected %d combinations, got %d", len(expected), len(combos))
	}

	for i, combo := range combos {
		if combo[0] != expected[i][0] || combo[1] != expected[i][1] {
			t.Errorf("combination %d: expected %v, got %v", i, expected[i], combo)
		}
	}

	// Test 3-of-5
	combos = combinations(5, 3)
	if len(combos) != 10 { // C(5,3) = 10
		t.Errorf("expected 10 combinations for 3-of-5, got %d", len(combos))
	}
}
