import {
  adminBlacklistWeb3Address,
  adminGetWeb3User,
  adminGetWeb3UsersPageList,
  adminGetWeb3UsersSummary,
  adminListWeb3UserTransactions,
  adminUnblacklistWeb3Address,
} from "@/api/generated/web3-users";
import { useRbac } from "@/hooks/useRbac";
import type { ActionType, ProColumns } from "@ant-design/pro-components";
import { PageContainer, ProTable } from "@ant-design/pro-components";
import {
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Drawer,
  Form,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Statistic,
  Switch,
  Tag,
} from "antd";
import React, { useEffect, useMemo, useRef, useState } from "react";

type GatewayResp<T> = {
  success?: boolean;
  code?: number;
  message?: string;
  data?: T;
};

type Pagination = {
  page?: number;
  page_size?: number;
  total?: string | number;
  total_pages?: number;
  has_next?: boolean;
  has_prev?: boolean;
};

type Web3UsersListItem = {
  id?: string;
  device_id?: string;
  platform?: string;
  os_version?: string;
  app_version?: string;
  wallet_count?: number;
  networks?: string[];
  primary_address?: string;
  two_factor_enabled?: boolean;
  biometric_enabled?: boolean;
  is_primary_blacklisted?: boolean;
  last_active_at?: string;
  created_at?: string;
};

type Web3UsersListResp = {
  items?: Web3UsersListItem[];
  pagination?: Pagination;
};

type Web3UsersSummaryResp = {
  total_devices?: string;
  total_addresses?: string;
  blacklisted_addresses?: string;
  two_factor_enabled_rate?: number;
};

type Web3UserDetail = {
  id?: string;
  device_id?: string;
  device_info?: {
    platform?: string;
    os_version?: string;
    app_version?: string;
    device_model?: string;
    device_name?: string;
    push_token?: string;
    locale?: string;
    last_active_at?: string;
    created_at?: string;
  };
  security_info?: {
    two_factor_enabled?: boolean;
    two_factor_type?: string;
    biometric_enabled?: boolean;
    biometric_type?: string;
    last_security_update?: string;
  };
  wallet_addresses?: Array<{
    network?: string;
    chain_id?: string;
    address?: string;
    is_primary?: boolean;
    enabled?: boolean;
    is_blacklisted?: boolean;
    balance?: string;
    balance_usd?: string;
    token_balances?: Array<{
      currency?: string;
      balance?: string;
      balance_usd?: string;
    }>;
    created_at?: string;
    last_transaction_at?: string;
    blacklist_reason?: string;
    risk_level?: string;
    blacklisted_at?: string;
    blacklisted_by?: string;
  }>;
  activity_summary?: {
    total_transactions?: string;
    total_deposit_usd?: string;
    total_withdrawal_usd?: string;
    total_swap_usd?: string;
    first_transaction_at?: string;
    last_transaction_at?: string;
  };
  created_at?: string;
  last_active_at?: string;
};

type Web3UserResp = {
  user?: Web3UserDetail;
};

type TxItem = {
  id?: string;
  tx_hash?: string;
  network?: string;
  chain_id?: string;
  user_address?: string;
  tx_type?: string;
  direction?: string;
  asset_code?: string;
  amount?: string;
  amount_usd?: string;
  from_address?: string;
  to_address?: string;
  status?: string;
  confirmations?: number;
  fee?: string;
  fee_asset?: string;
  block_number?: string;
  block_time?: string;
  created_at?: string;
};

type TxListResp = {
  items?: TxItem[];
  pagination?: Pagination;
};

const fmtTime = (v?: string) => {
  if (!v) return "—";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return v;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(
    d.getHours()
  )}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
};

const PERM = {
  list: "GetWeb3UsersPageList",
  summary: "GetWeb3UsersSummary",
  detail: "GetWeb3User",
  txList: "ListWeb3UserTransactions",
  blacklist: "BlacklistWeb3Address",
  unblacklist: "UnblacklistWeb3Address",
};

type BlacklistFormValues = {
  reason?: string;
  risk_level?: "high" | "medium" | "low";
  disable_address?: boolean;
  notify_user?: boolean;
};

type UnblacklistFormValues = {
  reason?: string;
  enable_address?: boolean;
  notify_user?: boolean;
};

const pickSortForList = (
  sort: Record<string, "ascend" | "descend" | null | undefined>
): {
  sort_by?: "created_at" | "last_active_at";
  sort_order?: "asc" | "desc";
} => {
  const entry = Object.entries(sort || {}).find(
    ([, v]) => v === "ascend" || v === "descend"
  );
  if (!entry) return {};
  const [field, dir] = entry;
  const sort_by =
    field === "last_active_at"
      ? "last_active_at"
      : field === "created_at"
      ? "created_at"
      : undefined;
  const sort_order = dir === "ascend" ? "asc" : "desc";
  return { sort_by, sort_order };
};

const SwapTagBool: React.FC<{ v?: boolean; yes?: string; no?: string }> = ({
  v,
  yes = "是",
  no = "否",
}) => {
  if (v === undefined || v === null) return <Tag>—</Tag>;
  return v ? <Tag color="green">{yes}</Tag> : <Tag color="default">{no}</Tag>;
};

const Web3UsersPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();

  const canList = canRpc(PERM.list);
  const canSummary = canRpc(PERM.summary);
  const canDetail = canRpc(PERM.detail);
  const canTxList = canRpc(PERM.txList);
  const canBlacklist = canRpc(PERM.blacklist);
  const canUnblacklist = canRpc(PERM.unblacklist);

  const actionRef = useRef<ActionType | null>(null);
  const txActionRef = useRef<ActionType | null>(null);

  const [summary, setSummary] = useState<Web3UsersSummaryResp | null>(null);
  const [summaryLoading, setSummaryLoading] = useState(false);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [currentDeviceId, setCurrentDeviceId] = useState<string | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<Web3UserDetail | null>(null);

  const [blacklistOpen, setBlacklistOpen] = useState(false);
  const [unblacklistOpen, setUnblacklistOpen] = useState(false);
  const [opDeviceId, setOpDeviceId] = useState<string | null>(null);
  const [addrTarget, setAddrTarget] = useState<{
    address: string;
    network?: string;
  } | null>(null);
  const [blacklistSubmitting, setBlacklistSubmitting] = useState(false);
  const [unblacklistSubmitting, setUnblacklistSubmitting] = useState(false);
  const [blacklistForm] = Form.useForm<BlacklistFormValues>();
  const [unblacklistForm] = Form.useForm<UnblacklistFormValues>();

  const [txModalOpen, setTxModalOpen] = useState(false);
  const [txTarget, setTxTarget] = useState<{
    deviceId: string;
    address?: string;
    network?: string;
  } | null>(null);

  const openDetail = (deviceId?: string) => {
    if (!deviceId) return;
    if (!canDetail) return;
    setCurrentDeviceId(deviceId);
    setDrawerOpen(true);
  };

  const refreshSummary = async () => {
    if (!canSummary) return;
    setSummaryLoading(true);
    try {
      const res = (await adminGetWeb3UsersSummary({
        skipErrorHandler: true,
      })) as GatewayResp<Web3UsersSummaryResp>;
      if (!res?.success) throw new Error(res?.message || "加载统计失败");
      setSummary(res.data || null);
    } catch (e: any) {
      message.error(e?.message || "加载统计失败");
    } finally {
      setSummaryLoading(false);
    }
  };

  const refreshDetail = async (deviceId: string) => {
    if (!canDetail) return;
    setDetailLoading(true);
    try {
      const res = (await adminGetWeb3User(
        { deviceId },
        { skipErrorHandler: true }
      )) as GatewayResp<Web3UserResp>;
      if (!res?.success) throw new Error(res?.message || "加载详情失败");
      const user = res.data?.user;
      if (!user) throw new Error("加载详情失败");
      setDetail(user);
    } catch (e: any) {
      message.error(e?.message || "加载详情失败");
    } finally {
      setDetailLoading(false);
    }
  };

  const openBlacklistModal = (
    deviceId: string,
    address?: string,
    network?: string
  ) => {
    if (!canBlacklist) return;
    if (!deviceId || !address) return;
    setOpDeviceId(deviceId);
    setAddrTarget({ address, network });
    blacklistForm.resetFields();
    blacklistForm.setFieldsValue({
      disable_address: true,
      notify_user: false,
      risk_level: "medium",
    });
    setBlacklistOpen(true);
  };

  const openUnblacklistModal = (
    deviceId: string,
    address?: string,
    network?: string
  ) => {
    if (!canUnblacklist) return;
    if (!deviceId || !address) return;
    setOpDeviceId(deviceId);
    setAddrTarget({ address, network });
    unblacklistForm.resetFields();
    unblacklistForm.setFieldsValue({
      enable_address: true,
      notify_user: false,
    });
    setUnblacklistOpen(true);
  };

  const openTxModalFromList = (
    deviceId?: string,
    address?: string,
    network?: string
  ) => {
    if (!canTxList) return;
    if (!deviceId) return;
    setTxTarget({ deviceId, address, network });
    setTxModalOpen(true);
    setTimeout(() => txActionRef.current?.reload(), 0);
  };

  useEffect(() => {
    refreshSummary();
  }, []);

  useEffect(() => {
    if (!drawerOpen || !currentDeviceId) return;
    refreshDetail(currentDeviceId);
  }, [drawerOpen, currentDeviceId]);

  const listColumns = useMemo(() => {
    const showAddBlacklistInList = false;

    const cols: ProColumns<Web3UsersListItem>[] = [
      {
        title: "设备ID",
        dataIndex: "device_id",
        width: 180,
      },
      {
        title: "最后活跃时间",
        dataIndex: "last_active_at",
        width: 180,
        hideInSearch: true,
        render: (_, r) => fmtTime(r.last_active_at),
      },
      {
        title: "平台",
        dataIndex: "platform",
        width: 110,
        valueType: "select",
        valueEnum: { iOS: { text: "iOS" }, Android: { text: "Android" } },
      },
      {
        title: "系统版本",
        dataIndex: "os_version",
        width: 140,
        hideInSearch: true,
      },
      {
        title: "App版本",
        dataIndex: "app_version",
        width: 120,
        hideInSearch: true,
      },
      {
        title: "钱包数",
        dataIndex: "wallet_count",
        width: 100,
        hideInSearch: true,
      },
      {
        title: "网络",
        dataIndex: "networks",
        width: 160,
        hideInSearch: true,
        render: (_, r) =>
          r.networks?.length ? (
            <span>{r.networks.join(", ")}</span>
          ) : (
            <span>—</span>
          ),
      },
      {
        title: "创建时间",
        dataIndex: "created_range",
        hideInTable: true,
        valueType: "dateRange",
      },
      {
        title: "最后活跃",
        dataIndex: "last_active_range",
        hideInTable: true,
        valueType: "dateRange",
      },
      {
        title: "搜索",
        dataIndex: "keyword",
        hideInTable: true,
        valueType: "text",
        fieldProps: { placeholder: "device_id 或 address" },
      },
      {
        title: "系统版本",
        dataIndex: "os_version_search",
        hideInTable: true,
        valueType: "text",
      },
      {
        title: "App版本",
        dataIndex: "app_version_search",
        hideInTable: true,
        valueType: "text",
      },
      {
        title: "设备型号",
        dataIndex: "device_model",
        hideInTable: true,
        valueType: "text",
      },
      {
        title: "设备名称",
        dataIndex: "device_name",
        hideInTable: true,
        valueType: "text",
      },
      {
        title: "语言/地区",
        dataIndex: "locale",
        hideInTable: true,
        valueType: "text",
      },
      {
        title: "创建时间",
        dataIndex: "created_at",
        width: 180,
        render: (_, r) => fmtTime(r.created_at),
      },
      {
        title: "网络筛选",
        dataIndex: "network",
        hideInTable: true,
        valueType: "text",
      },
      {
        title: "地址状态",
        dataIndex: "address_status",
        hideInTable: true,
        valueType: "select",
        valueEnum: {
          normal: { text: "normal" },
          blacklisted: { text: "blacklisted" },
        },
      },
      {
        title: "钱包数最小",
        dataIndex: "wallet_count_min",
        hideInTable: true,
        valueType: "digit",
        fieldProps: { precision: 0, min: 0 },
      },
      {
        title: "钱包数最大",
        dataIndex: "wallet_count_max",
        hideInTable: true,
        valueType: "digit",
        fieldProps: { precision: 0, min: 0 },
      },
      {
        title: "排序字段",
        dataIndex: "sort_by",
        hideInTable: true,
        valueType: "select",
        initialValue: "created_at",
        valueEnum: {
          created_at: { text: "created_at" },
          last_active_at: { text: "last_active_at" },
        },
      },
      {
        title: "排序方向",
        dataIndex: "sort_order",
        hideInTable: true,
        valueType: "select",
        initialValue: "desc",
        valueEnum: { desc: { text: "desc" }, asc: { text: "asc" } },
      },
      {
        title: "操作",
        valueType: "option",
        align: "center",
        width: 180,
        fixed: "right",
        render: (_, r) => {
          const deviceId = r.device_id || "";
          const address = r.primary_address || "";
          const hasPrimary = !!deviceId && !!address;
          const blacklisted = !!r.is_primary_blacklisted;

          return (
            <Space>
              <a
                onClick={() => {
                  if (!canDetail) return;
                  openDetail(r.device_id);
                }}
                style={{ color: canDetail ? undefined : "rgba(0,0,0,0.25)" }}
              >
                详情
              </a>

              <a
                onClick={() => {
                  if (!canTxList || !deviceId) return;
                  openTxModalFromList(deviceId, address || undefined);
                }}
                style={{
                  color: canTxList && deviceId ? undefined : "rgba(0,0,0,0.25)",
                }}
              >
                交易记录
              </a>

              {blacklisted ? (
                <a
                  onClick={() => {
                    if (!canUnblacklist || !hasPrimary) return;
                    openUnblacklistModal(deviceId, address);
                  }}
                  style={{
                    color:
                      canUnblacklist && hasPrimary
                        ? undefined
                        : "rgba(0,0,0,0.25)",
                  }}
                >
                  移出黑名单
                </a>
              ) : showAddBlacklistInList ? (
                <a
                  onClick={() => {
                    if (!canBlacklist || !hasPrimary) return;
                    openBlacklistModal(deviceId, address);
                  }}
                  style={{
                    color:
                      canBlacklist && hasPrimary
                        ? undefined
                        : "rgba(0,0,0,0.25)",
                  }}
                >
                  加入黑名单
                </a>
              ) : null}
            </Space>
          );
        },
      },
    ];
    return cols;
  }, [canDetail, canTxList, canBlacklist, canUnblacklist]);

  const addressColumns = useMemo(() => {
    const cols: ProColumns<
      NonNullable<Web3UserDetail["wallet_addresses"]>[number]
    >[] = [
      { title: "网络", dataIndex: "network", width: 120 },
      { title: "链ID", dataIndex: "chain_id", width: 100 },
      {
        title: "地址",
        dataIndex: "address",
        width: 280,
        copyable: true,
        ellipsis: true,
      },
      // {
      //   title: "主地址",
      //   dataIndex: "is_primary",
      //   width: 90,
      //   render: (_, r) =>
      //     r.is_primary ? <Tag color="blue">是</Tag> : <Tag>否</Tag>,
      // },
      {
        title: "启用",
        dataIndex: "enabled",
        width: 90,
        render: (_, r) => <SwapTagBool v={!!r.enabled} />,
      },
      // { title: "余额", dataIndex: "balance", width: 140, ellipsis: true },
      // {
      //   title: "余额(USD)",
      //   dataIndex: "balance_usd",
      //   width: 140,
      //   ellipsis: true,
      // },
      // {
      //   title: "最后交易",
      //   dataIndex: "last_transaction_at",
      //   width: 180,
      //   render: (_, r) => fmtTime(r.last_transaction_at),
      // },
      // {
      //   title: "加入黑名单时间",
      //   dataIndex: "blacklisted_at",
      //   width: 180,
      //   render: (_, r) => fmtTime(r.blacklisted_at),
      // },
      {
        title: "原因",
        dataIndex: "blacklist_reason",
        width: 200,
        ellipsis: true,
      },
    ];
    return cols;
  }, []);

  const txColumns = useMemo(() => {
    const cols: ProColumns<TxItem>[] = [
      {
        title: "时间",
        dataIndex: "block_time",
        width: 180,
        hideInSearch: true,
        render: (_, r) => fmtTime(r.block_time),
      },

      { title: "网络", dataIndex: "network", width: 120, hideInSearch: true },
      { title: "链ID", dataIndex: "chain_id", width: 100, hideInSearch: true },
      { title: "类型", dataIndex: "tx_type", width: 130, hideInSearch: true },
      { title: "方向", dataIndex: "direction", width: 90, hideInSearch: true },
      {
        title: "资产",
        dataIndex: "asset_code",
        width: 100,
        hideInSearch: true,
      },
      {
        title: "数量",
        dataIndex: "amount",
        width: 140,
        ellipsis: true,
        hideInSearch: true,
      },
      // {
      //   title: "USD",
      //   dataIndex: "amount_usd",
      //   width: 120,
      //   ellipsis: true,
      //   hideInSearch: true,
      // },
      { title: "状态", dataIndex: "status", width: 120, hideInSearch: true },

      {
        title: "确认数",
        dataIndex: "confirmations",
        width: 90,
        hideInSearch: true,
      },
      {
        title: "手续费",
        dataIndex: "fee",
        width: 120,
        hideInSearch: true,
        ellipsis: true,
      },
      {
        title: "手续费资产",
        dataIndex: "fee_asset",
        width: 120,
        hideInSearch: true,
      },
      {
        title: "From",
        dataIndex: "from_address",
        width: 260,
        hideInSearch: true,
        ellipsis: true,
        copyable: true,
      },
      {
        title: "To",
        dataIndex: "to_address",
        width: 260,
        hideInSearch: true,
        ellipsis: true,
        copyable: true,
      },
      {
        title: "TxHash",
        dataIndex: "tx_hash",
        width: 280,
        hideInSearch: true,
        ellipsis: true,
        copyable: true,
      },

      {
        title: "网络",
        dataIndex: "network_search",
        hideInTable: true,
        valueType: "text",
      },
      {
        title: "类型",
        dataIndex: "tx_type_search",
        hideInTable: true,
        valueType: "select",
        valueEnum: {
          receive: { text: "receive" },
          send: { text: "send" },
          swap: { text: "swap" },
          contract_call: { text: "contract_call" },
          approve: { text: "approve" },
        },
      },
      {
        title: "方向",
        dataIndex: "direction_search",
        hideInTable: true,
        valueType: "select",
        valueEnum: { in: { text: "in" }, out: { text: "out" } },
      },
      {
        title: "资产",
        dataIndex: "asset_code_search",
        hideInTable: true,
        valueType: "text",
      },
      {
        title: "状态",
        dataIndex: "status_search",
        hideInTable: true,
        valueType: "select",
        valueEnum: {
          pending: { text: "pending" },
          confirmed: { text: "confirmed" },
          failed: { text: "failed" },
        },
      },
      {
        title: "日期",
        dataIndex: "date_range",
        hideInTable: true,
        valueType: "dateRange",
      },
      {
        title: "排序字段",
        dataIndex: "sort_by",
        hideInTable: true,
        valueType: "select",
        initialValue: "block_time",
        valueEnum: {
          block_time: { text: "block_time" },
          created_at: { text: "created_at" },
        },
      },
      {
        title: "排序方向",
        dataIndex: "sort_order",
        hideInTable: true,
        valueType: "select",
        initialValue: "desc",
        valueEnum: { desc: { text: "desc" }, asc: { text: "asc" } },
      },
      {
        title: "创建时间",
        dataIndex: "created_at",
        width: 180,
        hideInSearch: true,
        render: (_, r) => fmtTime(r.created_at),
      },
    ];
    return cols;
  }, []);

  const submitBlacklist = async () => {
    const deviceId = opDeviceId || currentDeviceId;
    if (!deviceId || !addrTarget?.address) return;
    try {
      const values = await blacklistForm.validateFields();
      setBlacklistSubmitting(true);
      const res = (await adminBlacklistWeb3Address(
        { deviceId, address: addrTarget.address },
        {
          reason: values.reason,
          risk_level: values.risk_level,
          disable_address: values.disable_address ?? true,
          notify_user: values.notify_user ?? false,
        },
        { skipErrorHandler: true }
      )) as GatewayResp<any>;
      if (!res?.success) throw new Error(res?.message || "操作失败");
      message.success("已加入黑名单");
      setBlacklistOpen(false);
      setAddrTarget(null);
      setOpDeviceId(null);
      blacklistForm.resetFields();
      await refreshSummary();
      actionRef.current?.reload();
      if (drawerOpen && currentDeviceId === deviceId) {
        await refreshDetail(deviceId);
      }
      if (txModalOpen && txTarget?.deviceId === deviceId) {
        txActionRef.current?.reload();
      }
    } catch (e: any) {
      if (e?.errorFields) return;
      message.error(e?.message || "操作失败");
    } finally {
      setBlacklistSubmitting(false);
    }
  };

  const submitUnblacklist = async () => {
    const deviceId = opDeviceId || currentDeviceId;
    if (!deviceId || !addrTarget?.address) return;
    try {
      const values = await unblacklistForm.validateFields();
      setUnblacklistSubmitting(true);
      const res = (await adminUnblacklistWeb3Address(
        { deviceId, address: addrTarget.address },
        {
          reason: values.reason,
          enable_address: values.enable_address,
          notify_user: values.notify_user,
        },
        { skipErrorHandler: true }
      )) as GatewayResp<any>;
      if (!res?.success) throw new Error(res?.message || "操作失败");
      message.success("已移出黑名单");
      setUnblacklistOpen(false);
      setAddrTarget(null);
      setOpDeviceId(null);
      unblacklistForm.resetFields();
      await refreshSummary();
      actionRef.current?.reload();
      if (drawerOpen && currentDeviceId === deviceId) {
        await refreshDetail(deviceId);
      }
      if (txModalOpen && txTarget?.deviceId === deviceId) {
        txActionRef.current?.reload();
      }
    } catch (e: any) {
      if (e?.errorFields) return;
      message.error(e?.message || "操作失败");
    } finally {
      setUnblacklistSubmitting(false);
    }
  };

  const summaryCards = (
    <Row gutter={[12, 12]}>
      <Col xs={24} sm={12} md={6}>
        <Card loading={summaryLoading} size="small">
          <Statistic title="设备总数" value={summary?.total_devices ?? 0} />
        </Card>
      </Col>
      <Col xs={24} sm={12} md={6}>
        <Card loading={summaryLoading} size="small">
          <Statistic title="地址总数" value={summary?.total_addresses ?? 0} />
        </Card>
      </Col>
      <Col xs={24} sm={12} md={6}>
        <Card loading={summaryLoading} size="small">
          <Statistic
            title="黑名单地址数"
            value={summary?.blacklisted_addresses ?? 0}
          />
        </Card>
      </Col>
      <Col xs={24} sm={12} md={6}>
        <Card loading={summaryLoading} size="small">
          <Statistic
            title="2FA 启用率"
            value={summary?.two_factor_enabled_rate ?? 0}
            precision={2}
            suffix="%"
          />
        </Card>
      </Col>
    </Row>
  );

  return (
    <PageContainer>
      {summaryCards}

      <div style={{ height: 12 }} />

      <ProTable<Web3UsersListItem>
        actionRef={actionRef}
        rowKey={(r) => r.device_id || r.id || ""}
        columns={listColumns}
        scroll={{ x: 1500 }}
        search={{ labelWidth: 120 }}
        toolBarRender={() => [
          <Button
            key="refresh"
            onClick={refreshSummary}
            disabled={!canSummary}
            loading={summaryLoading}
          >
            刷新统计
          </Button>,
        ]}
        request={async (params, sort) => {
          if (!canList) return { success: true, data: [], total: 0 };

          const createdRange = (params as any).created_range as
            | [string, string]
            | undefined;
          const lastActiveRange = (params as any).last_active_range as
            | [string, string]
            | undefined;

          const keyword = (params as any).keyword as string | undefined;
          const device_id = (params as any).device_id as string | undefined;

          const body: any = {
            page: params.current,
            page_size: params.pageSize,
            keyword: keyword || undefined,
            device_id: device_id || undefined,
            platform: params.platform || undefined,
            os_version: (params as any).os_version_search || undefined,
            app_version: (params as any).app_version_search || undefined,
            device_model: (params as any).device_model || undefined,
            device_name: (params as any).device_name || undefined,
            locale: (params as any).locale || undefined,
            network: (params as any).network || undefined,
            address_status: (params as any).address_status || undefined,
            wallet_count_min: (params as any).wallet_count_min ?? undefined,
            wallet_count_max: (params as any).wallet_count_max ?? undefined,
            created_from: createdRange?.[0],
            created_to: createdRange?.[1],
            last_active_from: lastActiveRange?.[0],
            last_active_to: lastActiveRange?.[1],
            sort_by: (params as any).sort_by || undefined,
            sort_order: (params as any).sort_order || undefined,
          };

          const picked = pickSortForList(sort as any);
          if (picked.sort_by) body.sort_by = picked.sort_by;
          if (picked.sort_order) body.sort_order = picked.sort_order;

          try {
            const res = (await adminGetWeb3UsersPageList(body, {
              skipErrorHandler: true,
            })) as GatewayResp<Web3UsersListResp>;
            if (!res?.success) throw new Error(res?.message || "加载失败");
            const items = res.data?.items ?? [];
            const total = Number(res.data?.pagination?.total ?? 0);
            return { success: true, data: items, total };
          } catch (e: any) {
            message.error(e?.message || "加载失败");
            return { success: false, data: [], total: 0 };
          }
        }}
      />

      <Drawer
        title="Web3 用户详情"
        open={drawerOpen}
        width={1100}
        onClose={() => {
          setDrawerOpen(false);
          setCurrentDeviceId(null);
          setDetail(null);
          setAddrTarget(null);
          setOpDeviceId(null);
        }}
        destroyOnClose
      >
        <div style={{ marginBottom: 12 }}>
          <Space>
            <Button
              onClick={() => {
                if (!currentDeviceId) return;
                refreshDetail(currentDeviceId);
              }}
              loading={detailLoading}
              disabled={!canDetail}
            >
              刷新详情
            </Button>
          </Space>
        </div>

        <Card size="small" loading={detailLoading} style={{ marginBottom: 12 }}>
          <Descriptions size="small" column={2} bordered>
            <Descriptions.Item label="用户ID">
              {detail?.id || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="设备ID">
              {detail?.device_id || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="平台">
              {detail?.device_info?.platform || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="系统版本">
              {detail?.device_info?.os_version || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="App版本">
              {detail?.device_info?.app_version || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="设备型号">
              {detail?.device_info?.device_model || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="设备名称">
              {detail?.device_info?.device_name || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="语言/地区">
              {detail?.device_info?.locale || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="最后活跃">
              {fmtTime(
                detail?.device_info?.last_active_at || detail?.last_active_at
              )}
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">
              {fmtTime(detail?.device_info?.created_at || detail?.created_at)}
            </Descriptions.Item>
          </Descriptions>
        </Card>

        {/* <Card
          size="small"
          loading={detailLoading}
          style={{ marginBottom: 12 }}
          title="活动统计"
        >
          <Descriptions size="small" column={2} bordered>
            <Descriptions.Item label="总交易数">
              {detail?.activity_summary?.total_transactions || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="总兑换(USD)">
              {detail?.activity_summary?.total_swap_usd || "—"}
            </Descriptions.Item>
            <Descriptions.Item label="首次交易" span={2}>
              {fmtTime(detail?.activity_summary?.first_transaction_at)}
            </Descriptions.Item>
            <Descriptions.Item label="最后交易" span={2}>
              {fmtTime(detail?.activity_summary?.last_transaction_at)}
            </Descriptions.Item>
          </Descriptions>
        </Card> */}

        <Card
          size="small"
          loading={detailLoading}
          style={{ marginBottom: 12 }}
          title="钱包地址"
        >
          <ProTable<NonNullable<Web3UserDetail["wallet_addresses"]>[number]>
            rowKey={(r) => `${r.network || ""}-${r.address || ""}`}
            columns={addressColumns}
            search={false}
            pagination={{ pageSize: 8 }}
            dataSource={detail?.wallet_addresses || []}
            scroll={{ x: 1200 }}
            toolBarRender={false}
          />
        </Card>
      </Drawer>

      <Modal
        title={`加入黑名单${
          addrTarget?.network ? ` (${addrTarget.network})` : ""
        }`}
        open={blacklistOpen}
        onCancel={() => {
          setBlacklistOpen(false);
          setAddrTarget(null);
          setOpDeviceId(null);
          blacklistForm.resetFields();
        }}
        onOk={submitBlacklist}
        okText="确认"
        cancelText="取消"
        confirmLoading={blacklistSubmitting}
        okButtonProps={{ disabled: !canBlacklist }}
        destroyOnClose
      >
        <Form form={blacklistForm} layout="vertical" preserve={false}>
          <Form.Item
            name="reason"
            label="原因"
            rules={[{ required: true, message: "请输入原因" }]}
          >
            <Input.TextArea rows={4} placeholder="请输入黑名单原因" />
          </Form.Item>
          <Form.Item
            name="risk_level"
            label="风险级别"
            rules={[{ required: true, message: "请选择风险级别" }]}
          >
            <Select
              options={[
                { label: "低", value: "low" },
                { label: "中", value: "medium" },
                { label: "高", value: "high" },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="disable_address"
            label="禁用地址"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name="notify_user"
            label="通知用户"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`移出黑名单${
          addrTarget?.network ? ` (${addrTarget.network})` : ""
        }`}
        open={unblacklistOpen}
        onCancel={() => {
          setUnblacklistOpen(false);
          setAddrTarget(null);
          setOpDeviceId(null);
          unblacklistForm.resetFields();
        }}
        onOk={submitUnblacklist}
        okText="确认"
        cancelText="取消"
        confirmLoading={unblacklistSubmitting}
        okButtonProps={{ disabled: !canUnblacklist }}
        destroyOnClose
      >
        <Form form={unblacklistForm} layout="vertical" preserve={false}>
          <Form.Item
            name="reason"
            label="原因"
            rules={[{ required: true, message: "请输入移出原因" }]}
          >
            <Input.TextArea rows={4} placeholder="请输入移出原因" />
          </Form.Item>
          <Form.Item
            name="enable_address"
            label="启用地址"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name="notify_user"
            label="通知用户"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`交易记录${txTarget?.network ? ` (${txTarget.network})` : ""}${
          txTarget?.address ? ` - ${txTarget.address}` : ""
        }`}
        open={txModalOpen}
        onCancel={() => {
          setTxModalOpen(false);
          setTxTarget(null);
        }}
        footer={null}
        width={1200}
        destroyOnClose
      >
        <ProTable<TxItem>
          actionRef={txActionRef}
          rowKey={(r) =>
            r.id || r.tx_hash || `${r.block_number || ""}-${r.created_at || ""}`
          }
          columns={txColumns}
          scroll={{ x: 1900 }}
          search={{ labelWidth: 110 }}
          toolBarRender={false}
          // ✅ 如果你 date_range 想稳定是 string，建议加上这行
          // dateFormatter="string"

          // ✅ 注入参数：全部用 _xxx，避免覆盖表单字段
          params={{
            _deviceId: txTarget?.deviceId,
            _userAddress: txTarget?.address,
            // ❌ 不要注入 network（会覆盖搜索字段）
            // 如果你必须固定 network，也用 _networkFixed
            // _networkFixed: txTarget?.network,
          }}
          request={async (params) => {
            if (!canTxList) return { success: true, data: [], total: 0 };

            // ✅ 从注入参数取 deviceId / address（避免闭包 + 不撞字段）
            const deviceId = (params as any)._deviceId as string | undefined;
            const fixedUserAddress = (params as any)._userAddress as
              | string
              | undefined;

            if (!deviceId) return { success: true, data: [], total: 0 };

            const dateRange = (params as any).date_range as
              | [string, string]
              | undefined;

            const q: any = {
              page: params.current,
              page_size: params.pageSize,

              // ✅ 搜索字段（来自表单）
              network: (params as any).network_search || undefined,
              tx_type: (params as any).tx_type_search || undefined,
              direction: (params as any).direction_search || undefined,
              asset_code: (params as any).asset_code_search || undefined,
              status: (params as any).status_search || undefined,
              date_from: dateRange?.[0],
              date_to: dateRange?.[1],

              sort_by: (params as any).sort_by || "block_time",
              sort_order: (params as any).sort_order || "desc",

              // ✅ 固定过滤（来自弹窗选择）
              user_address: fixedUserAddress || undefined,
            };

            try {
              const res = (await adminListWeb3UserTransactions(
                { deviceId },
                q,
                { skipErrorHandler: true }
              )) as GatewayResp<TxListResp>;

              if (!res?.success) throw new Error(res?.message || "加载失败");
              const items = res.data?.items ?? [];
              const total = Number(res.data?.pagination?.total ?? 0);
              return { success: true, data: items, total };
            } catch (e: any) {
              message.error(e?.message || "加载失败");
              return { success: false, data: [], total: 0 };
            }
          }}
        />
      </Modal>
    </PageContainer>
  );
};

export default Web3UsersPage;
