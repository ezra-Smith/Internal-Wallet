import type {
  AdminApproveTransferBatchBody,
  AdminCancelTransferBatchBody,
  AdminExecuteTransferBatchBody,
  AdminSubmitTransferBatchBody,
  AdminTransferBatchItem,
  AdminTransferBatchSummary,
} from "@/api/generated/schemas";
import {
  adminApproveTransferBatch,
  adminCancelTransferBatch,
  adminExecuteTransferBatch,
  adminListTransferBatches,
  adminSubmitTransferBatch,
} from "@/api/generated/transfers";
import { useRbac } from "@/hooks/useRbac";
import { formatLocalDateTime } from "@/utils/date";
import {
  CalendarOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  DollarOutlined,
  EyeOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  SendOutlined,
  StopOutlined,
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
import { history, useIntl } from "@umijs/max";
import {
  App,
  AutoComplete,
  Button,
  DatePicker,
  Input,
  Select,
  Space,
  Tag,
  Typography,
} from "antd";
import { createStyles } from "antd-style";
import type { Dayjs } from "dayjs";
import React, { useEffect, useMemo, useRef, useState } from "react";
import CreateBatchModal from "./components/CreateBatchModal";

const FILTER_CONTROL_HEIGHT = 32;
const FILTER_CONTROL_RADIUS = 8;

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
    gap: 12,
    flexWrap: "wrap",
    marginBottom: 24,
  },
  headerLeft: {
    flex: 1,
    minWidth: 260,
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
    gridTemplateColumns: "repeat(2, 1fr)",
    gap: 16,
    marginBottom: 24,
    "@media (max-width: 992px)": {
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
    gap: 16,
    flexShrink: 0,
  },
  statsIcon: {
    width: 48,
    height: 48,
    borderRadius: 12,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    fontSize: 22,
    flexShrink: 0,
  },
  statsIconBlue: {
    background: "linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)",
    color: "#3b82f6",
  },
  statsIconOrange: {
    background: "linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%)",
    color: "#f59e0b",
  },
  statsIconGreen: {
    background: "linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%)",
    color: "#10b981",
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
    fontSize: 22,
    fontWeight: 700,
    fontFamily: '"DIN Alternate", "Bebas Neue", sans-serif',
    whiteSpace: "nowrap",
  },
  statsNumberBlue: {
    color: "#3b82f6",
  },
  statsNumberOrange: {
    color: "#f59e0b",
  },
  statsNumberGreen: {
    color: "#10b981",
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
  filterCard: {
    background: "#fff",
    borderRadius: 12,
    padding: "16px 20px",
    border: "1px solid #f0f0f0",
    marginBottom: 16,
  },
  filterGrid: {
    display: "grid",
    gridTemplateColumns: "repeat(4, minmax(0, 1fr))",
    gap: 12,
    alignItems: "end",
    "@media (max-width: 1200px)": {
      gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
    },
    "@media (max-width: 576px)": {
      gridTemplateColumns: "1fr",
    },
  },
  fieldLabel: {
    fontSize: 12,
    color: "#6b7280",
    marginBottom: 4,
    userSelect: "none",
  },
  input: {
    height: FILTER_CONTROL_HEIGHT,
    borderRadius: FILTER_CONTROL_RADIUS,
    width: "100%",
  },
  select: {
    width: "100%",
    "& .ant-select-selector": {
      height: `${FILTER_CONTROL_HEIGHT}px !important`,
      borderRadius: `${FILTER_CONTROL_RADIUS}px !important`,
      alignItems: "center",
    },
    "& .ant-select-selection-search-input": {
      height: `${FILTER_CONTROL_HEIGHT - 2}px !important`,
    },
  },
  dateRangePicker: {
    width: "100%",
    height: FILTER_CONTROL_HEIGHT,
    borderRadius: FILTER_CONTROL_RADIUS,
  },
  filterActions: {
    display: "flex",
    gap: 10,
    justifyContent: "flex-end",
    flexWrap: "wrap",
    marginTop: 12,
    "& .ant-btn": {
      height: FILTER_CONTROL_HEIGHT,
      borderRadius: FILTER_CONTROL_RADIUS,
    },
  },
  tableCard: {
    background: "#fff",
    borderRadius: 12,
    overflow: "hidden",
    border: "1px solid #f0f0f0",
    ".ant-pro-table-list-toolbar": {
      display: "none",
    },
    ".ant-table-thead > tr > th": {
      background: "#fafafa",
      fontWeight: 600,
    },
    ".ant-table-tbody > tr:hover > td": {
      background: "#f9fafb",
    },
  },
  pillTag: {
    borderRadius: 999,
    fontWeight: 600,
    border: "none",
    height: 26,
    display: "inline-flex",
    alignItems: "center",
    gap: 6,
    paddingInline: 10,
  },
  statusCompleted: {
    color: "#10b981",
    background: "#ecfdf5",
  },
  statusDraft: {
    color: "#6b7280",
    background: "#f3f4f6",
  },
  statusPending: {
    color: "#f59e0b",
    background: "#fffbeb",
  },
  statusApproved: {
    color: "#3b82f6",
    background: "#eff6ff",
  },
  statusProcessing: {
    color: "#7c3aed",
    background: "#f5f3ff",
  },
  statusFailed: {
    color: "#ef4444",
    background: "#fef2f2",
  },
  statusCancelled: {
    color: "#9ca3af",
    background: "#f3f4f6",
  },
  primaryLink: {
    padding: 0,
    height: "auto",
  },
  actionBtn: {
    padding: "0 8px",
    fontSize: 13,
  },
  mono: {
    fontFamily:
      'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
    fontVariantNumeric: "tabular-nums",
  },
  amountStrong: {
    fontWeight: 700,
    color: "#111827",
    fontVariantNumeric: "tabular-nums",
  },
  amountSub: {
    fontSize: 12,
    color: "#6b7280",
    marginTop: 2,
  },
}));

const { RangePicker } = DatePicker;

const PERM = {
  list: "ListTransferBatches",
  import: "ImportTransferBatch",
  submit: "SubmitTransferBatch",
  approve: "ApproveTransferBatch",
  cancel: "CancelTransferBatch",
  execute: "ExecuteTransferBatch",
  detail: "GetTransferBatch",
};

type BatchAction =
  | { type: "submit"; row: AdminTransferBatchItem }
  | { type: "approve"; row: AdminTransferBatchItem }
  | { type: "execute"; row: AdminTransferBatchItem }
  | { type: "cancel"; row: AdminTransferBatchItem };

type BatchFilters = {
  keyword: string;
  status?: string;
  currency?: string;
  dateRange: [Dayjs | null, Dayjs | null] | null;
};

const getInitialFilters = (): BatchFilters => ({
  keyword: "",
  status: undefined,
  currency: undefined,
  dateRange: null,
});

const TransferBatchesPage: React.FC = () => {
  const { styles } = useStyles();
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (
    id: string,
    defaultMessage: string,
    values?: Record<string, any>
  ) => intl.formatMessage({ id, defaultMessage }, values);

  const actionRef = useRef<ActionType | null>(null);
  const [summary, setSummary] = useState<AdminTransferBatchSummary | null>(
    null
  );
  const [batchAction, setBatchAction] = useState<BatchAction | null>(null);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [draftFilters, setDraftFilters] =
    useState<BatchFilters>(getInitialFilters);
  const [filters, setFilters] = useState<BatchFilters>(getInitialFilters);
  const [suggestedCurrencies, setSuggestedCurrencies] = useState<string[]>([]);

  const formatInt = (value: string | number | undefined | null): string => {
    if (value === undefined || value === null || value === "") return "0";
    const num =
      typeof value === "string"
        ? Number.parseInt(value, 10)
        : Math.trunc(value);
    if (!Number.isFinite(num)) return String(value);
    return num.toLocaleString("en-US");
  };

  const formatUsd = (value: string | number | undefined | null): string => {
    if (value === undefined || value === null || value === "") return "0.00";
    const num = typeof value === "string" ? Number(value) : value;
    if (!Number.isFinite(num)) return "0.00";
    return num.toLocaleString("en-US", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  };

  const stats = useMemo(
    () => ({
      processingBatches: formatInt(summary?.processing_batches),
      totalAmountUsd: formatUsd(summary?.total_transferred_usd),
    }),
    [summary]
  );

  const getStatusStyle = (status?: string) => {
    switch (status) {
      case "completed":
        return styles.statusCompleted;
      case "draft":
        return styles.statusDraft;
      case "pending":
        return styles.statusPending;
      case "approved":
        return styles.statusApproved;
      case "processing":
        return styles.statusProcessing;
      case "partial_failed":
        return styles.statusFailed;
      case "cancelled":
        return styles.statusCancelled;
      default:
        return "";
    }
  };

  const getStatusText = (status?: string) => {
    switch (status) {
      case "completed":
        return t("pages.transfers.batches.status.completed", "已完成");
      case "draft":
        return t("pages.transfers.batches.status.draft", "草稿");
      case "pending":
        return t("pages.transfers.batches.status.pending", "待审批");
      case "approved":
        return t("pages.transfers.batches.status.approved", "已审批");
      case "processing":
        return t("pages.transfers.batches.status.processing", "处理中");
      case "partial_failed":
        return t("pages.transfers.batches.status.partialFailed", "部分失败");
      case "cancelled":
        return t("pages.transfers.batches.status.cancelled", "已取消");
      default:
        return status || "-";
    }
  };

  const columns: ProColumns<AdminTransferBatchItem>[] = [
    {
      title: t("pages.transfers.batches.columns.batch", "批次"),
      dataIndex: "batch_id",
      width: 340,
      render: (_, row) => {
        const batchId = row.batch_id || "";
        const transferType = String(
          (row as any).transfer_type || ""
        ).toLowerCase();
        const typeLabel =
          transferType === "airdrop"
            ? t("pages.transfers.batches.type.airdrop", "Airdrop")
            : transferType === "normal"
            ? t("pages.transfers.batches.type.normal", "Normal")
            : transferType || "-";
        return (
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 2,
              minWidth: 0,
            }}
          >
            <Typography.Text strong ellipsis={{ tooltip: row.name || "-" }}>
              {row.name || "-"}
            </Typography.Text>
            <Space size={10} wrap>
              <Typography.Text
                className={styles.mono}
                type="secondary"
                copyable={batchId ? { text: batchId } : false}
              >
                {batchId ? `#${batchId}` : "-"}
              </Typography.Text>
              {transferType ? (
                <Tag
                  className={styles.pillTag}
                  style={{ background: "#fef3c7", color: "#b45309" }}
                >
                  {typeLabel}
                </Tag>
              ) : null}
              {row.created_by ? (
                <Typography.Text type="secondary">
                  {t("pages.transfers.batches.columns.createdBy", "创建人")}:{" "}
                  {row.created_by}
                </Typography.Text>
              ) : null}
            </Space>
          </div>
        );
      },
    },
    {
      title: t("pages.transfers.batches.columns.currency", "币种"),
      dataIndex: "currency",
      width: 140,
      render: (_, row) => {
        const currencyRaw = String(row.currency || "").trim();
        const currencyLabel =
          currencyRaw.toUpperCase() === "MIXED"
            ? t("common.mixedCurrencies", "多币种")
            : currencyRaw;
        return (
          <Space size={8} wrap>
            {currencyRaw ? (
              <Tag
                className={styles.pillTag}
                style={{ background: "#eff6ff", color: "#3b82f6" }}
              >
                {currencyLabel}
              </Tag>
            ) : null}
          </Space>
        );
      },
    },
    {
      title: t("pages.transfers.batches.columns.createdAt", "创建时间"),
      dataIndex: "created_at",
      width: 180,
      render: (_, row) => {
        const formatted = formatLocalDateTime(row.created_at);
        if (formatted === "-") return "-";
        return (
          <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
            <CalendarOutlined style={{ color: "#9ca3af" }} />
            {formatted}
          </span>
        );
      },
    },
    {
      title: t("pages.transfers.batches.columns.status", "状态"),
      dataIndex: "status",
      width: 120,
      render: (_, row) => (
        <Tag className={`${styles.pillTag} ${getStatusStyle(row.status)}`}>
          {getStatusText(row.status)}
        </Tag>
      ),
    },
    {
      title: t("pages.transfers.batches.columns.actions", "操作"),
      valueType: "option",
      width: 260,
      fixed: "right",
      render: (_, row) => {
        const batchId = row.batch_id;
        const status = row.status;
        return (
          <Space size={8}>
            <Button
              type="link"
              size="small"
              className={styles.actionBtn}
              icon={<EyeOutlined />}
              disabled={!canRpc(PERM.detail)}
              onClick={() => {
                if (batchId) history.push(`/transfers/batches/${batchId}`);
              }}
            >
              {t("pages.transfers.batches.actions.view", "查看")}
            </Button>
            {status === "draft" ? (
              <Button
                type="link"
                size="small"
                className={styles.actionBtn}
                icon={<SendOutlined />}
                style={{ color: "#3b82f6" }}
                disabled={!batchId || !canRpc(PERM.submit)}
                onClick={() => setBatchAction({ type: "submit", row })}
              >
                {t("pages.transfers.batches.actions.submit", "提交")}
              </Button>
            ) : null}
            {status === "pending" ? (
              <>
                <Button
                  type="link"
                  size="small"
                  className={styles.actionBtn}
                  icon={<CheckCircleOutlined />}
                  style={{ color: "#10b981" }}
                  disabled={!batchId || !canRpc(PERM.approve)}
                  onClick={() => setBatchAction({ type: "approve", row })}
                >
                  {t("pages.transfers.batches.actions.approve", "审批")}
                </Button>
                <Button
                  type="link"
                  size="small"
                  className={styles.actionBtn}
                  icon={<StopOutlined />}
                  danger
                  disabled={!batchId || !canRpc(PERM.cancel)}
                  onClick={() => setBatchAction({ type: "cancel", row })}
                >
                  {t("pages.transfers.batches.actions.cancel", "取消")}
                </Button>
              </>
            ) : null}
            {status === "approved" ? (
              <>
                <Button
                  type="link"
                  size="small"
                  className={styles.actionBtn}
                  icon={<SendOutlined />}
                  style={{ color: "#3b82f6" }}
                  disabled={!batchId || !canRpc(PERM.execute)}
                  onClick={() => setBatchAction({ type: "execute", row })}
                >
                  {t("pages.transfers.batches.actions.execute", "执行")}
                </Button>
                <Button
                  type="link"
                  size="small"
                  className={styles.actionBtn}
                  icon={<StopOutlined />}
                  danger
                  disabled={!batchId || !canRpc(PERM.cancel)}
                  onClick={() => setBatchAction({ type: "cancel", row })}
                >
                  {t("pages.transfers.batches.actions.cancel", "取消")}
                </Button>
              </>
            ) : null}
          </Space>
        );
      },
    },
  ];

  const applyFilters = () => {
    setFilters(draftFilters);
  };

  const resetFilters = () => {
    const next = getInitialFilters();
    setDraftFilters(next);
    setFilters(next);
  };

  const statusOptions = [
    {
      value: "draft",
      label: t("pages.transfers.batches.status.draft", "草稿"),
    },
    {
      value: "pending",
      label: t("pages.transfers.batches.status.pending", "待审批"),
    },
    {
      value: "approved",
      label: t("pages.transfers.batches.status.approved", "已审批"),
    },
    {
      value: "processing",
      label: t("pages.transfers.batches.status.processing", "处理中"),
    },
    {
      value: "completed",
      label: t("pages.transfers.batches.status.completed", "已完成"),
    },
    {
      value: "partial_failed",
      label: t("pages.transfers.batches.status.partialFailed", "部分失败"),
    },
    {
      value: "cancelled",
      label: t("pages.transfers.batches.status.cancelled", "已取消"),
    },
  ];

  const currencyOptions = suggestedCurrencies.map((x) => ({
    value: x,
    label: x,
  }));

  const didMountRef = useRef(false);
  useEffect(() => {
    if (!didMountRef.current) {
      didMountRef.current = true;
      return;
    }
    actionRef.current?.reloadAndRest?.();
  }, [filters]);

  return (
    <PageContainer header={{ title: null, breadcrumb: {} }}>
      <div className={styles.container}>
        {/* 页面头部 */}
        <div className={styles.header}>
          <div className={styles.headerLeft}>
            <div className={styles.title}>
              {t("pages.transfers.batches.title", "账本转账管理")}
            </div>
            <div className={styles.subtitle}>
              {t(
                "pages.transfers.batches.subtitle",
                "管理批量账本转账批次（内部余额变更），支持 Excel 导入和手动添加"
              )}
            </div>
          </div>
          <Space size={12} wrap>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              size="large"
              disabled={!canRpc(PERM.import)}
              onClick={() => setCreateModalOpen(true)}
            >
              {t("pages.transfers.batches.actions.create", "创建转账批次")}
            </Button>
          </Space>
        </div>

        {/* 统计卡片 */}
        <div className={styles.statsRow}>
          {/* 处理中 */}
          <div className={styles.statsCard}>
            <div className={styles.statsLeft}>
              <div className={`${styles.statsIcon} ${styles.statsIconOrange}`}>
                <ClockCircleOutlined />
              </div>
              <div className={styles.statsContent}>
                <span className={styles.statsLabel}>
                  {t(
                    "pages.transfers.batches.stats.processingBatches",
                    "处理中批次"
                  )}
                </span>
                <span className={styles.statsSublabel}>
                  {t(
                    "pages.transfers.batches.stats.processingHint",
                    "等待账本入账处理"
                  )}
                </span>
              </div>
            </div>
            <div className={styles.statsValue}>
              <span
                className={`${styles.statsNumber} ${styles.statsNumberOrange}`}
              >
                {stats.processingBatches}
              </span>
              <span className={styles.statsUnit}>
                {t("pages.transfers.batches.unit.batch", "批")}
              </span>
              <div className={styles.statsExtra}>
                {t(
                  "pages.transfers.batches.stats.note",
                  "可在历史记录查看明细"
                )}
              </div>
            </div>
          </div>

          {/* 累计转账金额 */}
          <div className={styles.statsCard}>
            <div className={styles.statsLeft}>
              <div className={`${styles.statsIcon} ${styles.statsIconGreen}`}>
                <DollarOutlined />
              </div>
              <div className={styles.statsContent}>
                <span className={styles.statsLabel}>
                  {t(
                    "pages.transfers.batches.stats.totalAmount",
                    "累计入账金额"
                  )}
                </span>
                <span className={styles.statsSublabel}>
                  {t("pages.transfers.batches.stats.usdEquivalent", "折合 USD")}
                </span>
              </div>
            </div>
            <div className={styles.statsValue}>
              <span
                className={`${styles.statsNumber} ${styles.statsNumberGreen}`}
              >
                ${stats.totalAmountUsd}
              </span>
              <div className={styles.statsExtra}>
                {t("pages.transfers.batches.stats.amountHint", "后台汇总金额")}
              </div>
            </div>
          </div>
        </div>

        {/* 筛选栏 */}
        <div className={styles.filterCard}>
          <div className={styles.filterGrid}>
            <div>
              <div className={styles.fieldLabel}>
                {t("pages.transfers.batches.filters.keyword", "关键词")}
              </div>
              <Input
                size="small"
                className={styles.input}
                placeholder={t(
                  "pages.transfers.batches.search.placeholder",
                  "搜索批次ID或批次名称..."
                )}
                prefix={<SearchOutlined style={{ color: "#9ca3af" }} />}
                value={draftFilters.keyword}
                onChange={(e) =>
                  setDraftFilters((prev) => ({
                    ...prev,
                    keyword: e.target.value,
                  }))
                }
                onPressEnter={applyFilters}
                allowClear
              />
            </div>
            <div>
              <div className={styles.fieldLabel}>
                {t("pages.transfers.batches.filters.status", "状态")}
              </div>
              <Select
                size="small"
                className={styles.select}
                placeholder={t("common.all", "全部")}
                allowClear
                value={draftFilters.status}
                onChange={(v) =>
                  setDraftFilters((prev) => ({ ...prev, status: v }))
                }
                options={statusOptions}
              />
            </div>
            <div>
              <div className={styles.fieldLabel}>
                {t("pages.transfers.batches.filters.currency", "币种")}
              </div>
              <AutoComplete
                size="small"
                className={styles.select}
                placeholder={t("common.all", "全部")}
                allowClear
                value={draftFilters.currency}
                onChange={(v) =>
                  setDraftFilters((prev) => ({
                    ...prev,
                    currency: v || undefined,
                  }))
                }
                options={currencyOptions}
              />
            </div>
            <div>
              <div className={styles.fieldLabel}>
                {t("pages.transfers.batches.filters.date", "日期")}
              </div>
              <RangePicker
                size="small"
                className={styles.dateRangePicker}
                value={draftFilters.dateRange as any}
                onChange={(v) =>
                  setDraftFilters((prev) => ({
                    ...prev,
                    dateRange: (v as any) ?? null,
                  }))
                }
                format="YYYY-MM-DD"
                placeholder={[
                  t("pages.transfers.batches.filters.dateFrom", "开始日期"),
                  t("pages.transfers.batches.filters.dateTo", "结束日期"),
                ]}
              />
            </div>
          </div>

          <div className={styles.filterActions}>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={resetFilters}
            >
              {t("common.reset", "重置")}
            </Button>
            <Button
              size="small"
              type="primary"
              icon={<SearchOutlined />}
              onClick={applyFilters}
              disabled={!canRpc(PERM.list)}
            >
              {t("common.search", "查询")}
            </Button>
          </div>
        </div>

        {/* 数据表格 */}
        <div className={styles.tableCard}>
          <ProTable<AdminTransferBatchItem>
            actionRef={actionRef}
            rowKey={(row) => row.batch_id || row.name || JSON.stringify(row)}
            columns={columns}
            request={async (params) => {
              if (!canRpc(PERM.list))
                return { success: true, data: [], total: 0 };
              const dateFrom = filters.dateRange?.[0];
              const dateTo = filters.dateRange?.[1];
              try {
                const res = await adminListTransferBatches(
                  {
                    page: params.current,
                    page_size: params.pageSize,
                    keyword: filters.keyword.trim() || undefined,
                    status: filters.status || undefined,
                    currency: filters.currency?.trim() || undefined,
                    date_from: dateFrom
                      ? dateFrom.format("YYYY-MM-DD")
                      : undefined,
                    date_to: dateTo ? dateTo.format("YYYY-MM-DD") : undefined,
                  },
                  { skipErrorHandler: true }
                );
                setSummary(res.data?.summary ?? null);

                const batches = res.data?.batches ?? [];
                setSuggestedCurrencies(
                  Array.from(
                    new Set(
                      batches.map((b) => b.currency).filter(Boolean) as string[]
                    )
                  ).sort()
                );

                return {
                  success: !!res?.success,
                  data: batches,
                  total: Number(res.data?.pagination?.total ?? 0),
                };
              } catch (e: any) {
                message.error(
                  e?.message ||
                    t(
                      "pages.transfers.batches.messages.loadFailed",
                      "Failed to load transfer batches"
                    )
                );
                return { success: false, data: [], total: 0 };
              }
            }}
            search={false}
            options={false}
            scroll={{ x: 1100 }}
            pagination={{
              showSizeChanger: true,
              showQuickJumper: true,
            }}
          />
        </div>
      </div>

      {/* Submit Modal */}
      <ModalForm<AdminSubmitTransferBatchBody>
        title={t("pages.transfers.batches.submit.title", "Submit Batch")}
        open={batchAction?.type === "submit"}
        onOpenChange={(open) => {
          if (!open) setBatchAction(null);
        }}
        onFinish={async (values) => {
          const batchId = batchAction?.row.batch_id;
          if (!batchId) return false;
          try {
            const res = await adminSubmitTransferBatch({ batchId }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.transfers.batches.messages.submitFailed",
                    "Submit failed"
                  )
              );
            message.success(
              t("pages.transfers.batches.messages.submitted", "Submitted")
            );
            setBatchAction(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.transfers.batches.messages.submitFailed",
                  "Submit failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="note"
          label={t("pages.transfers.batches.form.note", "Note")}
          fieldProps={{ rows: 3 }}
        />
      </ModalForm>

      {/* Approve Modal */}
      <ModalForm<AdminApproveTransferBatchBody>
        title={t("pages.transfers.batches.approve.title", "Approve Batch")}
        open={batchAction?.type === "approve"}
        onOpenChange={(open) => {
          if (!open) setBatchAction(null);
        }}
        initialValues={{ action: "approve" }}
        onFinish={async (values) => {
          const batchId = batchAction?.row.batch_id;
          if (!batchId) return false;
          try {
            const res = await adminApproveTransferBatch({ batchId }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.transfers.batches.messages.approveFailed",
                    "Approve failed"
                  )
              );
            message.success(
              t("pages.transfers.batches.messages.approved", "Approved")
            );
            setBatchAction(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.transfers.batches.messages.approveFailed",
                  "Approve failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormSelect
          name="action"
          label={t("pages.transfers.batches.form.action", "Action")}
          options={[
            {
              label: t("pages.transfers.batches.actions.approve", "Approve"),
              value: "approve",
            },
            {
              label: t("pages.transfers.batches.actions.reject", "Reject"),
              value: "reject",
            },
          ]}
        />
        <ProFormTextArea
          name="note"
          label={t("pages.transfers.batches.form.note", "Note")}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText
          name="two_fa_code"
          label={t("pages.transfers.batches.form.twoFaCode", "2FA Code")}
          rules={[
            {
              required: true,
              message: t("common.required", "Please enter 2FA Code"),
            },
          ]}
        />
      </ModalForm>

      {/* Execute Modal */}
      <ModalForm<AdminExecuteTransferBatchBody>
        title={t("pages.transfers.batches.execute.title", "Execute Batch")}
        open={batchAction?.type === "execute"}
        onOpenChange={(open) => {
          if (!open) setBatchAction(null);
        }}
        onFinish={async (values) => {
          const batchId = batchAction?.row.batch_id;
          if (!batchId) return false;
          try {
            const res = await adminExecuteTransferBatch({ batchId }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.transfers.batches.messages.executeFailed",
                    "Execute failed"
                  )
              );
            message.success(
              t(
                "pages.transfers.batches.messages.executed",
                "Execution started"
              )
            );
            setBatchAction(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.transfers.batches.messages.executeFailed",
                  "Execute failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="note"
          label={t("pages.transfers.batches.form.note", "Note")}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText
          name="two_fa_code"
          label={t("pages.transfers.batches.form.twoFaCode", "2FA Code")}
          rules={[
            {
              required: true,
              message: t("common.required", "Please enter 2FA Code"),
            },
            {
              pattern: /^\d{6}$/,
              message: t("common.invalid2fa", "2FA Code must be 6 digits"),
            },
          ]}
        />
      </ModalForm>

      {/* Cancel Modal */}
      <ModalForm<AdminCancelTransferBatchBody>
        title={t("pages.transfers.batches.cancel.title", "Cancel Batch")}
        open={batchAction?.type === "cancel"}
        onOpenChange={(open) => {
          if (!open) setBatchAction(null);
        }}
        submitter={{ submitButtonProps: { danger: true } }}
        onFinish={async (values) => {
          const batchId = batchAction?.row.batch_id;
          if (!batchId) return false;
          try {
            const res = await adminCancelTransferBatch({ batchId }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.transfers.batches.messages.cancelFailed",
                    "Cancel failed"
                  )
              );
            message.success(
              t("pages.transfers.batches.messages.cancelled", "Cancelled")
            );
            setBatchAction(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.transfers.batches.messages.cancelFailed",
                  "Cancel failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="reason"
          label={t("pages.transfers.batches.form.reason", "Reason")}
          rules={[{ required: true }]}
          fieldProps={{ rows: 3 }}
        />
      </ModalForm>

      {/* Create Batch Modal */}
      <CreateBatchModal
        open={createModalOpen}
        onOpenChange={setCreateModalOpen}
        canImport={canRpc(PERM.import)}
        onCreated={(batchId) => {
          actionRef.current?.reload();
          if (batchId) history.push(`/transfers/batches/${batchId}`);
        }}
        t={t}
      />
    </PageContainer>
  );
};

export default TransferBatchesPage;
