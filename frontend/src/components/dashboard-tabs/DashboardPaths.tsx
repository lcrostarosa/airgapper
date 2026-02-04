import { useState } from "react";
import type { PathsTabProps } from "./types";

export function DashboardPaths({ config, onAddPaths, onRemovePath }: PathsTabProps) {
  const [newPath, setNewPath] = useState("");

  const handleAdd = () => {
    if (newPath.trim()) {
      onAddPaths([newPath.trim()]);
      setNewPath("");
    }
  };

  return (
    <div className="bg-gray-800 rounded-lg p-6">
      <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
        <span>📁</span> Backup Paths
      </h2>
      <p className="text-sm text-gray-400 mb-4">
        Configure which folders to include in backups.
      </p>

      {/* Current paths */}
      {config.backupPaths && config.backupPaths.length > 0 ? (
        <div className="space-y-2 mb-4">
          {config.backupPaths.map((path) => (
            <div
              key={path}
              className="flex items-center justify-between bg-gray-700 rounded px-3 py-2"
            >
              <span className="font-mono text-sm truncate">{path}</span>
              <button
                onClick={() => onRemovePath(path)}
                className="text-gray-400 hover:text-red-400 ml-2"
              >
                &times;
              </button>
            </div>
          ))}
        </div>
      ) : (
        <div className="text-center py-8 text-gray-400 bg-gray-700/50 rounded-lg mb-4">
          <div className="text-4xl mb-2">📂</div>
          <p>No backup paths configured</p>
        </div>
      )}

      <div className="flex gap-2">
        <input
          type="text"
          value={newPath}
          onChange={(e) => setNewPath(e.target.value)}
          placeholder="/path/to/backup"
          className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:border-blue-500 transition-colors font-mono text-sm"
          onKeyDown={(e) => {
            if (e.key === "Enter" && newPath.trim()) {
              handleAdd();
            }
          }}
        />
        <button
          onClick={handleAdd}
          disabled={!newPath.trim()}
          className="px-4 py-3 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-800 disabled:cursor-not-allowed rounded-lg transition-colors"
        >
          Add
        </button>
      </div>

      {config.backupPaths && config.backupPaths.length > 0 && (
        <div className="mt-4 pt-4 border-t border-gray-700">
          <div className="text-sm text-gray-400 mb-2">Run backup now:</div>
          <code className="block bg-gray-900 rounded px-3 py-2 font-mono text-sm">
            airgapper backup {config.backupPaths.join(" ")}
          </code>
        </div>
      )}
    </div>
  );
}
