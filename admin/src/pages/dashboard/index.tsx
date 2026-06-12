import {
  adminGetDashboardActivities,
  adminGetDashboardOverview,
  adminGetDashboardTrends,
} from "@/api/generated/dashboard";
import { Column, Line, Pie } from "@ant-design/charts";
import {
  AuditOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  DollarCircleOutlined,
  FileTextOutlined,
  RiseOutlined,
  SendOutlined,
  TeamOutlined,
  UserOutlined,
  WalletOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-components";
import { Link, useIntl } from "@umijs/max";
import { Spin, message } from "antd";
import { createStyles } from "antd-style";
import dayjs from "dayjs";
import "dayjs/locale/zh-cn";
import relativeTime from "dayjs/plugin/relativeTime";
import React, { useCallback, useEffect, useMemo, useState } from "react";

dayjs.extend(relativeTime);
dayjs.locale("zh-cn");

const motionStyles = `
@keyframes dashboard-rise {
  0% { transform: translateY(14px); opacity: 0; }
  100% { transform: translateY(0); opacity: 1; }
}
@keyframes dashboard-shimmer {
  0% { background-position: 0% 50%; }
  100% { background-position: 100% 50%; }
}
@keyframes pulse-glow {
  0%, 100% { opacity: 0.6; }
  50% { opacity: 1; }
}
`;

const useStyles = createStyles(({ token }) => ({
  page: { position: "relative" },
  topKpiRow: {
    display: "grid",
    gridTemplateColumns: "repeat(4, 1fr)",
    gap: 16,
    marginBottom: 16,
    "@media (max-width: 1200px)": { gridTemplateColumns: "repeat(2, 1fr)" },
    "@media (max-width: 768px)": { gridTemplateColumns: "1fr" },
  },
  topKpiCard: {
    position: "relative",
    borderRadius: 16,
    padding: "20px 24px",
    color: "#fff",
    overflow: "hidden",
    animation: "dashboard-rise 0.5s ease-out both",
    minHeight: 100,
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
  },
  topKpiCardBlue: {
    background: "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
    boxShadow: "0 8px 32px rgba(102, 126, 234, 0.35)",
  },
  topKpiCardGreen: {
    background: "linear-gradient(135deg, #11998e 0%, #38ef7d 100%)",
    boxShadow: "0 8px 32px rgba(17, 153, 142, 0.35)",
  },
  topKpiCardPurple: {
    background: "linear-gradient(135deg, #667eea 0%, #a855f7 100%)",
    boxShadow: "0 8px 32px rgba(168, 85, 247, 0.35)",
  },
  topKpiCardOrange: {
    background: "linear-gradient(135deg, #f093fb 0%, #f5576c 100%)",
    boxShadow: "0 8px 32px rgba(245, 87, 108, 0.35)",
  },
  topKpiLeft: { display: "flex", flexDirection: "column", gap: 4 },
  topKpiIconWrap: {
    width: 40,
    height: 40,
    borderRadius: 10,
    background: "rgba(255, 255, 255, 0.2)",
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    marginBottom: 8,
    fontSize: 20,
  },
  topKpiLabel: { fontSize: 14, fontWeight: 500, opacity: 0.9 },
  topKpiSublabel: { fontSize: 12, opacity: 0.7 },
  topKpiValue: {
    fontSize: 36,
    fontWeight: 700,
    fontFamily: '"DIN Alternate", "Bebas Neue", "Teko", sans-serif',
    lineHeight: 1,
  },
  topKpiUnit: { fontSize: 18, fontWeight: 500, marginLeft: 4, opacity: 0.9 },
  topKpiDelta: {
    marginTop: 6,
    fontSize: 13,
    display: "flex",
    alignItems: "center",
    gap: 4,
    opacity: 0.9,
  },
  topKpiRight: { textAlign: "right" },
  topKpiExtra: { fontSize: 12, opacity: 0.8, marginTop: 4 },
  secondaryRow: {
    display: "grid",
    gridTemplateColumns: "repeat(2, 1fr)",
    gap: 16,
    marginBottom: 16,
    "@media (max-width: 768px)": { gridTemplateColumns: "1fr" },
  },
  secondaryCard: {
    borderRadius: 16,
    padding: "24px 28px",
    background: "#fff",
    border: "1px solid #f0f0f0",
    boxShadow: "0 4px 20px rgba(0, 0, 0, 0.04)",
    display: "flex",
    alignItems: "center",
    gap: 20,
    animation: "dashboard-rise 0.5s ease-out both",
  },
  secondaryIconWrap: {
    width: 52,
    height: 52,
    borderRadius: 14,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    fontSize: 24,
  },
  secondaryIconBlue: {
    background: "linear-gradient(135deg, #e0e7ff 0%, #c7d2fe 100%)",
    color: "#4f46e5",
  },
  secondaryIconPurple: {
    background: "linear-gradient(135deg, #f3e8ff 0%, #e9d5ff 100%)",
    color: "#9333ea",
  },
  secondaryContent: { flex: 1 },
  secondaryLabel: {
    fontSize: 15,
    fontWeight: 500,
    color: "#374151",
    display: "flex",
    alignItems: "center",
    gap: 8,
  },
  secondaryValue: {
    fontSize: 32,
    fontWeight: 700,
    color: "#4f46e5",
    fontFamily: '"DIN Alternate", "Bebas Neue", "Teko", sans-serif',
  },
  secondaryUnit: {
    fontSize: 18,
    fontWeight: 500,
    marginLeft: 4,
    color: "#6b7280",
  },
  secondarySub: { fontSize: 13, color: "#9ca3af", marginTop: 4 },
  chartsRow: {
    display: "grid",
    gridTemplateColumns: "repeat(2, 1fr)",
    gap: 16,
    marginBottom: 16,
    "@media (max-width: 1200px)": { gridTemplateColumns: "1fr" },
  },
  chartCard: {
    borderRadius: 16,
    padding: "24px",
    background: "#fff",
    border: "1px solid #f0f0f0",
    boxShadow: "0 4px 20px rgba(0, 0, 0, 0.04)",
    animation: "dashboard-rise 0.5s ease-out both",
  },
  chartHeader: { marginBottom: 20 },
  chartTitle: {
    fontSize: 18,
    fontWeight: 600,
    color: "#1f2937",
    marginBottom: 4,
  },
  chartSubtitle: { fontSize: 13, color: "#9ca3af" },
  chartLegend: { display: "flex", gap: 20, flexWrap: "wrap" },
  chartLegendItem: {
    display: "flex",
    alignItems: "center",
    gap: 6,
    fontSize: 13,
    color: "#6b7280",
  },
  legendDot: { width: 10, height: 10, borderRadius: "50%" },
  distributionRow: {
    display: "grid",
    gridTemplateColumns: "repeat(3, 1fr)",
    gap: 16,
    marginBottom: 16,
    "@media (max-width: 1200px)": { gridTemplateColumns: "repeat(2, 1fr)" },
    "@media (max-width: 768px)": { gridTemplateColumns: "1fr" },
  },
  distributionCard: {
    borderRadius: 16,
    padding: "24px",
    background: "#fff",
    border: "1px solid #f0f0f0",
    boxShadow: "0 4px 20px rgba(0, 0, 0, 0.04)",
    animation: "dashboard-rise 0.5s ease-out both",
  },
  distributionHeader: { marginBottom: 16 },
  distributionTitle: {
    fontSize: 18,
    fontWeight: 600,
    color: "#1f2937",
    marginBottom: 4,
  },
  distributionSubtitle: { fontSize: 13, color: "#9ca3af" },
  pieChartWrapper: {
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    gap: 16,
  },
  pieLegendList: {
    display: "flex",
    flexDirection: "column",
    gap: 10,
    width: "100%",
  },
  pieLegendItem: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
  },
  pieLegendLeft: { display: "flex", alignItems: "center", gap: 8 },
  pieLegendDot: { width: 10, height: 10, borderRadius: "50%" },
  pieLegendLabel: { fontSize: 14, color: "#6b7280" },
  pieLegendValue: { fontSize: 16, fontWeight: 600, color: "#1f2937" },
  vaultTokenList: { display: "flex", flexDirection: "column", gap: 16 },
  vaultTokenItem: { display: "flex", alignItems: "center", gap: 12 },
  vaultTokenRank: {
    width: 24,
    height: 24,
    borderRadius: 6,
    background: "#eff6ff",
    color: "#3b82f6",
    fontSize: 12,
    fontWeight: 600,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
  },
  vaultTokenInfo: { flex: 1 },
  vaultTokenTop: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: 6,
  },
  vaultTokenSymbol: { fontSize: 15, fontWeight: 600, color: "#1f2937" },
  vaultTokenUsd: { fontSize: 15, fontWeight: 600, color: "#1f2937" },
  vaultTokenBar: {
    height: 6,
    borderRadius: 3,
    background: "#e5e7eb",
    overflow: "hidden",
  },
  vaultTokenBarFill: {
    height: "100%",
    borderRadius: 3,
    background: "linear-gradient(90deg, #3b82f6 0%, #60a5fa 100%)",
  },
  vaultTokenBottom: { fontSize: 12, color: "#9ca3af", marginTop: 4 },
  activityCard: {
    borderRadius: 16,
    padding: "24px",
    background: "#fff",
    border: "1px solid #f0f0f0",
    boxShadow: "0 4px 20px rgba(0, 0, 0, 0.04)",
    animation: "dashboard-rise 0.5s ease-out both",
    marginTop: 16,
  },
  activityHeader: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "flex-start",
    marginBottom: 20,
  },
  activityHeaderLeft: { display: "flex", flexDirection: "column" },
  activityHeaderTitle: {
    fontSize: 18,
    fontWeight: 600,
    color: "#1f2937",
    marginBottom: 4,
  },
  activityHeaderSub: { fontSize: 13, color: "#9ca3af" },
  activityViewAll: {
    fontSize: 14,
    color: "#3b82f6",
    cursor: "pointer",
    "&:hover": { color: "#2563eb" },
  },
  activityList: { display: "flex", flexDirection: "column", gap: 16 },
  activityItem: {
    display: "flex",
    alignItems: "center",
    gap: 16,
    padding: "12px 0",
    borderBottom: "1px solid #f3f4f6",
    "&:last-child": { borderBottom: "none", paddingBottom: 0 },
  },
  activityIconWrap: {
    width: 44,
    height: 44,
    borderRadius: 12,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    fontSize: 20,
    flexShrink: 0,
  },
  activityIconTransfer: { background: "#fef3c7", color: "#f59e0b" },
  activityIconWithdrawal: { background: "#fef3c7", color: "#f59e0b" },
  activityIconUser: { background: "#eff6ff", color: "#3b82f6" },
  activityIconVault: { background: "#ecfdf5", color: "#10b981" },
  activityContent: { flex: 1, minWidth: 0 },
  activityTitle: {
    fontSize: 15,
    fontWeight: 600,
    color: "#1f2937",
    marginBottom: 4,
  },
  activityDesc: { fontSize: 13, color: "#6b7280", marginBottom: 4 },
  activityTime: { fontSize: 12, color: "#9ca3af" },
  activityStatus: { flexShrink: 0, fontSize: 20 },
  activityStatusSuccess: { color: "#10b981" },
  activityStatusError: { color: "#ef4444" },
  activityStatusInfo: { color: "#6b7280" },
}));

type ActivityLevel = "success" | "error" | "info" | "warning";
type UiActivityType = "transfer_batch" | "withdrawal" | "user" | "vault";

type UiActivity = {
  id: string;
  type: UiActivityType;
  title: string;
  description?: string;
  at: string;
  level: ActivityLevel;
};

const toNumber = (v: any) => {
  if (v === null || v === undefined) return 0;
  if (typeof v === "number") return Number.isFinite(v) ? v : 0;
  const s = String(v).trim();
  if (!s) return 0;
  const n = Number(s.replace(/,/g, ""));
  return Number.isFinite(n) ? n : 0;
};

const toPct = (v: any) => {
  const n = toNumber(v);
  if (!Number.isFinite(n)) return 0;
  if (n > 0 && n <= 1) return Number((n * 100).toFixed(1));
  return Number(n.toFixed(1));
};

const parseMs = (raw?: string) => {
  if (!raw) return 0;
  const d = dayjs(raw);
  const v = d.valueOf();
  return Number.isFinite(v) ? v : 0;
};

const formatMD = (raw?: string) => {
  if (!raw) return "";
  const d = dayjs(raw);
  return d.isValid() ? d.format("MM-DD") : raw;
};

const normalizeActivityType = (v?: string): UiActivityType => {
  const s = (v || "").toLowerCase();
  if (s.includes("withdraw")) return "withdrawal";
  if (s.includes("vault")) return "vault";
  if (s.includes("user") || s.includes("register")) return "user";
  if (s.includes("transfer") || s.includes("batch")) return "transfer_batch";
  return "transfer_batch";
};

const normalizeLevel = (v?: string): ActivityLevel => {
  const s = (v || "").toLowerCase();
  if (
    s.includes("success") ||
    s.includes("ok") ||
    s.includes("done") ||
    s.includes("completed")
  )
    return "success";
  if (s.includes("error") || s.includes("fail")) return "error";
  if (s.includes("warning") || s.includes("warn")) return "warning";
  return "info";
};

/**
 * 对包含"新用户注册"的 title 中的数字号码进行脱敏处理
 * 只要是数字号码长度大于6位，就把中间4位改成****
 * 例如：13812345678 -> 138****5678, 1234567 -> 123****7
 */
const maskPhoneInTitle = (title: string): string => {
  if (!title || !title.includes("新用户注册")) {
    return title;
  }
  
  // 匹配所有连续数字序列
  const numberRegex = /\d+/g;
  
  return title.replace(numberRegex, (number) => {
    // 如果数字长度大于6位，脱敏中间4位
    if (number.length > 6) {
      // 保留前3位，中间4位用****替换，保留后面的部分
      return number.substring(0, 3) + "****" + number.substring(7);
    }
    // 长度小于等于6位，不处理
    return number;
  });
};

const DashboardPage: React.FC = () => {
  const intl = useIntl();
  const t = (
    id: string,
    defaultMessage: string,
    values?: Record<string, any>
  ) => intl.formatMessage({ id, defaultMessage }, values);
  const { styles } = useStyles();

  const [loading, setLoading] = useState(false);
  const [overview, setOverview] = useState<any>(null);
  const [trends, setTrends] = useState<any>(null);
  const [recentActivity, setRecentActivity] = useState<UiActivity[]>([]);

  const fundFlowMonths = 6;
  const transferDays = 7;
  const forceRefresh = false;

  const fetchDashboard = useCallback(async () => {
    setLoading(true);
    try {
      const [ovRes, trRes, actRes] = await Promise.all([
        adminGetDashboardOverview({ force_refresh: forceRefresh } as any),
        adminGetDashboardTrends({
          fund_flow_months: fundFlowMonths,
          transfer_days: transferDays,
          force_refresh: forceRefresh,
        } as any),
        adminGetDashboardActivities({
          page: 1,
          page_size: 8,
          force_refresh: forceRefresh,
        } as any),
      ]);

      if (!ovRes?.success) throw new Error(ovRes?.message || "overview failed");
      if (!trRes?.success) throw new Error(trRes?.message || "trends failed");
      if (!actRes?.success)
        throw new Error(actRes?.message || "activities failed");

      setOverview(ovRes.data || null);
      setTrends(trRes.data || null);

      const acts = (actRes.data?.activities || []).map((a: any) => {
        const title = a?.title || "";
        const baseDesc = a?.description || "";
        const op = a?.operator ? `@${a.operator}` : "";
        const desc = baseDesc ? baseDesc : op ? `Operator: ${op}` : "";
        const at = a?.created_at || "";
        const id = a?.related_id || `${at}-${title}`;
        return {
          id,
          type: normalizeActivityType(a?.activity_type),
          title,
          description: desc,
          at,
          level: normalizeLevel(a?.status_icon),
        } as UiActivity;
      });

      setRecentActivity(acts);
    } catch (e: any) {
      message.error(e?.message || "Dashboard 加载失败");
    } finally {
      setLoading(false);
    }
  }, [fundFlowMonths, transferDays, forceRefresh]);

  useEffect(() => {
    fetchDashboard();
  }, [fetchDashboard]);

  const formatNumber = (value: number) => {
    const abs = Math.abs(value);
    const useCompact = abs >= 1000;
    return new Intl.NumberFormat(intl.locale, {
      notation: useCompact ? "compact" : "standard",
      maximumFractionDigits: useCompact ? 1 : 0,
    }).format(value);
  };

  const overviewUser = overview?.user_stats || {};
  const overviewVault = overview?.vault_balance || {};
  const overviewTodayTransfer = overview?.today_transfer || {};
  const overviewApproval = overview?.approval_stats || {};
  const overviewUserDist = overview?.user_status_dist || {};

  const totalUsers =
    toNumber(overviewUser.total_users) ||
    toNumber(overviewUserDist.normal) +
      toNumber(overviewUserDist.frozen) +
      toNumber(overviewUserDist.terminated);

  const normalUsers = toNumber(overviewUserDist.normal);
  const frozenUsers = toNumber(overviewUserDist.frozen);
  const terminatedUsers = toNumber(overviewUserDist.terminated);
  const distTotal = normalUsers + frozenUsers + terminatedUsers;

  const web2UsersValue = totalUsers;
  const vaultBalanceValue = toNumber(overviewVault.total_balance_usd);
  const vaultBalanceDelta = toPct(overviewVault.change_rate);

  const todayTransferAmount = toNumber(overviewTodayTransfer.today_amount_usd);
  const todayTransferCount = toNumber(overviewTodayTransfer.today_count);
  const todayTransferDelta = toPct(overviewTodayTransfer.growth_rate);

  const pendingWithdrawalsValue = toNumber(overviewApproval.pending_count);
  const pendingExtra = `${t(
    "pages.dashboard.kpi.todayNew",
    "今日新增"
  )} ${toNumber(overviewApproval.today_new_count)}`;

  const active7d = toNumber(overviewUser.active_users_7d);
  const supportedTokens = toNumber(overview?.supported_currencies);

  const fundFlowPoints = trends?.fund_flow_trend?.data_points || [];
  const batchPoints = trends?.transfer_batch_trend?.data_points || [];
  const approvalDist = trends?.approval_distribution || {};
  const vaultTop = trends?.vault_top_currencies || [];

  const inboundLabel = t("pages.dashboard.charts.fundFlow.inbound", "入账");
  const outboundLabel = t("pages.dashboard.charts.fundFlow.outbound", "出账");
  const internalLabel = t(
    "pages.dashboard.charts.fundFlow.internal",
    "内部转账"
  );

  const batchLabel = t("pages.dashboard.charts.batchExec.batch", "批次");
  const successLabel = t("pages.dashboard.charts.batchExec.success", "成功");
  const failedLabel = t("pages.dashboard.charts.batchExec.failed", "失败");

  const fundFlowColorMap = useMemo(
    () => ({
      [inboundLabel]: "#22c55e",
      [outboundLabel]: "#ef4444",
      [internalLabel]: "#3b82f6",
    }),
    [inboundLabel, outboundLabel, internalLabel]
  );

  const batchColorMap = useMemo(
    () => ({
      [batchLabel]: "#a855f7",
      [successLabel]: "#22c55e",
      [failedLabel]: "#ef4444",
    }),
    [batchLabel, successLabel, failedLabel]
  );

  const fundFlowTypeOrder = useMemo(
    () => [inboundLabel, outboundLabel, internalLabel],
    [inboundLabel, outboundLabel, internalLabel]
  );
  const batchTypeOrder = useMemo(
    () => [batchLabel, successLabel, failedLabel],
    [batchLabel, successLabel, failedLabel]
  );

  const trend = useMemo(() => {
    const rows = (fundFlowPoints || [])
      .map((p: any) => {
        const period = String(p?.period || "").trim();
        const sortKey = period ? parseMs(`${period}-01`) : 0;
        return {
          period,
          sortKey,
          inbound: toNumber(p?.deposit_amount),
          outbound: toNumber(p?.withdrawal_amount),
          internal: toNumber(p?.transfer_amount),
        };
      })
      .filter((r: any) => !!r.period);

    rows.sort((a: any, b: any) => Number(a.sortKey) - Number(b.sortKey));
    return rows;
  }, [fundFlowPoints]);

  const monthOrder = useMemo(() => trend.map((r: any) => r.period), [trend]);

  const fundFlowChartData = useMemo(() => {
    return trend.flatMap((item: any) => [
      { month: item.period, type: inboundLabel, value: item.inbound },
      { month: item.period, type: outboundLabel, value: item.outbound },
      { month: item.period, type: internalLabel, value: item.internal },
    ]);
  }, [trend, inboundLabel, outboundLabel, internalLabel]);

  const batchTrend7d = useMemo(() => {
    const rows = (batchPoints || [])
      .map((p: any) => {
        const rawDay = String(p?.date || "").trim();
        const sortKey = parseMs(rawDay);
        return {
          rawDay,
          day: formatMD(rawDay) || rawDay,
          sortKey,
          total: toNumber(p?.total_batches),
          success: toNumber(p?.completed_batches),
          failed: toNumber(p?.failed_batches),
        };
      })
      .filter((r: any) => !!r.rawDay);

    rows.sort((a: any, b: any) => Number(a.sortKey) - Number(b.sortKey));
    return rows;
  }, [batchPoints]);

  const dayOrder = useMemo(
    () => batchTrend7d.map((r: any) => r.day),
    [batchTrend7d]
  );

  const batchExecChartData = useMemo(() => {
    return batchTrend7d.flatMap((item: any) => [
      { day: item.day, type: batchLabel, value: item.total },
      { day: item.day, type: successLabel, value: item.success },
      { day: item.day, type: failedLabel, value: item.failed },
    ]);
  }, [batchTrend7d, batchLabel, successLabel, failedLabel]);

  const userStatusDistribution = useMemo(
    () => ({
      normal: normalUsers,
      frozen: frozenUsers,
      terminated: terminatedUsers,
      total: distTotal,
    }),
    [normalUsers, frozenUsers, terminatedUsers, distTotal]
  );

  const userStatusChartData = useMemo(
    () => [
      {
        type: t("pages.dashboard.distribution.user.normal", "正常"),
        value: userStatusDistribution.normal,
        color: "#22c55e",
      },
      {
        type: t("pages.dashboard.distribution.user.frozen", "冻结"),
        value: userStatusDistribution.frozen,
        color: "#f59e0b",
      },
      {
        type: t("pages.dashboard.distribution.user.terminated", "已终止"),
        value: userStatusDistribution.terminated,
        color: "#3b82f6",
      },
    ],
    [userStatusDistribution, t]
  );

  const withdrawalApprovalDistribution = useMemo(() => {
    const pending = toNumber(approvalDist.pending);
    const approved = toNumber(approvalDist.approved);
    const rejected = toNumber(approvalDist.rejected);
    const cancelled = toNumber(approvalDist.cancelled);
    return {
      pending,
      approved,
      rejected,
      cancelled,
      total: pending + approved + rejected + cancelled,
    };
  }, [approvalDist]);

  const withdrawalApprovalChartData = useMemo(
    () => [
      {
        type: t("pages.dashboard.distribution.withdrawal.pending", "待审批"),
        value: withdrawalApprovalDistribution.pending,
        color: "#f59e0b",
      },
      {
        type: t("pages.dashboard.distribution.withdrawal.approved", "已通过"),
        value: withdrawalApprovalDistribution.approved,
        color: "#22c55e",
      },
      {
        type: t("pages.dashboard.distribution.withdrawal.rejected", "已拒绝"),
        value: withdrawalApprovalDistribution.rejected,
        color: "#ef4444",
      },
      {
        type: t("pages.dashboard.distribution.withdrawal.cancelled", "已取消"),
        value: withdrawalApprovalDistribution.cancelled,
        color: "#9ca3af",
      },
    ],
    [withdrawalApprovalDistribution, t]
  );

  const vaultTokenBalances = useMemo(() => {
    return (vaultTop || []).slice(0, 5).map((v: any, idx: number) => {
      const pct = toPct(v?.percentage);
      return {
        rank: toNumber(v?.rank) || idx + 1,
        symbol: v?.currency || "",
        usd: toNumber(v?.balance_usd),
        pct: Number(pct.toFixed(2)),
      };
    });
  }, [vaultTop]);

  const iconConfig: Record<
    UiActivityType,
    { icon: React.ReactNode; style: string }
  > = {
    transfer_batch: {
      icon: <SendOutlined />,
      style: styles.activityIconTransfer,
    },
    withdrawal: {
      icon: <FileTextOutlined />,
      style: styles.activityIconWithdrawal,
    },
    user: { icon: <TeamOutlined />, style: styles.activityIconUser },
    vault: { icon: <WalletOutlined />, style: styles.activityIconVault },
  };

  const statusIcon: Record<ActivityLevel, React.ReactNode> = {
    success: (
      <CheckCircleOutlined
        className={`${styles.activityStatus} ${styles.activityStatusSuccess}`}
      />
    ),
    error: (
      <CloseCircleOutlined
        className={`${styles.activityStatus} ${styles.activityStatusError}`}
      />
    ),
    info: (
      <RiseOutlined
        className={`${styles.activityStatus} ${styles.activityStatusInfo}`}
      />
    ),
    warning: (
      <WarningOutlined
        className={`${styles.activityStatus} ${styles.activityStatusInfo}`}
      />
    ),
  };

  const fundFlowTooltip = useMemo(
    () => ({
      title: { field: "month" },
      items: [
        (d: any) => ({
          name: d?.type,
          value: Number(toNumber(d?.value)).toLocaleString(),
        }),
      ],
    }),
    []
  );

  const batchTooltip = useMemo(
    () => ({
      title: { field: "day" },
      items: [
        (d: any) => ({
          name: d?.type,
          value: Number(toNumber(d?.value)).toLocaleString(),
        }),
      ],
    }),
    []
  );

  const userStatusTooltip = useMemo(
    () => ({
      title: { field: "type" },
      items: [
        (d: any) => ({
          name: d?.type,
          value: Number(toNumber(d?.value)).toLocaleString(),
          color: d?.color,
        }),
      ],
    }),
    []
  );
  const colorDomain = batchTypeOrder;
  const colorRange = colorDomain.map((k) => batchColorMap[k] || "#3b82f6");
  const withdrawalTooltip = useMemo(
    () => ({
      title: { field: "type" },
      items: [
        (d: any) => ({
          name: d?.type,
          value: Number(toNumber(d?.value)).toLocaleString(),
          color: d?.color,
        }),
      ],
    }),
    []
  );

  return (
    <PageContainer
      className={styles.page}
      title={t("pages.dashboard.title", "Dashboard")}
    >
      <style>{motionStyles}</style>
      <Spin spinning={loading}>
        <div className={styles.topKpiRow}>
          <div
            className={`${styles.topKpiCard} ${styles.topKpiCardBlue}`}
            style={{ animationDelay: "0s" }}
          >
            <div className={styles.topKpiLeft}>
              <div className={styles.topKpiIconWrap}>
                <UserOutlined />
              </div>
              <div className={styles.topKpiLabel}>
                {t("pages.dashboard.kpi.web2_users", "Web2用户")}
              </div>
              <div className={styles.topKpiSublabel}>
                {t("pages.dashboard.kpi.totalRegistered", "总注册数")}
              </div>
            </div>
            <div className={styles.topKpiRight}>
              <div>
                <span className={styles.topKpiValue}>
                  {new Intl.NumberFormat(intl.locale).format(web2UsersValue)}
                </span>
                <span className={styles.topKpiUnit}>
                  {t("pages.dashboard.unit.person", "人")}
                </span>
              </div>
            </div>
          </div>

          <div
            className={`${styles.topKpiCard} ${styles.topKpiCardGreen}`}
            style={{ animationDelay: "0.05s" }}
          >
            <div className={styles.topKpiLeft}>
              <div className={styles.topKpiIconWrap}>
                <WalletOutlined />
              </div>
              <div className={styles.topKpiLabel}>
                {t("pages.dashboard.kpi.vault_balance", "Vault总余额")}
              </div>
              <div className={styles.topKpiSublabel}>
                {t("pages.dashboard.kpi.allCurrencies", "所有币种")}
              </div>
            </div>
            <div className={styles.topKpiRight}>
              <div>
                <span className={styles.topKpiValue}>
                  ${formatNumber(vaultBalanceValue)}
                </span>
              </div>
              <div className={styles.topKpiExtra}>
                ≈ {vaultBalanceValue.toLocaleString()} USD
              </div>
              {vaultBalanceDelta !== 0 && (
                <div className={styles.topKpiDelta}>
                  <RiseOutlined /> {vaultBalanceDelta > 0 ? "+" : ""}
                  {vaultBalanceDelta}%
                </div>
              )}
            </div>
          </div>

          <div
            className={`${styles.topKpiCard} ${styles.topKpiCardPurple}`}
            style={{ animationDelay: "0.1s" }}
          >
            <div className={styles.topKpiLeft}>
              <div className={styles.topKpiIconWrap}>
                <SendOutlined />
              </div>
              <div className={styles.topKpiLabel}>
                {t("pages.dashboard.kpi.today_transfers", "今日转账")}
              </div>
              <div className={styles.topKpiSublabel}>
                {t("pages.dashboard.kpi.transferCount", "{count} 笔", {
                  count: formatNumber(todayTransferCount),
                })}
              </div>
            </div>
            <div className={styles.topKpiRight}>
              <div>
                <span className={styles.topKpiValue}>
                  ${formatNumber(todayTransferAmount)}
                </span>
              </div>
              {todayTransferDelta !== 0 && (
                <div className={styles.topKpiDelta}>
                  <RiseOutlined /> {todayTransferDelta > 0 ? "+" : ""}
                  {todayTransferDelta}%
                </div>
              )}
            </div>
          </div>

          <div
            className={`${styles.topKpiCard} ${styles.topKpiCardOrange}`}
            style={{ animationDelay: "0.15s" }}
          >
            <div className={styles.topKpiLeft}>
              <div className={styles.topKpiIconWrap}>
                <AuditOutlined />
              </div>
              <div className={styles.topKpiLabel}>
                {t("pages.dashboard.kpi.pending_withdrawals", "待审批出金")}
              </div>
              <div className={styles.topKpiSublabel}>
                {t("pages.dashboard.kpi.needsReview", "需要处理")}
              </div>
            </div>
            <div className={styles.topKpiRight}>
              <div>
                <span className={styles.topKpiValue}>
                  {formatNumber(pendingWithdrawalsValue)}
                </span>
              </div>
              <div className={styles.topKpiExtra}>{pendingExtra}</div>
            </div>
          </div>
        </div>

        <div className={styles.secondaryRow}>
          <div
            className={styles.secondaryCard}
            style={{ animationDelay: "0.2s" }}
          >
            <div
              className={`${styles.secondaryIconWrap} ${styles.secondaryIconBlue}`}
            >
              <RiseOutlined />
            </div>
            <div className={styles.secondaryContent}>
              <div className={styles.secondaryLabel}>
                {t("pages.dashboard.stats.active7d", "活跃用户")}
              </div>
              <div>
                <span className={styles.secondaryValue}>
                  {formatNumber(active7d)}
                </span>
                <span className={styles.secondaryUnit}>
                  {t("pages.dashboard.unit.person", "人")}
                </span>
              </div>
              <div className={styles.secondarySub}>
                {t("pages.dashboard.stats.active7dDesc", "近7天活跃用户数")}
              </div>
            </div>
          </div>

          <div
            className={styles.secondaryCard}
            style={{ animationDelay: "0.25s" }}
          >
            <div
              className={`${styles.secondaryIconWrap} ${styles.secondaryIconPurple}`}
            >
              <DollarCircleOutlined />
            </div>
            <div className={styles.secondaryContent}>
              <div className={styles.secondaryLabel}>
                {t("pages.dashboard.stats.supportedTokens", "支持币种")}
              </div>
              <div>
                <span
                  className={styles.secondaryValue}
                  style={{ color: "#9333ea" }}
                >
                  {formatNumber(supportedTokens)}
                </span>
                <span className={styles.secondaryUnit}>
                  {t("pages.dashboard.unit.type", "种")}
                </span>
              </div>
              <div className={styles.secondarySub}>
                {t(
                  "pages.dashboard.stats.supportedTokensDesc",
                  "当前支持的主流币种数"
                )}
              </div>
            </div>
          </div>
        </div>

        <div className={styles.chartsRow}>
          <div className={styles.chartCard} style={{ animationDelay: "0.5s" }}>
            <div className={styles.chartHeader}>
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "flex-start",
                }}
              >
                <div>
                  <div className={styles.chartTitle}>
                    {t("pages.dashboard.charts.fundFlow.title", "资金流水趋势")}
                  </div>
                  <div className={styles.chartSubtitle}>
                    {t(
                      "pages.dashboard.charts.fundFlow.subtitle",
                      "近6个月入账、出账、内部转账统计"
                    )}
                  </div>
                </div>
                <div className={styles.chartLegend}>
                  {fundFlowTypeOrder.map((k) => (
                    <div className={styles.chartLegendItem} key={k}>
                      <span
                        className={styles.legendDot}
                        style={{ background: fundFlowColorMap[k] || "#3b82f6" }}
                      />
                      {k}
                    </div>
                  ))}
                </div>
              </div>
            </div>
            <Line
              data={fundFlowChartData}
              xField="month"
              yField="value"
              colorField="type"
              shapeField="smooth"
              height={280}
              legend={false}
              tooltip={fundFlowTooltip as any}
              point={{
                size: 4,
                shape: "circle",
                style: { stroke: "#fff", lineWidth: 2 },
              }}
              style={{ lineWidth: 3 }}
              scale={{
                x: { domain: monthOrder },
                color: {
                  domain: fundFlowTypeOrder,
                  range: fundFlowTypeOrder.map(
                    (k) => fundFlowColorMap[k] || "#3b82f6"
                  ),
                },
              }}
              axis={{
                x: { line: false, tickLine: false },
                y: {
                  grid: true,
                  gridStroke: "#f0f0f0",
                  gridLineDash: [4, 4],
                  labelFormatter: (v: any) => {
                    const num = Number(v);
                    if (!Number.isFinite(num)) return String(v);
                    return num >= 1000
                      ? `${(num / 1000).toFixed(0)}K`
                      : String(num);
                  },
                },
              }}
            />
          </div>

          <div className={styles.chartCard} style={{ animationDelay: "0.55s" }}>
            <div className={styles.chartHeader}>
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "flex-start",
                }}
              >
                <div>
                  <div className={styles.chartTitle}>
                    {t(
                      "pages.dashboard.charts.batchExec.title",
                      "转账批次执行趋势"
                    )}
                  </div>
                  <div className={styles.chartSubtitle}>
                    {t(
                      "pages.dashboard.charts.batchExec.subtitle",
                      "近7天批次执行与成功/失败统计"
                    )}
                  </div>
                </div>
                <div className={styles.chartLegend}>
                  {batchTypeOrder.map((k) => (
                    <div className={styles.chartLegendItem} key={k}>
                      <span
                        className={styles.legendDot}
                        style={{ background: batchColorMap[k] || "#3b82f6" }}
                      />
                      {k}
                    </div>
                  ))}
                </div>
              </div>
            </div>
            <Column
              data={batchExecChartData}
              xField="day"
              yField="value"
              seriesField="type"
              colorField="type"
              group
              height={280}
              legend={false}
              tooltip={batchTooltip as any}
              scale={{
                x: { domain: dayOrder },
                color: {
                  domain: colorDomain,
                  range: colorRange,
                },
              }}
              style={{
                radiusTopLeft: 6,
                radiusTopRight: 6,
              }}
              axis={{
                x: { line: false, tickLine: false, labelAutoHide: true },
                y: { grid: true, gridLineDash: [4, 4], gridStroke: "#f0f0f0" },
              }}
            />
          </div>
        </div>

        <div className={styles.distributionRow}>
          <div
            className={styles.distributionCard}
            style={{ animationDelay: "0.6s" }}
          >
            <div className={styles.distributionHeader}>
              <div className={styles.distributionTitle}>
                {t("pages.dashboard.distribution.user.title", "用户状态分布")}
              </div>
              <div className={styles.distributionSubtitle}>
                {t(
                  "pages.dashboard.distribution.user.subtitle",
                  "当前 {total} 个用户",
                  { total: userStatusDistribution.total }
                )}
              </div>
            </div>
            <div className={styles.pieChartWrapper}>
              <Pie
                data={userStatusChartData}
                angleField="value"
                colorField="type"
                radius={1}
                innerRadius={0.7}
                width={180}
                height={180}
                color={(d: any) => d?.color || "#3b82f6"}
                tooltip={userStatusTooltip as any}
                label={false as any}
                legend={false as any}
                statistic={{ title: false as any, content: false as any }}
              />
              <div className={styles.pieLegendList}>
                {userStatusChartData.map((item) => (
                  <div key={item.type} className={styles.pieLegendItem}>
                    <div className={styles.pieLegendLeft}>
                      <span
                        className={styles.pieLegendDot}
                        style={{ background: (item as any).color }}
                      />
                      <span className={styles.pieLegendLabel}>{item.type}</span>
                    </div>
                    <span className={styles.pieLegendValue}>
                      {Number((item as any).value || 0).toLocaleString()}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div
            className={styles.distributionCard}
            style={{ animationDelay: "0.65s" }}
          >
            <div className={styles.distributionHeader}>
              <div className={styles.distributionTitle}>
                {t(
                  "pages.dashboard.distribution.withdrawal.title",
                  "出金审批分布"
                )}
              </div>
              <div className={styles.distributionSubtitle}>
                {t(
                  "pages.dashboard.distribution.withdrawal.subtitle",
                  "总计 {total} 笔申请",
                  { total: withdrawalApprovalDistribution.total }
                )}
              </div>
            </div>
            <div className={styles.pieChartWrapper}>
              <Pie
                data={withdrawalApprovalChartData}
                angleField="value"
                colorField="type"
                radius={1}
                innerRadius={0.7}
                width={180}
                height={180}
                color={(d: any) => d?.color || "#3b82f6"}
                tooltip={withdrawalTooltip as any}
                label={false as any}
                legend={false as any}
                statistic={{ title: false as any, content: false as any }}
              />
              <div className={styles.pieLegendList}>
                {withdrawalApprovalChartData.map((item) => (
                  <div key={item.type} className={styles.pieLegendItem}>
                    <div className={styles.pieLegendLeft}>
                      <span
                        className={styles.pieLegendDot}
                        style={{ background: (item as any).color }}
                      />
                      <span className={styles.pieLegendLabel}>{item.type}</span>
                    </div>
                    <span className={styles.pieLegendValue}>
                      {Number((item as any).value || 0).toLocaleString()}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div
            className={styles.distributionCard}
            style={{ animationDelay: "0.7s" }}
          >
            <div className={styles.distributionHeader}>
              <div className={styles.distributionTitle}>
                {t("pages.dashboard.distribution.vault.title", "Vault币种余额")}
              </div>
              <div className={styles.distributionSubtitle}>
                {t(
                  "pages.dashboard.distribution.vault.subtitle",
                  "Top 5 币种分布"
                )}
              </div>
            </div>
            <div className={styles.vaultTokenList}>
              {vaultTokenBalances.map((token: any) => (
                <div
                  key={`${token.symbol}-${token.rank}`}
                  className={styles.vaultTokenItem}
                >
                  <div className={styles.vaultTokenRank}>{token.rank}</div>
                  <div className={styles.vaultTokenInfo}>
                    <div className={styles.vaultTokenTop}>
                      <span className={styles.vaultTokenSymbol}>
                        {token.symbol}
                      </span>
                      <span className={styles.vaultTokenUsd}>
                        ${formatNumber(token.usd)}
                      </span>
                    </div>
                    <div className={styles.vaultTokenBar}>
                      <div
                        className={styles.vaultTokenBarFill}
                        style={{
                          width: `${Math.max(0, Math.min(100, token.pct))}%`,
                        }}
                      />
                    </div>
                    <div className={styles.vaultTokenBottom}>
                      {token.pct}% · ${token.usd.toLocaleString()}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div
          className={styles.activityCard}
          style={{ animationDelay: "0.75s" }}
        >
          <div className={styles.activityHeader}>
            <div className={styles.activityHeaderLeft}>
              <div className={styles.activityHeaderTitle}>
                {t("pages.dashboard.activity.title", "最近操作动态")}
              </div>
              <div className={styles.activityHeaderSub}>
                {t("pages.dashboard.activity.subtitle", "实时系统操作记录")}
              </div>
            </div>
            <Link to="/audit/logs" className={styles.activityViewAll}>
              {t("pages.dashboard.activity.viewAll", "查看全部")}
            </Link>
          </div>
          <div className={styles.activityList}>
            {recentActivity.map((item) => {
              const config = iconConfig[item.type] || iconConfig.transfer_batch;
              return (
                <div key={item.id} className={styles.activityItem}>
                  <div className={`${styles.activityIconWrap} ${config.style}`}>
                    {config.icon}
                  </div>
                  <div className={styles.activityContent}>
                    <div className={styles.activityTitle}>
                      {maskPhoneInTitle(item.title)}
                    </div>
                    {item.description ? (
                      <div className={styles.activityDesc}>
                        {item.description}
                      </div>
                    ) : null}
                    <div className={styles.activityTime}>
                      {item.at
                        ? dayjs
                            .unix(Number(item.at))
                            .format("YYYY-MM-DD HH:mm:ss")
                        : "-"}
                    </div>
                  </div>
                  {statusIcon[item.level]}
                </div>
              );
            })}
          </div>
        </div>
      </Spin>
    </PageContainer>
  );
};

export default DashboardPage;
