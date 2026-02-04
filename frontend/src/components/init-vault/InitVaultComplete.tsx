import { Alert, CopyableField } from "../ui";
import type { CompleteStepProps } from "./types";

export function InitVaultComplete({
  result,
  copiedId,
  onCopy,
  onStartOver,
  onSave,
}: CompleteStepProps) {
  const isConsensus = !!result.consensus;

  return (
    <div className="max-w-2xl mx-auto">
      <div className="text-center mb-8">
        <div className="text-4xl mb-2">✅</div>
        <h1 className="text-2xl font-bold">Vault Initialized!</h1>
        <p className="text-gray-400">
          {isConsensus
            ? `${result.consensus!.threshold}-of-${result.consensus!.totalKeys} consensus configured`
            : "2-of-2 key sharing configured"}
        </p>
      </div>

      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <span>📋</span> Your Configuration
        </h2>

        <div className="space-y-4">
          <div>
            <label className="text-sm text-gray-400">Name</label>
            <div className="font-mono bg-gray-900 rounded px-3 py-2">
              {result.name}
            </div>
          </div>

          <div>
            <label className="text-sm text-gray-400">Backup Server</label>
            <div className="font-mono bg-gray-900 rounded px-3 py-2 break-all">
              {result.repoUrl}
            </div>
          </div>

          {isConsensus && (
            <>
              <div>
                <label className="text-sm text-gray-400">Your Key ID</label>
                <div className="font-mono bg-gray-900 rounded px-3 py-2">
                  {result.keyId}
                </div>
              </div>
              <div>
                <label className="text-sm text-gray-400">Consensus</label>
                <div className="font-mono bg-gray-900 rounded px-3 py-2">
                  {result.consensus!.threshold}-of-{result.consensus!.totalKeys}
                </div>
              </div>
            </>
          )}

          {!isConsensus && (
            <div>
              <label className="text-sm text-gray-400">
                Your Share (Index {result.shareIndex})
              </label>
              <div className="font-mono bg-gray-900 rounded px-3 py-2 break-all text-sm">
                {result.localShare}
              </div>
            </div>
          )}

          {result.backupPaths && result.backupPaths.length > 0 && (
            <div>
              <label className="text-sm text-gray-400">Backup Paths</label>
              <div className="space-y-1">
                {result.backupPaths.map((path) => (
                  <div
                    key={path}
                    className="font-mono bg-gray-900 rounded px-3 py-2 text-sm"
                  >
                    {path}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Sharing instructions - Consensus mode */}
      {isConsensus && result.consensus!.totalKeys > 1 && (
        <Alert variant="warning" className="mb-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            Invite Key Holders
          </h2>
          <p className="text-sm text-gray-300 mb-4">
            You need {result.consensus!.totalKeys - 1} more key holder(s) to
            complete your {result.consensus!.threshold}-of-
            {result.consensus!.totalKeys} setup.
          </p>
          <p className="text-sm text-gray-400 mb-2">
            Key holders can join by running:
          </p>
          <CopyableField
            value={`airgapper join --name THEIR_NAME --repo "${result.repoUrl}" --consensus`}
            id="consensusCommand"
            externalCopied={copiedId === "consensusCommand"}
            onCopy={() =>
              onCopy(
                `airgapper join --name THEIR_NAME --repo "${result.repoUrl}" --consensus`,
                "consensusCommand"
              )
            }
          />
        </Alert>
      )}

      {/* Sharing instructions - SSS mode */}
      {!isConsensus && result.peerShare && (
        <Alert variant="warning" className="mb-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            Share with Your Peer
          </h2>
          <p className="text-sm text-gray-300 mb-4">
            Give this information to your trusted backup host.
          </p>

          <div className="space-y-3">
            <CopyableField
              label={`Peer Share (Index ${result.peerShareIndex})`}
              value={result.peerShare}
              id="peerShare"
              externalCopied={copiedId === "peerShare"}
              onCopy={() => onCopy(result.peerShare!, "peerShare")}
            />

            <CopyableField
              label="Join Command"
              value={`airgapper join --name PEER_NAME --repo "${result.repoUrl}" --share ${result.peerShare} --index ${result.peerShareIndex}`}
              id="command"
              externalCopied={copiedId === "command"}
              onCopy={() =>
                onCopy(
                  `airgapper join --name PEER_NAME --repo "${result.repoUrl}" --share ${result.peerShare} --index ${result.peerShareIndex}`,
                  "command"
                )
              }
            />
          </div>
        </Alert>
      )}

      <div className="flex gap-4">
        <button
          onClick={onStartOver}
          className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Start Over
        </button>
        <button
          onClick={onSave}
          className="flex-1 bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Save & Continue
        </button>
      </div>
    </div>
  );
}
