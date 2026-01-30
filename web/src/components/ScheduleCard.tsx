import type { SchedulerStatus } from '../types';

interface ScheduleCardProps {
  schedule: SchedulerStatus | undefined;
  onTriggerBackup: () => void;
}

export function ScheduleCard({ schedule, onTriggerBackup }: ScheduleCardProps) {
  const formatDate = (dateStr: string | undefined) => {
    if (!dateStr) return 'N/A';
    return new Date(dateStr).toLocaleString();
  };

  const getTimeUntil = (dateStr: string | undefined) => {
    if (!dateStr) return '';
    const diff = new Date(dateStr).getTime() - Date.now();
    if (diff < 0) return '(overdue)';
    const hours = Math.floor(diff / (1000 * 60 * 60));
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
    if (hours > 24) {
      const days = Math.floor(hours / 24);
      return `(in ${days}d ${hours % 24}h)`;
    }
    return `(in ${hours}h ${minutes}m)`;
  };

  if (!schedule?.enabled) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Backup Schedule</h2>
        <div className="text-center py-6 text-gray-500">
          <svg
            className="w-12 h-12 mx-auto mb-3 text-gray-300"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <p>No schedule configured</p>
          <p className="text-sm mt-2">
            Configure with: <code className="bg-gray-100 px-1 rounded">airgapper schedule --set daily ~/Documents</code>
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-gray-900">Backup Schedule</h2>
        <span className="px-2 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800">
          ACTIVE
        </span>
      </div>

      <div className="space-y-3 mb-4">
        <div className="flex justify-between">
          <span className="text-gray-500">Schedule</span>
          <span className="font-mono text-sm">{schedule.schedule}</span>
        </div>

        {schedule.paths && schedule.paths.length > 0 && (
          <div className="flex justify-between">
            <span className="text-gray-500">Paths</span>
            <span className="font-mono text-sm text-right max-w-[200px] truncate" title={schedule.paths.join(', ')}>
              {schedule.paths.length} path{schedule.paths.length > 1 ? 's' : ''}
            </span>
          </div>
        )}

        {schedule.last_run && (
          <div className="flex justify-between">
            <span className="text-gray-500">Last Run</span>
            <span className="text-sm">
              {formatDate(schedule.last_run)}
              {schedule.last_error && (
                <span className="text-red-500 ml-1" title={schedule.last_error}>
                  ⚠️
                </span>
              )}
            </span>
          </div>
        )}

        {schedule.next_run && (
          <div className="flex justify-between">
            <span className="text-gray-500">Next Run</span>
            <span className="text-sm">
              {formatDate(schedule.next_run)}{' '}
              <span className="text-gray-400">{getTimeUntil(schedule.next_run)}</span>
            </span>
          </div>
        )}
      </div>

      {schedule.last_error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
          <span className="font-medium">Last error:</span> {schedule.last_error}
        </div>
      )}

      <button
        onClick={onTriggerBackup}
        className="w-full bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-md text-sm font-medium transition-colors"
      >
        📦 Trigger Manual Backup
      </button>
    </div>
  );
}
