package emergency

// NotifyConfig defines notification settings
type NotifyConfig struct {
	Enabled   bool                `json:"enabled"`
	Providers map[string]Provider `json:"providers"`
	Events    EventConfig         `json:"events"`
}

// Provider defines a notification provider
type Provider struct {
	Type     string            `json:"type"`
	Enabled  bool              `json:"enabled"`
	Settings map[string]string `json:"settings"`
	Priority string            `json:"priority"`
}

// EventConfig defines which events trigger notifications
type EventConfig struct {
	BackupStarted      bool `json:"backup_started"`
	BackupCompleted    bool `json:"backup_completed"`
	BackupFailed       bool `json:"backup_failed"`
	RestoreRequested   bool `json:"restore_requested"`
	RestoreApproved    bool `json:"restore_approved"`
	RestoreDenied      bool `json:"restore_denied"`
	DeletionRequested  bool `json:"deletion_requested"`
	DeletionApproved   bool `json:"deletion_approved"`
	ConsensusReceived  bool `json:"consensus_received"`
	EmergencyTriggered bool `json:"emergency_triggered"`
	DeadManWarning     bool `json:"dead_man_warning"`
	HeartbeatMissed    bool `json:"heartbeat_missed"`
}

// IsEnabled returns true if notifications are enabled (nil-safe)
func (n *NotifyConfig) IsEnabled() bool {
	return n != nil && n.Enabled
}

// HasProviders returns true if providers are configured (nil-safe)
func (n *NotifyConfig) HasProviders() bool {
	return n != nil && len(n.Providers) > 0
}

// ProviderCount returns the number of configured providers (nil-safe)
func (n *NotifyConfig) ProviderCount() int {
	if n == nil {
		return 0
	}
	return len(n.Providers)
}

// AddProvider adds a notification provider (nil-safe - no-op if nil)
func (n *NotifyConfig) AddProvider(id string, p Provider) {
	if n == nil {
		return
	}
	if n.Providers == nil {
		n.Providers = make(map[string]Provider)
	}
	n.Providers[id] = p
	n.Enabled = true
}

// RemoveProvider removes a notification provider (nil-safe)
func (n *NotifyConfig) RemoveProvider(id string) {
	if n == nil {
		return
	}
	delete(n.Providers, id)
	if len(n.Providers) == 0 {
		n.Enabled = false
	}
}

// EnableAllEvents enables all notification events (nil-safe)
func (n *NotifyConfig) EnableAllEvents() {
	if n == nil {
		return
	}
	n.Events = EventConfig{
		BackupStarted:      true,
		BackupCompleted:    true,
		BackupFailed:       true,
		RestoreRequested:   true,
		RestoreApproved:    true,
		RestoreDenied:      true,
		DeletionRequested:  true,
		DeletionApproved:   true,
		ConsensusReceived:  true,
		EmergencyTriggered: true,
		DeadManWarning:     true,
		HeartbeatMissed:    true,
	}
}

// DisableAllEvents disables all notification events (nil-safe)
func (n *NotifyConfig) DisableAllEvents() {
	if n == nil {
		return
	}
	n.Events = EventConfig{}
}
