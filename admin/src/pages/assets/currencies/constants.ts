export const PERM = {
  overview: 'GetCurrencyOverview',
  create: 'CreateCurrency',
  list: 'ListCurrencies',
  get: 'GetCurrencyConfig',
  update: 'UpdateCurrencyConfig',
  updateStatus: 'UpdateCurrencyStatus',
  globalGet: 'GetCurrencyGlobalWithdrawFee',
  globalUpdate: 'UpdateCurrencyGlobalWithdrawFee',
  globalAuditGet: 'GetCurrencyGlobalWithdrawAudit',
  globalAuditUpdate: 'UpdateCurrencyGlobalWithdrawAudit',
  globalTransferAuditGet: 'GetCurrencyGlobalTransferAudit',
  globalTransferAuditUpdate: 'UpdateCurrencyGlobalTransferAudit',
} as const;
