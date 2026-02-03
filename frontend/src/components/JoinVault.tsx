import { useState } from "react";
import type { VaultConfig, Step } from "../types";
import { generateKeyPair, keyId } from "../lib/crypto";
import {
  JoinVaultConsensus,
  JoinVaultSSS,
  JoinVaultResult,
  ModeSelector,
  ModeHelpText,
  validateCommonFields,
  validateSSSFields,
  buildSSSConfig,
  buildConsensusConfig,
} from "./join-vault";
import type { JoinMode, GeneratedKeys, SSSFormData, CommonFormData } from "./join-vault";

interface JoinVaultProps {
  onComplete: (config: VaultConfig) => void;
  onNavigate: (step: Step) => void;
}

export function JoinVault({ onComplete, onNavigate }: JoinVaultProps) {
  const [mode, setMode] = useState<JoinMode>("consensus");
  const [formData, setFormData] = useState<SSSFormData>({
    name: "",
    repoUrl: "",
    shareHex: "",
    shareIndex: "2",
  });
  const [generatedKeys, setGeneratedKeys] = useState<GeneratedKeys | null>(null);
  const [error, setError] = useState("");
  const [isGenerating, setIsGenerating] = useState(false);

  const handleFieldChange = (field: keyof SSSFormData | keyof CommonFormData, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
  };

  const handleJoinSSS = () => {
    setError("");
    const result = validateSSSFields(formData);
    if (!result.valid) {
      setError(result.error!);
      return;
    }
    onComplete(buildSSSConfig(formData));
  };

  const handleJoinConsensus = async () => {
    setError("");
    const result = validateCommonFields(formData.name, formData.repoUrl);
    if (!result.valid) {
      setError(result.error!);
      return;
    }
    setIsGenerating(true);
    try {
      const keys = await generateKeyPair();
      const id = await keyId(keys.publicKey);
      setGeneratedKeys({ publicKey: keys.publicKey, privateKey: keys.privateKey, keyId: id });
    } catch (err) {
      setError("Failed to generate keys: " + (err as Error).message);
    } finally {
      setIsGenerating(false);
    }
  };

  const handleConfirmConsensus = () => {
    if (!generatedKeys) return;
    onComplete(buildConsensusConfig(formData, generatedKeys));
  };

  if (generatedKeys) {
    return (
      <JoinVaultResult
        name={formData.name}
        repoUrl={formData.repoUrl}
        generatedKeys={generatedKeys}
        onBack={() => setGeneratedKeys(null)}
        onConfirm={handleConfirmConsensus}
      />
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

      <ModeSelector mode={mode} onModeChange={setMode} />

      <div className="bg-gray-800 rounded-lg p-6">
        {mode === "consensus" ? (
          <JoinVaultConsensus
            formData={formData}
            error={error}
            isGenerating={isGenerating}
            onFieldChange={handleFieldChange}
            onSubmit={handleJoinConsensus}
          />
        ) : (
          <JoinVaultSSS
            formData={formData}
            error={error}
            onFieldChange={handleFieldChange}
            onSubmit={handleJoinSSS}
          />
        )}
      </div>

      <ModeHelpText mode={mode} />
    </div>
  );
}
