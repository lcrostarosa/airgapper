import { useState, useEffect } from "react";
import type { VaultConfig, VaultContract, Step } from "../types";
import { getLocalIP, initHost, type InitHostResponse } from "../lib/client";
import { useClipboard } from "../hooks/useClipboard";
import { Alert, CopyableField, StepIndicator } from "./ui";

interface HostSetupProps {
  onComplete: (config: VaultConfig) => void;
  onNavigate: (step: Step) => void;
}

type SetupStep = "intro" | "storage" | "terms" | "initialize" | "complete";

export function HostSetup({ onComplete, onNavigate }: HostSetupProps) {
  const [currentStep, setCurrentStep] = useState<SetupStep>("intro");
  const [storagePath, setStoragePath] = useState("");
  const [storageQuota, setStorageQuota] = useState("");
  const [storageQuotaUnit, setStorageQuotaUnit] = useState<"GB" | "TB">("GB");

  // Contract terms
  const [appendOnly, setAppendOnly] = useState(true);
  const [restoreApproval, setRestoreApproval] = useState<
    "owner-only" | "host-only" | "both-required" | "either"
  >("both-required");
  const [deletionMode, setDeletionMode] = useState<
    "both-required" | "owner-only" | "time-lock-only" | "never"
  >("both-required");
  const [retentionDays, setRetentionDays] = useState("30");
  const [localIP, setLocalIP] = useState("192.168.1.x");
  const [name, setName] = useState("");

  // Initialization state
  const [initResult, setInitResult] = useState<InitHostResponse | null>(null);
  const [error, setError] = useState("");
  const [isInitializing, setIsInitializing] = useState(false);
  const { copiedId, copy } = useClipboard();

  // Fetch local IP on mount
  useEffect(() => {
    getLocalIP()
      .then((ip) => setLocalIP(ip))
      .catch(() => {
        // Keep default if API fails
      });
  }, []);


  const handleInitialize = async () => {
    setError("");

    if (!name.trim()) {
      setError("Name is required");
      return;
    }

    if (!storagePath.trim()) {
      setError("Storage path is required");
      return;
    }

    setIsInitializing(true);

    try {
      // Convert quota to bytes (bigint for proto)
      let quotaBytes: bigint | undefined;
      if (storageQuota) {
        const quotaNum = parseFloat(storageQuota);
        if (!isNaN(quotaNum) && quotaNum > 0) {
          const bytes =
            storageQuotaUnit === "TB"
              ? quotaNum * 1024 * 1024 * 1024 * 1024
              : quotaNum * 1024 * 1024 * 1024;
          quotaBytes = BigInt(Math.floor(bytes));
        }
      }

      const result = await initHost({
        name: name.trim(),
        storagePath: storagePath.trim(),
        storageQuotaBytes: quotaBytes,
        appendOnly,
        restoreApproval,
        retentionDays: retentionDays ? parseInt(retentionDays) : 0,
      });

      setInitResult(result);
      setCurrentStep("complete");
    } catch (err) {
      setError("Failed to initialize: " + (err as Error).message);
    } finally {
      setIsInitializing(false);
    }
  };

  const handleComplete = () => {
    if (!initResult) return;

    // Convert quota to bytes
    let quotaBytes: number | undefined;
    if (storageQuota) {
      const quotaNum = parseFloat(storageQuota);
      if (!isNaN(quotaNum) && quotaNum > 0) {
        quotaBytes =
          storageQuotaUnit === "TB"
            ? quotaNum * 1024 * 1024 * 1024 * 1024
            : quotaNum * 1024 * 1024 * 1024;
      }
    }

    // Build contract terms (host's offer)
    const contract: VaultContract = {
      version: 1,
      createdAt: new Date().toISOString(),
      storageQuotaBytes: quotaBytes,
      appendOnly,
      retentionDays: retentionDays ? parseInt(retentionDays) : 30,
      deletionMode,
      restoreApproval,
      hostKeyId: initResult.keyId,
    };

    const config: VaultConfig = {
      name: name.trim(),
      role: "host",
      repoUrl: initResult.storageUrl,
      publicKey: initResult.publicKey,
      keyId: initResult.keyId,
      storagePath: storagePath || undefined,
      storageQuotaBytes: quotaBytes,
      contract,
    };

    onComplete(config);
  };

  const setupSteps = [
    { key: "intro", label: "Welcome" },
    { key: "storage", label: "Storage" },
    { key: "terms", label: "Terms" },
    { key: "initialize", label: "Setup" },
    { key: "complete", label: "Done" },
  ];

  const currentStepIndex = setupSteps.findIndex((s) => s.key === currentStep);

  const renderStepIndicator = () => (
    <StepIndicator
      steps={setupSteps}
      currentStep={currentStepIndex}
      showLabels={false}
      className="justify-center mb-8"
    />
  );

  // Step 1: Introduction
  if (currentStep === "intro") {
    return (
      <div className="max-w-xl mx-auto">
        <button
          onClick={() => onNavigate("welcome")}
          className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
        >
          &larr; Back
        </button>

        {renderStepIndicator()}

        <div className="text-center mb-8">
          <div className="text-4xl mb-2">🖥️</div>
          <h1 className="text-2xl font-bold">Host Someone's Backup</h1>
          <p className="text-gray-400 mt-2">
            You'll be storing encrypted backups for someone else
          </p>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 mb-6">
          <h2 className="text-lg font-semibold mb-4">How it works</h2>
          <ul className="space-y-3 text-gray-300">
            <li className="flex items-start gap-3">
              <span className="text-green-400 mt-0.5">✓</span>
              <span>
                Backups are <strong>encrypted</strong> before they reach you
              </span>
            </li>
            <li className="flex items-start gap-3">
              <span className="text-green-400 mt-0.5">✓</span>
              <span>
                Restore approval requirements are <strong>agreed upon</strong>{" "}
                by you and the data owner
              </span>
            </li>
            <li className="flex items-start gap-3">
              <span className="text-green-400 mt-0.5">✓</span>
              <span>
                You <strong>cannot read</strong> the backup data (it's
                encrypted)
              </span>
            </li>
            <li className="flex items-start gap-3">
              <span className="text-green-400 mt-0.5">✓</span>
              <span>
                Storage uses <strong>append-only</strong> mode to prevent
                deletion
              </span>
            </li>
          </ul>
        </div>

        <div className="bg-blue-900/30 border border-blue-600/50 rounded-lg p-4 mb-6">
          <h3 className="font-medium text-blue-400 mb-2">What you'll need</h3>
          <ul className="text-sm text-gray-300 space-y-1">
            <li>• Storage space for backups</li>
            <li>• A network connection the owner can reach</li>
          </ul>
        </div>

        <button
          onClick={() => setCurrentStep("storage")}
          className="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Get Started
        </button>
      </div>
    );
  }

  // Step 2: Storage Location
  if (currentStep === "storage") {
    return (
      <div className="max-w-xl mx-auto">
        <button
          onClick={() => setCurrentStep("intro")}
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
            value={storagePath}
            onChange={(e) => setStoragePath(e.target.value)}
            placeholder="/data/backups"
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
          />

          {storagePath && (
            <p className="text-sm text-gray-400 mt-2">
              Selected: <code className="text-green-400">{storagePath}</code>
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
              value={storageQuota}
              onChange={(e) => setStorageQuota(e.target.value)}
              placeholder="e.g., 100"
              min="1"
              className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
            />
            <select
              value={storageQuotaUnit}
              onChange={(e) =>
                setStorageQuotaUnit(e.target.value as "GB" | "TB")
              }
              className="bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
            >
              <option value="GB">GB</option>
              <option value="TB">TB</option>
            </select>
          </div>

          {storageQuota && (
            <p className="text-sm text-gray-400 mt-2">
              Quota:{" "}
              <code className="text-green-400">
                {storageQuota} {storageQuotaUnit}
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
            onClick={() => setCurrentStep("intro")}
            className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Back
          </button>
          <button
            onClick={() => setCurrentStep("terms")}
            disabled={!storagePath.trim()}
            className="flex-1 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Next: Set Terms
          </button>
        </div>
      </div>
    );
  }

  // Step 3: Contract Terms
  if (currentStep === "terms") {
    return (
      <div className="max-w-xl mx-auto">
        <button
          onClick={() => setCurrentStep("storage")}
          className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
        >
          &larr; Back
        </button>

        {renderStepIndicator()}

        <div className="text-center mb-8">
          <div className="text-4xl mb-2">📜</div>
          <h1 className="text-2xl font-bold">Backup Terms</h1>
          <p className="text-gray-400 mt-2">
            Define the rules for this backup arrangement
          </p>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 mb-6">
          <h3 className="font-medium mb-4">Storage Protection</h3>

          <label className="flex items-start gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer mb-4">
            <input
              type="checkbox"
              checked={appendOnly}
              onChange={(e) => setAppendOnly(e.target.checked)}
              className="mt-1 rounded bg-gray-700"
            />
            <div>
              <div className="font-medium">Append-only mode</div>
              <div className="text-sm text-gray-400">
                Backups cannot be deleted or modified by anyone. This protects
                against ransomware and accidental deletion.
              </div>
            </div>
          </label>

          <div className="mb-4">
            <label className="block text-sm font-medium mb-2">
              Minimum retention period (optional)
            </label>
            <div className="flex gap-2 items-center">
              <input
                type="number"
                value={retentionDays}
                onChange={(e) => setRetentionDays(e.target.value)}
                placeholder="e.g., 90"
                min="1"
                className="w-32 bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 focus:outline-none focus:border-blue-500 transition-colors"
              />
              <span className="text-gray-400">days</span>
            </div>
            <p className="text-xs text-gray-500 mt-1">
              How long backups must be kept before they can be pruned
            </p>
          </div>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 mb-6">
          <h3 className="font-medium mb-4">Restore Approval</h3>
          <p className="text-sm text-gray-400 mb-4">
            Who must approve before backups can be restored?
          </p>

          <div className="space-y-2">
            <label className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer">
              <input
                type="radio"
                name="restoreApproval"
                value="both-required"
                checked={restoreApproval === "both-required"}
                onChange={() => setRestoreApproval("both-required")}
                className="bg-gray-700"
              />
              <div>
                <div className="font-medium">Both must approve</div>
                <div className="text-sm text-gray-400">
                  Owner AND host must both approve any restore
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer">
              <input
                type="radio"
                name="restoreApproval"
                value="either"
                checked={restoreApproval === "either"}
                onChange={() => setRestoreApproval("either")}
                className="bg-gray-700"
              />
              <div>
                <div className="font-medium">Either can approve</div>
                <div className="text-sm text-gray-400">
                  Owner OR host can independently approve
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer">
              <input
                type="radio"
                name="restoreApproval"
                value="owner-only"
                checked={restoreApproval === "owner-only"}
                onChange={() => setRestoreApproval("owner-only")}
                className="bg-gray-700"
              />
              <div>
                <div className="font-medium">Owner only</div>
                <div className="text-sm text-gray-400">
                  Only the data owner can initiate restores
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer">
              <input
                type="radio"
                name="restoreApproval"
                value="host-only"
                checked={restoreApproval === "host-only"}
                onChange={() => setRestoreApproval("host-only")}
                className="bg-gray-700"
              />
              <div>
                <div className="font-medium">Host only</div>
                <div className="text-sm text-gray-400">
                  Only the backup host can initiate restores
                </div>
              </div>
            </label>
          </div>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 mb-6">
          <h3 className="font-medium mb-4">Deletion Policy</h3>
          <p className="text-sm text-gray-400 mb-4">
            Who can authorize deletion of backup data (after retention period)?
          </p>

          <div className="space-y-2">
            <label className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer">
              <input
                type="radio"
                name="deletionMode"
                value="both-required"
                checked={deletionMode === "both-required"}
                onChange={() => setDeletionMode("both-required")}
                className="bg-gray-700"
              />
              <div>
                <div className="font-medium">Both must approve</div>
                <div className="text-sm text-gray-400">
                  Owner AND host must approve deletion requests
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer">
              <input
                type="radio"
                name="deletionMode"
                value="owner-only"
                checked={deletionMode === "owner-only"}
                onChange={() => setDeletionMode("owner-only")}
                className="bg-gray-700"
              />
              <div>
                <div className="font-medium">Owner only</div>
                <div className="text-sm text-gray-400">
                  Only the data owner can authorize deletion
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer">
              <input
                type="radio"
                name="deletionMode"
                value="time-lock-only"
                checked={deletionMode === "time-lock-only"}
                onChange={() => setDeletionMode("time-lock-only")}
                className="bg-gray-700"
              />
              <div>
                <div className="font-medium">Time-lock only</div>
                <div className="text-sm text-gray-400">
                  Automatic deletion after retention period (no approval needed)
                </div>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer">
              <input
                type="radio"
                name="deletionMode"
                value="never"
                checked={deletionMode === "never"}
                onChange={() => setDeletionMode("never")}
                className="bg-gray-700"
              />
              <div>
                <div className="font-medium">Never delete (archival)</div>
                <div className="text-sm text-gray-400">
                  Data is kept forever, cannot be deleted
                </div>
              </div>
            </label>
          </div>
        </div>

        <div className="bg-blue-900/30 border border-blue-600/50 rounded-lg p-4 mb-6">
          <h3 className="font-medium text-blue-400 mb-2">
            These terms are binding
          </h3>
          <p className="text-sm text-gray-300">
            Once the data owner accepts these terms, they cannot be changed
            without mutual agreement. This contract will be cryptographically
            signed by both parties.
          </p>
        </div>

        <div className="flex gap-4">
          <button
            onClick={() => setCurrentStep("storage")}
            className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Back
          </button>
          <button
            onClick={() => setCurrentStep("initialize")}
            className="flex-1 bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Next: Initialize
          </button>
        </div>
      </div>
    );
  }

  // Step 4: Initialize
  if (currentStep === "initialize") {
    return (
      <div className="max-w-xl mx-auto">
        <button
          onClick={() => setCurrentStep("terms")}
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
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g., bob-backup-server"
                className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
              />
              <p className="text-xs text-gray-500 mt-1">
                This identifies you to the data owner
              </p>
            </div>
          </div>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 mb-6">
          <h3 className="font-medium mb-4">Summary</h3>
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-gray-400">Storage Path:</span>
              <code className="text-green-400">{storagePath}</code>
            </div>
            {storageQuota && (
              <div className="flex justify-between">
                <span className="text-gray-400">Storage Quota:</span>
                <span>{storageQuota} {storageQuotaUnit}</span>
              </div>
            )}
            <div className="flex justify-between">
              <span className="text-gray-400">Append-Only:</span>
              <span>{appendOnly ? "Yes" : "No"}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Restore Approval:</span>
              <span className="capitalize">{restoreApproval.replace("-", " ")}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400">Deletion Policy:</span>
              <span className="capitalize">{deletionMode.replace("-", " ")}</span>
            </div>
            {retentionDays && (
              <div className="flex justify-between">
                <span className="text-gray-400">Retention Period:</span>
                <span>{retentionDays} days</span>
              </div>
            )}
            <div className="flex justify-between">
              <span className="text-gray-400">Your IP:</span>
              <span>{localIP}</span>
            </div>
          </div>
        </div>

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
            onClick={() => setCurrentStep("terms")}
            className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Back
          </button>
          <button
            onClick={handleInitialize}
            disabled={!name.trim() || isInitializing}
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

  // Step 5: Complete
  if (currentStep === "complete" && initResult) {
    return (
      <div className="max-w-xl mx-auto">
        {renderStepIndicator()}

        <div className="text-center mb-8">
          <div className="text-4xl mb-2">✅</div>
          <h1 className="text-2xl font-bold">Setup Complete!</h1>
          <p className="text-gray-400 mt-2">
            Share your details with the data owner
          </p>
        </div>

        <div className="bg-green-900/30 border border-green-600/50 rounded-lg p-4 mb-6">
          <div className="flex items-center gap-2 text-green-400 mb-2">
            <span>✓</span>
            <span className="font-medium">Storage server is running</span>
          </div>
          <p className="text-sm text-gray-300">
            The backup server is now accepting connections at the URL below.
          </p>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 mb-6">
          <div className="space-y-4">
            <div>
              <label className="text-sm text-gray-400">Your Name</label>
              <div className="font-mono bg-gray-900 rounded px-3 py-2">
                {initResult.name}
              </div>
            </div>

            <div>
              <label className="text-sm text-gray-400">Your Key ID</label>
              <div className="font-mono bg-gray-900 rounded px-3 py-2 text-sm">
                {initResult.keyId}
              </div>
            </div>

            <CopyableField
              label="Storage URL (share this)"
              value={initResult.storageUrl}
              id="url"
              externalCopied={copiedId === "url"}
              onCopy={() => copy(initResult.storageUrl, "url")}
            />
          </div>
        </div>

        <Alert variant="warning" className="mb-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            Share with the Data Owner
          </h2>
          <p className="text-sm text-gray-300 mb-4">
            The data owner needs your public key to add you as a key holder.
          </p>

          <CopyableField
            label="Your Public Key"
            value={initResult.publicKey}
            id="pubkey"
            externalCopied={copiedId === "pubkey"}
            onCopy={() => copy(initResult.publicKey, "pubkey")}
          />
        </Alert>

        <div className="flex gap-4">
          <button
            onClick={() => {
              setInitResult(null);
              setCurrentStep("initialize");
            }}
            className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Start Over
          </button>
          <button
            onClick={handleComplete}
            className="flex-1 bg-green-600 hover:bg-green-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Continue to Dashboard
          </button>
        </div>
      </div>
    );
  }

  return null;
}
