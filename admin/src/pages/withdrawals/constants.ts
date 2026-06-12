import type { ApprovalConfig, ApprovalRecord, ApprovalStats } from './types';

/**
 * 出金审批模块常量和模拟数据
 */

/** 默认审批配置 */
export const DEFAULT_APPROVAL_CONFIG: ApprovalConfig[] = [
  {
    type: 'withdrawal',
    enabled: true,
    threshold: 10000,
  },
  {
    type: 'transfer',
    enabled: true,
    threshold: 5000,
  },
];

/** 模拟统计数据 */
export const MOCK_STATS: Record<'withdrawal' | 'transfer', ApprovalStats> = {
  withdrawal: {
    pending: 2,
    approved: 3,
    rejected: 0,
  },
  transfer: {
    pending: 1,
    approved: 2,
    rejected: 0,
  },
};

/** 模拟审批记录 */
export const MOCK_APPROVAL_RECORDS: ApprovalRecord[] = [
  {
    id: '1',
    uid: 'U10001',
    name: '张三',
    phone: '138****5678',
    email: 'zhangsan@example.com',
    currency: 'USDT',
    amount: 10000,
    applyTime: '2024-12-09 10:30',
    status: 'pending',
    type: 'withdrawal',
  },
  {
    id: '2',
    uid: 'U10002',
    name: '李四',
    phone: undefined,
    email: 'lisi@example.com',
    currency: 'ETH',
    amount: 2.5,
    applyTime: '2024-12-09 11:15',
    status: 'pending',
    type: 'withdrawal',
  },
  {
    id: '3',
    uid: 'U10003',
    name: '王五',
    phone: '136****9876',
    email: undefined,
    currency: 'USDT',
    amount: 5000,
    applyTime: '2024-12-09 09:00',
    status: 'approved',
    type: 'withdrawal',
    auditor: 'admin',
    auditTime: '2024-12-09 09:30',
  },
  {
    id: '4',
    uid: 'U10007',
    name: '陈白',
    phone: '135****8888',
    email: 'chenbai@example.com',
    currency: 'USDT',
    amount: 3000,
    applyTime: '2024-12-09 13:30',
    status: 'approved',
    type: 'withdrawal',
    isWhitelist: true,
    auditor: 'system',
    auditTime: '2024-12-09 13:30',
  },
  {
    id: '5',
    uid: 'U10008',
    name: '刘白',
    phone: '136****9999',
    email: 'liubai@example.com',
    currency: 'USDT',
    amount: 4500,
    applyTime: '2024-12-09 16:20',
    status: 'approved',
    type: 'withdrawal',
    isWhitelist: true,
    auditor: 'system',
    auditTime: '2024-12-09 16:20',
  },
  {
    id: '6',
    uid: 'U10010',
    name: '赵六',
    phone: '137****1234',
    email: 'zhaoliu@example.com',
    currency: 'USDT',
    amount: 8000,
    applyTime: '2024-12-09 14:00',
    status: 'pending',
    type: 'transfer',
  },
  {
    id: '7',
    uid: 'U10011',
    name: '钱七',
    phone: '139****5678',
    email: 'qianqi@example.com',
    currency: 'USDT',
    amount: 6000,
    applyTime: '2024-12-09 15:00',
    status: 'approved',
    type: 'transfer',
    auditor: 'admin',
    auditTime: '2024-12-09 15:30',
  },
  {
    id: '8',
    uid: 'U10012',
    name: '孙八',
    phone: '138****9012',
    email: 'sunba@example.com',
    currency: 'BTC',
    amount: 0.5,
    applyTime: '2024-12-09 16:00',
    status: 'approved',
    type: 'transfer',
    auditor: 'admin',
    auditTime: '2024-12-09 16:30',
  },
];

/** 权限常量 */
export const PERM = {
  list: 'ListApprovals',
  approve: 'ApproveWithdrawal',
  reject: 'RejectWithdrawal',
  config: 'ConfigApprovalRules',
  whitelist: 'ManageWithdrawWhitelist',
};

/** 币种颜色映射 */
export const CURRENCY_COLORS: Record<string, string> = {
  USDT: '#26a17b',
  ETH: '#627eea',
  BTC: '#f7931a',
  BNB: '#f3ba2f',
  TRX: '#eb0029',
};

/** 审核规则说明 */
export const APPROVAL_RULES_INFO = {
  withdrawal: {
    title: '提现审核规则',
    description: '当提现金额（折合USDT）≥ 阈值时，需要管理员审核通过后才能完成提现。',
  },
  transfer: {
    title: '转账审核规则',
    description: '当转账金额（折合USDT）≥ 阈值时，需要管理员审核通过后才能完成转账。',
  },
};

