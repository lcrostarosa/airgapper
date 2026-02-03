import type { VaultConfig, VaultContract } from "../../types";
import type { InitHostResponse } from "../../lib/client";
import type { HostSetupState } from "./types";

export function calculateQuotaBytes(quota: string, unit: "GB" | "TB"): number | undefined {
  if (!quota) return undefined;
  const quotaNum = parseFloat(quota);
  if (isNaN(quotaNum) || quotaNum <= 0) return undefined;
  return unit === "TB"
    ? quotaNum * 1024 * 1024 * 1024 * 1024
    : quotaNum * 1024 * 1024 * 1024;
}

export function buildVaultConfig(
  state: HostSetupState,
  initResult: InitHostResponse
): VaultConfig {
  const quotaBytes = calculateQuotaBytes(state.storageQuota, state.storageQuotaUnit);

  const contract: VaultContract = {
    version: 1,
    createdAt: new Date().toISOString(),
    storageQuotaBytes: quotaBytes,
    appendOnly: state.appendOnly,
    retentionDays: state.retentionDays ? parseInt(state.retentionDays) : 30,
    deletionMode: state.deletionMode,
    restoreApproval: state.restoreApproval,
    hostKeyId: initResult.keyId,
  };

  return {
    name: state.name.trim(),
    role: "host",
    repoUrl: initResult.storageUrl,
    publicKey: initResult.publicKey,
    keyId: initResult.keyId,
    storagePath: state.storagePath || undefined,
    storageQuotaBytes: quotaBytes,
    contract,
  };
}

export const SETUP_STEPS = [
  { key: "intro", label: "Welcome" },
  { key: "storage", label: "Storage" },
  { key: "terms", label: "Terms" },
  { key: "initialize", label: "Setup" },
  { key: "complete", label: "Done" },
] as const;

export const INITIAL_STATE: HostSetupState = {
  storagePath: "",
  storageQuota: "",
  storageQuotaUnit: "GB",
  appendOnly: true,
  restoreApproval: "both-required",
  deletionMode: "both-required",
  retentionDays: "30",
  localIP: "192.168.1.x",
  name: "",
};
