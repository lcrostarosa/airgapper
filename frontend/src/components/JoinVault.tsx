import { useState } from "react";
import type { VaultConfig, Step } from "../types";
import { fromHex } from "../lib/sss";
import { generateKeyPair, keyId } from "../lib/crypto";

interface JoinVaultProps {
  onComplete: (config: VaultConfig) => void;
  onNavigate: (step: Step) => void;
}

type JoinMode = "sss" | "consensus";

export function JoinVault({ onComplete, onNavigate }: JoinVaultProps) {
  const [mode, setMode] = useState<JoinMode>("consensus");
  const [name, setName] = useState("");
  const [repoUrl, setRepoUrl] = useState("");

  // SSS mode fields
  const [shareHex, setShareHex] = useState("");
  const [shareIndex, setShareIndex] = useState("2");

  // Result state
  const [generatedKeys, setGeneratedKeys] = useState<{
    publicKey: string;
    privateKey: string;
    keyId: string;
  } | null>(null);
  const [error, setError] = useState("");
  const [isGenerating, setIsGenerating] = useState(false);
  const [copied, setCopied] = useState(false);

  const handleJoinSSS = () => {
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

  const handleJoinConsensus = async () => {
    setError("");

    if (!name.trim()) {
      setError("Name is required");
      return;
    }
    if (!repoUrl.trim()) {
      setError("Repository URL is required");
      return;
    }

    setIsGenerating(true);

    try {
      // Generate Ed25519 key pair
      const keys = await generateKeyPair();
      const id = await keyId(keys.publicKey);

      setGeneratedKeys({
        publicKey: keys.publicKey,
        privateKey: keys.privateKey,
        keyId: id,
      });
    } catch (err) {
      setError("Failed to generate keys: " + (err as Error).message);
    } finally {
      setIsGenerating(false);
    }
  };

  const handleConfirmConsensus = () => {
    if (!generatedKeys) return;

    const config: VaultConfig = {
      name: name.trim(),
      role: "host",
      repoUrl: repoUrl.trim(),
      publicKey: generatedKeys.publicKey,
      privateKey: generatedKeys.privateKey,
      keyId: generatedKeys.keyId,
    };

    onComplete(config);
  };

  const copyToClipboard = async (text: string) => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  // Show result screen for consensus mode
  if (generatedKeys) {
    return (
      <div className="max-w-xl mx-auto">
        <button
          onClick={() => setGeneratedKeys(null)}
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
              <div className="font-mono bg-gray-900 rounded px-3 py-2">
                {name}
              </div>
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

        <div className="bg-yellow-900/30 border border-yellow-600/50 rounded-lg p-6 mb-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2 text-yellow-400">
            <span>⚠️</span> Register with Vault Owner
          </h2>
          <p className="text-sm text-gray-300 mb-4">
            Share your public key with the vault owner so they can add you as a
            key holder.
          </p>

          <div>
            <label className="text-sm text-gray-400">Your Public Key</label>
            <div className="flex gap-2">
              <code className="flex-1 font-mono bg-gray-900 rounded px-3 py-2 break-all text-xs">
                {generatedKeys.publicKey}
              </code>
              <button
                onClick={() => copyToClipboard(generatedKeys.publicKey)}
                className="px-3 py-2 bg-gray-700 hover:bg-gray-600 rounded transition-colors"
              >
                {copied ? "✓" : "📋"}
              </button>
            </div>
          </div>
        </div>

        <div className="flex gap-4">
          <button
            onClick={() => setGeneratedKeys(null)}
            className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Start Over
          </button>
          <button
            onClick={handleConfirmConsensus}
            className="flex-1 bg-green-600 hover:bg-green-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
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
        &larr; Back
      </button>

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">🤝</div>
        <h1 className="text-2xl font-bold">Join as Key Holder</h1>
        <p className="text-gray-400">
          Join an existing vault as a backup host or key holder
        </p>
      </div>

      {/* Mode selector */}
      <div className="flex gap-2 mb-6">
        <button
          onClick={() => setMode("consensus")}
          className={`flex-1 py-2 px-4 rounded-lg transition-colors ${
            mode === "consensus"
              ? "bg-blue-600 text-white"
              : "bg-gray-700 text-gray-300 hover:bg-gray-600"
          }`}
        >
          Consensus Mode
        </button>
        <button
          onClick={() => setMode("sss")}
          className={`flex-1 py-2 px-4 rounded-lg transition-colors ${
            mode === "sss"
              ? "bg-blue-600 text-white"
              : "bg-gray-700 text-gray-300 hover:bg-gray-600"
          }`}
        >
          SSS Mode (Legacy)
        </button>
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

          {mode === "sss" && (
            <>
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
            </>
          )}
        </div>

        {error && (
          <div className="mt-4 p-3 bg-red-900/30 border border-red-500/50 rounded-lg text-red-400 text-sm">
            {error}
          </div>
        )}

        <div className="mt-6 pt-6 border-t border-gray-700">
          {mode === "consensus" ? (
            <button
              onClick={handleJoinConsensus}
              disabled={!name.trim() || !repoUrl.trim() || isGenerating}
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
          ) : (
            <button
              onClick={handleJoinSSS}
              disabled={!name.trim() || !repoUrl.trim() || !shareHex.trim()}
              className="w-full bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors flex items-center justify-center gap-2"
            >
              <span>✓</span>
              Join Vault
            </button>
          )}
        </div>
      </div>

      <div className="mt-6 bg-gray-800/50 rounded-lg p-4 text-sm text-gray-400">
        <h3 className="font-medium text-gray-300 mb-2">
          {mode === "consensus" ? "Consensus Mode:" : "SSS Mode:"}
        </h3>
        {mode === "consensus" ? (
          <ul className="list-disc list-inside space-y-1">
            <li>Generate your own Ed25519 key pair</li>
            <li>Share your public key with the vault owner</li>
            <li>Sign restore requests when needed</li>
            <li>Flexible m-of-n approval requirements</li>
          </ul>
        ) : (
          <ul className="list-disc list-inside space-y-1">
            <li>Store your key share securely</li>
            <li>Approve or deny restore requests from the owner</li>
            <li>You cannot access the backup data without the owner's share</li>
          </ul>
        )}
      </div>
    </div>
  );
}
