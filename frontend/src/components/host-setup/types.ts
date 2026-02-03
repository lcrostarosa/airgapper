import type { Step } from "../../types";
import type { InitHostResponse } from "../../lib/client";

export type SetupStep = "intro" | "storage" | "terms" | "initialize" | "complete";

export type RestoreApprovalMode = "owner-only" | "host-only" | "both-required" | "either";
export type DeletionMode = "both-required" | "owner-only" | "time-lock-only" | "never";

export interface HostSetupState {
  storagePath: string;
  storageQuota: string;
  storageQuotaUnit: "GB" | "TB";
  appendOnly: boolean;
  restoreApproval: RestoreApprovalMode;
  deletionMode: DeletionMode;
  retentionDays: string;
  localIP: string;
  name: string;
}

export interface StepProps {
  state: HostSetupState;
  updateState: <K extends keyof HostSetupState>(key: K, value: HostSetupState[K]) => void;
  onNext: (step: SetupStep) => void;
  onBack: (step: SetupStep) => void;
  renderStepIndicator: () => React.ReactNode;
}

export interface IntroStepProps extends StepProps {
  onNavigate: (step: Step) => void;
}

export interface InitializeStepProps extends StepProps {
  error: string;
  isInitializing: boolean;
  onInitialize: () => void;
}

export interface CompleteStepProps extends StepProps {
  initResult: InitHostResponse;
  onComplete: () => void;
  onStartOver: () => void;
}
