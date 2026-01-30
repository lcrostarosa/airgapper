import { useState, useEffect, useCallback } from 'react';
import type { Status, RestoreRequest } from './types';
import { getStatus, getRequests } from './api';
import { StatusCard } from './components/StatusCard';
import { RequestsCard } from './components/RequestsCard';
import { ScheduleCard } from './components/ScheduleCard';

function App() {
  const [status, setStatus] = useState<Status | null>(null);
  const [requests, setRequests] = useState<RestoreRequest[]>([]);
  const [statusLoading, setStatusLoading] = useState(true);
  const [requestsLoading, setRequestsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchStatus = useCallback(async () => {
    try {
      const data = await getStatus();
      setStatus(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to connect');
    } finally {
      setStatusLoading(false);
    }
  }, []);

  const fetchRequests = useCallback(async () => {
    try {
      const data = await getRequests();
      setRequests(data);
    } catch {
      // Don't show error for requests if status works
    } finally {
      setRequestsLoading(false);
    }
  }, []);

  const refreshAll = useCallback(() => {
    setStatusLoading(true);
    setRequestsLoading(true);
    fetchStatus();
    fetchRequests();
  }, [fetchStatus, fetchRequests]);

  useEffect(() => {
    fetchStatus();
    fetchRequests();

    // Poll every 30 seconds
    const interval = setInterval(() => {
      fetchStatus();
      fetchRequests();
    }, 30000);

    return () => clearInterval(interval);
  }, [fetchStatus, fetchRequests]);

  const handleTriggerBackup = () => {
    alert(
      'Manual backup trigger is not yet implemented in the API.\n\n' +
        'To trigger a backup, run:\n' +
        'airgapper backup <paths>'
    );
  };

  return (
    <div className="min-h-screen bg-gray-100">
      {/* Header */}
      <header className="bg-white shadow-sm">
        <div className="max-w-6xl mx-auto px-4 py-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="text-2xl">🔐</span>
              <h1 className="text-xl font-bold text-gray-900">Airgapper</h1>
            </div>
            <button
              onClick={refreshAll}
              className="text-gray-500 hover:text-gray-700 p-2 rounded-md hover:bg-gray-100 transition-colors"
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
        </div>
      </header>

      {/* Main content */}
      <main className="max-w-6xl mx-auto px-4 py-8 sm:px-6 lg:px-8">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Left column */}
          <div className="space-y-6">
            <StatusCard status={status} loading={statusLoading} error={error} />
            {status?.role === 'owner' && (
              <ScheduleCard
                schedule={status.scheduler}
                onTriggerBackup={handleTriggerBackup}
              />
            )}
          </div>

          {/* Right column */}
          <div>
            <RequestsCard
              requests={requests}
              loading={requestsLoading}
              onRefresh={() => {
                setRequestsLoading(true);
                fetchRequests();
              }}
            />
          </div>
        </div>

        {/* Footer */}
        <footer className="mt-12 text-center text-sm text-gray-500">
          <p>
            Airgapper v0.4.0 • Consensus-based encrypted backup
          </p>
          <p className="mt-1">
            <a
              href="https://github.com/lcrostarosa/airgapper"
              className="hover:text-gray-700"
              target="_blank"
              rel="noopener noreferrer"
            >
              GitHub
            </a>
          </p>
        </footer>
      </main>
    </div>
  );
}

export default App;
