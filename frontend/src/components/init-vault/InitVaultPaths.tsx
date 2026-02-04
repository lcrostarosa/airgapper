import type { PathsStepProps } from "./types";

export function InitVaultPaths({
  state,
  updateState,
  onNext,
  onNavigate,
  addPath,
  removePath,
  renderStepIndicator,
}: PathsStepProps) {
  const { name, backupPaths, newPath } = state;

  const handleNext = () => {
    if (!name.trim() || backupPaths.length === 0) return;
    onNext();
  };

  const handleAddPath = () => {
    if (newPath.trim()) {
      addPath(newPath);
      updateState("newPath", "");
    }
  };

  return (
    <div className="max-w-xl mx-auto">
      <button
        onClick={() => onNavigate("welcome")}
        className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
      >
        &larr; Back
      </button>

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">🔐</div>
        <h1 className="text-2xl font-bold">Initialize New Vault</h1>
        <p className="text-gray-400">Set up as the data owner</p>
      </div>

      {renderStepIndicator()}

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
              onChange={(e) => updateState("name", e.target.value)}
              placeholder="e.g., alice"
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors"
            />
            <p className="text-xs text-gray-500 mt-1">
              This identifies you in the backup system
            </p>
          </div>

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
                onChange={(e) => updateState("newPath", e.target.value)}
                placeholder="/path/to/backup"
                className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono text-sm"
                onKeyDown={(e) => {
                  if (e.key === "Enter" && newPath.trim()) {
                    handleAddPath();
                  }
                }}
              />
              <button
                onClick={handleAddPath}
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
            onClick={handleNext}
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
    </div>
  );
}
