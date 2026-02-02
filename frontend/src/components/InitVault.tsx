import { useState } from "react";
import type { VaultConfig, Step } from "../types";
import { split, generatePassword, toHex } from "../lib/sss";
import { generateKeyPair, keyId } from "../lib/crypto";
import { ConsensusSetup } from "./ConsensusSetup";
import { useClipboard } from "../hooks/useClipboard";
import { Alert, CopyableField, StepIndicator } from "./ui";

interface InitVaultProps {
  onComplete: (config: VaultConfig) => void;
  onNavigate: (step: Step) => void;
}

type InitStep = "paths" | "destination" | "security" | "result";

export function InitVault({ onComplete, onNavigate }: InitVaultProps) {
  const [currentStep, setCurrentStep] = useState<InitStep>("paths");
  const [name, setName] = useState("");
  const [repoUrl, setRepoUrl] = useState("");

  // Consensus configuration
  const [threshold, setThreshold] = useState(2);
  const [totalKeys, setTotalKeys] = useState(2);
  const [requireApproval, setRequireApproval] = useState(true);
  const [useConsensusMode, setUseConsensusMode] = useState(true);

  // Backup paths
  const [backupPaths, setBackupPaths] = useState<string[]>([]);
  const [newPath, setNewPath] = useState("");

  // Result
  const [isGenerating, setIsGenerating] = useState(false);
  const [result, setResult] = useState<VaultConfig | null>(null);
  const { copiedId, copy } = useClipboard();

  // Step 1: Paths - validate name and move to destination
  const handlePathsStepNext = () => {
    if (!name.trim()) return;
    if (backupPaths.length === 0) return;
    setCurrentStep("destination");
  };

  // Step 2: Destination - validate repo URL and move to security
  const handleDestinationNext = () => {
    if (!repoUrl.trim()) return;
    setCurrentStep("security");
  };

  const handleConsensusSelect = (
    m: number,
    n: number,
    needsApproval: boolean
  ) => {
    setThreshold(m);
    setTotalKeys(n);
    setRequireApproval(needsApproval);
  };

  // Step 3: Security - generate keys and finalize
  const handleSecurityNext = async () => {
    setIsGenerating(true);

    // Simulate a brief delay for UX
    await new Promise((r) => setTimeout(r, 500));

    try {
      let config: VaultConfig;

      if (useConsensusMode) {
        // Generate Ed25519 key pair
        const keys = await generateKeyPair();
        const id = await keyId(keys.publicKey);

        // Generate random password
        const password = generatePassword();
        const passwordHex = toHex(password);

        config = {
          name: name.trim(),
          role: "owner",
          repoUrl: repoUrl.trim(),
          password: passwordHex,
          publicKey: keys.publicKey,
          privateKey: keys.privateKey,
          keyId: id,
          consensus: {
            threshold,
            totalKeys,
            keyHolders: [
              {
                id,
                name: name.trim(),
                publicKey: keys.publicKey,
                isOwner: true,
              },
            ],
            requireApproval,
          },
          backupPaths,
        };
      } else {
        // Legacy SSS mode
        const password = generatePassword();
        const passwordHex = toHex(password);

        // Split using 2-of-2 Shamir's Secret Sharing
        const shares = split(password, 2, 2);

        config = {
          name: name.trim(),
          role: "owner",
          repoUrl: repoUrl.trim(),
          password: passwordHex,
          localShare: toHex(shares[0].data),
          shareIndex: shares[0].index,
          peerShare: toHex(shares[1].data),
          peerShareIndex: shares[1].index,
          backupPaths,
        };
      }

      setResult(config);
      setCurrentStep("result");
    } catch (error) {
      console.error("Failed to generate keys:", error);
      alert("Failed to generate keys: " + (error as Error).message);
    } finally {
      setIsGenerating(false);
    }
  };

  const addPath = (path: string) => {
    if (path.trim() && !backupPaths.includes(path.trim())) {
      setBackupPaths((prev) => [...prev, path.trim()]);
    }
  };

  const removePath = (path: string) => {
    setBackupPaths((prev) => prev.filter((p) => p !== path));
  };


  const handleSave = () => {
    if (result) {
      onComplete(result);
    }
  };

  // Result screen
  if (currentStep === "result" && result) {
    const isConsensus = !!result.consensus;

    return (
      <div className="max-w-2xl mx-auto">
        <div className="text-center mb-8">
          <div className="text-4xl mb-2">✅</div>
          <h1 className="text-2xl font-bold">Vault Initialized!</h1>
          <p className="text-gray-400">
            {isConsensus
              ? `${result.consensus!.threshold}-of-${result.consensus!.totalKeys} consensus configured`
              : "2-of-2 key sharing configured"}
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
              <label className="text-sm text-gray-400">Backup Server</label>
              <div className="font-mono bg-gray-900 rounded px-3 py-2 break-all">
                {result.repoUrl}
              </div>
            </div>

            {isConsensus && (
              <>
                <div>
                  <label className="text-sm text-gray-400">Your Key ID</label>
                  <div className="font-mono bg-gray-900 rounded px-3 py-2">
                    {result.keyId}
                  </div>
                </div>
                <div>
                  <label className="text-sm text-gray-400">Consensus</label>
                  <div className="font-mono bg-gray-900 rounded px-3 py-2">
                    {result.consensus!.threshold}-of-{result.consensus!.totalKeys}
                  </div>
                </div>
              </>
            )}

            {!isConsensus && (
              <div>
                <label className="text-sm text-gray-400">
                  Your Share (Index {result.shareIndex})
                </label>
                <div className="font-mono bg-gray-900 rounded px-3 py-2 break-all text-sm">
                  {result.localShare}
                </div>
              </div>
            )}

            {result.backupPaths && result.backupPaths.length > 0 && (
              <div>
                <label className="text-sm text-gray-400">Backup Paths</label>
                <div className="space-y-1">
                  {result.backupPaths.map((path) => (
                    <div
                      key={path}
                      className="font-mono bg-gray-900 rounded px-3 py-2 text-sm"
                    >
                      {path}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Sharing instructions */}
        {isConsensus && result.consensus!.totalKeys > 1 && (
          <Alert variant="warning" className="mb-6">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              Invite Key Holders
            </h2>
            <p className="text-sm text-gray-300 mb-4">
              You need {result.consensus!.totalKeys - 1} more key holder(s) to
              complete your {result.consensus!.threshold}-of-
              {result.consensus!.totalKeys} setup.
            </p>
            <p className="text-sm text-gray-400 mb-2">
              Key holders can join by running:
            </p>
            <CopyableField
              value={`airgapper join --name THEIR_NAME --repo "${result.repoUrl}" --consensus`}
              id="consensusCommand"
              externalCopied={copiedId === "consensusCommand"}
              onCopy={() => copy(`airgapper join --name THEIR_NAME --repo "${result.repoUrl}" --consensus`, "consensusCommand")}
            />
          </Alert>
        )}

        {!isConsensus && result.peerShare && (
          <Alert variant="warning" className="mb-6">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              Share with Your Peer
            </h2>
            <p className="text-sm text-gray-300 mb-4">
              Give this information to your trusted backup host.
            </p>

            <div className="space-y-3">
              <CopyableField
                label={`Peer Share (Index ${result.peerShareIndex})`}
                value={result.peerShare}
                id="peerShare"
                externalCopied={copiedId === "peerShare"}
                onCopy={() => copy(result.peerShare!, "peerShare")}
              />

              <CopyableField
                label="Join Command"
                value={`airgapper join --name PEER_NAME --repo "${result.repoUrl}" --share ${result.peerShare} --index ${result.peerShareIndex}`}
                id="command"
                externalCopied={copiedId === "command"}
                onCopy={() => copy(`airgapper join --name PEER_NAME --repo "${result.repoUrl}" --share ${result.peerShare} --index ${result.peerShareIndex}`, "command")}
              />
            </div>
          </Alert>
        )}

        <div className="flex gap-4">
          <button
            onClick={() => {
              setResult(null);
              setCurrentStep("paths");
            }}
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

  // Step indicator
  const steps = ["What to Back Up", "Storage", "Security"];
  const stepIndex = ["paths", "destination", "security"].indexOf(currentStep);

  return (
    <div className="max-w-xl mx-auto">
      <button
        onClick={() => {
          if (currentStep === "paths") {
            onNavigate("welcome");
          } else if (currentStep === "destination") {
            setCurrentStep("paths");
          } else if (currentStep === "security") {
            setCurrentStep("destination");
          }
        }}
        className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
      >
        &larr; Back
      </button>

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">🔐</div>
        <h1 className="text-2xl font-bold">Initialize New Vault</h1>
        <p className="text-gray-400">Set up as the data owner</p>
      </div>

      {/* Step indicator */}
      <StepIndicator
        steps={steps.map(label => ({ label }))}
        currentStep={stepIndex}
        showLabels={false}
        className="justify-center mb-8"
      />

      {/* Step 1: What to Back Up */}
      {currentStep === "paths" && (
        <div className="bg-gray-800 rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-2">What do you want to back up?</h3>
          <p className="text-sm text-gray-400 mb-6">
            Select the folders and files you want to protect.
          </p>

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
              <p className="text-xs text-gray-500 mt-1">
                This identifies you in the backup system
              </p>
            </div>

            {/* Selected paths */}
            {backupPaths.length > 0 && (
              <div>
                <label className="block text-sm font-medium mb-2">
                  Selected ({backupPaths.length})
                </label>
                <div className="space-y-2">
                  {backupPaths.map((path) => (
                    <div
                      key={path}
                      className="flex items-center justify-between bg-gray-700 rounded px-3 py-2"
                    >
                      <span className="font-mono text-sm truncate">{path}</span>
                      <button
                        onClick={() => removePath(path)}
                        className="text-gray-400 hover:text-red-400 ml-2"
                      >
                        &times;
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}

            <div>
              <label className="block text-sm font-medium mb-2">Add Backup Path</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newPath}
                  onChange={(e) => setNewPath(e.target.value)}
                  placeholder="/path/to/backup"
                  className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono text-sm"
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && newPath.trim()) {
                      addPath(newPath);
                      setNewPath("");
                    }
                  }}
                />
                <button
                  onClick={() => {
                    if (newPath.trim()) {
                      addPath(newPath);
                      setNewPath("");
                    }
                  }}
                  disabled={!newPath.trim()}
                  className="px-4 py-3 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-800 disabled:cursor-not-allowed rounded-lg transition-colors"
                >
                  Add
                </button>
              </div>
            </div>
          </div>

          <div className="mt-6 pt-6 border-t border-gray-700">
            <button
              onClick={handlePathsStepNext}
              disabled={!name.trim() || backupPaths.length === 0}
              className="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors"
            >
              Next: Storage Destination &rarr;
            </button>
            {backupPaths.length === 0 && (
              <p className="text-xs text-yellow-500 mt-2 text-center">
                Please select at least one folder to back up
              </p>
            )}
          </div>
        </div>
      )}

      {/* Step 2: Storage Destination */}
      {currentStep === "destination" && (
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
                onChange={(e) => setRepoUrl(e.target.value)}
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
              onClick={handleDestinationNext}
              disabled={!repoUrl.trim()}
              className="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors"
            >
              Next: Security Settings &rarr;
            </button>
          </div>
        </div>
      )}

      {/* Step 3: Security Settings */}
      {currentStep === "security" && (
        <div className="bg-gray-800 rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-2">Security Settings</h3>
          <p className="text-sm text-gray-400 mb-6">
            Configure how restore access is controlled.
          </p>

          <label className="flex items-center gap-2 text-sm text-gray-400 mb-4">
            <input
              type="checkbox"
              checked={useConsensusMode}
              onChange={(e) => setUseConsensusMode(e.target.checked)}
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
              onClick={handleSecurityNext}
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
      )}

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
