import { useState } from "react";
import type { VaultConfig, Step } from "../types";
import { split, generatePassword, toHex } from "../lib/sss";

interface InitVaultProps {
  onComplete: (config: VaultConfig) => void;
  onNavigate: (step: Step) => void;
}

export function InitVault({ onComplete, onNavigate }: InitVaultProps) {
  const [name, setName] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [isGenerating, setIsGenerating] = useState(false);
  const [result, setResult] = useState<VaultConfig | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const handleGenerate = async () => {
    if (!name.trim() || !repoUrl.trim()) return;

    setIsGenerating(true);

    // Simulate a brief delay for UX
    await new Promise((r) => setTimeout(r, 500));

    try {
      // Generate random password (32 bytes = 256 bits)
      const password = generatePassword();
      const passwordHex = toHex(password);

      // Split using 2-of-2 Shamir's Secret Sharing
      const shares = split(password, 2, 2);

      const config: VaultConfig = {
        name: name.trim(),
        role: "owner",
        repoUrl: repoUrl.trim(),
        password: passwordHex,
        localShare: toHex(shares[0].data),
        shareIndex: shares[0].index,
        peerShare: toHex(shares[1].data),
        peerShareIndex: shares[1].index,
      };

      setResult(config);
    } catch (error) {
      console.error("Failed to generate keys:", error);
      alert("Failed to generate keys: " + (error as Error).message);
    } finally {
      setIsGenerating(false);
    }
  };

  const copyToClipboard = async (text: string, label: string) => {
    await navigator.clipboard.writeText(text);
    setCopied(label);
    setTimeout(() => setCopied(null), 2000);
  };

  const handleSave = () => {
    if (result) {
      onComplete(result);
    }
  };

  if (result) {
    return (
      <div className="max-w-2xl mx-auto">
        <div className="text-center mb-8">
          <div className="text-4xl mb-2">✅</div>
          <h1 className="text-2xl font-bold">Vault Initialized!</h1>
          <p className="text-gray-400">
            Your encryption keys have been generated and split
          </p>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 mb-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <span>📋</span> Your Configuration
          </h2>

          <div className="space-y-4">
            <div>
              <label className="text-sm text-gray-400">Name</label>
              <div className="font-mono bg-gray-900 rounded px-3 py-2">
                {result.name}
              </div>
            </div>

            <div>
              <label className="text-sm text-gray-400">Repository</label>
              <div className="font-mono bg-gray-900 rounded px-3 py-2 break-all">
                {result.repoUrl}
              </div>
            </div>

            <div>
              <label className="text-sm text-gray-400">
                Your Share (Index {result.shareIndex})
              </label>
              <div className="font-mono bg-gray-900 rounded px-3 py-2 break-all text-sm">
                {result.localShare}
              </div>
            </div>
          </div>
        </div>

        <div className="bg-yellow-900/30 border border-yellow-600/50 rounded-lg p-6 mb-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2 text-yellow-400">
            <span>⚠️</span> Share with Your Peer
          </h2>
          <p className="text-sm text-gray-300 mb-4">
            Give this information to your trusted backup host. They need it to
            join and hold their key share.
          </p>

          <div className="space-y-3">
            <div>
              <label className="text-sm text-gray-400">
                Peer Share (Index {result.peerShareIndex})
              </label>
              <div className="flex gap-2">
                <code className="flex-1 font-mono bg-gray-900 rounded px-3 py-2 break-all text-sm">
                  {result.peerShare}
                </code>
                <button
                  onClick={() =>
                    copyToClipboard(result.peerShare!, "peerShare")
                  }
                  className="px-3 py-2 bg-gray-700 hover:bg-gray-600 rounded transition-colors"
                >
                  {copied === "peerShare" ? "✓" : "📋"}
                </button>
              </div>
            </div>

            <div>
              <label className="text-sm text-gray-400">Join Command</label>
              <div className="flex gap-2">
                <code className="flex-1 font-mono bg-gray-900 rounded px-3 py-2 break-all text-xs">
                  airgapper join --name PEER_NAME --repo "{result.repoUrl}"
                  --share {result.peerShare} --index {result.peerShareIndex}
                </code>
                <button
                  onClick={() =>
                    copyToClipboard(
                      `airgapper join --name PEER_NAME --repo "${result.repoUrl}" --share ${result.peerShare} --index ${result.peerShareIndex}`,
                      "command"
                    )
                  }
                  className="px-3 py-2 bg-gray-700 hover:bg-gray-600 rounded transition-colors"
                >
                  {copied === "command" ? "✓" : "📋"}
                </button>
              </div>
            </div>
          </div>
        </div>

        <div className="flex gap-4">
          <button
            onClick={() => setResult(null)}
            className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Start Over
          </button>
          <button
            onClick={handleSave}
            className="flex-1 bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Save & Continue
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-xl mx-auto">
      <button
        onClick={() => onNavigate("welcome")}
        className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
      >
        ← Back
      </button>

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">🔐</div>
        <h1 className="text-2xl font-bold">Initialize New Vault</h1>
        <p className="text-gray-400">Set up as the data owner</p>
      </div>

      <div className="bg-gray-800 rounded-lg p-6">
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">Your Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., alice"
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
            <p className="text-xs text-gray-500 mt-1">
              Local path or REST server URL for restic repository
            </p>
          </div>
        </div>

        <div className="mt-6 pt-6 border-t border-gray-700">
          <button
            onClick={handleGenerate}
            disabled={!name.trim() || !repoUrl.trim() || isGenerating}
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
                Generate Keys
              </>
            )}
          </button>
        </div>
      </div>

      <div className="mt-6 bg-gray-800/50 rounded-lg p-4 text-sm text-gray-400">
        <h3 className="font-medium text-gray-300 mb-2">What happens next:</h3>
        <ol className="list-decimal list-inside space-y-1">
          <li>A secure 256-bit password will be generated</li>
          <li>
            Password will be split into 2 shares using Shamir's Secret Sharing
          </li>
          <li>You keep one share, give the other to your backup host</li>
          <li>Both shares are required to restore data</li>
        </ol>
      </div>
    </div>
  );
}
