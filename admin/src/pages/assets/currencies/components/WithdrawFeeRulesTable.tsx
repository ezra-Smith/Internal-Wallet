import type { ProColumns } from "@ant-design/pro-components";
import { ProTable } from "@ant-design/pro-components";
import { Button, InputNumber, Select, Space, Switch, Typography } from "antd";
import React, { useCallback, useMemo } from "react";
import type { I18nT } from "../types";
import type { CurrencyWithdrawFeeRuleItem } from "../utils";

type Props = {
  rules: CurrencyWithdrawFeeRuleItem[];
  setRules: React.Dispatch<React.SetStateAction<CurrencyWithdrawFeeRuleItem[]>>;
  canEdit: boolean;
  t: I18nT;
};

const DECIMAL_RE = /^\d*\.?\d*$/;

const sanitizeDecimal = (raw: unknown) => {
  const s0 = String(raw ?? "").trim();
  if (!s0) return "";
  let s = s0.replace(/[^\d.]/g, "");
  const firstDot = s.indexOf(".");
  if (firstDot >= 0) {
    s = s.slice(0, firstDot + 1) + s.slice(firstDot + 1).replace(/\./g, "");
  }
  if (s === ".") s = "0.";
  return DECIMAL_RE.test(s) ? s : "";
};

const Sep: React.FC<{ disabled: boolean }> = ({ disabled }) => (
  <Typography.Text
    style={{
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      width: 28,
      border: "1px solid #d9d9d9",
      borderLeft: "none",
      borderRight: "none",
      background: disabled ? "#f5f5f5" : "#fafafa",
      color: "#6b7280",
      userSelect: "none",
      height: 32,
      flex: "0 0 28px",
    }}
  >
    ~
  </Typography.Text>
);

const WithdrawFeeRulesTable: React.FC<Props> = ({
  rules,
  setRules,
  canEdit,
  t,
}) => {
  const ruleKey = (r: CurrencyWithdrawFeeRuleItem) =>
    r.temp_id || String(r.id ?? "");

  const patchRow = useCallback(
    (
      row: CurrencyWithdrawFeeRuleItem,
      patch: Partial<CurrencyWithdrawFeeRuleItem>
    ) => {
      const key = ruleKey(row);
      setRules((prev) =>
        prev.map((x) => (ruleKey(x) === key ? { ...x, ...patch } : x))
      );
    },
    [setRules]
  );

  const columns: ProColumns<CurrencyWithdrawFeeRuleItem>[] = useMemo(
    () => [
      {
        title: t("pages.assets.currencies.feeModal.type", "Type"),
        dataIndex: "rule_type",
        width: 140,
        render: (_, row) => (
          <Select
            value={row.rule_type}
            onChange={(v) => {
              if (v === "fixed") {
                patchRow(row, { rule_type: v, min_fee: "", max_fee: "" });
              } else {
                patchRow(row, { rule_type: v });
              }
            }}
            disabled={!canEdit}
            style={{ width: "100%" }}
            options={[
              {
                label: t(
                  "pages.assets.currencies.feeModal.type.fixed",
                  "Fixed"
                ),
                value: "fixed",
              },
              {
                label: t(
                  "pages.assets.currencies.feeModal.type.percent",
                  "Percent"
                ),
                value: "percent",
              },
            ]}
          />
        ),
      },

      {
        title: t("pages.assets.currencies.feeModal.value", "Value"),
        dataIndex: "value",
        width: 220,
        render: (_, row) => {
          const isPercent = row.rule_type === "percent";
          return (
            <Space.Compact style={{ width: "100%" }}>
              <InputNumber<string>
                value={
                  row.value === null || row.value === undefined || row.value === ""
                    ? undefined
                    : String(row.value)
                }
                stringMode
                controls={false}
                min={0 as any}
                step={(isPercent ? 0.0001 : 0.01) as any}
                precision={8}
                onChange={(v) => patchRow(row, { value: sanitizeDecimal(v) })}
                parser={(s) => sanitizeDecimal(s)}
                formatter={(v) => sanitizeDecimal(v)}
                disabled={!canEdit}
                placeholder={
                  isPercent
                    ? t(
                        "pages.assets.currencies.placeholders.examplePercent",
                        "e.g. 0.1"
                      )
                    : t(
                        "pages.assets.currencies.placeholders.exampleFixed",
                        "e.g. 1"
                      )
                }
                inputMode="decimal"
                style={{ width: "100%" }}
              />
              {isPercent ? (
                <Typography.Text
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    padding: "0 10px",
                    border: "1px solid #d9d9d9",
                    borderLeft: "none",
                    borderRadius: "0 6px 6px 0",
                    background: !canEdit ? "#f5f5f5" : "#fafafa",
                    color: "#6b7280",
                    userSelect: "none",
                    height: 32,
                    flex: "0 0 auto",
                  }}
                >
                  %
                </Typography.Text>
              ) : null}
            </Space.Compact>
          );
        },
      },
      {
        title: t(
          "pages.assets.currencies.feeModal.withdrawAmountRange",
          "提现金额区间"
        ),
        dataIndex: "amount_range",
        width: 320,
        render: (_, row) => (
          <Space.Compact style={{ width: "100%" }}>
            <InputNumber<string>
              value={
                row.min_amount === null ||
                row.min_amount === undefined ||
                row.min_amount === ""
                  ? undefined
                  : String(row.min_amount)
              }
              stringMode
              controls={false}
              min={0 as any}
              step={0.01 as any}
              precision={8}
              onChange={(v) =>
                patchRow(row, { min_amount: sanitizeDecimal(v) })
              }
              parser={(s) => sanitizeDecimal(s)}
              formatter={(v) => sanitizeDecimal(v)}
              disabled={!canEdit}
              placeholder={t("common.min", "最小")}
              inputMode="decimal"
              style={{ width: "100%", minWidth: 0, flex: "1 1 0" }}
            />
            <Sep disabled={!canEdit} />
            <InputNumber<string>
              value={
                row.max_amount === null ||
                row.max_amount === undefined ||
                row.max_amount === ""
                  ? undefined
                  : String(row.max_amount)
              }
              stringMode
              controls={false}
              min={0 as any}
              step={0.01 as any}
              precision={8}
              onChange={(v) =>
                patchRow(row, { max_amount: sanitizeDecimal(v) })
              }
              parser={(s) => sanitizeDecimal(s)}
              formatter={(v) => sanitizeDecimal(v)}
              disabled={!canEdit}
              placeholder={t("common.max", "最大")}
              inputMode="decimal"
              style={{ width: "100%", minWidth: 0, flex: "1 1 0" }}
            />
          </Space.Compact>
        ),
      },
      {
        title: t("pages.assets.currencies.feeModal.feeRange", "手续费区间"),
        dataIndex: "fee_range",
        width: 320,
        render: (_, row) => {
          const disableFeeRange = !canEdit || row.rule_type === "fixed";
          return (
            <Space.Compact style={{ width: "100%" }}>
              <InputNumber<string>
                value={
                  row.min_fee === null ||
                  row.min_fee === undefined ||
                  row.min_fee === ""
                    ? undefined
                    : String(row.min_fee)
                }
                stringMode
                controls={false}
                min={0 as any}
                step={0.01 as any}
                precision={8}
                onChange={(v) => patchRow(row, { min_fee: sanitizeDecimal(v) })}
                parser={(s) => sanitizeDecimal(s)}
                formatter={(v) => sanitizeDecimal(v)}
                disabled={disableFeeRange}
                placeholder={t(
                  "pages.assets.currencies.feeModal.minFee",
                  "最小费用"
                )}
                inputMode="decimal"
                style={{ width: "100%", minWidth: 0, flex: "1 1 0" }}
              />
              <Sep disabled={disableFeeRange} />
              <InputNumber<string>
                value={
                  row.max_fee === null ||
                  row.max_fee === undefined ||
                  row.max_fee === ""
                    ? undefined
                    : String(row.max_fee)
                }
                stringMode
                controls={false}
                min={0 as any}
                step={0.01 as any}
                precision={8}
                onChange={(v) => patchRow(row, { max_fee: sanitizeDecimal(v) })}
                parser={(s) => sanitizeDecimal(s)}
                formatter={(v) => sanitizeDecimal(v)}
                disabled={disableFeeRange}
                placeholder={t(
                  "pages.assets.currencies.feeModal.maxFee",
                  "最大费用"
                )}
                inputMode="decimal"
                style={{ width: "100%", minWidth: 0, flex: "1 1 0" }}
              />
            </Space.Compact>
          );
        },
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
            value={Number(row.sort_order ?? 0)}
            min={0}
            precision={0}
            controls={false}
            onChange={(v) => patchRow(row, { sort_order: Number(v ?? 0) })}
            disabled={!canEdit}
            inputMode="numeric"
            style={{ width: "100%" }}
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
    [canEdit, patchRow, setRules, t]
  );

  return (
    <ProTable<CurrencyWithdrawFeeRuleItem>
      rowKey={(r) => r.temp_id || String(r.id ?? "")}
      search={false}
      options={false}
      pagination={false}
      scroll={{ x: "max-content" }}
      dataSource={rules}
      columns={columns}
    />
  );
};

export default WithdrawFeeRulesTable;
