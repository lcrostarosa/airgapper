package testutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// RepositoryFixture represents a test restic-like repository structure
type RepositoryFixture struct {
	// BasePath is the parent directory containing the repo
	BasePath string
	// RepoName is the name of the repository directory
	RepoName string
	// RepoPath is the full path to the repository
	RepoPath string
	// DataFiles tracks the created data files and their hashes
	DataFiles []DataFileInfo
	// ConfigData is the repository config content
	ConfigData []byte
	// KeyData is the repository key content
	KeyData []byte
	// SnapshotData tracks snapshot file content
	SnapshotData map[string][]byte
}

// DataFileInfo holds information about a created data file
type DataFileInfo struct {
	Path    string
	Content []byte
	Hash    [32]byte
	HashHex string
}

// RepositoryFixtureBuilder constructs repository fixtures
type RepositoryFixtureBuilder struct {
	basePath     string
	repoName     string
	dataFileCount int
	configData   []byte
	keyData      []byte
	snapshots    map[string][]byte
	t            *testing.T
}

// NewRepositoryFixture starts building a repository fixture
func NewRepositoryFixture(t *testing.T) *RepositoryFixtureBuilder {
	return &RepositoryFixtureBuilder{
		basePath:     t.TempDir(),
		repoName:     "testrepo",
		dataFileCount: 5,
		configData:   []byte("test config data"),
		keyData:      []byte("key data"),
		snapshots:    map[string][]byte{"snap123": []byte("snapshot data")},
		t:            t,
	}
}

// WithBasePath sets the base directory (defaults to t.TempDir())
func (b *RepositoryFixtureBuilder) WithBasePath(path string) *RepositoryFixtureBuilder {
	b.basePath = path
	return b
}

// WithRepoName sets the repository directory name
func (b *RepositoryFixtureBuilder) WithRepoName(name string) *RepositoryFixtureBuilder {
	b.repoName = name
	return b
}

// WithDataFileCount sets how many data files to create
func (b *RepositoryFixtureBuilder) WithDataFileCount(count int) *RepositoryFixtureBuilder {
	b.dataFileCount = count
	return b
}

// WithConfigData sets custom config file content
func (b *RepositoryFixtureBuilder) WithConfigData(data []byte) *RepositoryFixtureBuilder {
	b.configData = data
	return b
}

// WithKeyData sets custom key file content
func (b *RepositoryFixtureBuilder) WithKeyData(data []byte) *RepositoryFixtureBuilder {
	b.keyData = data
	return b
}

// WithSnapshot adds a snapshot with given ID and content
func (b *RepositoryFixtureBuilder) WithSnapshot(id string, data []byte) *RepositoryFixtureBuilder {
	b.snapshots[id] = data
	return b
}

// Build creates the repository structure on disk
func (b *RepositoryFixtureBuilder) Build() (*RepositoryFixture, error) {
	repoPath := filepath.Join(b.basePath, b.repoName)

	// Create directory structure
	dirs := []string{"data", "keys", "snapshots", "index", "locks"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(repoPath, dir), 0755); err != nil {
			return nil, fmt.Errorf("failed to create dir %s: %w", dir, err)
		}
	}

	fixture := &RepositoryFixture{
		BasePath:     b.basePath,
		RepoName:     b.repoName,
		RepoPath:     repoPath,
		DataFiles:    make([]DataFileInfo, 0, b.dataFileCount),
		ConfigData:   b.configData,
		KeyData:      b.keyData,
		SnapshotData: b.snapshots,
	}

	// Create config file
	if err := os.WriteFile(filepath.Join(repoPath, "config"), b.configData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	// Create data files (content-addressable)
	for i := 0; i < b.dataFileCount; i++ {
		content := []byte{byte(i), byte(i + 1), byte(i + 2)}
		hash := sha256.Sum256(content)
		hashHex := hex.EncodeToString(hash[:])

		// Data files go in subdirectories by first 2 chars
		subdir := filepath.Join(repoPath, "data", hashHex[:2])
		if err := os.MkdirAll(subdir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data subdir: %w", err)
		}

		filePath := filepath.Join(subdir, hashHex)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			return nil, fmt.Errorf("failed to write data file: %w", err)
		}

		fixture.DataFiles = append(fixture.DataFiles, DataFileInfo{
			Path:    filePath,
			Content: content,
			Hash:    hash,
			HashHex: hashHex,
		})
	}

	// Create key file
	keyHash := sha256.Sum256(b.keyData)
	keyPath := filepath.Join(repoPath, "keys", hex.EncodeToString(keyHash[:]))
	if err := os.WriteFile(keyPath, b.keyData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	// Create snapshot files
	for id, data := range b.snapshots {
		snapPath := filepath.Join(repoPath, "snapshots", id)
		if err := os.WriteFile(snapPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write snapshot %s: %w", id, err)
		}
	}

	return fixture, nil
}

// MustBuild creates the fixture or fails the test
func (b *RepositoryFixtureBuilder) MustBuild() *RepositoryFixture {
	f, err := b.Build()
	if err != nil {
		b.t.Fatalf("Failed to build repository fixture: %v", err)
	}
	return f
}

// CorruptDataFile modifies a data file to introduce corruption
func (f *RepositoryFixture) CorruptDataFile(index int) error {
	if index < 0 || index >= len(f.DataFiles) {
		return fmt.Errorf("invalid data file index %d", index)
	}

	// Write garbage to the file
	return os.WriteFile(f.DataFiles[index].Path, []byte("CORRUPTED DATA"), 0644)
}

// DeleteDataFile removes a data file
func (f *RepositoryFixture) DeleteDataFile(index int) error {
	if index < 0 || index >= len(f.DataFiles) {
		return fmt.Errorf("invalid data file index %d", index)
	}

	return os.Remove(f.DataFiles[index].Path)
}

// AddExtraDataFile adds a new data file to the repository
func (f *RepositoryFixture) AddExtraDataFile(content []byte) (*DataFileInfo, error) {
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])

	subdir := filepath.Join(f.RepoPath, "data", hashHex[:2])
	if err := os.MkdirAll(subdir, 0755); err != nil {
		return nil, err
	}

	filePath := filepath.Join(subdir, hashHex)
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return nil, err
	}

	info := &DataFileInfo{
		Path:    filePath,
		Content: content,
		Hash:    hash,
		HashHex: hashHex,
	}
	f.DataFiles = append(f.DataFiles, *info)

	return info, nil
}

// Cleanup removes the repository (usually not needed with t.TempDir())
func (f *RepositoryFixture) Cleanup() error {
	return os.RemoveAll(f.BasePath)
}
