// 风险等级
export type RiskLevel = 'high' | 'medium' | 'low';

// 监测状态
export type MonitorStatus = 'active' | 'stopped';

// 区块链网络
// Dynamic value from DB `chain.network` (normalized to lowercase for API).
export type NetworkType = string;

// 来源类型
export type SourceType = 'user_report' | 'third_party' | 'auto_detect' | 'association';

// 黑名单地址记录
export interface BlacklistAddress {
  id: string;
  address: string;
  network: NetworkType;
  riskLevel: RiskLevel;
  source: SourceType;
  hitCount: number;
  lastHitAt?: string;
  monitorStatus: MonitorStatus;
  addedAt: string;
  addedBy: string;
  reason?: string;
}

// 统计数据
export interface BlacklistStats {
  total: number;
  highRisk: number;
  highRiskRatio: number;
  monitoring: number;
  totalHits: number;
}

// 筛选条件
export interface BlacklistFilters {
  keyword: string;
  riskLevel: 'all' | RiskLevel;
  network: 'all' | NetworkType;
  monitorStatus: 'all' | MonitorStatus;
}
