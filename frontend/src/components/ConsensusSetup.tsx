import { useState } from "react";
import type { ConsensusPreset } from "../types";

interface ConsensusSetupProps {
  onSelect: (threshold: number, totalKeys: number, requireApproval: boolean) => void;
  initialThreshold?: number;
  initialTotalKeys?: number;
}

interface PresetInfo {
  id: ConsensusPreset;
  name: string;
  threshold: number;
  totalKeys: number;
  description: string;
  icon: string;
}

const presets: PresetInfo[] = [
  {
    id: "solo",
    name: "Solo (1/1)",
    threshold: 1,
    totalKeys: 1,
    description: "Only you control backups. Restore anytime without approval.",
    icon: "👤",
  },
  {
    id: "dual",
    name: "Dual Control (2/2)",
    threshold: 2,
    totalKeys: 2,
    description:
      "Two parties required. Both must approve to restore data.",
    icon: "👥",
  },
  {
    id: "twoOfThree",
    name: "Two of Three (2/3)",
    threshold: 2,
    totalKeys: 3,
    description:
      "Any 2 of 3 key holders can approve. Protects against one party being unavailable.",
    icon: "👥+",
  },
  {
    id: "custom",
    name: "Custom (m/n)",
    threshold: 0,
    totalKeys: 0,
    description: "Choose your own threshold and number of key holders.",
    icon: "⚙️",
  },
];

export function ConsensusSetup({
  onSelect,
  initialThreshold = 2,
  initialTotalKeys = 2,
}: ConsensusSetupProps) {
  const [selectedPreset, setSelectedPreset] = useState<ConsensusPreset | null>(
    null
  );
  const [customThreshold, setCustomThreshold] = useState(initialThreshold);
  const [customTotalKeys, setCustomTotalKeys] = useState(initialTotalKeys);
  const [requireApproval, setRequireApproval] = useState(true);

  const handlePresetSelect = (preset: PresetInfo) => {
    setSelectedPreset(preset.id);
    if (preset.id !== "custom") {
      // For solo mode, don't require approval by default
      const needsApproval = preset.id !== "solo";
      onSelect(preset.threshold, preset.totalKeys, needsApproval);
    }
  };

  const handleCustomConfirm = () => {
    if (customThreshold >= 1 && customTotalKeys >= customThreshold) {
      onSelect(customThreshold, customTotalKeys, requireApproval);
    }
  };

  return (
    <div className="space-y-6">
      <div className="text-center mb-4">
        <h3 className="text-lg font-semibold">Choose Consensus Model</h3>
        <p className="text-sm text-gray-400">
          How many approvals are needed to restore data?
        </p>
      </div>

      {/* Preset options */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {presets.map((preset) => (
          <button
            key={preset.id}
            onClick={() => handlePresetSelect(preset)}
            className={`p-4 rounded-lg text-left transition-all ${
              selectedPreset === preset.id
                ? "bg-blue-900/50 border-2 border-blue-500"
                : "bg-gray-700 border-2 border-transparent hover:border-gray-500"
            }`}
          >
            <div className="flex items-start gap-3">
              <span className="text-2xl">{preset.icon}</span>
              <div>
                <div className="font-semibold">{preset.name}</div>
                <div className="text-sm text-gray-400 mt-1">
                  {preset.description}
                </div>
              </div>
            </div>
          </button>
        ))}
      </div>

      {/* Custom configuration */}
      {selectedPreset === "custom" && (
        <div className="bg-gray-700 rounded-lg p-4 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium mb-2">
                Required Approvals (m)
              </label>
              <input
                type="number"
                min={1}
                max={customTotalKeys}
                value={customThreshold}
                onChange={(e) =>
                  setCustomThreshold(Math.max(1, parseInt(e.target.value) || 1))
                }
                className="w-full bg-gray-800 border border-gray-600 rounded px-3 py-2 focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-2">
                Total Key Holders (n)
              </label>
              <input
                type="number"
                min={customThreshold}
                max={10}
                value={customTotalKeys}
                onChange={(e) =>
                  setCustomTotalKeys(
                    Math.max(
                      customThreshold,
                      parseInt(e.target.value) || customThreshold
                    )
                  )
                }
                className="w-full bg-gray-800 border border-gray-600 rounded px-3 py-2 focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          {customThreshold === 1 && customTotalKeys === 1 && (
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={requireApproval}
                onChange={(e) => setRequireApproval(e.target.checked)}
                className="rounded bg-gray-800"
              />
              <span>
                Require explicit approval (creates audit trail even for solo
                mode)
              </span>
            </label>
          )}

          <div className="text-sm text-gray-400">
            Configuration:{" "}
            <span className="font-mono">
              {customThreshold}-of-{customTotalKeys}
            </span>
            {customThreshold === customTotalKeys
              ? " (all parties must approve)"
              : ` (any ${customThreshold} of ${customTotalKeys} must approve)`}
          </div>

          <button
            onClick={handleCustomConfirm}
            disabled={
              customThreshold < 1 || customTotalKeys < customThreshold
            }
            className="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed py-2 rounded transition-colors"
          >
            Confirm Configuration
          </button>
        </div>
      )}

      {/* Visual explanation */}
      {selectedPreset && selectedPreset !== "custom" && (
        <div className="bg-gray-700/50 rounded-lg p-4">
          <div className="text-sm text-gray-400 mb-2">How it works:</div>
          {selectedPreset === "solo" && (
            <ul className="text-sm space-y-1">
              <li className="flex items-center gap-2">
                <span className="text-green-400">✓</span>
                You can backup and restore anytime
              </li>
              <li className="flex items-center gap-2">
                <span className="text-green-400">✓</span>
                No external approval needed
              </li>
              <li className="flex items-center gap-2">
                <span className="text-yellow-400">!</span>
                Less protection if your keys are compromised
              </li>
            </ul>
          )}
          {selectedPreset === "dual" && (
            <ul className="text-sm space-y-1">
              <li className="flex items-center gap-2">
                <span className="text-green-400">✓</span>
                You can backup anytime
              </li>
              <li className="flex items-center gap-2">
                <span className="text-blue-400">→</span>
                Restore requires approval from your partner
              </li>
              <li className="flex items-center gap-2">
                <span className="text-green-400">✓</span>
                Protection against theft or coercion
              </li>
              <li className="flex items-center gap-2">
                <span className="text-yellow-400">!</span>
                Both parties must be available to restore
              </li>
            </ul>
          )}
          {selectedPreset === "twoOfThree" && (
            <ul className="text-sm space-y-1">
              <li className="flex items-center gap-2">
                <span className="text-green-400">✓</span>
                You can backup anytime
              </li>
              <li className="flex items-center gap-2">
                <span className="text-blue-400">→</span>
                Restore requires any 2 of 3 approvals
              </li>
              <li className="flex items-center gap-2">
                <span className="text-green-400">✓</span>
                One party can be unavailable and restore still works
              </li>
              <li className="flex items-center gap-2">
                <span className="text-green-400">✓</span>
                Good balance of security and availability
              </li>
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
