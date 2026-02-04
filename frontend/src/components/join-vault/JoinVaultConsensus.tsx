import { Alert } from "../ui";
import type { JoinVaultConsensusProps } from "./types";

export function JoinVaultConsensus({
  formData,
  error,
  isGenerating,
  onFieldChange,
  onSubmit,
}: JoinVaultConsensusProps) {
  const isValid = formData.name.trim() && formData.repoUrl.trim();

  return (
    <>
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium mb-2">Your Name</label>
          <input
            type="text"
            value={formData.name}
            onChange={(e) => onFieldChange("name", e.target.value)}
            placeholder="e.g., bob"
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
          />
        </div>

        <div>
          <label className="block text-sm font-medium mb-2">
            Repository URL
          </label>
          <input
            type="text"
            value={formData.repoUrl}
            onChange={(e) => onFieldChange("repoUrl", e.target.value)}
            placeholder="e.g., rest:http://192.168.1.50:8000/backup"
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
          />
        </div>
      </div>

      {error && (
        <Alert variant="error" className="mt-4">
          {error}
        </Alert>
      )}

      <div className="mt-6 pt-6 border-t border-gray-700">
        <button
          onClick={onSubmit}
          disabled={!isValid || isGenerating}
          className="w-full bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors flex items-center justify-center gap-2"
        >
          {isGenerating ? (
            <>
              <span className="animate-spin">⏳</span>
              Generating Keys...
            </>
          ) : (
            <>
              <span>🔑</span>
              Generate Keys
            </>
          )}
        </button>
      </div>
    </>
  );
}
