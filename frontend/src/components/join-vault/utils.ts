import type { VaultConfig } from "../../types";
import { fromHex } from "../../lib/sss";
import type { SSSFormData, GeneratedKeys } from "./types";

export interface ValidationResult {
  valid: boolean;
  error?: string;
}

export function validateCommonFields(name: string, repoUrl: string): ValidationResult {
  if (!name.trim()) {
    return { valid: false, error: "Name is required" };
  }
  if (!repoUrl.trim()) {
    return { valid: false, error: "Repository URL is required" };
  }
  return { valid: true };
}

export function validateSSSFields(formData: SSSFormData): ValidationResult {
  const common = validateCommonFields(formData.name, formData.repoUrl);
  if (!common.valid) return common;

  if (!formData.shareHex.trim()) {
    return { valid: false, error: "Share is required" };
  }
  try {
    const share = fromHex(formData.shareHex.trim());
    if (share.length !== 32) {
      return { valid: false, error: "Invalid share (expected 64 hex characters)" };
    }
  } catch {
    return { valid: false, error: "Invalid share (must be hex encoded)" };
  }
  return { valid: true };
}

export function buildSSSConfig(formData: SSSFormData): VaultConfig {
  return {
    name: formData.name.trim(),
    role: "host",
    repoUrl: formData.repoUrl.trim(),
    localShare: formData.shareHex.trim(),
    shareIndex: parseInt(formData.shareIndex),
  };
}

export function buildConsensusConfig(
  formData: SSSFormData,
  generatedKeys: GeneratedKeys
): VaultConfig {
  return {
    name: formData.name.trim(),
    role: "host",
    repoUrl: formData.repoUrl.trim(),
    publicKey: generatedKeys.publicKey,
    privateKey: generatedKeys.privateKey,
    keyId: generatedKeys.keyId,
  };
}
