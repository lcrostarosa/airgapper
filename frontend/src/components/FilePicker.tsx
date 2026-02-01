import { useState, useEffect } from "react";
import type { FilesystemEntry } from "../types";
import { browseFilesystem } from "../lib/api";

interface FilePickerProps {
  isOpen: boolean;
  onClose: () => void;
  onSelect: (paths: string[]) => void;
  selectionMode?: "single" | "multiple";
  allowFolders?: boolean;
  allowFiles?: boolean;
  initialPath?: string;
}

export function FilePicker({
  isOpen,
  onClose,
  onSelect,
  selectionMode = "multiple",
  allowFolders = true,
  allowFiles = true,
  initialPath,
}: FilePickerProps) {
  const [currentPath, setCurrentPath] = useState(initialPath || "");
  const [entries, setEntries] = useState<FilesystemEntry[]>([]);
  const [parentPath, setParentPath] = useState<string | undefined>();
  const [selectedPaths, setSelectedPaths] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showHidden, setShowHidden] = useState(false);

  useEffect(() => {
    if (isOpen) {
      loadDirectory(currentPath);
    }
  }, [isOpen, currentPath, showHidden]);

  const loadDirectory = async (path: string) => {
    setLoading(true);
    setError(null);
    try {
      const response = await browseFilesystem(path || undefined, showHidden);
      setEntries(response.entries);
      setCurrentPath(response.path);
      setParentPath(response.parent);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const navigateTo = (path: string) => {
    setCurrentPath(path);
  };

  const toggleSelection = (entry: FilesystemEntry) => {
    // Check if selection is allowed for this type
    if (entry.isDir && !allowFolders) return;
    if (!entry.isDir && !allowFiles) return;

    const newSelected = new Set(selectedPaths);
    if (newSelected.has(entry.path)) {
      newSelected.delete(entry.path);
    } else {
      if (selectionMode === "single") {
        newSelected.clear();
      }
      newSelected.add(entry.path);
    }
    setSelectedPaths(newSelected);
  };

  const handleConfirm = () => {
    onSelect(Array.from(selectedPaths));
    setSelectedPaths(new Set());
    onClose();
  };

  const formatSize = (bytes?: number) => {
    if (!bytes) return "";
    const units = ["B", "KB", "MB", "GB"];
    let size = bytes;
    let unitIndex = 0;
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024;
      unitIndex++;
    }
    return `${size.toFixed(1)} ${units[unitIndex]}`;
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleDateString() + " " + date.toLocaleTimeString();
  };

  // Get breadcrumb parts
  const pathParts = currentPath.split("/").filter(Boolean);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-gray-800 rounded-lg w-full max-w-3xl max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="p-4 border-b border-gray-700">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-lg font-semibold">Select Files/Folders</h2>
            <button
              onClick={onClose}
              className="text-gray-400 hover:text-white transition-colors"
            >
              &times;
            </button>
          </div>

          {/* Breadcrumb navigation */}
          <div className="flex items-center gap-1 text-sm overflow-x-auto">
            <button
              onClick={() => navigateTo("/")}
              className="px-2 py-1 hover:bg-gray-700 rounded transition-colors"
            >
              /
            </button>
            {pathParts.map((part, index) => {
              const path = "/" + pathParts.slice(0, index + 1).join("/");
              return (
                <span key={path} className="flex items-center gap-1">
                  <span className="text-gray-500">/</span>
                  <button
                    onClick={() => navigateTo(path)}
                    className="px-2 py-1 hover:bg-gray-700 rounded transition-colors truncate max-w-[150px]"
                    title={part}
                  >
                    {part}
                  </button>
                </span>
              );
            })}
          </div>
        </div>

        {/* Toolbar */}
        <div className="px-4 py-2 border-b border-gray-700 flex items-center justify-between">
          <div className="flex items-center gap-2">
            {parentPath && (
              <button
                onClick={() => navigateTo(parentPath)}
                className="px-3 py-1 bg-gray-700 hover:bg-gray-600 rounded text-sm transition-colors"
              >
                &larr; Up
              </button>
            )}
          </div>
          <label className="flex items-center gap-2 text-sm text-gray-400">
            <input
              type="checkbox"
              checked={showHidden}
              onChange={(e) => setShowHidden(e.target.checked)}
              className="rounded bg-gray-700"
            />
            Show hidden files
          </label>
        </div>

        {/* File list */}
        <div className="flex-1 overflow-y-auto p-4">
          {loading ? (
            <div className="text-center text-gray-400 py-8">Loading...</div>
          ) : error ? (
            <div className="text-center text-red-400 py-8">{error}</div>
          ) : entries.length === 0 ? (
            <div className="text-center text-gray-400 py-8">
              Empty directory
            </div>
          ) : (
            <div className="space-y-1">
              {entries.map((entry) => {
                const isSelected = selectedPaths.has(entry.path);
                const isSelectable =
                  (entry.isDir && allowFolders) ||
                  (!entry.isDir && allowFiles);

                return (
                  <div
                    key={entry.path}
                    className={`flex items-center gap-3 p-2 rounded transition-colors ${
                      isSelected
                        ? "bg-blue-900/50 border border-blue-500"
                        : "hover:bg-gray-700 border border-transparent"
                    } ${!isSelectable ? "opacity-50" : "cursor-pointer"}`}
                    onClick={() => {
                      if (entry.isDir && !isSelected) {
                        // Double-click behavior: single click to select folder, or navigate
                        // For simplicity, clicking a folder navigates into it unless it's selected
                      }
                      if (isSelectable) {
                        toggleSelection(entry);
                      }
                    }}
                    onDoubleClick={() => {
                      if (entry.isDir) {
                        navigateTo(entry.path);
                      }
                    }}
                  >
                    {/* Checkbox */}
                    {isSelectable && (
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => toggleSelection(entry)}
                        className="rounded bg-gray-700"
                        onClick={(e) => e.stopPropagation()}
                      />
                    )}
                    {!isSelectable && <div className="w-4" />}

                    {/* Icon */}
                    <span className="text-xl">
                      {entry.isDir ? "📁" : "📄"}
                    </span>

                    {/* Name */}
                    <div className="flex-1 min-w-0">
                      <div className="truncate font-medium">{entry.name}</div>
                      <div className="text-xs text-gray-500 truncate">
                        {formatDate(entry.modTime)}
                      </div>
                    </div>

                    {/* Size */}
                    {!entry.isDir && (
                      <div className="text-sm text-gray-400">
                        {formatSize(entry.size)}
                      </div>
                    )}

                    {/* Navigate button for folders */}
                    {entry.isDir && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          navigateTo(entry.path);
                        }}
                        className="px-2 py-1 text-sm text-gray-400 hover:text-white hover:bg-gray-600 rounded transition-colors"
                      >
                        Open &rarr;
                      </button>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-gray-700">
          <div className="flex items-center justify-between">
            <div className="text-sm text-gray-400">
              {selectedPaths.size === 0
                ? "No items selected"
                : `${selectedPaths.size} item${
                    selectedPaths.size > 1 ? "s" : ""
                  } selected`}
            </div>
            <div className="flex gap-3">
              <button
                onClick={onClose}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleConfirm}
                disabled={selectedPaths.size === 0}
                className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed rounded transition-colors"
              >
                Select ({selectedPaths.size})
              </button>
            </div>
          </div>

          {/* Selected items preview */}
          {selectedPaths.size > 0 && (
            <div className="mt-3 text-sm">
              <div className="text-gray-400 mb-1">Selected:</div>
              <div className="flex flex-wrap gap-2">
                {Array.from(selectedPaths).map((path) => (
                  <span
                    key={path}
                    className="px-2 py-1 bg-gray-700 rounded flex items-center gap-1"
                  >
                    <span className="truncate max-w-[200px]" title={path}>
                      {path.split("/").pop()}
                    </span>
                    <button
                      onClick={() => {
                        const newSelected = new Set(selectedPaths);
                        newSelected.delete(path);
                        setSelectedPaths(newSelected);
                      }}
                      className="text-gray-400 hover:text-red-400"
                    >
                      &times;
                    </button>
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
