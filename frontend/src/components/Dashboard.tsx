import { useState } from "react";
import type { VaultConfig, Step } from "../types";
import { fromHex, combine, toHex } from "../lib/sss";

interface DashboardProps {
  config: VaultConfig;
  onNavigate?: (step: Step) => void;
  onClear: () => void;
}

export function Dashboard({ config, onClear }: DashboardProps) {
  const [showShare, setShowShare] = useState(false);
  const [testShare, setTestShare] = useState("");
  const [testResult, setTestResult] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const copyToClipboard = async (text: string) => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const testReconstruct = () => {
    if (!testShare.trim() || !config.localShare) {
      setTestResult("Enter both shares to test");
      return;
    }

    try {
      const share1 = {
        index: config.shareIndex!,
        data: fromHex(config.localShare),
      };
      const share2 = {
        index: config.shareIndex === 1 ? 2 : 1,
        data: fromHex(testShare.trim()),
      };

      const reconstructed = combine([share1, share2]);
      const reconstructedHex = toHex(reconstructed);

      if (config.password && reconstructedHex === config.password) {
        setTestResult("✅ Success! Password reconstructed correctly.");
      } else if (config.password) {
        setTestResult("❌ Mismatch! The reconstructed password doesn't match.");
      } else {
        setTestResult(`✅ Reconstructed: ${reconstructedHex}`);
      }
    } catch (error) {
      setTestResult("❌ Error: " + (error as Error).message);
    }
  };

  return (
    <div className="max-w-3xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-3">
            <span className="text-3xl">
              {config.role === "owner" ? "👑" : "🤝"}
            </span>
            {config.name}
          </h1>
          <p className="text-gray-400">
            {config.role === "owner" ? "Data Owner" : "Backup Host"}
          </p>
        </div>
        <button
          onClick={onClear}
          className="text-sm text-gray-500 hover:text-red-400 transition-colors"
        >
          Reset Vault
        </button>
      </div>

      {/* Status Card */}
      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <span>📊</span> Status
        </h2>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="bg-gray-700 rounded p-3">
            <div className="text-sm text-gray-400">Role</div>
            <div className="font-medium capitalize">{config.role}</div>
          </div>
          <div className="bg-gray-700 rounded p-3">
            <div className="text-sm text-gray-400">Share Index</div>
            <div className="font-medium">{config.shareIndex}</div>
          </div>
          <div className="bg-gray-700 rounded p-3">
            <div className="text-sm text-gray-400">Share</div>
            <div className="font-medium text-green-400">✓ Stored</div>
          </div>
          <div className="bg-gray-700 rounded p-3">
            <div className="text-sm text-gray-400">Password</div>
            <div className="font-medium">
              {config.password ? (
                <span className="text-green-400">✓ Available</span>
              ) : (
                <span className="text-gray-500">— (host)</span>
              )}
            </div>
          </div>
        </div>

        <div className="mt-4">
          <div className="text-sm text-gray-400 mb-1">Repository</div>
          <code className="block bg-gray-900 rounded px-3 py-2 font-mono text-sm break-all">
            {config.repoUrl}
          </code>
        </div>
      </div>

      {/* Key Share Card */}
      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <span>🔑</span> Your Key Share
        </h2>

        <div className="mb-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm text-gray-400">
              Share {config.shareIndex}
            </span>
            <button
              onClick={() => setShowShare(!showShare)}
              className="text-sm text-blue-400 hover:text-blue-300 transition-colors"
            >
              {showShare ? "Hide" : "Show"}
            </button>
          </div>
          {showShare ? (
            <div className="flex gap-2">
              <code className="flex-1 bg-gray-900 rounded px-3 py-2 font-mono text-sm break-all">
                {config.localShare}
              </code>
              <button
                onClick={() => copyToClipboard(config.localShare!)}
                className="px-3 py-2 bg-gray-700 hover:bg-gray-600 rounded transition-colors"
              >
                {copied ? "✓" : "📋"}
              </button>
            </div>
          ) : (
            <div className="bg-gray-900 rounded px-3 py-2 font-mono text-sm text-gray-500">
              ••••••••••••••••••••••••••••••••
            </div>
          )}
        </div>

        {config.role === "owner" && config.peerShare && (
          <div className="pt-4 border-t border-gray-700">
            <div className="text-sm text-yellow-400 mb-2">
              ⚠️ Peer share (give to your backup host):
            </div>
            <code className="block bg-gray-900 rounded px-3 py-2 font-mono text-xs break-all">
              {config.peerShare}
            </code>
          </div>
        )}
      </div>

      {/* Test Reconstruction (Owner only) */}
      {config.role === "owner" && (
        <div className="bg-gray-800 rounded-lg p-6 mb-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <span>🧪</span> Test Key Reconstruction
          </h2>
          <p className="text-sm text-gray-400 mb-4">
            Verify that the shares can reconstruct the password correctly.
          </p>

          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">
                Peer's Share (Index {config.shareIndex === 1 ? 2 : 1})
              </label>
              <input
                type="text"
                value={testShare}
                onChange={(e) => setTestShare(e.target.value)}
                placeholder="Paste the peer's share to test reconstruction"
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono text-sm"
              />
            </div>

            <button
              onClick={testReconstruct}
              disabled={!testShare.trim()}
              className="bg-purple-600 hover:bg-purple-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-2 px-4 rounded-lg transition-colors"
            >
              Test Reconstruction
            </button>

            {testResult && (
              <div
                className={`p-3 rounded-lg text-sm ${
                  testResult.startsWith("✅")
                    ? "bg-green-900/30 border border-green-500/50 text-green-400"
                    : testResult.startsWith("❌")
                    ? "bg-red-900/30 border border-red-500/50 text-red-400"
                    : "bg-gray-700 text-gray-300"
                }`}
              >
                {testResult}
              </div>
            )}
          </div>
        </div>
      )}

      {/* CLI Integration */}
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <span>💻</span> CLI Commands
        </h2>

        <div className="space-y-3 text-sm">
          {config.role === "owner" ? (
            <>
              <div>
                <div className="text-gray-400 mb-1">Create backup:</div>
                <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                  airgapper backup ~/Documents
                </code>
              </div>
              <div>
                <div className="text-gray-400 mb-1">List snapshots:</div>
                <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                  airgapper snapshots
                </code>
              </div>
              <div>
                <div className="text-gray-400 mb-1">Request restore:</div>
                <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                  airgapper request --snapshot latest --reason "need files"
                </code>
              </div>
            </>
          ) : (
            <>
              <div>
                <div className="text-gray-400 mb-1">View pending requests:</div>
                <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                  airgapper pending
                </code>
              </div>
              <div>
                <div className="text-gray-400 mb-1">Approve a request:</div>
                <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                  airgapper approve {"<request-id>"}
                </code>
              </div>
              <div>
                <div className="text-gray-400 mb-1">Start API server:</div>
                <code className="block bg-gray-900 rounded px-3 py-2 font-mono">
                  airgapper serve --addr :8080
                </code>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
