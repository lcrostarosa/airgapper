import { Alert } from "../ui";
import type { VaultConfig } from "../../types";

interface KeyHoldersCardProps {
  config: VaultConfig;
}

export function KeyHoldersCard({ config }: KeyHoldersCardProps) {
  if (!config.consensus) return null;

  return (
    <div className="bg-gray-800 rounded-lg p-6 mb-6">
      <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
        <span>👥</span> Key Holders
      </h2>

      <div className="space-y-3">
        {config.consensus.keyHolders.map((holder) => (
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

        {config.consensus.keyHolders.length < config.consensus.totalKeys && (
          <Alert variant="warning">
            {config.consensus.totalKeys - config.consensus.keyHolders.length}{" "}
            more key holder(s) needed
          </Alert>
        )}
      </div>
    </div>
  );
}
