// Package restic wraps the restic backup tool
package restic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Client wraps restic operations
type Client struct {
	RepoURL  string
	Password string
}

// NewClient creates a new restic client
// Automatically adds the "rest:" prefix for HTTP/HTTPS URLs
func NewClient(repoURL, password string) *Client {
	// Add rest: prefix for HTTP URLs if not already present
	if (strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://")) &&
		!strings.HasPrefix(repoURL, "rest:") {
		repoURL = "rest:" + repoURL
	}

	return &Client{
		RepoURL:  repoURL,
		Password: password,
	}
}

// Init initializes a new restic repository
func (c *Client) Init(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "restic", "init", "-r", c.RepoURL)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+c.Password)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if repo already exists
		if strings.Contains(stderr.String(), "already initialized") {
			return nil
		}
		return fmt.Errorf("restic init failed: %s", stderr.String())
	}

	return nil
}

// Backup creates a backup of the specified paths
func (c *Client) Backup(ctx context.Context, paths []string, tags []string) error {
	if len(paths) == 0 {
		return errors.New("no paths specified for backup")
	}

	args := []string{"backup", "-r", c.RepoURL}

	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}

	args = append(args, paths...)

	cmd := exec.CommandContext(ctx, "restic", args...)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+c.Password)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Restore restores a snapshot to the target directory
func (c *Client) Restore(ctx context.Context, snapshotID, target string) error {
	if snapshotID == "" {
		snapshotID = "latest"
	}

	args := []string{"restore", "-r", c.RepoURL, snapshotID, "--target", target}

	cmd := exec.CommandContext(ctx, "restic", args...)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+c.Password)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Snapshots lists all snapshots
func (c *Client) Snapshots(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "restic", "snapshots", "-r", c.RepoURL)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+c.Password)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// Check verifies repository integrity
func (c *Client) Check(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "restic", "check", "-r", c.RepoURL)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+c.Password)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// IsInstalled checks if restic is available
func IsInstalled() bool {
	_, err := exec.LookPath("restic")
	return err == nil
}

// Version returns the restic version
func Version() (string, error) {
	cmd := exec.Command("restic", "version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
