import type { JoinMode } from "./types";

interface ModeSelectorProps {
  mode: JoinMode;
  onModeChange: (mode: JoinMode) => void;
}

export function ModeSelector({ mode, onModeChange }: ModeSelectorProps) {
  return (
    <div className="flex gap-2 mb-6">
      <button
        onClick={() => onModeChange("consensus")}
        className={`flex-1 py-2 px-4 rounded-lg transition-colors ${
          mode === "consensus"
            ? "bg-blue-600 text-white"
            : "bg-gray-700 text-gray-300 hover:bg-gray-600"
        }`}
      >
        Consensus Mode
      </button>
      <button
        onClick={() => onModeChange("sss")}
        className={`flex-1 py-2 px-4 rounded-lg transition-colors ${
          mode === "sss"
            ? "bg-blue-600 text-white"
            : "bg-gray-700 text-gray-300 hover:bg-gray-600"
        }`}
      >
        SSS Mode (Legacy)
      </button>
    </div>
  );
}
