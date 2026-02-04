import { Alert, CopyableField } from "../ui";
import { useClipboard } from "../../hooks/useClipboard";
import type { CompleteStepProps } from "./types";

export function HostSetupComplete({
  renderStepIndicator,
  initResult,
  onComplete,
  onStartOver,
}: CompleteStepProps) {
  const { copiedId, copy } = useClipboard();

  return (
    <div className="max-w-xl mx-auto">
      {renderStepIndicator()}

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">✅</div>
        <h1 className="text-2xl font-bold">Setup Complete!</h1>
        <p className="text-gray-400 mt-2">
          Share your details with the data owner
        </p>
      </div>

      <div className="bg-green-900/30 border border-green-600/50 rounded-lg p-4 mb-6">
        <div className="flex items-center gap-2 text-green-400 mb-2">
          <span>✓</span>
          <span className="font-medium">Storage server is running</span>
        </div>
        <p className="text-sm text-gray-300">
          The backup server is now accepting connections at the URL below.
        </p>
      </div>

      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <div className="space-y-4">
          <div>
            <label className="text-sm text-gray-400">Your Name</label>
            <div className="font-mono bg-gray-900 rounded px-3 py-2">
              {initResult.name}
            </div>
          </div>

          <div>
            <label className="text-sm text-gray-400">Your Key ID</label>
            <div className="font-mono bg-gray-900 rounded px-3 py-2 text-sm">
              {initResult.keyId}
            </div>
          </div>

          <CopyableField
            label="Storage URL (share this)"
            value={initResult.storageUrl}
            id="url"
            externalCopied={copiedId === "url"}
            onCopy={() => copy(initResult.storageUrl, "url")}
          />
        </div>
      </div>

      <Alert variant="warning" className="mb-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          Share with the Data Owner
        </h2>
        <p className="text-sm text-gray-300 mb-4">
          The data owner needs your public key to add you as a key holder.
        </p>

        <CopyableField
          label="Your Public Key"
          value={initResult.publicKey}
          id="pubkey"
          externalCopied={copiedId === "pubkey"}
          onCopy={() => copy(initResult.publicKey, "pubkey")}
        />
      </Alert>

      <div className="flex gap-4">
        <button
          onClick={onStartOver}
          className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Start Over
        </button>
        <button
          onClick={onComplete}
          className="flex-1 bg-green-600 hover:bg-green-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Continue to Dashboard
        </button>
      </div>
    </div>
  );
}
