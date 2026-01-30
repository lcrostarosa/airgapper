import { useState } from 'react';
import type { RestoreRequest } from '../types';
import { approveRequest, denyRequest } from '../api';

interface RequestsCardProps {
  requests: RestoreRequest[];
  loading: boolean;
  onRefresh: () => void;
}

export function RequestsCard({ requests, loading, onRefresh }: RequestsCardProps) {
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleApprove = async (id: string) => {
    setActionLoading(id);
    setError(null);
    try {
      await approveRequest(id);
      onRefresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to approve');
    } finally {
      setActionLoading(null);
    }
  };

  const handleDeny = async (id: string) => {
    setActionLoading(id);
    setError(null);
    try {
      await denyRequest(id);
      onRefresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to deny');
    } finally {
      setActionLoading(null);
    }
  };

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleString();
  };

  const getTimeRemaining = (expiresAt: string) => {
    const diff = new Date(expiresAt).getTime() - Date.now();
    if (diff < 0) return 'Expired';
    const hours = Math.floor(diff / (1000 * 60 * 60));
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
    return `${hours}h ${minutes}m`;
  };

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-gray-900">Pending Requests</h2>
        <button
          onClick={onRefresh}
          className="text-gray-400 hover:text-gray-600"
          title="Refresh"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-red-700 text-sm">
          {error}
        </div>
      )}

      {loading ? (
        <div className="animate-pulse space-y-3">
          <div className="h-20 bg-gray-200 rounded"></div>
          <div className="h-20 bg-gray-200 rounded"></div>
        </div>
      ) : requests.length === 0 ? (
        <div className="text-center py-8 text-gray-500">
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
              d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
            />
          </svg>
          <p>No pending requests</p>
        </div>
      ) : (
        <div className="space-y-4">
          {requests.map((req) => (
            <div
              key={req.id}
              className="border border-gray-200 rounded-lg p-4 hover:border-gray-300 transition-colors"
            >
              <div className="flex items-start justify-between mb-2">
                <div>
                  <span className="font-mono text-sm text-gray-600">{req.id}</span>
                  <p className="font-medium text-gray-900 mt-1">{req.reason}</p>
                </div>
                <span className="text-xs text-gray-500">
                  {getTimeRemaining(req.expires_at)} left
                </span>
              </div>

              <div className="text-sm text-gray-500 space-y-1 mb-3">
                <p>
                  <span className="font-medium">From:</span> {req.requester}
                </p>
                <p>
                  <span className="font-medium">Snapshot:</span> {req.snapshot_id}
                </p>
                <p>
                  <span className="font-medium">Created:</span> {formatDate(req.created_at)}
                </p>
              </div>

              <div className="flex gap-2">
                <button
                  onClick={() => handleApprove(req.id)}
                  disabled={actionLoading === req.id}
                  className="flex-1 bg-green-600 hover:bg-green-700 disabled:bg-green-300 text-white px-4 py-2 rounded-md text-sm font-medium transition-colors"
                >
                  {actionLoading === req.id ? 'Processing...' : '✓ Approve'}
                </button>
                <button
                  onClick={() => handleDeny(req.id)}
                  disabled={actionLoading === req.id}
                  className="flex-1 bg-red-600 hover:bg-red-700 disabled:bg-red-300 text-white px-4 py-2 rounded-md text-sm font-medium transition-colors"
                >
                  {actionLoading === req.id ? 'Processing...' : '✗ Deny'}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
