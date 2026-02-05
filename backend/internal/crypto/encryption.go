// Package crypto provides cryptographic utilities for config encryption
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

// Argon2 parameters (OWASP recommended for 2023)
const (
	argon2Time    = 3
	argon2Memory  = 64 * 1024 // 64 MB
	argon2Threads = 4
	argon2KeyLen  = 32 // AES-256
	saltLen       = 16
	nonceLen      = 12 // GCM standard nonce size
)

// EncryptedData holds encrypted data with its encryption parameters
type EncryptedData struct {
	// Version allows future algorithm changes
	Version int `json:"version"`
	// Salt for key derivation
	Salt string `json:"salt"`
	// Nonce for AES-GCM
	Nonce string `json:"nonce"`
	// Ciphertext is the encrypted data
	Ciphertext string `json:"ciphertext"`
}

// EncryptedSecrets holds encrypted sensitive config fields
type EncryptedSecrets struct {
	// Password is the encrypted restic repository password
	Password *EncryptedData `json:"password,omitempty"`
	// PrivateKey is the encrypted Ed25519 private key
	PrivateKey *EncryptedData `json:"private_key,omitempty"`
	// LocalShare is the encrypted SSS share
	LocalShare *EncryptedData `json:"local_share,omitempty"`
	// APIKey is the encrypted API key
	APIKey *EncryptedData `json:"api_key,omitempty"`
}

// DeriveKey derives an AES-256 key from a passphrase using Argon2id
func DeriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(passphrase),
		salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)
}

// Encrypt encrypts data using AES-256-GCM with a passphrase
func Encrypt(plaintext []byte, passphrase string) (*EncryptedData, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}

	// Generate random salt
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key
	key := DeriveKey(passphrase, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedData{
		Version:    1,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

// Decrypt decrypts data encrypted with Encrypt
func Decrypt(data *EncryptedData, passphrase string) ([]byte, error) {
	if data == nil {
		return nil, nil
	}

	if data.Version != 1 {
		return nil, fmt.Errorf("unsupported encryption version: %d", data.Version)
	}

	// Decode base64 values
	salt, err := base64.StdEncoding.DecodeString(data.Salt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(data.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(data.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Derive key
	key := DeriveKey(passphrase, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("decryption failed: invalid passphrase or corrupted data")
	}

	return plaintext, nil
}

// EncryptString encrypts a string value
func EncryptString(value, passphrase string) (*EncryptedData, error) {
	if value == "" {
		return nil, nil
	}
	return Encrypt([]byte(value), passphrase)
}

// DecryptString decrypts to a string value
func DecryptString(data *EncryptedData, passphrase string) (string, error) {
	plaintext, err := Decrypt(data, passphrase)
	if err != nil {
		return "", err
	}
	if plaintext == nil {
		return "", nil
	}
	return string(plaintext), nil
}

// EncryptJSON encrypts a JSON-serializable value
func EncryptJSON(value interface{}, passphrase string) (*EncryptedData, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal value: %w", err)
	}
	return Encrypt(data, passphrase)
}

// DecryptJSON decrypts and unmarshals JSON
func DecryptJSON(data *EncryptedData, passphrase string, target interface{}) error {
	plaintext, err := Decrypt(data, passphrase)
	if err != nil {
		return err
	}
	if plaintext == nil {
		return nil
	}
	return json.Unmarshal(plaintext, target)
}

// IsEncrypted checks if secrets appear to be encrypted (has EncryptedData fields)
func IsEncrypted(secrets *EncryptedSecrets) bool {
	if secrets == nil {
		return false
	}
	return secrets.Password != nil ||
		secrets.PrivateKey != nil ||
		secrets.LocalShare != nil ||
		secrets.APIKey != nil
}
