package emergency

import "time"

// DeadManSwitchConfig defines inactivity-triggered actions
type DeadManSwitchConfig struct {
	Enabled        bool          `json:"enabled"`
	InactivityDays int           `json:"inactivity_days"`
	WarningDays    int           `json:"warning_days"`
	LastActivity   time.Time     `json:"last_activity"`
	OnTrigger      TriggerAction `json:"on_trigger"`
}

// TriggerAction defines what happens when dead man's switch triggers
type TriggerAction struct {
	Action        string   `json:"action"` // "notify", "unlock-escrow", "auto-approve"
	NotifyEmails  []string `json:"notify_emails,omitempty"`
	NotifyWebhook string   `json:"notify_webhook,omitempty"`
}

// WithDeadManSwitch configures the dead man's switch
func (c *Config) WithDeadManSwitch(inactivityDays int, contacts []string) *Config {
	warningDays := 7
	if inactivityDays < 30 {
		warningDays = inactivityDays / 4
	}

	c.DeadManSwitch = &DeadManSwitchConfig{
		Enabled:        true,
		InactivityDays: inactivityDays,
		WarningDays:    warningDays,
		LastActivity:   time.Now(),
		OnTrigger: TriggerAction{
			Action:       "notify",
			NotifyEmails: contacts,
		},
	}

	return c
}

// IsEnabled returns true if dead man's switch is enabled (nil-safe)
func (d *DeadManSwitchConfig) IsEnabled() bool {
	return d != nil && d.Enabled
}

// RecordActivity updates the last activity timestamp (nil-safe)
func (d *DeadManSwitchConfig) RecordActivity() {
	if d != nil {
		d.LastActivity = time.Now()
	}
}

// DaysSinceActivity returns days since last recorded activity (nil-safe)
func (d *DeadManSwitchConfig) DaysSinceActivity() int {
	if d == nil || d.LastActivity.IsZero() {
		return -1
	}
	return int(time.Since(d.LastActivity).Hours() / 24)
}

// IsTriggered returns true if the switch should trigger (nil-safe)
func (d *DeadManSwitchConfig) IsTriggered() bool {
	if !d.IsEnabled() {
		return false
	}
	return d.DaysSinceActivity() >= d.InactivityDays
}

// IsWarning returns true if we're in the warning period (nil-safe)
func (d *DeadManSwitchConfig) IsWarning() bool {
	if !d.IsEnabled() {
		return false
	}
	days := d.DaysSinceActivity()
	threshold := d.InactivityDays - d.WarningDays
	return days >= threshold && days < d.InactivityDays
}

// DaysUntilTrigger returns days until the switch triggers (nil-safe)
func (d *DeadManSwitchConfig) DaysUntilTrigger() int {
	if d == nil {
		return -1
	}
	return d.InactivityDays - d.DaysSinceActivity()
}
