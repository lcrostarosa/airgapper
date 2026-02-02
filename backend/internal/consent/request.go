package consent

import "time"

// Request is an interface for consent requests (restore or deletion).
// Both RestoreRequest and DeletionRequest implement this interface.
type Request interface {
	// GetID returns the unique request ID.
	GetID() string
	// GetStatus returns the current request status.
	GetStatus() RequestStatus
	// SetStatus updates the request status.
	SetStatus(RequestStatus)
	// GetExpiresAt returns when the request expires.
	GetExpiresAt() time.Time
	// GetApprovals returns the list of approvals for this request.
	GetApprovals() []Approval
	// AddApproval adds an approval to the request.
	AddApproval(Approval)
	// GetRequiredApprovals returns the number of approvals needed.
	GetRequiredApprovals() int
}

// --- RestoreRequest implements Request ---

func (r *RestoreRequest) GetID() string             { return r.ID }
func (r *RestoreRequest) GetStatus() RequestStatus  { return r.Status }
func (r *RestoreRequest) SetStatus(s RequestStatus) { r.Status = s }
func (r *RestoreRequest) GetExpiresAt() time.Time   { return r.ExpiresAt }
func (r *RestoreRequest) GetApprovals() []Approval  { return r.Approvals }
func (r *RestoreRequest) AddApproval(a Approval)    { r.Approvals = append(r.Approvals, a) }
func (r *RestoreRequest) GetRequiredApprovals() int { return r.RequiredApprovals }

// --- DeletionRequest implements Request ---

func (r *DeletionRequest) GetID() string             { return r.ID }
func (r *DeletionRequest) GetStatus() RequestStatus  { return r.Status }
func (r *DeletionRequest) SetStatus(s RequestStatus) { r.Status = s }
func (r *DeletionRequest) GetExpiresAt() time.Time   { return r.ExpiresAt }
func (r *DeletionRequest) GetApprovals() []Approval  { return r.Approvals }
func (r *DeletionRequest) AddApproval(a Approval)    { r.Approvals = append(r.Approvals, a) }
func (r *DeletionRequest) GetRequiredApprovals() int { return r.RequiredApprovals }
