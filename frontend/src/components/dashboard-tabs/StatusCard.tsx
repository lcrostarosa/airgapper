import type { VaultConfig } from "../../types";

interface StatusCardProps {
  config: VaultConfig;
  isConsensusMode: boolean;
  isSSSMode: boolean;
}

export function StatusCard({ config, isConsensusMode, isSSSMode }: StatusCardProps) {
  return (
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
  );
}
