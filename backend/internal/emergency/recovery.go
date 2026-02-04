package emergency

import (
	"errors"
	"time"
)

// RecoveryConfig defines m-of-n recovery share settings
type RecoveryConfig struct {
	Enabled      bool        `json:"enabled"`
	Threshold    int         `json:"threshold"`    // k shares needed
	TotalShares  int         `json:"total_shares"` // n total shares
	Custodians   []Custodian `json:"custodians,omitempty"`
	ShareIndexes []byte      `json:"share_indexes,omitempty"`
}

// Custodian represents a third-party holding a recovery share
type Custodian struct {
	Name       string `json:"name"`
	Contact    string `json:"contact,omitempty"`
	ShareIndex byte   `json:"share_index"`
	ExportedAt string `json:"exported_at,omitempty"`
}

// WithRecovery configures m-of-n recovery shares
func (c *Config) WithRecovery(threshold, totalShares int, custodians []string) *Config {
	indexes := make([]byte, totalShares)
	for i := 0; i < totalShares; i++ {
		indexes[i] = byte(i + 1)
	}

	c.Recovery = &RecoveryConfig{
		Enabled:      true,
		Threshold:    threshold,
		TotalShares:  totalShares,
		ShareIndexes: indexes,
	}

	// Add custodians (they get shares starting at index 3)
	now := time.Now().Format(time.RFC3339)
	for i, name := range custodians {
		if i+3 <= totalShares {
			c.Recovery.Custodians = append(c.Recovery.Custodians, Custodian{
				Name:       name,
				ShareIndex: byte(i + 3),
				ExportedAt: now,
			})
		}
	}

	return c
}

// IsEnabled returns true if recovery is enabled (nil-safe)
func (r *RecoveryConfig) IsEnabled() bool {
	return r != nil && r.Enabled
}

// GetThreshold returns threshold or default (nil-safe)
func (r *RecoveryConfig) GetThreshold() int {
	if r == nil || !r.Enabled {
		return 2
	}
	return r.Threshold
}

// GetTotalShares returns total shares or default (nil-safe)
func (r *RecoveryConfig) GetTotalShares() int {
	if r == nil || !r.Enabled {
		return 2
	}
	return r.TotalShares
}

// Validate checks if recovery config is valid
func (r *RecoveryConfig) Validate() error {
	if r == nil || !r.Enabled {
		return nil
	}
	if r.Threshold < 1 {
		return errors.New("threshold must be at least 1")
	}
	if r.TotalShares < r.Threshold {
		return errors.New("total shares must be >= threshold")
	}
	if r.TotalShares > 255 {
		return errors.New("total shares must be <= 255")
	}
	return nil
}
