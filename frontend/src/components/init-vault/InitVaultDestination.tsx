import type { StepProps } from "./types";

export function InitVaultDestination({
  state,
  updateState,
  onNext,
  onBack,
  renderStepIndicator,
}: StepProps) {
  const { repoUrl } = state;

  const handleNext = () => {
    if (!repoUrl.trim()) return;
    onNext();
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
        <h3 className="text-lg font-semibold mb-2">Where should backups be stored?</h3>
        <p className="text-sm text-gray-400 mb-6">
          Your encrypted backups will be sent to this server.
        </p>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">
              Backup Server URL
            </label>
            <input
              type="text"
              value={repoUrl}
              onChange={(e) => updateState("repoUrl", e.target.value)}
              placeholder="rest:http://192.168.1.50:8000/backup"
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono"
            />
            <p className="text-xs text-gray-500 mt-2">
              This is typically a restic REST server on your network or a remote backup service.
            </p>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <p className="text-xs text-gray-400 mb-2">Examples:</p>
            <ul className="text-xs text-gray-500 space-y-1 font-mono">
              <li>rest:http://192.168.1.50:8000/mybackup</li>
              <li>rest:https://backup.example.com:8000/vault</li>
            </ul>
          </div>
        </div>

        <div className="mt-6 pt-6 border-t border-gray-700">
          <button
            onClick={handleNext}
            disabled={!repoUrl.trim()}
            className="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Next: Security Settings &rarr;
          </button>
        </div>
      </div>
    </div>
  );
}
