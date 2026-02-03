package testutil

import (
	"crypto/sha256"
	"encoding/hex"
)

// PasswordFixture represents a test password with pre-computed values
type PasswordFixture struct {
	// Raw is the raw password bytes (typically 32 bytes)
	Raw []byte
	// Hex is the hex-encoded password (typically 64 chars for 32 bytes)
	Hex string
	// Hash is the SHA256 hash of the hex-encoded password
	Hash [32]byte
}

// NewPasswordFixture creates a new password fixture.
// By default, generates a random 32-byte password.
func NewPasswordFixture(opts ...FixtureOption) *PasswordFixture {
	r := newRand(opts...)
	return NewPasswordFixtureWithSize(32, r)
}

// NewPasswordFixtureWithSize creates a password fixture with specified byte size
func NewPasswordFixtureWithSize(size int, r interface{ Read([]byte) (int, error) }) *PasswordFixture {
	raw := make([]byte, size)
	if r != nil {
		_, _ = r.Read(raw)
	}
	hexStr := hex.EncodeToString(raw)
	hash := sha256.Sum256([]byte(hexStr))

	return &PasswordFixture{
		Raw:  raw,
		Hex:  hexStr,
		Hash: hash,
	}
}

// ValidateHash checks if data matches the password's hash
func (p *PasswordFixture) ValidateHash(data []byte) bool {
	h := sha256.Sum256(data)
	return h == p.Hash
}

// Bytes returns the hex-encoded password as bytes (what SSS typically operates on)
func (p *PasswordFixture) Bytes() []byte {
	return []byte(p.Hex)
}

// DataFixture represents arbitrary test data with pre-computed hash
type DataFixture struct {
	// Data is the raw byte content
	Data []byte
	// Hash is the SHA256 hash of the data
	Hash [32]byte
	// Size is the byte length of Data
	Size int
}

// NewDataFixture creates a data fixture with random data of specified size
func NewDataFixture(size int, opts ...FixtureOption) *DataFixture {
	r := newRand(opts...)
	data := generateRandomBytes(r, size)
	hash := sha256.Sum256(data)

	return &DataFixture{
		Data: data,
		Hash: hash,
		Size: size,
	}
}

// NewDataFixtureFromBytes creates a data fixture from existing bytes
func NewDataFixtureFromBytes(data []byte) *DataFixture {
	hash := sha256.Sum256(data)
	return &DataFixture{
		Data: data,
		Hash: hash,
		Size: len(data),
	}
}

// ValidateHash checks if data matches this fixture's hash
func (d *DataFixture) ValidateHash(data []byte) bool {
	h := sha256.Sum256(data)
	return h == d.Hash
}

// ValidateContent checks if data matches this fixture byte-for-byte
func (d *DataFixture) ValidateContent(data []byte) bool {
	if len(data) != len(d.Data) {
		return false
	}
	for i := range d.Data {
		if d.Data[i] != data[i] {
			return false
		}
	}
	return true
}
