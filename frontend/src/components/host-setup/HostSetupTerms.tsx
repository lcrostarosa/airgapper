import type { StepProps, RestoreApprovalMode, DeletionMode } from "./types";

export function HostSetupTerms({
  state,
  updateState,
  onNext,
  onBack,
  renderStepIndicator,
}: StepProps) {
  return (
    <div className="max-w-xl mx-auto">
      <button
        onClick={() => onBack("storage")}
        className="mb-6 text-gray-400 hover:text-white transition-colors flex items-center gap-2"
      >
        &larr; Back
      </button>

      {renderStepIndicator()}

      <div className="text-center mb-8">
        <div className="text-4xl mb-2">📜</div>
        <h1 className="text-2xl font-bold">Backup Terms</h1>
        <p className="text-gray-400 mt-2">
          Define the rules for this backup arrangement
        </p>
      </div>

      <div className="bg-gray-800 rounded-lg p-6 mb-6">
        <h3 className="font-medium mb-4">Storage Protection</h3>

        <label className="flex items-start gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer mb-4">
          <input
            type="checkbox"
            checked={state.appendOnly}
            onChange={(e) => updateState("appendOnly", e.target.checked)}
            className="mt-1 rounded bg-gray-700"
          />
          <div>
            <div className="font-medium">Append-only mode</div>
            <div className="text-sm text-gray-400">
              Backups cannot be deleted or modified by anyone. This protects
              against ransomware and accidental deletion.
            </div>
          </div>
        </label>

        <div className="mb-4">
          <label className="block text-sm font-medium mb-2">
            Minimum retention period (optional)
          </label>
          <div className="flex gap-2 items-center">
            <input
              type="number"
              value={state.retentionDays}
              onChange={(e) => updateState("retentionDays", e.target.value)}
              placeholder="e.g., 90"
              min="1"
              className="w-32 bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 focus:outline-none focus:border-blue-500 transition-colors"
            />
            <span className="text-gray-400">days</span>
          </div>
          <p className="text-xs text-gray-500 mt-1">
            How long backups must be kept before they can be pruned
          </p>
        </div>
      </div>

      <RestoreApprovalSection
        value={state.restoreApproval}
        onChange={(value) => updateState("restoreApproval", value)}
      />

      <DeletionPolicySection
        value={state.deletionMode}
        onChange={(value) => updateState("deletionMode", value)}
      />

      <div className="bg-blue-900/30 border border-blue-600/50 rounded-lg p-4 mb-6">
        <h3 className="font-medium text-blue-400 mb-2">
          These terms are binding
        </h3>
        <p className="text-sm text-gray-300">
          Once the data owner accepts these terms, they cannot be changed
          without mutual agreement. This contract will be cryptographically
          signed by both parties.
        </p>
      </div>

      <div className="flex gap-4">
        <button
          onClick={() => onBack("storage")}
          className="flex-1 bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Back
        </button>
        <button
          onClick={() => onNext("initialize")}
          className="flex-1 bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          Next: Initialize
        </button>
      </div>
    </div>
  );
}

interface RestoreApprovalSectionProps {
  value: RestoreApprovalMode;
  onChange: (value: RestoreApprovalMode) => void;
}

function RestoreApprovalSection({ value, onChange }: RestoreApprovalSectionProps) {
  const options: { value: RestoreApprovalMode; label: string; description: string }[] = [
    { value: "both-required", label: "Both must approve", description: "Owner AND host must both approve any restore" },
    { value: "either", label: "Either can approve", description: "Owner OR host can independently approve" },
    { value: "owner-only", label: "Owner only", description: "Only the data owner can initiate restores" },
    { value: "host-only", label: "Host only", description: "Only the backup host can initiate restores" },
  ];

  return (
    <div className="bg-gray-800 rounded-lg p-6 mb-6">
      <h3 className="font-medium mb-4">Restore Approval</h3>
      <p className="text-sm text-gray-400 mb-4">
        Who must approve before backups can be restored?
      </p>

      <div className="space-y-2">
        {options.map((opt) => (
          <label
            key={opt.value}
            className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer"
          >
            <input
              type="radio"
              name="restoreApproval"
              value={opt.value}
              checked={value === opt.value}
              onChange={() => onChange(opt.value)}
              className="bg-gray-700"
            />
            <div>
              <div className="font-medium">{opt.label}</div>
              <div className="text-sm text-gray-400">{opt.description}</div>
            </div>
          </label>
        ))}
      </div>
    </div>
  );
}

interface DeletionPolicySectionProps {
  value: DeletionMode;
  onChange: (value: DeletionMode) => void;
}

function DeletionPolicySection({ value, onChange }: DeletionPolicySectionProps) {
  const options: { value: DeletionMode; label: string; description: string }[] = [
    { value: "both-required", label: "Both must approve", description: "Owner AND host must approve deletion requests" },
    { value: "owner-only", label: "Owner only", description: "Only the data owner can authorize deletion" },
    { value: "time-lock-only", label: "Time-lock only", description: "Automatic deletion after retention period (no approval needed)" },
    { value: "never", label: "Never delete (archival)", description: "Data is kept forever, cannot be deleted" },
  ];

  return (
    <div className="bg-gray-800 rounded-lg p-6 mb-6">
      <h3 className="font-medium mb-4">Deletion Policy</h3>
      <p className="text-sm text-gray-400 mb-4">
        Who can authorize deletion of backup data (after retention period)?
      </p>

      <div className="space-y-2">
        {options.map((opt) => (
          <label
            key={opt.value}
            className="flex items-center gap-3 p-3 bg-gray-900 rounded-lg cursor-pointer"
          >
            <input
              type="radio"
              name="deletionMode"
              value={opt.value}
              checked={value === opt.value}
              onChange={() => onChange(opt.value)}
              className="bg-gray-700"
            />
            <div>
              <div className="font-medium">{opt.label}</div>
              <div className="text-sm text-gray-400">{opt.description}</div>
            </div>
          </label>
        ))}
      </div>
    </div>
  );
}
