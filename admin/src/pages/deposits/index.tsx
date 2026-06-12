import {
  adminListDeposits,
  adminUpdateDeposit,
} from "@/api/generated/deposits";
import type {
  AdminUpdateDepositBody,
  AdminWalletDepositItem,
} from "@/api/generated/schemas";
import { useRbac } from "@/hooks/useRbac";
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  DollarOutlined,
  WalletOutlined,
} from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-components";
import {
  type ActionType,
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProTable,
} from "@ant-design/pro-components";
import { useIntl } from "@umijs/max";
import { App, Button, Space, Tag } from "antd";
import { createStyles } from "antd-style";
import React, { useMemo, useRef, useState } from "react";

const useStyles = createStyles(() => ({
  container: {
    padding: 24,
    background: "#f5f7fa",
    minHeight: "100vh",
  },
  header: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "flex-start",
    marginBottom: 24,
  },
  headerLeft: {
    flex: 1,
  },
  title: {
    fontSize: 24,
    fontWeight: 600,
    color: "#1f2937",
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 14,
    color: "#6b7280",
  },
  statsRow: {
    display: "grid",
    gridTemplateColumns: "repeat(4, minmax(220px, 1fr))",
    gap: 16,
    marginBottom: 24,
    "@media (max-width: 1400px)": {
      gridTemplateColumns: "repeat(2, 1fr)",
    },
    "@media (max-width: 576px)": {
      gridTemplateColumns: "1fr",
    },
  },
  statsCard: {
    background: "#fff",
    borderRadius: 12,
    padding: "20px 24px",
    border: "1px solid #f0f0f0",
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 12,
    overflow: "hidden",
    transition: "all 0.3s ease",
    "&:hover": {
      boxShadow: "0 4px 12px rgba(0, 0, 0, 0.08)",
      transform: "translateY(-2px)",
    },
  },
  statsLeft: {
    display: "flex",
    alignItems: "center",
    gap: 12,
    flexShrink: 0,
    minWidth: 0,
  },
  statsIcon: {
    width: 44,
    height: 44,
    minWidth: 44,
    borderRadius: 10,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    fontSize: 20,
    flexShrink: 0,
  },
  statsIconBlue: {
    background: "linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)",
    color: "#3b82f6",
  },
  statsIconGreen: {
    background: "linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%)",
    color: "#10b981",
  },
  statsIconOrange: {
    background: "linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%)",
    color: "#f59e0b",
  },
  statsIconPurple: {
    background: "linear-gradient(135deg, #faf5ff 0%, #e9d5ff 100%)",
    color: "#a855f7",
  },
  statsContent: {
    display: "flex",
    flexDirection: "column",
    gap: 2,
  },
  statsLabel: {
    fontSize: 14,
    color: "#374151",
    fontWeight: 500,
  },
  statsSublabel: {
    fontSize: 12,
    color: "#9ca3af",
  },
  statsValue: {
    textAlign: "right",
    flexShrink: 1,
    minWidth: 0,
  },
  statsNumber: {
    fontSize: 20,
    fontWeight: 700,
    fontFamily: '"DIN Alternate", "Bebas Neue", sans-serif',
    whiteSpace: "nowrap",
  },
  statsNumberBlue: {
    color: "#3b82f6",
  },
  statsNumberGreen: {
    color: "#10b981",
  },
  statsNumberOrange: {
    color: "#f59e0b",
  },
  statsNumberPurple: {
    color: "#a855f7",
  },
  statsUnit: {
    fontSize: 14,
    color: "#6b7280",
    marginLeft: 4,
  },
  statsExtra: {
    fontSize: 12,
    color: "#9ca3af",
    marginTop: 2,
  },
  tableCard: {
    background: "#fff",
    borderRadius: 12,
    overflow: "hidden",
    border: "1px solid #f0f0f0",
    ".ant-pro-table-search": {
      background: "#fff",
      borderRadius: "12px 12px 0 0",
      padding: "20px 24px 16px",
      marginBottom: 0,
      borderBottom: "1px solid #f0f0f0",
    },
    ".ant-pro-form-query-filter": {
      ".ant-row": {
        rowGap: "12px",
      },
    },
    ".ant-form-item": {
      marginBottom: 0,
    },
    ".ant-form-item-label > label": {
      color: "#374151",
      fontWeight: 500,
    },
    ".ant-input, .ant-select-selector": {
      borderRadius: "8px !important",
      borderColor: "#e5e7eb !important",
      "&:hover, &:focus": {
        borderColor: "#3b82f6 !important",
      },
    },
    ".ant-pro-form-query-filter-actions": {
      gap: 8,
    },
    ".ant-btn-default": {
      borderRadius: 8,
      borderColor: "#e5e7eb",
      "&:hover": {
        borderColor: "#3b82f6",
        color: "#3b82f6",
      },
    },
    ".ant-btn-primary": {
      borderRadius: 8,
      background: "#3b82f6",
      "&:hover": {
        background: "#2563eb",
      },
    },
    ".ant-pro-table-list-toolbar": {
      padding: "12px 24px",
      borderBottom: "1px solid #f0f0f0",
    },
  },
  statusTag: {
    borderRadius: 6,
    fontWeight: 500,
    border: "none",
    padding: "2px 10px",
  },
  statusPending: {
    background: "#fffbeb",
    color: "#f59e0b",
  },
  statusConfirmed: {
    background: "#eff6ff",
    color: "#3b82f6",
  },
  statusCompleted: {
    background: "#ecfdf5",
    color: "#10b981",
  },
  statusFailed: {
    background: "#fef2f2",
    color: "#ef4444",
  },
  addressText: {
    fontFamily: "monospace",
    fontSize: 13,
    color: "#374151",
  },
  amountText: {
    fontWeight: 600,
    color: "#1f2937",
    fontFamily: '"DIN Alternate", monospace',
  },
  actionBtn: {
    padding: "0 8px",
    fontSize: 13,
  },
  hashLink: {
    fontFamily: "monospace",
    fontSize: 12,
    color: "#3b82f6",
    cursor: "pointer",
    "&:hover": {
      textDecoration: "underline",
    },
  },
}));

const PERM = {
  list: "ListDeposits",
  update: "UpdateDeposit",
};

const DepositsPage: React.FC = () => {
  const { styles } = useStyles();
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (
    id: string,
    defaultMessage: string,
    values?: Record<string, any>
  ) => intl.formatMessage({ id, defaultMessage }, values);

  const depositStatusValueEnum = {
    pending: { text: t("pages.deposits.status.pending", "Pending") },
    confirmed: { text: t("pages.deposits.status.confirmed", "Confirmed") },
    completed: { text: t("pages.deposits.status.completed", "Completed") },
    failed: { text: t("pages.deposits.status.failed", "Failed") },
  };

  const actionRef = useRef<ActionType | null>(null);
  const [editing, setEditing] = useState<AdminWalletDepositItem | null>(null);
  const [summary, setSummary] = useState<{
    total?: number;
    pending?: number;
    completed?: number;
    totalAmount?: string;
  }>({});

  const trimTrailingZeros = (raw: unknown) => {
    if (raw === null || raw === undefined) return "-";
    const s = String(raw).trim();
    if (!s) return "-";
    // 科学计数法/非标准数值字符串不处理
    if (/[eE]/.test(s)) return s;
    const dotIdx = s.indexOf(".");
    if (dotIdx < 0) return s;
    const intPart = s.slice(0, dotIdx);
    let fracPart = s.slice(dotIdx + 1);
    // 只去掉小数部分末尾的 0
    fracPart = fracPart.replace(/0+$/, "");
    if (!fracPart) return intPart;
    return `${intPart}.${fracPart}`;
  };

  // 统计数据（模拟，实际应从 API 获取）
  const stats = useMemo(
    () => ({
      total: summary.total ?? 156,
      pending: summary.pending ?? 12,
      completed: summary.completed ?? 138,
      totalAmount: trimTrailingZeros(summary.totalAmount ?? "86,520.00"),
    }),
    [summary]
  );

  const getStatusStyle = (status?: string) => {
    switch (status) {
      case "completed":
        return styles.statusCompleted;
      case "confirmed":
        return styles.statusConfirmed;
      case "pending":
        return styles.statusPending;
      case "failed":
        return styles.statusFailed;
      default:
        return "";
    }
  };

  const getStatusText = (status?: string) => {
    switch (status) {
      case "completed":
        return t("pages.deposits.status.completed", "已完成");
      case "confirmed":
        return t("pages.deposits.status.confirmed", "已确认");
      case "pending":
        return t("pages.deposits.status.pending", "待处理");
      case "failed":
        return t("pages.deposits.status.failed", "失败");
      default:
        return status || "-";
    }
  };

  const renderDashIfEmpty = (v: unknown) => {
    if (v === null || v === undefined) return "-";
    if (typeof v === "string") {
      const s = v.trim();
      if (!s || s === "0") return "-";
      return s;
    }
    return String(v);
  };

  const columns: ProColumns<AdminWalletDepositItem>[] = [
    {
      title: t("pages.deposits.columns.tradeId", "tradeId"),
      dataIndex: "id",
      copyable: true,
      width: 140,
      render: (_, row) => (
        <span style={{ fontFamily: "monospace", fontSize: 13 }}>{row.id}</span>
      ),
    },
    {
      title: t("pages.deposits.columns.userId", "User ID"),
      dataIndex: "user_id",
      copyable: true,
      width: 140,
      render: (_, row) => (
        <span style={{ fontFamily: "monospace", fontSize: 13 }}>
          {row.user_id}
        </span>
      ),
    },
    {
      title: t("pages.deposits.columns.tokenId", "Asset"),
      dataIndex: "asset_code",
      copyable: true,
      width: 110,
      render: (_, row) => (
        <Tag color="blue" style={{ borderRadius: 4, fontWeight: 500 }}>
          {row.asset_code}
        </Tag>
      ),
    },
    {
      title: t("pages.deposits.columns.chain", "Chain"),
      dataIndex: "chain_code",
      copyable: true,
      width: 110,
      render: (_, row) => (
        <Tag color="purple" style={{ borderRadius: 4, fontWeight: 500 }}>
          {row.chain_code}
        </Tag>
      ),
    },
    {
      title: t("pages.deposits.columns.address", "Address"),
      dataIndex: "deposit_address",
      copyable: true,
      ellipsis: true,
      width: 180,
      render: (_, row) => (
        <span className={styles.addressText}>
          {row.deposit_address
            ? `${row.deposit_address.slice(0, 8)}...${row.deposit_address.slice(
                -6
              )}`
            : "-"}
        </span>
      ),
    },
    {
      title: t("pages.deposits.columns.amount", "Amount"),
      dataIndex: "amount",
      width: 130,
      render: (_, row) => (
        <span className={styles.amountText}>
          {trimTrailingZeros(row.amount)}
        </span>
      ),
    },
    {
      title: t("pages.deposits.columns.status", "Status"),
      dataIndex: "status",
      width: 110,
      render: (_, row) => (
        <Tag className={`${styles.statusTag} ${getStatusStyle(row.status)}`}>
          {getStatusText(row.status)}
        </Tag>
      ),
    },
    {
      title: t("pages.deposits.columns.txHash", "Tx Hash"),
      dataIndex: "transaction_hash",
      copyable: false,
      ellipsis: true,
      width: 150,
      render: (_, row) => {
        const hash = (row.transaction_hash || "").trim();
        if (!hash) return "-";

        const short = `${hash.slice(0, 8)}...${hash.slice(-6)}`;

        const doCopy = async () => {
          try {
            if (navigator.clipboard?.writeText) {
              await navigator.clipboard.writeText(hash);
            } else {
              const ta = document.createElement("textarea");
              ta.value = hash;
              ta.style.position = "fixed";
              ta.style.left = "-9999px";
              ta.style.top = "-9999px";
              document.body.appendChild(ta);
              ta.focus();
              ta.select();
              document.execCommand("copy");
              document.body.removeChild(ta);
            }
            message.success(t("common.copySuccess", "复制成功"));
          } catch {
            message.error(t("common.copyFailed", "复制失败"));
          }
        };

        return (
          <span
            className={styles.hashLink}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              void doCopy();
            }}
            style={{ cursor: "pointer" }}
            title={hash}
          >
            {short}
          </span>
        );
      },
    },

    {
      title: t("pages.deposits.columns.block", "Block"),
      dataIndex: "block_number",
      width: 100,
      render: (_, row) => (
        <span style={{ fontFamily: "monospace", color: "#6b7280" }}>
          {renderDashIfEmpty(row.block_number)}
        </span>
      ),
    },
    {
      title: t("pages.deposits.columns.confirmations", "Confirmations"),
      dataIndex: "confirmations",
      width: 120,
      render: (_, row) => {
        const v = row.confirmations;
        if (v === null || v === undefined) {
          return <span style={{ fontWeight: 500, color: "#9ca3af" }}>-</span>;
        }
        return (
          <span
            style={{
              fontWeight: 500,
              color: v >= 6 ? "#10b981" : "#f59e0b",
            }}
          >
            {v}
          </span>
        );
      },
    },
    {
      title: t("pages.deposits.columns.memo", "Memo"),
      dataIndex: "memo",
      ellipsis: true,
      width: 120,
    },
    {
      title: t("pages.deposits.columns.createdAt", "Created At"),
      dataIndex: "created_at",
      valueType: "dateTime",
      width: 170,
    },
    {
      title: t("pages.deposits.columns.updatedAt", "Updated At"),
      dataIndex: "updated_at",
      valueType: "dateTime",
      width: 170,
    },
    {
      title: t("pages.deposits.columns.actions", "Actions"),
      valueType: "option",
      width: 120,
      fixed: "right",
      render: (_, row) => {
        const canUpdate = canRpc(PERM.update);
        return (
          <Space size={4}>
            <Button
              type="link"
              size="small"
              className={styles.actionBtn}
              disabled={!canUpdate}
              onClick={() => setEditing(row)}
            >
              {t("pages.deposits.actions.edit", "编辑")}
            </Button>
          </Space>
        );
      },
    },

    {
      title: t("pages.deposits.search.userId", "User ID"),
      dataIndex: "user_id",
      hideInTable: true,
      search: { transform: (v) => ({ user_id: v }) },
    },
    {
      title: t("pages.deposits.search.tokenId", "Token ID"),
      dataIndex: "asset_code",
      hideInTable: true,
      search: { transform: (v) => ({ asset_code: v }) },
    },
    {
      title: t("pages.deposits.search.chain", "Chain"),
      dataIndex: "chain_code",
      hideInTable: true,
      search: { transform: (v) => ({ chain_code: v }) },
    },
    {
      title: t("pages.deposits.search.depositAddress", "Deposit Address"),
      dataIndex: "deposit_address",
      hideInTable: true,
      search: { transform: (v) => ({ deposit_address: v }) },
    },
    {
      title: t("pages.deposits.search.txHash", "Tx Hash"),
      dataIndex: "transaction_hash",
      hideInTable: true,
      search: { transform: (v) => ({ transaction_hash: v }) },
    },
    {
      title: t("pages.deposits.search.status", "Status"),
      dataIndex: "status",
      hideInTable: true,
      valueEnum: depositStatusValueEnum,
      search: { transform: (v) => ({ status: v }) },
    },
    {
      title: t("pages.deposits.search.createdFrom", "Created From"),
      dataIndex: "created_from",
      hideInTable: true,
      search: { transform: (v) => ({ created_from: v }) },
    },
    {
      title: t("pages.deposits.search.createdTo", "Created To"),
      dataIndex: "created_to",
      hideInTable: true,
      search: { transform: (v) => ({ created_to: v }) },
    },
    {
      title: t("pages.deposits.search.minAmount", "Min Amount"),
      dataIndex: "min_amount",
      hideInTable: true,
      search: { transform: (v) => ({ min_amount: v }) },
    },
    {
      title: t("pages.deposits.search.maxAmount", "Max Amount"),
      dataIndex: "max_amount",
      hideInTable: true,
      search: { transform: (v) => ({ max_amount: v }) },
    },
  ];

  return (
    <PageContainer header={{ title: null, breadcrumb: {} }}>
      <div className={styles.container}>
        {/* 页面头部 */}
        <div className={styles.header}>
          <div className={styles.headerLeft}>
            <div className={styles.title}>
              {t("pages.deposits.title", "充值记录")}
            </div>
            <div className={styles.subtitle}>
              {t("pages.deposits.subtitle", "管理和查看所有用户充值交易记录")}
            </div>
          </div>
        </div>

        {/* 统计卡片 */}
        <div className={styles.statsRow}>
          {/* 总充值笔数 */}
          <div className={styles.statsCard}>
            <div className={styles.statsLeft}>
              <div className={`${styles.statsIcon} ${styles.statsIconBlue}`}>
                <WalletOutlined />
              </div>
              <div className={styles.statsContent}>
                <span className={styles.statsLabel}>
                  {t("pages.deposits.stats.total", "总充值笔数")}
                </span>
                <span className={styles.statsSublabel}>
                  {t("pages.deposits.stats.allRecords", "所有充值记录")}
                </span>
              </div>
            </div>
            <div className={styles.statsValue}>
              <span
                className={`${styles.statsNumber} ${styles.statsNumberBlue}`}
              >
                {stats.total}
              </span>
              <span className={styles.statsUnit}>
                {t("pages.deposits.unit.count", "笔")}
              </span>
            </div>
          </div>

          {/* 待处理 */}
          <div className={styles.statsCard}>
            <div className={styles.statsLeft}>
              <div className={`${styles.statsIcon} ${styles.statsIconOrange}`}>
                <ClockCircleOutlined />
              </div>
              <div className={styles.statsContent}>
                <span className={styles.statsLabel}>
                  {t("pages.deposits.stats.pending", "待处理")}
                </span>
                <span className={styles.statsSublabel}>
                  {t("pages.deposits.stats.pendingDesc", "等待确认中")}
                </span>
              </div>
            </div>
            <div className={styles.statsValue}>
              <span
                className={`${styles.statsNumber} ${styles.statsNumberOrange}`}
              >
                {stats.pending}
              </span>
              <span className={styles.statsUnit}>
                {t("pages.deposits.unit.count", "笔")}
              </span>
            </div>
          </div>

          {/* 已完成 */}
          <div className={styles.statsCard}>
            <div className={styles.statsLeft}>
              <div className={`${styles.statsIcon} ${styles.statsIconGreen}`}>
                <CheckCircleOutlined />
              </div>
              <div className={styles.statsContent}>
                <span className={styles.statsLabel}>
                  {t("pages.deposits.stats.completed", "已完成")}
                </span>
                <span className={styles.statsSublabel}>
                  {t("pages.deposits.stats.completedDesc", "成功入账")}
                </span>
              </div>
            </div>
            <div className={styles.statsValue}>
              <span
                className={`${styles.statsNumber} ${styles.statsNumberGreen}`}
              >
                {stats.completed}
              </span>
              <span className={styles.statsUnit}>
                {t("pages.deposits.unit.count", "笔")}
              </span>
            </div>
          </div>

          {/* 累计金额 */}
          <div className={styles.statsCard}>
            <div className={styles.statsLeft}>
              <div className={`${styles.statsIcon} ${styles.statsIconPurple}`}>
                <DollarOutlined />
              </div>
              <div className={styles.statsContent}>
                <span className={styles.statsLabel}>
                  {t("pages.deposits.stats.totalAmount", "累计充值金额")}
                </span>
                <span className={styles.statsSublabel}>
                  {t("pages.deposits.stats.usdEquivalent", "折合USD")}
                </span>
              </div>
            </div>

            <div
              className={styles.statsValue}
              style={{
                minWidth: 0,
                flex: 1,
                display: "flex",
                justifyContent: "flex-end",
                alignItems: "baseline",
                flexWrap: "wrap",
                gap: 6,
              }}
            >
              <span
                className={`${styles.statsNumber} ${styles.statsNumberPurple} ${styles.statsNumberAdaptive}`}
                style={{
                  minWidth: 0,
                  maxWidth: "100%",
                  textAlign: "right",
                  lineHeight: 1.15,

                  whiteSpace: "normal",
                  overflowWrap: "anywhere",
                  wordBreak: "break-word",

                  fontSize: "clamp(14px, 2vw, 22px)",

                  fontVariantNumeric: "tabular-nums",
                }}
              >
                ${stats.totalAmount}
              </span>
            </div>
          </div>
        </div>

        {/* 数据表格 */}
        <div className={styles.tableCard}>
          <ProTable<AdminWalletDepositItem>
            actionRef={actionRef}
            rowKey={(row) =>
              row.id || row.transaction_hash || JSON.stringify(row)
            }
            columns={columns}
            request={async (params) => {
              if (!canRpc(PERM.list))
                return { success: true, data: [], total: 0 };
              try {
                const res = await adminListDeposits(
                  {
                    page: params.current,
                    page_size: params.pageSize,
                    user_id: (params as any).user_id,
                    asset_code: (params as any).asset_code,
                    chain_code: (params as any).chain_code,
                    status: (params as any).status,
                    deposit_address: (params as any).deposit_address,
                    transaction_hash: (params as any).transaction_hash,
                    created_from: (params as any).created_from,
                    created_to: (params as any).created_to,
                    min_amount: (params as any).min_amount,
                    max_amount: (params as any).max_amount,
                  },
                  { skipErrorHandler: true }
                );
                // 更新统计数据（如果 API 返回；当前 OpenAPI schema 未声明该字段）
                const maybeSummary = (res.data as any)?.summary as
                  | {
                      total?: number;
                      pending?: number;
                      completed?: number;
                      totalAmount?: string;
                    }
                  | undefined;
                if (maybeSummary) setSummary(maybeSummary);
                return {
                  success: !!res?.success,
                  data: res.data?.deposits ?? [],
                  total: Number(res.data?.pagination?.total ?? 0),
                };
              } catch (e: any) {
                message.error(
                  e?.message ||
                    t(
                      "pages.deposits.messages.loadFailed",
                      "Failed to load deposits"
                    )
                );
                return { success: false, data: [], total: 0 };
              }
            }}
            search={{ labelWidth: 120 }}
            scroll={{ x: 1800 }}
            options={{
              density: true,
              fullScreen: true,
              reload: true,
            }}
            pagination={{
              showSizeChanger: true,
              showQuickJumper: true,
            }}
          />
        </div>
      </div>

      <ModalForm<AdminUpdateDepositBody>
        title={t("pages.deposits.edit.title", "编辑充值")}
        open={!!editing}
        width={500}
        modalProps={{
          destroyOnClose: true,
          maskClosable: false,
        }}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
        initialValues={{
          status: editing?.status,
          block_number: editing?.block_number,
          confirmations: editing?.confirmations,
        }}
        onFinish={async (values) => {
          const id = editing?.id;
          if (!id) return false;
          try {
            const res = await adminUpdateDeposit({ id }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.deposits.messages.updateFailed", "Update failed")
              );
            message.success(t("pages.deposits.messages.updated", "更新成功"));
            setEditing(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.deposits.messages.updateFailed", "更新失败")
            );
            return false;
          }
        }}
      >
        <ProFormSelect
          name="status"
          label={t("pages.deposits.form.status", "状态")}
          options={[
            {
              label: t("pages.deposits.status.pending", "待处理"),
              value: "pending",
            },
            {
              label: t("pages.deposits.status.confirmed", "已确认"),
              value: "confirmed",
            },
            {
              label: t("pages.deposits.status.completed", "已完成"),
              value: "completed",
            },
            {
              label: t("pages.deposits.status.failed", "失败"),
              value: "failed",
            },
          ]}
          placeholder={t("pages.deposits.form.selectStatus", "请选择状态")}
          rules={[
            {
              required: true,
              message: t("pages.deposits.form.statusRequired", "请选择状态"),
            },
          ]}
        />
        <ProFormText
          name="block_number"
          label={t("pages.deposits.form.blockNumber", "区块号")}
          placeholder={t(
            "pages.deposits.form.blockNumberPlaceholder",
            "请输入区块号"
          )}
        />
        <ProFormDigit
          name="confirmations"
          label={t("pages.deposits.form.confirmations", "确认数")}
          min={0}
          fieldProps={{ precision: 0 }}
          placeholder={t(
            "pages.deposits.form.confirmationsPlaceholder",
            "请输入确认数"
          )}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default DepositsPage;
