/**
 * API client for Airgapper backend
 */

const API_BASE = import.meta.env.VITE_API_URL || "http://localhost:8081";

export interface KeyHolder {
  id: string;
  name: string;
  publicKey: string;
  address?: string;
  joinedAt: string;
  isOwner: boolean;
}

export interface ConsensusInfo {
  threshold: number;
  totalKeys: number;
  keyHolders: KeyHolder[];
  requireApproval: boolean;
}

export interface StatusResponse {
  name: string;
  role: "owner" | "host";
  repo_url: string;
  has_share: boolean;
  share_index: number;
  pending_requests: number;
  backup_paths?: string[];
  mode: "consensus" | "sss" | "none";
  consensus?: ConsensusInfo;
  peer?: {
    name: string;
    address: string;
  };
  scheduler?: {
    enabled: boolean;
    schedule: string;
    paths: string[];
    last_run?: string;
    next_run?: string;
    last_error?: string;
  };
}

export interface RestoreRequest {
  id: string;
  requester: string;
  snapshot_id: string;
  paths: string[];
  reason: string;
  status: "pending" | "approved" | "denied" | "expired";
  created_at: string;
  expires_at: string;
  approved_at?: string;
  approved_by?: string;
  required_approvals?: number;
  approvals?: {
    key_holder_id: string;
    key_holder_name?: string;
    approved_at: string;
  }[];
}

export interface VaultInitParams {
  name: string;
  repoUrl: string;
  threshold: number;
  totalKeys: number;
  backupPaths?: string[];
  requireApproval?: boolean;
}

export interface VaultInitResponse {
  name: string;
  keyId: string;
  publicKey: string;
  threshold: number;
  totalKeys: number;
}

export interface RegisterKeyHolderParams {
  name: string;
  publicKey: string;
  address?: string;
}

export interface SignRequestParams {
  keyHolderId: string;
  signature: string;
}

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

async function fetchApi<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  const result: ApiResponse<T> = await response.json();

  if (!result.success) {
    throw new Error(result.error || "API request failed");
  }

  return result.data as T;
}

/**
 * Get system status
 */
export async function getStatus(): Promise<StatusResponse> {
  return fetchApi<StatusResponse>("/api/status");
}

/**
 * List pending restore requests
 */
export async function listRequests(): Promise<RestoreRequest[]> {
  return fetchApi<RestoreRequest[]>("/api/requests");
}

/**
 * Get a specific restore request
 */
export async function getRequest(id: string): Promise<RestoreRequest> {
  return fetchApi<RestoreRequest>(`/api/requests/${id}`);
}

/**
 * Create a restore request
 */
export async function createRequest(
  snapshotId: string,
  reason: string,
  paths?: string[]
): Promise<{ id: string; status: string; expires_at: string }> {
  return fetchApi(`/api/requests`, {
    method: "POST",
    body: JSON.stringify({
      snapshot_id: snapshotId,
      reason,
      paths,
    }),
  });
}

/**
 * Approve a restore request (legacy SSS mode)
 */
export async function approveRequest(
  id: string
): Promise<{ status: string; message: string }> {
  return fetchApi(`/api/requests/${id}/approve`, {
    method: "POST",
    body: JSON.stringify({}),
  });
}

/**
 * Sign a restore request (consensus mode)
 */
export async function signRequest(
  id: string,
  params: SignRequestParams
): Promise<{
  status: string;
  currentApprovals: number;
  requiredApprovals: number;
  isApproved: boolean;
}> {
  return fetchApi(`/api/requests/${id}/sign`, {
    method: "POST",
    body: JSON.stringify(params),
  });
}

/**
 * Deny a restore request
 */
export async function denyRequest(
  id: string
): Promise<{ status: string }> {
  return fetchApi(`/api/requests/${id}/deny`, {
    method: "POST",
  });
}

/**
 * Initialize a new vault
 */
export async function initVault(
  params: VaultInitParams
): Promise<VaultInitResponse> {
  return fetchApi(`/api/vault/init`, {
    method: "POST",
    body: JSON.stringify(params),
  });
}

/**
 * List key holders
 */
export async function listKeyHolders(): Promise<ConsensusInfo> {
  return fetchApi<ConsensusInfo>("/api/keyholders");
}

/**
 * Register a new key holder
 */
export async function registerKeyHolder(
  params: RegisterKeyHolderParams
): Promise<{ id: string; name: string; joinedAt: string }> {
  return fetchApi(`/api/keyholders`, {
    method: "POST",
    body: JSON.stringify(params),
  });
}

/**
 * Get a specific key holder
 */
export async function getKeyHolder(id: string): Promise<KeyHolder> {
  return fetchApi<KeyHolder>(`/api/keyholders/${id}`);
}

/**
 * Update backup schedule
 */
export async function updateSchedule(
  schedule: string,
  paths: string[]
): Promise<{ status: string; message: string }> {
  return fetchApi(`/api/schedule`, {
    method: "POST",
    body: JSON.stringify({ schedule, paths }),
  });
}

/**
 * Get schedule info
 */
export async function getSchedule(): Promise<{
  schedule: string;
  paths: string[];
  enabled: boolean;
  last_run?: string;
  next_run?: string;
  last_error?: string;
}> {
  return fetchApi(`/api/schedule`);
}

/**
 * Get local IP address (best guess from server)
 */
export async function getLocalIP(): Promise<string> {
  const result = await fetchApi<{ ip: string }>("/api/network/local-ip");
  return result.ip;
}

// Storage server management

export interface StorageStatus {
  configured: boolean;
  running: boolean;
  startTime?: string;
  basePath?: string;
  appendOnly?: boolean;
  quotaBytes?: number;
  usedBytes?: number;
  requestCount?: number;
}

/**
 * Get storage server status
 */
export async function getStorageStatus(): Promise<StorageStatus> {
  return fetchApi<StorageStatus>("/api/storage/status");
}

/**
 * Start the storage server
 */
export async function startStorage(): Promise<{ status: string }> {
  return fetchApi("/api/storage/start", { method: "POST" });
}

/**
 * Stop the storage server
 */
export async function stopStorage(): Promise<{ status: string }> {
  return fetchApi("/api/storage/stop", { method: "POST" });
}

// Host initialization

export interface HostInitParams {
  name: string;
  storagePath: string;
  storageQuotaBytes?: number;
  appendOnly: boolean;
  restoreApproval: "both-required" | "either" | "owner-only" | "host-only";
  retentionDays?: number;
}

export interface HostInitResponse {
  name: string;
  keyId: string;
  publicKey: string;
  storageUrl: string;
  storagePath: string;
}

/**
 * Initialize as a backup host
 */
export async function initHost(
  params: HostInitParams
): Promise<HostInitResponse> {
  return fetchApi("/api/host/init", {
    method: "POST",
    body: JSON.stringify(params),
  });
}

// ============================================================================
// Policy Management
// ============================================================================

export type DeletionMode = "both-required" | "owner-only" | "time-lock-only" | "never";

export interface Policy {
  id: string;
  version: number;
  name?: string;
  ownerName: string;
  ownerKeyId: string;
  ownerPublicKey: string;
  hostName: string;
  hostKeyId: string;
  hostPublicKey: string;
  retentionDays: number;
  deletionMode: DeletionMode;
  appendOnlyLocked: boolean;
  maxStorageBytes?: number;
  createdAt: string;
  effectiveAt: string;
  expiresAt?: string;
  ownerSignature?: string;
  hostSignature?: string;
}

export interface PolicyResponse {
  hasPolicy: boolean;
  policy?: Policy;
  isFullySigned?: boolean;
  isActive?: boolean;
}

export interface CreatePolicyParams {
  ownerName: string;
  ownerKeyId: string;
  ownerPublicKey: string;
  hostName: string;
  hostKeyId: string;
  hostPublicKey: string;
  retentionDays?: number;
  deletionMode?: DeletionMode;
  maxStorageBytes?: number;
  ownerSignature?: string;
  hostSignature?: string;
}

export interface PolicySignParams {
  policyJson: string;
  signature: string;
  signerRole: "owner" | "host";
}

/**
 * Get the current policy
 */
export async function getPolicy(): Promise<PolicyResponse> {
  return fetchApi<PolicyResponse>("/api/policy");
}

/**
 * Create a new policy
 */
export async function createPolicy(
  params: CreatePolicyParams
): Promise<{ policy: Policy; policyJSON: string; isFullySigned: boolean }> {
  return fetchApi("/api/policy", {
    method: "POST",
    body: JSON.stringify(params),
  });
}

/**
 * Sign a policy
 */
export async function signPolicy(
  params: PolicySignParams
): Promise<{ policy: Policy; policyJSON: string; isFullySigned: boolean }> {
  return fetchApi("/api/policy/sign", {
    method: "POST",
    body: JSON.stringify(params),
  });
}

// ============================================================================
// Deletion Requests
// ============================================================================

export type DeletionType = "snapshot" | "path" | "prune" | "all";

export interface DeletionRequest {
  id: string;
  requester: string;
  deletionType: DeletionType;
  snapshotIds?: string[];
  paths?: string[];
  reason: string;
  status: "pending" | "approved" | "denied" | "expired";
  createdAt: string;
  expiresAt: string;
  approvedAt?: string;
  executedAt?: string;
  requiredApprovals: number;
  currentApprovals?: number;
  approvals?: {
    keyHolderId: string;
    keyHolderName?: string;
    approvedAt: string;
  }[];
}

export interface CreateDeletionParams {
  deletionType: DeletionType;
  snapshotIds?: string[];
  paths?: string[];
  reason: string;
  requiredApprovals?: number;
}

/**
 * List pending deletion requests
 */
export async function listDeletions(): Promise<DeletionRequest[]> {
  return fetchApi<DeletionRequest[]>("/api/deletions");
}

/**
 * Get a specific deletion request
 */
export async function getDeletion(id: string): Promise<DeletionRequest> {
  return fetchApi<DeletionRequest>(`/api/deletions/${id}`);
}

/**
 * Create a deletion request
 */
export async function createDeletion(
  params: CreateDeletionParams
): Promise<{ id: string; status: string; expiresAt: string }> {
  return fetchApi("/api/deletions", {
    method: "POST",
    body: JSON.stringify(params),
  });
}

/**
 * Approve a deletion request
 */
export async function approveDeletion(
  id: string,
  keyHolderId: string,
  signature: string
): Promise<{
  status: string;
  currentApprovals: number;
  requiredApprovals: number;
  isApproved: boolean;
}> {
  return fetchApi(`/api/deletions/${id}/approve`, {
    method: "POST",
    body: JSON.stringify({ keyHolderId, signature }),
  });
}

/**
 * Deny a deletion request
 */
export async function denyDeletion(
  id: string
): Promise<{ status: string }> {
  return fetchApi(`/api/deletions/${id}/deny`, {
    method: "POST",
  });
}

// ============================================================================
// Integrity Check
// ============================================================================

export interface IntegrityCheckResponse {
  status: string;
  storageRunning: boolean;
  usedBytes: number;
  diskUsagePct: number;
  diskFreeBytes: number;
  hasPolicy: boolean;
  policyId?: string;
  requestCount: number;
}

/**
 * Run an integrity check
 */
export async function checkIntegrity(): Promise<IntegrityCheckResponse> {
  return fetchApi<IntegrityCheckResponse>("/api/integrity/check");
}

// Extended storage status with disk info
export interface StorageStatusExtended extends StorageStatus {
  hasPolicy?: boolean;
  policyId?: string;
  maxDiskUsagePct?: number;
  diskUsagePct?: number;
  diskFreeBytes?: number;
  diskTotalBytes?: number;
}

/**
 * Get extended storage server status
 */
export async function getStorageStatusExtended(): Promise<StorageStatusExtended> {
  return fetchApi<StorageStatusExtended>("/api/storage/status");
}
