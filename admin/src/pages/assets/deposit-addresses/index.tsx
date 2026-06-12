import {
  CheckCircleOutlined,
  CopyOutlined,
  DownOutlined,
  FileTextOutlined,
  FilterOutlined,
  LineChartOutlined,
  MoreOutlined,
  PauseCircleOutlined,
  PieChartOutlined,
  RightOutlined,
  SearchOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-components";
import { useIntl } from "@umijs/max";
import {
  App,
  Button,
  Dropdown,
  Empty,
  Input,
  InputNumber,
  Modal,
  Pagination,
  Select,
  Slider,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
} from "antd";
import { createStyles } from "antd-style";
import type { ColumnsType } from "antd/es/table";
import * as echarts from "echarts";
import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { adminUpdateDepositAddress } from "@/api/generated/assets";
import type {
  AdminBatchCreateAndExecuteSweepData,
  AdminBatchSweepItem,
  AdminDepositAddressBalanceItem,
  AdminPagination,
  AdminSweepTaskItem,
  GatewayPayloadAdminGetDepositAddressBalanceStatsResponse,
} from "@/api/generated/schemas";
import {
  adminBatchCreateAndExecuteSweep,
  adminCreateAndExecuteSweep,
  adminGetDepositAddressBalanceByAsset,
  adminGetDepositAddressBalanceStats,
  adminGetSweepTaskStatsByDate,
  adminListDepositAddressBalances,
  adminListSweepTasks,
} from "@/api/generated/sub-addresses";
import { useRbac } from "@/hooks/useRbac";

const { Text } = Typography;

const MAX_BALANCE_USD = 100000;
const BALANCE_RANGE_DEBOUNCE_MS = 400;
const FILTER_APPLY_DEBOUNCE_MS = 300;
const TREND_DAYS = 7;

const currencyOptions = [
  { value: "USDT", label: "USDT", color: "#26A17B" },
  { value: "USDC", label: "USDC", color: "#2775CA" },
  { value: "ETH", label: "ETH", color: "#627EEA" },
  { value: "TRX", label: "TRX", color: "#FF0013" },
  { value: "BNB", label: "BNB", color: "#F3BA2F" },
];

const chainOptions = [
  { value: "all", label: "全部" },
  { value: "ETH", label: "ETH" },
  { value: "TRON", label: "TRON" },
  { value: "BSC", label: "BSC" },
];

const syncStatusOptions = [
  { value: "all", label: "全部" },
  { value: "unknown", label: "未知" },
  { value: "synced", label: "已同步" },
  { value: "syncing", label: "同步中" },
  { value: "error", label: "异常" },
];

const needsSweepOptions = [
  { value: "all", label: "全部" },
  { value: "true", label: "待归集" },
  { value: "false", label: "无需归集" },
];

const PERM = {
  listBalances: "ListDepositAddressBalances",
  stats: "GetDepositAddressBalanceStats",
  listTasks: "ListSweepTasks",
  sweepOneClick: "CreateAndExecuteSweep",
  sweepBatch: "BatchCreateAndExecuteSweep",
  updateAddress: "UpdateDepositAddress",
};

const useStyles = createStyles(() => ({
  page: { position: "relative" },
  header: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "flex-start",
    marginBottom: 24,
  },
  headerLeft: { display: "flex", flexDirection: "column", gap: 4 },
  title: { fontSize: 24, fontWeight: 700, color: "#1f2937", margin: 0 },
  subtitle: { fontSize: 14, color: "#6b7280" },
  headerRight: { display: "flex", alignItems: "center", gap: 12 },
  refreshBtn: { display: "flex", alignItems: "center", gap: 6 },
  searchInput: { width: 240, borderRadius: 8 },

  statsRow: {
    display: "grid",
    gridTemplateColumns: "repeat(4, 1fr)",
    gap: 16,
    marginBottom: 24,
    "@media (max-width: 1200px)": { gridTemplateColumns: "repeat(2, 1fr)" },
    "@media (max-width: 768px)": { gridTemplateColumns: "1fr" },
  },
  statCard: {
    borderRadius: 12,
    background: "#fff",
    border: "1px solid #f0f0f0",
    padding: "20px 24px",
    display: "flex",
    justifyContent: "space-between",
    alignItems: "flex-start",
  },
  statLeft: { display: "flex", flexDirection: "column", gap: 8 },
  statLabel: { fontSize: 14, color: "#6b7280" },
  statValue: {
    fontSize: 32,
    fontWeight: 700,
    fontFamily: '"DIN Alternate", "Bebas Neue", "Teko", sans-serif',
    color: "#1f2937",
  },
  statSub: { fontSize: 13, color: "#10b981" },
  statSubGray: { color: "#6b7280" },
  statIcon: {
    width: 44,
    height: 44,
    borderRadius: 12,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    fontSize: 22,
  },
  statIconBlue: { background: "#eff6ff", color: "#3b82f6" },
  statIconGreen: { background: "#ecfdf5", color: "#10b981" },
  statIconPurple: { background: "#faf5ff", color: "#a855f7" },
  statIconOrange: { background: "#fff7ed", color: "#f97316" },

  visualSection: { marginBottom: 24 },
  visualHeader: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    padding: "12px 0",
    cursor: "pointer",
    color: "#6b7280",
    fontSize: 14,
    "&:hover": { color: "#4b5563" },
  },
  visualHeaderLeft: { display: "flex", alignItems: "center", gap: 8 },
  visualTools: { display: "flex", alignItems: "center", gap: 10 },
  visualContent: {
    display: "grid",
    gridTemplateColumns: "repeat(2, 1fr)",
    gap: 16,
    "@media (max-width: 992px)": { gridTemplateColumns: "1fr" },
  },
  chartCard: {
    borderRadius: 12,
    background: "#fff",
    border: "1px solid #f0f0f0",
    padding: 24,
  },
  chartTitleRow: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 12,
    marginBottom: 12,
  },
  chartTitle: {
    display: "flex",
    alignItems: "center",
    gap: 8,
    fontSize: 16,
    fontWeight: 600,
    color: "#1f2937",
  },
  chartPlaceholder: {
    height: 420,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    color: "#9ca3af",
    fontSize: 14,
    background: "#fafafa",
    borderRadius: 8,
  },

  tabsContainer: { marginBottom: 24 },

  mainContent: { display: "flex", gap: 24 },
  filterSidebar: {
    width: 280,
    flexShrink: 0,
    borderRadius: 12,
    background: "#fff",
    border: "1px solid #f0f0f0",
    padding: 20,
  },
  filterHeader: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    marginBottom: 20,
    cursor: "pointer",
  },
  filterTitle: {
    display: "flex",
    alignItems: "center",
    gap: 8,
    fontSize: 15,
    fontWeight: 600,
    color: "#4b5563",
  },
  filterSection: { marginBottom: 24 },
  filterSectionTitle: {
    fontSize: 14,
    fontWeight: 500,
    color: "#374151",
    marginBottom: 12,
  },
  sliderContainer: { padding: "0 8px" },
  sliderInputs: {
    display: "flex",
    alignItems: "center",
    gap: 8,
    marginTop: 12,
  },
  sliderInput: { flex: 1, textAlign: "center" },
  collectAllBtn: {
    width: "100%",
    height: 44,
    borderRadius: 10,
    fontWeight: 500,
    background: "linear-gradient(135deg, #3b82f6 0%, #60a5fa 100%)",
    border: "none",
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
  },
  resetBtn: {
    width: "100%",
    height: 44,
    borderRadius: 10,
    fontWeight: 500,
  },

  tableSection: { flex: 1, minWidth: 0 },
  tableHeader: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: 16,
  },
  tableHeaderLeft: { display: "flex", alignItems: "center", gap: 16 },
  recordCount: { fontSize: 14, color: "#6b7280" },
  tableCard: {
    borderRadius: 12,
    background: "#fff",
    border: "1px solid #f0f0f0",
    overflow: "hidden",
  },

  addressCell: { display: "flex", alignItems: "center", gap: 8 },
  addressIcon: {
    width: 28,
    height: 28,
    borderRadius: 14,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    fontSize: 12,
    fontWeight: 600,
    color: "#fff",
  },
  addressText: { fontFamily: "monospace", fontSize: 14, color: "#1f2937" },
  copyBtn: { color: "#9ca3af", "&:hover": { color: "#6b7280" } },

  balanceCell: { display: "flex", flexDirection: "column", gap: 2 },
  balanceValue: { fontSize: 14, fontWeight: 600, color: "#3b82f6" },
  balanceUsdt: { fontSize: 12, color: "#9ca3af" },

  statusTag: { borderRadius: 6, border: "none", fontWeight: 500, fontSize: 12 },
  statusActive: { background: "#dcfce7", color: "#16a34a" },
  statusFrozen: { background: "#fef3c7", color: "#d97706" },
  statusIdle: { background: "#f3f4f6", color: "#6b7280" },
  statusError: { background: "#fee2e2", color: "#dc2626" },

  collectBtn: {
    borderRadius: 8,
    fontWeight: 500,
    background: "#10b981",
    borderColor: "#10b981",
    "&:hover": {
      background: "#059669 !important",
      borderColor: "#059669 !important",
    },
  },
  moreBtn: { color: "#9ca3af" },

  emptyIcon: { fontSize: 64, color: "#d1d5db", marginBottom: 16 },
  emptyText: { fontSize: 14, color: "#9ca3af" },

  taskRecordsCard: {
    borderRadius: 12,
    background: "#fff",
    border: "1px solid #f0f0f0",
    padding: 24,
  },
  taskRecordsHeader: { marginBottom: 24 },
  taskRecordsTitle: {
    fontSize: 16,
    fontWeight: 600,
    color: "#1f2937",
    marginBottom: 4,
  },
  taskRecordsSub: { fontSize: 13, color: "#9ca3af" },

  collectModalTitle: { display: "flex", alignItems: "center", gap: 10 },
  collectModalIcon: {
    width: 28,
    height: 28,
    borderRadius: 14,
    background: "#dcfce7",
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    color: "#16a34a",
    fontSize: 16,
  },
  collectModalTitleText: { fontSize: 18, fontWeight: 600, color: "#1f2937" },
  collectModalSubtitle: {
    fontSize: 14,
    color: "#6b7280",
    marginTop: -8,
    marginBottom: 20,
  },
  collectModalCard: {
    background: "#f9fafb",
    borderRadius: 12,
    padding: "20px 24px",
    marginBottom: 20,
  },
  collectModalRow: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    padding: "12px 0",
    borderBottom: "1px solid #e5e7eb",
    "&:last-child": { borderBottom: "none" },
  },
  collectModalLabel: { fontSize: 14, color: "#6b7280" },
  collectModalValue: { display: "flex", alignItems: "center", gap: 8 },
  collectModalAddressIcon: {
    width: 24,
    height: 24,
    borderRadius: 12,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    fontSize: 10,
    fontWeight: 700,
    color: "#fff",
  },
  collectModalAddress: {
    fontSize: 14,
    fontFamily: "monospace",
    color: "#1f2937",
  },
  collectModalAmount: { fontSize: 16, fontWeight: 600, color: "#10b981" },
  collectModalUsdt: { fontSize: 16, fontWeight: 600, color: "#1f2937" },
  collectModalFooter: {
    textAlign: "center",
    fontSize: 13,
    color: "#9ca3af",
    marginBottom: 8,
  },
}));

const EChart: React.FC<{
  option: echarts.EChartsOption;
  loading?: boolean;
  height?: number;
}> = ({ option, loading, height = 420 }) => {
  const divRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<echarts.EChartsType | null>(null);

  useEffect(() => {
    if (!divRef.current) return;
    const chart = echarts.init(divRef.current);
    chartRef.current = chart;
    return () => {
      chart.dispose();
      chartRef.current = null;
    };
  }, []);

  useEffect(() => {
    const chart = chartRef.current;
    if (!chart) return;
    chart.setOption(option, { notMerge: true, lazyUpdate: true });
  }, [option]);

  useEffect(() => {
    const el = divRef.current;
    const chart = chartRef.current;
    if (!el || !chart) return;
    const ro = new ResizeObserver(() => chart.resize());
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    const chart = chartRef.current;
    if (!chart) return;
    if (loading) chart.showLoading("default");
    else chart.hideLoading();
  }, [loading]);

  return <div ref={divRef} style={{ width: "100%", height }} />;
};

const SubAddressesPage: React.FC = () => {
  const intl = useIntl();
  const { styles, cx } = useStyles();
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();

  const [activeTab, setActiveTab] = useState<"addresses" | "tasks">(
    "addresses"
  );
  const [visualExpanded, setVisualExpanded] = useState(false);
  const [filterExpanded, setFilterExpanded] = useState(true);

  const [assetCodeDraft, setAssetCodeDraft] = useState<string>("all");
  const [chainCodeDraft, setChainCodeDraft] = useState<string>("all");
  const [syncStatusDraft, setSyncStatusDraft] = useState<string>("all");
  const [needsSweepDraft, setNeedsSweepDraft] = useState<string>("all");
  const [balanceRangeUsdDraft, setBalanceRangeUsdDraft] = useState<
    [number, number]
  >([0, MAX_BALANCE_USD]);
  const balanceRangeUsdDraftSourceRef = useRef<null | "input" | "slider">(null);

  const [assetCodeApplied, setAssetCodeApplied] = useState<string>("all");
  const [chainCodeApplied, setChainCodeApplied] = useState<string>("all");
  const [syncStatusApplied, setSyncStatusApplied] = useState<string>("all");
  const [needsSweepApplied, setNeedsSweepApplied] = useState<string>("all");
  const [balanceRangeUsdApplied, setBalanceRangeUsdApplied] = useState<
    [number, number]
  >([0, MAX_BALANCE_USD]);

  const [searchText, setSearchText] = useState<string>("");
  const [userIdFilter, setUserIdFilter] = useState<string | undefined>(
    undefined
  );

  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);

  const [statsLoading, setStatsLoading] = useState(false);
  const [stats, setStats] =
    useState<GatewayPayloadAdminGetDepositAddressBalanceStatsResponse | null>(
      null
    );

  const [balancesLoading, setBalancesLoading] = useState(false);
  const [balances, setBalances] = useState<AdminDepositAddressBalanceItem[]>(
    []
  );
  const [balancesPagination, setBalancesPagination] =
    useState<AdminPagination | null>(null);

  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [selectedRows, setSelectedRows] = useState<
    AdminDepositAddressBalanceItem[]
  >([]);

  const [tasksLoading, setTasksLoading] = useState(false);
  const [tasks, setTasks] = useState<AdminSweepTaskItem[]>([]);
  const [tasksPagination, setTasksPagination] =
    useState<AdminPagination | null>(null);
  const [tasksPage, setTasksPage] = useState(1);
  const [tasksPageSize, setTasksPageSize] = useState(20);

  const [collectModalVisible, setCollectModalVisible] = useState(false);
  const [collectResult, setCollectResult] = useState<{
    balance: AdminDepositAddressBalanceItem;
    taskNo: string;
    txHash: string;
  } | null>(null);

  const [sweepSubmitting, setSweepSubmitting] = useState(false);

  const [assetDistLoading, setAssetDistLoading] = useState(false);
  const [assetDist, setAssetDist] = useState<any>(null);

  const [trendLoading, setTrendLoading] = useState(false);
  const [trend, setTrend] = useState<any>(null);

  const toNumber = (v: unknown): number => {
    if (v === null || v === undefined) return 0;
    const n = Number(v);
    return Number.isFinite(n) ? n : 0;
  };

  const t = useCallback(
    (id: string, defaultMessage: string, values?: Record<string, any>) =>
      intl.formatMessage({ id, defaultMessage }, values),
    [intl]
  );

  const isPositiveIntString = (s: string) => /^[0-9]+$/.test(s) && s !== "0";

  const normalizeBalanceRangeUsd = useCallback(
    (range: [number, number]): [number, number] => {
      const clamp = (v: number) =>
        Math.min(MAX_BALANCE_USD, Math.max(0, Math.round(v)));
      let [minUsd, maxUsd] = range;
      minUsd = clamp(Number.isFinite(minUsd) ? minUsd : 0);
      maxUsd = clamp(Number.isFinite(maxUsd) ? maxUsd : MAX_BALANCE_USD);
      if (minUsd > maxUsd) [minUsd, maxUsd] = [maxUsd, minUsd];
      return [minUsd, maxUsd];
    },
    []
  );

  useEffect(() => {
    if (balanceRangeUsdDraftSourceRef.current !== "input") return;
    const normalized = normalizeBalanceRangeUsd(balanceRangeUsdDraft);
    if (
      normalized[0] === balanceRangeUsdDraft[0] &&
      normalized[1] === balanceRangeUsdDraft[1]
    )
      return;

    const timer = window.setTimeout(() => {
      if (balanceRangeUsdDraftSourceRef.current !== "input") return;
      balanceRangeUsdDraftSourceRef.current = null;
      setBalanceRangeUsdDraft(normalized);
    }, BALANCE_RANGE_DEBOUNCE_MS);

    return () => window.clearTimeout(timer);
  }, [balanceRangeUsdDraft, normalizeBalanceRangeUsd]);

  const isSameRange = (a: [number, number], b: [number, number]) =>
    a[0] === b[0] && a[1] === b[1];

  const filterAutoApplyFirstRunRef = useRef(true);
  const filterAutoApplyTimerRef = useRef<number | null>(null);
  const lastAppliedSignatureRef = useRef<string>("");

  useEffect(() => {
    if (filterAutoApplyFirstRunRef.current) {
      filterAutoApplyFirstRunRef.current = false;
      return;
    }

    if (filterAutoApplyTimerRef.current)
      window.clearTimeout(filterAutoApplyTimerRef.current);

    filterAutoApplyTimerRef.current = window.setTimeout(() => {
      const normalizedRange = normalizeBalanceRangeUsd(balanceRangeUsdDraft);

      const signature = JSON.stringify({
        a: assetCodeDraft,
        c: chainCodeDraft,
        s: syncStatusDraft,
        n: needsSweepDraft,
        r0: normalizedRange[0],
        r1: normalizedRange[1],
      });

      if (signature === lastAppliedSignatureRef.current) return;
      lastAppliedSignatureRef.current = signature;

      const needResetPage = currentPage !== 1;
      const needResetTaskPage = tasksPage !== 1;

      if (assetCodeApplied !== assetCodeDraft)
        setAssetCodeApplied(assetCodeDraft);
      if (chainCodeApplied !== chainCodeDraft)
        setChainCodeApplied(chainCodeDraft);
      if (syncStatusApplied !== syncStatusDraft)
        setSyncStatusApplied(syncStatusDraft);
      if (needsSweepApplied !== needsSweepDraft)
        setNeedsSweepApplied(needsSweepDraft);
      if (!isSameRange(balanceRangeUsdApplied, normalizedRange))
        setBalanceRangeUsdApplied(normalizedRange);

      if (needResetPage) setCurrentPage(1);
      if (needResetTaskPage) setTasksPage(1);

      setSelectedRowKeys([]);
      setSelectedRows([]);
    }, FILTER_APPLY_DEBOUNCE_MS);

    return () => {
      if (filterAutoApplyTimerRef.current)
        window.clearTimeout(filterAutoApplyTimerRef.current);
    };
  }, [
    assetCodeDraft,
    chainCodeDraft,
    syncStatusDraft,
    needsSweepDraft,
    balanceRangeUsdDraft,
    normalizeBalanceRangeUsd,
    assetCodeApplied,
    chainCodeApplied,
    syncStatusApplied,
    needsSweepApplied,
    balanceRangeUsdApplied,
    currentPage,
    tasksPage,
  ]);

  const buildStatsParams = useCallback(
    (p: { asset: string; chain: string; userId?: string }) => {
      const params: Record<string, any> = {};
      if (p.userId) params.user_id = p.userId;
      if (p.asset !== "all") params.asset_code = p.asset;
      if (p.chain !== "all") params.chain_code = p.chain;
      return params;
    },
    []
  );

  const buildListParams = useCallback(
    (p: {
      page: number;
      pageSize: number;
      asset: string;
      chain: string;
      sync: string;
      needs: string;
      range: [number, number];
      userId?: string;
    }) => {
      const params: Record<string, any> = {
        page: p.page,
        page_size: p.pageSize,
        sort_by: "balance_usd_raw",
        sort_order: "desc",
      };
      if (p.userId) params.user_id = p.userId;
      if (p.asset !== "all") params.asset_code = p.asset;
      if (p.chain !== "all") params.chain_code = p.chain;
      if (p.sync !== "all") params.sync_status = p.sync;
      if (p.needs !== "all") params.needs_sweep = p.needs === "true";
      const [minUsd, maxUsd] = p.range;
      if (minUsd > 0)
        params.min_balance_usd_raw = String(Math.floor(minUsd * 100));
      if (maxUsd < MAX_BALANCE_USD)
        params.max_balance_usd_raw = String(Math.ceil(maxUsd * 100));
      return params;
    },
    []
  );

  const buildTasksParams = useCallback(
    (p: {
      page: number;
      pageSize: number;
      asset: string;
      chain: string;
      userId?: string;
    }) => {
      const params: Record<string, any> = {
        page: p.page,
        page_size: p.pageSize,
      };
      if (p.userId) params.user_id = p.userId;
      if (p.asset !== "all") params.asset_code = p.asset;
      if (p.chain !== "all") params.chain_code = p.chain;
      return params;
    },
    []
  );

  const fetchStats = useCallback(async () => {
    if (!canRpc(PERM.stats)) {
      setStats(null);
      return;
    }
    setStatsLoading(true);
    try {
      const params = buildStatsParams({
        asset: assetCodeApplied,
        chain: chainCodeApplied,
        userId: userIdFilter,
      });
      const res = await adminGetDepositAddressBalanceStats(params, {
        skipErrorHandler: true,
      });
      if (!res?.success) throw new Error(res?.message || "");
      setStats(res?.data || null);
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.vault.subAddresses.messages.loadFailed", "加载充值地址失败")
      );
      setStats(null);
    } finally {
      setStatsLoading(false);
    }
  }, [
    assetCodeApplied,
    buildStatsParams,
    canRpc,
    chainCodeApplied,
    message,
    t,
    userIdFilter,
  ]);

  const fetchBalances = useCallback(async () => {
    if (!canRpc(PERM.listBalances)) {
      setBalances([]);
      setBalancesPagination(null);
      return;
    }
    setBalancesLoading(true);
    try {
      const params = buildListParams({
        page: currentPage,
        pageSize,
        asset: assetCodeApplied,
        chain: chainCodeApplied,
        sync: syncStatusApplied,
        needs: needsSweepApplied,
        range: balanceRangeUsdApplied,
        userId: userIdFilter,
      });
      const res = await adminListDepositAddressBalances(params, {
        skipErrorHandler: true,
      });
      if (!res?.success) throw new Error(res?.message || "");
      setBalances(res?.data?.balances || []);
      setBalancesPagination(res?.data?.pagination || null);
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.vault.subAddresses.messages.loadFailed", "加载充值地址失败")
      );
      setBalances([]);
      setBalancesPagination(null);
    } finally {
      setBalancesLoading(false);
    }
  }, [
    assetCodeApplied,
    balanceRangeUsdApplied,
    buildListParams,
    canRpc,
    chainCodeApplied,
    currentPage,
    message,
    needsSweepApplied,
    pageSize,
    syncStatusApplied,
    t,
    userIdFilter,
  ]);

  const fetchTasks = useCallback(async () => {
    if (!canRpc(PERM.listTasks)) {
      setTasks([]);
      setTasksPagination(null);
      return;
    }
    setTasksLoading(true);
    try {
      const params = buildTasksParams({
        page: tasksPage,
        pageSize: tasksPageSize,
        asset: assetCodeApplied,
        chain: chainCodeApplied,
        userId: userIdFilter,
      });
      const res = await adminListSweepTasks(params, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || "");
      setTasks(res?.data?.tasks || []);
      setTasksPagination(res?.data?.pagination || null);
    } catch (e: any) {
      message.error(
        e?.message ||
          t(
            "pages.vault.subAddresses.taskRecords.loadFailed",
            "加载归集任务失败"
          )
      );
      setTasks([]);
      setTasksPagination(null);
    } finally {
      setTasksLoading(false);
    }
  }, [
    assetCodeApplied,
    buildTasksParams,
    canRpc,
    chainCodeApplied,
    message,
    t,
    tasksPage,
    tasksPageSize,
    userIdFilter,
  ]);

  const fetchAssetDist = useCallback(async () => {
    if (!canRpc(PERM.stats)) {
      setAssetDist(null);
      return;
    }
    setAssetDistLoading(true);
    try {
      const params: Record<string, any> = {};
      if (assetCodeApplied !== "all") params.assetCode = assetCodeApplied;
      const res = await adminGetDepositAddressBalanceByAsset(params as any, {
        skipErrorHandler: true,
      });
      if (!res?.success) throw new Error(res?.message || "");
      setAssetDist(res?.data || null);
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.vault.subAddresses.messages.loadFailed", "加载充值地址失败")
      );
      setAssetDist(null);
    } finally {
      setAssetDistLoading(false);
    }
  }, [assetCodeApplied, canRpc, message, t]);

  const fetchTrend = useCallback(async () => {
    if (!canRpc(PERM.stats)) {
      setTrend(null);
      return;
    }
    setTrendLoading(true);
    try {
      const params: Record<string, any> = { days: TREND_DAYS };
      if (assetCodeApplied !== "all") params.assetCode = assetCodeApplied;
      const res = await adminGetSweepTaskStatsByDate(params as any, {
        skipErrorHandler: true,
      });
      if (!res?.success) throw new Error(res?.message || "");
      setTrend(res?.data || null);
    } catch (e: any) {
      message.error(
        e?.message ||
          t(
            "pages.vault.subAddresses.taskRecords.loadFailed",
            "加载归集任务失败"
          )
      );
      setTrend(null);
    } finally {
      setTrendLoading(false);
    }
  }, [assetCodeApplied, canRpc, message, t]);

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  useEffect(() => {
    fetchBalances();
  }, [fetchBalances]);

  useEffect(() => {
    if (activeTab !== "tasks") return;
    fetchTasks();
  }, [activeTab, fetchTasks]);

  useEffect(() => {
    if (!visualExpanded) return;
    fetchAssetDist();
    fetchTrend();
  }, [fetchAssetDist, fetchTrend, visualExpanded]);

  const refreshing =
    statsLoading ||
    balancesLoading ||
    tasksLoading ||
    assetDistLoading ||
    trendLoading;

  const handleRefresh = useCallback(async () => {
    const jobs: Promise<any>[] = [fetchStats(), fetchBalances()];
    if (activeTab === "tasks") jobs.push(fetchTasks());
    if (visualExpanded) jobs.push(fetchAssetDist(), fetchTrend());
    await Promise.all(jobs);
  }, [
    activeTab,
    fetchAssetDist,
    fetchBalances,
    fetchStats,
    fetchTasks,
    fetchTrend,
    visualExpanded,
  ]);

  const applyUserIdSearch = useCallback(async () => {
    const raw = searchText.trim();
    if (!raw) {
      setUserIdFilter(undefined);
      setCurrentPage(1);
      setTasksPage(1);
      setSelectedRowKeys([]);
      setSelectedRows([]);
      return;
    }
    if (!isPositiveIntString(raw)) {
      message.error(
        t("pages.vault.subAddresses.search.invalidUserId", "请输入正确的用户ID")
      );
      return;
    }
    setUserIdFilter(raw);
    setCurrentPage(1);
    setTasksPage(1);
    setSelectedRowKeys([]);
    setSelectedRows([]);
  }, [isPositiveIntString, message, searchText, t]);

  const handleCloseCollectModal = () => {
    setCollectModalVisible(false);
    setCollectResult(null);
  };

  const confirmFreezeAddress = useCallback(
    (depositAddressId: string | undefined) => {
      const id = (depositAddressId || "").trim();
      if (!id) {
        message.error(
          t("pages.vault.subAddresses.messages.missingAddressId", "缺少地址ID")
        );
        return;
      }
      if (!canRpc(PERM.updateAddress)) return;

      modal.confirm({
        title: t(
          "pages.vault.subAddresses.actions.confirmFreezeTitle",
          "确认冻结该地址？"
        ),
        content: t(
          "pages.vault.subAddresses.actions.confirmFreezeDesc",
          "冻结后将禁止继续使用该充值地址（状态设为 inactive）"
        ),
        okText: t("pages.vault.subAddresses.actions.confirm", "确认"),
        cancelText: t("pages.vault.subAddresses.actions.cancel", "取消"),
        onOk: async () => {
          try {
            const res = await adminUpdateDepositAddress(
              { id },
              { status: "inactive" },
              { skipErrorHandler: true }
            );
            if (!res?.success) throw new Error(res?.message || "");
            message.success(
              t("pages.vault.subAddresses.messages.freezeSuccess", "地址已冻结")
            );
            await fetchBalances();
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.vault.subAddresses.messages.freezeFailed", "冻结失败")
            );
            throw e;
          }
        },
      });
    },
    [canRpc, fetchBalances, message, modal, t]
  );

  const handleCollect = useCallback(
    async (record: AdminDepositAddressBalanceItem) => {
      if (!canRpc(PERM.sweepOneClick)) return;
      const balanceId = (record.id || "").trim();
      if (!balanceId) {
        message.error(
          t(
            "pages.vault.subAddresses.messages.missingBalanceId",
            "缺少余额记录ID"
          )
        );
        return;
      }
      setSweepSubmitting(true);
      try {
        const res = await adminCreateAndExecuteSweep(
          { balance_id: balanceId },
          { skipErrorHandler: true }
        );
        if (!res?.success) throw new Error(res?.message || "");
        setCollectResult({
          balance: record,
          taskNo: (res?.data?.task_no || "").trim(),
          txHash: (res?.data?.tx_hash || "").trim(),
        });
        setCollectModalVisible(true);
        setSelectedRowKeys([]);
        setSelectedRows([]);
        await Promise.all([
          fetchStats(),
          fetchBalances(),
          visualExpanded ? fetchAssetDist() : Promise.resolve(),
          visualExpanded ? fetchTrend() : Promise.resolve(),
        ]);
      } catch (e: any) {
        message.error(
          e?.message ||
            t("pages.vault.subAddresses.messages.collectFailed", "归集失败")
        );
      } finally {
        setSweepSubmitting(false);
      }
    },
    [
      canRpc,
      fetchAssetDist,
      fetchBalances,
      fetchStats,
      fetchTrend,
      message,
      t,
      visualExpanded,
    ]
  );

  const getCurrencyColor = (currency: string) => {
    const colors: Record<string, string> = {
      BTC: "#F7931A",
      ETH: "#627EEA",
      USDT: "#26A17B",
      USDC: "#2775CA",
    };
    return colors[currency] || "#6b7280";
  };

  const handleCollectAll = useCallback(async () => {
    if (!canRpc(PERM.sweepBatch)) return;

    const fromAddressIds = Array.from(
      new Set(
        selectedRows
          .filter((r) => !!r?.needs_sweep)
          .map((r) => (r.deposit_address_id || "").trim())
          .filter((v) => !!v)
      )
    );

    if (!fromAddressIds.length) {
      message.error(
        t(
          "pages.vault.subAddresses.messages.missingAddressId",
          "请先勾选待归集的地址"
        )
      );
      return;
    }

    setSweepSubmitting(true);
    try {
      const res = await adminBatchCreateAndExecuteSweep(
        { fromAddressIds, continueOnError: true } as any,
        { skipErrorHandler: true }
      );
      if (!res?.success) throw new Error(res?.message || "");

      const data: AdminBatchCreateAndExecuteSweepData | undefined =
        res?.data as any;
      const successCount =
        data?.success_count ?? (data as any)?.successCount ?? 0;
      const failedCount = data?.failed_count ?? (data as any)?.failedCount ?? 0;

      message.success(
        t(
          "pages.vault.subAddresses.messages.batchCollectDone",
          "批量归集完成：{success} 成功，{failed} 失败",
          {
            success: successCount,
            failed: failedCount,
          }
        )
      );

      if (failedCount > 0) {
        const results: AdminBatchSweepItem[] = (data?.results ||
          (data as any)?.results ||
          []) as any;
        const failedItems: AdminBatchSweepItem[] = results.filter(
          (it: any) => String(it?.status || "").toLowerCase() !== "success"
        );
        modal.info({
          title: t(
            "pages.vault.subAddresses.messages.batchCollectPartial",
            "部分归集失败"
          ),
          content: (
            <div style={{ paddingTop: 8 }}>
              <div style={{ marginBottom: 8 }}>
                {t(
                  "pages.vault.subAddresses.messages.batchCollectPartialDesc",
                  "请查看失败项错误信息："
                )}
              </div>
              <div style={{ maxHeight: 260, overflow: "auto" }}>
                {failedItems.map((it: any, idx: number) => (
                  <div
                    key={`${
                      it.from_address_id || it.fromAddressId || "unknown"
                    }-${idx}`}
                    style={{ marginBottom: 8 }}
                  >
                    <Text code>
                      {String(it.from_address_id || it.fromAddressId || "-")}
                    </Text>{" "}
                    <Text type="danger">
                      {String(it.error_message || it.errorMessage || "-")}
                    </Text>
                  </div>
                ))}
              </div>
            </div>
          ),
          width: 640,
        });
      }

      setSelectedRowKeys([]);
      setSelectedRows([]);
      await Promise.all([
        fetchStats(),
        fetchBalances(),
        visualExpanded ? fetchAssetDist() : Promise.resolve(),
        visualExpanded ? fetchTrend() : Promise.resolve(),
      ]);
    } catch (e: any) {
      message.error(
        e?.message ||
          t(
            "pages.vault.subAddresses.messages.batchCollectFailed",
            "批量归集失败"
          )
      );
    } finally {
      setSweepSubmitting(false);
    }
  }, [
    canRpc,
    fetchAssetDist,
    fetchBalances,
    fetchStats,
    fetchTrend,
    message,
    modal,
    selectedRows,
    t,
    visualExpanded,
  ]);

  const handleReset = useCallback(async () => {
    setAssetCodeDraft("all");
    setChainCodeDraft("all");
    setSyncStatusDraft("all");
    setNeedsSweepDraft("all");
    setBalanceRangeUsdDraft([0, MAX_BALANCE_USD]);

    setAssetCodeApplied("all");
    setChainCodeApplied("all");
    setSyncStatusApplied("all");
    setNeedsSweepApplied("all");
    setBalanceRangeUsdApplied([0, MAX_BALANCE_USD]);

    lastAppliedSignatureRef.current = JSON.stringify({
      a: "all",
      c: "all",
      s: "all",
      n: "all",
      r0: 0,
      r1: MAX_BALANCE_USD,
    });

    setCurrentPage(1);
    setTasksPage(1);
    setSelectedRowKeys([]);
    setSelectedRows([]);

    await Promise.all([
      fetchStats(),
      fetchBalances(),
      activeTab === "tasks" ? fetchTasks() : Promise.resolve(),
      visualExpanded ? fetchAssetDist() : Promise.resolve(),
      visualExpanded ? fetchTrend() : Promise.resolve(),
    ]);
  }, [
    activeTab,
    fetchAssetDist,
    fetchBalances,
    fetchStats,
    fetchTasks,
    fetchTrend,
    visualExpanded,
  ]);

  const pieOption = useMemo<echarts.EChartsOption>(() => {
    const assets: any[] = assetDist?.assets || [];
    const seriesData = assets.map((a: any) => {
      const code = String(a?.asset_code ?? a?.assetCode ?? "").trim() || "-";
      const raw = a?.total_balance_usd_raw ?? a?.totalBalanceUsdRaw;
      const usd = a?.total_balance_usd ?? a?.totalBalanceUsd;
      const value =
        raw !== undefined && raw !== null && String(raw).trim() !== ""
          ? toNumber(raw) / 100
          : toNumber(String(usd).replace("$", ""));
      return { name: code, value: Number.isFinite(value) ? value : 0 };
    });

    return {
      tooltip: {
        trigger: "item",
        formatter: (p: any) => {
          const name = p?.name ?? "-";
          const value = Number.isFinite(p?.value) ? p.value : 0;
          const percent = Number.isFinite(p?.percent) ? p.percent : 0;
          return `${name}<br/>$${value.toFixed(2)}<br/>${percent.toFixed(2)}%`;
        },
      },
      legend: { type: "scroll", bottom: 0 },
      series: [
        {
          type: "pie",
          radius: ["36%", "72%"],
          center: ["50%", "46%"],
          avoidLabelOverlap: true,
          itemStyle: { borderRadius: 6, borderColor: "#fff", borderWidth: 2 },
          label: { show: true, formatter: "{b}\n{d}%" },
          labelLine: { show: true, length: 14, length2: 10 },
          data: seriesData,
        },
      ],
    };
  }, [assetDist, toNumber]);

  const trendOption = useMemo<echarts.EChartsOption>(() => {
    const daily: any[] = trend?.daily_stats || trend?.dailyStats || [];
    const dates = daily.map((d: any) => String(d?.date ?? "").trim());
    const totalCounts = daily.map((d: any) =>
      toNumber(d?.total_count ?? d?.totalCount)
    );
    const successRates = daily.map((d: any) =>
      toNumber(d?.success_rate ?? d?.successRate)
    );

    return {
      tooltip: { trigger: "axis" },
      legend: { bottom: 0 },
      grid: { left: 44, right: 60, top: 18, bottom: 56, containLabel: true },
      xAxis: {
        type: "category",
        data: dates,
        axisLabel: { formatter: (v: string) => v },
      },
      yAxis: [
        {
          type: "value",
          name: t("pages.vault.subAddresses.visualization.count", "次数"),
          minInterval: 1,
        },
        {
          type: "value",
          name: t(
            "pages.vault.subAddresses.visualization.successRate",
            "成功率"
          ),
          min: 0,
          max: 100,
        },
      ],
      series: [
        {
          name: t(
            "pages.vault.subAddresses.visualization.totalCount",
            "归集次数"
          ),
          type: "line",
          smooth: true,
          data: totalCounts,
        },
        {
          name: t(
            "pages.vault.subAddresses.visualization.successRate",
            "成功率"
          ),
          type: "line",
          smooth: true,
          yAxisIndex: 1,
          data: successRates,
          tooltip: { valueFormatter: (v: any) => `${toNumber(v).toFixed(2)}%` },
        },
      ],
    };
  }, [t, toNumber, trend]);

  const columns: ColumnsType<AdminDepositAddressBalanceItem> = [
    {
      title: t("pages.vault.subAddresses.table.address", "地址"),
      dataIndex: "address",
      width: 260,
      render: (text, record) => {
        const address = (text || "").trim();
        const asset = (record.asset_code || "").trim().toUpperCase();
        const iconText = (asset || "?").charAt(0);
        return (
          <div className={styles.addressCell}>
            {/* <div className={styles.addressIcon} style={{ background: getCurrencyColor(asset) }}>
              {iconText}
            </div> */}
            <span className={styles.addressText}>{address || "-"}</span>
            <Button
              type="text"
              size="small"
              icon={<CopyOutlined />}
              className={styles.copyBtn}
              disabled={!address}
              onClick={() => {
                navigator.clipboard.writeText(address);
                message.success(
                  t("pages.vault.subAddresses.messages.copied", "已复制")
                );
              }}
            />
          </div>
        );
      },
    },
    {
      title: t("pages.vault.subAddresses.table.userId", "用户ID"),
      dataIndex: "user_id",
      width: 120,
      render: (v) => (v ? String(v) : "-"),
    },
    {
      title: t("pages.vault.subAddresses.table.currency", "币种"),
      dataIndex: "asset_code",
      width: 90,
      render: (v) => (v ? String(v) : "-"),
    },
    {
      title: t("pages.vault.subAddresses.table.chain", "链"),
      dataIndex: "chain_code",
      width: 90,
      render: (v) => (v ? String(v) : "-"),
    },
    {
      title: t("pages.vault.subAddresses.table.balance", "余额"),
      dataIndex: "balance",
      width: 200,
      render: (_, record) => {
        const asset = (record.asset_code || "").trim().toUpperCase();
        const balance = record.balance ?? "0";
        const usd = record.balance_usd ?? "0";
        return (
          <div className={styles.balanceCell}>
            <span className={styles.balanceValue}>
              {balance} {asset}
            </span>
            <span className={styles.balanceUsdt}>≈ ${usd}</span>
          </div>
        );
      },
    },
    {
      title: t("pages.vault.subAddresses.table.lastCollection", "上次归集"),
      dataIndex: "last_sweep_at",
      width: 160,
      render: (v) => (v ? String(v) : "-"),
    },
    {
      title: t("pages.vault.subAddresses.table.status", "状态"),
      key: "status",
      width: 160,
      render: (_, record) => {
        const sync = (record.sync_status || "unknown").trim().toLowerCase();
        const syncConfig: Record<string, { label: string; className: string }> =
          {
            synced: { label: "已同步", className: styles.statusActive },
            syncing: { label: "同步中", className: styles.statusFrozen },
            unknown: { label: "未知", className: styles.statusIdle },
            error: { label: "异常", className: styles.statusError },
          };
        const config = syncConfig[sync] || syncConfig.unknown;
        return (
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              flexWrap: "wrap",
            }}
          >
            {record.needs_sweep ? (
              <Tag className={cx(styles.statusTag, styles.statusFrozen)}>
                待归集
              </Tag>
            ) : null}
            <Tag className={cx(styles.statusTag, config.className)}>
              {config.label}
            </Tag>
          </div>
        );
      },
    },
    {
      title: t("pages.vault.subAddresses.table.actions", "操作"),
      key: "actions",
      width: 160,
      render: (_, record) => (
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <Button
            type="primary"
            size="small"
            icon={<SyncOutlined />}
            className={styles.collectBtn}
            disabled={
              !canRpc(PERM.sweepOneClick) ||
              !record.needs_sweep ||
              sweepSubmitting
            }
            onClick={() => handleCollect(record)}
          >
            {t("pages.vault.subAddresses.actions.collect", "归集")}
          </Button>
          <Dropdown
            menu={{
              items: [
                {
                  key: "freeze",
                  label: t(
                    "pages.vault.subAddresses.actions.freezeAddress",
                    "冻结地址"
                  ),
                  icon: <PauseCircleOutlined />,
                  disabled: !canRpc(PERM.updateAddress),
                  onClick: () =>
                    confirmFreezeAddress(record.deposit_address_id),
                },
              ],
            }}
            trigger={["click"]}
          >
            <Button
              type="text"
              size="small"
              icon={<MoreOutlined />}
              className={styles.moreBtn}
            />
          </Dropdown>
        </div>
      ),
    },
  ];

  const taskColumns: ColumnsType<AdminSweepTaskItem> = [
    {
      title: t("pages.vault.subAddresses.taskRecords.columns.taskNo", "任务号"),
      dataIndex: "task_no",
      width: 180,
      render: (v) => (v ? <Text code>{String(v)}</Text> : "-"),
    },
    {
      title: t("pages.vault.subAddresses.taskRecords.columns.userId", "用户ID"),
      dataIndex: "user_id",
      width: 120,
      render: (v) => (v ? String(v) : "-"),
    },
    {
      title: t("pages.vault.subAddresses.taskRecords.columns.asset", "币种"),
      dataIndex: "asset_code",
      width: 90,
      render: (v) => (v ? String(v) : "-"),
    },
    {
      title: t("pages.vault.subAddresses.taskRecords.columns.chain", "链"),
      dataIndex: "chain_code",
      width: 90,
      render: (v) => (v ? String(v) : "-"),
    },
    {
      title: t("pages.vault.subAddresses.taskRecords.columns.amount", "金额"),
      dataIndex: "amount",
      width: 200,
      render: (_, record) => {
        const asset = (record.asset_code || "").trim().toUpperCase();
        const amount = record.amount ?? (record as any).amount_raw ?? "-";
        const usd =
          (record as any).amount_usd ?? (record as any).amount_usd_raw ?? "-";
        return (
          <div className={styles.balanceCell}>
            <span className={styles.balanceValue}>
              {String(amount)} {asset}
            </span>
            <span className={styles.balanceUsdt}>≈ ${String(usd)}</span>
          </div>
        );
      },
    },
    {
      title: t("pages.vault.subAddresses.taskRecords.columns.status", "状态"),
      dataIndex: "status",
      width: 100,
      render: (v) => {
        const status = (v || "").trim().toLowerCase();
        const statusConfig: Record<
          string,
          { label: string; className: string }
        > = {
          completed: { label: "完成", className: styles.statusActive },
          pending: { label: "处理中", className: styles.statusFrozen },
          failed: { label: "失败", className: styles.statusError },
        };
        const config = statusConfig[status] || {
          label: status || "unknown",
          className: styles.statusIdle,
        };
        return (
          <Tag className={cx(styles.statusTag, config.className)}>
            {config.label}
          </Tag>
        );
      },
    },
    {
      title: t("pages.vault.subAddresses.taskRecords.columns.txHash", "TxHash"),
      dataIndex: "tx_hash",
      width: 260,
      render: (v) => {
        const tx = (v || "").trim();
        if (!tx) return "-";
        return (
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <Text code>{tx}</Text>
            <Button
              type="text"
              size="small"
              icon={<CopyOutlined />}
              className={styles.copyBtn}
              onClick={() => {
                navigator.clipboard.writeText(tx);
                message.success(
                  t("pages.vault.subAddresses.messages.copied", "已复制")
                );
              }}
            />
          </div>
        );
      },
    },
    {
      title: t(
        "pages.vault.subAddresses.taskRecords.columns.createdAt",
        "创建时间"
      ),
      dataIndex: "created_at",
      width: 180,
      render: (v) => (v ? String(v) : "-"),
    },
  ];

  const tabItems = [
    {
      key: "addresses",
      label: t("pages.vault.subAddresses.tabs.addressList", "子地址列表"),
    },
    {
      key: "tasks",
      label: t("pages.vault.subAddresses.tabs.taskRecords", "归集任务记录"),
    },
  ];

  const hasChartPerm = canRpc(PERM.stats);
  const pieHasData = (assetDist?.assets || []).length > 0;
  const trendHasData =
    (trend?.daily_stats || trend?.dailyStats || []).length > 0;

  const batchBtnDisabled =
    !canRpc(PERM.sweepBatch) || sweepSubmitting || selectedRows.length === 0;

  return (
    <PageContainer
      className={styles.page}
      title={false}
      header={{ title: "", breadcrumb: {} }}
    >
      <div className={styles.header}>
        <div className={styles.headerLeft}>
          <h1 className={styles.title}>
            {t("pages.vault.subAddresses.title", "子地址管理")}
          </h1>
          <span className={styles.subtitle}>
            {t(
              "pages.vault.subAddresses.subtitle",
              "管理活跃钱包的所有子地址和资金归集"
            )}
          </span>
        </div>
        <div className={styles.headerRight}>
          <Button
            icon={<SyncOutlined spin={refreshing} />}
            onClick={handleRefresh}
            className={styles.refreshBtn}
            disabled={refreshing}
          >
            {t("pages.vault.subAddresses.actions.refresh", "刷新")}
          </Button>
          <Input
            placeholder={t(
              "pages.vault.subAddresses.search.placeholder",
              "搜索用户ID..."
            )}
            prefix={<SearchOutlined style={{ color: "#9ca3af" }} />}
            className={styles.searchInput}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            onPressEnter={applyUserIdSearch}
            allowClear
          />
        </div>
      </div>

      <div className={styles.statsRow}>
        <div className={styles.statCard}>
          <div className={styles.statLeft}>
            <span className={styles.statLabel}>
              {t("pages.vault.subAddresses.stats.totalAddresses", "总子地址数")}
            </span>
            <span className={styles.statValue}>
              {toNumber(stats?.total_addresses)}
            </span>
            <span className={styles.statSub}>
              {t("pages.vault.subAddresses.stats.needsSweep", "待归集")}{" "}
              {toNumber(stats?.needs_sweep_count)}
            </span>
          </div>
          <div className={cx(styles.statIcon, styles.statIconBlue)}>
            <LineChartOutlined />
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statLeft}>
            <span className={styles.statLabel}>
              {t(
                "pages.vault.subAddresses.stats.pendingCollection",
                "待归集总额"
              )}
            </span>
            <span className={styles.statValue}>
              ${stats?.pending_sweep_usd ?? "0"}
            </span>
            <span className={cx(styles.statSub, styles.statSubGray)}>
              {t("pages.vault.subAddresses.stats.totalBalance", "总余额")} $
              {stats?.total_balance_usd ?? "0"}
            </span>
          </div>
          <div className={cx(styles.statIcon, styles.statIconGreen)}>
            <LineChartOutlined />
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statLeft}>
            <span className={styles.statLabel}>
              {t(
                "pages.vault.subAddresses.stats.activeAddresses",
                "活跃地址数"
              )}
            </span>
            <span className={styles.statValue}>
              {toNumber(stats?.active_addresses)}
            </span>
            <span className={cx(styles.statSub, styles.statSubGray)}>
              {t("pages.vault.subAddresses.stats.percentage", "占比")}{" "}
              {(() => {
                const total = toNumber(stats?.total_addresses);
                const active = toNumber(stats?.active_addresses);
                if (!total) return "0.0%";
                return `${((active / total) * 100).toFixed(1)}%`;
              })()}
            </span>
          </div>
          <div className={cx(styles.statIcon, styles.statIconPurple)}>
            <CheckCircleOutlined />
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statLeft}>
            <span className={styles.statLabel}>
              {t(
                "pages.vault.subAddresses.stats.last24hSweep",
                "24小时归集次数"
              )}
            </span>
            <span className={styles.statValue}>
              {toNumber(stats?.last_24h_sweep_count)}
            </span>
            <span className={styles.statSub}>
              {t("pages.vault.subAddresses.stats.successRate", "成功率")}{" "}
              {(stats?.sweep_success_rate ?? 0).toFixed(2)}%
            </span>
          </div>
          <div className={cx(styles.statIcon, styles.statIconOrange)}>
            <SyncOutlined />
          </div>
        </div>
      </div>

      <div className={styles.visualSection}>
        <div
          className={styles.visualHeader}
          onClick={() => {
            setVisualExpanded(!visualExpanded);
          }}
        >
          <div className={styles.visualHeaderLeft}>
            {visualExpanded ? <DownOutlined /> : <RightOutlined />}
            <span>
              {t("pages.vault.subAddresses.visualization.title", "数据可视化")}
            </span>
          </div>
          <div
            className={styles.visualTools}
            onClick={(e) => e.stopPropagation()}
          >
            <span style={{ color: "#9ca3af" }}>
              {t("pages.vault.subAddresses.visualization.lastDays", "最近")}
              {TREND_DAYS}
              {t("pages.vault.subAddresses.visualization.days", "天")}
            </span>
          </div>
        </div>

        {visualExpanded && (
          <div className={styles.visualContent}>
            <div className={styles.chartCard}>
              <div className={styles.chartTitleRow}>
                <div className={styles.chartTitle}>
                  <PieChartOutlined style={{ color: "#3b82f6" }} />
                  {t(
                    "pages.vault.subAddresses.visualization.currencyDistribution",
                    "币种分布（待归集金额）"
                  )}
                </div>
                <Button
                  size="small"
                  icon={<SyncOutlined />}
                  onClick={() => fetchAssetDist()}
                  disabled={assetDistLoading || !hasChartPerm}
                >
                  {t("pages.vault.subAddresses.actions.refresh", "刷新")}
                </Button>
              </div>
              {!hasChartPerm ? (
                <div className={styles.chartPlaceholder}>
                  {t(
                    "pages.vault.subAddresses.taskRecords.noPermission",
                    "暂无权限"
                  )}
                </div>
              ) : !pieHasData && !assetDistLoading ? (
                <div className={styles.chartPlaceholder}>
                  {t(
                    "pages.vault.subAddresses.visualization.empty",
                    "暂无数据"
                  )}
                </div>
              ) : (
                <EChart
                  option={pieOption}
                  loading={assetDistLoading}
                  height={420}
                />
              )}
            </div>

            <div className={styles.chartCard}>
              <div className={styles.chartTitleRow}>
                <div className={styles.chartTitle}>
                  <LineChartOutlined style={{ color: "#10b981" }} />
                  {t(
                    "pages.vault.subAddresses.visualization.collectionTrend",
                    "归集频率趋势（最近7天）"
                  )}
                </div>
                <Button
                  size="small"
                  icon={<SyncOutlined />}
                  onClick={() => fetchTrend()}
                  disabled={trendLoading || !hasChartPerm}
                >
                  {t("pages.vault.subAddresses.actions.refresh", "刷新")}
                </Button>
              </div>
              {!hasChartPerm ? (
                <div className={styles.chartPlaceholder}>
                  {t(
                    "pages.vault.subAddresses.taskRecords.noPermission",
                    "暂无权限"
                  )}
                </div>
              ) : !trendHasData && !trendLoading ? (
                <div className={styles.chartPlaceholder}>
                  {t(
                    "pages.vault.subAddresses.visualization.empty",
                    "暂无数据"
                  )}
                </div>
              ) : (
                <EChart
                  option={trendOption}
                  loading={trendLoading}
                  height={420}
                />
              )}
            </div>
          </div>
        )}
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={(key) => {
          const next = key as "addresses" | "tasks";
          setActiveTab(next);
          if (next === "tasks") setTasksPage(1);
        }}
        items={tabItems}
        className={styles.tabsContainer}
      />

      {activeTab === "addresses" ? (
        <div className={styles.mainContent}>
          <div className={styles.filterSidebar}>
            <div
              className={styles.filterHeader}
              onClick={() => setFilterExpanded(!filterExpanded)}
            >
              <div className={styles.filterTitle}>
                <FilterOutlined />
                {t("pages.vault.subAddresses.filter.title", "筛选条件")}
              </div>
              {filterExpanded ? <DownOutlined /> : <RightOutlined />}
            </div>

            {filterExpanded && (
              <>
                <div className={styles.filterSection}>
                  <div className={styles.filterSectionTitle}>
                    {t("pages.vault.subAddresses.filter.currency", "币种")}
                  </div>
                  <Select
                    value={assetCodeDraft}
                    onChange={(v) => setAssetCodeDraft(v)}
                    options={[
                      { value: "all", label: "全部" },
                      ...currencyOptions.map((c) => ({
                        value: c.value,
                        label: c.label,
                      })),
                    ]}
                    style={{ width: "100%" }}
                  />
                </div>

                <div className={styles.filterSection}>
                  <div className={styles.filterSectionTitle}>
                    {t("pages.vault.subAddresses.filter.chain", "链")}
                  </div>
                  <Select
                    value={chainCodeDraft}
                    onChange={(v) => setChainCodeDraft(v)}
                    options={chainOptions}
                    style={{ width: "100%" }}
                  />
                </div>

                <div className={styles.filterSection}>
                  <div className={styles.filterSectionTitle}>
                    {t("pages.vault.subAddresses.filter.needsSweep", "归集")}
                  </div>
                  <Select
                    value={needsSweepDraft}
                    onChange={(v) => setNeedsSweepDraft(v)}
                    options={needsSweepOptions}
                    style={{ width: "100%" }}
                  />
                </div>

                <div className={styles.filterSection}>
                  <div className={styles.filterSectionTitle}>
                    {t(
                      "pages.vault.subAddresses.filter.syncStatus",
                      "同步状态"
                    )}
                  </div>
                  <Select
                    value={syncStatusDraft}
                    onChange={(v) => setSyncStatusDraft(v)}
                    options={syncStatusOptions}
                    style={{ width: "100%" }}
                  />
                </div>

                <div className={styles.filterSection}>
                  <div className={styles.filterSectionTitle}>
                    {t(
                      "pages.vault.subAddresses.filter.balanceRange",
                      "余额范围（USD）"
                    )}
                  </div>
                  <div className={styles.sliderContainer}>
                    <Slider
                      range
                      min={0}
                      max={MAX_BALANCE_USD}
                      value={balanceRangeUsdDraft}
                      onChange={(value) => {
                        balanceRangeUsdDraftSourceRef.current = "slider";
                        setBalanceRangeUsdDraft(value as [number, number]);
                      }}
                      onAfterChange={(value) => {
                        balanceRangeUsdDraftSourceRef.current = null;
                        setBalanceRangeUsdDraft(
                          normalizeBalanceRangeUsd(value as [number, number])
                        );
                      }}
                    />
                    <div className={styles.sliderInputs}>
                      <InputNumber
                        className={styles.sliderInput}
                        value={balanceRangeUsdDraft[0]}
                        min={0}
                        max={MAX_BALANCE_USD}
                        step={1}
                        precision={0}
                        onChange={(v) => {
                          balanceRangeUsdDraftSourceRef.current = "input";
                          setBalanceRangeUsdDraft([
                            typeof v === "number" ? v : 0,
                            balanceRangeUsdDraft[1],
                          ]);
                        }}
                      />
                      <span>
                        {t("pages.vault.subAddresses.filter.to", "至")}
                      </span>
                      <InputNumber
                        className={styles.sliderInput}
                        value={balanceRangeUsdDraft[1]}
                        min={0}
                        max={MAX_BALANCE_USD}
                        step={1}
                        precision={0}
                        onChange={(v) => {
                          balanceRangeUsdDraftSourceRef.current = "input";
                          setBalanceRangeUsdDraft([
                            balanceRangeUsdDraft[0],
                            typeof v === "number" ? v : MAX_BALANCE_USD,
                          ]);
                        }}
                      />
                    </div>
                  </div>
                </div>

                <Space direction="vertical" size={10} style={{ width: "100%" }}>
                  <Button
                    type="primary"
                    icon={<SyncOutlined />}
                    className={styles.collectAllBtn}
                    onClick={handleCollectAll}
                    disabled={batchBtnDisabled}
                  >
                    {t(
                      "pages.vault.subAddresses.actions.collectSelected",
                      "批量一键归集选中地址"
                    )}
                  </Button>
                  <Button
                    className={styles.resetBtn}
                    onClick={handleReset}
                    disabled={sweepSubmitting}
                  >
                    {t("pages.vault.subAddresses.actions.reset", "重置")}
                  </Button>
                </Space>
              </>
            )}
          </div>

          <div className={styles.tableSection}>
            <div className={styles.tableHeader}>
              <div className={styles.tableHeaderLeft}>
                <span className={styles.recordCount}>
                  {t(
                    "pages.vault.subAddresses.table.total",
                    "共 {count} 条记录",
                    { count: toNumber(balancesPagination?.total) }
                  )}
                </span>
                <span>
                  {t("pages.vault.subAddresses.table.pageSize", "每页显示")}
                  <Select
                    value={pageSize}
                    onChange={(v) => {
                      setPageSize(v);
                      setCurrentPage(1);
                    }}
                    options={[
                      { value: 20, label: "20 条" },
                      { value: 50, label: "50 条" },
                      { value: 100, label: "100 条" },
                    ]}
                    style={{ width: 90, marginLeft: 8 }}
                    size="small"
                  />
                </span>
              </div>
              <Space size={10}>
                <Button
                  size="small"
                  icon={<SyncOutlined />}
                  onClick={() => fetchBalances()}
                  disabled={balancesLoading || !canRpc(PERM.listBalances)}
                >
                  {t("pages.vault.subAddresses.actions.refresh", "刷新")}
                </Button>
                <Pagination
                  current={currentPage}
                  pageSize={pageSize}
                  total={toNumber(balancesPagination?.total)}
                  onChange={(page) => setCurrentPage(page)}
                  size="small"
                  showSizeChanger={false}
                />
              </Space>
            </div>
            <div className={styles.tableCard}>
              <Table
                rowSelection={{
                  selectedRowKeys,
                  onChange: (keys, rows) => {
                    setSelectedRowKeys(keys);
                    setSelectedRows(rows);
                  },
                }}
                columns={columns}
                dataSource={balances}
                rowKey="id"
                pagination={false}
                loading={balancesLoading}
                size="middle"
                scroll={{ x: "max-content" }}
              />
            </div>
          </div>
        </div>
      ) : (
        <div className={styles.taskRecordsCard}>
          <div className={styles.taskRecordsHeader}>
            <div className={styles.taskRecordsTitle}>
              {t("pages.vault.subAddresses.taskRecords.title", "归集任务记录")}
            </div>
            <div className={styles.taskRecordsSub}>
              {t(
                "pages.vault.subAddresses.taskRecords.subtitle",
                "查看所有归集操作的执行记录"
              )}
            </div>
          </div>
          {!canRpc(PERM.listTasks) ? (
            <Empty
              image={<FileTextOutlined className={styles.emptyIcon} />}
              description={
                <span className={styles.emptyText}>
                  {t(
                    "pages.vault.subAddresses.taskRecords.noPermission",
                    "暂无权限"
                  )}
                </span>
              }
            />
          ) : (
            <>
              <div className={styles.tableHeader}>
                <div className={styles.tableHeaderLeft}>
                  <span className={styles.recordCount}>
                    {t(
                      "pages.vault.subAddresses.table.total",
                      "共 {count} 条记录",
                      { count: toNumber(tasksPagination?.total) }
                    )}
                  </span>
                  <span>
                    {t("pages.vault.subAddresses.table.pageSize", "每页显示")}
                    <Select
                      value={tasksPageSize}
                      onChange={(v) => {
                        setTasksPageSize(v);
                        setTasksPage(1);
                      }}
                      options={[
                        { value: 10, label: "10 条" },
                        { value: 20, label: "20 条" },
                        { value: 50, label: "50 条" },
                      ]}
                      style={{ width: 90, marginLeft: 8 }}
                      size="small"
                    />
                  </span>
                </div>
                <Space size={10}>
                  <Button
                    size="small"
                    icon={<SyncOutlined />}
                    onClick={() => fetchTasks()}
                    disabled={tasksLoading}
                  >
                    {t("pages.vault.subAddresses.actions.refresh", "刷新")}
                  </Button>
                  <Pagination
                    current={tasksPage}
                    pageSize={tasksPageSize}
                    total={toNumber(tasksPagination?.total)}
                    onChange={(page) => setTasksPage(page)}
                    size="small"
                    showSizeChanger={false}
                  />
                </Space>
              </div>
              <div className={styles.tableCard}>
                <Table
                  columns={taskColumns}
                  dataSource={tasks}
                  rowKey="id"
                  pagination={false}
                  loading={tasksLoading}
                  size="middle"
                  scroll={{ x: "max-content" }}
                  locale={{
                    emptyText: (
                      <Empty
                        image={
                          <FileTextOutlined className={styles.emptyIcon} />
                        }
                        description={
                          <span className={styles.emptyText}>
                            {t(
                              "pages.vault.subAddresses.taskRecords.empty",
                              "暂无归集任务记录"
                            )}
                          </span>
                        }
                      />
                    ),
                  }}
                />
              </div>
            </>
          )}
        </div>
      )}

      <Modal
        open={collectModalVisible}
        onCancel={handleCloseCollectModal}
        footer={[
          <Button key="close" onClick={handleCloseCollectModal}>
            {t("pages.vault.subAddresses.modal.collect.close", "关闭")}
          </Button>,
        ]}
        width={520}
        title={
          <div className={styles.collectModalTitle}>
            <div className={styles.collectModalIcon}>
              <CheckCircleOutlined />
            </div>
            <span className={styles.collectModalTitleText}>
              {t("pages.vault.subAddresses.modal.collect.title", "归集成功")}
            </span>
          </div>
        }
      >
        <div className={styles.collectModalSubtitle}>
          {t(
            "pages.vault.subAddresses.modal.collect.subtitle",
            "子地址资金已成功提交归集任务"
          )}
        </div>

        {collectResult && (
          <div className={styles.collectModalCard}>
            <div className={styles.collectModalRow}>
              <span className={styles.collectModalLabel}>
                {t(
                  "pages.vault.subAddresses.modal.collect.address",
                  "归集地址"
                )}
              </span>
              <div className={styles.collectModalValue}>
                <div
                  className={styles.collectModalAddressIcon}
                  style={{
                    background: getCurrencyColor(
                      collectResult.balance.asset_code || ""
                    ),
                  }}
                >
                  {(collectResult.balance.asset_code || "?")
                    .toUpperCase()
                    .charAt(0)}
                </div>
                <span className={styles.collectModalAddress}>
                  {collectResult.balance.address || "-"}
                </span>
              </div>
            </div>
            <div className={styles.collectModalRow}>
              <span className={styles.collectModalLabel}>
                {t("pages.vault.subAddresses.modal.collect.amount", "归集金额")}
              </span>
              <span className={styles.collectModalAmount}>
                {collectResult.balance.balance || "0"}{" "}
                {(collectResult.balance.asset_code || "").toUpperCase()}
              </span>
            </div>
            <div className={styles.collectModalRow}>
              <span className={styles.collectModalLabel}>
                {t(
                  "pages.vault.subAddresses.modal.collect.usdtValue",
                  "等值USDT"
                )}
              </span>
              <span className={styles.collectModalUsdt}>
                ${collectResult.balance.balance_usd || "0"}
              </span>
            </div>
            <div className={styles.collectModalRow}>
              <span className={styles.collectModalLabel}>
                {t("pages.vault.subAddresses.modal.collect.taskNo", "任务号")}
              </span>
              <span className={styles.collectModalValue}>
                <Text code>{collectResult.taskNo || "-"}</Text>
              </span>
            </div>
            <div className={styles.collectModalRow}>
              <span className={styles.collectModalLabel}>
                {t("pages.vault.subAddresses.modal.collect.txHash", "TxHash")}
              </span>
              <span className={styles.collectModalValue}>
                <Text code>{collectResult.txHash || "-"}</Text>
              </span>
            </div>
          </div>
        )}

        <div className={styles.collectModalFooter}>
          {t(
            "pages.vault.subAddresses.modal.collect.processingTime",
            "归集任务已提交，预计5-10分钟内完成处理"
          )}
        </div>
      </Modal>
    </PageContainer>
  );
};

export default SubAddressesPage;
