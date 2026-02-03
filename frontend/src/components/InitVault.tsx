import { useState, useCallback } from "react";
import type { VaultConfig, Step } from "../types";
import { split, generatePassword, toHex } from "../lib/sss";
import { generateKeyPair, keyId } from "../lib/crypto";
import { useClipboard } from "../hooks/useClipboard";
import { StepIndicator } from "./ui";
import {
  InitVaultPaths,
  InitVaultDestination,
  InitVaultSecurity,
  InitVaultComplete,
} from "./init-vault";
import type { InitStep, InitVaultState } from "./init-vault";

interface InitVaultProps {
  onComplete: (config: VaultConfig) => void;
  onNavigate: (step: Step) => void;
}

const STEPS = ["What to Back Up", "Storage", "Security"];

export function InitVault({ onComplete, onNavigate }: InitVaultProps) {
  const [currentStep, setCurrentStep] = useState<InitStep>("paths");
  const [isGenerating, setIsGenerating] = useState(false);
  const [result, setResult] = useState<VaultConfig | null>(null);
  const { copiedId, copy } = useClipboard();

  const [state, setState] = useState<InitVaultState>({
    name: "",
    repoUrl: "",
    threshold: 2,
    totalKeys: 2,
    requireApproval: true,
    useConsensusMode: true,
    backupPaths: [],
    newPath: "",
  });

  const updateState = useCallback(
    <K extends keyof InitVaultState>(key: K, value: InitVaultState[K]) => {
      setState((prev) => ({ ...prev, [key]: value }));
    },
    []
  );

  const addPath = useCallback((path: string) => {
    if (path.trim()) {
      setState((prev) => {
        if (prev.backupPaths.includes(path.trim())) return prev;
        return { ...prev, backupPaths: [...prev.backupPaths, path.trim()] };
      });
    }
  }, []);

  const removePath = useCallback((path: string) => {
    setState((prev) => ({
      ...prev,
      backupPaths: prev.backupPaths.filter((p) => p !== path),
    }));
  }, []);

  const handleGenerate = async () => {
    setIsGenerating(true);
    await new Promise((r) => setTimeout(r, 500));

    try {
      let config: VaultConfig;
      const { name, repoUrl, threshold, totalKeys, requireApproval, useConsensusMode, backupPaths } = state;

      if (useConsensusMode) {
        const keys = await generateKeyPair();
        const id = await keyId(keys.publicKey);
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
            keyHolders: [{ id, name: name.trim(), publicKey: keys.publicKey, isOwner: true }],
            requireApproval,
          },
          backupPaths,
        };
      } else {
        const password = generatePassword();
        const passwordHex = toHex(password);
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

  const handleStartOver = () => {
    setResult(null);
    setCurrentStep("paths");
  };

  const handleSave = () => {
    if (result) onComplete(result);
  };

  const stepIndex = ["paths", "destination", "security"].indexOf(currentStep);

  const renderStepIndicator = useCallback(
    () => (
      <StepIndicator
        steps={STEPS.map((label) => ({ label }))}
        currentStep={stepIndex}
        showLabels={false}
        className="justify-center mb-8"
      />
    ),
    [stepIndex]
  );

  if (currentStep === "result" && result) {
    return (
      <InitVaultComplete
        result={result}
        copiedId={copiedId}
        onCopy={copy}
        onStartOver={handleStartOver}
        onSave={handleSave}
      />
    );
  }

  if (currentStep === "paths") {
    return (
      <InitVaultPaths
        state={state}
        updateState={updateState}
        onNext={() => setCurrentStep("destination")}
        onNavigate={onNavigate}
        addPath={addPath}
        removePath={removePath}
        renderStepIndicator={renderStepIndicator}
      />
    );
  }

  if (currentStep === "destination") {
    return (
      <InitVaultDestination
        state={state}
        updateState={updateState}
        onNext={() => setCurrentStep("security")}
        onBack={() => setCurrentStep("paths")}
        renderStepIndicator={renderStepIndicator}
      />
    );
  }

  return (
    <InitVaultSecurity
      state={state}
      updateState={updateState}
      onNext={() => {}}
      onBack={() => setCurrentStep("destination")}
      isGenerating={isGenerating}
      onGenerate={handleGenerate}
      renderStepIndicator={renderStepIndicator}
    />
  );
}
