import { Alert } from "../ui";
import type { InitializeStepProps } from "./types";

export function HostSetupInitialize({
  state,
  updateState,
  onBack,
  renderStepIndicator,
  error,
  isInitializing,
  onInitialize,
}: InitializeStepProps) {
  return (
    <div className="max-w-xl mx-auto">
      <button
        onClick={() => onBack("terms")}
        className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
      >
        &larr; Back
      </button>

      {renderStepIndicator()}

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">🚀</div>
        <h1 className="text-2xl font-bold">Initialize Storage Server</h1>
        <p className="text-gray-400 mt-2">
          Set your name and start the backup server
        </p>
      </div>

      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">Your Name</label>
            <input
              type="text"
              value={state.name}
              onChange={(e) => updateState("name", e.target.value)}
              placeholder="e.g., bob-backup-server"
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
            />
            <p className="text-xs text-gray-500 mt-1">
              This identifies you to the data owner
            </p>
          </div>
        </div>
      </div>

      <SummarySection state={state} />

      <div className="bg-gray-800/50 rounded-lg p-4 mb-6 text-sm text-gray-400">
        <h3 className="font-medium text-gray-300 mb-2">What happens next</h3>
        <ul className="list-disc list-inside space-y-1">
          <li>Your Ed25519 key pair will be generated</li>
          <li>The storage server will start automatically</li>
          <li>You'll receive a URL to share with the data owner</li>
        </ul>
      </div>

      {error && (
        <Alert variant="error" className="mb-6">
          {error}
        </Alert>
      )}

      <div className="flex gap-4">
        <button
          onClick={() => onBack("terms")}
          className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Back
        </button>
        <button
          onClick={onInitialize}
          disabled={!state.name.trim() || isInitializing}
          className="flex-1 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors flex items-center justify-center gap-2"
        >
          {isInitializing ? (
            <>
              <span className="animate-spin">⏳</span>
              Starting...
            </>
          ) : (
            <>
              <span>🚀</span>
              Initialize & Start Server
            </>
          )}
        </button>
      </div>
    </div>
  );
}

interface SummarySectionProps {
  state: {
    storagePath: string;
    storageQuota: string;
    storageQuotaUnit: string;
    appendOnly: boolean;
    restoreApproval: string;
    deletionMode: string;
    retentionDays: string;
    localIP: string;
  };
}

function SummarySection({ state }: SummarySectionProps) {
  return (
    <div className="bg-gray-800 rounded-lg p-6 mb-6">
      <h3 className="font-medium mb-4">Summary</h3>
      <div className="space-y-2 text-sm">
        <div className="flex justify-between">
          <span className="text-gray-400">Storage Path:</span>
          <code className="text-green-400">{state.storagePath}</code>
        </div>
        {state.storageQuota && (
          <div className="flex justify-between">
            <span className="text-gray-400">Storage Quota:</span>
            <span>{state.storageQuota} {state.storageQuotaUnit}</span>
          </div>
        )}
        <div className="flex justify-between">
          <span className="text-gray-400">Append-Only:</span>
          <span>{state.appendOnly ? "Yes" : "No"}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-gray-400">Restore Approval:</span>
          <span className="capitalize">{state.restoreApproval.replace("-", " ")}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-gray-400">Deletion Policy:</span>
          <span className="capitalize">{state.deletionMode.replace("-", " ")}</span>
        </div>
        {state.retentionDays && (
          <div className="flex justify-between">
            <span className="text-gray-400">Retention Period:</span>
            <span>{state.retentionDays} days</span>
          </div>
        )}
        <div className="flex justify-between">
          <span className="text-gray-400">Your IP:</span>
          <span>{state.localIP}</span>
        </div>
      </div>
    </div>
  );
}
