import type { IntroStepProps } from "./types";

export function HostSetupIntro({
  onNext,
  onNavigate,
  renderStepIndicator,
}: IntroStepProps) {
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
        onClick={() => onNext("storage")}
        className="w-full bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
      >
        Get Started
      </button>
    </div>
  );
}
