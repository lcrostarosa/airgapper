import { ConsensusSetup } from "../ConsensusSetup";
import type { SecurityStepProps } from "./types";

export function InitVaultSecurity({
  state,
  updateState,
  onBack,
  isGenerating,
  onGenerate,
  renderStepIndicator,
}: SecurityStepProps) {
  const { threshold, totalKeys, useConsensusMode } = state;

  const handleConsensusSelect = (
    m: number,
    n: number,
    needsApproval: boolean
  ) => {
    updateState("threshold", m);
    updateState("totalKeys", n);
    updateState("requireApproval", needsApproval);
  };

  return (
    <div className="max-w-xl mx-auto">
      <button
        onClick={onBack}
        className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
      >
        &larr; Back
      </button>

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">🔐</div>
        <h1 className="text-2xl font-bold">Initialize New Vault</h1>
        <p className="text-gray-400">Set up as the data owner</p>
      </div>

      {renderStepIndicator()}

      <div className="bg-gray-800 rounded-lg p-6">
        <h3 className="text-lg font-semibold mb-2">Security Settings</h3>
        <p className="text-sm text-gray-400 mb-6">
          Configure how restore access is controlled.
        </p>

        <label className="flex items-center gap-2 text-sm text-gray-400 mb-4">
          <input
            type="checkbox"
            checked={useConsensusMode}
            onChange={(e) => updateState("useConsensusMode", e.target.checked)}
            className="rounded bg-gray-700"
          />
          Use consensus mode (Ed25519 signatures, recommended)
        </label>

        {useConsensusMode ? (
          <ConsensusSetup
            onSelect={handleConsensusSelect}
            initialThreshold={threshold}
            initialTotalKeys={totalKeys}
          />
        ) : (
          <div className="text-center py-8 bg-gray-900 rounded-lg">
            <div className="text-4xl mb-4">👥</div>
            <h3 className="text-lg font-semibold mb-2">2-of-2 Key Sharing</h3>
            <p className="text-gray-400 text-sm">
              Using Shamir's Secret Sharing. The password will be split into 2
              shares. Both are required to restore.
            </p>
          </div>
        )}

        <div className="mt-6 pt-6 border-t border-gray-700">
          <button
            onClick={onGenerate}
            disabled={isGenerating}
            className="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors flex items-center justify-center gap-2"
          >
            {isGenerating ? (
              <>
                <span className="animate-spin">⏳</span>
                Generating Keys...
              </>
            ) : (
              <>
                <span>🔑</span>
                Generate Keys & Initialize
              </>
            )}
          </button>
        </div>
      </div>

      {/* Info box */}
      <div className="mt-6 bg-gray-800/50 rounded-lg p-4 text-sm text-gray-400">
        <h3 className="font-medium text-gray-300 mb-2">What happens:</h3>
        <ol className="list-decimal list-inside space-y-1">
          <li>A secure 256-bit password is generated</li>
          {useConsensusMode ? (
            <>
              <li>Ed25519 key pair is created for signing</li>
              <li>
                {threshold === 1 && totalKeys === 1
                  ? "Solo mode: you control all restores"
                  : `${threshold} of ${totalKeys} signatures required to restore`}
              </li>
            </>
          ) : (
            <>
              <li>Password is split using Shamir's Secret Sharing</li>
              <li>Both shares are required to restore data</li>
            </>
          )}
        </ol>
      </div>
    </div>
  );
}
