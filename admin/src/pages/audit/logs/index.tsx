import { adminGetAuditLogs } from "@/api/generated/audit";
import { adminListRoles } from "@/api/generated/rbac";
import type {
  AdminAuditLogItem,
  AdminGetAuditLogsParams,
} from "@/api/generated/schemas";
import { useRbac } from "@/hooks/useRbac";
import { DownloadOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-components";
import { useIntl } from "@umijs/max";
import { App, Button, Drawer, Input, Pagination, Spin, Typography } from "antd";
import { createStyles } from "antd-style";
import React, { useCallback, useEffect, useMemo, useState } from "react";

import dayjs from "dayjs";
import timezone from "dayjs/plugin/timezone";
import utc from "dayjs/plugin/utc";
import FilterSection from "./components/FilterSection";
import LogTable from "./components/LogTable";
import StatsCards from "./components/StatsCards";
import { MODULE_CONFIG } from "./constants";
import type {
  AuditLogFilters,
  AuditLogRow,
  AuditLogStats,
  ModuleType,
} from "./types";

dayjs.extend(utc);
dayjs.extend(timezone);

const useStyles = createStyles(() => ({
  page: {
    background: "#f5f7fa",
    minHeight: "100vh",
  },
  exportBtn: {
    height: 40,
    borderRadius: 8,
    fontWeight: 500,
  },
  tableWrapper: {
    marginTop: 16,
  },
  pager: {
    display: "flex",
    justifyContent: "flex-end",
    padding: "16px 0",
  },
}));

function safeJson(value: unknown): string {
  try {
    return JSON.stringify(value ?? {}, null, 2);
  } catch {
    return "{}";
  }
}

function formatTime(iso?: string) {
  if (!iso) return "-";
  return dayjs.utc(iso).local().format("YYYY-MM-DD HH:mm:ss");
}

function normalizeModule(module: string | undefined): ModuleType {
  const v = (module || "").trim();
  const known: ModuleType[] = [
    "all",
    "account",
    "vault",
    "transfer",
    "user",
    "currency",
    "swap",
    "approval",
    "role",
    "blacklist",
    "system",
    "audit",
  ];
  if ((known as string[]).includes(v)) return v as ModuleType;
  return "system";
}

function toRow(item: AdminAuditLogItem): AuditLogRow {
  const operatorName =
    item.operator?.name ||
    item.operator?.username ||
    item.operator_email ||
    "-";
  const target =
    item.target_type && item.target_id
      ? `${item.target_type}:${item.target_id}`
      : item.target_id || item.target_type || "-";
  const module = normalizeModule(item.module);
  return {
    id: item.id || "",
    createdAt: formatTime(item.created_at) || "-",
    operator: {
      name: operatorName,
      username: item.operator?.username || undefined,
      role: item.operator?.role || undefined,
      email: item.operator_email || undefined,
    },
    module,
    action: item.description || item.action || "-",
    target,
    description: item.description || item.error_message || "-",
    status: item.success ? "success" : "failed",
    ip: item.ip || undefined,
    requestId: item.request_id || undefined,
    rpcMethod: item.rpc_method || undefined,
    httpMethod: item.http_method || undefined,
    httpPath: item.http_path || undefined,
    durationMs: item.duration_ms || undefined,
    errorMessage: item.error_message || undefined,
    details: (item.details as any) || undefined,
    raw: item,
  };
}

function downloadText(
  filename: string,
  text: string,
  mime = "text/plain;charset=utf-8"
) {
  const blob = new Blob([text], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

function toCsvValue(value: unknown): string {
  const raw = value == null ? "" : String(value);
  const escaped = raw.replace(/"/g, '""');
  return `"${escaped}"`;
}

const AuditLogsPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const { styles } = useStyles();

  const t = useCallback(
    (id: string, defaultMessage: string, values?: Record<string, any>) =>
      intl.formatMessage({ id, defaultMessage }, values),
    [intl]
  );

  const defaultFilters: AuditLogFilters = useMemo(
    () => ({
      keyword: "",
      module: "all",
      status: "all",
      dateRange: [null, null],
      operator: "",
      role: "",
      action: "",
      targetType: "",
      targetId: "",
      ip: "",
      requestId: "",
      httpMethod: "",
      httpPath: "",
      rpcMethod: "",
      durationMsFrom: null,
      durationMsTo: null,
    }),
    []
  );

  const [draftFilters, setDraftFilters] =
    useState<AuditLogFilters>(defaultFilters);
  const [appliedFilters, setAppliedFilters] =
    useState<AuditLogFilters>(defaultFilters);

  const [loading, setLoading] = useState(false);
  const [logs, setLogs] = useState<AuditLogRow[]>([]);
  const [stats, setStats] = useState<AuditLogStats>({
    total: 0,
    success: 0,
    failed: 0,
    today: 0,
  });
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [selected, setSelected] = useState<AuditLogRow | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const canView = useMemo(() => canRpc("GetAuditLogs"), [canRpc]);
  const canListRoles = useMemo(() => canRpc("ListRoles"), [canRpc]);

  const [roleOptions, setRoleOptions] = useState<
    { value: string; label: string }[]
  >([]);

  useEffect(() => {
    if (!canListRoles) return;
    const run = async () => {
      try {
        const res = await adminListRoles(
          { page: 1, page_size: 200, status: 1 },
          { skipErrorHandler: true }
        );
        if (!res?.success) return;
        const list = res.data?.list ?? [];
        setRoleOptions(
          list
            .map((r) => {
              const code = (r.code || "").trim();
              if (!code) return null;
              const name = (r.name || "").trim();
              return { value: code, label: name ? `${name} (${code})` : code };
            })
            .filter(Boolean) as { value: string; label: string }[]
        );
      } catch {}
    };
    void run();
  }, [canListRoles]);

  const params = useMemo(() => {
    const p: AdminGetAuditLogsParams = { page, page_size: pageSize };
    const keyword = appliedFilters.keyword.trim();
    if (keyword) p.keyword = keyword;
    const operator = appliedFilters.operator.trim();
    if (operator) p.operator = operator;
    const role = appliedFilters.role.trim();
    if (role) p.role = role;
    const action = appliedFilters.action.trim();
    if (action) p.action = action;
    const targetType = appliedFilters.targetType.trim();
    if (targetType) p.target_type = targetType;
    const targetId = appliedFilters.targetId.trim();
    if (targetId) p.target_id = targetId;
    const ip = appliedFilters.ip.trim();
    if (ip) p.ip = ip;
    const requestId = appliedFilters.requestId.trim();
    if (requestId) p.request_id = requestId;
    const httpMethod = appliedFilters.httpMethod.trim();
    if (httpMethod) p.http_method = httpMethod.toUpperCase();
    const httpPath = appliedFilters.httpPath.trim();
    if (httpPath) p.http_path = httpPath;
    const rpcMethod = appliedFilters.rpcMethod.trim();
    if (rpcMethod) p.rpc_method = rpcMethod;
    if (
      appliedFilters.durationMsFrom != null &&
      appliedFilters.durationMsFrom > 0
    ) {
      p.duration_ms_from = appliedFilters.durationMsFrom;
    }
    if (
      appliedFilters.durationMsTo != null &&
      appliedFilters.durationMsTo > 0
    ) {
      p.duration_ms_to = appliedFilters.durationMsTo;
    }
    if (appliedFilters.module && appliedFilters.module !== "all")
      p.module = appliedFilters.module;
    if (appliedFilters.status && appliedFilters.status !== "all")
      p.success = appliedFilters.status === "success";
    if (appliedFilters.dateRange[0]) p.date_from = appliedFilters.dateRange[0];
    if (appliedFilters.dateRange[1]) p.date_to = appliedFilters.dateRange[1];
    return p;
  }, [appliedFilters, page, pageSize]);

  const fetchLogs = useCallback(async () => {
    if (!canView) return;
    setLoading(true);
    try {
      const res = await adminGetAuditLogs(params, { skipErrorHandler: true });
      if (!res?.success)
        throw new Error(
          res?.message ||
            t(
              "pages.audit.logs.messages.loadFailed",
              "Failed to load audit logs"
            )
        );

      const items = res.data?.logs ?? [];
      const rows = (items as AdminAuditLogItem[]).map(toRow);
      setLogs(rows);

      const s = res.data?.stats;
      setStats({
        total: Number(s?.total ?? res.data?.pagination?.total ?? 0),
        success: Number(s?.success ?? 0),
        failed: Number(s?.failed ?? 0),
        today: Number(s?.today ?? 0),
      });
      setTotal(Number(res.data?.pagination?.total ?? s?.total ?? 0));
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.audit.logs.messages.loadFailed", "Failed to load audit logs")
      );
    } finally {
      setLoading(false);
    }
  }, [canView, message, params, t]);

  useEffect(() => {
    void fetchLogs();
  }, [fetchLogs]);

  const handleExport = () => {
    const header = [
      "created_at",
      "operator",
      "username",
      "role",
      "email",
      "module",
      "action",
      "target",
      "status",
      "ip",
      "duration_ms",
      "request_id",
      "http",
      "rpc_method",
      "description",
      "error_message",
    ];

    const lines = [header.map(toCsvValue).join(",")];
    for (const row of logs) {
      const http =
        row.httpMethod && row.httpPath
          ? `${row.httpMethod} ${row.httpPath}`
          : "";
      const moduleLabel = t(
        `pages.audit.logs.modules.${row.module}`,
        MODULE_CONFIG[row.module]?.label || row.module
      );
      lines.push(
        [
          row.createdAt,
          row.operator.name,
          row.operator.username || "",
          row.operator.role || "",
          row.operator.email || "",
          moduleLabel,
          row.action,
          row.target,
          row.status,
          row.ip || "",
          row.durationMs ?? "",
          row.requestId || "",
          http,
          row.rpcMethod || "",
          row.description,
          row.errorMessage || "",
        ]
          .map(toCsvValue)
          .join(",")
      );
    }
    downloadText(
      `audit-logs-page-${page}.csv`,
      lines.join("\n"),
      "text/csv;charset=utf-8"
    );
  };

  return (
    <PageContainer
      className={styles.page}
      header={{
        title: t("pages.audit.logs.title", "操作日志"),
        subTitle: t("pages.audit.logs.subtitle", "查看和审计系统所有操作记录"),
      }}
      extra={[
        <Button
          key="export"
          type="primary"
          icon={<DownloadOutlined />}
          className={styles.exportBtn}
          onClick={handleExport}
          disabled={!canView || logs.length === 0}
        >
          {t("pages.audit.logs.actions.export", "导出日志")}
        </Button>,
      ]}
    >
      <StatsCards stats={stats} t={t} />

      <FilterSection
        draftFilters={draftFilters}
        onDraftFiltersChange={setDraftFilters}
        onSearch={() => {
          const from = draftFilters.durationMsFrom;
          const to = draftFilters.durationMsTo;
          if (from != null && to != null && from > 0 && to > 0 && from > to) {
            message.error(
              t(
                "pages.audit.logs.messages.invalidDurationRange",
                "耗时范围不正确：最小值不能大于最大值"
              )
            );
            return;
          }
          setAppliedFilters(draftFilters);
          setPage(1);
        }}
        onReset={() => {
          setDraftFilters(defaultFilters);
          setAppliedFilters(defaultFilters);
          setPage(1);
        }}
        roleOptions={roleOptions}
        t={t}
      />

      <div className={styles.tableWrapper}>
        <Spin spinning={loading}>
          <LogTable
            logs={logs}
            t={t}
            onSelect={(row) => {
              setSelected(row);
              setDrawerOpen(true);
            }}
          />
        </Spin>
        <div className={styles.pager}>
          <Pagination
            current={page}
            pageSize={pageSize}
            total={total}
            showSizeChanger
            pageSizeOptions={[20, 50, 100, 200]}
            onChange={(p, ps) => {
              setPage(p);
              setPageSize(ps);
            }}
          />
        </div>
      </div>

      <Drawer
        title={t("pages.audit.logs.drawer.titleWithId", "Audit Log: {id}", {
          id: selected?.id || "-",
        })}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={720}
        destroyOnClose
      >
        {!selected ? (
          <Typography.Text type="secondary">
            {t("common.noData", "暂无数据")}
          </Typography.Text>
        ) : (
          <>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.createdAt", "Created At")}:
              </Typography.Text>{" "}
              {selected.createdAt}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.operator", "Operator")}:
              </Typography.Text>{" "}
              {selected.operator.name}
              {selected.operator.username
                ? ` (${selected.operator.username})`
                : ""}
              {selected.operator.role ? ` / ${selected.operator.role}` : ""}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.action", "Action")}:
              </Typography.Text>{" "}
              {selected.action}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.targetType", "Target Type")}:
              </Typography.Text>{" "}
              {selected.target}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.ip", "IP")}:
              </Typography.Text>{" "}
              {selected.ip || "-"}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.requestId", "Request ID")}:
              </Typography.Text>{" "}
              {selected.requestId || "-"}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.http", "HTTP")}:
              </Typography.Text>{" "}
              {selected.httpMethod && selected.httpPath
                ? `${selected.httpMethod} ${selected.httpPath}`
                : "-"}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.rpc", "RPC")}:
              </Typography.Text>{" "}
              {selected.rpcMethod || "-"}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.duration", "Duration")}:
              </Typography.Text>{" "}
              {selected.durationMs != null ? `${selected.durationMs}ms` : "-"}
            </Typography.Paragraph>
            <Typography.Paragraph>
              <Typography.Text strong>
                {t("pages.audit.logs.columns.status", "Status")}:
              </Typography.Text>{" "}
              {selected.status}
            </Typography.Paragraph>
            {selected.errorMessage ? (
              <Typography.Paragraph>
                <Typography.Text strong>
                  {t("pages.audit.logs.columns.errorMessage", "Error")}:
                </Typography.Text>{" "}
                {selected.errorMessage}
              </Typography.Paragraph>
            ) : null}

            <Typography.Title level={5} style={{ marginTop: 16 }}>
              {t("pages.audit.logs.drawer.rawPayload", "Raw payload")}
            </Typography.Title>
            <Input.TextArea
              value={safeJson(selected.raw)}
              readOnly
              autoSize={{ minRows: 14, maxRows: 28 }}
            />
          </>
        )}
      </Drawer>
    </PageContainer>
  );
};

export default AuditLogsPage;
