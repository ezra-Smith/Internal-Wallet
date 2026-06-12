/**
 * 出金审批模块类型定义
 */

/** 审核类型 */
export type ApprovalType = 'withdrawal' | 'transfer';

/** 审核状态 */
export type ApprovalStatus = 'pending' | 'approved' | 'rejected';

/** 审批配置 */
export interface ApprovalConfig {
  type: ApprovalType;
  enabled: boolean;
  threshold: number; // USDT threshold
}

/** 审批统计数据 */
export interface ApprovalStats {
  pending: number;
  approved: number;
  rejected: number;
}

/** 审批记录 */
export interface ApprovalRecord {
  id: string;
  uid: string;
  name: string;
  phone?: string;
  email?: string;
  currency: string;
  amount: number;
  applyTime: string;
  status: ApprovalStatus;
  isWhitelist?: boolean;
  type: ApprovalType;
  /** 审核备注 */
  auditNote?: string;
  /** 拒绝原因 */
  rejectReason?: string;
  /** 审核人 */
  auditor?: string;
  /** 审核时间 */
  auditTime?: string;
}

/** 审批规则配置 */
export interface ApprovalRuleConfig {
  /** 提现审核开关 */
  withdrawalEnabled: boolean;
  /** 提现单笔金额审核阈值 */
  withdrawalThreshold: number;
  /** 提现单日总额审核阈值 */
  withdrawalDailyThreshold: number;
  /** 转账审核开关 */
  transferEnabled: boolean;
  /** 转账单笔金额审核阈值 */
  transferThreshold: number;
  /** 转账单日总额审核阈值 */
  transferDailyThreshold: number;
  /** 白名单免审开关 */
  whitelistEnabled: boolean;
  /** 白名单免审阈值（单笔） */
  whitelistThreshold: number;
  /** 白名单免审总额（单日） */
  whitelistDailyLimit: number;
}

/** 表格操作类型 */
export type TableAction = 'approve' | 'reject' | 'whitelist';

/** 批量操作参数 */
export interface BatchActionParams {
  action: TableAction;
  ids: string[];
  note?: string;
  reason?: string;
}

