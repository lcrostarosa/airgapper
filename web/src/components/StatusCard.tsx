import type { Status } from '../types';

interface StatusCardProps {
  status: Status | null;
  loading: boolean;
  error: string | null;
}

export function StatusCard({ status, loading, error }: StatusCardProps) {
  if (loading) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <div className="animate-pulse">
          <div className="h-4 bg-gray-200 rounded w-1/4 mb-4"></div>
          <div className="h-8 bg-gray-200 rounded w-1/2"></div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-lg p-6">
        <h3 className="text-red-800 font-medium">Connection Error</h3>
        <p className="text-red-600 text-sm mt-1">{error}</p>
        <p className="text-red-500 text-xs mt-2">
          Make sure the Airgapper server is running (airgapper serve)
        </p>
      </div>
    );
  }

  if (!status) return null;

  const isOwner = status.role === 'owner';

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-gray-900">System Status</h2>
        <span
          className={`px-2 py-1 rounded-full text-xs font-medium ${
            isOwner
              ? 'bg-blue-100 text-blue-800'
              : 'bg-green-100 text-green-800'
          }`}
        >
          {status.role.toUpperCase()}
        </span>
      </div>

      <div className="space-y-3">
        <div className="flex justify-between">
          <span className="text-gray-500">Name</span>
          <span className="font-medium">{status.name}</span>
        </div>

        <div className="flex justify-between">
          <span className="text-gray-500">Repository</span>
          <span className="font-mono text-sm truncate max-w-[200px]" title={status.repo_url}>
            {status.repo_url}
          </span>
        </div>

        {status.threshold && (
          <div className="flex justify-between">
            <span className="text-gray-500">Threshold</span>
            <span className="font-medium">
              {status.threshold}-of-{status.total_shares}
            </span>
          </div>
        )}

        <div className="flex justify-between">
          <span className="text-gray-500">Key Share</span>
          <span className="font-medium">
            {status.has_share ? (
              <span className="text-green-600">
                ✓ Index {status.share_index}
              </span>
            ) : (
              <span className="text-red-600">✗ Missing</span>
            )}
          </span>
        </div>

        <div className="flex justify-between">
          <span className="text-gray-500">Pending Requests</span>
          <span
            className={`font-medium ${
              status.pending_requests > 0 ? 'text-yellow-600' : 'text-gray-900'
            }`}
          >
            {status.pending_requests}
          </span>
        </div>

        {status.peer && (
          <div className="flex justify-between">
            <span className="text-gray-500">Peer</span>
            <span className="font-medium">
              {status.peer.name}
              {status.peer.address && (
                <span className="text-gray-400 text-xs ml-1">
                  ({status.peer.address})
                </span>
              )}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
