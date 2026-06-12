import type { ProColumns } from "@ant-design/pro-components";
import { ProTable } from "@ant-design/pro-components";
import { Button, Input, InputNumber, Select, Switch } from "antd";
import React, { useCallback, useMemo } from "react";
import type { I18nT } from "../types";
import type { CurrencyWithdrawAuditRuleItem } from "../utils";

type StrategyValue = "auto" | "manual_auto" | "manual_manual";

type Props = {
  rules: CurrencyWithdrawAuditRuleItem[];
  setRules: React.Dispatch<
    React.SetStateAction<CurrencyWithdrawAuditRuleItem[]>
  >;
  canEdit: boolean;
  t: I18nT;
  hiddenStrategies?: StrategyValue[];
};

const WithdrawAuditRulesTable: React.FC<Props> = ({
  rules,
  setRules,
  canEdit,
  t,
  hiddenStrategies = [],
}) => {
  const ruleKey = (r: CurrencyWithdrawAuditRuleItem) =>
    r.temp_id || String(r.id ?? "");

  const patchRow = useCallback(
    (
      row: CurrencyWithdrawAuditRuleItem,
      patch: Partial<CurrencyWithdrawAuditRuleItem>
    ) => {
      const key = ruleKey(row);
      setRules((prev) =>
        prev.map((x) => (ruleKey(x) === key ? { ...x, ...patch } : x))
      );
    },
    [setRules]
  );

  const strategyOptions = useMemo(() => {
    const all: Array<{ label: string; value: StrategyValue }> = [
      {
        label: t("pages.assets.currencies.auditModal.strategy.auto", "Auto"),
        value: "auto",
      },
      {
        label: t(
          "pages.assets.currencies.auditModal.strategy.manual_auto",
          "Manual approve, auto payout"
        ),
        value: "manual_auto",
      },
      {
        label: t(
          "pages.assets.currencies.auditModal.strategy.manual_manual",
          "Manual approve, manual transfer"
        ),
        value: "manual_manual",
      },
    ];
    const hidden = new Set(hiddenStrategies);
    return all.filter((o) => !hidden.has(o.value));
  }, [hiddenStrategies, t]);

  const normalizeStrategyValue = useCallback(
    (v: unknown) => {
      const s = String(v ?? "") as StrategyValue;
      const ok = strategyOptions.some((o) => o.value === s);
      return ok ? s : "auto";
    },
    [strategyOptions]
  );

  const columns: ProColumns<CurrencyWithdrawAuditRuleItem>[] = useMemo(
    () => [
      {
        title: t("pages.assets.currencies.feeModal.min", "Min"),
        dataIndex: "min_amount",
        width: 160,
        render: (_, row) => (
          <Input
            value={row.min_amount}
            onChange={(e) => patchRow(row, { min_amount: e.target.value })}
            disabled={!canEdit}
            placeholder={t(
              "pages.assets.currencies.placeholders.exampleAmount",
              "e.g. 10"
            )}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.feeModal.max", "Max"),
        dataIndex: "max_amount",
        width: 160,
        render: (_, row) => (
          <Input
            value={row.max_amount}
            onChange={(e) => patchRow(row, { max_amount: e.target.value })}
            disabled={!canEdit}
            placeholder={t(
              "pages.assets.currencies.placeholders.optional",
              "optional"
            )}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.auditModal.strategy", "Strategy"),
        dataIndex: "strategy",
        width: 220,
        render: (_, row) => (
          <Select
            value={normalizeStrategyValue(row.strategy)}
            onChange={(v) => patchRow(row, { strategy: v as any })}
            disabled={!canEdit}
            options={strategyOptions}
            onDropdownVisibleChange={(open) => {
              if (!open) return;
              const v = normalizeStrategyValue(row.strategy);
              if (v !== row.strategy) patchRow(row, { strategy: v as any });
            }}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.feeModal.enabled", "Enabled"),
        dataIndex: "enabled",
        width: 100,
        render: (_, row) => (
          <Switch
            checked={!!row.enabled}
            onChange={(v) => patchRow(row, { enabled: v })}
            disabled={!canEdit}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.feeModal.order", "Order"),
        dataIndex: "sort_order",
        width: 110,
        render: (_, row) => (
          <InputNumber
            value={row.sort_order}
            min={0}
            precision={0}
            onChange={(v) => patchRow(row, { sort_order: Number(v ?? 0) })}
            disabled={!canEdit}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.feeModal.actions", "Actions"),
        valueType: "option",
        width: 90,
        render: (_, row) => (
          <Button
            danger
            size="small"
            onClick={() =>
              setRules((prev) =>
                prev.filter((x) => ruleKey(x) !== ruleKey(row))
              )
            }
            disabled={!canEdit}
          >
            {t("pages.assets.currencies.feeModal.delete", "Delete")}
          </Button>
        ),
      },
    ],
    [
      canEdit,
      normalizeStrategyValue,
      patchRow,
      ruleKey,
      setRules,
      strategyOptions,
      t,
    ]
  );

  return (
    <ProTable<CurrencyWithdrawAuditRuleItem>
      rowKey={(r) => r.temp_id || String(r.id ?? "")}
      search={false}
      options={false}
      pagination={false}
      dataSource={rules}
      columns={columns}
    />
  );
};

export default WithdrawAuditRulesTable;
