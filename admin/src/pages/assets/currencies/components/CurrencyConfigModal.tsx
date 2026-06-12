import {
  adminUpdateChainIcon,
  customApiV1AdminChainsIconUploadPOST,
} from "@/api/generated/assets";
import type {
  AdminCurrencyChainSettingItem,
  AdminCurrencyFeatureSettings,
  AdminCurrencyItem,
  AdminCurrencyWithdrawAuditRuleItem,
  AdminCurrencyWithdrawFeeRuleItem,
} from "@/api/generated/schemas";
import type { ProColumns } from "@ant-design/pro-components";
import { ProTable } from "@ant-design/pro-components";
import {
  Alert,
  App,
  Avatar,
  Button,
  Card,
  Divider,
  Drawer,
  Input,
  Modal,
  Popconfirm,
  Space,
  Switch,
  Typography,
  Upload,
  type UploadProps,
} from "antd";
import { createStyles } from "antd-style";
import React, { useCallback, useMemo, useState } from "react";
import type { I18nT } from "../types";
import { normalizeAuditRules, normalizeRules } from "../utils";
import WithdrawAuditRulesTable from "./WithdrawAuditRulesTable";
import WithdrawFeeRulesTable from "./WithdrawFeeRulesTable";

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
const DECIMAL_MAX_LEN = 40;
const DECIMAL_MAX_SCALE = 6;
const useStyles = createStyles(({ token }) => ({
  modalBody: {
    display: "flex",
    flexDirection: "column",
    gap: token.marginMD,
    width: "100%",
  },
  gridCard: {
    height: "100%",
    borderRadius: token.borderRadiusLG,
  },
  featureSection: {
    display: "flex",
    flexDirection: "column",
    gap: token.marginXS,
  },
  featureGrid: {
    display: "grid",
    gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
    gap: token.marginSM,
    "@media (max-width: 576px)": {
      gridTemplateColumns: "minmax(0, 1fr)",
    },
  },
  featureItem: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    gap: token.marginSM,
    padding: `${token.paddingXS}px ${token.paddingSM}px`,
    border: `1px solid ${token.colorBorderSecondary}`,
    borderRadius: token.borderRadius,
    background: token.colorFillQuaternary,
    minHeight: 40,
  },
  featureItemLabel: {
    fontSize: token.fontSize,
    color: token.colorText,
  },
  feeRow: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    gap: token.marginSM,
    padding: `${token.paddingXS}px ${token.paddingSM}px`,
    border: `1px solid ${token.colorBorderSecondary}`,
    borderRadius: token.borderRadius,
    background: token.colorFillQuaternary,
  },
  chainTable: {
    width: "100%",
  },
}));

type Props = {
  open: boolean;
  confirmLoading: boolean;
  currency: CurrencyItem | null;
  settings: CurrencyFeatureSettings;
  setSettings: React.Dispatch<React.SetStateAction<CurrencyFeatureSettings>>;
  chains: CurrencyChainSettingItem[];
  setChains: React.Dispatch<React.SetStateAction<CurrencyChainSettingItem[]>>;
  feeRulesByChain: Record<string, CurrencyWithdrawFeeRuleItem[]>;
  setFeeRulesByChain: React.Dispatch<
    React.SetStateAction<Record<string, CurrencyWithdrawFeeRuleItem[]>>
  >;
  auditRulesByChain: Record<string, CurrencyWithdrawAuditRuleItem[]>;
  setAuditRulesByChain: React.Dispatch<
    React.SetStateAction<Record<string, CurrencyWithdrawAuditRuleItem[]>>
  >;
  canUpdate: boolean;
  canGlobalGet: boolean;
  canGlobalAuditGet: boolean;
  canGlobalTransferAuditGet: boolean;
  // Handlers to open global modals rendered at page level
  openGlobalFeeModal?: () => void;
  openGlobalAuditModal?: () => void;
  openGlobalTransferAuditModal?: () => void;
  onCancel: () => void;
  onOk: () => void | Promise<void>;
  t: I18nT;
};

const CurrencyConfigModal: React.FC<Props> = ({
  open,
  confirmLoading,
  currency,
  settings,
  setSettings,
  chains,
  setChains,
  feeRulesByChain,
  setFeeRulesByChain,
  auditRulesByChain,
  setAuditRulesByChain,
  canUpdate,
  canGlobalGet,
  canGlobalAuditGet,
  canGlobalTransferAuditGet,
  openGlobalFeeModal,
  openGlobalAuditModal,
  openGlobalTransferAuditModal,
  onCancel,
  onOk,
  t,
}) => {
  const { styles } = useStyles();
  const { message } = App.useApp();
  const [feeModalOpen, setFeeModalOpen] = useState(false);
  const [feeModalChain, setFeeModalChain] = useState<string>("");
  const [feeModalRules, setFeeModalRules] = useState<
    CurrencyWithdrawFeeRuleItem[]
  >([]);

  const [auditModalOpen, setAuditModalOpen] = useState(false);
  const [auditModalChain, setAuditModalChain] = useState<string>("");
  const [auditModalRules, setAuditModalRules] = useState<
    CurrencyWithdrawAuditRuleItem[]
  >([]);

  const handleClose = useCallback(() => {
    setFeeModalOpen(false);
    setAuditModalOpen(false);

    onCancel();
  }, [onCancel]);

  const openFeeModal = useCallback(
    (chainCode: string) => {
      setFeeModalChain(chainCode);
      setFeeModalRules(normalizeRules(feeRulesByChain[chainCode]));
      setFeeModalOpen(true);
    },
    [feeRulesByChain]
  );
  function normalizeDecimalInput(raw: string): string {
    if (raw == null) return "";
    let s = String(raw).trim();

    s = s.replace(/[^\d.]/g, "");

    const firstDot = s.indexOf(".");
    if (firstDot !== -1) {
      s = s.slice(0, firstDot + 1) + s.slice(firstDot + 1).replace(/\./g, "");
    }

    if (firstDot !== -1) {
      const [i, d = ""] = s.split(".");
      s = `${i}.${d.slice(0, DECIMAL_MAX_SCALE)}`;
    }

    s = s.replace(/^0+(?=\d)/, "");
    if (s.startsWith(".")) s = `0${s}`;

    if (s.length > DECIMAL_MAX_LEN) s = s.slice(0, DECIMAL_MAX_LEN);

    return s;
  }

  function _decimalToNumberOrUndef(v: any): number | undefined {
    const s = normalizeDecimalInput(v ?? "");
    if (!s) return undefined;
    const n = Number(s);
    if (!Number.isFinite(n)) return undefined;
    return n;
  }

  function _assertDecimalValid(label: string, v: any): string | null {
    const s = String(v ?? "").trim();
    if (!s) return null;

    const normalized = normalizeDecimalInput(s);

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
  const saveFeeModal = useCallback(() => {
    setFeeRulesByChain((prev) => ({ ...prev, [feeModalChain]: feeModalRules }));
    setFeeModalOpen(false);
  }, [feeModalChain, feeModalRules, setFeeRulesByChain]);

  const openAuditModal = useCallback(
    (chainCode: string) => {
      setAuditModalChain(chainCode);
      setAuditModalRules(normalizeAuditRules(auditRulesByChain[chainCode]));
      setAuditModalOpen(true);
    },
    [auditRulesByChain]
  );

  const saveAuditModal = useCallback(() => {
    setAuditRulesByChain((prev) => ({
      ...prev,
      [auditModalChain]: auditModalRules,
    }));
    setAuditModalOpen(false);
  }, [auditModalChain, auditModalRules, setAuditRulesByChain]);

  const deleteChain = useCallback(
    (row: CurrencyChainSettingItem) => {
      setChains((prev) => prev.filter((x) => x.temp_id !== row.temp_id));
      const code = (row.chain_code || "").toUpperCase();
      if (!code) return;
      setFeeRulesByChain((prev) => {
        const next = { ...prev };
        delete next[code];
        return next;
      });
      setAuditRulesByChain((prev) => {
        const next = { ...prev };
        delete next[code];
        return next;
      });
    },
    [setAuditRulesByChain, setChains, setFeeRulesByChain]
  );

  const handleAddChain = useCallback(() => {
    setChains((prev) => {
      const newId = `tmp-${Date.now()}`;
      return [
        ...prev,
        {
          chain_code: "",
          chain_name: "",
          contract_address: "",
          status: 1,
          deposit_enabled: true,
          withdraw_enabled: true,
          web3_asset_display_enabled: true,
          temp_id: newId,
        },
      ];
    });
  }, [setChains]);

  const updateChainField = useCallback(
    (
      row: CurrencyChainSettingItem,
      key: keyof CurrencyChainSettingItem,
      value: any
    ) => {
      const normalizedValue =
        key === "chain_code" ? String(value || "").toUpperCase() : value;
      setChains((prev) => {
        return prev.map((x) => {
          if (
            (x.chain_code === row.chain_code && !row.temp_id) ||
            x.temp_id === row.temp_id
          ) {
            return { ...x, [key]: normalizedValue } as CurrencyChainSettingItem;
          }
          return x;
        });
      });

      if (key === "chain_code") {
        const newCode = String(value || "").toUpperCase();
        const oldCode = row.chain_code?.toUpperCase?.() || "";
        if (newCode && newCode !== oldCode) {
          setFeeRulesByChain((prev) => {
            if (!prev[oldCode]) return prev;
            const next = { ...prev };
            if (!next[newCode]) {
              next[newCode] = next[oldCode];
            }
            delete next[oldCode];
            return next;
          });
          setAuditRulesByChain((prev) => {
            if (!prev[oldCode]) return prev;
            const next = { ...prev };
            if (!next[newCode]) {
              next[newCode] = next[oldCode];
            }
            delete next[oldCode];
            return next;
          });
        }
      }
    },
    [setAuditRulesByChain, setChains, setFeeRulesByChain]
  );

  const chainTableColumns = useMemo<ProColumns<CurrencyChainSettingItem>[]>(
    () => [
      {
        title: t("pages.assets.currencies.config.chain", "Chain"),
        dataIndex: "chain_code",
        width: 110,
        ellipsis: true,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Input
            value={row.chain_code}
            onChange={(e) =>
              updateChainField(row, "chain_code", e.target.value)
            }
            disabled={!canUpdate}
            placeholder={t(
              "pages.assets.currencies.config.chainPlaceholder",
              "e.g. TRON"
            )}
            size="small"
          />
        ),
      },
      {
        title: t("pages.assets.currencies.config.chainLogo", "Logo"),
        dataIndex: "chain_icon_url",
        width: 140,
        render: (_: any, row: CurrencyChainSettingItem) => {
          const chainCode = (row.chain_code || "").toUpperCase();
          const url = (row.chain_icon_url || "").trim();
          const uploadProps: UploadProps = {
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
              const allowed = [
                "image/png",
                "image/jpeg",
                "image/webp",
                "image/gif",
              ];
              if (file.type && !allowed.includes(file.type)) {
                message.error(
                  t(
                    "pages.assets.currencies.messages.unsupportedFileType",
                    "Unsupported file type"
                  )
                );
                return false;
              }
              if (!chainCode) {
                message.error(
                  t(
                    "pages.assets.currencies.messages.chainCodeRequired",
                    "Chain code is required"
                  )
                );
                return false;
              }
              return true;
            },
            customRequest: async ({ file, onSuccess, onError }) => {
              try {
                const up = await customApiV1AdminChainsIconUploadPOST(
                  { file: file as Blob, chain_code: chainCode },
                  { skipErrorHandler: true }
                );
                const nextUrl = up?.data?.url || "";
                if (!up?.success || !nextUrl)
                  throw new Error(up?.message || "");

                const res = await adminUpdateChainIcon(
                  { chainCode },
                  { icon_url: nextUrl },
                  { skipErrorHandler: true }
                );
                if (!res?.success) throw new Error(res?.message || "");

                updateChainField(row, "chain_icon_url", nextUrl);
                message.success(
                  t("pages.assets.currencies.messages.saved", "Saved")
                );
                onSuccess?.({ url: nextUrl }, undefined as any);
              } catch (e: any) {
                message.error(
                  e?.message ||
                    t(
                      "pages.assets.currencies.messages.saveFailed",
                      "Save failed"
                    )
                );
                onError?.(e);
              }
            },
          };

          return (
            <Space>
              <Avatar shape="square" size={24} src={url || undefined}>
                {(chainCode || "?").slice(0, 1)}
              </Avatar>
              {canUpdate && (
                <Upload {...uploadProps}>
                  <Button size="small" disabled={!chainCode}>
                    {t("pages.assets.currencies.actions.uploadIcon", "Upload")}
                  </Button>
                </Upload>
              )}
            </Space>
          );
        },
      },
      {
        title: t("pages.assets.currencies.config.chainName", "Name"),
        dataIndex: "chain_name",
        width: 140,
        ellipsis: true,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Input
            value={row.chain_name}
            onChange={(e) =>
              updateChainField(row, "chain_name", e.target.value)
            }
            disabled={!canUpdate}
            placeholder={t(
              "pages.assets.currencies.config.chainNamePlaceholder",
              "e.g. TRON Mainnet"
            )}
            size="small"
          />
        ),
      },
      {
        title: t("pages.assets.currencies.config.contractAddress", "Contract"),
        dataIndex: "contract_address",
        width: 220,
        ellipsis: true,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Input
            value={row.contract_address}
            onChange={(e) =>
              updateChainField(row, "contract_address", e.target.value)
            }
            disabled={!canUpdate}
            placeholder={t(
              "pages.assets.currencies.config.contractAddressPlaceholder",
              "Optional (native empty), e.g. 0x…"
            )}
            size="small"
          />
        ),
      },
      {
        title: t("pages.assets.currencies.config.chainStatus", "Enabled"),
        dataIndex: "status",
        width: 96,
        align: "center" as const,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Switch
            checked={row.status === 1}
            onChange={(v) =>
              setChains((prev) =>
                prev.map((x) =>
                  (x.chain_code === row.chain_code && !row.temp_id) ||
                  x.temp_id === row.temp_id
                    ? { ...x, status: v ? 1 : 2 }
                    : x
                )
              )
            }
            disabled={!canUpdate}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.config.deposit", "Deposit"),
        dataIndex: "deposit_enabled",
        width: 96,
        align: "center" as const,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Switch
            checked={!!row.deposit_enabled}
            onChange={(v) =>
              setChains((prev) =>
                prev.map((x) =>
                  (x.chain_code === row.chain_code && !row.temp_id) ||
                  x.temp_id === row.temp_id
                    ? { ...x, deposit_enabled: v }
                    : x
                )
              )
            }
            disabled={!canUpdate}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.config.withdraw", "Withdraw"),
        dataIndex: "withdraw_enabled",
        width: 96,
        align: "center" as const,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Switch
            checked={!!row.withdraw_enabled}
            onChange={(v) =>
              setChains((prev) =>
                prev.map((x) =>
                  (x.chain_code === row.chain_code && !row.temp_id) ||
                  x.temp_id === row.temp_id
                    ? { ...x, withdraw_enabled: v }
                    : x
                )
              )
            }
            disabled={!canUpdate}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.config.web3Display", "Web3 Display"),
        dataIndex: "web3_asset_display_enabled",
        width: 110,
        align: "center" as const,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Switch
            checked={!!row.web3_asset_display_enabled}
            onChange={(v) =>
              setChains((prev) =>
                prev.map((x) =>
                  (x.chain_code === row.chain_code && !row.temp_id) ||
                  x.temp_id === row.temp_id
                    ? { ...x, web3_asset_display_enabled: v }
                    : x
                )
              )
            }
            disabled={!canUpdate}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.config.minWithdraw", "Min withdraw"),
        dataIndex: "min_withdraw_amount",
        width: 150,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Input
            value={row.min_withdraw_amount}
            onChange={(e) =>
              setChains((prev) =>
                prev.map((x) =>
                  (x.chain_code === row.chain_code && !row.temp_id) ||
                  x.temp_id === row.temp_id
                    ? { ...x, min_withdraw_amount: e.target.value }
                    : x
                )
              )
            }
            disabled={!canUpdate}
            size="small"
            placeholder={t(
              "pages.assets.currencies.placeholders.exampleAmount",
              "e.g. 10"
            )}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.config.minDeposit", "Min deposit"),
        dataIndex: "min_deposit_amount",
        width: 150,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Input
            value={row.min_deposit_amount}
            onChange={(e) =>
              setChains((prev) =>
                prev.map((x) =>
                  (x.chain_code === row.chain_code && !row.temp_id) ||
                  x.temp_id === row.temp_id
                    ? { ...x, min_deposit_amount: e.target.value }
                    : x
                )
              )
            }
            disabled={!canUpdate}
            size="small"
            placeholder={t(
              "pages.assets.currencies.placeholders.exampleAmount",
              "e.g. 10"
            )}
          />
        ),
      },
      {
        title: t("pages.assets.currencies.config.feeRules", "Fee rules"),
        width: 170,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Space size={8}>
            <Button
              size="small"
              onClick={() => row.chain_code && openFeeModal(row.chain_code)}
              disabled={!canUpdate || !row.chain_code}
            >
              {t("pages.assets.currencies.config.editFees", "Edit")}
            </Button>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {t("pages.assets.currencies.rules.count", "{count} rules", {
                count: row.chain_code
                  ? feeRulesByChain[row.chain_code]?.length ?? 0
                  : 0,
              })}
            </Typography.Text>
          </Space>
        ),
      },
      {
        title: t("pages.assets.currencies.config.auditRules", "Withdraw Audit"),
        width: 190,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Space size={8}>
            <Button
              size="small"
              onClick={() => row.chain_code && openAuditModal(row.chain_code)}
              disabled={!canUpdate || !row.chain_code}
            >
              {t("pages.assets.currencies.config.editAudit", "Edit")}
            </Button>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {t("pages.assets.currencies.rules.count", "{count} rules", {
                count: row.chain_code
                  ? auditRulesByChain[row.chain_code]?.length ?? 0
                  : 0,
              })}
            </Typography.Text>
          </Space>
        ),
      },
      // 内部转账审核规则按链配置（与提现审核一致）
      // {
      //   title: t('pages.assets.currencies.config.transferAuditRules', 'Transfer Audit'),
      //   width: 190,
      //   render: (_: any, row: CurrencyChainSettingItem) => (
      //     <Space size={8}>
      //       <Button
      //         size="small"
      //         onClick={() => row.chain_code && openTransferAuditModal(row.chain_code)}
      //         disabled={!canUpdate || !row.chain_code}
      //       >
      //         {t('pages.assets.currencies.config.editTransferAudit', 'Edit')}
      //       </Button>
      //       <Typography.Text type="secondary" style={{ fontSize: 12 }}>
      //         {t('pages.assets.currencies.rules.count', '{count} rules', {
      //           count: row.chain_code ? transferAuditRulesByChain[row.chain_code]?.length ?? 0 : 0,
      //         })}
      //       </Typography.Text>
      //     </Space>
      //   ),
      // },
      {
        title: t("pages.assets.currencies.config.actions", "Actions"),
        valueType: "option",
        width: 90,
        render: (_: any, row: CurrencyChainSettingItem) => (
          <Popconfirm
            title={t(
              "pages.assets.currencies.config.deleteChainConfirm",
              "Remove this chain?"
            )}
            onConfirm={() => deleteChain(row)}
            disabled={!canUpdate}
          >
            <Button danger size="small" disabled={!canUpdate}>
              {t("pages.assets.currencies.config.deleteChain", "Delete")}
            </Button>
          </Popconfirm>
        ),
      },
    ],
    [
      auditRulesByChain,
      canUpdate,
      deleteChain,
      feeRulesByChain,
      message,
      openAuditModal,
      openFeeModal,
      t,
      updateChainField,
    ]
  );

  return (
    <>
      <Drawer
        title={`${currency?.asset_code || ""} ${t(
          "pages.assets.currencies.config.title",
          "Config"
        )}`}
        open={open}
        onClose={handleClose}
        width={1340}
        destroyOnClose
        footer={
          <Space style={{ display: "flex", justifyContent: "flex-end" }}>
            <Button onClick={handleClose}>
              {t("pages.assets.currencies.config.cancel", "Cancel")}
            </Button>
            <Button
              type="primary"
              onClick={onOk}
              loading={confirmLoading}
              disabled={!canUpdate}
            >
              {t("pages.assets.currencies.config.ok", "OK")}
            </Button>
          </Space>
        }
      >
        <div className={styles.modalBody}>
          <Alert
            type="info"
            showIcon
            message={t(
              "pages.assets.currencies.config.desc",
              "Feature toggles take effect immediately in Business deposit/withdraw APIs."
            )}
          />

          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <Card
              title={t("pages.assets.currencies.config.features", "Features")}
              size="small"
              className={styles.gridCard}
            >
              <div className={styles.featureSection}>
                <Typography.Text strong>Web2</Typography.Text>
                <div className={styles.featureGrid}>
                  <div className={styles.featureItem}>
                    <span className={styles.featureItemLabel}>
                      {t("pages.assets.currencies.labels.deposit", "Deposit")}
                    </span>
                    <Switch
                      checked={settings.web2_deposit_enabled}
                      onChange={(v) =>
                        setSettings((s) => ({ ...s, web2_deposit_enabled: v }))
                      }
                      disabled={!canUpdate}
                    />
                  </div>
                  <div className={styles.featureItem}>
                    <span className={styles.featureItemLabel}>
                      {t("pages.assets.currencies.labels.withdraw", "Withdraw")}
                    </span>
                    <Switch
                      checked={settings.web2_withdraw_enabled}
                      onChange={(v) =>
                        setSettings((s) => ({
                          ...s,
                          web2_withdraw_enabled: v,
                        }))
                      }
                      disabled={!canUpdate}
                    />
                  </div>
                  <div className={styles.featureItem}>
                    <span className={styles.featureItemLabel}>
                      {t("pages.assets.currencies.labels.transfer", "Transfer")}
                    </span>
                    <Switch
                      checked={settings.web2_transfer_enabled}
                      onChange={(v) =>
                        setSettings((s) => ({
                          ...s,
                          web2_transfer_enabled: v,
                        }))
                      }
                      disabled={!canUpdate}
                    />
                  </div>
                </div>
              </div>
            </Card>

            <Card
              title={t("pages.assets.currencies.config.fees", "Withdraw fee")}
              size="small"
              className={styles.gridCard}
            >
              <div className={styles.feeRow}>
                <Space size={8} split={<Divider type="vertical" />}>
                  <span>
                    {t(
                      "pages.assets.currencies.config.useGlobal",
                      "Use global"
                    )}
                  </span>
                  <Switch
                    checked={settings.use_global_withdraw_fee}
                    onChange={(v) =>
                      setSettings((s) => ({
                        ...s,
                        use_global_withdraw_fee: v,
                      }))
                    }
                    disabled={!canUpdate}
                  />
                </Space>
                <Button
                  size="small"
                  onClick={openGlobalFeeModal}
                  disabled={!canGlobalGet}
                >
                  {t(
                    "pages.assets.currencies.config.viewGlobal",
                    "View global"
                  )}
                </Button>
              </div>
              <Typography.Paragraph
                type="secondary"
                style={{ margin: "10px 0 0" }}
              >
                {t(
                  "pages.assets.currencies.feeModal.noteGlobal",
                  "Note: global fees are used when “Use global” is enabled; when disabled and chain rules are empty, the system still falls back to global fees."
                )}
              </Typography.Paragraph>
            </Card>

            <Card
              title={t(
                "pages.assets.currencies.config.audit",
                "Withdraw audit"
              )}
              size="small"
              className={styles.gridCard}
            >
              <div className={styles.feeRow}>
                <Space size={8} split={<Divider type="vertical" />}>
                  <span>
                    {t(
                      "pages.assets.currencies.config.useGlobal",
                      "Use global"
                    )}
                  </span>
                  <Switch
                    checked={settings.use_global_withdraw_audit}
                    onChange={(v) =>
                      setSettings((s) => ({
                        ...s,
                        use_global_withdraw_audit: v,
                      }))
                    }
                    disabled={!canUpdate}
                  />
                </Space>
                <Button
                  size="small"
                  onClick={openGlobalAuditModal}
                  disabled={!canGlobalAuditGet}
                >
                  {t(
                    "pages.assets.currencies.config.viewGlobalAudit",
                    "View global"
                  )}
                </Button>
              </div>
              <Typography.Paragraph
                type="secondary"
                style={{ margin: "10px 0 0" }}
              >
                {t(
                  "pages.assets.currencies.auditModal.noteGlobal",
                  'Note: global audit rules are used when "Use global" is enabled; when disabled and chain rules are empty, the system still falls back to global audit rules. If no rule matches, withdrawals are rejected.'
                )}
              </Typography.Paragraph>
            </Card>

            <Card
              title={t(
                "pages.assets.currencies.config.transferAudit",
                "Transfer audit"
              )}
              size="small"
              className={styles.gridCard}
            >
              <div className={styles.feeRow}>
                <Space size={8} split={<Divider type="vertical" />}>
                  <span>
                    {t(
                      "pages.assets.currencies.config.useGlobal",
                      "Use global"
                    )}
                  </span>
                  <Switch
                    checked={settings.use_global_transfer_audit}
                    onChange={(v) =>
                      setSettings((s) => ({
                        ...s,
                        use_global_transfer_audit: v,
                      }))
                    }
                    disabled={!canUpdate}
                  />
                </Space>
                <Button
                  size="small"
                  onClick={openGlobalTransferAuditModal}
                  disabled={!canGlobalTransferAuditGet}
                >
                  {t(
                    "pages.assets.currencies.config.viewGlobalTransferAudit",
                    "View global"
                  )}
                </Button>
              </div>
              <Typography.Paragraph
                type="secondary"
                style={{ margin: "10px 0 0" }}
              >
                {t(
                  "pages.assets.currencies.transferAuditModal.noteChainConfig",
                  '提示：内部转账使用全局审核规则（按资产分组）；若转账金额未命中任何规则则拒绝转账。'
                )}
              </Typography.Paragraph>
            </Card>

            <Card
              title={t("pages.assets.currencies.config.chains", "Chains")}
              size="small"
              className={styles.gridCard}
            >
              <ProTable<CurrencyChainSettingItem>
                className={styles.chainTable}
                rowKey="temp_id"
                search={false}
                options={false}
                pagination={false}
                size="small"
                dataSource={chains}
                columns={chainTableColumns}
                scroll={{ x: 1500 }}
                toolBarRender={() => [
                  <Button
                    key="add"
                    type="dashed"
                    onClick={handleAddChain}
                    disabled={!canUpdate}
                  >
                    {t("pages.assets.currencies.config.addChain", "Add chain")}
                  </Button>,
                ]}
              />
            </Card>
          </Space>
        </div>
      </Drawer>

      <Modal
        title={`${t(
          "pages.assets.currencies.feeModal.title",
          "Withdraw fee rules"
        )} - ${feeModalChain}`}
        open={feeModalOpen}
        onCancel={() => setFeeModalOpen(false)}
        onOk={saveFeeModal}
        okButtonProps={{ disabled: !canUpdate }}
        width={980}
        zIndex={1300}
      >
        {settings.use_global_withdraw_fee && (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message={t(
              "pages.assets.currencies.feeModal.noteGlobal",
              "Note: global fees are used when “Use global” is enabled; when disabled and chain rules are empty, the system still falls back to global fees."
            )}
          />
        )}
        <Space style={{ marginBottom: 12 }}>
          <Button
            size="small"
            onClick={() =>
              setFeeModalRules((prev) => [
                ...prev,
                {
                  temp_id: `tmp-${Date.now()}`,
                  rule_type: "fixed",
                  value: "0",
                  min_fee: "",
                  max_fee: "",
                  min_amount: "",
                  max_amount: "",
                  enabled: true,
                  sort_order: prev.length,
                },
              ])
            }
            disabled={!canUpdate}
          >
            {t("pages.assets.currencies.feeModal.add", "Add rule")}
          </Button>
        </Space>

        <WithdrawFeeRulesTable
          rules={feeModalRules}
          setRules={setFeeModalRules}
          canEdit={canUpdate}
          t={t}
        />
      </Modal>

      <Modal
        title={`${t(
          "pages.assets.currencies.auditModal.title",
          "Withdraw audit rules"
        )} - ${auditModalChain}`}
        open={auditModalOpen}
        onCancel={() => setAuditModalOpen(false)}
        onOk={saveAuditModal}
        okButtonProps={{ disabled: !canUpdate }}
        width={980}
        zIndex={1325}
      >
        {settings.use_global_withdraw_audit && (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message={t(
              "pages.assets.currencies.auditModal.noteGlobal",
              "Note: global audit rules are used when “Use global” is enabled; when disabled and chain rules are empty, the system still falls back to global audit rules. If no rule matches, withdrawals are rejected."
            )}
          />
        )}

        <Space style={{ marginBottom: 12 }}>
          <Button
            size="small"
            onClick={() =>
              setAuditModalRules((prev) => [
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
            disabled={!canUpdate}
          >
            {t("pages.assets.currencies.feeModal.add", "Add rule")}
          </Button>
        </Space>

        <WithdrawAuditRulesTable
          rules={auditModalRules}
          setRules={setAuditModalRules}
          canEdit={canUpdate}
          t={t}
        />
      </Modal>

      {/* Global rules modals moved to page-level component; none here */}
    </>
  );
};

export default CurrencyConfigModal;
