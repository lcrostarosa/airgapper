import type { VaultConfig } from "../../types";

interface CLICommandsCardProps {
  config: VaultConfig;
}

export function CLICommandsCard({ config }: CLICommandsCardProps) {
  return (
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
              <div className="text-gray-400 mb-1">View pending requests:</div>
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
  );
}
