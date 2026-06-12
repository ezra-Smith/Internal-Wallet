import type { TransactionRecord, FundFlowStats, TransactionType, TransactionStatus, NetworkType } from './types';

// 交易类型配置
export const TRANSACTION_TYPE_CONFIG: Record<TransactionType, { label: string; color: string; icon: string }> = {
  deposit: { label: '入账', color: '#3b82f6', icon: 'arrow-down' },
  withdraw: { label: '出账', color: '#f59e0b', icon: 'arrow-up' },
  swap: { label: 'Swap', color: '#8b5cf6', icon: 'swap' },
  internal: { label: '内部转账', color: '#6b7280', icon: 'swap' },
};

// 交易状态配置
export const TRANSACTION_STATUS_CONFIG: Record<TransactionStatus, { label: string; color: string }> = {
  success: { label: '成功', color: '#10b981' },
  pending: { label: '处理中', color: '#f59e0b' },
  failed: { label: '失败', color: '#ef4444' },
};

// 网络配置
export const NETWORK_CONFIG: Record<NetworkType, { label: string; color: string }> = {
  ethereum: { label: 'Ethereum', color: '#627eea' },
  bsc: { label: 'BSC', color: '#f3ba2f' },
  polygon: { label: 'Polygon', color: '#8247e5' },
  arbitrum: { label: 'Arbitrum', color: '#28a0f0' },
  optimism: { label: 'Optimism', color: '#ff0420' },
};

// 币种列表
export const CURRENCIES = ['USDT', 'ETH', 'BTC', 'BNB', 'MATIC', 'ARB', 'OP'];

// 模拟统计数据
export const MOCK_STATS: FundFlowStats = {
  total: 5,
  success: 3,
  successRate: 60.0,
  pending: 1,
  todayAmount: 0,
  todayCount: 0,
};

// 模拟交易数据
export const MOCK_TRANSACTIONS: TransactionRecord[] = [
  {
    id: '1',
    type: 'deposit',
    amount: 10000,
    currency: 'USDT',
    fee: 5.2,
    feeCurrency: 'ETH',
    fromAddress: '0x742d...0bEb',
    toAddress: '0x8B31...F3E2',
    network: 'ethereum',
    status: 'success',
    timestamp: '2024-12-08 14:30:25',
    txHash: '0x1234...5678',
  },
  {
    id: '2',
    type: 'swap',
    amount: 5000,
    currency: 'USDT',
    fee: 12.5,
    feeCurrency: 'ETH',
    fromAddress: '0x8B31...F3E2',
    toAddress: '0x8B31...F3E2',
    network: 'ethereum',
    status: 'success',
    timestamp: '2024-12-08 15:45:10',
    txHash: '0x2345...6789',
  },
  {
    id: '3',
    type: 'withdraw',
    amount: 3000,
    currency: 'USDT',
    fee: 0.8,
    feeCurrency: 'BNB',
    fromAddress: '0x8B31...F3E2',
    toAddress: '0xA1B2...ABCD',
    network: 'bsc',
    status: 'success',
    timestamp: '2024-12-08 16:20:35',
    txHash: '0x3456...7890',
  },
  {
    id: '4',
    type: 'internal',
    amount: 2000,
    currency: 'USDT',
    fee: 0.1,
    feeCurrency: 'MATIC',
    fromAddress: '0x8B31...F3E2',
    toAddress: '0xC4F8...C7A6',
    network: 'polygon',
    status: 'pending',
    timestamp: '2024-12-08 17:05:50',
  },
  {
    id: '5',
    type: 'withdraw',
    amount: 500,
    currency: 'ETH',
    fee: 8.5,
    feeCurrency: 'ETH',
    fromAddress: '0xDEF1...2345',
    toAddress: '0xBCA9...8765',
    network: 'ethereum',
    status: 'failed',
    timestamp: '2024-12-08 18:30:15',
    txHash: '0x4567...8901',
  },
];

// 权限配置
export const PERM = {
  VIEW: 'fund-flow:view',
  EXPORT: 'fund-flow:export',
};

