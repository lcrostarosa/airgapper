// API Response types
export interface APIResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

// Status types
export interface PeerInfo {
  name: string;
  address?: string;
}

export interface SchedulerStatus {
  enabled: boolean;
  schedule?: string;
  paths?: string[];
  last_run?: string;
  last_error?: string;
  next_run?: string;
}

export interface Status {
  name: string;
  role: 'owner' | 'host';
  repo_url: string;
  has_share: boolean;
  share_index: number;
  pending_requests: number;
  threshold?: number;
  total_shares?: number;
  peer?: PeerInfo;
  scheduler?: SchedulerStatus;
}

// Request types
export interface RestoreRequest {
  id: string;
  requester: string;
  snapshot_id: string;
  paths?: string[];
  reason: string;
  status: 'pending' | 'approved' | 'denied' | 'expired';
  created_at: string;
  expires_at: string;
  approved_at?: string;
  approved_by?: string;
}

// Schedule types
export interface ScheduleInfo {
  schedule: string;
  paths: string[];
  enabled: boolean;
  last_run?: string;
  last_error?: string;
  next_run?: string;
}
