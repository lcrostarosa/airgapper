package api

// CreateRequestBody is the request body for creating a restore request
type CreateRequestBody struct {
	SnapshotID string   `json:"snapshot_id"`
	Paths      []string `json:"paths"`
	Reason     string   `json:"reason"`
}

func (b *CreateRequestBody) Validate() error {
	if err := required("reason", b.Reason); err != nil {
		return err
	}
	if b.SnapshotID == "" {
		b.SnapshotID = "latest"
	}
	return nil
}

// ApproveBody is the request body for approving a restore request
type ApproveBody struct {
	// Share is optional - if not provided, server uses its local share
	Share      []byte `json:"share,omitempty"`
	ShareIndex byte   `json:"share_index,omitempty"`
}

// ReceiveShareBody is the request body for receiving a key share
type ReceiveShareBody struct {
	Share      []byte `json:"share"`
	ShareIndex byte   `json:"share_index"`
	RepoURL    string `json:"repo_url"`
	PeerName   string `json:"peer_name"`
}

func (b *ReceiveShareBody) Validate() error {
	if b.Share == nil {
		return ValidationError{Field: "share", Message: "share is required"}
	}
	if err := required("repo_url", b.RepoURL); err != nil {
		return err
	}
	return required("peer_name", b.PeerName)
}

// RegisterKeyHolderBody is the request body for registering a key holder
type RegisterKeyHolderBody struct {
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"` // Hex encoded
	Address   string `json:"address,omitempty"`
}

func (b *RegisterKeyHolderBody) Validate() error {
	if err := required("name", b.Name); err != nil {
		return err
	}
	return required("publicKey", b.PublicKey)
}

// VaultInitBody is the request body for initializing a vault
type VaultInitBody struct {
	Name            string   `json:"name"`
	RepoURL         string   `json:"repoUrl"`
	Threshold       int      `json:"threshold"`   // m in m-of-n
	TotalKeys       int      `json:"totalKeys"`   // n in m-of-n
	BackupPaths     []string `json:"backupPaths,omitempty"`
	RequireApproval bool     `json:"requireApproval,omitempty"` // For 1/1 solo mode
}

func (b *VaultInitBody) Validate() error {
	if err := required("name", b.Name); err != nil {
		return err
	}
	if err := required("repoUrl", b.RepoURL); err != nil {
		return err
	}
	if err := requiredInt("threshold", b.Threshold, 1); err != nil {
		return err
	}
	if b.TotalKeys < b.Threshold {
		return ValidationError{Field: "totalKeys", Message: "totalKeys must be >= threshold"}
	}
	return nil
}

// SignRequestBody is the request body for signing a restore request
type SignRequestBody struct {
	KeyHolderID string `json:"keyHolderId"`
	Signature   string `json:"signature"` // Hex encoded
}

func (b *SignRequestBody) Validate() error {
	if err := required("keyHolderId", b.KeyHolderID); err != nil {
		return err
	}
	return required("signature", b.Signature)
}

// HostInitBody is the request body for initializing a host
type HostInitBody struct {
	Name            string `json:"name"`
	StoragePath     string `json:"storagePath"`
	StorageQuota    int64  `json:"storageQuotaBytes,omitempty"`
	AppendOnly      bool   `json:"appendOnly"`
	RestoreApproval string `json:"restoreApproval"` // "both-required", "either", "owner-only", "host-only"
	RetentionDays   int    `json:"retentionDays,omitempty"`
}

func (b *HostInitBody) Validate() error {
	if err := required("name", b.Name); err != nil {
		return err
	}
	return required("storagePath", b.StoragePath)
}

// CreatePolicyBody is the request body for creating a policy
type CreatePolicyBody struct {
	OwnerName       string `json:"ownerName"`
	OwnerKeyID      string `json:"ownerKeyId"`
	OwnerPubKey     string `json:"ownerPublicKey"`
	HostName        string `json:"hostName"`
	HostKeyID       string `json:"hostKeyId"`
	HostPubKey      string `json:"hostPublicKey"`
	RetentionDays   int    `json:"retentionDays"`
	DeletionMode    string `json:"deletionMode"` // "both-required", "owner-only", "time-lock-only", "never"
	MaxStorageBytes int64  `json:"maxStorageBytes,omitempty"`
	// Signatures (optional - can be added later)
	OwnerSignature string `json:"ownerSignature,omitempty"`
	HostSignature  string `json:"hostSignature,omitempty"`
}

// PolicySignBody is the request body for signing a policy
type PolicySignBody struct {
	PolicyJSON string `json:"policyJson"`
	Signature  string `json:"signature"`
	SignerRole string `json:"signerRole"` // "owner" or "host"
}

// CreateDeletionBody is the request body for creating a deletion request
type CreateDeletionBody struct {
	DeletionType      string   `json:"deletionType"` // "snapshot", "path", "prune", "all"
	SnapshotIDs       []string `json:"snapshotIds,omitempty"`
	Paths             []string `json:"paths,omitempty"`
	Reason            string   `json:"reason"`
	RequiredApprovals int      `json:"requiredApprovals"`
}

func (b *CreateDeletionBody) Validate() error {
	if err := required("reason", b.Reason); err != nil {
		return err
	}
	if err := required("deletionType", b.DeletionType); err != nil {
		return err
	}
	if b.RequiredApprovals < 1 {
		b.RequiredApprovals = 2 // Default to both parties
	}
	return nil
}

// ApproveDeletionBody is the request body for approving a deletion request
type ApproveDeletionBody struct {
	KeyHolderID string `json:"keyHolderId"`
	Signature   string `json:"signature"` // Hex encoded
}

func (b *ApproveDeletionBody) Validate() error {
	if err := required("keyHolderId", b.KeyHolderID); err != nil {
		return err
	}
	return required("signature", b.Signature)
}

// UpdateVerificationConfigBody is the request body for updating verification config
type UpdateVerificationConfigBody struct {
	Enabled           *bool  `json:"enabled,omitempty"`
	Interval          string `json:"interval,omitempty"`
	CheckType         string `json:"checkType,omitempty"`
	RepoName          string `json:"repoName,omitempty"`
	SnapshotID        string `json:"snapshotId,omitempty"`
	AlertOnCorruption *bool  `json:"alertOnCorruption,omitempty"`
	AlertWebhook      string `json:"alertWebhook,omitempty"`
}

// ScheduleUpdateBody is the request body for updating backup schedule
type ScheduleUpdateBody struct {
	Schedule string   `json:"schedule"`
	Paths    []string `json:"paths"`
}

// CreateIntegrityRecordBody is the request body for creating an integrity record
type CreateIntegrityRecordBody struct {
	RepoName   string `json:"repoName"`
	SnapshotID string `json:"snapshotId"`
	OwnerKeyID string `json:"ownerKeyId"`
}
