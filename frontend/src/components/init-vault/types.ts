import type { Step, VaultConfig } from "../../types";

export type InitStep = "paths" | "destination" | "security" | "result";

export interface InitVaultState {
  name: string;
  repoUrl: string;
  threshold: number;
  totalKeys: number;
  requireApproval: boolean;
  useConsensusMode: boolean;
  backupPaths: string[];
  newPath: string;
}

export interface StepProps {
  state: InitVaultState;
  updateState: <K extends keyof InitVaultState>(key: K, value: InitVaultState[K]) => void;
  onNext: () => void;
  onBack: () => void;
  renderStepIndicator: () => React.ReactNode;
}

export interface PathsStepProps {
  state: InitVaultState;
  updateState: <K extends keyof InitVaultState>(key: K, value: InitVaultState[K]) => void;
  onNext: () => void;
  onNavigate: (step: Step) => void;
  addPath: (path: string) => void;
  removePath: (path: string) => void;
  renderStepIndicator: () => React.ReactNode;
}

export interface SecurityStepProps extends StepProps {
  isGenerating: boolean;
  onGenerate: () => void;
}

export interface CompleteStepProps {
  result: VaultConfig;
  copiedId: string | null;
  onCopy: (value: string, id: string) => void;
  onStartOver: () => void;
  onSave: () => void;
}
