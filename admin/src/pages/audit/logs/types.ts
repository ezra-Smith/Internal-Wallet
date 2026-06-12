export type AuditLogStatus = 'success' | 'failed';

export type ModuleType =
  | 'all'
  | 'account'
  | 'vault'
  | 'transfer'
  | 'user'
  | 'currency'
  | 'swap'
  | 'approval'
  | 'role'
  | 'blacklist'
  | 'system'
  | 'audit';

export interface AuditLogRow {
  id: string;
  createdAt: string;
  operator: {
    name: string;
    username?: string;
    role?: string;
    email?: string;
  };
  module: ModuleType;
  action: string;
  target: string;
  description: string;
  status: AuditLogStatus;
  ip?: string;
  requestId?: string;
  rpcMethod?: string;
  httpMethod?: string;
  httpPath?: string;
  durationMs?: number;
  errorMessage?: string;
  details?: Record<string, unknown>;
  raw?: unknown;
}

export interface AuditLogStats {
  total: number;
  success: number;
  failed: number;
  today: number;
}

export interface AuditLogFilters {
  keyword: string;
  module: ModuleType;
  status: 'all' | AuditLogStatus;
  dateRange: [string | null, string | null];
  operator: string;
  role: string;
  action: string;
  targetType: string;
  targetId: string;
  ip: string;
  requestId: string;
  httpMethod: string;
  httpPath: string;
  rpcMethod: string;
  durationMsFrom: number | null;
  durationMsTo: number | null;
}
