import type { RiskLevel, MonitorStatus, SourceType } from './types';

// 风险等级配置
export const RISK_LEVEL_CONFIG: Record<RiskLevel, { label: string; color: string }> = {
  high: { label: '高风险', color: '#ef4444' },
  medium: { label: '中风险', color: '#f59e0b' },
  low: { label: '低风险', color: '#10b981' },
};

// 监测状态配置
export const MONITOR_STATUS_CONFIG: Record<MonitorStatus, { label: string; color: string }> = {
  active: { label: '监测中', color: '#10b981' },
  stopped: { label: '已停止', color: '#9ca3af' },
};

// 网络配置
export const NETWORK_CONFIG: Record<string, { label: string; color: string }> = {
  // New: chain.network based (TRC20/ERC20/BEP20/...)
  erc20: { label: 'ERC20', color: '#627eea' },
  trc20: { label: 'TRC20', color: '#ff0013' },
  bep20: { label: 'BEP20', color: '#f3ba2f' },
  polygon: { label: 'Polygon', color: '#8247e5' },
  bitcoin: { label: 'Bitcoin', color: '#f7931a' },

  // Legacy values (in case old data exists)
  ethereum: { label: 'Ethereum', color: '#627eea' },
  bsc: { label: 'BSC', color: '#f3ba2f' },
  tron: { label: 'Tron', color: '#ff0013' },
  arbitrum: { label: 'Arbitrum', color: '#28a0f0' },
  optimism: { label: 'Optimism', color: '#ff0420' },
};

// 来源配置
export const SOURCE_CONFIG: Record<SourceType, { label: string }> = {
  user_report: { label: '用户举报' },
  third_party: { label: '第三方数据源' },
  auto_detect: { label: '系统自动检测' },
  association: { label: '关联分析' },
};

// 权限（按 Admin RPC method name）
export const PERM = {
  list: 'ListBlacklistAddresses',
  create: 'CreateBlacklistAddress',
  batchCreate: 'BatchCreateBlacklistAddresses',
  updateMonitor: 'UpdateBlacklistAddressMonitorStatus',
  delete: 'DeleteBlacklistAddress',
  export: 'ExportBlacklistAddresses',
} as const;
