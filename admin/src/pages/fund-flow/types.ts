// 交易类型
export type TransactionType = 'deposit' | 'withdraw' | 'swap' | 'internal';

// 交易状态
export type TransactionStatus = 'success' | 'pending' | 'failed';

// 区块链网络
export type NetworkType = 'ethereum' | 'bsc' | 'polygon' | 'arbitrum' | 'optimism';

// 交易记录
export interface TransactionRecord {
  id: string;
  type: TransactionType;
  amount: number;
  currency: string;
  fee: number;
  feeCurrency: string;
  fromAddress: string;
  toAddress: string;
  network: NetworkType;
  status: TransactionStatus;
  timestamp: string;
  txHash?: string;
}

// 统计数据
export interface FundFlowStats {
  total: number;
  success: number;
  successRate: number;
  pending: number;
  todayAmount: number;
  todayCount: number;
}

// 筛选条件
export interface FundFlowFilters {
  keyword: string;
  type: 'all' | TransactionType;
  status: 'all' | TransactionStatus;
  network: 'all' | NetworkType;
  currency: string;
  startDate: string | null;
  endDate: string | null;
}

