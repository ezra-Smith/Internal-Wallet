import type {
  AdminApproveCurrencyWithdrawalBody,
  AdminCreateCurrencyWithdrawalRequest,
  AdminCurrencyWithdrawalEventItem,
  AdminCurrencyWithdrawalItem,
  AdminListCurrencyWithdrawalEventsParams,
  AdminListCurrencyWithdrawalsParams,
  AdminRejectCurrencyWithdrawalBody,
  AdminUpdateCurrencyWithdrawalBody,
} from "@/api/generated/schemas";
import {
  adminApproveCurrencyWithdrawal,
  adminCancelCurrencyWithdrawal,
  adminCreateCurrencyWithdrawal,
  adminGetCurrencyWithdrawal2,
  adminListCurrencyWithdrawalEvents,
  adminListCurrencyWithdrawals,
  adminRejectCurrencyWithdrawal,
  adminUpdateCurrencyWithdrawal,
} from "@/api/generated/withdrawals";
import { useRbac } from "@/hooks/useRbac";
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  DollarOutlined,
  DownloadOutlined,
  HistoryOutlined,
  SendOutlined,
} from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-components";
import {
  type ActionType,
  ModalForm,
  PageContainer,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from "@ant-design/pro-components";
import { useIntl } from "@umijs/max";
import { App, Button, Drawer, Input, Popconfirm, Space, Tag } from "antd";
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
    padding: "0 6px",
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
  list: "ListCurrencyWithdrawals",
  create: "CreateCurrencyWithdrawal",
  get: "GetCurrencyWithdrawal",
  update: "UpdateCurrencyWithdrawal",
  approve: "ApproveCurrencyWithdrawal",
  reject: "RejectCurrencyWithdrawal",
  cancel: "CancelCurrencyWithdrawal",
  events: "ListCurrencyWithdrawalEvents",
};

const WithdrawalsPage: React.FC = () => {
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

  const [createOpen, setCreateOpen] = useState(false);
  const [queryByTxOpen, setQueryByTxOpen] = useState(false);

  const [editing, setEditing] = useState<AdminCurrencyWithdrawalItem | null>(
    null
  );
  const [approving, setApproving] =
    useState<AdminCurrencyWithdrawalItem | null>(null);
  const [rejecting, setRejecting] =
    useState<AdminCurrencyWithdrawalItem | null>(null);
  const [completingPayout, setCompletingPayout] =
    useState<AdminCurrencyWithdrawalItem | null>(null);

  const eventsActionRef = useRef<ActionType | null>(null);
  const [eventsOpen, setEventsOpen] = useState(false);
  const [eventsTarget, setEventsTarget] =
    useState<AdminCurrencyWithdrawalItem | null>(null);
  const [eventsRows, setEventsRows] = useState<
    AdminCurrencyWithdrawalEventItem[]
  >([]);

  const [summary, setSummary] = useState<{
    total?: number;
    pending?: number;
    completed?: number;
    failed?: number;
    totalAmount?: string;
  }>({});

  const stats = useMemo(
    () => ({
      total: summary.total || 89,
      pending: summary.pending || 8,
      completed: summary.completed || 76,
      failed: summary.failed || 5,
      totalAmount: summary.totalAmount || "52,180.00",
    }),
    [summary]
  );

  const withdrawalStatusValueEnum = {
    pending: { text: t("pages.withdrawals.status.pending", "待处理") },
    processing: { text: t("pages.withdrawals.status.processing", "处理中") },
    completed: { text: t("pages.withdrawals.status.completed", "已完成") },
    failed: { text: t("pages.withdrawals.status.failed", "失败") },
    cancelled: { text: t("pages.withdrawals.status.cancelled", "已取消") },
    rejected: { text: t("pages.withdrawals.status.rejected", "已拒绝") },
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
        return t("pages.withdrawals.status.completed", "已完成");
      case "processing":
        return t("pages.withdrawals.status.processing", "处理中");
      case "pending":
        return t("pages.withdrawals.status.pending", "待处理");
      case "failed":
        return t("pages.withdrawals.status.failed", "失败");
      case "cancelled":
        return t("pages.withdrawals.status.cancelled", "已取消");
      case "rejected":
        return t("pages.withdrawals.status.rejected", "已拒绝");
      default:
        return status || "-";
    }
  };

  const getStrategyText = (strategy?: string) => {
    const s = (strategy || "").trim();
    if (s === "auto")
      return t("pages.withdrawals.strategy.auto", "自动审核+自动放币");
    if (s === "manual_auto")
      return t("pages.withdrawals.strategy.manualAuto", "人工审核后自动放币");
    if (s === "manual_manual")
      return t("pages.withdrawals.strategy.manualManual", "人工审核后手动转账");
    return s || "-";
  };

  const getFeeRuleSourceText = (source?: string) => {
    const s = (source || "").trim();
    if (!s) return "-";
    return t(`pages.withdrawals.feeRuleSource.${s}`, s);
  };

  const getEventTypeText = (eventType?: string) => {
    const v = (eventType || "").trim();
    if (!v) return "-";
    return t(`pages.withdrawals.eventType.${v}`, v);
  };

  const getActorTypeText = (actorType?: string) => {
    const v = (actorType || "").trim();
    if (!v) return "-";
    return t(`pages.withdrawals.actorType.${v}`, v);
  };

  const formatEventActor = (row: AdminCurrencyWithdrawalEventItem) => {
    const actorType = (row.actor_type || "").trim();
    const actorId = (row.actor_id || "").trim();
    const actorTypeText = getActorTypeText(actorType);
    if (!actorType || actorType === "system") return actorTypeText;
    return actorId ? `${actorTypeText} #${actorId}` : actorTypeText;
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
        <Tag className={styles.strategyTag} color="red">
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
    if (s.includes(".")) return s.replace(/\.?0+$/, "");
    return s;
  };

  const csvEscape = (value: unknown) => {
    if (value == null) return '""';
    const s = String(value);
    return `"${s
      .replaceAll('"', '""')
      .replaceAll("\r\n", "\n")
      .replaceAll("\r", "\n")}"`;
  };

  const exportEventsCsv = () => {
    if (!eventsTarget?.id) return;
    if (!eventsRows.length) {
      message.info(
        t("pages.withdrawals.events.messages.noData", "暂无数据可导出")
      );
      return;
    }
    const header = [
      "id",
      "withdraw_order_id",
      "event_type",
      "summary",
      "actor_type",
      "actor_id",
      "ip",
      "created_at",
      "details",
    ];
    const lines = eventsRows.map((r) =>
      [
        csvEscape(r.id),
        csvEscape(r.withdraw_order_id),
        csvEscape(r.event_type),
        csvEscape(r.summary),
        csvEscape(r.actor_type),
        csvEscape(r.actor_id),
        csvEscape(r.ip),
        csvEscape(r.created_at),
        csvEscape(r.details ? JSON.stringify(r.details) : ""),
      ].join(",")
    );
    const csv = [header.join(","), ...lines].join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `withdrawal-events-${eventsTarget.id}-page.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const toDateOnly = (v: any) => {
    if (!v) return undefined;
    if (typeof v === "string") return v.slice(0, 10);
    if (v instanceof Date) return v.toISOString().slice(0, 10);
    if (typeof v?.format === "function") return v.format("YYYY-MM-DD");
    return undefined;
  };

  const columns: ProColumns<AdminCurrencyWithdrawalItem>[] = [
    {
      title: t("pages.withdrawals.columns.id", "ID"),
      dataIndex: "id",
      copyable: true,
      width: 130,
      hideInSearch: true,
      render: (_, row) => (
        <span style={{ fontFamily: "monospace", fontSize: 13 }}>{row.id}</span>
      ),
    },
    {
      title: t("pages.withdrawals.columns.userId", "用户ID"),
      dataIndex: "user_id",
      copyable: true,
      width: 120,
      search: { transform: (v) => ({ user_id: Number(v) }) },
      render: (_, row) => (
        <span style={{ fontFamily: "monospace", fontSize: 13 }}>
          {row.user_id}
        </span>
      ),
    },
    {
      title: t("pages.withdrawals.currency.columns.asset", "资产"),
      dataIndex: "asset_code",
      width: 90,
      search: { transform: (v) => ({ asset_code: v }) },
      render: (_, row) => (
        <Tag color="blue" style={{ borderRadius: 4, fontWeight: 500 }}>
          {row.asset_code}
        </Tag>
      ),
    },
    {
      title: t("pages.withdrawals.currency.columns.chain", "链"),
      dataIndex: "chain_code",
      width: 90,
      search: { transform: (v) => ({ chain_code: v }) },
      render: (_, row) => (
        <Tag color="purple" style={{ borderRadius: 4, fontWeight: 500 }}>
          {row.chain_code}
        </Tag>
      ),
    },
    {
      title: t("pages.withdrawals.columns.amount", "金额"),
      dataIndex: "amount",
      width: 120,
      hideInSearch: true,
      render: (_, row) => (
        <span className={styles.amountText}>
          {formatAmountOrDash(row.amount)}
        </span>
      ),
    },
    {
      title: t("pages.withdrawals.columns.fee", "手续费"),
      dataIndex: "fee",
      width: 100,
      hideInSearch: true,
      render: (_, row) => (
        <span style={{ color: "#6b7280" }}>{formatAmountOrDash(row.fee)}</span>
      ),
    },
    {
      title: t("pages.withdrawals.columns.feeRuleSummary", "手续费规则"),
      dataIndex: "fee_rule_summary",
      ellipsis: true,
      width: 220,
      hideInSearch: true,
      render: (_, row) => {
        const summaryText = (row.fee_rule_summary || "").trim();
        const source = (row.fee_rule_source || "").trim();
        return (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <span style={{ fontSize: 12, color: "#374151" }}>
              {summaryText || "-"}
            </span>
            {source ? (
              <Tag
                style={{ margin: 0, width: "fit-content" }}
                color={source === "asset" ? "purple" : "default"}
              >
                {getFeeRuleSourceText(source)}
              </Tag>
            ) : null}
          </div>
        );
      },
    },
    {
      title: t("pages.withdrawals.currency.columns.strategy", "策略"),
      dataIndex: "strategy",
      width: 110,
      valueEnum: {
        auto: { text: getStrategyText("auto") },
        manual_auto: { text: getStrategyText("manual_auto") },
        manual_manual: { text: getStrategyText("manual_manual") },
      },
      search: { transform: (v) => ({ strategy: v }) },
      render: (_, row) => strategyTag(row.strategy),
    },
    {
      title: t("pages.withdrawals.columns.status", "状态"),
      dataIndex: "status",
      width: 100,
      valueEnum: withdrawalStatusValueEnum,
      search: { transform: (v) => ({ status: v }) },
      render: (_, row) => (
        <Tag className={`${styles.statusTag} ${getStatusStyle(row.status)}`}>
          {getStatusText(row.status)}
        </Tag>
      ),
    },
    {
      title: t("pages.withdrawals.currency.columns.toAddress", "目标地址"),
      dataIndex: "to_address",
      copyable: true,
      ellipsis: true,
      width: 160,
      search: { transform: (v) => ({ to_address: v }) },
      render: (_, row) => (
        <span className={styles.addressText}>
          {row.to_address
            ? `${row.to_address.slice(0, 8)}...${row.to_address.slice(-6)}`
            : "-"}
        </span>
      ),
    },
    {
      title: t("pages.withdrawals.columns.txHash", "交易哈希"),
      dataIndex: "tx_hash",
      copyable: true,
      ellipsis: true,
      width: 140,
      search: { transform: (v) => ({ tx_hash: v }) },
      render: (_, row) =>
        row.tx_hash ? (
          <span className={styles.hashLink}>
            {`${row.tx_hash.slice(0, 8)}...${row.tx_hash.slice(-6)}`}
          </span>
        ) : (
          "-"
        ),
    },
    // {
    //   title: t("pages.withdrawals.columns.memo", "备注"),
    //   dataIndex: "memo_tag",
    //   ellipsis: true,
    //   width: 120,
    //   hideInSearch: true,
    // },
    {
      title: t("pages.withdrawals.currency.columns.errorMessage", "错误信息"),
      dataIndex: "error_message",
      ellipsis: true,
      width: 150,
      hideInSearch: true,
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
      title: t("pages.withdrawals.columns.auditAdminId", "审核管理员"),
      dataIndex: "audit_admin_id",
      width: 110,
      hideInSearch: true,
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
      title: t("pages.withdrawals.columns.createdBy", "创建者"),
      dataIndex: "created_by_admin_id",
      width: 100,
      hideInSearch: true,
    },
    {
      title: t("pages.withdrawals.columns.createdAt", "创建时间"),
      dataIndex: "created_at",
      valueType: "dateTime",
      width: 160,
      hideInSearch: true,
    },
    {
      title: t("pages.withdrawals.columns.updatedAt", "更新时间"),
      dataIndex: "updated_at",
      valueType: "dateTime",
      width: 160,
      hideInSearch: true,
    },
    {
      title: t("pages.withdrawals.columns.actions", "操作"),
      valueType: "option",
      width: 260,
      fixed: "right",
      hideInSearch: true,
      render: (_, row) => {
        const canUpdate = canRpc(PERM.update);
        const canApprove = canRpc(PERM.approve);
        const canReject = canRpc(PERM.reject);
        const canCancel = canRpc(PERM.cancel);
        const canEvents = canRpc(PERM.events);

        const id = row.id;
        const status = (row.status || "").trim();
        const strategy = (row.strategy || "").trim();

        const isPending = status === "pending";
        const isProcessing = status === "processing";
        const isManual =
          strategy === "manual_auto" || strategy === "manual_manual";
        const isManualManual = strategy === "manual_manual";
        const isImmutable =
          status === "completed" ||
          status === "failed" ||
          status === "cancelled" ||
          status === "rejected";
        const canCompletePayout = isProcessing && isManualManual;

        return (
          <Space size={4} wrap>
            <Button
              type="link"
              size="small"
              className={styles.actionBtn}
              icon={<HistoryOutlined />}
              disabled={!canEvents}
              onClick={() => {
                setEventsTarget(row);
                setEventsOpen(true);
              }}
            >
              {t("pages.withdrawals.actions.events", "事件")}
            </Button>

            {canCompletePayout ? (
              <Button
                type="link"
                size="small"
                className={styles.actionBtn}
                icon={<CheckCircleOutlined />}
                disabled={!canUpdate}
                onClick={() => setCompletingPayout(row)}
              >
                {t("pages.withdrawals.actions.completePayout", "完成打款")}
              </Button>
            ) : (
              <Button
                type="link"
                size="small"
                className={styles.actionBtn}
                disabled={!canUpdate || isImmutable}
                onClick={() => setEditing(row)}
              >
                {t("pages.withdrawals.actions.edit", "编辑")}
              </Button>
            )}

            {isPending ? (
              <>
                <Button
                  type="link"
                  size="small"
                  className={styles.actionBtn}
                  style={{ color: "#10b981" }}
                  disabled={!canApprove || !isManual}
                  onClick={() => setApproving(row)}
                >
                  {t("pages.withdrawals.actions.approve", "通过")}
                </Button>
                <Button
                  type="link"
                  size="small"
                  className={styles.actionBtn}
                  style={{ color: "#f59e0b" }}
                  disabled={!canReject}
                  onClick={() => setRejecting(row)}
                >
                  {t("pages.withdrawals.actions.reject", "拒绝")}
                </Button>
                <Popconfirm
                  title={t(
                    "pages.withdrawals.actions.cancelConfirm",
                    "确定取消此提现?"
                  )}
                  okButtonProps={{
                    danger: true,
                    disabled: !canCancel,
                  }}
                  onConfirm={() => {
                    if (!canCancel) return;
                    if (!id) return;
                    let reason = "";
                    modal.confirm({
                      title: t(
                        "pages.withdrawals.actions.confirmCancelTitle",
                        "确认取消"
                      ),
                      okButtonProps: { danger: true },
                      content: (
                        <div style={{ marginTop: 8 }}>
                          <div style={{ marginBottom: 8 }}>
                            {t(
                              "pages.withdrawals.actions.reasonOptional",
                              "原因（可选）"
                            )}
                          </div>
                          <Input.TextArea
                            autoSize
                            onChange={(e) => {
                              reason = e.target.value;
                            }}
                          />
                        </div>
                      ),
                      onOk: async () => {
                        try {
                          const res = await adminCancelCurrencyWithdrawal(
                            { id: String(id) },
                            { reason },
                            { skipErrorHandler: true }
                          );
                          if (!res?.success)
                            throw new Error(
                              res?.message ||
                                t(
                                  "pages.withdrawals.messages.cancelFailed",
                                  "取消失败"
                                )
                            );
                          message.success(
                            t("pages.withdrawals.messages.cancelled", "已取消")
                          );
                          actionRef.current?.reload();
                        } catch (e: any) {
                          message.error(
                            e?.message ||
                              t(
                                "pages.withdrawals.messages.cancelFailed",
                                "取消失败"
                              )
                          );
                        }
                      },
                    });
                  }}
                >
                  <Button
                    type="link"
                    size="small"
                    className={styles.actionBtn}
                    danger
                    disabled={!canCancel}
                  >
                    {t("pages.withdrawals.actions.cancel", "取消")}
                  </Button>
                </Popconfirm>
              </>
            ) : null}
          </Space>
        );
      },
    },
  ];

  const eventTypeOptions = [
    { value: "order.created", label: getEventTypeText("order.created") },
    { value: "fee.calculated", label: getEventTypeText("fee.calculated") },
    {
      value: "audit.rule_matched",
      label: getEventTypeText("audit.rule_matched"),
    },
    {
      value: "audit.whitelist_hit",
      label: getEventTypeText("audit.whitelist_hit"),
    },
    {
      value: "audit.whitelist_miss",
      label: getEventTypeText("audit.whitelist_miss"),
    },
    { value: "audit.approved", label: getEventTypeText("audit.approved") },
    { value: "audit.rejected", label: getEventTypeText("audit.rejected") },
    { value: "asset.frozen", label: getEventTypeText("asset.frozen") },
    { value: "asset.unfrozen", label: getEventTypeText("asset.unfrozen") },
    { value: "asset.deducted", label: getEventTypeText("asset.deducted") },
    { value: "fee.collected", label: getEventTypeText("fee.collected") },
    {
      value: "transfer.initiated",
      label: getEventTypeText("transfer.initiated"),
    },
    {
      value: "transfer.completed",
      label: getEventTypeText("transfer.completed"),
    },
    { value: "transfer.failed", label: getEventTypeText("transfer.failed") },
    { value: "order.cancelled", label: getEventTypeText("order.cancelled") },
  ];

  const actorTypeOptions = [
    { value: "system", label: getActorTypeText("system") },
    { value: "user", label: getActorTypeText("user") },
    { value: "admin", label: getActorTypeText("admin") },
  ];

  const eventColumns: ProColumns<AdminCurrencyWithdrawalEventItem>[] = [
    {
      title: t("pages.withdrawals.events.columns.eventType", "事件类型"),
      dataIndex: "event_type",
      width: 160,
      render: (_, row) => (
        <Tag style={{ margin: 0 }} color="blue">
          {getEventTypeText(row.event_type)}
        </Tag>
      ),
    },
    {
      title: t("pages.withdrawals.events.columns.summary", "摘要"),
      dataIndex: "summary",
      ellipsis: true,
    },
    {
      title: t("pages.withdrawals.events.columns.actor", "操作人"),
      dataIndex: "actor_id",
      width: 160,
      render: (_, row) => (
        <span style={{ fontFamily: "monospace", fontSize: 12 }}>
          {formatEventActor(row)}
        </span>
      ),
    },
    {
      title: t("pages.withdrawals.events.columns.ip", "IP"),
      dataIndex: "ip",
      width: 140,
      render: (_, row) =>
        row.ip ? (
          <span style={{ fontFamily: "monospace", fontSize: 12 }}>
            {row.ip}
          </span>
        ) : (
          "-"
        ),
    },
    {
      title: t("pages.withdrawals.events.columns.createdAt", "时间"),
      dataIndex: "created_at",
      valueType: "dateTime",
      width: 170,
    },
    {
      title: t("pages.withdrawals.events.search.eventTypes", "事件类型"),
      dataIndex: "event_types",
      hideInTable: true,
      valueType: "select",
      fieldProps: {
        mode: "multiple",
        options: eventTypeOptions,
        placeholder: t(
          "pages.withdrawals.events.search.eventTypesPlaceholder",
          "请选择"
        ),
      },
      search: { transform: (v) => ({ event_types: v }) },
    },
    {
      title: t("pages.withdrawals.events.search.actorTypes", "操作人类型"),
      dataIndex: "actor_types",
      hideInTable: true,
      valueType: "select",
      fieldProps: {
        mode: "multiple",
        options: actorTypeOptions,
        placeholder: t(
          "pages.withdrawals.events.search.actorTypesPlaceholder",
          "请选择"
        ),
      },
      search: { transform: (v) => ({ actor_types: v }) },
    },
    {
      title: t("pages.withdrawals.events.search.dateRange", "时间范围"),
      dataIndex: "created_time_range",
      hideInTable: true,
      valueType: "dateRange",
      search: {
        transform: (v) => ({
          date_from: toDateOnly(v?.[0]),
          date_to: toDateOnly(v?.[1]),
        }),
      },
    },
  ];

  return (
    <PageContainer header={{ title: null, breadcrumb: {} }}>
      <div className={styles.container}>
        <>
          <div className={styles.header}>
            <div className={styles.headerLeft}>
              <div className={styles.title}>
                {t("pages.withdrawals.title", "提现记录")}
              </div>
              <div className={styles.subtitle}>
                {t(
                  "pages.withdrawals.subtitle",
                  "管理和查看所有用户提现交易记录"
                )}
              </div>
            </div>
          </div>

          <div className={styles.statsRow}>
            <div className={styles.statsCard}>
              <div className={styles.statsLeft}>
                <div className={`${styles.statsIcon} ${styles.statsIconBlue}`}>
                  <SendOutlined />
                </div>
                <div className={styles.statsContent}>
                  <span className={styles.statsLabel}>
                    {t("pages.withdrawals.stats.total", "总提现笔数")}
                  </span>
                  <span className={styles.statsSublabel}>
                    {t("pages.withdrawals.stats.allRecords", "所有提现记录")}
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
                  {t("pages.withdrawals.unit.count", "笔")}
                </span>
              </div>
            </div>

            <div className={styles.statsCard}>
              <div className={styles.statsLeft}>
                <div
                  className={`${styles.statsIcon} ${styles.statsIconOrange}`}
                >
                  <ClockCircleOutlined />
                </div>
                <div className={styles.statsContent}>
                  <span className={styles.statsLabel}>
                    {t("pages.withdrawals.stats.pending", "待处理")}
                  </span>
                  <span className={styles.statsSublabel}>
                    {t("pages.withdrawals.stats.pendingDesc", "等待审核")}
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
                  {t("pages.withdrawals.unit.count", "笔")}
                </span>
              </div>
            </div>

            <div className={styles.statsCard}>
              <div className={styles.statsLeft}>
                <div className={`${styles.statsIcon} ${styles.statsIconGreen}`}>
                  <CheckCircleOutlined />
                </div>
                <div className={styles.statsContent}>
                  <span className={styles.statsLabel}>
                    {t("pages.withdrawals.stats.completed", "已完成")}
                  </span>
                  <span className={styles.statsSublabel}>
                    {t("pages.withdrawals.stats.completedDesc", "成功出账")}
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
                  {t("pages.withdrawals.unit.count", "笔")}
                </span>
              </div>
            </div>

            <div className={styles.statsCard}>
              <div className={styles.statsLeft}>
                <div className={`${styles.statsIcon} ${styles.statsIconRed}`}>
                  <DollarOutlined />
                </div>
                <div className={styles.statsContent}>
                  <span className={styles.statsLabel}>
                    {t("pages.withdrawals.stats.totalAmount", "累计提现金额")}
                  </span>
                  <span className={styles.statsSublabel}>
                    {t("pages.withdrawals.stats.usdEquivalent", "折合USD")}
                  </span>
                </div>
              </div>
              <div className={styles.statsValue}>
                <span
                  className={`${styles.statsNumber} ${styles.statsNumberRed}`}
                >
                  ${stats.totalAmount}
                </span>
              </div>
            </div>
          </div>

          <div className={styles.tableCard}>
            <ProTable<AdminCurrencyWithdrawalItem>
              actionRef={actionRef}
              rowKey={(row) => row.id || row.tx_hash || JSON.stringify(row)}
              columns={columns}
              request={async (params) => {
                if (!canRpc(PERM.list))
                  return { success: true, data: [], total: 0 };
                try {
                  const res = await adminListCurrencyWithdrawals(
                    {
                      page: params.current,
                      page_size: params.pageSize,
                      user_id: (params as any).user_id,
                      asset_code: (params as any).asset_code,
                      chain_code: (params as any).chain_code,
                      status: (params as any).status,
                      strategy: (params as any).strategy,
                      to_address: (params as any).to_address,
                      tx_hash: (params as any).tx_hash,
                      created_from: (params as any).created_from,
                      created_to: (params as any).created_to,
                      min_amount: (params as any).min_amount,
                      max_amount: (params as any).max_amount,
                    } as AdminListCurrencyWithdrawalsParams,
                    { skipErrorHandler: true }
                  );
                  const summaryAny = (res as any)?.data?.summary;
                  if (summaryAny) setSummary(summaryAny);
                  return {
                    success: !!res?.success,
                    data: res.data?.withdrawals ?? [],
                    total: Number(res.data?.pagination?.total ?? 0),
                  };
                } catch (e: any) {
                  message.error(
                    e?.message ||
                      t("pages.withdrawals.messages.loadFailed", "加载失败")
                  );
                  return { success: false, data: [], total: 0 };
                }
              }}
              search={{ labelWidth: 100 }}
              scroll={{ x: 2200 }}
              options={{ density: true, fullScreen: true, reload: true }}
              pagination={{ showSizeChanger: true, showQuickJumper: true }}
            />
          </div>
        </>
      </div>

      <Drawer
        title={t("pages.withdrawals.events.titleWithId", "提现事件：{id}", {
          id: eventsTarget?.id || "-",
        })}
        open={eventsOpen}
        onClose={() => {
          setEventsOpen(false);
          setEventsTarget(null);
          setEventsRows([]);
        }}
        width={980}
        destroyOnClose
        extra={
          <Button
            icon={<DownloadOutlined />}
            onClick={exportEventsCsv}
            disabled={!eventsRows.length}
          >
            {t("pages.withdrawals.events.actions.exportCsv", "导出CSV")}
          </Button>
        }
      >
        <ProTable<AdminCurrencyWithdrawalEventItem>
          actionRef={eventsActionRef}
          rowKey={(row) => row.id || JSON.stringify(row)}
          columns={eventColumns}
          search={{ labelWidth: 88 }}
          scroll={{ x: 900 }}
          pagination={{ showSizeChanger: true }}
          options={{ density: true, reload: true }}
          request={async (params) => {
            const id = (eventsTarget?.id || "").trim();
            if (!id) return { success: true, data: [], total: 0 };
            if (!canRpc(PERM.events))
              return { success: true, data: [], total: 0 };
            try {
              const res = await adminListCurrencyWithdrawalEvents(
                { id },
                {
                  page: params.current,
                  page_size: params.pageSize,
                  event_types: (params as any).event_types,
                  actor_types: (params as any).actor_types,
                  date_from: (params as any).date_from,
                  date_to: (params as any).date_to,
                } as AdminListCurrencyWithdrawalEventsParams,
                { skipErrorHandler: true }
              );
              const list = res.data?.events ?? [];
              setEventsRows(list);
              return {
                success: !!res?.success,
                data: list,
                total: Number(res.data?.pagination?.total ?? 0),
              };
            } catch (e: any) {
              message.error(
                e?.message ||
                  t(
                    "pages.withdrawals.events.messages.loadFailed",
                    "加载事件失败"
                  )
              );
              return { success: false, data: [], total: 0 };
            }
          }}
          expandable={{
            expandedRowRender: (record) => (
              <pre style={{ margin: 0, whiteSpace: "pre-wrap" }}>
                {JSON.stringify(record.details ?? null, null, 2)}
              </pre>
            ),
            rowExpandable: (record) =>
              !!record.details && Object.keys(record.details).length > 0,
          }}
        />
      </Drawer>

      <ModalForm<{ tx_hash: string }>
        title={t("pages.withdrawals.queryByTx.title", "根据交易哈希查询")}
        open={queryByTxOpen}
        onOpenChange={setQueryByTxOpen}
        width={500}
        onFinish={async (values) => {
          const txHash = (values?.tx_hash || "").trim();
          if (!txHash) {
            message.error(
              t("pages.withdrawals.messages.invalidTxHash", "无效的交易哈希")
            );
            return false;
          }
          try {
            const res = await adminGetCurrencyWithdrawal2(
              { txHash },
              undefined,
              { skipErrorHandler: true }
            );
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.withdrawals.messages.loadFailed", "加载失败")
              );
            const w = res?.data?.withdrawal;
            modal.info({
              title: t("pages.withdrawals.queryByTx.resultTitle", "提现详情"),
              width: 720,
              content: (
                <pre style={{ margin: 0, whiteSpace: "pre-wrap" }}>
                  {JSON.stringify(w ?? null, null, 2)}
                </pre>
              ),
            });
            setQueryByTxOpen(false);
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.withdrawals.messages.loadFailed", "加载失败")
            );
            return false;
          }
        }}
      >
        <ProFormText
          name="tx_hash"
          label={t("pages.withdrawals.form.txHash", "交易哈希")}
          placeholder={t(
            "pages.withdrawals.form.txHashPlaceholder",
            "请输入交易哈希"
          )}
          rules={[
            {
              required: true,
              message: t(
                "pages.withdrawals.form.txHashRequired",
                "请输入交易哈希"
              ),
            },
          ]}
        />
      </ModalForm>

      <ModalForm<AdminCreateCurrencyWithdrawalRequest>
        title={t("pages.withdrawals.create.title", "创建提现")}
        open={createOpen}
        onOpenChange={setCreateOpen}
        width={560}
        modalProps={{ destroyOnClose: true, maskClosable: false }}
        onFinish={async (values) => {
          const strategy = (values?.strategy || "manual_manual").trim();
          const fromAddress = (values?.from_address || "").trim();
          if (strategy === "manual_auto" && !fromAddress) {
            message.error(
              t(
                "pages.withdrawals.messages.fromAddressRequired",
                "manual_auto策略需要填写来源地址"
              )
            );
            return false;
          }
          try {
            const res = await adminCreateCurrencyWithdrawal(values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.withdrawals.messages.createFailed", "创建失败")
              );
            message.success(
              t("pages.withdrawals.messages.created", "创建成功")
            );
            setCreateOpen(false);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.withdrawals.messages.createFailed", "创建失败")
            );
            return false;
          }
        }}
      >
        <ProFormText
          name="user_id"
          label={t("pages.withdrawals.form.userId", "用户ID")}
          placeholder={t(
            "pages.withdrawals.form.userIdPlaceholder",
            "请输入用户ID"
          )}
          rules={[
            {
              required: true,
              message: t(
                "pages.withdrawals.form.userIdRequired",
                "请输入用户ID"
              ),
            },
          ]}
        />
        <ProFormText
          name="asset_code"
          label={t("pages.withdrawals.currency.form.asset", "资产")}
          placeholder={t(
            "pages.withdrawals.form.assetPlaceholder",
            "请输入资产代码，如USDT"
          )}
          rules={[
            {
              required: true,
              message: t("pages.withdrawals.form.assetRequired", "请输入资产"),
            },
          ]}
        />
        <ProFormText
          name="chain_code"
          label={t("pages.withdrawals.currency.form.chain", "链")}
          placeholder={t(
            "pages.withdrawals.form.chainPlaceholder",
            "请输入链代码，如ETH"
          )}
          rules={[
            {
              required: true,
              message: t("pages.withdrawals.form.chainRequired", "请输入链"),
            },
          ]}
        />
        <ProFormText
          name="to_address"
          label={t("pages.withdrawals.currency.form.toAddress", "目标地址")}
          placeholder={t(
            "pages.withdrawals.form.toAddressPlaceholder",
            "请输入目标地址"
          )}
          rules={[
            {
              required: true,
              message: t(
                "pages.withdrawals.form.toAddressRequired",
                "请输入目标地址"
              ),
            },
          ]}
        />
        <ProFormText
          name="amount"
          label={t("pages.withdrawals.form.amount", "金额")}
          placeholder={t(
            "pages.withdrawals.form.amountPlaceholder",
            "请输入金额"
          )}
          rules={[
            {
              required: true,
              message: t("pages.withdrawals.form.amountRequired", "请输入金额"),
            },
          ]}
        />
        <ProFormText
          name="fee"
          label={t("pages.withdrawals.form.fee", "手续费")}
          placeholder={t(
            "pages.withdrawals.form.feePlaceholder",
            "请输入手续费"
          )}
        />
        <ProFormText
          name="memo_tag"
          label={t("pages.withdrawals.form.memo", "备注")}
          placeholder={t(
            "pages.withdrawals.form.memoPlaceholder",
            "请输入备注（可选）"
          )}
        />
        <ProFormSelect
          name="strategy"
          label={t("pages.withdrawals.currency.form.strategy", "策略")}
          initialValue="manual_manual"
          placeholder={t(
            "pages.withdrawals.form.strategyPlaceholder",
            "请选择策略"
          )}
          options={[
            {
              label: t("pages.withdrawals.strategy.manualManual", "手动+手动"),
              value: "manual_manual",
            },
            {
              label: t("pages.withdrawals.strategy.manualAuto", "手动+自动"),
              value: "manual_auto",
            },
          ]}
        />
        <ProFormText
          name="from_address"
          label={t("pages.withdrawals.currency.form.fromAddress", "来源地址")}
          placeholder={t(
            "pages.withdrawals.form.fromAddressPlaceholder",
            "manual_auto策略时必填"
          )}
        />
      </ModalForm>

      <ModalForm<AdminUpdateCurrencyWithdrawalBody>
        title={t("pages.withdrawals.edit.title", "编辑提现")}
        open={!!editing}
        width={500}
        modalProps={{ destroyOnClose: true, maskClosable: false }}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
        initialValues={{
          status: editing?.status,
          tx_hash: editing?.tx_hash,
          transfer_note: editing?.transfer_note,
        }}
        onFinish={async (values) => {
          const id = editing?.id;
          if (!id) return false;
          try {
            const res = await adminUpdateCurrencyWithdrawal(
              { id: String(id) },
              values,
              { skipErrorHandler: true }
            );
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.withdrawals.messages.updateFailed", "更新失败")
              );
            message.success(
              t("pages.withdrawals.messages.updated", "更新成功")
            );
            setEditing(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.withdrawals.messages.updateFailed", "更新失败")
            );
            return false;
          }
        }}
      >
        <ProFormSelect
          name="status"
          label={t("pages.withdrawals.form.status", "状态")}
          placeholder={t(
            "pages.withdrawals.form.statusPlaceholder",
            "请选择状态"
          )}
          options={(() => {
            const status = (editing?.status || "").trim();
            if (status === "pending") {
              return [
                {
                  label: t("pages.withdrawals.status.pending", "待处理"),
                  value: "pending",
                  disabled: true,
                },
                {
                  label: t("pages.withdrawals.status.processing", "处理中"),
                  value: "processing",
                },
                {
                  label: t("pages.withdrawals.status.failed", "失败"),
                  value: "failed",
                },
                {
                  label: t("pages.withdrawals.status.cancelled", "已取消"),
                  value: "cancelled",
                },
              ];
            }
            if (status === "processing") {
              return [
                {
                  label: t("pages.withdrawals.status.processing", "处理中"),
                  value: "processing",
                  disabled: true,
                },
                {
                  label: t("pages.withdrawals.status.failed", "失败"),
                  value: "failed",
                },
              ];
            }
            return [];
          })()}
        />
        <ProFormTextArea
          name="transfer_note"
          label={t("pages.withdrawals.form.transferNote", "转账备注")}
          placeholder={t(
            "pages.withdrawals.form.transferNotePlaceholder",
            "请输入转账备注"
          )}
          fieldProps={{ rows: 3 }}
        />
      </ModalForm>

      <ModalForm<{ tx_hash: string }>
        title={t("pages.withdrawals.completePayout.title", "完成打款")}
        open={!!completingPayout}
        width={520}
        modalProps={{ destroyOnClose: true, maskClosable: false }}
        onOpenChange={(open) => {
          if (!open) setCompletingPayout(null);
        }}
        initialValues={{ tx_hash: completingPayout?.tx_hash }}
        onFinish={async (values) => {
          const id = completingPayout?.id;
          const status = (completingPayout?.status || "").trim();
          const strategy = (completingPayout?.strategy || "").trim();
          const txHash = (values?.tx_hash || "").trim();

          if (strategy !== "manual_manual" || status !== "processing") {
            message.error(
              t(
                "pages.withdrawals.completePayout.messages.notAllowed",
                "当前记录不支持完成打款"
              )
            );
            return false;
          }

          if (!id) return false;

          if (!txHash) {
            message.error(
              t(
                "pages.withdrawals.completePayout.form.txHashRequired",
                "请输入交易哈希"
              )
            );
            return false;
          }

          try {
            const res = await adminUpdateCurrencyWithdrawal(
              { id: String(id) },
              {
                status: "completed",
                tx_hash: txHash,
              } as AdminUpdateCurrencyWithdrawalBody,
              { skipErrorHandler: true }
            );
            if (!res?.success) {
              throw new Error(
                res?.message ||
                  t(
                    "pages.withdrawals.completePayout.messages.failed",
                    "完成打款失败"
                  )
              );
            }
            message.success(
              t(
                "pages.withdrawals.completePayout.messages.success",
                "已完成打款"
              )
            );
            setCompletingPayout(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.withdrawals.completePayout.messages.failed",
                  "完成打款失败"
                )
            );
            return false;
          }
        }}
      >
        <ProFormText
          name="tx_hash"
          label={t("pages.withdrawals.completePayout.form.txHash", "交易哈希")}
          placeholder={t(
            "pages.withdrawals.completePayout.form.txHashPlaceholder",
            "请输入交易哈希"
          )}
          rules={[
            {
              required: true,
              message: t(
                "pages.withdrawals.completePayout.form.txHashRequired",
                "请输入交易哈希"
              ),
            },
          ]}
        />
      </ModalForm>

      <ModalForm<AdminApproveCurrencyWithdrawalBody>
        title={t("pages.withdrawals.approve.title", "审批通过")}
        open={!!approving}
        width={500}
        modalProps={{ destroyOnClose: true, maskClosable: false }}
        onOpenChange={(open) => {
          if (!open) setApproving(null);
        }}
        onFinish={async (values) => {
          const id = approving?.id;
          if (!id) return false;
          try {
            const res = await adminApproveCurrencyWithdrawal(
              { id: String(id) },
              values,
              { skipErrorHandler: true }
            );
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.withdrawals.messages.approveFailed", "审批失败")
              );
            message.success(
              t("pages.withdrawals.messages.approved", "审批通过")
            );
            setApproving(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.withdrawals.messages.approveFailed", "审批失败")
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="audit_note"
          label={t("pages.withdrawals.form.auditNote", "审核备注")}
          placeholder={t(
            "pages.withdrawals.form.auditNotePlaceholder",
            "请输入审核备注（可选）"
          )}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText
          name="two_fa_code"
          label={t("pages.withdrawals.form.twoFaCode", "2FA验证码")}
          placeholder={t(
            "pages.withdrawals.form.twoFaCodePlaceholder",
            "请输入2FA验证码"
          )}
          rules={[
            {
              required: true,
              message: t(
                "pages.withdrawals.form.twoFaCodeRequired",
                "请输入2FA验证码"
              ),
            },
          ]}
        />
      </ModalForm>

      <ModalForm<AdminRejectCurrencyWithdrawalBody>
        title={t("pages.withdrawals.reject.title", "拒绝提现")}
        open={!!rejecting}
        width={500}
        modalProps={{ destroyOnClose: true, maskClosable: false }}
        onOpenChange={(open) => {
          if (!open) setRejecting(null);
        }}
        onFinish={async (values) => {
          const id = rejecting?.id;
          if (!id) return false;
          try {
            const res = await adminRejectCurrencyWithdrawal(
              { id: String(id) },
              values,
              { skipErrorHandler: true }
            );
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.withdrawals.messages.rejectFailed", "拒绝失败")
              );
            message.success(t("pages.withdrawals.messages.rejected", "已拒绝"));
            setRejecting(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.withdrawals.messages.rejectFailed", "拒绝失败")
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="reason"
          label={t("pages.withdrawals.form.reason", "拒绝原因")}
          placeholder={t(
            "pages.withdrawals.form.reasonPlaceholder",
            "请输入拒绝原因"
          )}
          rules={[
            {
              required: true,
              message: t(
                "pages.withdrawals.form.reasonRequired",
                "请输入拒绝原因"
              ),
            },
          ]}
          fieldProps={{ rows: 3 }}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default WithdrawalsPage;
