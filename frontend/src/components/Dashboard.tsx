import { useState } from "react";
import type { VaultConfig, Step, PendingRequest } from "../types";
import { useClipboard } from "../hooks/useClipboard";
import { DashboardOverview, DashboardRequests, DashboardPaths } from "./dashboard-tabs";
import type { DashboardTab } from "./dashboard-tabs";

interface DashboardProps {
  config: VaultConfig;
  onNavigate?: (step: Step) => void;
  onClear: () => void;
  onUpdateConfig?: (config: VaultConfig) => void;
}

export function Dashboard({ config, onClear, onUpdateConfig }: DashboardProps) {
  const { copiedId, copy } = useClipboard();
  const [activeTab, setActiveTab] = useState<DashboardTab>("status");

  // For simulating pending requests (in real app, these come from API)
  const [pendingRequests] = useState<PendingRequest[]>([]);

  const isConsensusMode = !!config.consensus;
  const isSSSMode = !!config.localShare && !config.consensus;

  const handleAddPaths = (paths: string[]) => {
    if (!onUpdateConfig) return;
    const currentPaths = config.backupPaths || [];
    const newPaths = [...currentPaths, ...paths.filter((p) => !currentPaths.includes(p))];
    onUpdateConfig({ ...config, backupPaths: newPaths });
  };

  const handleRemovePath = (path: string) => {
    if (!onUpdateConfig) return;
    const newPaths = (config.backupPaths || []).filter((p) => p !== path);
    onUpdateConfig({ ...config, backupPaths: newPaths });
  };

  return (
    <div className="max-w-3xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-3">
            <span className="text-3xl">
              {config.role === "owner" ? "👑" : "🤝"}
            </span>
            {config.name}
          </h1>
          <p className="text-gray-400">
            {config.role === "owner" ? "Data Owner" : "Key Holder"}
            {isConsensusMode && (
              <span className="ml-2 text-blue-400">
                ({config.consensus!.threshold}-of-{config.consensus!.totalKeys}{" "}
                consensus)
              </span>
            )}
            {isSSSMode && (
              <span className="ml-2 text-purple-400">(2-of-2 SSS)</span>
            )}
          </p>
        </div>
        <button
          onClick={onClear}
          className="text-sm text-gray-500 hover:text-red-400 transition-colors"
        >
          Reset Vault
        </button>
      </div>

      {/* Tab navigation */}
      <div className="flex gap-2 mb-6">
        <button
          onClick={() => setActiveTab("status")}
          className={`px-4 py-2 rounded-lg transition-colors ${
            activeTab === "status"
              ? "bg-gray-700 text-white"
              : "text-gray-400 hover:text-white"
          }`}
        >
          Status
        </button>
        <button
          onClick={() => setActiveTab("requests")}
          className={`px-4 py-2 rounded-lg transition-colors ${
            activeTab === "requests"
              ? "bg-gray-700 text-white"
              : "text-gray-400 hover:text-white"
          }`}
        >
          Requests
          {pendingRequests.length > 0 && (
            <span className="ml-2 bg-red-500 text-white text-xs px-2 py-0.5 rounded-full">
              {pendingRequests.length}
            </span>
          )}
        </button>
        {config.role === "owner" && (
          <button
            onClick={() => setActiveTab("paths")}
            className={`px-4 py-2 rounded-lg transition-colors ${
              activeTab === "paths"
                ? "bg-gray-700 text-white"
                : "text-gray-400 hover:text-white"
            }`}
          >
            Backup Paths
          </button>
        )}
      </div>

      {/* Tab content */}
      {activeTab === "status" && (
        <DashboardOverview config={config} copiedId={copiedId} copy={copy} />
      )}

      {activeTab === "requests" && (
        <DashboardRequests config={config} pendingRequests={pendingRequests} />
      )}

      {activeTab === "paths" && config.role === "owner" && (
        <DashboardPaths
          config={config}
          onAddPaths={handleAddPaths}
          onRemovePath={handleRemovePath}
        />
      )}
    </div>
  );
}
