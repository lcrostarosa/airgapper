import type { StepProps } from "./types";

export function HostSetupStorage({
  state,
  updateState,
  onNext,
  onBack,
  renderStepIndicator,
}: StepProps) {
  return (
    <div className="max-w-xl mx-auto">
      <button
        onClick={() => onBack("intro")}
        className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
      >
        &larr; Back
      </button>

      {renderStepIndicator()}

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">📁</div>
        <h1 className="text-2xl font-bold">Storage Location</h1>
        <p className="text-gray-400 mt-2">
          Where should backups be stored on this machine?
        </p>
      </div>

      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <label className="block text-sm font-medium mb-2">
          Select a folder for backup storage
        </label>

        <input
          type="text"
          value={state.storagePath}
          onChange={(e) => updateState("storagePath", e.target.value)}
          placeholder="/data/backups"
          className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
        />

        {state.storagePath && (
          <p className="text-sm text-gray-400 mt-2">
            Selected: <code className="text-green-400">{state.storagePath}</code>
          </p>
        )}
      </div>

      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <label className="block text-sm font-medium mb-2">
          Maximum storage quota (optional)
        </label>
        <p className="text-sm text-gray-400 mb-3">
          Limit how much space the data owner can use for backups
        </p>

        <div className="flex gap-2">
          <input
            type="number"
            value={state.storageQuota}
            onChange={(e) => updateState("storageQuota", e.target.value)}
            placeholder="e.g., 100"
            min="1"
            className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
          />
          <select
            value={state.storageQuotaUnit}
            onChange={(e) =>
              updateState("storageQuotaUnit", e.target.value as "GB" | "TB")
            }
            className="bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
          >
            <option value="GB">GB</option>
            <option value="TB">TB</option>
          </select>
        </div>

        {state.storageQuota && (
          <p className="text-sm text-gray-400 mt-2">
            Quota:{" "}
            <code className="text-green-400">
              {state.storageQuota} {state.storageQuotaUnit}
            </code>
          </p>
        )}
      </div>

      <div className="bg-yellow-900/30 border border-yellow-600/50 rounded-lg p-4 mb-6">
        <h3 className="font-medium text-yellow-400 mb-2">Recommendations</h3>
        <ul className="text-sm text-gray-300 space-y-1">
          <li>• Use a dedicated folder (not your home directory)</li>
          <li>• Ensure adequate free space for backups</li>
          <li>• Consider using a separate drive or RAID</li>
        </ul>
      </div>

      <div className="flex gap-4">
        <button
          onClick={() => onBack("intro")}
          className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Back
        </button>
        <button
          onClick={() => onNext("terms")}
          disabled={!state.storagePath.trim()}
          className="flex-1 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Next: Set Terms
        </button>
      </div>
    </div>
  );
}
