import type { RequestsTabProps } from "./types";

export function DashboardRequests({ config, pendingRequests }: RequestsTabProps) {
  return (
    <div className="bg-gray-800 rounded-lg p-6">
      <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
        <span>📋</span> Restore Requests
      </h2>

      {pendingRequests.length === 0 ? (
        <div className="text-center py-8 text-gray-400">
          <div className="text-4xl mb-4">📭</div>
          <p>No pending restore requests</p>
          {config.role === "owner" && (
            <p className="text-sm mt-2">
              Create a request with:{" "}
              <code className="bg-gray-900 px-2 py-1 rounded">
                airgapper request
              </code>
            </p>
          )}
        </div>
      ) : (
        <div className="space-y-4">
          {pendingRequests.map((request) => (
            <div
              key={request.id}
              className="bg-gray-700 rounded-lg p-4"
            >
              <div className="flex items-start justify-between mb-3">
                <div>
                  <div className="font-medium">{request.reason}</div>
                  <div className="text-sm text-gray-400">
                    From: {request.requester}
                  </div>
                </div>
                <span
                  className={`text-xs px-2 py-1 rounded ${
                    request.status === "pending"
                      ? "bg-yellow-900/50 text-yellow-400"
                      : request.status === "approved"
                      ? "bg-green-900/50 text-green-400"
                      : "bg-red-900/50 text-red-400"
                  }`}
                >
                  {request.status}
                </span>
              </div>

              {/* Approval progress for consensus mode */}
              {request.requiredApprovals && (
                <div className="mb-3">
                  <div className="text-sm text-gray-400 mb-1">
                    Approvals: {(request.approvals || []).length}/
                    {request.requiredApprovals}
                  </div>
                  <div className="h-2 bg-gray-600 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-blue-500 transition-all"
                      style={{
                        width: `${
                          ((request.approvals || []).length /
                            request.requiredApprovals) *
                          100
                        }%`,
                      }}
                    />
                  </div>
                </div>
              )}

              <div className="text-xs text-gray-500">
                ID: {request.id} | Expires:{" "}
                {new Date(request.expiresAt).toLocaleString()}
              </div>

              {request.status === "pending" && config.role === "host" && (
                <div className="mt-3 flex gap-2">
                  <button className="flex-1 bg-green-600 hover:bg-green-700 text-white py-2 rounded transition-colors">
                    Approve
                  </button>
                  <button className="flex-1 bg-red-600 hover:bg-red-700 text-white py-2 rounded transition-colors">
                    Deny
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
