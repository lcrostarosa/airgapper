import type { VaultConfig, Step } from "../../types";

export type JoinMode = "sss" | "consensus";

export interface GeneratedKeys {
  publicKey: string;
  privateKey: string;
  keyId: string;
}

export interface CommonFormData {
  name: string;
  repoUrl: string;
}

export interface SSSFormData extends CommonFormData {
  shareHex: string;
  shareIndex: string;
}

export interface JoinVaultResultProps {
  name: string;
  repoUrl: string;
  generatedKeys: GeneratedKeys;
  onBack: () => void;
  onConfirm: () => void;
}

export interface JoinVaultSSSProps {
  formData: SSSFormData;
  error: string;
  onFieldChange: (field: keyof SSSFormData, value: string) => void;
  onSubmit: () => void;
}

export interface JoinVaultConsensusProps {
  formData: CommonFormData;
  error: string;
  isGenerating: boolean;
  onFieldChange: (field: keyof CommonFormData, value: string) => void;
  onSubmit: () => void;
}

export interface JoinVaultProps {
  onComplete: (config: VaultConfig) => void;
  onNavigate: (step: Step) => void;
}
