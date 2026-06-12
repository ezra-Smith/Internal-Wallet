import {
  adminGetCurrencyGlobalTransferAudit,
  adminGetCurrencyGlobalWithdrawAudit,
  adminGetCurrencyGlobalWithdrawFee,
  adminListCurrencies,
  adminUpdateCurrencyGlobalTransferAudit,
  adminUpdateCurrencyGlobalWithdrawAudit,
  adminUpdateCurrencyGlobalWithdrawFee,
} from "@/api/generated/assets";
import type {
  AdminCurrencyChainAuditRules as CurrencyChainAuditRules,
  AdminCurrencyChainFeeRules as CurrencyChainFeeRules,
} from "@/api/generated/schemas";
import {
  Alert,
  App,
  Button,
  InputNumber,
  Modal,
  Select,
  Space,
  Typography,
} from "antd";

import React, {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useMemo,
  useState,
} from "react";
import type { I18nT } from "../types";
import {
  normalizeAuditRules,
  normalizeRules,
  type CurrencyWithdrawAuditRuleItem,
  type CurrencyWithdrawFeeRuleItem,
} from "../utils";
import WithdrawAuditRulesTable from "./WithdrawAuditRulesTable";
import WithdrawFeeRulesTable from "./WithdrawFeeRulesTable";

type CurrencyAssetTransferAuditRules = {
  asset_code: string;
  rules?: Array<{
    min_amount?: string;
    max_amount?: string;
    strategy?: string;
    enabled?: boolean;
    sort_order?: number;
  }>;
};

export type GlobalRulesModalsRef = {
  openFee: () => void;
  openAudit: () => void;
  openTransferAudit: () => void;
};

type Props = {
  t: I18nT;
  canGlobalGet: boolean;
  canGlobalUpdate: boolean;
  canGlobalAuditGet: boolean;
  canGlobalAuditUpdate: boolean;
  canGlobalTransferAuditGet: boolean;
  canGlobalTransferAuditUpdate: boolean;
};

const PAGE_SIZE = 200;

const normalizeCode = (v: unknown) =>
  String(v ?? "")
    .trim()
    .toUpperCase();

const uniqSorted = (arr: string[]) =>
  Array.from(new Set(arr.map(normalizeCode).filter(Boolean))).sort();

const toDigits = (v: unknown) => String(v ?? "").replace(/\D+/g, "");

const toDecimalStr = (v: unknown, scale = 6, maxDigits = 40) => {
  let s = String(v ?? "").trim();
  if (!s) return "";

  s = s.replace(/[^\d.]/g, "");
  const dot = s.indexOf(".");
  if (dot >= 0) {
    s = s.slice(0, dot + 1) + s.slice(dot + 1).replace(/\./g, "");
  }

  if (s === ".") s = "0.";
  if (s.startsWith(".")) s = `0${s}`;

  let [intPartRaw, fracRaw = ""] = s.split(".");
  intPartRaw = intPartRaw.replace(/^0+(?=\d)/, "");
  const intPart = intPartRaw === "" ? "0" : intPartRaw;
  const frac = fracRaw.slice(0, scale);

  const digitsTotal = (intPart + frac).replace(/^0+(?=\d)/, "").length || 1;
  if (digitsTotal > maxDigits) {
    const keepInt = Math.min(intPart.length, maxDigits);
    const newInt = intPart.slice(0, keepInt) || "0";
    const remain = maxDigits - newInt.length;
    const newFrac = remain > 0 ? frac.slice(0, remain) : "";
    return newFrac ? `${newInt}.${newFrac}` : newInt;
  }

  return frac.length > 0 ? `${intPart}.${frac}` : intPart;
};

const _DigitInput: React.FC<{
  value?: string | number;
  onChange?: (next: string) => void;
  disabled?: boolean;
  placeholder?: string;
  min?: number;
  max?: number;
  style?: React.CSSProperties;
  suffix?: string;
  mode?: "int" | "decimal";
  scale?: number;
  maxDigits?: number;
}> = ({
  value,
  onChange,
  disabled,
  placeholder,
  min,
  max,
  style,
  suffix,
  mode = "decimal",
  scale = 6,
  maxDigits = 40,
}) => {
  const vv =
    mode === "int"
      ? toDigits(value)
      : toDecimalStr(value ?? "", scale, maxDigits);

  return (
    <Space.Compact style={style}>
      <InputNumber<string>
        size="small"
        value={vv === "" ? undefined : vv}
        stringMode
        controls={false}
        inputMode={mode === "int" ? "numeric" : "decimal"}
        placeholder={placeholder}
        disabled={disabled}
        min={min as any}
        max={max as any}
        parser={(x) =>
          mode === "int" ? toDigits(x) : toDecimalStr(x, scale, maxDigits)
        }
        formatter={(x) =>
          mode === "int" ? toDigits(x) : toDecimalStr(x, scale, maxDigits)
        }
        onChange={(val) =>
          onChange?.(
            mode === "int" ? toDigits(val) : toDecimalStr(val, scale, maxDigits)
          )
        }
        style={{
          width: suffix ? 160 : undefined,
        }}
      />

      {suffix ? (
        <Typography.Text
          style={{
            display: "inline-flex",
            alignItems: "center",
            padding: "0 10px",
            border: "1px solid #d9d9d9",
            borderLeft: "none",
            borderRadius: "0 6px 6px 0",
            background: disabled ? "#f5f5f5" : "#fafafa",
            color: "#6b7280",
            userSelect: "none",
            height: 24,
          }}
        >
          {suffix}
        </Typography.Text>
      ) : null}
    </Space.Compact>
  );
};

const GlobalRulesModals = forwardRef<GlobalRulesModalsRef, Props>(
  (
    {
      t,
      canGlobalGet,
      canGlobalUpdate,
      canGlobalAuditGet,
      canGlobalAuditUpdate,
      canGlobalTransferAuditGet,
      canGlobalTransferAuditUpdate,
    },
    ref
  ) => {
    const { message } = App.useApp();

    const [currencyAssetCodes, setCurrencyAssetCodes] = useState<string[]>([]);

    const [globalModalOpen, setGlobalModalOpen] = useState(false);
    const [globalModalLoading, setGlobalModalLoading] = useState(false);
    const [globalModalSaving, setGlobalModalSaving] = useState(false);
    const [globalChain, setGlobalChain] = useState<string>("");
    const [globalRulesByChain, setGlobalRulesByChain] = useState<
      Record<string, CurrencyWithdrawFeeRuleItem[]>
    >({});

    const [globalAuditModalOpen, setGlobalAuditModalOpen] = useState(false);
    const [globalAuditModalLoading, setGlobalAuditModalLoading] =
      useState(false);
    const [globalAuditModalSaving, setGlobalAuditModalSaving] = useState(false);
    const [globalAuditChain, setGlobalAuditChain] = useState<string>("");
    const [globalAuditRulesByChain, setGlobalAuditRulesByChain] = useState<
      Record<string, CurrencyWithdrawAuditRuleItem[]>
    >({});

    const [globalTransferAuditModalOpen, setGlobalTransferAuditModalOpen] =
      useState(false);
    const [
      globalTransferAuditModalLoading,
      setGlobalTransferAuditModalLoading,
    ] = useState(false);
    const [globalTransferAuditModalSaving, setGlobalTransferAuditModalSaving] =
      useState(false);
    const [globalTransferAuditAsset, setGlobalTransferAuditAsset] =
      useState<string>("");
    const [
      globalTransferAuditRulesByAsset,
      setGlobalTransferAuditRulesByAsset,
    ] = useState<Record<string, CurrencyWithdrawAuditRuleItem[]>>({});

    const ensureCurrencyAssetCodes = useCallback(async () => {
      if (currencyAssetCodes.length > 0) return currencyAssetCodes;
      const codes = new Set<string>();
      let page = 1;
      while (true) {
        const res = await adminListCurrencies(
          { page, page_size: PAGE_SIZE },
          { skipErrorHandler: true }
        );
        if (!res?.success) break;
        const list = res.data?.currencies || [];
        for (const c of list) {
          const code = normalizeCode((c as any).asset_code || (c as any).code);
          if (code) codes.add(code);
        }
        if (list.length < PAGE_SIZE) break;
        page += 1;
        if (page > 50) break;
      }
      const arr = Array.from(codes).sort();
      setCurrencyAssetCodes(arr);
      return arr;
    }, [currencyAssetCodes.length]);

    const mergeCurrencyAssetCodes = useCallback(
      async (extra: string[] = []) => {
        let base: string[] = [];
        try {
          base = await ensureCurrencyAssetCodes();
        } catch {
          base = currencyAssetCodes;
        }
        const merged = uniqSorted([...base, ...extra]);
        setCurrencyAssetCodes(merged);
        return merged;
      },
      [ensureCurrencyAssetCodes, currencyAssetCodes]
    );

    const globalChainCodes = useMemo(
      () => currencyAssetCodes,
      [currencyAssetCodes]
    );
    const globalAuditChainCodes = useMemo(
      () => currencyAssetCodes,
      [currencyAssetCodes]
    );
    const globalTransferAuditAssetCodes = useMemo(
      () => currencyAssetCodes,
      [currencyAssetCodes]
    );

    const setGlobalRules = useCallback(
      (updater: React.SetStateAction<CurrencyWithdrawFeeRuleItem[]>) => {
        if (!globalChain) return;
        setGlobalRulesByChain((prev) => {
          const current = prev[globalChain] || [];
          const next =
            typeof updater === "function"
              ? (
                  updater as (
                    p: CurrencyWithdrawFeeRuleItem[]
                  ) => CurrencyWithdrawFeeRuleItem[]
                )(current)
              : updater;
          return { ...prev, [globalChain]: next };
        });
      },
      [globalChain]
    );

    const setGlobalAuditRules = useCallback(
      (updater: React.SetStateAction<CurrencyWithdrawAuditRuleItem[]>) => {
        if (!globalAuditChain) return;
        setGlobalAuditRulesByChain((prev) => {
          const current = prev[globalAuditChain] || [];
          const next =
            typeof updater === "function"
              ? (
                  updater as (
                    p: CurrencyWithdrawAuditRuleItem[]
                  ) => CurrencyWithdrawAuditRuleItem[]
                )(current)
              : updater;
          return { ...prev, [globalAuditChain]: next };
        });
      },
      [globalAuditChain]
    );

    const setGlobalTransferAuditRules = useCallback(
      (updater: React.SetStateAction<CurrencyWithdrawAuditRuleItem[]>) => {
        if (!globalTransferAuditAsset) return;
        setGlobalTransferAuditRulesByAsset((prev) => {
          const current = prev[globalTransferAuditAsset] || [];
          const next =
            typeof updater === "function"
              ? (
                  updater as (
                    p: CurrencyWithdrawAuditRuleItem[]
                  ) => CurrencyWithdrawAuditRuleItem[]
                )(current)
              : updater;
          return { ...prev, [globalTransferAuditAsset]: next };
        });
      },
      [globalTransferAuditAsset]
    );

    const openFee = useCallback(async () => {
      if (!canGlobalGet) return;
      setGlobalModalOpen(true);
      setGlobalModalLoading(true);
      try {
        const res = await adminGetCurrencyGlobalWithdrawFee({
          skipErrorHandler: true,
        });
        if (!res?.success) throw new Error(res?.message || "");

        const raw: Record<string, CurrencyWithdrawFeeRuleItem[]> = {};
        const extraCodes: string[] = [];

        for (const g of res.data?.fee_rules || []) {
          const code = normalizeCode(
            (g as any).asset_code || (g as any).chain_code
          );
          if (!code) continue;
          extraCodes.push(code);
          raw[code] = normalizeRules((g as any).rules);
        }

        const assets = await mergeCurrencyAssetCodes(extraCodes);
        const aligned: Record<string, CurrencyWithdrawFeeRuleItem[]> = {};
        for (const code of assets) aligned[code] = raw[code] || [];
        setGlobalRulesByChain(aligned);
        setGlobalChain((prev) => {
          if (prev && aligned[prev]) return prev;
          return assets[0] || "";
        });
      } catch (e: any) {
        message.error(
          e?.message ||
            t(
              "pages.assets.currencies.messages.loadGlobalFailed",
              "Failed to load global fees"
            )
        );
        setGlobalModalOpen(false);
      } finally {
        setGlobalModalLoading(false);
      }
    }, [canGlobalGet, mergeCurrencyAssetCodes, message, t]);

    const saveGlobal = useCallback(async () => {
      if (!canGlobalUpdate) return;
      if (!globalChain) return;
      setGlobalModalSaving(true);
      try {
        const assets = uniqSorted([
          ...Object.keys(globalRulesByChain || {}),
          ...currencyAssetCodes,
        ]);
        const fee_rules: CurrencyChainFeeRules[] = assets.map((code) => ({
          chain_code: code,
          rules: (globalRulesByChain[code] || []).map((r) => ({
            rule_type: r.rule_type,
            value: toDecimalStr(r.value ?? "", 6, 40),
            min_amount: toDecimalStr(r.min_amount ?? "0", 6, 40) || "0",
            max_amount: toDecimalStr(r.max_amount ?? "", 6, 40),

            min_fee: toDecimalStr(r.min_fee ?? "", 6, 40),
            max_fee: toDecimalStr(r.max_fee ?? "", 6, 40),
            enabled: !!r.enabled,
            sort_order: Number(toDigits(r.sort_order ?? 0) || 0),
          })),
        }));
        const res = await adminUpdateCurrencyGlobalWithdrawFee(
          { fee_rules },
          { skipErrorHandler: true }
        );
        if (!res?.success) throw new Error(res?.message || "");
        message.success(
          t("pages.assets.currencies.messages.savedGlobal", "Saved global fees")
        );
        setGlobalModalOpen(false);
      } catch (e: any) {
        message.error(
          e?.message ||
            t(
              "pages.assets.currencies.messages.saveGlobalFailed",
              "Save failed"
            )
        );
      } finally {
        setGlobalModalSaving(false);
      }
    }, [
      canGlobalUpdate,
      currencyAssetCodes,
      globalChain,
      globalRulesByChain,
      message,
      t,
    ]);

    const openAudit = useCallback(async () => {
      if (!canGlobalAuditGet) return;
      setGlobalAuditModalOpen(true);
      setGlobalAuditModalLoading(true);
      try {
        const res = await adminGetCurrencyGlobalWithdrawAudit({
          skipErrorHandler: true,
        });
        if (!res?.success) throw new Error(res?.message || "");

        const raw: Record<string, CurrencyWithdrawAuditRuleItem[]> = {};
        const extraCodes: string[] = [];

        for (const g of res.data?.audit_rules || []) {
          const code = normalizeCode(
            (g as any).asset_code || (g as any).chain_code
          );
          if (!code) continue;
          extraCodes.push(code);
          raw[code] = normalizeAuditRules((g as any).rules);
        }

        const assets = await mergeCurrencyAssetCodes(extraCodes);
        const aligned: Record<string, CurrencyWithdrawAuditRuleItem[]> = {};
        for (const code of assets) aligned[code] = raw[code] || [];
        setGlobalAuditRulesByChain(aligned);
        setGlobalAuditChain((prev) => {
          if (prev && aligned[prev]) return prev;
          return assets[0] || "";
        });
      } catch (e: any) {
        message.error(
          e?.message ||
            t(
              "pages.assets.currencies.messages.loadGlobalAuditFailed",
              "Failed to load global audit rules"
            )
        );
        setGlobalAuditModalOpen(false);
      } finally {
        setGlobalAuditModalLoading(false);
      }
    }, [canGlobalAuditGet, mergeCurrencyAssetCodes, message, t]);

    const saveGlobalAudit = useCallback(async () => {
      if (!canGlobalAuditUpdate) return;
      if (!globalAuditChain) return;
      setGlobalAuditModalSaving(true);
      try {
        const assets = uniqSorted([
          ...Object.keys(globalAuditRulesByChain || {}),
          ...currencyAssetCodes,
        ]);
        const audit_rules: CurrencyChainAuditRules[] = assets.map((code) => ({
          chain_code: code,
          rules: (globalAuditRulesByChain[code] || []).map((r) => ({
            min_amount: toDecimalStr(r.min_amount ?? "0", 6, 40) || "0",
            max_amount: toDecimalStr(r.max_amount ?? "", 6, 40),
            strategy: r.strategy,
            enabled: !!r.enabled,
            sort_order: Number(toDigits(r.sort_order ?? 0) || 0),
          })),
        }));
        const res = await adminUpdateCurrencyGlobalWithdrawAudit(
          { audit_rules },
          { skipErrorHandler: true }
        );
        if (!res?.success) throw new Error(res?.message || "");
        message.success(
          t(
            "pages.assets.currencies.messages.savedGlobalAudit",
            "Saved global audit rules"
          )
        );
        setGlobalAuditModalOpen(false);
      } catch (e: any) {
        message.error(
          e?.message ||
            t(
              "pages.assets.currencies.messages.saveGlobalAuditFailed",
              "Save failed"
            )
        );
      } finally {
        setGlobalAuditModalSaving(false);
      }
    }, [
      canGlobalAuditUpdate,
      currencyAssetCodes,
      globalAuditChain,
      globalAuditRulesByChain,
      message,
      t,
    ]);

    const openTransferAudit = useCallback(async () => {
      if (!canGlobalTransferAuditGet) return;
      setGlobalTransferAuditModalOpen(true);
      setGlobalTransferAuditModalLoading(true);
      try {
        const res = await adminGetCurrencyGlobalTransferAudit({
          skipErrorHandler: true,
        });
        if (!res?.success) throw new Error(res?.message || "");

        const raw: Record<string, CurrencyWithdrawAuditRuleItem[]> = {};
        const extraCodes: string[] = [];

        for (const g of res.data?.audit_rules || []) {
          const code = normalizeCode((g as any).asset_code);
          if (!code) continue;
          extraCodes.push(code);
          raw[code] = normalizeAuditRules((g as any).rules);
        }

        const assets = await mergeCurrencyAssetCodes(extraCodes);
        const aligned: Record<string, CurrencyWithdrawAuditRuleItem[]> = {};
        for (const code of assets) aligned[code] = raw[code] || [];
        setGlobalTransferAuditRulesByAsset(aligned);
        setGlobalTransferAuditAsset((prev) => {
          if (prev && aligned[prev]) return prev;
          return assets[0] || "";
        });
      } catch (e: any) {
        message.error(
          e?.message ||
            t(
              "pages.assets.currencies.messages.loadGlobalTransferAuditFailed",
              "加载全局转账审核规则失败"
            )
        );
        setGlobalTransferAuditModalOpen(false);
      } finally {
        setGlobalTransferAuditModalLoading(false);
      }
    }, [canGlobalTransferAuditGet, mergeCurrencyAssetCodes, message, t]);

    const saveGlobalTransferAudit = useCallback(async () => {
      if (!canGlobalTransferAuditUpdate) return;
      if (!globalTransferAuditAsset) return;
      setGlobalTransferAuditModalSaving(true);
      try {
        const assets = uniqSorted([
          ...Object.keys(globalTransferAuditRulesByAsset || {}),
          ...currencyAssetCodes,
        ]);
        const audit_rules: CurrencyAssetTransferAuditRules[] = assets.map(
          (asset_code) => ({
            asset_code,
            rules: (globalTransferAuditRulesByAsset[asset_code] || []).map(
              (r) => ({
                min_amount: toDecimalStr(r.min_amount ?? "0", 6, 40) || "0",
                max_amount: toDecimalStr(r.max_amount ?? "", 6, 40),
                strategy: r.strategy || "auto",
                enabled: !!r.enabled,
                sort_order: Number(toDigits(r.sort_order ?? 0) || 0),
              })
            ),
          })
        );
        const res = await adminUpdateCurrencyGlobalTransferAudit(
          { audit_rules },
          { skipErrorHandler: true }
        );
        if (!res?.success) throw new Error(res?.message || "");
        message.success(
          t(
            "pages.assets.currencies.messages.savedGlobalTransferAudit",
            "已保存全局转账审核规则"
          )
        );
        setGlobalTransferAuditModalOpen(false);
      } catch (e: any) {
        message.error(
          e?.message ||
            t(
              "pages.assets.currencies.messages.saveGlobalTransferAuditFailed",
              "保存失败"
            )
        );
      } finally {
        setGlobalTransferAuditModalSaving(false);
      }
    }, [
      canGlobalTransferAuditUpdate,
      currencyAssetCodes,
      globalTransferAuditAsset,
      globalTransferAuditRulesByAsset,
      message,
      t,
    ]);

    useImperativeHandle(
      ref,
      () => ({ openFee, openAudit, openTransferAudit }),
      [openFee, openAudit, openTransferAudit]
    );

    return (
      <>
        <Modal
          title={`${t(
            "pages.assets.currencies.global.rulesModalTitle",
            "Global withdraw fee rules"
          )} - ${globalChain}`}
          open={globalModalOpen}
          onCancel={() => setGlobalModalOpen(false)}
          onOk={saveGlobal}
          confirmLoading={globalModalSaving}
          okButtonProps={{
            disabled: !canGlobalUpdate || !globalChain || globalModalLoading,
          }}
          width={980}
          zIndex={1350}
        >
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message={t(
              "pages.assets.currencies.feeModal.noteGlobal",
              "Note: global fees are used when “Use global” is enabled; when disabled and chain rules are empty, the system still falls back to global fees."
            )}
          />

          <Space style={{ marginBottom: 12 }} wrap>
            <Select
              style={{ width: 220 }}
              value={globalChain || undefined}
              options={globalChainCodes.map((cc) => ({ label: cc, value: cc }))}
              onChange={(v) => setGlobalChain(v || "")}
              disabled={globalModalLoading}
              placeholder={t("pages.assets.currencies.global.asset", "Asset")}
              showSearch
              allowClear
              filterOption={(input, option) =>
                (option?.value || "")
                  .toString()
                  .toUpperCase()
                  .includes((input || "").toUpperCase())
              }
            />
            <Button
              size="small"
              onClick={() =>
                setGlobalRules((prev) => [
                  ...prev,
                  {
                    temp_id: `tmp-${Date.now()}`,
                    rule_type: "fixed",
                    value: "0",
                    min_amount: "0",
                    max_amount: "",
                    min_fee: "",
                    max_fee: "",
                    enabled: true,
                    sort_order: prev.length,
                  },
                ])
              }
              disabled={!canGlobalUpdate || !globalChain || globalModalLoading}
            >
              {t("pages.assets.currencies.feeModal.add", "Add rule")}
            </Button>
          </Space>

          <WithdrawFeeRulesTable
            rules={globalRulesByChain[globalChain] || []}
            setRules={setGlobalRules}
            canEdit={canGlobalUpdate}
            t={t}
          />
        </Modal>

        <Modal
          title={`${t(
            "pages.assets.currencies.global.auditRulesModalTitle",
            "Global withdraw audit rules"
          )} - ${globalAuditChain}`}
          open={globalAuditModalOpen}
          onCancel={() => setGlobalAuditModalOpen(false)}
          onOk={saveGlobalAudit}
          confirmLoading={globalAuditModalSaving}
          okButtonProps={{
            disabled:
              !canGlobalAuditUpdate ||
              !globalAuditChain ||
              globalAuditModalLoading,
          }}
          width={980}
          zIndex={1375}
        >
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message={t(
              "pages.assets.currencies.auditModal.noteGlobal",
              "Note: global audit rules are used when “Use global” is enabled; when disabled and chain rules are empty, the system still falls back to global audit rules. If no rule matches, withdrawals are rejected."
            )}
          />

          <Space style={{ marginBottom: 12 }} wrap>
            <Select
              style={{ width: 220 }}
              value={globalAuditChain || undefined}
              options={globalAuditChainCodes.map((cc) => ({
                label: cc,
                value: cc,
              }))}
              onChange={(v) => setGlobalAuditChain(v || "")}
              disabled={globalAuditModalLoading}
              placeholder={t("pages.assets.currencies.global.asset", "Asset")}
              showSearch
              allowClear
              filterOption={(input, option) =>
                (option?.value || "")
                  .toString()
                  .toUpperCase()
                  .includes((input || "").toUpperCase())
              }
            />
            <Button
              size="small"
              onClick={() =>
                setGlobalAuditRules((prev) => [
                  ...prev,
                  {
                    temp_id: `tmp-${Date.now()}`,
                    min_amount: "0",
                    max_amount: "",
                    strategy: "auto",
                    enabled: true,
                    sort_order: prev.length,
                  },
                ])
              }
              disabled={
                !canGlobalAuditUpdate ||
                !globalAuditChain ||
                globalAuditModalLoading
              }
            >
              {t("pages.assets.currencies.feeModal.add", "Add rule")}
            </Button>
          </Space>

          <WithdrawAuditRulesTable
            rules={globalAuditRulesByChain[globalAuditChain] || []}
            setRules={setGlobalAuditRules}
            canEdit={canGlobalAuditUpdate}
            t={t}
          />
        </Modal>

        <Modal
          title={`${t(
            "pages.assets.currencies.global.transferAuditRulesModalTitle",
            "Global transfer audit rules"
          )} - ${globalTransferAuditAsset}`}
          open={globalTransferAuditModalOpen}
          onCancel={() => setGlobalTransferAuditModalOpen(false)}
          onOk={saveGlobalTransferAudit}
          confirmLoading={globalTransferAuditModalSaving}
          okButtonProps={{
            disabled:
              !canGlobalTransferAuditUpdate ||
              !globalTransferAuditAsset ||
              globalTransferAuditModalLoading,
          }}
          width={980}
          zIndex={1400}
        >
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message={t(
              "pages.assets.currencies.transferAuditModal.noteGlobal",
              '提示：全局转账审核规则按资产分组配置。当币种启用"使用全局"时优先使用该资产的全局规则；若未命中任何规则则拒绝转账。'
            )}
          />

          <Space style={{ marginBottom: 12 }} wrap>
            <Select
              style={{ width: 220 }}
              value={globalTransferAuditAsset || undefined}
              options={globalTransferAuditAssetCodes.map((ac) => ({
                label: ac,
                value: ac,
              }))}
              onChange={(v) => setGlobalTransferAuditAsset(v || "")}
              disabled={globalTransferAuditModalLoading}
              placeholder={t("pages.assets.currencies.global.asset", "资产")}
              showSearch
              allowClear
              filterOption={(input, option) =>
                (option?.value || "")
                  .toString()
                  .toUpperCase()
                  .includes((input || "").toUpperCase())
              }
            />
            <Button
              size="small"
              onClick={() =>
                setGlobalTransferAuditRules((prev) => [
                  ...prev,
                  {
                    temp_id: `tmp-${Date.now()}`,
                    min_amount: "0",
                    max_amount: "",
                    strategy: "auto",
                    enabled: true,
                    sort_order: prev.length,
                  },
                ])
              }
              disabled={
                !canGlobalTransferAuditUpdate ||
                !globalTransferAuditAsset ||
                globalTransferAuditModalLoading
              }
            >
              {t("pages.assets.currencies.feeModal.add", "Add rule")}
            </Button>
          </Space>

          <WithdrawAuditRulesTable
            rules={
              globalTransferAuditRulesByAsset[globalTransferAuditAsset] || []
            }
            setRules={setGlobalTransferAuditRules}
            canEdit={canGlobalTransferAuditUpdate}
            t={t}
            hiddenStrategies={["manual_manual"]}
          />
        </Modal>
      </>
    );
  }
);

export default GlobalRulesModals;
export type { Props as GlobalRulesModalsProps };
