export type Role = "owner" | "host";

export interface FilesystemEntry {
  name: string;
  path: string;
  isDir: boolean;
  size?: number;
  modTime: string;
}

export interface KeyHolder {
  id: string;
  name: string;
  publicKey?: string;
  address?: string;
  joinedAt?: string;
  isOwner?: boolean;
}

export interface ConsensusConfig {
  threshold: number; // m (required approvals)
  totalKeys: number; // n (total participants)
  keyHolders: KeyHolder[];
  requireApproval?: boolean; // For 1/1 solo mode
}

export interface Approval {
  keyHolderId: string;
  keyHolderName?: string;
  signature?: string;
  approvedAt: string;
}

export interface VaultConfig {
  name: string;
  role: Role;
  repoUrl: string;
  password?: string; // Only for owner, hex encoded

  // SSS mode (legacy)
  localShare?: string; // Hex encoded share
  shareIndex?: number;
  peerShare?: string; // Hex encoded share to give to peer
  peerShareIndex?: number;

  // Consensus mode
  publicKey?: string; // Ed25519 public key (hex)
  privateKey?: string; // Ed25519 private key (hex)
  keyId?: string; // Derived from public key
  consensus?: ConsensusConfig;

  // Backup configuration
  backupPaths?: string[];

  // Host configuration
  storagePath?: string;
  storageQuotaBytes?: number;

  // Contract (agreed terms between owner and host)
  contract?: VaultContract;
}

export interface PendingRequest {
  id: string;
  requester: string;
  snapshotId: string;
  paths?: string[];
  reason: string;
  status: "pending" | "approved" | "denied" | "expired";
  createdAt: string;
  expiresAt: string;
  approvedAt?: string;
  approvedBy?: string;
  requiredApprovals?: number;
  approvals?: Approval[];
}

export type Step = "welcome" | "init" | "join" | "dashboard";

export type ConsensusPreset = "solo" | "dual" | "twoOfThree" | "custom";

/**
 * VaultContract defines the immutable agreement between owner and host.
 * Once both parties sign, these terms cannot be changed without mutual consent.
 */
export interface VaultContract {
  version: 1;
  createdAt: string;

  // Storage terms
  storageQuotaBytes?: number; // Max storage allowed (undefined = unlimited)
  appendOnly: boolean; // If true, backups cannot be deleted by anyone
  retentionDays?: number; // Min days to keep backups (undefined = forever)

  // Deletion terms
  deletionMode?: "both-required" | "owner-only" | "time-lock-only" | "never";

  // Restore terms
  restoreApproval: "owner-only" | "host-only" | "both-required" | "either";
  consensusThreshold?: number; // m in m-of-n (for multi-party setups)
  consensusTotalKeys?: number; // n in m-of-n

  // Signatures (hex-encoded)
  ownerSignature?: string;
  hostSignature?: string;
  ownerKeyId?: string;
  hostKeyId?: string;
}
