/**
 * Connect-RPC client for Airgapper backend
 *
 * This client uses the Connect protocol which supports:
 * - HTTP/1.1 and HTTP/2
 * - JSON and binary (Protobuf) formats
 * - Browser-compatible (no proxy required)
 */

import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

// Import generated service definitions
import { HealthService } from "../gen/airgapper/v1/health_pb";
import { VaultService } from "../gen/airgapper/v1/vault_pb";
import { HostService } from "../gen/airgapper/v1/host_pb";
import { RestoreRequestService } from "../gen/airgapper/v1/requests_pb";
import { DeletionService } from "../gen/airgapper/v1/deletions_pb";
import { KeyHolderService } from "../gen/airgapper/v1/keyholders_pb";
import { ScheduleService } from "../gen/airgapper/v1/schedule_pb";
import { StorageService } from "../gen/airgapper/v1/storage_pb";
import { PolicyService } from "../gen/airgapper/v1/policy_pb";
import { IntegrityService } from "../gen/airgapper/v1/integrity_pb";
import { VerificationService } from "../gen/airgapper/v1/verification_pb";
import { NetworkService } from "../gen/airgapper/v1/network_pb";

// Re-export types for convenience
export type {
  CheckRequest,
  CheckResponse,
  GetStatusRequest,
  GetStatusResponse,
  SchedulerInfo,
} from "../gen/airgapper/v1/health_pb";

export type {
  Role,
  RequestStatus,
  DeletionType,
  DeletionMode,
  OperationMode,
  CheckType,
  KeyHolder,
  ConsensusInfo,
  Peer,
  Approval,
  ApprovalProgress,
} from "../gen/airgapper/v1/common_pb";

export type {
  InitVaultRequest,
  InitVaultResponse,
} from "../gen/airgapper/v1/vault_pb";

export type {
  InitHostRequest,
  InitHostResponse,
  ReceiveShareRequest,
  ReceiveShareResponse,
  Snapshot,
} from "../gen/airgapper/v1/host_pb";

export type {
  RestoreRequest,
  ListRequestsRequest,
  ListRequestsResponse,
  GetRequestRequest,
  GetRequestResponse,
  CreateRequestRequest,
  CreateRequestResponse,
  ApproveRequestRequest,
  ApproveRequestResponse,
  SignRequestRequest,
  SignRequestResponse,
  DenyRequestRequest,
  DenyRequestResponse,
} from "../gen/airgapper/v1/requests_pb";

export type {
  DeletionRequest,
  ListDeletionsRequest,
  ListDeletionsResponse,
} from "../gen/airgapper/v1/deletions_pb";

export type {
  GetStorageStatusResponse,
} from "../gen/airgapper/v1/storage_pb";

export type {
  Policy,
  GetPolicyResponse,
} from "../gen/airgapper/v1/policy_pb";

export type {
  IntegrityCheckResult,
  CheckIntegrityResponse,
} from "../gen/airgapper/v1/integrity_pb";

// API base URL from environment or default
const API_BASE = import.meta.env.VITE_API_URL || "http://localhost:8081";

// Create the Connect transport
const transport = createConnectTransport({
  baseUrl: API_BASE,
});

// ============================================================================
// Service Clients
// ============================================================================

/**
 * Health service client for health checks and status
 */
export const healthClient = createClient(HealthService, transport);

/**
 * Vault service client for vault initialization
 */
export const vaultClient = createClient(VaultService, transport);

/**
 * Host service client for host initialization and share exchange
 */
export const hostClient = createClient(HostService, transport);

/**
 * Restore request service client
 */
export const requestsClient = createClient(RestoreRequestService, transport);

/**
 * Deletion service client
 */
export const deletionsClient = createClient(DeletionService, transport);

/**
 * Key holder service client
 */
export const keyHoldersClient = createClient(KeyHolderService, transport);

/**
 * Schedule service client
 */
export const scheduleClient = createClient(ScheduleService, transport);

/**
 * Storage service client
 */
export const storageClient = createClient(StorageService, transport);

/**
 * Policy service client
 */
export const policyClient = createClient(PolicyService, transport);

/**
 * Integrity service client
 */
export const integrityClient = createClient(IntegrityService, transport);

/**
 * Verification service client
 */
export const verificationClient = createClient(VerificationService, transport);

/**
 * Network service client
 */
export const networkClient = createClient(NetworkService, transport);

// ============================================================================
// Convenience Functions (matching existing api.ts interface)
// ============================================================================

/**
 * Get system status
 */
export async function getStatus() {
  return healthClient.getStatus({});
}

/**
 * List pending restore requests
 */
export async function listRequests() {
  const response = await requestsClient.listRequests({});
  return response.requests;
}

/**
 * Get a specific restore request
 */
export async function getRequest(id: string) {
  const response = await requestsClient.getRequest({ id });
  return response.request;
}

/**
 * Create a restore request
 */
export async function createRequest(
  snapshotId: string,
  reason: string,
  paths?: string[]
) {
  return requestsClient.createRequest({
    snapshotId,
    reason,
    paths: paths || [],
  });
}

/**
 * Approve a restore request (legacy SSS mode)
 */
export async function approveRequest(id: string) {
  return requestsClient.approveRequest({ id });
}

/**
 * Sign a restore request (consensus mode)
 */
export async function signRequest(
  id: string,
  keyHolderId: string,
  signature: string
) {
  return requestsClient.signRequest({ id, keyHolderId, signature });
}

/**
 * Deny a restore request
 */
export async function denyRequest(id: string) {
  return requestsClient.denyRequest({ id });
}

/**
 * Initialize a new vault
 */
export async function initVault(params: {
  name: string;
  repoUrl: string;
  threshold: number;
  totalKeys: number;
  backupPaths?: string[];
  requireApproval?: boolean;
}) {
  return vaultClient.initVault(params);
}

/**
 * List key holders
 */
export async function listKeyHolders() {
  const response = await keyHoldersClient.listKeyHolders({});
  return response.consensus;
}

/**
 * Register a new key holder
 */
export async function registerKeyHolder(params: {
  name: string;
  publicKey: string;
  address?: string;
}) {
  return keyHoldersClient.registerKeyHolder(params);
}

/**
 * Get a specific key holder
 */
export async function getKeyHolder(id: string) {
  const response = await keyHoldersClient.getKeyHolder({ id });
  return response.keyHolder;
}

/**
 * Update backup schedule
 */
export async function updateSchedule(schedule: string, paths: string[]) {
  return scheduleClient.updateSchedule({ schedule, paths });
}

/**
 * Get schedule info
 */
export async function getSchedule() {
  return scheduleClient.getSchedule({});
}

/**
 * Get local IP address
 */
export async function getLocalIP() {
  const response = await networkClient.getLocalIP({});
  return response.ip;
}

/**
 * Get storage server status
 */
export async function getStorageStatus() {
  return storageClient.getStorageStatus({});
}

/**
 * Start the storage server
 */
export async function startStorage() {
  return storageClient.startStorage({});
}

/**
 * Stop the storage server
 */
export async function stopStorage() {
  return storageClient.stopStorage({});
}

/**
 * Initialize as a backup host
 */
export async function initHost(params: {
  name: string;
  storagePath: string;
  storageQuotaBytes?: bigint;
  appendOnly: boolean;
  restoreApproval: string;
  retentionDays?: number;
}) {
  return hostClient.initHost(params);
}

/**
 * Get the current policy
 */
export async function getPolicy() {
  return policyClient.getPolicy({});
}

/**
 * Create a new policy
 */
export async function createPolicy(params: {
  ownerName: string;
  ownerKeyId: string;
  ownerPublicKey: string;
  hostName: string;
  hostKeyId: string;
  hostPublicKey: string;
  retentionDays?: number;
  deletionMode?: number;
  maxStorageBytes?: bigint;
  ownerSignature?: string;
  hostSignature?: string;
}) {
  return policyClient.createPolicy(params);
}

/**
 * Sign a policy
 */
export async function signPolicy(params: {
  policyJson: string;
  signature: string;
  signerRole: string;
}) {
  return policyClient.signPolicy(params);
}

/**
 * List pending deletion requests
 */
export async function listDeletions() {
  const response = await deletionsClient.listDeletions({});
  return response.deletions;
}

/**
 * Get a specific deletion request
 */
export async function getDeletion(id: string) {
  const response = await deletionsClient.getDeletion({ id });
  return response.deletion;
}

/**
 * Create a deletion request
 */
export async function createDeletion(params: {
  deletionType: number;
  snapshotIds?: string[];
  paths?: string[];
  reason: string;
  requiredApprovals?: number;
}) {
  return deletionsClient.createDeletion(params);
}

/**
 * Approve a deletion request
 */
export async function approveDeletion(
  id: string,
  keyHolderId: string,
  signature: string
) {
  return deletionsClient.approveDeletion({ id, keyHolderId, signature });
}

/**
 * Deny a deletion request
 */
export async function denyDeletion(id: string) {
  return deletionsClient.denyDeletion({ id });
}

/**
 * Run an integrity check
 */
export async function checkIntegrity() {
  return integrityClient.checkIntegrity({});
}
