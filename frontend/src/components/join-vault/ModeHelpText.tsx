import type { JoinMode } from "./types";

interface ModeHelpTextProps {
  mode: JoinMode;
}

export function ModeHelpText({ mode }: ModeHelpTextProps) {
  return (
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
  );
}
