import type { ModuleType } from './types';

// 模块配置
export const MODULE_CONFIG: Record<ModuleType, { label: string; color: string }> = {
  all: { label: '全部模块', color: '#6b7280' },
  account: { label: '后台账号管理', color: '#3b82f6' },
  vault: { label: '公司Vault管理', color: '#8b5cf6' },
  transfer: { label: '转账管理', color: '#10b981' },
  user: { label: '用户管理', color: '#f59e0b' },
  currency: { label: '币种管理', color: '#ec4899' },
  swap: { label: 'Swap配置', color: '#06b6d4' },
  approval: { label: '出金审批', color: '#ef4444' },
  role: { label: '角色权限管理', color: '#84cc16' },
  blacklist: { label: '黑名单管理', color: '#64748b' },
  system: { label: '系统配置', color: '#a855f7' },
  audit: { label: '审计日志', color: '#6366f1' },
};
