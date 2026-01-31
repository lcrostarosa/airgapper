import { useState } from "react";
import type { VaultConfig, Step } from "../types";
import { fromHex } from "../lib/sss";

interface JoinVaultProps {
  onComplete: (config: VaultConfig) => void;
  onNavigate: (step: Step) => void;
}

export function JoinVault({ onComplete, onNavigate }: JoinVaultProps) {
  const [name, setName] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [shareHex, setShareHex] = useState("");
  const [shareIndex, setShareIndex] = useState("2");
  const [error, setError] = useState("");

  const handleJoin = () => {
    setError("");

    if (!name.trim()) {
      setError("Name is required");
      return;
    }
    if (!repoUrl.trim()) {
      setError("Repository URL is required");
      return;
    }
    if (!shareHex.trim()) {
      setError("Share is required");
      return;
    }

    // Validate hex
    try {
      const share = fromHex(shareHex.trim());
      if (share.length !== 32) {
        setError("Invalid share (expected 64 hex characters)");
        return;
      }
    } catch {
      setError("Invalid share (must be hex encoded)");
      return;
    }

    const config: VaultConfig = {
      name: name.trim(),
      role: "host",
      repoUrl: repoUrl.trim(),
      localShare: shareHex.trim(),
      shareIndex: parseInt(shareIndex),
    };

    onComplete(config);
  };

  return (
    <div className="max-w-xl mx-auto">
      <button
        onClick={() => onNavigate("welcome")}
        className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
      >
        ← Back
      </button>

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">🤝</div>
        <h1 className="text-2xl font-bold">Join as Backup Host</h1>
        <p className="text-gray-400">
          Enter the share given to you by the data owner
        </p>
      </div>

      <div className="bg-gray-800 rounded-lg p-6">
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">Your Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
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
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder="e.g., rest:http://192.168.1.50:8000/backup"
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
            />
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">
              Key Share (from data owner)
            </label>
            <textarea
              value={shareHex}
              onChange={(e) => setShareHex(e.target.value)}
              placeholder="Paste the 64-character hex share here"
              rows={3}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono text-sm"
            />
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">
              Share Index
            </label>
            <select
              value={shareIndex}
              onChange={(e) => setShareIndex(e.target.value)}
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
          <div className="mt-4 p-3 bg-red-900/30 border border-red-500/50 rounded-lg text-red-400 text-sm">
            {error}
          </div>
        )}

        <div className="mt-6 pt-6 border-t border-gray-700">
          <button
            onClick={handleJoin}
            disabled={!name.trim() || !repoUrl.trim() || !shareHex.trim()}
            className="w-full bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors flex items-center justify-center gap-2"
          >
            <span>✓</span>
            Join Vault
          </button>
        </div>
      </div>

      <div className="mt-6 bg-gray-800/50 rounded-lg p-4 text-sm text-gray-400">
        <h3 className="font-medium text-gray-300 mb-2">Your role as host:</h3>
        <ul className="list-disc list-inside space-y-1">
          <li>Store your key share securely</li>
          <li>Approve or deny restore requests from the owner</li>
          <li>You cannot access the backup data without the owner's share</li>
        </ul>
      </div>
    </div>
  );
}
