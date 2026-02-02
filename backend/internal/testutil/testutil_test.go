package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTestSeed(t *testing.T) {
	seed := GetTestSeed(t)
	assert.NotZero(t, seed, "seed should not be zero")
}

func TestHashData(t *testing.T) {
	data := []byte("test data")
	hash1 := HashData(data)
	hash2 := HashData(data)

	assert.Equal(t, hash1, hash2, "same data should produce same hash")

	differentData := []byte("different data")
	hash3 := HashData(differentData)
	assert.NotEqual(t, hash1, hash3, "different data should produce different hash")
}

func TestHashHex(t *testing.T) {
	data := []byte("test data")
	hexHash := HashHex(data)

	assert.Len(t, hexHash, 64, "SHA256 hex should be 64 chars")
}

func TestCompareHashes(t *testing.T) {
	data := []byte("test data")
	hash1 := HashData(data)
	hash2 := HashData(data)

	assert.True(t, CompareHashes(hash1, hash2), "identical hashes should compare equal")

	hash3 := HashData([]byte("other"))
	assert.False(t, CompareHashes(hash1, hash3), "different hashes should not compare equal")
}

func TestValidateHash(t *testing.T) {
	data := []byte("test data")
	hash := HashData(data)

	assert.True(t, ValidateHash(data, hash), "data should validate against its own hash")
	assert.False(t, ValidateHash([]byte("wrong data"), hash), "wrong data should not validate")
}

func TestPasswordFixture(t *testing.T) {
	pf := NewPasswordFixture()

	assert.Len(t, pf.Raw, 32)
	assert.Len(t, pf.Hex, 64)
	assert.True(t, pf.ValidateHash(pf.Bytes()), "password should validate its own hash")
}

func TestPasswordFixtureWithSeed(t *testing.T) {
	seed := int64(12345)
	pf1 := NewPasswordFixture(WithSeed(seed))
	pf2 := NewPasswordFixture(WithSeed(seed))

	assert.Equal(t, pf1.Raw, pf2.Raw, "same seed should produce same password")
}

func TestDataFixture(t *testing.T) {
	df := NewDataFixture(100)

	assert.Equal(t, 100, df.Size)
	assert.Len(t, df.Data, 100)
	assert.True(t, df.ValidateHash(df.Data), "data should validate its own hash")
	assert.True(t, df.ValidateContent(df.Data), "data should match itself")
}

func TestDataFixtureFromBytes(t *testing.T) {
	original := []byte("specific test content")
	df := NewDataFixtureFromBytes(original)

	assert.Equal(t, original, df.Data, "data should match original")
	assert.True(t, df.ValidateHash(original), "should validate original data")
}

func TestSSSFixture(t *testing.T) {
	sss, err := NewSSSFixture().
		WithRandomSecret(32).
		WithThreshold(2, 2).
		Build()
	require.NoError(t, err, "build failed")

	assert.Len(t, sss.Shares, 2)

	// Test reconstruction
	err = sss.ValidateReconstruction(0, 1)
	assert.NoError(t, err, "reconstruction failed")
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
			require.NoError(t, err, "build failed for %d-of-%d", scheme.k, scheme.n)

			assert.Len(t, sss.Shares, scheme.n)

			// Test all valid combinations
			combos := sss.AllCombinations()
			for _, combo := range combos {
				err := sss.ValidateReconstruction(combo...)
				assert.NoError(t, err, "reconstruction failed for combo %v", combo)
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
	require.NoError(t, err, "combine failed")

	// Hash should NOT match
	if sss.ValidateReconstruction(0, 1) == nil {
		// The tampered version should be different
		assert.NotEqual(t, sss.Secret, reconstructed, "tampered reconstruction should differ from original")
	}
}

func TestCryptoKeyFixture(t *testing.T) {
	key, err := NewCryptoKeyFixture("alice")
	require.NoError(t, err, "failed to create key")

	assert.Equal(t, "alice", key.Name)
	assert.NotEmpty(t, key.KeyID)

	// Test sign/verify
	message := []byte("test message")
	sig, err := key.Sign(message)
	require.NoError(t, err, "sign failed")

	assert.True(t, key.Verify(message, sig), "verification should succeed")
	assert.False(t, key.Verify([]byte("different message"), sig), "verification should fail for different message")
}

func TestCryptoKeyFixtureRoundTrip(t *testing.T) {
	key := MustNewCryptoKeyFixture("test")

	err := key.EncodeDecodeRoundTrip()
	assert.NoError(t, err, "round trip failed")
}

func TestKeyHoldersFixture(t *testing.T) {
	kf, err := NewKeyHoldersFixture("alice", "bob", "charlie")
	require.NoError(t, err, "failed to create key holders")

	assert.Len(t, kf.Holders, 3)

	alice := kf.Get("alice")
	require.NotNil(t, alice)
	assert.Equal(t, "alice", alice.Name)

	bob := kf.GetByIndex(1)
	require.NotNil(t, bob)
	assert.Equal(t, "bob", bob.Name)

	assert.Len(t, kf.PublicKeys(), 3)
	assert.Len(t, kf.KeyIDs(), 3)
}

func TestRestoreRequestFixture(t *testing.T) {
	key := MustNewCryptoKeyFixture("alice")
	req := NewRestoreRequestFixture()

	// Sign the request
	sig, err := req.Sign(key)
	require.NoError(t, err, "sign failed")

	// Verify
	valid, err := req.Verify(key, sig)
	require.NoError(t, err, "verify failed")
	assert.True(t, valid, "signature should be valid")

	// Test tampered request - verification should fail
	tamperedReq := req.WithTamperedReason("malicious reason")
	valid, _ = tamperedReq.Verify(key, sig)
	assert.False(t, valid, "tampered request should fail verification")
}

func TestRepositoryFixture(t *testing.T) {
	repo := NewRepositoryFixture(t).
		WithRepoName("myrepo").
		WithDataFileCount(3).
		MustBuild()

	assert.Equal(t, "myrepo", repo.RepoName)
	assert.Len(t, repo.DataFiles, 3)

	// Test corruption
	err := repo.CorruptDataFile(0)
	assert.NoError(t, err, "corrupt failed")
}

func TestConfigFixture(t *testing.T) {
	cfg := NewConfigFixture(t).
		AsOwner().
		WithName("test-owner").
		WithRepoURL("rest:http://test:8000/").
		WithPassword("testpassword").
		MustBuild()

	assert.Equal(t, "test-owner", cfg.Config.Name)
	assert.True(t, cfg.Config.IsOwner())
}

func TestConfigFixtureConsensus(t *testing.T) {
	holders := MustNewKeyHoldersFixture("alice", "bob")

	cfg := NewConfigFixture(t).
		AsOwner().
		WithName("alice").
		WithConsensusMode(2, 2).
		WithKeyHolders(holders.Holders...).
		MustBuild()

	assert.True(t, cfg.Config.UsesConsensusMode())
	assert.Equal(t, 2, cfg.Config.Consensus.Threshold)
}

func TestWorkflowFixture(t *testing.T) {
	wf := NewWorkflowFixture(t).
		With2of2SSS("alice", "bob").
		MustBuild()

	assert.Equal(t, "alice", wf.Owner.Name)
	require.Len(t, wf.Hosts, 1)
	assert.Equal(t, "bob", wf.Hosts[0].Name)
	assert.NotNil(t, wf.SSS)

	// Simulate restore
	secret, err := wf.SimulateRestore()
	require.NoError(t, err, "restore simulation failed")
	assert.Equal(t, wf.SSS.Secret, secret, "restored secret should match original")
}

func TestWorkflowFixtureConsensus(t *testing.T) {
	wf := NewWorkflowFixture(t).
		WithConsensus(2, "alice", "bob", "charlie").
		MustBuild()

	assert.Nil(t, wf.SSS, "SSS should be nil in consensus mode")
	assert.Equal(t, 2, wf.Threshold)
	assert.Equal(t, 3, wf.TotalParties)

	// Simulate consensus approval
	sigs, err := wf.SimulateConsensusApproval()
	require.NoError(t, err, "consensus simulation failed")
	assert.Len(t, sigs, 2)

	// Verify signatures
	validCount, err := wf.VerifySignatures(sigs)
	require.NoError(t, err, "verification failed")
	assert.Equal(t, 2, validCount)
}

func TestCombinations(t *testing.T) {
	// Test 2-of-3
	combos := combinations(3, 2)
	expected := [][]int{{0, 1}, {0, 2}, {1, 2}}

	require.Len(t, combos, len(expected))

	for i, combo := range combos {
		assert.Equal(t, expected[i][0], combo[0], "combination %d", i)
		assert.Equal(t, expected[i][1], combo[1], "combination %d", i)
	}

	// Test 3-of-5
	combos = combinations(5, 3)
	assert.Len(t, combos, 10, "C(5,3) = 10")
}
