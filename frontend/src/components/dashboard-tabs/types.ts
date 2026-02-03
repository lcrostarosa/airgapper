import type { VaultConfig, Step, PendingRequest } from "../../types";

export type DashboardTab = "status" | "requests" | "paths";

export interface DashboardProps {
  config: VaultConfig;
  onNavigate?: (step: Step) => void;
  onClear: () => void;
  onUpdateConfig?: (config: VaultConfig) => void;
}

export interface TabProps {
  config: VaultConfig;
}

export interface OverviewTabProps extends TabProps {
  copiedId: string | null;
  copy: (text: string, id: string) => void;
}

export interface RequestsTabProps extends TabProps {
  pendingRequests: PendingRequest[];
}

export interface PathsTabProps extends TabProps {
  onAddPaths: (paths: string[]) => void;
  onRemovePath: (path: string) => void;
}
