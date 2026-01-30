import type { APIResponse, Status, RestoreRequest, ScheduleInfo } from './types';

const API_BASE = '/api';

async function fetchAPI<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  });

  const json: APIResponse<T> = await response.json();

  if (!json.success) {
    throw new Error(json.error || 'Unknown error');
  }

  return json.data as T;
}

// Status
export async function getStatus(): Promise<Status> {
  return fetchAPI<Status>('/status');
}

// Requests
export async function getRequests(): Promise<RestoreRequest[]> {
  return fetchAPI<RestoreRequest[]>('/requests');
}

export async function getRequest(id: string): Promise<RestoreRequest> {
  return fetchAPI<RestoreRequest>(`/requests/${id}`);
}

export async function createRequest(
  snapshotId: string,
  reason: string,
  paths?: string[]
): Promise<{ id: string; status: string; expires_at: string }> {
  return fetchAPI('/requests', {
    method: 'POST',
    body: JSON.stringify({
      snapshot_id: snapshotId,
      reason,
      paths,
    }),
  });
}

export async function approveRequest(id: string): Promise<{ status: string; message: string }> {
  return fetchAPI(`/requests/${id}/approve`, {
    method: 'POST',
    body: JSON.stringify({}),
  });
}

export async function denyRequest(id: string): Promise<{ status: string }> {
  return fetchAPI(`/requests/${id}/deny`, {
    method: 'POST',
    body: JSON.stringify({}),
  });
}

// Schedule
export async function getSchedule(): Promise<ScheduleInfo> {
  return fetchAPI<ScheduleInfo>('/schedule');
}

export async function updateSchedule(
  schedule: string,
  paths: string[]
): Promise<{ status: string; message: string }> {
  return fetchAPI('/schedule', {
    method: 'POST',
    body: JSON.stringify({ schedule, paths }),
  });
}

// Backup (trigger manual backup via server restart - placeholder)
export async function triggerBackup(): Promise<void> {
  // Note: In production, this would POST to a /backup endpoint
  // For now, we just show a message
  throw new Error('Manual backup trigger not yet implemented in API');
}
