import { adminAccountingGetUserBalances } from "@/api/generated/accounting";
import { adminListChains, adminListCurrencies } from "@/api/generated/assets";
import type {
  AdminAccountingUserBalanceItem as AccountingUserBalanceItem,
  AdminAddUserNoteBody,
  AdminExportUsersFilters,
  AdminFreezeUserBody,
  AdminGetUserDetailData,
  AdminListUser2FAHistoryParams,
  AdminResetUserPasswordBody,
  AdminResetUserTradePasswordBody,
  AdminTerminateUserBody,
  AdminUnbindUserTotpBody,
  AdminUnfreezeUserBody,
  AdminUpdateUserMemberLevelBody,
  AdminUpdateUserRoleBody,
  AdminUpsertUserWithdrawAuditWhitelistRuleInput,
  AdminUpsertUserWithdrawAuditWhitelistRulesBody,
  AdminUser2FAHistoryItem,
  AdminUserListItem,
  AdminUsersStatisticsData,
} from "@/api/generated/schemas";
import {
  adminAddUserNote,
  adminExportUsers,
  adminFreezeUser,
  adminGetUserDetail,
  adminGetUsersStatistics,
  adminListUser2FAHistory,
  adminListUsers,
  adminListUserWithdrawAuditWhitelistRules,
  adminResetUserPassword,
  adminResetUserTradePassword,
  adminTerminateUser,
  adminUnbindUserTotp,
  adminUnfreezeUser,
  adminUpdateUserMemberLevel,
  adminUpdateUserRole,
  adminUpsertUserWithdrawAuditWhitelistRules,
} from "@/api/generated/users";
import { useRbac } from "@/hooks/useRbac";
import {
  AuditOutlined,
  DisconnectOutlined,
  HistoryOutlined,
  LockOutlined,
  MinusCircleOutlined,
  MobileOutlined,
  SafetyOutlined,
  SearchOutlined,
  SettingOutlined,
  StopOutlined,
  TeamOutlined,
  WalletOutlined,
} from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-components";
import {
  type ActionType,
  EditableProTable,
  ModalForm,
  PageContainer,
  type ProFormInstance,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from "@ant-design/pro-components";
import { useIntl } from "@umijs/max";

import {
  App,
  Button,
  Col,
  DatePicker,
  Divider,
  Drawer,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Steps,
  Tag,
  Typography,
} from "antd";

import { createStyles } from "antd-style";
import dayjs, { type Dayjs } from "dayjs";
import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

const useStyles = createStyles(() => ({
  statsRow: {
    display: "grid",
    gridTemplateColumns: "repeat(3, 1fr)",
    gap: 16,
    marginBottom: 16,
  },
  statCard: {
    borderRadius: 12,
    padding: "20px 24px",
    background: "#fff",
    border: "1px solid #f0f0f0",
    display: "flex",
    alignItems: "center",
    gap: 16,
  },
  statCardBlue: {
    background: "linear-gradient(135deg, #e0f2fe 0%, #bae6fd 100%)",
    border: "1px solid #7dd3fc",
  },
  statIconWrap: {
    width: 44,
    height: 44,
    borderRadius: 10,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    fontSize: 22,
  },
  statIconBlue: { background: "#3b82f6", color: "#fff" },
  statIconGray: { background: "#f3f4f6", color: "#6b7280" },
  statIconPurple: { background: "#8b5cf6", color: "#fff" },
  statIconGreen: { background: "#10b981", color: "#fff" },
  statContent: { flex: 1 },
  statLabel: { fontSize: 14, fontWeight: 500, color: "#374151" },
  statSub: { fontSize: 12, color: "#9ca3af", marginTop: 2 },
  statValue: {
    fontSize: 28,
    fontWeight: 700,
    color: "#1f2937",
    textAlign: "right" as const,
  },
  statUnit: { fontSize: 14, fontWeight: 400, color: "#6b7280", marginLeft: 4 },
  statPct: {
    fontSize: 12,
    color: "#9ca3af",
    textAlign: "right" as const,
    marginTop: 2,
  },
  whitelistCard: {
    borderRadius: 12,
    padding: "20px 24px",
    background: "linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%)",
    border: "1px solid #6ee7b7",
    display: "flex",
    alignItems: "center",
    gap: 16,
    marginBottom: 16,
  },
  searchCard: {
    borderRadius: 12,
    padding: "16px 20px",
    background: "#fff",
    border: "1px solid #f0f0f0",
    marginBottom: 16,
  },
  filtersCard: {
    borderRadius: 12,
    padding: "20px 24px",
    background: "#fff",
    border: "1px solid #f0f0f0",
    marginBottom: 16,
  },
  filtersRow: {
    display: "grid",
    gridTemplateColumns: "repeat(4, 1fr)",
    gap: 16,
  },
  filterItem: { display: "flex", flexDirection: "column" as const, gap: 8 },
  filterLabel: { fontSize: 13, color: "#6b7280" },
  resultCount: { fontSize: 14, color: "#6b7280", marginTop: 16 },
  resultHighlight: { color: "#3b82f6", fontWeight: 600 },
  tableCard: {
    borderRadius: 12,
    background: "#fff",
    border: "1px solid #f0f0f0",
    padding: "8px 16px 16px",
  },
}));

const PERM = {
  list: "ListUsers",
  stats: "GetUsersStatistics",
  detail: "GetUserDetail",
  twofaHistory: "ListUser2FAHistory",
  export: "ExportUsers",
  freeze: "FreezeUser",
  unfreeze: "UnfreezeUser",
  terminate: "TerminateUser",
  updateRole: "UpdateUserRole",
  updateMemberLevel: "UpdateUserMemberLevel",
  resetPassword: "ResetUserPassword",
  resetTradePassword: "ResetUserTradePassword",
  listWithdrawAuditWhitelistRules: "ListUserWithdrawAuditWhitelistRules",
  upsertWithdrawAuditWhitelistRules: "UpsertUserWithdrawAuditWhitelistRules",
  unbind2fa: "UnbindUserTotp",
  addNote: "AddUserNote",
  accountingGetBalances: "AccountingGetUserBalances",
};

const isPositiveIntString = (s: string) => /^[0-9]+$/.test(s) && s !== "0";
const Z_SETTINGS = 1000;
const Z_SUB = 1100;
const Z_SETTINGS_SUBMODAL = 1200;

const UsersPage: React.FC = () => {
  const [whitelistStep, setWhitelistStep] = useState<0 | 1>(0);
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const { styles } = useStyles();

  const t = useCallback(
    (id: string, defaultMessage: string, values?: Record<string, any>) =>
      intl.formatMessage({ id, defaultMessage }, values),
    [intl]
  );

  const [stats, setStats] = useState<AdminUsersStatisticsData | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);
  const [tableTotal, setTableTotal] = useState<number>(0);

  const [filterRole, setFilterRole] = useState<"all" | "user" | "vip">("all");
  const [filterStatus, setFilterStatus] = useState<
    "all" | "active" | "pending_kyc" | "frozen" | "terminated"
  >("all");
  const [filterWhitelist, setFilterWhitelist] = useState<
    "all" | "enabled" | "disabled"
  >("all");
  const [filter2FA, setFilter2FA] = useState<"all" | "enabled" | "disabled">(
    "all"
  );
  const [createdFrom, setCreatedFrom] = useState<string | undefined>(undefined);
  const [createdTo, setCreatedTo] = useState<string | undefined>(undefined);

  const [searchKeyword, setSearchKeyword] = useState("");

  const base64ToBlob = (b64: string, mime: string) => {
    const clean = (b64 || "")
      .replace(/^data:.*;base64,/, "")
      .replace(/\s/g, "")
      .replace(/-/g, "+")
      .replace(/_/g, "/");
    const binary = window.atob(clean);
    const len = binary.length;
    const bytes = new Uint8Array(len);
    for (let i = 0; i < len; i++) bytes[i] = binary.charCodeAt(i);
    return new Blob([bytes], { type: mime });
  };

  const buildExportFilters = useCallback(
    (sort?: any): AdminExportUsersFilters => {
      const keyword = searchKeyword.trim() || undefined;
      const status = filterStatus === "all" ? undefined : filterStatus;
      const role = filterRole === "all" ? undefined : filterRole;

      const bypass_withdraw_audit =
        filterWhitelist === "enabled"
          ? true
          : filterWhitelist === "disabled"
          ? false
          : undefined;

      const two_factor_enabled =
        filter2FA === "enabled"
          ? true
          : filter2FA === "disabled"
          ? false
          : undefined;

      const sortEntry = Object.entries(sort || {}).find(([, v]) => v);
      const sortKey = sortEntry?.[0];
      const sortValue = sortEntry?.[1];

      const sort_order =
        sortValue === "ascend"
          ? "asc"
          : sortValue === "descend"
          ? "desc"
          : undefined;

      const sort_by =
        sortKey === "created_at"
          ? "created_at"
          : sortKey === "last_login_at"
          ? "last_login_at"
          : undefined;

      return {
        keyword,
        status,
        role,
        created_from: createdFrom,
        created_to: createdTo,
        sort_by,
        sort_order,
        two_factor_enabled,
        bypass_withdraw_audit,
      };
    },
    [
      searchKeyword,
      filterStatus,
      filterRole,
      filterWhitelist,
      filter2FA,
      createdFrom,
      createdTo,
    ]
  );

  const downloadBlob = (blob: Blob, fileName: string) => {
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = fileName || "download";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  const userStatusValueEnum: Record<string, { text: string }> = {
    active: { text: t("pages.users.status.active", "Active") },
    frozen: { text: t("pages.users.status.frozen", "Frozen") },
    pending_kyc: { text: t("pages.users.status.pendingKyc", "Pending KYC") },
    terminated: { text: t("pages.users.status.terminated", "Terminated") },
  };

  const userRoleValueEnum: Record<string, { text: string }> = {
    user: { text: t("pages.users.roles.user", "User") },
    vip: { text: t("pages.users.roles.vip", "VIP") },
  };

  const userKycStatusValueEnum: Record<string, { text: string }> = {
    verified: { text: t("pages.users.kycStatus.verified", "Verified") },
    pending: { text: t("pages.users.kycStatus.pending", "Pending") },
    unverified: { text: t("pages.users.kycStatus.unverified", "Unverified") },
  };

  const enumText = useCallback(
    (valueEnum: Record<string, { text: string }>, value?: string) => {
      if (!value) return "-";
      return valueEnum[value]?.text ?? value;
    },
    []
  );
  const fmtDateTime = useCallback((v?: string | null) => {
    if (!v) return "-";
    const d = dayjs(v);
    return d.isValid() ? d.format("YYYY-MM-DD HH:mm:ss") : String(v);
  }, []);

  const closeSettings = useCallback(() => {
    setSettingsUser(null);
    setUnbind2faUser(null);
    setTwofaHistoryUid("");
    setWhitelistUser(null);
    setResetPasswordUser(null);
    setResetTradePasswordUser(null);
    setRoleUser(null);
    setMemberLevelUser(null);
    setTerminateUser(null);
    setNoteUser(null);
  }, []);

  const actionRef = useRef<ActionType | null>(null);

  const lastFiltersRef = useRef<AdminExportUsersFilters>({});
  const [exporting, setExporting] = useState(false);

  const [detailUid, setDetailUid] = useState<string>("");
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<AdminGetUserDetailData | null>(null);

  const [settingsUser, setSettingsUser] = useState<AdminUserListItem | null>(
    null
  );
  const [freezeUser, setFreezeUser] = useState<AdminUserListItem | null>(null);
  const [unfreezeUser, setUnfreezeUser] = useState<AdminUserListItem | null>(
    null
  );
  const [roleUser, setRoleUser] = useState<AdminUserListItem | null>(null);
  const [memberLevelUser, setMemberLevelUser] =
    useState<AdminUserListItem | null>(null);
  const [noteUser, setNoteUser] = useState<AdminUserListItem | null>(null);
  const [terminateUser, setTerminateUser] = useState<AdminUserListItem | null>(
    null
  );
  const [resetPasswordUser, setResetPasswordUser] =
    useState<AdminUserListItem | null>(null);
  const [resetTradePasswordUser, setResetTradePasswordUser] =
    useState<AdminUserListItem | null>(null);
  const [whitelistUser, setWhitelistUser] = useState<AdminUserListItem | null>(
    null
  );
  const [unbind2faUser, setUnbind2faUser] = useState<AdminUserListItem | null>(
    null
  );
  const [twofaHistoryUid, setTwofaHistoryUid] = useState<string>("");

  const [balancesUser, setBalancesUser] = useState<AdminUserListItem | null>(
    null
  );
  const [balancesLoading, setBalancesLoading] = useState(false);
  const [balanceRows, setBalanceRows] = useState<AccountingUserBalanceItem[]>(
    []
  );

  const whitelistRulesFormRef = useRef<
    ProFormInstance<AdminUpsertUserWithdrawAuditWhitelistRulesBody> | undefined
  >(undefined);
  const [whitelistRulesLoading, setWhitelistRulesLoading] = useState(false);
  const [editableRuleKeys, setEditableRuleKeys] = useState<React.Key[]>([]);
  const [chainOptions, setChainOptions] = useState<
    { label: string; value: string }[]
  >([]);
  const [assetOptions, setAssetOptions] = useState<
    { label: string; value: string }[]
  >([]);
  const [assetLoading, setAssetLoading] = useState(false);

  const [columnsStateMap, setColumnsStateMap] = useState<
    Record<string, { show?: boolean; fixed?: any; order?: number }>
  >({});

  const loadChains = useCallback(async () => {
    if (!canRpc(PERM.upsertWithdrawAuditWhitelistRules)) return;
    try {
      const res = await adminListChains(
        { include_disabled: false },
        { skipErrorHandler: true }
      );
      if (!res?.success) return;
      const list = res.data?.chains ?? [];
      const opts = list
        .map((c) => {
          const code = (c.chain_code || "").trim().toUpperCase();
          const network = (c.network || "").trim();
          if (!code) return null;
          return {
            value: code,
            label: network ? `${code} (${network})` : code,
          };
        })
        .filter(Boolean) as { value: string; label: string }[];
      setChainOptions(opts);
    } catch {
      return;
    }
  }, [canRpc]);

  const loadAssets = useCallback(
    async (keyword: string) => {
      if (!canRpc(PERM.upsertWithdrawAuditWhitelistRules)) return;
      const q = keyword.trim();
      setAssetLoading(true);
      try {
        const res = await adminListCurrencies(
          { page: 1, page_size: 50, status: 1, keyword: q || undefined },
          { skipErrorHandler: true }
        );
        if (!res?.success) return;
        const list = res.data?.currencies ?? [];
        const opts = list
          .map((c) => {
            const code = (c.asset_code || "").trim().toUpperCase();
            if (!code) return null;
            const name = (c.asset_name || "").trim();
            return { value: code, label: name ? `${code} - ${name}` : code };
          })
          .filter(Boolean) as { value: string; label: string }[];
        setAssetOptions(opts);
      } finally {
        setAssetLoading(false);
      }
    },
    [canRpc]
  );

  const refreshStats = useCallback(async () => {
    if (!canRpc(PERM.stats)) return;
    setStatsLoading(true);
    try {
      const res = await adminGetUsersStatistics({ skipErrorHandler: true });
      if (!res?.success)
        throw new Error(
          res?.message ||
            t(
              "pages.users.messages.statsLoadFailed",
              "Failed to load statistics"
            )
        );
      setStats(res.data ?? null);
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.users.messages.statsLoadFailed", "Failed to load statistics")
      );
    } finally {
      setStatsLoading(false);
    }
  }, [canRpc, message, t]);

  useEffect(() => {
    void refreshStats();
  }, [refreshStats]);

  useEffect(() => {
    actionRef.current?.reloadAndRest?.();
  }, [
    filterRole,
    filterStatus,
    filterWhitelist,
    filter2FA,
    createdFrom,
    createdTo,
  ]);

  useEffect(() => {
    const h = window.setTimeout(() => {
      actionRef.current?.reloadAndRest?.();
    }, 300);
    return () => window.clearTimeout(h);
  }, [searchKeyword]);

  const openDetail = useCallback(
    async (uid: string) => {
      if (!uid) return;
      setDetailUid(uid);
      setDetailLoading(true);
      setDetail(null);
      try {
        const res = await adminGetUserDetail(
          { uid },
          { skipErrorHandler: true }
        );
        if (!res?.success)
          throw new Error(
            res?.message ||
              t(
                "pages.users.messages.detailLoadFailed",
                "Failed to load user detail"
              )
          );
        setDetail(res.data ?? null);
      } catch (e: any) {
        message.error(
          e?.message ||
            t(
              "pages.users.messages.detailLoadFailed",
              "Failed to load user detail"
            )
        );
      } finally {
        setDetailLoading(false);
      }
    },
    [message, t]
  );

  const loadUserBalances = useCallback(
    async (uid: string) => {
      if (!canRpc(PERM.accountingGetBalances)) return;
      const uidStr = uid.trim();
      if (!isPositiveIntString(uidStr)) return;
      setBalancesLoading(true);
      try {
        const res = await adminAccountingGetUserBalances(
          { userId: uidStr },
          { skipErrorHandler: true }
        );
        if (!res?.success) throw new Error(res?.message || "");
        const payload = res?.data as any;
        const items = payload?.items ?? payload?.data?.items;
        setBalanceRows(Array.isArray(items) ? items : []);
      } catch (e: any) {
        message.error(
          e?.message ||
            t(
              "pages.accounting.userBalances.messages.loadFailed",
              "Failed to load"
            )
        );
      } finally {
        setBalancesLoading(false);
      }
    },
    [canRpc, message, t]
  );

  const openBalances = useCallback(
    (row: AdminUserListItem) => {
      if (!canRpc(PERM.accountingGetBalances)) return;
      const uid = (row?.uid || "").trim();
      if (!isPositiveIntString(uid)) return;
      setBalancesUser(row);
      setBalanceRows([]);
      void loadUserBalances(uid);
    },
    [canRpc, loadUserBalances]
  );

  const balanceColumns: ProColumns<AccountingUserBalanceItem>[] =
    useMemo(() => {
      return [
        {
          title: t("pages.accounting.userBalances.columns.asset", "Asset"),
          dataIndex: "asset_code",
          width: 120,
          render: (_, row) => (
            <Typography.Text code>{row.asset_code}</Typography.Text>
          ),
        },
        {
          title: t("pages.accounting.userBalances.columns.scale", "Scale"),
          dataIndex: "scale",
          width: 80,
        },
        {
          title: t(
            "pages.accounting.userBalances.columns.available",
            "Available"
          ),
          dataIndex: "available",
          width: 140,
          render: (_, row) => (
            <Typography.Text>{row.available}</Typography.Text>
          ),
        },
        {
          title: t("pages.accounting.userBalances.columns.locked", "Locked"),
          dataIndex: "locked",
          width: 140,
          render: (_, row) => <Typography.Text>{row.locked}</Typography.Text>,
        },
        {
          title: t(
            "pages.accounting.userBalances.columns.availableRaw",
            "Available (raw)"
          ),
          dataIndex: "available_raw",
          render: (_, row) => (
            <Typography.Text copyable>{row.available_raw}</Typography.Text>
          ),
        },
        {
          title: t(
            "pages.accounting.userBalances.columns.lockedRaw",
            "Locked (raw)"
          ),
          dataIndex: "locked_raw",
          render: (_, row) => (
            <Typography.Text copyable>{row.locked_raw}</Typography.Text>
          ),
        },
      ];
    }, [t]);

  useEffect(() => {
    const uid = whitelistUser?.uid;
    if (!uid) return;
    setWhitelistStep(0);
    void loadChains();
    void loadAssets("");

    setWhitelistRulesLoading(true);
    void (async () => {
      try {
        const res = await adminListUserWithdrawAuditWhitelistRules(
          { uid },
          { skipErrorHandler: true }
        );
        if (!res?.success)
          throw new Error(
            res?.message ||
              t(
                "pages.users.messages.whitelistRulesLoadFailed",
                "Failed to load whitelist rules"
              )
          );

        const normalizedRules: AdminUpsertUserWithdrawAuditWhitelistRuleInput[] =
          (res.data?.rules ?? []).map((r) => ({
            asset_code: r.asset_code || "",
            chain_code: r.chain_code || "",
            address: r.address || "",
            limit_usdt: r.limit_usdt || "0",
            enabled: r.enabled ?? true,
            source: r.source || "admin",
          }));

        const ruleRows = (normalizedRules || []).map((r, idx) => ({
          ...r,
          key: `${uid}-${idx}-${Date.now()}`,
        }));

        setEditableRuleKeys(
          ruleRows
            .filter(
              (r) => (r.source || "").toString().trim().toLowerCase() !== "user"
            )
            .map((r) => r.key as any)
        );

        whitelistRulesFormRef.current?.setFieldsValue({
          global_enabled: !!res.data?.global_enabled,
          rules: ruleRows as any,
          reason: "",
        } satisfies Partial<AdminUpsertUserWithdrawAuditWhitelistRulesBody>);
      } catch (e: any) {
        message.error(
          e?.message ||
            t(
              "pages.users.messages.whitelistRulesLoadFailed",
              "Failed to load whitelist rules"
            )
        );
        setEditableRuleKeys([]);
        whitelistRulesFormRef.current?.setFieldsValue({
          global_enabled: false,
          rules: [] as any,
          reason: "",
        } satisfies Partial<AdminUpsertUserWithdrawAuditWhitelistRulesBody>);
      } finally {
        setWhitelistRulesLoading(false);
      }
    })();
  }, [loadAssets, loadChains, message, t, whitelistUser]);

  const removeWhitelistRuleRow = useCallback((rowKey: React.Key) => {
    const currentRules = (whitelistRulesFormRef.current?.getFieldValue?.(
      "rules"
    ) ?? []) as any[];
    const current = (currentRules || []).find((r) => r?.key === rowKey);
    const src = (current?.source || "").toString().trim().toLowerCase();
    if (src === "user") return;
    const nextRules = (currentRules || []).filter((r) => r?.key !== rowKey);
    whitelistRulesFormRef.current?.setFieldsValue?.({ rules: nextRules });
    setEditableRuleKeys((prev) => prev.filter((k) => k !== rowKey));
  }, []);

  const columns: ProColumns<AdminUserListItem>[] = [
    {
      title: t("pages.users.columns.uid", "UID"),
      dataIndex: "uid",
      key: "uid",
      copyable: true,
      width: 140,
      exportField: "uid",
    },
    {
      title: t("pages.users.columns.email", "Email"),
      dataIndex: "email",
      key: "email",
      copyable: true,
      ellipsis: true,
      width: 240,
    },
    {
      title: t("pages.users.columns.phone", "Phone"),
      dataIndex: "phone",
      key: "phone",
      copyable: true,
      width: 150,
    },
    {
      title: t("pages.users.columns.name", "Name"),
      dataIndex: "name",
      key: "name",
      ellipsis: true,
      width: 160,
    },
    {
      title: t("pages.users.columns.role", "Role"),
      dataIndex: "role",
      key: "role",
      valueEnum: userRoleValueEnum,
      width: 120,
    },

    {
      title: t("pages.users.columns.twoFa", "2FA"),
      dataIndex: "two_factor_enabled",
      key: "two_factor_enabled",
      width: 120,
      render: (_, row) =>
        row.two_factor_enabled ? (
          <Tag color="green">{t("pages.users.twofa.totp", "TOTP")}</Tag>
        ) : (
          <Tag>{t("pages.users.twofa.disabled", "Disabled")}</Tag>
        ),
    },

    {
      title: t("pages.users.columns.whitelist", "Whitelist"),
      dataIndex: "bypass_withdraw_audit",
      key: "bypass_withdraw_audit",
      width: 140,
      render: (_, row) =>
        (row as any)?.bypass_withdraw_audit ? (
          <Tag color="green">
            {t("pages.users.filters.options.enabled", "Enabled")}
          </Tag>
        ) : (
          <Tag>{t("pages.users.filters.options.disabled", "Disabled")}</Tag>
        ),
    },

    {
      title: t("pages.users.columns.createdAt", "Created At"),
      dataIndex: "created_at",
      key: "created_at",
      valueType: "dateTime",
      width: 170,
    },
    {
      title: t("pages.users.columns.lastLogin", "Last Login"),
      dataIndex: "last_login_at",
      key: "last_login_at",
      valueType: "dateTime",
      width: 170,
    },
    {
      title: t("pages.users.columns.lastIp", "Last IP"),
      dataIndex: "last_login_ip",
      key: "last_login_ip",
      copyable: true,
      width: 140,
    },

    {
      title: t("pages.users.columns.actions", "Actions"),
      dataIndex: "__actions",
      key: "__actions",
      valueType: "option",
      width: 260,
      render: (_, row) => {
        const uid = row.uid;
        if (!uid) return null;
        const isFrozen = row.status === "frozen";
        const isTerminated = row.status === "terminated";
        return (
          <Space wrap>
            <Button
              size="small"
              disabled={!canRpc(PERM.detail)}
              onClick={() => void openDetail(uid)}
            >
              {t("pages.users.actions.detail", "Detail")}
            </Button>
            <Button
              size="small"
              icon={<WalletOutlined />}
              disabled={!canRpc(PERM.accountingGetBalances)}
              onClick={() => openBalances(row)}
            >
              {t("pages.users.actions.assets", "Assets")}
            </Button>
            {isFrozen ? (
              <Button
                size="small"
                disabled={!canRpc(PERM.unfreeze)}
                onClick={() => setUnfreezeUser(row)}
              >
                {t("pages.users.actions.unfreeze", "Unfreeze")}
              </Button>
            ) : !isTerminated ? (
              <Button
                size="small"
                danger
                disabled={!canRpc(PERM.freeze)}
                onClick={() => setFreezeUser(row)}
              >
                {t("pages.users.actions.freeze", "Freeze")}
              </Button>
            ) : null}
            <Button
              size="small"
              icon={<SettingOutlined />}
              onClick={() => setSettingsUser(row)}
            >
              {t("pages.users.actions.settings", "Settings")}
            </Button>
          </Space>
        );
      },
    },
  ];

  const getExportFieldsFromVisibleColumns = useCallback(() => {
    const excludedKeys = new Set(["__actions"]);
    const exportFieldAllowed = new Set<string>([
      "uid",
      "email",
      "phone",
      "name",
      "role",
      "status",
      "kyc_status",
      "two_factor_enabled",
      "created_at",
      "last_login_at",
      "last_login_ip",
      "total_balance_usd",
      "freeze_assets",
      "freeze_reason",
      "bypass_withdraw_audit",
    ]);

    const fields = (columns || [])
      .map((c) => {
        const di: any = (c as any)?.dataIndex;
        const keyRaw: any = (c as any)?.key;

        const columnKey =
          (typeof keyRaw === "string" && keyRaw.trim()) ||
          (typeof di === "string" && di.trim()) ||
          "";

        if (!columnKey) return null;
        if (excludedKeys.has(columnKey)) return null;
        if ((c as any)?.valueType === "option") return null;
        if ((c as any)?.hideInSetting === true) return null;
        if (columnsStateMap?.[columnKey]?.show === false) return null;

        const exportFieldRaw: any = (c as any)?.exportField;
        const exportField =
          (typeof exportFieldRaw === "string" && exportFieldRaw.trim()) ||
          (typeof di === "string" && di.trim()) ||
          "";

        if (!exportField) return null;
        if (!exportFieldAllowed.has(exportField)) return null;

        return exportField;
      })
      .filter(Boolean) as string[];

    return fields.length
      ? Array.from(new Set(fields))
      : [
          "uid",
          "email",
          "phone",
          "name",
          "role",
          "two_factor_enabled",
          "bypass_withdraw_audit",
          "created_at",
          "last_login_at",
          "last_login_ip",
        ];
  }, [columns, columnsStateMap]);

  const detailTitle = useMemo(() => {
    const uid = detail?.basic_info?.uid || detailUid;
    const email = detail?.basic_info?.email;
    return email
      ? t("pages.users.detail.titleWithEmail", "User: {uid} ({email})", {
          uid,
          email,
        })
      : t("pages.users.detail.title", "User: {uid}", { uid: uid || "-" });
  }, [detail, detailUid, t]);

  const statsNums = useMemo(() => {
    const total = Number(stats?.total_users ?? 0);
    const web2 = Number(stats?.web2_total ?? 0);
    const web3 = Number(stats?.web3_total ?? 0);
    const whitelist = Number(stats?.whitelist_total ?? 0);
    return { total, web2, web3, whitelist };
  }, [stats]);

  const onCreatedRangeChange = useCallback((v: null | [Dayjs, Dayjs]) => {
    if (!v || !v[0] || !v[1]) {
      setCreatedFrom(undefined);
      setCreatedTo(undefined);
      return;
    }
    setCreatedFrom(v[0].startOf("day").format("YYYY-MM-DD"));
    setCreatedTo(v[1].endOf("day").format("YYYY-MM-DD"));
  }, []);

  return (
    <PageContainer
      header={{
        title: t("pages.users.title", "Users"),
        subTitle: t("pages.users.subtitle", "Manage platform users"),
      }}
    >
      <div className={styles.statsRow}>
        <div className={`${styles.statCard} ${styles.statCardBlue}`}>
          <div className={`${styles.statIconWrap} ${styles.statIconBlue}`}>
            <TeamOutlined />
          </div>
          <div className={styles.statContent}>
            <div className={styles.statLabel}>
              {t("pages.users.stats.total.label", "Total users")}
            </div>
            <div className={styles.statSub}>
              {t("pages.users.stats.total.sub", "All registered users")}
            </div>
          </div>
          <div>
            <div className={styles.statValue}>
              {statsLoading ? "-" : stats ? statsNums.total : "-"}
              <span className={styles.statUnit}>
                {t("pages.users.stats.unitUsers", "users")}
              </span>
            </div>
          </div>
        </div>

        <div className={styles.statCard}>
          <div className={`${styles.statIconWrap} ${styles.statIconGray}`}>
            <MobileOutlined />
          </div>
          <div className={styles.statContent}>
            <div className={styles.statLabel}>
              {t("pages.users.stats.web2.label", "Web2 users")}
            </div>
            <div className={styles.statSub}>
              {t("pages.users.stats.web2.sub", "Email/phone login")}
            </div>
          </div>
          <div>
            <div className={styles.statValue}>
              {statsLoading ? "-" : stats ? statsNums.web2 : "-"}
            </div>
            <div className={styles.statPct}>
              {t("pages.users.stats.sharePrefix", "Share")}{" "}
              {statsLoading || !stats
                ? "-"
                : `${
                    statsNums.total
                      ? ((statsNums.web2 / statsNums.total) * 100).toFixed(1)
                      : 0
                  }%`}
            </div>
          </div>
        </div>

        <div className={styles.statCard}>
          <div className={`${styles.statIconWrap} ${styles.statIconPurple}`}>
            <WalletOutlined />
          </div>
          <div className={styles.statContent}>
            <div className={styles.statLabel}>
              {t("pages.users.stats.web3.label", "Web3 users")}
            </div>
            <div className={styles.statSub}>
              {t("pages.users.stats.web3.sub", "Wallet login")}
            </div>
          </div>
          <div>
            <div className={styles.statValue}>
              {statsLoading ? "-" : stats ? statsNums.web3 : "-"}
            </div>
            <div className={styles.statPct}>
              {t("pages.users.stats.sharePrefix", "Share")}{" "}
              {statsLoading || !stats
                ? "-"
                : `${
                    statsNums.total
                      ? ((statsNums.web3 / statsNums.total) * 100).toFixed(1)
                      : 0
                  }%`}
            </div>
          </div>
        </div>
      </div>

      <div className={styles.whitelistCard}>
        <div className={`${styles.statIconWrap} ${styles.statIconGreen}`}>
          <SafetyOutlined />
        </div>
        <div className={styles.statContent}>
          <div className={styles.statLabel}>
            {t("pages.users.stats.whitelist.label", "Whitelist users")}
          </div>
          <div className={styles.statSub}>
            {t(
              "pages.users.stats.whitelist.sub",
              "Users with whitelist enabled will bypass withdraw audit"
            )}
          </div>
        </div>
        <div>
          <div className={styles.statValue}>
            {statsLoading ? "-" : stats ? statsNums.whitelist : "-"}
            <span className={styles.statUnit}>
              {t("pages.users.stats.unitUsers", "users")}
            </span>
          </div>
        </div>
      </div>

      <div className={styles.searchCard}>
        <Input
          size="large"
          placeholder={t(
            "pages.users.search.placeholder",
            "Search UID, phone, or email..."
          )}
          prefix={<SearchOutlined style={{ color: "#9ca3af" }} />}
          style={{ borderRadius: 8, height: 44 }}
          value={searchKeyword}
          onChange={(e) => setSearchKeyword(e.target.value)}
          allowClear
        />
      </div>

      <div className={styles.filtersCard}>
        <div className={styles.filtersRow}>
          <div className={styles.filterItem}>
            <div className={styles.filterLabel}>
              {t("pages.users.filters.status.label", "Account status")}
            </div>
            <Select
              style={{ width: "100%" }}
              value={filterStatus}
              onChange={setFilterStatus}
              options={[
                {
                  label: t("pages.users.filters.options.all", "All"),
                  value: "all",
                },
                {
                  label: t("pages.users.status.active", "Active"),
                  value: "active",
                },
                {
                  label: t("pages.users.status.frozen", "Frozen"),
                  value: "frozen",
                },
                {
                  label: t("pages.users.status.terminated", "Terminated"),
                  value: "terminated",
                },
              ]}
            />
          </div>

          <div className={styles.filterItem}>
            <div className={styles.filterLabel}>
              {t("pages.users.filters.whitelist.label", "Whitelist")}
            </div>
            <Select
              style={{ width: "100%" }}
              value={filterWhitelist}
              onChange={setFilterWhitelist}
              options={[
                {
                  label: t("pages.users.filters.options.all", "All"),
                  value: "all",
                },
                {
                  label: t("pages.users.filters.options.enabled", "Enabled"),
                  value: "enabled",
                },
                {
                  label: t("pages.users.filters.options.disabled", "Disabled"),
                  value: "disabled",
                },
              ]}
            />
          </div>

          <div className={styles.filterItem}>
            <div className={styles.filterLabel}>
              {t("pages.users.filters.twofa.label", "2FA")}
            </div>
            <Select
              style={{ width: "100%" }}
              value={filter2FA}
              onChange={setFilter2FA}
              options={[
                {
                  label: t("pages.users.filters.options.all", "All"),
                  value: "all",
                },
                {
                  label: t("pages.users.filters.options.enabled", "Enabled"),
                  value: "enabled",
                },
                {
                  label: t("pages.users.filters.options.disabled", "Disabled"),
                  value: "disabled",
                },
              ]}
            />
          </div>

          <div className={styles.filterItem}>
            <div className={styles.filterLabel}>
              {t("pages.users.filters.createdAt.label", "Created at")}
            </div>
            <DatePicker.RangePicker
              style={{ width: "100%" }}
              allowClear
              value={
                createdFrom && createdTo
                  ? [
                      dayjs(createdFrom, "YYYY-MM-DD"),
                      dayjs(createdTo, "YYYY-MM-DD"),
                    ]
                  : null
              }
              onChange={(v) => onCreatedRangeChange(v as any)}
            />
          </div>
        </div>

        <div className={styles.resultCount}>
          {t("pages.users.filters.resultCount.prefix", "Found")}{" "}
          <span className={styles.resultHighlight}>{tableTotal}</span>{" "}
          {t("pages.users.filters.resultCount.suffix", "users")}
        </div>
      </div>

      <div className={styles.tableCard}>
        <ProTable<AdminUserListItem>
          actionRef={actionRef}
          rowKey={(row) => row.uid || row.email || JSON.stringify(row)}
          columns={columns}
          columnsState={{
            value: columnsStateMap as any,
            onChange: (v) => setColumnsStateMap(v as any),
          }}
          scroll={{ x: "max-content" }}
          search={false}
          request={async (params, sort) => {
            if (!canRpc(PERM.list)) {
              setTableTotal(0);
              return { success: true, data: [], total: 0 };
            }

            const filters = buildExportFilters(sort);
            lastFiltersRef.current = filters;

            try {
              const res = await adminListUsers(
                {
                  page: params.current,
                  page_size: params.pageSize,
                  ...filters,
                } as any,
                { skipErrorHandler: true }
              );
              const total = Number(res.data?.pagination?.total ?? 0);
              setTableTotal(total);
              return {
                success: !!res?.success,
                data: res.data?.users ?? [],
                total,
              };
            } catch (e: any) {
              message.error(
                e?.message ||
                  t("pages.users.messages.loadFailed", "Failed to load users")
              );
              setTableTotal(0);
              return { success: false, data: [], total: 0 };
            }
          }}
          toolbar={{
            actions: [
              <Button
                key="export"
                disabled={!canRpc(PERM.export)}
                loading={exporting}
                onClick={() => {
                  if (!canRpc(PERM.export)) return;
                  setExporting(true);
                  void (async () => {
                    try {
                      const clean = (obj: any) =>
                        Object.fromEntries(
                          Object.entries(obj || {}).filter(
                            ([, v]) => v !== undefined
                          )
                        );

                      const exportFilters = clean(lastFiltersRef.current);
                      const fields = getExportFieldsFromVisibleColumns().filter(
                        (f) => f !== "status"
                      );
                      const res = await adminExportUsers(
                        { filters: exportFilters, fields, format: "xlsx" },
                        { skipErrorHandler: true }
                      );

                      if (!res?.success)
                        throw new Error(
                          res?.message ||
                            t(
                              "pages.users.messages.exportFailed",
                              "Export failed"
                            )
                        );

                      const payload = res.data as any;
                      const data = payload?.data ?? payload;

                      const content = data?.content || "";
                      const fileName =
                        data?.file_name || `users_${Date.now()}.xlsx`;

                      const fmt = String(data?.format || "csv").toLowerCase();
                      const mime =
                        fmt === "csv"
                          ? "text/csv;charset=utf-8"
                          : fmt === "xlsx"
                          ? "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                          : "application/octet-stream";

                      if (!content)
                        throw new Error(
                          t(
                            "pages.users.messages.exportFailed",
                            "Export failed"
                          )
                        );

                      const blob = base64ToBlob(content, mime);
                      downloadBlob(blob, fileName);
                      message.success(
                        t(
                          "pages.users.messages.exportSuccess",
                          "Export success"
                        )
                      );
                    } catch (e: any) {
                      message.error(
                        e?.message ||
                          t(
                            "pages.users.messages.exportFailed",
                            "Export failed"
                          )
                      );
                    } finally {
                      setExporting(false);
                    }
                  })();
                }}
              >
                {t("pages.users.actions.export", "Export")}
              </Button>,
            ],
          }}
        />
      </div>

      <Modal
        title={
          balancesUser?.uid
            ? t("pages.users.assets.titleWithUid", "Assets: {uid}", {
                uid: balancesUser.uid,
              })
            : t("pages.users.assets.title", "Assets")
        }
        open={!!balancesUser}
        onCancel={() => {
          setBalancesUser(null);
          setBalanceRows([]);
        }}
        footer={null}
        width={760}
        destroyOnClose
      >
        <Space direction="vertical" style={{ width: "100%" }} size={12}>
          <div
            style={{
              padding: 12,
              borderRadius: 8,
              background: "rgba(0,0,0,0.02)",
            }}
          >
            <Space size={12} wrap>
              <Typography.Text strong>
                {t("pages.users.detail.uid", "UID")}: {balancesUser?.uid || "-"}
              </Typography.Text>
              <Typography.Text type="secondary">
                {t("pages.users.detail.email", "Email")}:{" "}
                {balancesUser?.email || "-"}
              </Typography.Text>
              <Typography.Text type="secondary">
                {t("pages.users.detail.phone", "Phone")}:{" "}
                {balancesUser?.phone || "-"}
              </Typography.Text>
            </Space>
          </div>

          <Space wrap>
            <Button
              type="primary"
              loading={balancesLoading}
              disabled={
                !balancesUser?.uid || !canRpc(PERM.accountingGetBalances)
              }
              onClick={() =>
                void loadUserBalances((balancesUser?.uid || "").trim())
              }
            >
              {t("pages.accounting.userBalances.actions.refresh", "Refresh")}
            </Button>
            <Button
              disabled={!isPositiveIntString((balancesUser?.uid || "").trim())}
              onClick={() => {
                const uid = (balancesUser?.uid || "").trim();
                if (!isPositiveIntString(uid)) return;
                window.open(
                  `/accounting/user-ledger?user_id=${encodeURIComponent(uid)}`,
                  "_blank",
                  "noopener,noreferrer"
                );
              }}
            >
              {t("pages.accounting.userBalances.actions.ledger", "Ledger")}
            </Button>
          </Space>

          <ProTable<AccountingUserBalanceItem>
            rowKey={(row) => `${row.asset_code}`}
            search={false}
            loading={balancesLoading}
            dataSource={balanceRows}
            pagination={false}
            columns={balanceColumns}
            options={false}
            toolBarRender={false}
          />
        </Space>
      </Modal>

      <Modal
        title={
          settingsUser?.uid
            ? t("pages.users.settings.titleWithUid", "User Settings: {uid}", {
                uid: settingsUser.uid,
              })
            : t("pages.users.settings.title", "User Settings")
        }
        open={!!settingsUser}
        onCancel={closeSettings}
        footer={null}
        width={760}
        destroyOnClose
        zIndex={Z_SETTINGS}
      >
        <Space direction="vertical" style={{ width: "100%" }} size={16}>
          <div
            style={{
              padding: 12,
              borderRadius: 8,
              background: "rgba(0,0,0,0.02)",
            }}
          >
            <Space size={12} wrap>
              <Typography.Text strong>
                {t("pages.users.detail.uid", "UID")}: {settingsUser?.uid || "-"}
              </Typography.Text>
              <Typography.Text type="secondary">
                {t("pages.users.detail.email", "Email")}:{" "}
                {settingsUser?.email || "-"}
              </Typography.Text>
              <Typography.Text type="secondary">
                {t("pages.users.detail.phone", "Phone")}:{" "}
                {settingsUser?.phone || "-"}
              </Typography.Text>
            </Space>
          </div>

          <div>
            <Divider orientation="left" plain style={{ margin: 0 }}>
              {t("pages.users.settings.sections.security", "Security")}
            </Divider>
            <div style={{ marginTop: 12 }}>
              <Row gutter={[12, 12]}>
                <Col span={12}>
                  <Space
                    direction="vertical"
                    style={{ width: "100%" }}
                    size={4}
                  >
                    <Button
                      block
                      danger
                      icon={<LockOutlined />}
                      disabled={!canRpc(PERM.resetTradePassword)}
                      onClick={() => {
                        const u = settingsUser;
                        if (!u) return;
                        setResetTradePasswordUser(u);
                      }}
                    >
                      {t(
                        "pages.users.actions.resetTradePassword",
                        "Change Trade Password"
                      )}
                    </Button>
                    <Typography.Text type="secondary">
                      {t(
                        "pages.users.settings.desc.resetTradePassword",
                        "Change the trade password"
                      )}
                    </Typography.Text>
                  </Space>
                </Col>

                <Col span={12}>
                  <Space
                    direction="vertical"
                    style={{ width: "100%" }}
                    size={4}
                  >
                    <Button
                      block
                      danger
                      icon={<DisconnectOutlined />}
                      disabled={!canRpc(PERM.unbind2fa)}
                      onClick={() => {
                        const u = settingsUser;
                        if (!u) return;
                        setUnbind2faUser(u);
                      }}
                    >
                      {t("pages.users.actions.unbind2fa", "Unbind 2FA")}
                    </Button>
                    <Typography.Text type="secondary">
                      {t(
                        "pages.users.settings.desc.unbind2fa",
                        "Unbind TOTP for the user"
                      )}
                    </Typography.Text>
                  </Space>
                </Col>

                <Col span={12}>
                  <Space
                    direction="vertical"
                    style={{ width: "100%" }}
                    size={4}
                  >
                    <Button
                      block
                      icon={<HistoryOutlined />}
                      disabled={!canRpc(PERM.twofaHistory)}
                      onClick={() => {
                        const u = settingsUser;
                        if (!u?.uid) return;
                        setTwofaHistoryUid(u.uid);
                      }}
                    >
                      {t("pages.users.actions.twofaHistory", "2FA History")}
                    </Button>
                    <Typography.Text type="secondary">
                      {t(
                        "pages.users.settings.desc.twofaHistory",
                        "View 2FA bind/unbind history"
                      )}
                    </Typography.Text>
                  </Space>
                </Col>
              </Row>
            </div>
          </div>

          <div>
            <Divider orientation="left" plain style={{ margin: 0 }}>
              {t("pages.users.settings.sections.controls", "Controls")}
            </Divider>
            <div style={{ marginTop: 12 }}>
              <Row gutter={[12, 12]}>
                <Col span={12}>
                  <Space
                    direction="vertical"
                    style={{ width: "100%" }}
                    size={4}
                  >
                    <Button
                      block
                      icon={<AuditOutlined />}
                      disabled={
                        !canRpc(PERM.upsertWithdrawAuditWhitelistRules) &&
                        !canRpc(PERM.listWithdrawAuditWhitelistRules)
                      }
                      onClick={() => {
                        const u = settingsUser;
                        if (!u) return;
                        setWhitelistUser(u);
                      }}
                    >
                      {t(
                        "pages.users.actions.withdrawAuditWhitelist",
                        "Withdraw Audit Whitelist"
                      )}
                    </Button>
                    <Typography.Text type="secondary">
                      {t(
                        "pages.users.settings.desc.withdrawAuditWhitelist",
                        "Bypass withdraw audit for this user"
                      )}
                    </Typography.Text>
                  </Space>
                </Col>
                {/*
                <Col span={12}>
                  <Space
                    direction="vertical"
                    style={{ width: "100%" }}
                    size={4}
                  >
                    <Button
                      block
                      icon={<FormOutlined />}
                      disabled={!canRpc(PERM.addNote)}
                      onClick={() => {
                        const u = settingsUser;
                        if (!u) return;
                        setNoteUser(u);
                      }}
                    >
                      {t("pages.users.actions.note", "Note")}
                    </Button>
                    <Typography.Text type="secondary">
                      {t(
                        "pages.users.settings.desc.note",
                        "Add internal admin notes"
                      )}
                    </Typography.Text>
                  </Space>
                </Col> */}
              </Row>
            </div>
          </div>

          <div>
            <Divider orientation="left" plain style={{ margin: 0 }}>
              {t("pages.users.settings.sections.risk", "Risk")}
            </Divider>
            <div style={{ marginTop: 12 }}>
              <Row gutter={[12, 12]}>
                <Col span={12}>
                  <Space
                    direction="vertical"
                    style={{ width: "100%" }}
                    size={4}
                  >
                    <Button
                      block
                      danger
                      icon={<StopOutlined />}
                      disabled={!canRpc(PERM.terminate)}
                      onClick={() => {
                        const u = settingsUser;
                        if (!u) return;
                        setTerminateUser(u);
                      }}
                    >
                      {t("pages.users.actions.terminate", "Terminate")}
                    </Button>
                    <Typography.Text type="secondary">
                      {t(
                        "pages.users.settings.desc.terminate",
                        "Irreversible: terminate the user"
                      )}
                    </Typography.Text>
                  </Space>
                </Col>
              </Row>
            </div>
          </div>
        </Space>
      </Modal>

      <Drawer
        title={detailTitle}
        width={720}
        open={!!detailUid}
        onClose={() => {
          setDetailUid("");
          setDetail(null);
          setDetailLoading(false);
        }}
      >
        {detailLoading ? (
          <Typography.Text>
            {t("pages.users.detail.loading", "Loading...")}
          </Typography.Text>
        ) : (
          <Space direction="vertical" style={{ width: "100%" }} size="middle">
            <div>
              <Typography.Title level={5} style={{ marginTop: 0 }}>
                {t("pages.users.detail.basicTitle", "Basic")}
              </Typography.Title>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "160px 1fr",
                  rowGap: 8,
                }}
              >
                <div>{t("pages.users.detail.uid", "UID")}</div>
                <div>{detail?.basic_info?.uid || "-"}</div>
                <div>{t("pages.users.detail.email", "Email")}</div>
                <div>{detail?.basic_info?.email || "-"}</div>
                <div>{t("pages.users.detail.phone", "Phone")}</div>
                <div>{detail?.basic_info?.phone || "-"}</div>
                <div>{t("pages.users.detail.name", "Name")}</div>
                <div>{detail?.basic_info?.name || "-"}</div>
                <div>{t("pages.users.detail.status", "Status")}</div>
                <div>
                  {enumText(userStatusValueEnum, detail?.basic_info?.status)}
                </div>
                <div>{t("pages.users.detail.role", "Role")}</div>
                <div>
                  {enumText(userRoleValueEnum, detail?.basic_info?.role)}
                </div>
                <div>{t("pages.users.detail.createdAt", "Created At")}</div>
                <div>{fmtDateTime(detail?.basic_info?.created_at)}</div>
              </div>
            </div>

            <div>
              <Typography.Title level={5}>
                {t("pages.users.detail.securityTitle", "Security")}
              </Typography.Title>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "160px 1fr",
                  rowGap: 8,
                }}
              >
                <div>{t("pages.users.detail.twoFaEnabled", "2FA Enabled")}</div>
                <div>
                  {detail?.security_info?.two_factor_enabled
                    ? t("pages.users.values.yes", "Yes")
                    : t("pages.users.values.no", "No")}
                </div>
                <div>{t("pages.users.detail.twoFaType", "2FA Type")}</div>
                <div>{detail?.security_info?.two_factor_type || "-"}</div>
                <div>
                  {t("pages.users.detail.googleAuthBound", "Google Auth Bound")}
                </div>
                <div>
                  {detail?.security_info?.google_auth_bound
                    ? t("pages.users.values.yes", "Yes")
                    : t("pages.users.values.no", "No")}
                </div>

                <div>{t("pages.users.detail.lastLogin", "Last Login")}</div>
                <div>{fmtDateTime(detail?.security_info?.last_login_at)}</div>
                <div>
                  {t("pages.users.detail.lastLoginIp", "Last Login IP")}
                </div>
                <div>{detail?.security_info?.last_login_ip || "-"}</div>
                <div>
                  {t(
                    "pages.users.detail.withdrawAuditBypass",
                    "Withdraw Audit Bypass"
                  )}
                </div>
                <div>
                  {detail?.bypass_withdraw_audit
                    ? t("pages.users.values.yes", "Yes")
                    : t("pages.users.values.no", "No")}
                </div>
              </div>
            </div>
            {/*
            <div>
              <Typography.Title level={5}>
                {t("pages.users.detail.kycTitle", "KYC")}
              </Typography.Title>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "160px 1fr",
                  rowGap: 8,
                }}
              >
                <div>{t("pages.users.detail.kycStatus", "Status")}</div>
                <div>
                  {enumText(userKycStatusValueEnum, detail?.kyc_info?.status)}
                </div>
                <div>{t("pages.users.detail.kycLevel", "Level")}</div>
                <div>{detail?.kyc_info?.level ?? "-"}</div>
                <div>{t("pages.users.detail.verifiedAt", "Verified At")}</div>
                <div>{detail?.kyc_info?.verified_at || "-"}</div>
              </div>
            </div> */}
          </Space>
        )}
      </Drawer>

      <Drawer
        title={
          twofaHistoryUid
            ? t("pages.users.twofaHistory.titleWithUid", "2FA History: {uid}", {
                uid: twofaHistoryUid,
              })
            : t("pages.users.twofaHistory.title", "2FA History")
        }
        width={720}
        open={!!twofaHistoryUid}
        onClose={() => setTwofaHistoryUid("")}
        zIndex={Z_SUB}
        destroyOnClose
      >
        <ProTable<AdminUser2FAHistoryItem>
          rowKey={(row) => row.id || JSON.stringify(row)}
          columns={[
            {
              title: t(
                "pages.users.twofaHistory.columns.createdAt",
                "Created At"
              ),
              dataIndex: "created_at",
              valueType: "dateTime",
              width: 170,
            },
            {
              title: t("pages.users.twofaHistory.columns.event", "Event"),
              dataIndex: "event",
              width: 140,
            },
            {
              title: t(
                "pages.users.twofaHistory.columns.operatorType",
                "Operator Type"
              ),
              dataIndex: "operator_type",
              width: 140,
            },
            {
              title: t("pages.users.twofaHistory.columns.operator", "Operator"),
              dataIndex: "operator",
              width: 160,
            },
            {
              title: t("pages.users.twofaHistory.columns.ip", "IP"),
              dataIndex: "ip",
              width: 140,
              ellipsis: true,
            },
            {
              title: t("pages.users.twofaHistory.columns.reason", "Reason"),
              dataIndex: "reason",
              ellipsis: true,
            },
          ]}
          request={async (params) => {
            if (!twofaHistoryUid || !canRpc(PERM.twofaHistory))
              return { success: true, data: [], total: 0 };
            const q: AdminListUser2FAHistoryParams = {
              page: params.current,
              page_size: params.pageSize,
            };
            try {
              const res = await adminListUser2FAHistory(
                { uid: twofaHistoryUid },
                q,
                { skipErrorHandler: true }
              );
              return {
                success: !!res?.success,
                data: res.data?.items ?? [],
                total: Number(res.data?.pagination?.total ?? 0),
              };
            } catch (e: any) {
              message.error(
                e?.message ||
                  t(
                    "pages.users.messages.twofaHistoryFailed",
                    "Failed to load 2FA history"
                  )
              );
              return { success: false, data: [], total: 0 };
            }
          }}
          search={false}
          options={false}
          pagination={{ pageSize: 20 }}
        />
      </Drawer>

      <ModalForm<AdminFreezeUserBody>
        title={
          freezeUser?.uid
            ? t("pages.users.freeze.titleWithUid", "Freeze User: {uid}", {
                uid: freezeUser.uid,
              })
            : t("pages.users.freeze.title", "Freeze User")
        }
        open={!!freezeUser}
        onOpenChange={(open) => {
          if (!open) setFreezeUser(null);
        }}
        onFinish={async (values) => {
          const uid = freezeUser?.uid;
          if (!uid) return false;
          try {
            const res = await adminFreezeUser({ uid }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.users.messages.freezeFailed", "Freeze failed")
              );
            message.success(t("pages.users.messages.frozen", "Frozen"));
            setFreezeUser(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.users.messages.freezeFailed", "Freeze failed")
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="reason"
          label={t("pages.users.form.reason", "Reason")}
          rules={[
            {
              required: true,
              whitespace: true,
              message: t("pages.users.form.reasonRequired", "请输入原因"),
            },
          ]}
          fieldProps={{ rows: 3, maxLength: 200, showCount: true }}
        />
        <ProFormSwitch
          name="freeze_assets"
          label={t("pages.users.form.freezeAssets", "Freeze assets")}
          initialValue={false}
        />
      </ModalForm>

      <ModalForm<AdminUnfreezeUserBody>
        title={
          unfreezeUser?.uid
            ? t("pages.users.unfreeze.titleWithUid", "Unfreeze User: {uid}", {
                uid: unfreezeUser.uid,
              })
            : t("pages.users.unfreeze.title", "Unfreeze User")
        }
        open={!!unfreezeUser}
        onOpenChange={(open) => {
          if (!open) setUnfreezeUser(null);
        }}
        onFinish={async (values) => {
          const uid = unfreezeUser?.uid;
          if (!uid) return false;
          try {
            const res = await adminUnfreezeUser({ uid }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.users.messages.unfreezeFailed", "Unfreeze failed")
              );
            message.success(t("pages.users.messages.unfrozen", "Unfrozen"));
            setUnfreezeUser(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.users.messages.unfreezeFailed", "Unfreeze failed")
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="reason"
          label={t("pages.users.form.reason", "Reason")}
          rules={[
            {
              required: true,
              whitespace: true,
              message: t("pages.users.form.reasonRequired", "请输入原因"),
            },
          ]}
          fieldProps={{ rows: 3, maxLength: 200, showCount: true }}
        />
        <ProFormSwitch
          name="unfreeze_assets"
          label={t("pages.users.form.unfreezeAssets", "Unfreeze assets")}
          initialValue={false}
        />
      </ModalForm>

      <ModalForm<AdminUpdateUserRoleBody>
        title={
          roleUser?.uid
            ? t("pages.users.role.titleWithUid", "Update Role: {uid}", {
                uid: roleUser.uid,
              })
            : t("pages.users.role.title", "Update Role")
        }
        open={!!roleUser}
        onOpenChange={(open) => {
          if (!open) setRoleUser(null);
        }}
        onFinish={async (values) => {
          const uid = roleUser?.uid;
          if (!uid) return false;
          try {
            const res = await adminUpdateUserRole({ uid }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.users.messages.updateRoleFailed",
                    "Update role failed"
                  )
              );
            message.success(
              t("pages.users.messages.roleUpdated", "Role updated")
            );
            setRoleUser(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.users.messages.updateRoleFailed", "Update role failed")
            );
            return false;
          }
        }}
      >
        <ProFormSelect
          name="role"
          label={t("pages.users.form.role", "Role")}
          rules={[{ required: true }]}
          options={[
            { label: t("pages.users.roles.user", "user"), value: "user" },
            { label: t("pages.users.roles.vip", "vip"), value: "vip" },
          ]}
        />
        <ProFormTextArea
          name="reason"
          label={t("pages.users.form.reason", "Reason")}
          fieldProps={{ rows: 3 }}
        />
      </ModalForm>

      <ModalForm<AdminUpdateUserMemberLevelBody>
        title={
          memberLevelUser?.uid
            ? t(
                "pages.users.memberLevel.titleWithUid",
                "Update Member Level: {uid}",
                { uid: memberLevelUser.uid }
              )
            : t("pages.users.memberLevel.title", "Update Member Level")
        }
        open={!!memberLevelUser}
        onOpenChange={(open) => {
          if (!open) setMemberLevelUser(null);
        }}
        initialValues={{
          member_level: memberLevelUser?.role === "vip" ? 2 : 1,
        }}
        onFinish={async (values) => {
          const uid = memberLevelUser?.uid;
          if (!uid) return false;
          try {
            const res = await adminUpdateUserMemberLevel({ uid }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.users.messages.memberLevelFailed",
                    "Update member level failed"
                  )
              );
            message.success(
              t(
                "pages.users.messages.memberLevelUpdated",
                "Member level updated"
              )
            );
            setMemberLevelUser(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.users.messages.memberLevelFailed",
                  "Update member level failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormSelect
          name="member_level"
          label={t("pages.users.form.memberLevel", "Member Level")}
          rules={[{ required: true }]}
          options={[
            { label: "1", value: 1 },
            { label: "2", value: 2 },
            { label: "3", value: 3 },
            { label: "4", value: 4 },
            { label: "5", value: 5 },
          ]}
        />
        <ProFormTextArea
          name="reason"
          label={t("pages.users.form.reason", "Reason")}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText.Password
          name={["reauth", "admin_password"]}
          label={t("pages.users.form.adminPassword", "Admin Password")}
          rules={[{ required: true }]}
        />
        <ProFormText
          name={["reauth", "totp_code"]}
          label={t("pages.users.form.totpCode", "TOTP Code")}
          rules={[{ required: true }]}
        />
      </ModalForm>

      <ModalForm<AdminResetUserPasswordBody>
        title={
          resetPasswordUser?.uid
            ? t(
                "pages.users.resetPassword.titleWithUid",
                "Reset Password: {uid}",
                { uid: resetPasswordUser.uid }
              )
            : t("pages.users.resetPassword.title", "Reset Password")
        }
        open={!!resetPasswordUser}
        onOpenChange={(open) => {
          if (!open) setResetPasswordUser(null);
        }}
        onFinish={async (values) => {
          const uid = resetPasswordUser?.uid;
          if (!uid) return false;
          try {
            const res = await adminResetUserPassword({ uid }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.users.messages.resetPasswordFailed",
                    "Reset password failed"
                  )
              );
            message.success(
              t("pages.users.messages.passwordReset", "Password reset")
            );
            setResetPasswordUser(null);
            actionRef.current?.reload();
            const temp = res.data?.temp_password;
            if (temp) {
              modal.info({
                title: t(
                  "pages.users.resetPassword.tempTitle",
                  "Temporary password"
                ),
                content: (
                  <div>
                    <div style={{ marginBottom: 8 }}>
                      {t(
                        "pages.users.resetPassword.tempHint",
                        "Copy and deliver to the user securely."
                      )}
                    </div>
                    <Input.TextArea value={temp} readOnly autoSize />
                  </div>
                ),
                okText: t("pages.users.actions.close", "Close"),
              });
            }
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.users.messages.resetPasswordFailed",
                  "Reset password failed"
                )
            );
            return false;
          }
        }}
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          {t(
            "pages.users.resetPassword.warning",
            "This will generate a new temporary password and force the user to re-login."
          )}
        </Typography.Paragraph>
        <ProFormTextArea
          name="reason"
          label={t("pages.users.form.reason", "Reason")}
          rules={[{ required: true }]}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText.Password
          name={["reauth", "admin_password"]}
          label={t("pages.users.form.adminPassword", "Admin Password")}
          rules={[{ required: true }]}
        />
        <ProFormText
          name={["reauth", "totp_code"]}
          label={t("pages.users.form.totpCode", "TOTP Code")}
          rules={[{ required: true }]}
        />
      </ModalForm>

      <ModalForm<AdminResetUserTradePasswordBody>
        title={
          resetTradePasswordUser?.uid
            ? t(
                "pages.users.resetTradePassword.titleWithUid",
                "Reset Trade Password: {uid}",
                {
                  uid: resetTradePasswordUser.uid,
                }
              )
            : t("pages.users.resetTradePassword.title", "Reset Trade Password")
        }
        open={!!resetTradePasswordUser}
        onOpenChange={(open) => {
          if (!open) setResetTradePasswordUser(null);
        }}
        onFinish={async (values) => {
          const uid = resetTradePasswordUser?.uid;
          if (!uid) return false;
          try {
            const res = await adminResetUserTradePassword({ uid }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.users.messages.resetTradePasswordFailed",
                    "Reset trade password failed"
                  )
              );
            message.success(
              t(
                "pages.users.messages.tradePasswordReset",
                "Trade password reset"
              )
            );
            setResetTradePasswordUser(null);
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.users.messages.resetTradePasswordFailed",
                  "Reset trade password failed"
                )
            );
            return false;
          }
        }}
        modalProps={{
          destroyOnClose: true,
          zIndex: Z_SETTINGS_SUBMODAL,
          width: 980,
          bodyStyle: { maxHeight: "72vh", overflowY: "auto" },
        }}
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          {t(
            "pages.users.resetTradePassword.warning",
            "This will clear the trade password. The user may need to set it again."
          )}
        </Typography.Paragraph>
        <ProFormTextArea
          name="reason"
          label={t("pages.users.form.reason", "Reason")}
          rules={[{ required: true }]}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText.Password
          name={["reauth", "admin_password"]}
          label={t("pages.users.form.adminPassword", "Admin Password")}
          rules={[{ required: true }]}
        />
        <ProFormText
          name={["reauth", "totp_code"]}
          label={t("pages.users.form.totpCode", "TOTP Code")}
          rules={[{ required: true }]}
        />
      </ModalForm>

      <ModalForm<AdminUpsertUserWithdrawAuditWhitelistRulesBody>
        formRef={whitelistRulesFormRef}
        title={
          whitelistUser?.uid
            ? t(
                "pages.users.whitelistRules.titleWithUid",
                "Withdraw Audit Whitelist Rules: {uid}",
                { uid: whitelistUser.uid }
              )
            : t(
                "pages.users.whitelistRules.title",
                "Withdraw Audit Whitelist Rules"
              )
        }
        open={!!whitelistUser}
        onOpenChange={(open) => {
          if (!open) {
            setWhitelistUser(null);
            setWhitelistStep(0);
            return;
          }

          setWhitelistStep(0);
        }}
        onFinish={async () => {
          const uid = whitelistUser?.uid;
          if (!uid) return false;

          const allValues = (whitelistRulesFormRef.current?.getFieldsValue?.(
            true
          ) ?? {}) as AdminUpsertUserWithdrawAuditWhitelistRulesBody;

          try {
            const payload: AdminUpsertUserWithdrawAuditWhitelistRulesBody = {
              global_enabled: !!(allValues as any).global_enabled,
              rules: ((allValues as any).rules ?? [])
                .filter(
                  (r: any) =>
                    (r.source || "").toString().trim().toLowerCase() !== "user"
                )
                .map((r: any) => ({
                  asset_code: (r.asset_code || "").trim().toUpperCase(),
                  chain_code: (r.chain_code || "").trim().toUpperCase(),
                  address: (r.address || "").trim(),
                  limit_usdt: (r.limit_usdt || "0").trim(),
                  enabled: !!r.enabled,
                  source: "admin",
                })),
              reason: (allValues as any).reason,
              reauth: (allValues as any).reauth,
            };

            const res = await adminUpsertUserWithdrawAuditWhitelistRules(
              { uid },
              payload,
              { skipErrorHandler: true }
            );

            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.users.messages.whitelistFailed",
                    "Update whitelist failed"
                  )
              );

            message.success(
              t("pages.users.messages.whitelistUpdated", "Whitelist updated")
            );
            setWhitelistUser(null);
            setWhitelistStep(0);
            actionRef.current?.reload();
            void refreshStats();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.users.messages.whitelistFailed",
                  "Update whitelist failed"
                )
            );
            return false;
          }
        }}
        submitter={{
          render: (props, doms) => {
            const form = props.form;
            const cancelBtn = doms?.[0];
            const loading = whitelistRulesLoading;

            if (whitelistStep === 0) {
              return [
                cancelBtn,
                <Button
                  key="next"
                  type="primary"
                  loading={loading}
                  onClick={async () => {
                    try {
                      await form?.validateFields?.([
                        "global_enabled",
                        "rules",
                      ] as any);
                      setWhitelistStep(1);
                    } catch {}
                  }}
                >
                  {t("pages.common.users.next", "Next")}
                </Button>,
              ];
            }

            return [
              cancelBtn,
              <Button
                key="back"
                onClick={() => setWhitelistStep(0)}
                disabled={loading}
              >
                {t("pages.vault.chainDetail.actions.back", "Back")}
              </Button>,
              <Button
                key="submit"
                type="primary"
                loading={loading}
                onClick={() => form?.submit?.()}
              >
                {t("common.submit", "Submit")}
              </Button>,
            ];
          },
        }}
        modalProps={{
          destroyOnClose: true,
          width: 980,
          bodyStyle: { maxHeight: "72vh", overflowY: "auto" },
        }}
      >
        <Steps
          current={whitelistStep}
          style={{ marginBottom: 16 }}
          items={[
            { title: t("pages.users.whitelistRules.tabs.rules", "Rules") },
            { title: t("pages.users.whitelistRules.tabs.verify", "Verify") },
          ]}
        />

        <div style={{ display: whitelistStep === 0 ? "block" : "none" }}>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
            {t(
              "pages.users.whitelistRules.desc",
              "Match strictly by chain code. limit_usdt=0 means unlimited."
            )}
          </Typography.Paragraph>

          <ProFormSwitch
            name="global_enabled"
            label={t(
              "pages.users.whitelistRules.form.globalEnabled",
              "Global bypass"
            )}
            tooltip={t(
              "pages.users.whitelistRules.form.globalEnabledTip",
              "When enabled, all withdrawals bypass audit (legacy)."
            )}
          />

          <EditableProTable<any>
            name="rules"
            rowKey="key"
            loading={whitelistRulesLoading}
            size="small"
            scroll={{ x: "max-content", y: 360 }}
            recordCreatorProps={{
              position: "bottom",
              creatorButtonText: t(
                "pages.users.whitelistRules.actions.addRule",
                "Add rule"
              ),
              record: () => ({
                key: `new-${Date.now()}`,
                asset_code: "",
                chain_code: "",
                address: "",
                limit_usdt: "0",
                enabled: true,
                source: "admin",
              }),
            }}
            editable={{
              type: "multiple",
              editableKeys: editableRuleKeys,
              onChange: (keys) => {
                const currentRules =
                  (whitelistRulesFormRef.current?.getFieldValue?.("rules") ??
                    []) as any[];
                const filtered = (keys || []).filter((k) => {
                  const row = (currentRules || []).find((r) => r?.key === k);
                  const src = (row?.source || "")
                    .toString()
                    .trim()
                    .toLowerCase();
                  return src !== "user";
                });
                setEditableRuleKeys(filtered as any);
              },
              actionRender: () => [],
            }}
            columns={[
              {
                title: t("pages.users.whitelistRules.table.token", "Token"),
                dataIndex: "asset_code",
                width: 160,
                editable: (_: any, row: any) =>
                  (row?.source || "").toString().trim().toLowerCase() !==
                  "user",
                formItemProps: { rules: [{ required: true }] },
                renderFormItem: (_, config) => (
                  <Select
                    showSearch
                    filterOption={false}
                    options={assetOptions}
                    loading={assetLoading}
                    placeholder={t(
                      "pages.users.whitelistRules.form.asset",
                      "Asset"
                    )}
                    value={config.value}
                    onChange={(v) => config.onChange?.(v)}
                    onSearch={(v) => void loadAssets(v)}
                  />
                ),
              },
              {
                title: t("pages.users.whitelistRules.table.chain", "Chain"),
                dataIndex: "chain_code",
                width: 170,
                editable: (_: any, row: any) =>
                  (row?.source || "").toString().trim().toLowerCase() !==
                  "user",
                formItemProps: { rules: [{ required: true }] },
                renderFormItem: (_, config) => (
                  <Select
                    showSearch
                    filterOption={(input, option) =>
                      (option?.value || "")
                        .toString()
                        .toLowerCase()
                        .includes(input.toLowerCase()) ||
                      (option?.label || "")
                        .toString()
                        .toLowerCase()
                        .includes(input.toLowerCase())
                    }
                    options={chainOptions}
                    placeholder={t(
                      "pages.users.whitelistRules.form.chain",
                      "Chain"
                    )}
                    value={config.value}
                    onChange={(v) => config.onChange?.(v)}
                  />
                ),
              },

              {
                title: t("pages.users.whitelistRules.table.address", "Address"),
                dataIndex: "address",
                width: 360,
                editable: (_: any, row: any) =>
                  (row?.source || "").toString().trim().toLowerCase() !==
                  "user",
                formItemProps: { rules: [{ required: true }] },
                renderFormItem: (_, config) => (
                  <Input
                    placeholder={t(
                      "pages.users.whitelistRules.form.address",
                      "Address"
                    )}
                    value={config.value}
                    onChange={(e) => config.onChange?.(e.target.value)}
                  />
                ),
              },
              {
                title: t(
                  "pages.users.whitelistRules.table.limitUsdt",
                  "Limit (USDT)"
                ),
                dataIndex: "limit_usdt",
                width: 160,
                editable: (_: any, row: any) =>
                  (row?.source || "").toString().trim().toLowerCase() !==
                  "user",
                formItemProps: {
                  rules: [
                    { required: true },
                    {
                      pattern: /^(0|[1-9]\d*)(\.\d{1,8})?$/,
                      message: t(
                        "pages.users.whitelistRules.form.limitInvalid",
                        "Invalid number (max 8 decimals)"
                      ),
                    },
                  ],
                },
                renderFormItem: (_, config) => (
                  <Input
                    placeholder={t(
                      "pages.users.whitelistRules.form.limitUsdtPlaceholder",
                      "0 = unlimited; up to 8 decimals"
                    )}
                    value={config.value}
                    onChange={(e) => config.onChange?.(e.target.value)}
                  />
                ),
              },
              {
                title: t("pages.users.whitelistRules.table.enabled", "Enabled"),
                dataIndex: "enabled",
                width: 110,
                valueType: "switch",
                editable: (_: any, row: any) =>
                  (row?.source || "").toString().trim().toLowerCase() !==
                  "user",
                render: (_: any, row: any) =>
                  row?.enabled ? (
                    <Tag color="green">
                      {t("pages.users.filters.options.enabled", "Enabled")}
                    </Tag>
                  ) : (
                    <Tag>
                      {t("pages.users.filters.options.disabled", "Disabled")}
                    </Tag>
                  ),
              },

              {
                title: t("pages.users.whitelistRules.table.source", "Source"),
                dataIndex: "source",
                width: 110,
                editable: false,
                render: (_: any, row: any) => {
                  const src = (row?.source || "")
                    .toString()
                    .trim()
                    .toLowerCase();
                  if (src === "user") {
                    return (
                      <Tag color="blue">
                        {t("pages.users.whitelistRules.source.user", "User")}
                      </Tag>
                    );
                  }
                  return (
                    <Tag>
                      {t("pages.users.whitelistRules.source.admin", "Admin")}
                    </Tag>
                  );
                },
              },
              {
                title: t("pages.users.whitelistRules.table.actions", "操作"),
                dataIndex: "__actions",
                editable: false,
                valueType: "option",
                width: 90,
                render: (_: any, row: any) => {
                  const key = row?.key as React.Key | undefined;
                  if (!key) return null;
                  const src = (row?.source || "")
                    .toString()
                    .trim()
                    .toLowerCase();
                  if (src === "user") return null;
                  return (
                    <Button
                      type="link"
                      danger
                      size="small"
                      icon={<MinusCircleOutlined />}
                      onClick={() => {
                        modal.confirm({
                          title: t(
                            "pages.users.whitelistRules.actions.removeConfirmTitle",
                            "Remove this rule?"
                          ),
                          okButtonProps: { danger: true },
                          okText: t("common.confirm", "Confirm"),
                          cancelText: t("common.cancel", "Cancel"),
                          onOk: () => removeWhitelistRuleRow(key),
                        });
                      }}
                    >
                      {t("pages.users.whitelistRules.actions.remove", "移除")}
                    </Button>
                  );
                },
              },
            ]}
            pagination={{ pageSize: 5, showSizeChanger: false }}
          />
        </div>

        <div style={{ display: whitelistStep === 1 ? "block" : "none" }}>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
            {t(
              "pages.users.whitelistRules.verifyHint",
              "Verification is required to submit changes."
            )}
          </Typography.Paragraph>

          <ProFormTextArea
            name="reason"
            label={t("pages.users.form.reason", "Reason")}
            rules={whitelistStep === 1 ? [{ required: true }] : []}
            fieldProps={{ rows: 3 }}
          />
          <ProFormText.Password
            name={["reauth", "admin_password"]}
            label={t("pages.users.form.adminPassword", "Admin Password")}
            rules={whitelistStep === 1 ? [{ required: true }] : []}
          />
          <ProFormText
            name={["reauth", "totp_code"]}
            label={t("pages.users.form.totpCode", "TOTP Code")}
            rules={whitelistStep === 1 ? [{ required: true }] : []}
          />
        </div>
      </ModalForm>

      <ModalForm<AdminUnbindUserTotpBody>
        title={
          unbind2faUser?.uid
            ? t("pages.users.unbind2fa.titleWithUid", "Unbind 2FA: {uid}", {
                uid: unbind2faUser.uid,
              })
            : t("pages.users.unbind2fa.title", "Unbind 2FA")
        }
        open={!!unbind2faUser}
        onOpenChange={(open) => {
          if (!open) setUnbind2faUser(null);
        }}
        onFinish={async (values) => {
          const uid = unbind2faUser?.uid;
          if (!uid) return false;
          try {
            const res = await adminUnbindUserTotp({ uid }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.users.messages.unbind2faFailed", "Unbind 2FA failed")
              );
            message.success(
              t("pages.users.messages.unbind2faSuccess", "2FA unbound")
            );
            setUnbind2faUser(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.users.messages.unbind2faFailed", "Unbind 2FA failed")
            );
            return false;
          }
        }}
        submitter={{ submitButtonProps: { danger: true } }}
        modalProps={{ destroyOnClose: true, zIndex: Z_SETTINGS_SUBMODAL }}
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          {t(
            "pages.users.unbind2fa.warning",
            "Unbinding 2FA is a high-risk action. Proceed with caution."
          )}
        </Typography.Paragraph>
        <ProFormTextArea
          name="reason"
          label={t("pages.users.form.reason", "Reason")}
          rules={[{ required: true }]}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText.Password
          name={["reauth", "admin_password"]}
          label={t("pages.users.form.adminPassword", "Admin Password")}
          rules={[{ required: true }]}
        />
        <ProFormText
          name={["reauth", "totp_code"]}
          label={t("pages.users.form.totpCode", "TOTP Code")}
          rules={[{ required: true }]}
        />
      </ModalForm>

      <ModalForm<AdminAddUserNoteBody>
        title={
          noteUser?.uid
            ? t("pages.users.note.titleWithUid", "Add Note: {uid}", {
                uid: noteUser.uid,
              })
            : t("pages.users.note.title", "Add Note")
        }
        open={!!noteUser}
        onOpenChange={(open) => {
          if (!open) setNoteUser(null);
        }}
        onFinish={async (values) => {
          const uid = noteUser?.uid;
          if (!uid) return false;
          try {
            const res = await adminAddUserNote({ uid }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.users.messages.addNoteFailed", "Add note failed")
              );
            message.success(t("pages.users.messages.noteAdded", "Note added"));
            setNoteUser(null);
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.users.messages.addNoteFailed", "Add note failed")
            );
            return false;
          }
        }}
        modalProps={{ destroyOnClose: true, zIndex: Z_SETTINGS_SUBMODAL }}
      >
        <ProFormTextArea
          name="content"
          label={t("pages.users.form.content", "Content")}
          rules={[{ required: true }]}
          fieldProps={{ rows: 4 }}
        />
        <ProFormSwitch
          name="is_important"
          label={t("pages.users.form.important", "Important")}
          initialValue={false}
        />
      </ModalForm>

      <ModalForm<AdminTerminateUserBody>
        title={
          terminateUser?.uid
            ? t("pages.users.terminate.titleWithUid", "Terminate User: {uid}", {
                uid: terminateUser.uid,
              })
            : t("pages.users.terminate.title", "Terminate User")
        }
        open={!!terminateUser}
        onOpenChange={(open) => {
          if (!open) setTerminateUser(null);
        }}
        onFinish={async (values) => {
          const uid = terminateUser?.uid;
          if (!uid) return false;
          try {
            const res = await adminTerminateUser({ uid }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t("pages.users.messages.terminateFailed", "Terminate failed")
              );
            message.success(t("pages.users.messages.terminated", "Terminated"));
            setTerminateUser(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.users.messages.terminateFailed", "Terminate failed")
            );
            return false;
          }
        }}
        submitter={{ submitButtonProps: { danger: true } }}
      >
        <Typography.Paragraph type="secondary">
          {t(
            "pages.users.terminate.warning",
            "This action is irreversible. Make sure you understand the impact."
          )}
        </Typography.Paragraph>
        <ProFormTextArea
          name="reason"
          label={t("pages.users.form.reason", "Reason")}
          rules={[{ required: true }]}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText
          name="effective_date"
          label={t("pages.users.form.effectiveDate", "Effective Date")}
          placeholder={t(
            "pages.users.form.effectiveDatePlaceholder",
            "YYYY-MM-DD (optional)"
          )}
        />
        <ProFormText
          name="handle_balance"
          label={t("pages.users.form.handleBalance", "Handle Balance")}
          placeholder={t(
            "pages.users.form.handleBalancePlaceholder",
            "e.g. freeze | withdraw | burn (optional)"
          )}
        />
        <ProFormText
          name="withdraw_address"
          label={t("pages.users.form.withdrawAddress", "Withdraw Address")}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default UsersPage;
