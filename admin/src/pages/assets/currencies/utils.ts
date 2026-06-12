import type {
  AdminCurrencyFeatureSettings,
  AdminCurrencyWithdrawAuditRuleItem,
  AdminCurrencyWithdrawFeeRuleItem,
} from "@/api/generated/schemas";

export type CurrencyFeatureSettings = AdminCurrencyFeatureSettings;
export type CurrencyWithdrawFeeRuleItem = AdminCurrencyWithdrawFeeRuleItem & {
  temp_id?: string;
};
export type CurrencyWithdrawAuditRuleItem =
  AdminCurrencyWithdrawAuditRuleItem & { temp_id?: string };

export const defaultSettingsFor = (
  assetCode: string
): CurrencyFeatureSettings => ({
  web2_deposit_enabled: false,
  web2_withdraw_enabled: false,
  web2_transfer_enabled: false,
  web3_deposit_enabled: true,
  web3_withdraw_enabled: true,
  web3_swap_enabled: assetCode.toUpperCase() === "USDT",
  use_global_withdraw_fee: true,
  use_global_withdraw_audit: true,
  use_global_transfer_audit: true,
});

export function normalizeRules(
  rules: CurrencyWithdrawFeeRuleItem[] | undefined
): CurrencyWithdrawFeeRuleItem[] {
  return (rules || []).map((r, idx) => ({
    temp_id: r.temp_id || `tmp-${Date.now()}-${idx}`,
    id: r.id,
    rule_type: r.rule_type || "fixed",
    value: r.value ?? "0",
    min_amount: r.min_amount ?? "0",
    max_amount: r.max_amount ?? "",
    min_fee: r.min_fee ?? "",
    max_fee: r.max_fee ?? "",
    enabled: r.enabled ?? true,
    sort_order: typeof r.sort_order === "number" ? r.sort_order : idx,
  }));
}

export function normalizeAuditRules(
  rules: CurrencyWithdrawAuditRuleItem[] | undefined
): CurrencyWithdrawAuditRuleItem[] {
  return (rules || []).map((r, idx) => ({
    temp_id: r.temp_id || `tmp-${Date.now()}-${idx}`,
    id: r.id,
    min_amount: r.min_amount ?? "0",
    max_amount: r.max_amount ?? "",
    strategy: (r.strategy as any) || "auto",
    enabled: r.enabled ?? true,
    sort_order: typeof r.sort_order === "number" ? r.sort_order : idx,
  }));
}
