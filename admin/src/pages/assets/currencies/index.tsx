import {
  adminCreateCurrency,
  adminGetCurrency,
  adminGetCurrencyConfig,
  adminGetCurrencyOverview,
  adminListCurrencies,
  adminUpdateCurrency,
  adminUpdateCurrencyConfig,
  adminUpdateCurrencyStatus,
  customApiV1AdminCurrenciesIconUploadPOST,
} from "@/api/generated/assets";
import type {
  AdminCurrencyChainAuditRules,
  AdminCurrencyChainFeeRules,
  AdminCurrencyChainSettingItem,
  AdminCurrencyFeatureSettings,
  AdminCurrencyItem,
  AdminCurrencyWithdrawAuditRuleItem,
  AdminCurrencyWithdrawFeeRuleItem,
} from "@/api/generated/schemas";
import { useRbac } from "@/hooks/useRbac";
import type { ActionType, ProColumns } from "@ant-design/pro-components";
import { PageContainer, ProTable } from "@ant-design/pro-components";
import { useIntl } from "@umijs/max";
import {
  App,
  Avatar,
  Button,
  Divider,
  Popconfirm,
  Space,
  Tag,
  Typography,
  Upload,
  type UploadProps,
} from "antd";
import React, { useMemo, useRef, useState } from "react";
import type { CreateCurrencyFormValues } from "./components/CreateCurrencyModal";
import CreateCurrencyModal from "./components/CreateCurrencyModal";
import CurrencyConfigModal from "./components/CurrencyConfigModal";
import EditCurrencyModal, {
  type EditCurrencyFormValues,
} from "./components/EditCurrencyModal";
import GlobalRulesModals, {
  type GlobalRulesModalsRef,
} from "./components/GlobalRulesModals";
import OverviewCards, {
  type CurrenciesOverview,
} from "./components/OverviewCards";
import { PERM } from "./constants";
import {
  defaultSettingsFor,
  normalizeAuditRules,
  normalizeRules,
} from "./utils";

type CurrencyItem = AdminCurrencyItem;
type CurrencyFeatureSettings = AdminCurrencyFeatureSettings;
type CurrencyChainSettingItem = AdminCurrencyChainSettingItem & {
  temp_id?: string;
};
type CurrencyWithdrawFeeRuleItem = AdminCurrencyWithdrawFeeRuleItem & {
  temp_id?: string;
};
type CurrencyWithdrawAuditRuleItem = AdminCurrencyWithdrawAuditRuleItem & {
  temp_id?: string;
};
type CurrencyChainFeeRules = AdminCurrencyChainFeeRules;
type CurrencyChainAuditRules = AdminCurrencyChainAuditRules;

const CurrenciesPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (
    id: string,
    defaultMessage: string,
    values?: Record<string, any>
  ) => intl.formatMessage({ id, defaultMessage }, values);

  const actionRef = useRef<ActionType | null>(null);
  const globalRulesRef = useRef<GlobalRulesModalsRef | null>(null);

  const [overview, setOverview] = useState<CurrenciesOverview>({});

  const [configOpen, setConfigOpen] = useState(false);
  const [configLoading, setConfigLoading] = useState(false);
  const [currentCurrency, setCurrentCurrency] = useState<CurrencyItem | null>(
    null
  );
  const [settings, setSettings] = useState<CurrencyFeatureSettings>(
    defaultSettingsFor("USDT")
  );
  const [chains, setChains] = useState<CurrencyChainSettingItem[]>([]);
  const [feeRulesByChain, setFeeRulesByChain] = useState<
    Record<string, CurrencyWithdrawFeeRuleItem[]>
  >({});
  const [auditRulesByChain, setAuditRulesByChain] = useState<
    Record<string, CurrencyWithdrawAuditRuleItem[]>
  >({});

  const [createOpen, setCreateOpen] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);

  const [editOpen, setEditOpen] = useState(false);
  const [editLoading, setEditLoading] = useState(false);
  const [editAssetCode, setEditAssetCode] = useState<string>("");
  const [editInitialValues, setEditInitialValues] =
    useState<EditCurrencyFormValues>({
      asset_code: "",
      asset_name: "",
      precision: 0,
      icon_url: "",
      status: 2,
    });

  const DECIMAL_MAX_LEN = 40;
  const DECIMAL_MAX_SCALE = 6;

  function normalizeDecimalStr(raw: unknown): string {
    if (raw === null || raw === undefined) return "";
    let s = String(raw).trim();
    if (!s) return "";

    s = s.replace(/[^\d.]/g, "");

    const firstDot = s.indexOf(".");
    if (firstDot !== -1) {
      s = s.slice(0, firstDot + 1) + s.slice(firstDot + 1).replace(/\./g, "");
    }

    if (firstDot !== -1) {
      const [i, d = ""] = s.split(".");
      s = `${i}.${d.slice(0, DECIMAL_MAX_SCALE)}`;
    }

    if (s.startsWith(".")) s = `0${s}`;

    if (s.length > DECIMAL_MAX_LEN) s = s.slice(0, DECIMAL_MAX_LEN);

    return s;
  }
  function _validateDecimalStr(label: string, v: any): string | null {
    const s = String(v ?? "").trim();
    if (!s) return null;

    const normalized = normalizeDecimalStr(s);

    if (normalized.length > DECIMAL_MAX_LEN) {
      return `${label} 长度不能超过 ${DECIMAL_MAX_LEN}`;
    }
    const dot = normalized.indexOf(".");
    if (dot !== -1 && normalized.length - dot - 1 > DECIMAL_MAX_SCALE) {
      return `${label} 小数位不能超过 ${DECIMAL_MAX_SCALE}`;
    }
    if (!/^\d+(\.\d+)?$/.test(normalized)) {
      return `${label} 不是有效数字`;
    }
    return null;
  }
  function _decimalToNumberOrUndef(v: any): number | undefined {
    const s = normalizeDecimalStr(v ?? "");
    if (!s) return undefined;
    const n = Number(s);
    if (!Number.isFinite(n)) return undefined;
    return n;
  }

  function _assertDecimalValid(label: string, v: any): string | null {
    const s = String(v ?? "").trim();
    if (!s) return null;

    const normalized = normalizeDecimalStr(s);

    if (normalized !== s.replace(/[^\d.]/g, "").trim()) {
    }

    if (normalized.length > DECIMAL_MAX_LEN) {
      return `${label} 长度不能超过 ${DECIMAL_MAX_LEN}`;
    }
    const dot = normalized.indexOf(".");
    if (dot !== -1) {
      const scale = normalized.length - dot - 1;
      if (scale > DECIMAL_MAX_SCALE) {
        return `${label} 小数位不能超过 ${DECIMAL_MAX_SCALE}`;
      }
    }
    if (Number.isNaN(Number(normalized))) {
      return `${label} 不是有效数字`;
    }
    return null;
  }

  const isValidHttpUrl = (value: string): boolean => {
    const v = (value || "").trim();
    if (!v) return true;
    try {
      const u = new URL(v);
      return u.protocol === "http:" || u.protocol === "https:";
    } catch {
      return false;
    }
  };

  const toNumber = (v: unknown) => {
    if (v === null || v === undefined) return 0;
    const n = Number(v);
    return Number.isFinite(n) ? n : 0;
  };

  const loadOverview = async () => {
    if (!canRpc(PERM.overview)) return;
    try {
      const res = await adminGetCurrencyOverview({ skipErrorHandler: true });
      if (!res?.success) return;
      setOverview({
        total: toNumber(res.data?.total),
        enabled: toNumber(res.data?.enabled),
        disabled: toNumber(res.data?.disabled),
        base: res.data?.base_currency,
      });
    } catch {}
  };

  const createCurrency = async (values: CreateCurrencyFormValues) => {
    if (!canRpc(PERM.create)) return;
    setCreateLoading(true);
    try {
      const res = await adminCreateCurrency(values, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || "");
      message.success(t("pages.assets.currencies.messages.created", "Created"));
      setCreateOpen(false);
      actionRef.current?.reload();
      loadOverview();
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.assets.currencies.messages.createFailed", "Create failed")
      );
    } finally {
      setCreateLoading(false);
    }
  };

  const openConfig = async (row: CurrencyItem) => {
    if (!canRpc(PERM.get)) return;
    if (!row.asset_code) return;
    setConfigOpen(true);
    setConfigLoading(true);
    setCurrentCurrency(row);
    try {
      const res = await adminGetCurrencyConfig(
        { assetCode: row.asset_code },
        { skipErrorHandler: true }
      );

      if (!res?.success) throw new Error(res?.message || "");
      const currency = res.data?.currency;
      const assetCode = currency?.asset_code || row.asset_code;
      setSettings(currency?.settings || defaultSettingsFor(assetCode));
      setChains(
        (res.data?.chains || []).map((c, idx) => ({
          ...c,
          temp_id: `${c.chain_code || "CHAIN"}-${idx}-${Date.now()}`,
        }))
      );
      const map: Record<string, CurrencyWithdrawFeeRuleItem[]> = {};
      for (const g of res.data?.fee_rules || []) {
        const code = (g.chain_code || "").toUpperCase();
        if (!code) continue;
        map[code] = normalizeRules(g.rules);
      }
      setFeeRulesByChain(map);

      const auditMap: Record<string, CurrencyWithdrawAuditRuleItem[]> = {};
      for (const g of res.data?.audit_rules || []) {
        const code = (g.chain_code || "").toUpperCase();
        if (!code) continue;
        auditMap[code] = normalizeAuditRules(g.rules);
      }
      setAuditRulesByChain(auditMap);

    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.assets.currencies.messages.loadFailed", "Failed to load")
      );
      setConfigOpen(false);
    } finally {
      setConfigLoading(false);
    }
  };

  const saveConfig = async () => {
    if (!currentCurrency?.asset_code) return;
    if (!canRpc(PERM.update)) return;
    const assetCode = currentCurrency.asset_code;

    const chainCodes = new Set<string>();
    for (const c of chains) {
      const code = (c.chain_code || "").trim().toUpperCase();
      if (!code) {
        message.error(
          t(
            "pages.assets.currencies.messages.chainCodeRequired",
            "Chain code is required"
          )
        );
        return;
      }
      if (chainCodes.has(code)) {
        message.error(
          t(
            "pages.assets.currencies.messages.chainCodeDuplicate",
            "Duplicate chain code"
          )
        );
        return;
      }
      chainCodes.add(code);
      if (!feeRulesByChain[code]) feeRulesByChain[code] = [];
      if (!auditRulesByChain[code]) auditRulesByChain[code] = [];
    }

    const fee_rules: CurrencyChainFeeRules[] = Object.entries(
      feeRulesByChain
    ).map(([chain_code, rules]) => ({
      chain_code,
      rules: rules.map((r) => ({
        rule_type: r.rule_type,

        min_amount: normalizeDecimalStr(r.min_amount),
        max_amount: normalizeDecimalStr(r.max_amount),

        value: normalizeDecimalStr(r.value),
        min_fee: normalizeDecimalStr(r.min_fee),
        max_fee: normalizeDecimalStr(r.max_fee),

        enabled: !!r.enabled,
        sort_order: Number(r.sort_order ?? 0),
      })),
    }));

    const audit_rules: CurrencyChainAuditRules[] = Object.entries(
      auditRulesByChain
    ).map(([chain_code, rules]) => ({
      chain_code,
      rules: rules.map((r) => ({
        min_amount: normalizeDecimalStr(r.min_amount),
        max_amount: normalizeDecimalStr(r.max_amount),

        strategy: r.strategy,
        enabled: !!r.enabled,
        sort_order: Number(r.sort_order ?? 0),
      })),
    }));

    try {
      const res = await adminUpdateCurrencyConfig(
        { assetCode },
        {
          settings,
          chains: chains.map(({ temp_id, ...c }) => ({
            ...c,
            chain_code: (c.chain_code || "").toUpperCase(),
          })),
          fee_rules,
          audit_rules,
        },
        { skipErrorHandler: true }
      );
      if (!res?.success) throw new Error(res?.message || "");
      message.success(t("pages.assets.currencies.messages.saved", "Saved"));
      setConfigOpen(false);
      actionRef.current?.reload();
      loadOverview();
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.assets.currencies.messages.saveFailed", "Save failed")
      );
    }
  };

  const openEdit = async (row: CurrencyItem) => {
    if (!canRpc(PERM.get)) return;
    const assetCode = row.asset_code;
    if (!assetCode) return;

    setEditOpen(true);
    setEditLoading(true);
    setEditAssetCode(assetCode);

    try {
      const res = await adminGetCurrency(
        { assetCode },
        { skipErrorHandler: true }
      );
      if (!res?.success) throw new Error(res?.message || "");
      const c = res.data?.currency;
      setEditInitialValues({
        asset_code: c?.asset_code || assetCode,
        asset_name: c?.asset_name || "",
        precision: Number(c?.precision ?? 0),
        icon_url: c?.icon_url || "",
        status: Number(c?.status ?? row.status ?? 2),
      });
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.assets.currencies.messages.loadFailed", "Failed to load")
      );
      setEditOpen(false);
    } finally {
      setEditLoading(false);
    }
  };

  const submitEdit = async (values: EditCurrencyFormValues) => {
    if (!canRpc(PERM.update)) return;
    if (!editAssetCode) return;

    if (values.status === 1) {
      message.error(
        t(
          "pages.assets.currencies.messages.onlyEditWhenDisabled",
          "Only editable when disabled"
        )
      );
      return;
    }

    const iconUrl = (values.icon_url || "").trim();
    if (!isValidHttpUrl(iconUrl)) {
      message.error(
        t(
          "pages.assets.currencies.messages.invalidIconUrl",
          "Invalid icon URL (http/https only)"
        )
      );
      return;
    }

    setEditLoading(true);
    try {
      const res = await adminUpdateCurrency(
        { assetCode: editAssetCode },
        {
          asset_name: values.asset_name || "",
          precision: Number(values.precision ?? 0),
          icon_url: iconUrl,
        },
        { skipErrorHandler: true }
      );
      if (!res?.success) throw new Error(res?.message || "");
      message.success(t("pages.assets.currencies.messages.saved", "Saved"));
      setEditOpen(false);
      actionRef.current?.reload();
      loadOverview();
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.assets.currencies.messages.saveFailed", "Save failed")
      );
    } finally {
      setEditLoading(false);
    }
  };

  const saveIconUrl = async (row: CurrencyItem, nextUrl: string) => {
    if (!canRpc(PERM.update)) return;
    if (!row.asset_code) return;

    if (row.status === 1) {
      message.error(
        t(
          "pages.assets.currencies.messages.onlyEditWhenDisabled",
          "Only editable when disabled"
        )
      );
      return;
    }

    const iconUrl = (nextUrl || "").trim();
    if (!isValidHttpUrl(iconUrl)) {
      message.error(
        t(
          "pages.assets.currencies.messages.invalidIconUrl",
          "Invalid icon URL (http/https only)"
        )
      );
      return;
    }

    let asset_name = row.asset_name || "";
    let precision = Number(row.precision ?? 0);

    if (
      (!asset_name || row.precision === undefined || row.precision === null) &&
      canRpc(PERM.get)
    ) {
      try {
        const r = await adminGetCurrency(
          { assetCode: row.asset_code },
          { skipErrorHandler: true }
        );
        if (r?.success) {
          asset_name = r.data?.currency?.asset_name || asset_name;
          precision = Number(r.data?.currency?.precision ?? precision);
          if (Number(r.data?.currency?.status ?? row.status) === 1) {
            message.error(
              t(
                "pages.assets.currencies.messages.onlyEditWhenDisabled",
                "Only editable when disabled"
              )
            );
            return;
          }
        }
      } catch {
        // ignore
      }
    }

    try {
      const res = await adminUpdateCurrency(
        { assetCode: row.asset_code },
        { asset_name, precision, icon_url: iconUrl },
        { skipErrorHandler: true }
      );
      if (!res?.success) throw new Error(res?.message || "");
      message.success(
        t("pages.assets.currencies.messages.iconUpdated", "Icon updated")
      );
      actionRef.current?.reload();
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.assets.currencies.messages.saveFailed", "Save failed")
      );
    }
  };

  const uploadPropsFor = (row: CurrencyItem): UploadProps => ({
    accept: "image/png,image/jpeg,image/webp,image/gif",
    showUploadList: false,
    maxCount: 1,
    beforeUpload: (file) => {
      const maxMB = 2;
      if (file.size > maxMB * 1024 * 1024) {
        message.error(
          t(
            "pages.assets.currencies.messages.fileTooLarge",
            "File too large (max {max}MB)",
            { max: maxMB }
          )
        );
        return false;
      }
      const allowed = ["image/png", "image/jpeg", "image/webp", "image/gif"];
      if (file.type && !allowed.includes(file.type)) {
        message.error(
          t(
            "pages.assets.currencies.messages.unsupportedFileType",
            "Unsupported file type"
          )
        );
        return false;
      }
      return true;
    },
    customRequest: async ({ file, onSuccess, onError }) => {
      try {
        if (!row.asset_code) throw new Error("asset_code missing");
        if (row.status === 1) {
          message.error(
            t(
              "pages.assets.currencies.messages.onlyEditWhenDisabled",
              "Only editable when disabled"
            )
          );
          throw new Error("only editable when disabled");
        }
        const res = await customApiV1AdminCurrenciesIconUploadPOST(
          { file: file as Blob, asset_code: row.asset_code },
          { skipErrorHandler: true }
        );
        const url = res?.data?.url || "";
        if (!res?.success || !url) throw new Error(res?.message || "");
        await saveIconUrl(row, url);
        onSuccess?.({ url }, undefined as any);
      } catch (e: any) {
        message.error(
          e?.message ||
            t("pages.assets.currencies.messages.uploadFailed", "Upload failed")
        );
        onError?.(e);
      }
    },
  });

  const statusTag = (status?: number) => {
    if (status === 1)
      return (
        <Tag color="green">
          {t("pages.assets.currencies.status.enabled", "Enabled")}
        </Tag>
      );
    return (
      <Tag color="default">
        {t("pages.assets.currencies.status.disabled", "Disabled")}
      </Tag>
    );
  };

  const yesNo = (v?: boolean) =>
    v ? (
      <Tag color="blue">{t("pages.assets.currencies.values.on", "ON")}</Tag>
    ) : (
      <Tag>{t("pages.assets.currencies.values.off", "OFF")}</Tag>
    );

  const columns: ProColumns<CurrencyItem>[] = useMemo(
    () => [
      {
        title: t("pages.assets.currencies.columns.asset", "Asset"),
        dataIndex: "asset_code",
        width: 120,
        copyable: true,
        hideInSearch: true,
      },
      {
        title: t("pages.assets.currencies.columns.icon", "Icon"),
        dataIndex: "icon_url",
        width: 140,
        hideInSearch: true,
        render: (_, row) => {
          const canUpdate = canRpc(PERM.update);
          const url = (row.icon_url || "").trim();
          return (
            <Space>
              <Avatar shape="square" size={24} src={url || undefined}>
                {(row.asset_code || "?").slice(0, 1)}
              </Avatar>
              {canUpdate && (
                <Upload {...uploadPropsFor(row)}>
                  <Button size="small" disabled={row.status === 1}>
                    {t("pages.assets.currencies.actions.uploadIcon", "Upload")}
                  </Button>
                </Upload>
              )}
            </Space>
          );
        },
      },
      {
        title: t("pages.assets.currencies.columns.name", "Name"),
        dataIndex: "asset_name",
        ellipsis: true,
        hideInSearch: true,
      },
      {
        title: t("pages.assets.currencies.columns.iconUrl", "Icon URL"),
        dataIndex: "icon_url",
        width: 320,
        ellipsis: true,
        hideInSearch: true,
        render: (_, row) => {
          const url = (row.icon_url || "").trim();
          const canUpdate = canRpc(PERM.update) && row.status !== 1;
          return (
            <Typography.Text
              ellipsis={{ tooltip: url || "-" }}
              editable={
                canUpdate
                  ? {
                      text: url,
                      onChange: (v) => saveIconUrl(row, v),
                      triggerType: ["icon", "text"],
                    }
                  : false
              }
            >
              {url || "-"}
            </Typography.Text>
          );
        },
      },
      {
        title: t("pages.assets.currencies.columns.status", "Status"),
        dataIndex: "status",
        width: 110,
        valueEnum: {
          0: { text: t("pages.assets.currencies.search.status.all", "All") },
          1: {
            text: t("pages.assets.currencies.search.status.enabled", "Enabled"),
          },
          2: {
            text: t(
              "pages.assets.currencies.search.status.disabled",
              "Disabled"
            ),
          },
        },
        render: (_, row) => statusTag(row.status),

        search: {
          transform: (v) => ({
            status:
              v === 0 || v === "0" || v === undefined || v === null
                ? undefined
                : Number(v),
          }),
        },
      },

      {
        title: t("pages.assets.currencies.columns.web2", "Web2"),
        dataIndex: "settings",
        hideInSearch: true,
        width: 260,
        render: (_, row) => (
          <Space split={<Divider type="vertical" />}>
            <span>
              {t("pages.assets.currencies.labels.deposit", "Deposit")}{" "}
              {yesNo(row.settings?.web2_deposit_enabled)}
            </span>
            <span>
              {t("pages.assets.currencies.labels.withdraw", "Withdraw")}{" "}
              {yesNo(row.settings?.web2_withdraw_enabled)}
            </span>
            <span>
              {t("pages.assets.currencies.labels.transfer", "Transfer")}{" "}
              {yesNo(row.settings?.web2_transfer_enabled)}
            </span>
          </Space>
        ),
      },
      {
        title: t("pages.assets.currencies.columns.chains", "Chains"),
        width: 160,
        hideInSearch: true,
        render: (_, row) => (
          <span>
            {row.chain_enabled ?? 0}/{row.chain_total ?? 0}
          </span>
        ),
      },
      {
        title: t("pages.assets.currencies.columns.actions", "Actions"),
        valueType: "option",
        width: 280,
        render: (_, row) => {
          const canUpdate = canRpc(PERM.update);
          const canStatus = canRpc(PERM.updateStatus);
          const nextStatus = row.status === 1 ? 2 : 1;
          const canEditBase = canUpdate;
          return (
            <Space>
              <Button
                size="small"
                disabled={!canRpc(PERM.get)}
                onClick={() => openConfig(row)}
              >
                {t("pages.assets.currencies.actions.config", "Config")}
              </Button>
              <Button
                size="small"
                disabled={!canEditBase}
                onClick={() => openEdit(row)}
              >
                {t("pages.assets.currencies.actions.editBase", "编辑")}
              </Button>
              <Popconfirm
                title={t(
                  "pages.assets.currencies.actions.confirmStatus",
                  "Confirm status change?"
                )}
                disabled={!canStatus}
                onConfirm={async () => {
                  try {
                    const assetCode = row.asset_code;
                    if (!assetCode) return;
                    const res = await adminUpdateCurrencyStatus(
                      { assetCode },
                      { status: nextStatus },
                      { skipErrorHandler: true }
                    );
                    if (!res?.success) throw new Error(res?.message || "");
                    message.success(
                      t("pages.assets.currencies.messages.updated", "Updated")
                    );
                    actionRef.current?.reload();
                    loadOverview();
                  } catch (e: any) {
                    message.error(
                      e?.message ||
                        t(
                          "pages.assets.currencies.messages.updateFailed",
                          "Update failed"
                        )
                    );
                  }
                }}
              >
                <Button size="small" type="primary" ghost disabled={!canStatus}>
                  {row.status === 1
                    ? t("pages.assets.currencies.actions.disable", "Disable")
                    : t("pages.assets.currencies.actions.enable", "Enable")}
                </Button>
              </Popconfirm>
              {!canUpdate && (
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {t("pages.assets.currencies.hint.noEdit", "read-only")}
                </Typography.Text>
              )}
            </Space>
          );
        },
      },
      {
        title: t("pages.assets.currencies.search.keyword", "Keyword"),
        dataIndex: "keyword",
        hideInTable: true,
        search: { transform: (v) => ({ keyword: v }) },
      },
    ],
    [canRpc, t]
  );

  return (
    <PageContainer onBack={undefined}>
      <OverviewCards overview={overview} t={t} />

      <ProTable<CurrencyItem>
        actionRef={actionRef}
        rowKey={(row) =>
          row.asset_code || row.asset_name || JSON.stringify(row)
        }
        columns={columns}
        scroll={{ x: "max-content" }}
        tableStyle={{ width: "100%" }}
        cardProps={{
          bodyStyle: { overflowX: "auto" },
        }}
        request={async (params) => {
          if (!canRpc(PERM.list)) return { success: true, data: [], total: 0 };

          await loadOverview();

          try {
            const { current, pageSize, ...rest } = params as any;

            const res = await adminListCurrencies(
              {
                page: current,
                page_size: pageSize,
                ...rest, // ✅这里就把 asset_code / asset_name / icon_url / web2_xxx / chain_code 等都带进去了
              },
              { skipErrorHandler: true }
            );

            return {
              success: !!res?.success,
              data: res.data?.currencies ?? [],
              total: toNumber(res.data?.pagination?.total),
            };
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.assets.currencies.messages.loadFailed",
                  "Failed to load"
                )
            );
            return { success: false, data: [], total: 0 };
          }
        }}
        search={{ labelWidth: 110 }}
        toolbar={{
          actions: [
            <Button
              key="create"
              size="small"
              type="primary"
              onClick={() => setCreateOpen(true)}
              disabled={!canRpc(PERM.create)}
            >
              {t("pages.assets.currencies.actions.create", "Create")}
            </Button>,
            <Button
              key="view-global-fee"
              size="small"
              onClick={() => globalRulesRef.current?.openFee()}
              disabled={!canRpc(PERM.globalGet)}
            >
              {t(
                "pages.assets.currencies.toolbar.viewGlobalFee",
                "View Global Withdrawal Fee Rules"
              )}
            </Button>,
            <Button
              key="view-global-audit"
              size="small"
              onClick={() => globalRulesRef.current?.openAudit()}
              disabled={!canRpc(PERM.globalAuditGet)}
            >
              {t(
                "pages.assets.currencies.toolbar.viewGlobalAudit",
                "View Global Withdrawal Audit Rules"
              )}
            </Button>,
            <Button
              key="view-global-transfer-audit"
              size="small"
              onClick={() => globalRulesRef.current?.openTransferAudit()}
              disabled={!canRpc(PERM.globalTransferAuditGet)}
            >
              {t(
                "pages.assets.currencies.toolbar.viewGlobalTransferAudit",
                "View Global Transfer Audit Rules"
              )}
            </Button>,
          ],
        }}
      />

      <CurrencyConfigModal
        open={configOpen}
        confirmLoading={configLoading}
        currency={currentCurrency}
        settings={settings}
        setSettings={setSettings}
        chains={chains}
        setChains={setChains}
        feeRulesByChain={feeRulesByChain}
        setFeeRulesByChain={setFeeRulesByChain}
        auditRulesByChain={auditRulesByChain}
        setAuditRulesByChain={setAuditRulesByChain}
        canUpdate={canRpc(PERM.update)}
        canGlobalGet={canRpc(PERM.globalGet)}
        canGlobalAuditGet={canRpc(PERM.globalAuditGet)}
        canGlobalTransferAuditGet={canRpc(PERM.globalTransferAuditGet)}
        openGlobalFeeModal={() => globalRulesRef.current?.openFee()}
        openGlobalAuditModal={() => globalRulesRef.current?.openAudit()}
        openGlobalTransferAuditModal={() =>
          globalRulesRef.current?.openTransferAudit()
        }
        onCancel={() => setConfigOpen(false)}
        onOk={saveConfig}
        t={t}
      />

      <GlobalRulesModals
        ref={globalRulesRef}
        t={t}
        canGlobalGet={canRpc(PERM.globalGet)}
        canGlobalUpdate={canRpc(PERM.globalUpdate)}
        canGlobalAuditGet={canRpc(PERM.globalAuditGet)}
        canGlobalAuditUpdate={canRpc(PERM.globalAuditUpdate)}
        canGlobalTransferAuditGet={canRpc(PERM.globalTransferAuditGet)}
        canGlobalTransferAuditUpdate={canRpc(PERM.globalTransferAuditUpdate)}
      />

      <CreateCurrencyModal
        open={createOpen}
        confirmLoading={createLoading}
        canUpload={canRpc(PERM.create) || canRpc(PERM.update)}
        onCancel={() => setCreateOpen(false)}
        onSubmit={createCurrency}
        t={t}
      />

      <EditCurrencyModal
        open={editOpen}
        confirmLoading={editLoading}
        assetCode={editAssetCode}
        initialValues={editInitialValues}
        canUpdate={canRpc(PERM.update)}
        onCancel={() => setEditOpen(false)}
        onSubmit={submitEdit}
        t={t}
      />
    </PageContainer>
  );
};

export default CurrenciesPage;
