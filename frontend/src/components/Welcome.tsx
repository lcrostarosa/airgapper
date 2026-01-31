import type { Step } from "../types";

interface WelcomeProps {
  onNavigate: (step: Step) => void;
  hasVault: boolean;
}

export function Welcome({ onNavigate, hasVault }: WelcomeProps) {
  return (
    <div className="max-w-2xl mx-auto text-center">
      <div className="mb-8">
        <div className="text-6xl mb-4">🔐</div>
        <h1 className="text-4xl font-bold mb-2">Airgapper</h1>
        <p className="text-gray-400 text-lg">
          Consensus-based encrypted backup system
        </p>
      </div>

      <div className="bg-gray-800 rounded-lg p-6 mb-8">
        <h2 className="text-xl font-semibold mb-4">How it works</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
          <div className="bg-gray-700 rounded p-4">
            <div className="text-2xl mb-2">🔑</div>
            <div className="font-medium mb-1">Split Keys</div>
            <div className="text-gray-400">
              Your backup password is split using Shamir's Secret Sharing
            </div>
          </div>
          <div className="bg-gray-700 rounded p-4">
            <div className="text-2xl mb-2">🤝</div>
            <div className="font-medium mb-1">Consensus</div>
            <div className="text-gray-400">
              Restoring requires approval from your trusted peer
            </div>
          </div>
          <div className="bg-gray-700 rounded p-4">
            <div className="text-2xl mb-2">🛡️</div>
            <div className="font-medium mb-1">Security</div>
            <div className="text-gray-400">
              No single party can access your backups alone
            </div>
          </div>
        </div>
      </div>

      {hasVault ? (
        <button
          onClick={() => onNavigate("dashboard")}
          className="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Open Dashboard
        </button>
      ) : (
        <div className="space-y-4">
          <button
            onClick={() => onNavigate("init")}
            className="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Initialize New Vault (Data Owner)
          </button>
          <button
            onClick={() => onNavigate("join")}
            className="w-full bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Join as Backup Host
          </button>
        </div>
      )}
    </div>
  );
}
