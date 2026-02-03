import { useClipboard } from "../../hooks/useClipboard";
import { Alert, CopyableField } from "../ui";
import type { JoinVaultResultProps } from "./types";

export function JoinVaultResult({
  name,
  repoUrl,
  generatedKeys,
  onBack,
  onConfirm,
}: JoinVaultResultProps) {
  const { copied, copy } = useClipboard();

  return (
    <div className="max-w-xl mx-auto">
      <button
        onClick={onBack}
        className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
      >
        &larr; Back
      </button>

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">🔑</div>
        <h1 className="text-2xl font-bold">Keys Generated!</h1>
        <p className="text-gray-400">
          Share your public key with the vault owner
        </p>
      </div>

      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <div className="space-y-4">
          <div>
            <label className="text-sm text-gray-400">Your Name</label>
            <div className="font-mono bg-gray-900 rounded px-3 py-2">{name}</div>
          </div>

          <div>
            <label className="text-sm text-gray-400">Your Key ID</label>
            <div className="font-mono bg-gray-900 rounded px-3 py-2">
              {generatedKeys.keyId}
            </div>
          </div>

          <div>
            <label className="text-sm text-gray-400">Repository</label>
            <div className="font-mono bg-gray-900 rounded px-3 py-2 break-all">
              {repoUrl}
            </div>
          </div>
        </div>
      </div>

      <Alert variant="warning" className="mb-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          Register with Vault Owner
        </h2>
        <p className="text-sm text-gray-300 mb-4">
          Share your public key with the vault owner so they can add you as a
          key holder.
        </p>

        <CopyableField
          label="Your Public Key"
          value={generatedKeys.publicKey}
          externalCopied={copied}
          onCopy={() => copy(generatedKeys.publicKey)}
        />
      </Alert>

      <div className="flex gap-4">
        <button
          onClick={onBack}
          className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Start Over
        </button>
        <button
          onClick={onConfirm}
          className="flex-1 bg-green-600 hover:bg-green-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Save & Continue
        </button>
      </div>
    </div>
  );
}
