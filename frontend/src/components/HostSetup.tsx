import { useState, useEffect, useCallback } from "react";
import type { VaultConfig, Step } from "../types";
import { getLocalIP, initHost, type InitHostResponse } from "../lib/client";
import { StepIndicator } from "./ui";
import {
  HostSetupIntro,
  HostSetupStorage,
  HostSetupTerms,
  HostSetupInitialize,
  HostSetupComplete,
  type SetupStep,
  type HostSetupState,
} from "./host-setup";
import { SETUP_STEPS, INITIAL_STATE, buildVaultConfig, calculateQuotaBytes } from "./host-setup/utils";

interface HostSetupProps {
  onComplete: (config: VaultConfig) => void;
  onNavigate: (step: Step) => void;
}

export function HostSetup({ onComplete, onNavigate }: HostSetupProps) {
  const [currentStep, setCurrentStep] = useState<SetupStep>("intro");
  const [state, setState] = useState<HostSetupState>(INITIAL_STATE);
  const [initResult, setInitResult] = useState<InitHostResponse | null>(null);
  const [error, setError] = useState("");
  const [isInitializing, setIsInitializing] = useState(false);

  const updateState = useCallback(<K extends keyof HostSetupState>(
    key: K,
    value: HostSetupState[K]
  ) => {
    setState((prev) => ({ ...prev, [key]: value }));
  }, []);

  useEffect(() => {
    getLocalIP()
      .then((ip) => updateState("localIP", ip))
      .catch(() => {});
  }, [updateState]);

  const handleInitialize = async () => {
    setError("");
    if (!state.name.trim()) {
      setError("Name is required");
      return;
    }
    if (!state.storagePath.trim()) {
      setError("Storage path is required");
      return;
    }

    setIsInitializing(true);
    try {
      const quotaBytes = calculateQuotaBytes(state.storageQuota, state.storageQuotaUnit);
      const result = await initHost({
        name: state.name.trim(),
        storagePath: state.storagePath.trim(),
        storageQuotaBytes: quotaBytes ? BigInt(Math.floor(quotaBytes)) : undefined,
        appendOnly: state.appendOnly,
        restoreApproval: state.restoreApproval,
        retentionDays: state.retentionDays ? parseInt(state.retentionDays) : 0,
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
    onComplete(buildVaultConfig(state, initResult));
  };

  const currentStepIndex = SETUP_STEPS.findIndex((s) => s.key === currentStep);
  const renderStepIndicator = () => (
    <StepIndicator
      steps={[...SETUP_STEPS]}
      currentStep={currentStepIndex}
      showLabels={false}
      className="justify-center mb-8"
    />
  );

  const stepProps = {
    state,
    updateState,
    onNext: setCurrentStep,
    onBack: setCurrentStep,
    renderStepIndicator,
  };

  switch (currentStep) {
    case "intro":
      return <HostSetupIntro {...stepProps} onNavigate={onNavigate} />;
    case "storage":
      return <HostSetupStorage {...stepProps} />;
    case "terms":
      return <HostSetupTerms {...stepProps} />;
    case "initialize":
      return (
        <HostSetupInitialize
          {...stepProps}
          error={error}
          isInitializing={isInitializing}
          onInitialize={handleInitialize}
        />
      );
    case "complete":
      if (!initResult) return null;
      return (
        <HostSetupComplete
          {...stepProps}
          initResult={initResult}
          onComplete={handleComplete}
          onStartOver={() => {
            setInitResult(null);
            setCurrentStep("initialize");
          }}
        />
      );
    default:
      return null;
  }
}
