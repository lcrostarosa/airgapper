import { Alert } from "../ui";
import type { JoinVaultSSSProps } from "./types";

export function JoinVaultSSS({
  formData,
  error,
  onFieldChange,
  onSubmit,
}: JoinVaultSSSProps) {
  const isValid =
    formData.name.trim() && formData.repoUrl.trim() && formData.shareHex.trim();

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

        <div>
          <label className="block text-sm font-medium mb-2">
            Key Share (from data owner)
          </label>
          <textarea
            value={formData.shareHex}
            onChange={(e) => onFieldChange("shareHex", e.target.value)}
            placeholder="Paste the 64-character hex share here"
            rows={3}
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono text-sm"
          />
        </div>

        <div>
          <label className="block text-sm font-medium mb-2">Share Index</label>
          <select
            value={formData.shareIndex}
            onChange={(e) => onFieldChange("shareIndex", e.target.value)}
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
          >
            <option value="1">1</option>
            <option value="2">2</option>
          </select>
          <p className="text-xs text-gray-500 mt-1">
            Usually 2 for the backup host
          </p>
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
          disabled={!isValid}
          className="w-full bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors flex items-center justify-center gap-2"
        >
          <span>✓</span>
          Join Vault
        </button>
      </div>
    </>
  );
}
