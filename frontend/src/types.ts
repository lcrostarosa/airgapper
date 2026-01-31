export type Role = "owner" | "host";

export interface VaultConfig {
  name: string;
  role: Role;
  repoUrl: string;
  password?: string; // Only for owner, hex encoded
  localShare?: string; // Hex encoded share
  shareIndex?: number;
  peerShare?: string; // Hex encoded share to give to peer
  peerShareIndex?: number;
}

export interface PendingRequest {
  id: string;
  requester: string;
  snapshotId: string;
  reason: string;
  status: "pending" | "approved" | "denied" | "expired";
  createdAt: string;
  expiresAt: string;
}

export type Step = "welcome" | "init" | "join" | "dashboard";
