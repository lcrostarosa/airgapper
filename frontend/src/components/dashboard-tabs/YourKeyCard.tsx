import { useState } from "react";
import { CopyableField } from "../ui";
import type { VaultConfig } from "../../types";

interface YourKeyCardProps {
  config: VaultConfig;
  isConsensusMode: boolean;
  isSSSMode: boolean;
  copiedId: string | null;
  copy: (text: string, id: string) => void;
}

export function YourKeyCard({
  config,
  isConsensusMode,
  isSSSMode,
  copiedId,
  copy,
}: YourKeyCardProps) {
  const [showShare, setShowShare] = useState(false);

  return (
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
  );
}
