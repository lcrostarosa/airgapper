import { StatusCard } from "./StatusCard";
import { KeyHoldersCard } from "./KeyHoldersCard";
import { YourKeyCard } from "./YourKeyCard";
import { TestReconstructionCard } from "./TestReconstructionCard";
import { CLICommandsCard } from "./CLICommandsCard";
import type { OverviewTabProps } from "./types";

export function DashboardOverview({ config, copiedId, copy }: OverviewTabProps) {
  const isConsensusMode = !!config.consensus;
  const isSSSMode = !!config.localShare && !config.consensus;

  return (
    <>
      <StatusCard
        config={config}
        isConsensusMode={isConsensusMode}
        isSSSMode={isSSSMode}
      />

      {isConsensusMode && <KeyHoldersCard config={config} />}

      <YourKeyCard
        config={config}
        isConsensusMode={isConsensusMode}
        isSSSMode={isSSSMode}
        copiedId={copiedId}
        copy={copy}
      />

      {config.role === "owner" && isSSSMode && (
        <TestReconstructionCard config={config} />
      )}

      <CLICommandsCard config={config} />
    </>
  );
}
