import { useState } from "react";
import type { VaultConfig, Step, PendingRequest } from "../types";
import { fromHex, combine, toHex } from "../lib/sss";
import { useClipboard } from "../hooks/useClipboard";
import { Alert, CopyableField } from "./ui";

interface DashboardProps {
  config: VaultConfig;
  onNavigate?: (step: Step) => void;
  onClear: () => void;
  onUpdateConfig?: (config: VaultConfig) => void;
}

export function Dashboard({ config, onClear, onUpdateConfig }: DashboardProps) {
  const [showShare, setShowShare] = useState(false);
  const [testShare, setTestShare] = useState("");
  const [testResult, setTestResult] = useState<string | null>(null);
  const { copiedId, copy } = useClipboard();
  const [newPath, setNewPath] = useState("");
  const [activeTab, setActiveTab] = useState<"status" | "requests" | "paths">(
    "status"
  );

  // For simulating pending requests (in real app, these come from API)
  const [pendingRequests] = useState<PendingRequest[]>([]);

  const isConsensusMode = !!config.consensus;
  const isSSSMode = !!config.localShare && !config.consensus;

  const testReconstruct = () => {
    if (!testShare.trim() || !config.localShare) {
      setTestResult("Enter both shares to test");
      return;
    }

    try {
      const share1 = {
        index: config.shareIndex!,
        data: fromHex(config.localShare),
      };
      const share2 = {
        index: config.shareIndex === 1 ? 2 : 1,
        data: fromHex(testShare.trim()),
      };

      const reconstructed = combine([share1, share2]);
      const reconstructedHex = toHex(reconstructed);

      if (config.password && reconstructedHex === config.password) {
        setTestResult("Success! Password reconstructed correctly.");
      } else if (config.password) {
        setTestResult("Mismatch! The reconstructed password doesn't match.");
      } else {
        setTestResult(`Reconstructed: ${reconstructedHex}`);
      }
    } catch (error) {
      setTestResult("Error: " + (error as Error).message);
    }
  };

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

      {/* Status Tab */}
      {activeTab === "status" && (
        <>
          {/* Status Card */}
          <div className="bg-gray-800 rounded-lg p-6 mb-6">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <span>📊</span> Status
            </h2>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div className="bg-gray-700 rounded p-3">
                <div className="text-sm text-gray-400">Role</div>
                <div className="font-medium capitalize">{config.role}</div>
              </div>
              <div className="bg-gray-700 rounded p-3">
                <div className="text-sm text-gray-400">Mode</div>
                <div className="font-medium">
                  {isConsensusMode ? "Consensus" : isSSSMode ? "SSS" : "—"}
                </div>
              </div>
              {isConsensusMode && (
                <>
                  <div className="bg-gray-700 rounded p-3">
                    <div className="text-sm text-gray-400">Threshold</div>
                    <div className="font-medium">
                      {config.consensus!.threshold}/{config.consensus!.totalKeys}
                    </div>
                  </div>
                  <div className="bg-gray-700 rounded p-3">
                    <div className="text-sm text-gray-400">Key Holders</div>
                    <div className="font-medium">
                      {config.consensus!.keyHolders.length}/
                      {config.consensus!.totalKeys}
                    </div>
                  </div>
                </>
              )}
              {isSSSMode && (
                <>
                  <div className="bg-gray-700 rounded p-3">
                    <div className="text-sm text-gray-400">Share Index</div>
                    <div className="font-medium">{config.shareIndex}</div>
                  </div>
                  <div className="bg-gray-700 rounded p-3">
                    <div className="text-sm text-gray-400">Share</div>
                    <div className="font-medium text-green-400">Stored</div>
                  </div>
                </>
              )}
              <div className="bg-gray-700 rounded p-3">
                <div className="text-sm text-gray-400">Password</div>
                <div className="font-medium">
                  {config.password ? (
                    <span className="text-green-400">Available</span>
                  ) : (
                    <span className="text-gray-500">— (host)</span>
                  )}
                </div>
              </div>
            </div>

            <div className="mt-4">
              <div className="text-sm text-gray-400 mb-1">Repository</div>
              <code className="block bg-gray-900 rounded px-3 py-2 font-mono text-sm break-all">
                {config.repoUrl}
              </code>
            </div>
          </div>

          {/* Key Holders Card (Consensus Mode) */}
          {isConsensusMode && (
            <div className="bg-gray-800 rounded-lg p-6 mb-6">
              <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
                <span>👥</span> Key Holders
              </h2>

              <div className="space-y-3">
                {config.consensus!.keyHolders.map((holder) => (
                  <div
                    key={holder.id}
                    className="flex items-center justify-between bg-gray-700 rounded p-3"
                  >
                    <div className="flex items-center gap-3">
                      <span className="text-xl">
                        {holder.isOwner ? "👑" : "🔑"}
                      </span>
                      <div>
                        <div className="font-medium">{holder.name}</div>
                        <div className="text-xs text-gray-400 font-mono">
                          {holder.id}
                        </div>
                      </div>
                    </div>
                    {holder.isOwner && (
                      <span className="text-xs text-blue-400 bg-blue-900/50 px-2 py-1 rounded">
                        Owner
                      </span>
                    )}
                  </div>
                ))}

                {config.consensus!.keyHolders.length <
                  config.consensus!.totalKeys && (
                  <Alert variant="warning">
                    {config.consensus!.totalKeys -
                      config.consensus!.keyHolders.length}{" "}
                    more key holder(s) needed
                  </Alert>
                )}
              </div>
            </div>
          )}

          {/* Your Key Card */}
          <div className="bg-gray-800 rounded-lg p-6 mb-6">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <span>🔑</span> Your Key
            </h2>

            {isConsensusMode && config.keyId && (
              <CopyableField
                label="Key ID"
                value={config.keyId}
                id="keyId"
                externalCopied={copiedId === "keyId"}
                onCopy={() => copy(config.keyId!, "keyId")}
                className="mb-4"
              />
            )}

            {isSSSMode && (
              <div className="mb-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm text-gray-400">
                    Share {config.shareIndex}
                  </span>
                  <button
                    onClick={() => setShowShare(!showShare)}
                    className="text-sm text-blue-400 hover:text-blue-300 transition-colors"
                  >
                    {showShare ? "Hide" : "Show"}
                  </button>
                </div>
                {showShare ? (
                  <CopyableField
                    value={config.localShare!}
                    id="localShare"
                    externalCopied={copiedId === "localShare"}
                    onCopy={() => copy(config.localShare!, "localShare")}
                  />
                ) : (
                  <div className="bg-gray-900 rounded px-3 py-2 font-mono text-sm text-gray-500">
                    ••••••••••••••••••••••••••••••••
                  </div>
                )}
              </div>
            )}

            {config.role === "owner" && isSSSMode && config.peerShare && (
              <div className="pt-4 border-t border-gray-700">
                <div className="text-sm text-yellow-400 mb-2">
                  Peer share (give to your backup host):
                </div>
                <code className="block bg-gray-900 rounded px-3 py-2 font-mono text-xs break-all">
                  {config.peerShare}
                </code>
              </div>
            )}
          </div>

          {/* Test Reconstruction (SSS Owner only) */}
          {config.role === "owner" && isSSSMode && (
            <div className="bg-gray-800 rounded-lg p-6 mb-6">
              <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
                <span>🧪</span> Test Key Reconstruction
              </h2>
              <p className="text-sm text-gray-400 mb-4">
                Verify that the shares can reconstruct the password correctly.
              </p>

              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium mb-2">
                    Peer's Share (Index {config.shareIndex === 1 ? 2 : 1})
                  </label>
                  <input
                    type="text"
                    value={testShare}
                    onChange={(e) => setTestShare(e.target.value)}
                    placeholder="Paste the peer's share to test reconstruction"
                    className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono text-sm"
                  />
                </div>

                <button
                  onClick={testReconstruct}
                  disabled={!testShare.trim()}
                  className="bg-purple-600 hover:bg-purple-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-2 px-4 rounded-lg transition-colors"
                >
                  Test Reconstruction
                </button>

                {testResult && (
                  <Alert
                    variant={
                      testResult.startsWith("Success")
                        ? "success"
                        : testResult.startsWith("Error") || testResult.startsWith("Mismatch")
                        ? "error"
                        : "info"
                    }
                  >
                    {testResult}
                  </Alert>
                )}
              </div>
            </div>
          )}

          {/* CLI Commands */}
          <div className="bg-gray-800 rounded-lg p-6">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <span>💻</span> CLI Commands
            </h2>

            <div className="space-y-3 text-sm">
              {config.role === "owner" ? (
                <>
                  <div>
                    <div className="text-gray-400 mb-1">Create backup:</div>
                    <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                      airgapper backup ~/Documents
                    </code>
                  </div>
                  <div>
                    <div className="text-gray-400 mb-1">List snapshots:</div>
                    <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                      airgapper snapshots
                    </code>
                  </div>
                  <div>
                    <div className="text-gray-400 mb-1">Request restore:</div>
                    <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                      airgapper request --snapshot latest --reason "need files"
                    </code>
                  </div>
                </>
              ) : (
                <>
                  <div>
                    <div className="text-gray-400 mb-1">
                      View pending requests:
                    </div>
                    <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                      airgapper pending
                    </code>
                  </div>
                  <div>
                    <div className="text-gray-400 mb-1">Approve a request:</div>
                    <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                      airgapper approve {"<request-id>"}
                    </code>
                  </div>
                  <div>
                    <div className="text-gray-400 mb-1">Start API server:</div>
                    <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                      airgapper serve --addr :8080
                    </code>
                  </div>
                </>
              )}
            </div>
          </div>
        </>
      )}

      {/* Requests Tab */}
      {activeTab === "requests" && (
        <div className="bg-gray-800 rounded-lg p-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <span>📋</span> Restore Requests
          </h2>

          {pendingRequests.length === 0 ? (
            <div className="text-center py-8 text-gray-400">
              <div className="text-4xl mb-4">📭</div>
              <p>No pending restore requests</p>
              {config.role === "owner" && (
                <p className="text-sm mt-2">
                  Create a request with:{" "}
                  <code className="bg-gray-900 px-2 py-1 rounded">
                    airgapper request
                  </code>
                </p>
              )}
            </div>
          ) : (
            <div className="space-y-4">
              {pendingRequests.map((request) => (
                <div
                  key={request.id}
                  className="bg-gray-700 rounded-lg p-4"
                >
                  <div className="flex items-start justify-between mb-3">
                    <div>
                      <div className="font-medium">{request.reason}</div>
                      <div className="text-sm text-gray-400">
                        From: {request.requester}
                      </div>
                    </div>
                    <span
                      className={`text-xs px-2 py-1 rounded ${
                        request.status === "pending"
                          ? "bg-yellow-900/50 text-yellow-400"
                          : request.status === "approved"
                          ? "bg-green-900/50 text-green-400"
                          : "bg-red-900/50 text-red-400"
                      }`}
                    >
                      {request.status}
                    </span>
                  </div>

                  {/* Approval progress for consensus mode */}
                  {request.requiredApprovals && (
                    <div className="mb-3">
                      <div className="text-sm text-gray-400 mb-1">
                        Approvals: {(request.approvals || []).length}/
                        {request.requiredApprovals}
                      </div>
                      <div className="h-2 bg-gray-600 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-blue-500 transition-all"
                          style={{
                            width: `${
                              ((request.approvals || []).length /
                                request.requiredApprovals) *
                              100
                            }%`,
                          }}
                        />
                      </div>
                    </div>
                  )}

                  <div className="text-xs text-gray-500">
                    ID: {request.id} | Expires:{" "}
                    {new Date(request.expiresAt).toLocaleString()}
                  </div>

                  {request.status === "pending" && config.role === "host" && (
                    <div className="mt-3 flex gap-2">
                      <button className="flex-1 bg-green-600 hover:bg-green-700 text-white py-2 rounded transition-colors">
                        Approve
                      </button>
                      <button className="flex-1 bg-red-600 hover:bg-red-700 text-white py-2 rounded transition-colors">
                        Deny
                      </button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Backup Paths Tab */}
      {activeTab === "paths" && config.role === "owner" && (
        <div className="bg-gray-800 rounded-lg p-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <span>📁</span> Backup Paths
          </h2>
          <p className="text-sm text-gray-400 mb-4">
            Configure which folders to include in backups.
          </p>

          {/* Current paths */}
          {config.backupPaths && config.backupPaths.length > 0 ? (
            <div className="space-y-2 mb-4">
              {config.backupPaths.map((path) => (
                <div
                  key={path}
                  className="flex items-center justify-between bg-gray-700 rounded px-3 py-2"
                >
                  <span className="font-mono text-sm truncate">{path}</span>
                  <button
                    onClick={() => handleRemovePath(path)}
                    className="text-gray-400 hover:text-red-400 ml-2"
                  >
                    &times;
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-8 text-gray-400 bg-gray-700/50 rounded-lg mb-4">
              <div className="text-4xl mb-2">📂</div>
              <p>No backup paths configured</p>
            </div>
          )}

          <div className="flex gap-2">
            <input
              type="text"
              value={newPath}
              onChange={(e) => setNewPath(e.target.value)}
              placeholder="/path/to/backup"
              className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono text-sm"
              onKeyDown={(e) => {
                if (e.key === "Enter" && newPath.trim()) {
                  handleAddPaths([newPath.trim()]);
                  setNewPath("");
                }
              }}
            />
            <button
              onClick={() => {
                if (newPath.trim()) {
                  handleAddPaths([newPath.trim()]);
                  setNewPath("");
                }
              }}
              disabled={!newPath.trim()}
              className="px-4 py-3 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-800 disabled:cursor-not-allowed rounded-lg transition-colors"
            >
              Add
            </button>
          </div>

          {config.backupPaths && config.backupPaths.length > 0 && (
            <div className="mt-4 pt-4 border-t border-gray-700">
              <div className="text-sm text-gray-400 mb-2">Run backup now:</div>
              <code className="block bg-gray-900 rounded px-3 py-2 font-mono text-sm">
                airgapper backup {config.backupPaths.join(" ")}
              </code>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
