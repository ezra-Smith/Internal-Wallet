import dayjs from 'dayjs';

export type DashboardKpi = {
  id: 'web2_users' | 'vault_balance' | 'today_transfers' | 'pending_withdrawals';
  title: string;
  value: number;
  unit?: string;
  deltaPct?: number;
  deltaLabel?: string;
};

export type DashboardSystemStatusItem = {
  name: string;
  status: 'ok' | 'warn' | 'down';
  detail?: string;
};

export type DashboardActivityItem = {
  id: string;
  type: 'transfer_batch' | 'withdrawal' | 'user' | 'vault';
  title: string;
  description?: string;
  at: string;
  level: 'success' | 'warning' | 'error' | 'info';
};

export type UserStatusDistribution = {
  normal: number;
  frozen: number;
  pending: number;
  total: number;
};

export type WithdrawalApprovalDistribution = {
  pending: number;
  approved: number;
  rejected: number;
  total: number;
};

export type VaultTokenBalance = {
  rank: number;
  symbol: string;
  usd: number;
  pct: number;
};

export type DashboardMockData = {
  kpis: DashboardKpi[];
  userStats: { active7d: number; normal: number; frozen: number; newToday: number };
  userStatusDistribution: UserStatusDistribution;
  withdrawalApprovalDistribution: WithdrawalApprovalDistribution;
  vaultTokenBalances: VaultTokenBalance[];
  vaultStats: { supportedTokens: number; topBalances: { symbol: string; usd: number; pct: number }[] };
  trend: { month: string; inbound: number; outbound: number; internal: number }[];
  batchTrend7d: { day: string; total: number; success: number; failed: number }[];
  systemStatus: DashboardSystemStatusItem[];
  recentActivity: DashboardActivityItem[];
};

const now = dayjs();

// 资金流水趋势数据 - 近6个月入账、出账、内部转账统计
const fundFlowBase = [
  { inbound: 145000, outbound: 98000, internal: 32000 },
  { inbound: 165000, outbound: 118000, internal: 30000 },
  { inbound: 142000, outbound: 102000, internal: 38000 },
  { inbound: 185000, outbound: 132000, internal: 42000 },
  { inbound: 178000, outbound: 135000, internal: 38000 },
  { inbound: 205000, outbound: 148000, internal: 46000 },
];

const trend = fundFlowBase.map((item, index) => {
  const monthNum = now.subtract(fundFlowBase.length - 1 - index, 'month').month() + 1;
  return { month: `${monthNum}月`, ...item };
});

// 转账批次执行趋势 - 近7天批次执行与成功/失败人数
const batchExecutionBase = [
  { batch: 5, success: 82, failed: 3 },
  { batch: 6, success: 156, failed: 4 },
  { batch: 4, success: 88, failed: 2 },
  { batch: 6, success: 142, failed: 5 },
  { batch: 7, success: 185, failed: 3 },
  { batch: 5, success: 138, failed: 4 },
  { batch: 5, success: 115, failed: 2 },
];

const batchTrend7d = batchExecutionBase.map((item, index) => {
  const day = now.subtract(batchExecutionBase.length - 1 - index, 'day').format('MM/DD');
  return {
    day,
    total: item.batch,
    success: item.success,
    failed: item.failed,
  };
});

export const dashboardMockData: DashboardMockData = {
  kpis: [
    {
      id: 'web2_users',
      title: 'Web2 users',
      value: 1000,
      unit: '人',
      deltaPct: 8.5,
      deltaLabel: 'pages.dashboard.delta.vsYesterday',
    },
    {
      id: 'vault_balance',
      title: 'Vault balance',
      value: 748000,
      unit: 'USD',
      deltaPct: 2.1,
      deltaLabel: 'pages.dashboard.delta.vsYesterday',
    },
    {
      id: 'today_transfers',
      title: 'Today transfers',
      value: 128,
      unit: '笔',
      deltaPct: -4.3,
      deltaLabel: 'pages.dashboard.delta.vsYesterday',
    },
    {
      id: 'pending_withdrawals',
      title: 'Pending withdrawals',
      value: 17,
      unit: '笔',
      deltaPct: 3.2,
      deltaLabel: 'pages.dashboard.delta.vsYesterday',
    },
  ],
  userStats: {
    active7d: 612,
    normal: 945,
    frozen: 55,
    newToday: 26,
  },
  // 用户状态分布
  userStatusDistribution: {
    normal: 856,
    frozen: 34,
    pending: 110,
    total: 1000,
  },
  // 出金审批分布
  withdrawalApprovalDistribution: {
    pending: 25,
    approved: 186,
    rejected: 12,
    total: 223,
  },
  // Vault币种余额 Top 5
  vaultTokenBalances: [
    { rank: 1, symbol: 'USDT', usd: 285000, pct: 38.1 },
    { rank: 2, symbol: 'USDC', usd: 142000, pct: 19.0 },
    { rank: 3, symbol: 'ETH', usd: 98000, pct: 13.1 },
    { rank: 4, symbol: 'BTC', usd: 156000, pct: 20.9 },
    { rank: 5, symbol: 'BNB', usd: 67000, pct: 9.0 },
  ],
  vaultStats: {
    supportedTokens: 18,
    topBalances: [
      { symbol: 'USDT', usd: 312000, pct: 41.7 },
      { symbol: 'USDC', usd: 198000, pct: 26.5 },
      { symbol: 'ETH', usd: 132000, pct: 17.6 },
      { symbol: 'BTC', usd: 86000, pct: 11.5 },
      { symbol: 'Others', usd: 20000, pct: 2.7 },
    ],
  },
  trend,
  batchTrend7d,
  systemStatus: [
    { name: 'API Gateway', status: 'ok', detail: 'p95=118ms' },
    { name: 'Admin RPC', status: 'ok', detail: 'p95=96ms' },
    { name: 'MariaDB', status: 'warn', detail: 'replica lag 4.2s' },
    { name: 'Signer', status: 'down', detail: 'HSM handshake timeout' },
  ],
  recentActivity: [
    {
      id: 'act-1001',
      type: 'transfer_batch',
      title: '转账批次执行完成',
      description: '春节红包批次，成功发放 115 人',
      at: now.subtract(5, 'minute').toISOString(),
      level: 'success',
    },
    {
      id: 'act-1002',
      type: 'withdrawal',
      title: '出金审批通过',
      description: '用户 U10023 提现 5,000 USDT',
      at: now.subtract(12, 'minute').toISOString(),
      level: 'success',
    },
    {
      id: 'act-1003',
      type: 'user',
      title: '新用户注册',
      description: '3个新用户完成注册验证',
      at: now.subtract(25, 'minute').toISOString(),
      level: 'info',
    },
    {
      id: 'act-1004',
      type: 'withdrawal',
      title: '出金审批拒绝',
      description: '用户 U10045 提现申请被拒绝',
      at: now.subtract(38, 'minute').toISOString(),
      level: 'error',
    },
    {
      id: 'act-1005',
      type: 'vault',
      title: 'Vault充值成功',
      description: 'USDT充值 50,000',
      at: now.subtract(1, 'hour').toISOString(),
      level: 'success',
    },
  ],
};
