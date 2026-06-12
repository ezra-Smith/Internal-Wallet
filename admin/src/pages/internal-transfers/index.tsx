import {
  adminApproveInternalTransfer,
  adminCancelInternalTransfer,
  adminListInternalTransfers,
  adminRejectInternalTransfer,
} from "@/api/generated/internal-transfers";
import type {
  AdminApproveInternalTransferBody,
  AdminInternalTransferItem,
  AdminListInternalTransfersParams,
  AdminRejectInternalTransferBody,
} from "@/api/generated/schemas";
import { useRbac } from "@/hooks/useRbac";
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  DollarOutlined,
  SendOutlined,
} from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-components";
import {
  type ActionType,
  ModalForm,
  PageContainer,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from "@ant-design/pro-components";
import { useIntl } from "@umijs/max";
import { App, Button, Popconfirm, Space, Tag } from "antd";
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
    gridTemplateColumns: "repeat(4, minmax(200px, 1fr))",
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
  statsIconRed: {
    background: "linear-gradient(135deg, #fef2f2 0%, #fecaca 100%)",
    color: "#ef4444",
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
  statsNumberRed: {
    color: "#ef4444",
  },
  statsUnit: {
    fontSize: 14,
    color: "#6b7280",
    marginLeft: 4,
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
  statusProcessing: {
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
  statusCancelled: {
    background: "#f3f4f6",
    color: "#6b7280",
  },
  statusRejected: {
    background: "#fef2f2",
    color: "#dc2626",
  },
  strategyTag: {
    borderRadius: 6,
    fontWeight: 500,
    border: "none",
  },
  amountText: {
    fontWeight: 600,
    color: "#1f2937",
    fontFamily: '"DIN Alternate", monospace',
  },
  actionBtn: {
    padding: "0 6px",
    fontSize: 13,
  },
}));

// 使用生成的类型（已从 @/api/generated/schemas 导入）

const PERM = {
  list: "ListCurrencyTransfers", // 对应后端权限：rpc:ListCurrencyTransfers
  get: "GetCurrencyTransfer", // 对应后端权限：rpc:GetCurrencyTransfer
  approve: "ApproveCurrencyTransfer", // 对应后端权限：rpc:ApproveCurrencyTransfer
  reject: "RejectCurrencyTransfer", // 对应后端权限：rpc:RejectCurrencyTransfer
  cancel: "CancelCurrencyTransfer", // 对应后端权限：rpc:CancelCurrencyTransfer
};

const InternalTransfersPage: React.FC = () => {
  const { styles } = useStyles();
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (
    id: string,
    defaultMessage: string,
    values?: Record<string, any>
  ) => intl.formatMessage({ id, defaultMessage }, values);

  const actionRef = useRef<ActionType | null>(null);

  const [approving, setApproving] = useState<AdminInternalTransferItem | null>(
    null
  );
  const [rejecting, setRejecting] = useState<AdminInternalTransferItem | null>(
    null
  );
  const [viewing, setViewing] = useState<AdminInternalTransferItem | null>(
    null
  );

  const [summary, setSummary] = useState<{
    total?: number;
    pending?: number;
    completed?: number;
    failed?: number;
    cancelled?: number;
    rejected?: number;
    totalAmount?: string;
  }>({});

  // 统计数据
  const stats = useMemo(
    () => ({
      total: summary.total || 0,
      pending: summary.pending || 0,
      completed: summary.completed || 0,
      failed: summary.failed || 0,
      cancelled: summary.cancelled || 0,
      rejected: summary.rejected || 0,
      totalAmount: summary.totalAmount || "0.00",
    }),
    [summary]
  );

  const transferStatusValueEnum = {
    pending: { text: t("pages.internalTransfers.status.pending", "待处理") },
    processing: {
      text: t("pages.internalTransfers.status.processing", "处理中"),
    },
    completed: {
      text: t("pages.internalTransfers.status.completed", "已完成"),
    },
    failed: { text: t("pages.internalTransfers.status.failed", "失败") },
    cancelled: {
      text: t("pages.internalTransfers.status.cancelled", "已取消"),
    },
    rejected: {
      text: t("pages.internalTransfers.status.rejected", "已拒绝"),
    },
  };

  const getStatusStyle = (status?: string) => {
    switch (status) {
      case "completed":
        return styles.statusCompleted;
      case "processing":
        return styles.statusProcessing;
      case "pending":
        return styles.statusPending;
      case "failed":
        return styles.statusFailed;
      case "cancelled":
        return styles.statusCancelled;
      case "rejected":
        return styles.statusRejected;
      default:
        return "";
    }
  };

  const getStatusText = (status?: string) => {
    switch (status) {
      case "completed":
        return t("pages.internalTransfers.status.completed", "已完成");
      case "processing":
        return t("pages.internalTransfers.status.processing", "处理中");
      case "pending":
        return t("pages.internalTransfers.status.pending", "待处理");
      case "failed":
        return t("pages.internalTransfers.status.failed", "失败");
      case "cancelled":
        return t("pages.internalTransfers.status.cancelled", "已取消");
      case "rejected":
        return t("pages.internalTransfers.status.rejected", "已拒绝");
      default:
        return status || "-";
    }
  };

  const getStrategyText = (strategy?: string) => {
    const s = (strategy || "").trim();
    if (s === "auto")
      return t("pages.internalTransfers.strategy.auto", "自动审核");
    if (s === "manual_auto")
      return t(
        "pages.internalTransfers.strategy.manualAuto",
        "人工审核后自动放币"
      );
    if (s === "manual_manual")
      return t(
        "pages.internalTransfers.strategy.manualManual",
        "人工审核后手动转账"
      );
    return s || "-";
  };

  const strategyTag = (strategy?: string) => {
    const s = (strategy || "").trim();
    const text = getStrategyText(s);
    if (s === "auto")
      return (
        <Tag className={styles.strategyTag} color="green">
          {text}
        </Tag>
      );
    if (s === "manual_auto")
      return (
        <Tag className={styles.strategyTag} color="orange">
          {text}
        </Tag>
      );
    if (s === "manual_manual")
      return (
        <Tag className={styles.strategyTag} color="blue">
          {text}
        </Tag>
      );
    return <Tag className={styles.strategyTag}>{text}</Tag>;
  };

  const isZeroLike = (value?: string) => {
    if (value == null) return true;
    const s = String(value).trim();
    if (!s) return true;
    const n = Number(s);
    return Number.isFinite(n) && n === 0;
  };

  const formatAmountOrDash = (value?: string) => {
    if (isZeroLike(value)) return "-";
    const s = String(value).trim();
    // Remove trailing zeros after decimal point
    if (s.includes(".")) {
      return s.replace(/\.?0+$/, "");
    }
    return s;
  };

  const columns: ProColumns<AdminInternalTransferItem>[] = [
    {
      title: t("pages.internalTransfers.columns.id", "ID"),
      dataIndex: "id",
      copyable: true,
      width: 130,
      render: (_, row) => (
        <span style={{ fontFamily: "monospace", fontSize: 13 }}>{row.id}</span>
      ),
    },
    {
      title: t("pages.internalTransfers.columns.fromUserId", "转出用户ID"),
      dataIndex: "from_user_id",
      copyable: true,
      width: 120,
      render: (_, row) => (
        <span style={{ fontFamily: "monospace", fontSize: 13 }}>
          {row.from_user_id}
        </span>
      ),
    },
    {
      title: t("pages.internalTransfers.columns.fromUserEmail", "转出用户邮箱"),
      dataIndex: "from_user_email",
      ellipsis: true,
      width: 180,
      render: (_, row) => (
        <span style={{ fontSize: 13 }}>{row.from_user_email || "-"}</span>
      ),
    },
    {
      title: t("pages.internalTransfers.columns.fromUserPhone", "转出用户手机"),
      dataIndex: "from_user_phone",
      ellipsis: true,
      width: 140,
      render: (_, row) => (
        <span style={{ fontSize: 13 }}>{row.from_user_phone || "-"}</span>
      ),
    },
    {
      title: t("pages.internalTransfers.columns.toUserId", "转入用户ID"),
      dataIndex: "to_user_id",
      copyable: true,
      width: 120,
      render: (_, row) => (
        <span style={{ fontFamily: "monospace", fontSize: 13 }}>
          {row.to_user_id}
        </span>
      ),
    },
    {
      title: t("pages.internalTransfers.columns.toUserEmail", "转入用户邮箱"),
      dataIndex: "to_user_email",
      ellipsis: true,
      width: 180,
      render: (_, row) => (
        <span style={{ fontSize: 13 }}>{row.to_user_email || "-"}</span>
      ),
    },
    {
      title: t("pages.internalTransfers.columns.toUserPhone", "转入用户手机"),
      dataIndex: "to_user_phone",
      ellipsis: true,
      width: 140,
      render: (_, row) => (
        <span style={{ fontSize: 13 }}>{row.to_user_phone || "-"}</span>
      ),
    },
    {
      title: t("pages.internalTransfers.columns.asset", "资产"),
      dataIndex: "asset_code",
      width: 90,
      render: (_, row) => (
        <Tag color="blue" style={{ borderRadius: 4, fontWeight: 500 }}>
          {row.asset_code}
        </Tag>
      ),
    },
    {
      title: t("pages.internalTransfers.columns.amount", "金额"),
      dataIndex: "amount",
      width: 120,
      render: (_, row) => (
        <span className={styles.amountText}>
          {formatAmountOrDash(row.amount)}
        </span>
      ),
    },
    // {
    //   title: t("pages.internalTransfers.columns.fee", "手续费"),
    //   dataIndex: "fee",
    //   width: 100,
    //   render: (_, row) => (
    //     <span style={{ color: "#6b7280" }}>{formatAmountOrDash(row.fee)}</span>
    //   ),
    // },
    {
      title: t("pages.internalTransfers.columns.strategy", "策略"),
      dataIndex: "strategy",
      width: 110,
      render: (_, row) => strategyTag(row.strategy),
    },
    {
      title: t("pages.internalTransfers.columns.status", "状态"),
      dataIndex: "status",
      width: 100,
      render: (_, row) => (
        <Tag className={`${styles.statusTag} ${getStatusStyle(row.status)}`}>
          {getStatusText(row.status)}
        </Tag>
      ),
    },
    {
      title: t("pages.internalTransfers.columns.note", "备注"),
      dataIndex: "note",
      ellipsis: true,
      width: 120,
    },
    {
      title: t("pages.internalTransfers.columns.errorMessage", "错误信息"),
      dataIndex: "error_message",
      ellipsis: true,
      width: 150,
      render: (_, row) =>
        row.error_message ? (
          <span style={{ color: "#ef4444", fontSize: 12 }}>
            {row.error_message}
          </span>
        ) : (
          "-"
        ),
    },
    {
      title: t("pages.internalTransfers.columns.auditAdminId", "审核管理员"),
      dataIndex: "audit_admin_id",
      width: 110,
      render: (_, row) =>
        row.audit_admin_id ? (
          <span style={{ fontFamily: "monospace", fontSize: 12 }}>
            {row.audit_admin_id}
          </span>
        ) : (
          "-"
        ),
    },
    {
      title: t("pages.internalTransfers.columns.createdAt", "创建时间"),
      dataIndex: "created_at",
      valueType: "dateTime",
      width: 160,
    },
    {
      title: t("pages.internalTransfers.columns.updatedAt", "更新时间"),
      dataIndex: "updated_at",
      valueType: "dateTime",
      width: 160,
    },
    {
      title: t("pages.internalTransfers.columns.actions", "操作"),
      valueType: "option",
      width: 200,
      fixed: "right",
      render: (_, row) => {
        const canApprove = canRpc(PERM.approve);
        const canReject = canRpc(PERM.reject);
        const canCancel = canRpc(PERM.cancel);
        const canGet = canRpc(PERM.get);
        const id = row.id;
        const status = (row.status || "").trim();
        const strategy = (row.strategy || "").trim();
        const isPending = status === "pending";
        const isManual =
          strategy === "manual_auto" || strategy === "manual_manual";
        const isImmutable =
          status === "completed" ||
          status === "failed" ||
          status === "cancelled" ||
          status === "rejected";

        return (
          <Space size={4} wrap>
            <Button
              type="link"
              size="small"
              className={styles.actionBtn}
              disabled={!canGet}
              onClick={() => setViewing(row)}
            >
              {t("pages.internalTransfers.actions.view", "查看")}
            </Button>
            <Button
              type="link"
              size="small"
              className={styles.actionBtn}
              style={{ color: "#10b981" }}
              disabled={!canApprove || !isPending || !isManual}
              onClick={() => setApproving(row)}
            >
              {t("pages.internalTransfers.actions.approve", "通过")}
            </Button>
            <Button
              type="link"
              size="small"
              className={styles.actionBtn}
              style={{ color: "#f59e0b" }}
              disabled={!canReject || !isPending}
              onClick={() => setRejecting(row)}
            >
              {t("pages.internalTransfers.actions.reject", "拒绝")}
            </Button>
            <Popconfirm
              title={t(
                "pages.internalTransfers.actions.cancelConfirm",
                "确定取消此内部转账?"
              )}
              okButtonProps={{
                danger: true,
                disabled: !canCancel || !isPending,
              }}
              onConfirm={async () => {
                if (!canCancel || !isPending) return;
                if (!id) return;
                try {
                  const res = await adminCancelInternalTransfer(
                    { id: String(id) },
                    { reason: "" },
                    { skipErrorHandler: true }
                  );
                  if (!res?.success)
                    throw new Error(
                      res?.message ||
                        t(
                          "pages.internalTransfers.messages.cancelFailed",
                          "取消失败"
                        )
                    );
                  message.success(
                    t("pages.internalTransfers.messages.cancelled", "已取消")
                  );
                  actionRef.current?.reload();
                } catch (e: any) {
                  message.error(
                    e?.message ||
                      t(
                        "pages.internalTransfers.messages.cancelFailed",
                        "取消失败"
                      )
                  );
                }
              }}
            >
              <Button
                type="link"
                size="small"
                className={styles.actionBtn}
                danger
                disabled={!canCancel || !isPending}
              >
                {t("pages.internalTransfers.actions.cancel", "取消")}
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },

    // Search fields
    {
      title: t("pages.internalTransfers.search.fromUserId", "转出用户ID"),
      dataIndex: "from_user_id",
      hideInTable: true,
      search: { transform: (v) => ({ from_user_id: Number(v) }) },
    },
    {
      title: t("pages.internalTransfers.search.toUserId", "转入用户ID"),
      dataIndex: "to_user_id",
      hideInTable: true,
      search: { transform: (v) => ({ to_user_id: Number(v) }) },
    },
    {
      title: t("pages.internalTransfers.search.asset", "资产"),
      dataIndex: "asset_code",
      hideInTable: true,
      search: { transform: (v) => ({ asset_code: v }) },
    },
    {
      title: t("pages.internalTransfers.search.status", "状态"),
      dataIndex: "status",
      hideInTable: true,
      valueEnum: transferStatusValueEnum,
      search: { transform: (v) => ({ status: v }) },
    },
    {
      title: t("pages.internalTransfers.search.strategy", "策略"),
      dataIndex: "strategy",
      hideInTable: true,
      valueEnum: {
        auto: { text: getStrategyText("auto") },
        manual_auto: { text: getStrategyText("manual_auto") },
      },
      search: { transform: (v) => ({ strategy: v }) },
    },
  ];

  return (
    <PageContainer header={{ title: null, breadcrumb: {} }}>
      <div className={styles.container}>
        {/* 页面头部 */}
        <div className={styles.header}>
          <div className={styles.headerLeft}>
            <div className={styles.title}>
              {t("pages.internalTransfers.title", "内部转账审核")}
            </div>
            <div className={styles.subtitle}>
              {t(
                "pages.internalTransfers.subtitle",
                "管理和审核用户之间的内部转账记录"
              )}
            </div>
          </div>
        </div>

        {/* 统计卡片 */}
        <div className={styles.statsRow}>
          {/* 总转账笔数 */}
          <div className={styles.statsCard}>
            <div className={styles.statsLeft}>
              <div className={`${styles.statsIcon} ${styles.statsIconBlue}`}>
                <SendOutlined />
              </div>
              <div className={styles.statsContent}>
                <span className={styles.statsLabel}>
                  {t("pages.internalTransfers.stats.total", "总转账笔数")}
                </span>
                <span className={styles.statsSublabel}>
                  {t(
                    "pages.internalTransfers.stats.allRecords",
                    "所有转账记录"
                  )}
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
                {t("pages.internalTransfers.unit.count", "笔")}
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
                  {t("pages.internalTransfers.stats.pending", "待处理")}
                </span>
                <span className={styles.statsSublabel}>
                  {t("pages.internalTransfers.stats.pendingDesc", "等待审核")}
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
                {t("pages.internalTransfers.unit.count", "笔")}
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
                  {t("pages.internalTransfers.stats.completed", "已完成")}
                </span>
                <span className={styles.statsSublabel}>
                  {t("pages.internalTransfers.stats.completedDesc", "成功转账")}
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
                {t("pages.internalTransfers.unit.count", "笔")}
              </span>
            </div>
          </div>

          {/* 累计金额 */}
          <div className={styles.statsCard}>
            <div className={styles.statsLeft}>
              <div className={`${styles.statsIcon} ${styles.statsIconRed}`}>
                <DollarOutlined />
              </div>
              <div className={styles.statsContent}>
                <span className={styles.statsLabel}>
                  {t(
                    "pages.internalTransfers.stats.totalAmount",
                    "累计转账金额"
                  )}
                </span>
                <span className={styles.statsSublabel}>
                  {t(
                    "pages.internalTransfers.stats.allRecords",
                    "所有转账记录"
                  )}
                </span>
              </div>
            </div>
            <div className={styles.statsValue}>
              <span
                className={`${styles.statsNumber} ${styles.statsNumberRed}`}
              >
                {stats.totalAmount}
              </span>
            </div>
          </div>
        </div>

        {/* 数据表格 */}
        <div className={styles.tableCard}>
          <ProTable<AdminInternalTransferItem>
            actionRef={actionRef}
            rowKey={(row) => row.id || JSON.stringify(row)}
            columns={columns}
            request={async (params) => {
              if (!canRpc(PERM.list))
                return { success: true, data: [], total: 0 };
              try {
                const res = await adminListInternalTransfers(
                  {
                    page: params.current,
                    page_size: params.pageSize,
                    from_user_id: (params as any).from_user_id,
                    to_user_id: (params as any).to_user_id,
                    asset_code: (params as any).asset_code,
                    status: (params as any).status,
                    strategy: (params as any).strategy,
                  } as AdminListInternalTransfersParams,
                  { skipErrorHandler: true }
                );
                // 使用生成的类型，不需要 as any
                if (res.data?.summary) {
                  setSummary({
                    total: Number(res.data.summary.total) || 0,
                    pending: Number(res.data.summary.pending) || 0,
                    completed: Number(res.data.summary.completed) || 0,
                    failed: Number(res.data.summary.failed) || 0,
                    cancelled: Number(res.data.summary.cancelled) || 0,
                    rejected: Number(res.data.summary.rejected) || 0,
                    totalAmount: res.data.summary.total_amount || "0.00",
                  });
                }
                return {
                  success: !!res?.success,
                  data: res.data?.transfers ?? [],
                  total: Number(res.data?.pagination?.total ?? 0),
                };
              } catch (e: any) {
                message.error(
                  e?.message ||
                    t("pages.internalTransfers.messages.loadFailed", "加载失败")
                );
                return { success: false, data: [], total: 0 };
              }
            }}
            search={{ labelWidth: 100 }}
            scroll={{ x: 2000 }}
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

      {/* 查看详情 Modal */}
      <ModalForm
        title={t("pages.internalTransfers.view.title", "内部转账详情")}
        open={!!viewing}
        width={600}
        modalProps={{
          destroyOnClose: true,
          maskClosable: false,
        }}
        onOpenChange={(open) => {
          if (!open) setViewing(null);
        }}
        submitter={false}
      >
        {viewing && (
          <div style={{ padding: "16px 0" }}>
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}>
                ID
              </div>
              <div style={{ fontFamily: "monospace", fontSize: 14 }}>
                {viewing.id}
              </div>
            </div>
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}>
                转出用户
              </div>
              <div style={{ fontSize: 14 }}>
                ID: {viewing.from_user_id}{" "}
                {viewing.from_user_email ? `(${viewing.from_user_email})` : ""}
              </div>
            </div>
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}>
                转入用户
              </div>
              <div style={{ fontSize: 14 }}>
                ID: {viewing.to_user_id}{" "}
                {viewing.to_user_email ? `(${viewing.to_user_email})` : ""}
              </div>
            </div>
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}>
                资产
              </div>
              <div style={{ fontSize: 14 }}>{viewing.asset_code}</div>
            </div>
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}>
                金额
              </div>
              <div style={{ fontSize: 14, fontWeight: 600 }}>
                {formatAmountOrDash(viewing.amount)}
              </div>
            </div>
            {/* <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}>
                手续费
              </div>
              <div style={{ fontSize: 14 }}>
                {formatAmountOrDash(viewing.fee)}
              </div>
            </div> */}
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}>
                策略
              </div>
              <div style={{ fontSize: 14 }}>
                {strategyTag(viewing.strategy)}
              </div>
            </div>
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}>
                状态
              </div>
              <div style={{ fontSize: 14 }}>
                <Tag
                  className={`${styles.statusTag} ${getStatusStyle(
                    viewing.status
                  )}`}
                >
                  {getStatusText(viewing.status)}
                </Tag>
              </div>
            </div>
            {viewing.note && (
              <div style={{ marginBottom: 16 }}>
                <div
                  style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}
                >
                  备注
                </div>
                <div style={{ fontSize: 14 }}>{viewing.note}</div>
              </div>
            )}
            {viewing.error_message && (
              <div style={{ marginBottom: 16 }}>
                <div
                  style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}
                >
                  错误信息
                </div>
                <div style={{ fontSize: 14, color: "#ef4444" }}>
                  {viewing.error_message}
                </div>
              </div>
            )}
            {viewing.audit_admin_id && (
              <div style={{ marginBottom: 16 }}>
                <div
                  style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}
                >
                  审核管理员
                </div>
                <div style={{ fontSize: 14, fontFamily: "monospace" }}>
                  {viewing.audit_admin_id}
                </div>
              </div>
            )}
            {viewing.audit_note && (
              <div style={{ marginBottom: 16 }}>
                <div
                  style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}
                >
                  审核备注
                </div>
                <div style={{ fontSize: 14 }}>{viewing.audit_note}</div>
              </div>
            )}
            {viewing.audited_at && (
              <div style={{ marginBottom: 16 }}>
                <div
                  style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}
                >
                  审核时间
                </div>
                <div style={{ fontSize: 14 }}>{viewing.audited_at}</div>
              </div>
            )}
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 4 }}>
                创建时间
              </div>
              <div style={{ fontSize: 14 }}>{viewing.created_at}</div>
            </div>
          </div>
        )}
      </ModalForm>

      {/* 审批通过 Modal */}
      <ModalForm<AdminApproveInternalTransferBody>
        title={t("pages.internalTransfers.approve.title", "审批通过")}
        open={!!approving}
        width={500}
        modalProps={{
          destroyOnClose: true,
          maskClosable: false,
        }}
        onOpenChange={(open) => {
          if (!open) setApproving(null);
        }}
        onFinish={async (values) => {
          const id = approving?.id;
          if (!id) return false;
          try {
            const res = await adminApproveInternalTransfer(
              { id: String(id) },
              values,
              { skipErrorHandler: true }
            );
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.internalTransfers.messages.approveFailed",
                    "审批失败"
                  )
              );
            message.success(
              t("pages.internalTransfers.messages.approved", "审批通过")
            );
            setApproving(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.internalTransfers.messages.approveFailed", "审批失败")
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="audit_note"
          label={t("pages.internalTransfers.form.auditNote", "审核备注")}
          placeholder={t(
            "pages.internalTransfers.form.auditNotePlaceholder",
            "请输入审核备注（可选）"
          )}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText
          name="two_fa_code"
          label={t("pages.internalTransfers.form.twoFaCode", "2FA验证码")}
          placeholder={t(
            "pages.internalTransfers.form.twoFaCodePlaceholder",
            "请输入2FA验证码"
          )}
          rules={[
            {
              required: true,
              message: t(
                "pages.internalTransfers.form.twoFaCodeRequired",
                "请输入2FA验证码"
              ),
            },
          ]}
        />
      </ModalForm>

      {/* 拒绝 Modal */}
      <ModalForm<AdminRejectInternalTransferBody>
        title={t("pages.internalTransfers.reject.title", "拒绝内部转账")}
        open={!!rejecting}
        width={500}
        modalProps={{
          destroyOnClose: true,
          maskClosable: false,
        }}
        onOpenChange={(open) => {
          if (!open) setRejecting(null);
        }}
        onFinish={async (values) => {
          const id = rejecting?.id;
          if (!id) return false;
          try {
            const res = await adminRejectInternalTransfer(
              { id: String(id) },
              values,
              { skipErrorHandler: true }
            );
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.internalTransfers.messages.rejectFailed", "拒绝失败")
              );
            message.success(
              t("pages.internalTransfers.messages.rejected", "已拒绝")
            );
            setRejecting(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.internalTransfers.messages.rejectFailed", "拒绝失败")
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="reason"
          label={t("pages.internalTransfers.form.reason", "拒绝原因")}
          placeholder={t(
            "pages.internalTransfers.form.reasonPlaceholder",
            "请输入拒绝原因"
          )}
          rules={[
            {
              required: true,
              message: t(
                "pages.internalTransfers.form.reasonRequired",
                "请输入拒绝原因"
              ),
            },
          ]}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText
          name="two_fa_code"
          label={t("pages.internalTransfers.form.twoFaCode", "2FA验证码")}
          placeholder={t(
            "pages.internalTransfers.form.twoFaCodePlaceholder",
            "请输入2FA验证码"
          )}
          rules={[
            {
              required: true,
              message: t(
                "pages.internalTransfers.form.twoFaCodeRequired",
                "请输入2FA验证码"
              ),
            },
          ]}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default InternalTransfersPage;
