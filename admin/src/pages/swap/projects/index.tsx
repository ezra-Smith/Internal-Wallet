import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import {
  ModalForm,
  PageContainer,
  ProFormDateTimePicker,
  ProFormDependency,
  ProFormSwitch,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { App, Button, Modal, Space, Tag, Typography } from 'antd';
import React, { useMemo, useRef, useState } from 'react';
import dayjs from 'dayjs';
import {
  adminCreateSwapApiKey,
  adminDisableSwapConfigForProject,
  adminEnableSwapConfigForProject,
  adminListProjectSwapConfigs,
  adminListSwapApiKeys,
  adminListSwapConfigs,
  adminRevokeSwapApiKey,
} from '@/api/generated/swap';
import type { AdminAdminCreateSwapApiKeyRequest } from '@/api/generated/schemas';
import { useRbac } from '@/hooks/useRbac';

const { Text } = Typography;

const PERM = {
  listKeys: 'ListSwapApiKeys',
  createKey: 'CreateSwapApiKey',
  revokeKey: 'RevokeSwapApiKey',
  listGlobalConfigs: 'ListSwapConfigs',
  listProjectConfigs: 'ListProjectSwapConfigs',
  enableProjectConfig: 'EnableSwapConfigForProject',
  disableProjectConfig: 'DisableSwapConfigForProject',
};

type ApiKeyItem = {
  id?: string;
  projectName?: string;
  keyName?: string;
  rateLimitTier?: string;
  isActive?: boolean;
  createdAtUnix?: string;
  expiresAtUnix?: string;
  tokenDisplay?: string;
};

type GlobalConfigItem = {
  id?: string;
  provider_id?: string;
  provider?: string;
  provider_name?: string;
  chain_id?: string;
  chain_name?: string;
  chain_symbol?: string;
  token_symbol?: string;
  token_name?: string;
  contract_address?: string;
  decimals?: number;
  icon_url?: string;
  is_enabled?: boolean;
  priority?: number;
  router_address?: string;
  min_swap_amount_usd?: string;
  max_swap_amount_usd?: string;
  default_slippage?: string;
  max_slippage?: string;
  fee_rate?: string;
  config_json?: string;
  created_at?: string;
  updated_at?: string;
};

type ProjectConfigItem = {
  enabledId?: string;
  projectName?: string;
  configId?: string;
  projectEnabled?: boolean;
  provider?: string;
  providerName?: string;
  chainId?: string;
  chainName?: string;
  chainSymbol?: string;
  tokenSymbol?: string;
  tokenName?: string;
  contractAddress?: string;
  decimals?: number;
  iconUrl?: string;
  globalEnabled?: boolean;
  priority?: number;
  routerAddress?: string;
  minSwapAmountUsd?: string;
  maxSwapAmountUsd?: string;
  defaultSlippage?: string;
  maxSlippage?: string;
  feeRate?: string;
  createdAt?: string;
  updatedAt?: string;
};

type CreateFormValues = {
  projectName?: string;
  keyName?: string;
  rateLimitTier?: string;
  neverExpire?: boolean;
  expiresAt?: any;
};

const formatUnixSeconds = (sec?: string) => {
  const n = Number(sec);
  if (!Number.isFinite(n) || n <= 0) return '-';
  const d = new Date(n * 1000);
  const pad = (x: number) => String(x).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(
    d.getSeconds(),
  )}`;
};

const normalizeApiKeyItem = (it: any): ApiKeyItem => ({
  id: String(it?.id ?? '').trim() || undefined,
  projectName: String(it?.project_name ?? it?.projectName ?? '').trim() || undefined,
  keyName: String(it?.key_name ?? it?.keyName ?? '').trim() || undefined,
  rateLimitTier: String(it?.rate_limit_tier ?? it?.rateLimitTier ?? '').trim() || undefined,
  isActive: typeof it?.is_active === 'boolean' ? it.is_active : !!it?.isActive,
  createdAtUnix: String(it?.created_at_unix ?? it?.createdAtUnix ?? '').trim() || undefined,
  expiresAtUnix: String(it?.expires_at_unix ?? it?.expiresAtUnix ?? '').trim() || undefined,
  tokenDisplay: String(it?.token_display ?? it?.tokenDisplay ?? '').trim() || undefined,
});

const normalizeGlobalConfigItem = (it: any): GlobalConfigItem => ({
  id: String(it?.id ?? '').trim() || undefined,
  provider_id: String(it?.provider_id ?? '').trim() || undefined,
  provider: String(it?.provider ?? '').trim() || undefined,
  provider_name: String(it?.provider_name ?? '').trim() || undefined,
  chain_id: String(it?.chain_id ?? '').trim() || undefined,
  chain_name: String(it?.chain_name ?? '').trim() || undefined,
  chain_symbol: String(it?.chain_symbol ?? '').trim() || undefined,
  token_symbol: String(it?.token_symbol ?? '').trim() || undefined,
  token_name: String(it?.token_name ?? '').trim() || undefined,
  contract_address: String(it?.contract_address ?? '').trim() || undefined,
  decimals: it?.decimals ?? undefined,
  icon_url: String(it?.icon_url ?? '').trim() || undefined,
  is_enabled: typeof it?.is_enabled === 'boolean' ? it.is_enabled : !!it?.is_enabled,
  priority: it?.priority ?? undefined,
  router_address: String(it?.router_address ?? '').trim() || undefined,
  min_swap_amount_usd: String(it?.min_swap_amount_usd ?? '').trim() || undefined,
  max_swap_amount_usd: String(it?.max_swap_amount_usd ?? '').trim() || undefined,
  default_slippage: String(it?.default_slippage ?? '').trim() || undefined,
  max_slippage: String(it?.max_slippage ?? '').trim() || undefined,
  fee_rate: String(it?.fee_rate ?? '').trim() || undefined,
  config_json: String(it?.config_json ?? '').trim() || undefined,
  created_at: String(it?.created_at ?? '').trim() || undefined,
  updated_at: String(it?.updated_at ?? '').trim() || undefined,
});

const normalizeProjectConfigItem = (it: any): ProjectConfigItem => ({
  enabledId: String(it?.enabledId ?? it?.enabled_id ?? '').trim() || undefined,
  projectName: String(it?.projectName ?? it?.project_name ?? '').trim() || undefined,
  configId: String(it?.configId ?? it?.config_id ?? '').trim() || undefined,
  projectEnabled:
    typeof it?.projectEnabled === 'boolean'
      ? it.projectEnabled
      : typeof it?.project_enabled === 'boolean'
        ? it.project_enabled
        : !!it?.projectEnabled,
  provider: String(it?.provider ?? '').trim() || undefined,
  providerName: String(it?.providerName ?? it?.provider_name ?? '').trim() || undefined,
  chainId: String(it?.chainId ?? it?.chain_id ?? '').trim() || undefined,
  chainName: String(it?.chainName ?? it?.chain_name ?? '').trim() || undefined,
  chainSymbol: String(it?.chainSymbol ?? it?.chain_symbol ?? '').trim() || undefined,
  tokenSymbol: String(it?.tokenSymbol ?? it?.token_symbol ?? '').trim() || undefined,
  tokenName: String(it?.tokenName ?? it?.token_name ?? '').trim() || undefined,
  contractAddress: String(it?.contractAddress ?? it?.contract_address ?? '').trim() || undefined,
  decimals: it?.decimals ?? undefined,
  iconUrl: String(it?.iconUrl ?? it?.icon_url ?? '').trim() || undefined,
  globalEnabled:
    typeof it?.globalEnabled === 'boolean'
      ? it.globalEnabled
      : typeof it?.global_enabled === 'boolean'
        ? it.global_enabled
        : !!it?.globalEnabled,
  priority: it?.priority ?? undefined,
  routerAddress: String(it?.routerAddress ?? it?.router_address ?? '').trim() || undefined,
  minSwapAmountUsd: String(it?.minSwapAmountUsd ?? it?.min_swap_amount_usd ?? '').trim() || undefined,
  maxSwapAmountUsd: String(it?.maxSwapAmountUsd ?? it?.max_swap_amount_usd ?? '').trim() || undefined,
  defaultSlippage: String(it?.defaultSlippage ?? it?.default_slippage ?? '').trim() || undefined,
  maxSlippage: String(it?.maxSlippage ?? it?.max_slippage ?? '').trim() || undefined,
  feeRate: String(it?.feeRate ?? it?.fee_rate ?? '').trim() || undefined,
  createdAt: String(it?.createdAt ?? it?.created_at ?? '').trim() || undefined,
  updatedAt: String(it?.updatedAt ?? it?.updated_at ?? '').trim() || undefined,
});

const SwapProjectsPage: React.FC = () => {
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();

  const canListKeys = canRpc(PERM.listKeys);
  const canCreateKey = canRpc(PERM.createKey);
  const canRevokeKey = canRpc(PERM.revokeKey);

  const canListGlobalConfigs = canRpc(PERM.listGlobalConfigs);
  const canListProjectConfigs = canRpc(PERM.listProjectConfigs);
  const canEnableProjectConfig = canRpc(PERM.enableProjectConfig);
  const canDisableProjectConfig = canRpc(PERM.disableProjectConfig);

  const actionRef = useRef<ActionType | null>(null);
  const createFormRef = useRef<ProFormInstance<CreateFormValues> | undefined>(undefined);

  const globalCfgActionRef = useRef<ActionType | null>(null);
  const projectCfgActionRef = useRef<ActionType | null>(null);

  const [createOpen, setCreateOpen] = useState(false);
  const [cfgOpen, setCfgOpen] = useState(false);
  const [cfgProjectName, setCfgProjectName] = useState<string>('');

  const openCreate = () => {
    if (!canCreateKey) return;
    setCreateOpen(true);
    setTimeout(() => createFormRef.current?.resetFields(), 0);
  };

  const openProjectConfigs = (projectName?: string) => {
    const pn = String(projectName || '').trim();
    if (!pn) {
      message.error('缺少 projectName');
      return;
    }
    if (!canListProjectConfigs) return;
    setCfgProjectName(pn);
    setCfgOpen(true);
    setTimeout(() => {
      globalCfgActionRef.current?.reload();
      projectCfgActionRef.current?.reload();
    }, 0);
  };

  const keysColumns = useMemo(() => {
    return [
      { title: 'ID', dataIndex: 'id', width: 170, copyable: true, fixed: 'left', hideInSearch: true },
      { title: '项目', dataIndex: 'projectName', width: 180, hideInSearch: true },
      { title: '名称', dataIndex: 'keyName', width: 160, hideInSearch: true, render: (v: any) => (v ? String(v) : '-') },
      {
        title: 'Token',
        dataIndex: 'tokenDisplay',
        width: 520,
        hideInSearch: true,
        render: (v: any) => {
          const s = String(v || '').trim();
          if (!s) return '-';
          return (
            <Space size={8}>
              <Text code>{s}</Text>
              <Button
                type="text"
                size="small"
                onClick={() => {
                  navigator.clipboard.writeText(s);
                  message.success('已复制');
                }}
              >
                复制
              </Button>
            </Space>
          );
        },
      },
      {
        title: '限流等级',
        dataIndex: 'rateLimitTier',
        width: 140,
        hideInSearch: true,
        render: (v: any) => (v ? <Tag>{String(v)}</Tag> : '-'),
      },
      {
        title: '状态',
        dataIndex: 'isActive',
        width: 120,
        hideInSearch: true,
        render: (v: any) => (v ? <Tag color="success">启用</Tag> : <Tag color="default">已撤销</Tag>),
      },
      {
        title: '创建时间',
        dataIndex: 'createdAtUnix',
        width: 180,
        hideInSearch: true,
        render: (v: any) => formatUnixSeconds(String(v || '')),
      },
      {
        title: '过期时间',
        dataIndex: 'expiresAtUnix',
        width: 190,
        hideInSearch: true,
        render: (v: any) => {
          const s = String(v || '').trim();
          if (!s || s === '0') return '永不过期';
          return formatUnixSeconds(s);
        },
      },
      {
        title: '操作',
        valueType: 'option',
        width: 220,
        fixed: 'right',
        render: (_, row) => {
          const id = String(row?.id || '').trim();
          const projectName = String(row?.projectName || '').trim();
          const active = !!row?.isActive;

          const nodes: React.ReactNode[] = [];

          nodes.push(
            <a
              key="configs"
              onClick={() => openProjectConfigs(projectName)}
              style={{ color: canListProjectConfigs ? undefined : 'rgba(0,0,0,0.25)' }}
            >
              项目配置
            </a>,
          );

          nodes.push(
            <a
              key="revoke"
              onClick={() => {
                if (!canRevokeKey) return;
                if (!id) return;
                if (!active) return;

                modal.confirm({
                  title: '确认撤销该密钥？',
                  content: (
                    <div style={{ paddingTop: 8 }}>
                      <div style={{ marginBottom: 8 }}>
                        项目：<Text strong>{projectName || '-'}</Text>
                      </div>
                      <div>
                        Token：<Text code>{String(row?.tokenDisplay || '-')}</Text>
                      </div>
                    </div>
                  ),
                  okText: '撤销',
                  cancelText: '取消',
                  okButtonProps: { danger: true },
                  onOk: async () => {
                    const res: any = await adminRevokeSwapApiKey({ id } as any, {} as any, { skipErrorHandler: true });
                    if (!res?.success) throw new Error(res?.message || '撤销失败');
                    message.success('已撤销');
                    actionRef.current?.reload();
                  },
                });
              }}
              style={{ color: canRevokeKey && active ? undefined : 'rgba(0,0,0,0.25)' }}
            >
              撤销
            </a>,
          );

          return nodes;
        },
      },
    ] as ProColumns<ApiKeyItem>[];
  }, [canListProjectConfigs, canRevokeKey, modal, message]);

  const onCreateFinish = async (values: CreateFormValues) => {
    try {
      if (!canCreateKey) return false;

      const projectName = String(values.projectName || '').trim();
      if (!projectName) {
        message.error('projectName 必填');
        return false;
      }

      const keyName = values.keyName ? String(values.keyName).trim() : undefined;
      const rateLimitTier = values.rateLimitTier ? String(values.rateLimitTier).trim() : undefined;

      const neverExpire = values.neverExpire !== false;
      let expiresAtUnix = '0';

      if (!neverExpire) {
        const sec = dayjs(values.expiresAt).unix();
        if (!Number.isFinite(sec) || sec <= 0) {
          message.error('请选择过期时间');
          return false;
        }
        expiresAtUnix = String(sec);
      }

      const body: AdminAdminCreateSwapApiKeyRequest = { projectName, keyName, rateLimitTier, expiresAtUnix } as any;

      const res: any = await adminCreateSwapApiKey(body, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || '创建失败');

      setCreateOpen(false);
      createFormRef.current?.resetFields();
      message.success('创建成功');
      actionRef.current?.reload();

      const data = res?.data || res || {};
      const apiKey = String(data?.api_key ?? data?.apiKey ?? data?.token_display ?? data?.tokenDisplay ?? '').trim();

      if (apiKey) {
        modal.info({
          title: '密钥已创建（仅显示一次）',
          width: 760,
          content: (
            <div style={{ paddingTop: 8 }}>
              <Space direction="vertical" style={{ width: '100%' }} size={10}>
                <div style={{ color: 'rgba(0,0,0,0.45)' }}>API Key</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <Text code style={{ flex: 1, wordBreak: 'break-all' }}>
                    {apiKey}
                  </Text>
                  <Button
                    onClick={() => {
                      navigator.clipboard.writeText(apiKey);
                      message.success('已复制');
                    }}
                  >
                    复制
                  </Button>
                </div>
              </Space>
            </div>
          ),
          okText: '我已保存',
        });
      }

      return true;
    } catch (e: any) {
      message.error(e?.message || '创建失败');
      return false;
    }
  };

  const globalCfgColumns = useMemo(() => {
    return [
      { title: 'provider', dataIndex: 'provider', hideInTable: true, valueType: 'text' },
      { title: 'chain_id', dataIndex: 'chain_id', hideInTable: true, valueType: 'text' },
      { title: 'token_symbol', dataIndex: 'token_symbol', hideInTable: true, valueType: 'text' },
      { title: '启用', dataIndex: 'is_enabled', hideInTable: true, valueType: 'select', valueEnum: { true: { text: '是' }, false: { text: '否' } } },

      {
        title: '全局状态',
        dataIndex: 'is_enabled',
        width: 110,
        hideInSearch: true,
        render: (v: any) => (v ? <Tag color="success">启用</Tag> : <Tag color="default">禁用</Tag>),
      },
      {
        title: 'Provider',
        dataIndex: 'provider',
        width: 160,
        hideInSearch: true,
        render: (_: any, row: GlobalConfigItem) => (
          <Space size={6}>
            <Text code>{String(row.provider || '-')}</Text>
            {row.provider_name ? <Text type="secondary">{String(row.provider_name)}</Text> : null}
          </Space>
        ),
      },
      {
        title: '链',
        dataIndex: 'chain_name',
        width: 170,
        hideInSearch: true,
        render: (_: any, row: GlobalConfigItem) => (
          <Space size={6}>
            <Text>{row.chain_name || '-'}</Text>
            {row.chain_id ? <Text code>{String(row.chain_id)}</Text> : null}
          </Space>
        ),
      },
      {
        title: 'Token',
        dataIndex: 'token_symbol',
        width: 210,
        hideInSearch: true,
        render: (_: any, row: GlobalConfigItem) => (
          <Space size={6}>
            <Text strong>{row.token_symbol || '-'}</Text>
            {row.token_name ? <Text type="secondary">{String(row.token_name)}</Text> : null}
          </Space>
        ),
      },
      {
        title: '合约',
        dataIndex: 'contract_address',
        width: 300,
        hideInSearch: true,
        render: (v: any) => {
          const s = String(v || '').trim();
          if (!s) return '-';
          return (
            <Space size={8}>
              <Text code>{s}</Text>
              <Button
                type="text"
                size="small"
                onClick={() => {
                  navigator.clipboard.writeText(s);
                  message.success('已复制');
                }}
              >
                复制
              </Button>
            </Space>
          );
        },
      },
      {
        title: '优先级',
        dataIndex: 'priority',
        width: 100,
        hideInSearch: true,
        render: (v: any) => (v === null || v === undefined ? '-' : String(v)),
      },
      {
        title: '操作',
        valueType: 'option',
        width: 110,
        fixed: 'right',
        render: (_, row: GlobalConfigItem) => {
          const configId = String(row?.id || '').trim();
          const projectName = String(cfgProjectName || '').trim();
          const globalOk = !!row?.is_enabled;

          return (
            <a
              onClick={() => {
                if (!canEnableProjectConfig) return;
                if (!projectName) return;
                if (!configId) return;
                if (!globalOk) return;

                Modal.confirm({
                  title: '确认为项目启用该配置？',
                  content: (
                    <div style={{ paddingTop: 8 }}>
                      <div style={{ marginBottom: 6 }}>
                        项目：<Text strong>{projectName}</Text>
                      </div>
                      <div style={{ marginBottom: 6 }}>
                        Provider：<Text code>{row.provider || '-'}</Text>
                      </div>
                      <div>
                        Token：<Text code>{row.token_symbol || '-'}</Text>
                      </div>
                    </div>
                  ),
                  okText: '启用',
                  cancelText: '取消',
                  onOk: async () => {
                    const res: any = await adminEnableSwapConfigForProject(
                      { projectName } as any,
                      { configId, isEnabled: true } as any,
                      { skipErrorHandler: true },
                    );
                    if (!res?.success) throw new Error(res?.message || '启用失败');
                    message.success('已启用');
                    projectCfgActionRef.current?.reload();
                  },
                });
              }}
              style={{ color: canEnableProjectConfig && globalOk && projectName && configId ? undefined : 'rgba(0,0,0,0.25)' }}
            >
              启用
            </a>
          );
        },
      },
    ] as ProColumns<GlobalConfigItem>[];
  }, [canEnableProjectConfig, cfgProjectName, message]);

  const projectCfgColumns = useMemo(() => {
    return [
      {
        title: '状态',
        dataIndex: 'projectEnabled',
        width: 120,
        hideInSearch: true,
        render: (v: any, row: ProjectConfigItem) => {
          if (!row?.globalEnabled) return <Tag color="default">全局禁用</Tag>;
          return v ? <Tag color="success">项目启用</Tag> : <Tag color="warning">项目未启用</Tag>;
        },
      },
      {
        title: 'Provider',
        dataIndex: 'provider',
        width: 170,
        hideInSearch: true,
        render: (v: any, row: ProjectConfigItem) => (
          <Space size={6}>
            <Text code>{String(v || '-')}</Text>
            {row?.providerName ? <Text type="secondary">{String(row.providerName)}</Text> : null}
          </Space>
        ),
      },
      {
        title: '链',
        dataIndex: 'chainName',
        width: 190,
        hideInSearch: true,
        render: (_: any, row: ProjectConfigItem) => (
          <Space size={6}>
            <Text>{row.chainName || '-'}</Text>
            {row.chainId ? <Text code>{String(row.chainId)}</Text> : null}
          </Space>
        ),
      },
      {
        title: 'Token',
        dataIndex: 'tokenSymbol',
        width: 240,
        hideInSearch: true,
        render: (_: any, row: ProjectConfigItem) => (
          <Space size={6}>
            <Text strong>{row.tokenSymbol || '-'}</Text>
            {row.tokenName ? <Text type="secondary">{String(row.tokenName)}</Text> : null}
          </Space>
        ),
      },
      {
        title: '合约',
        dataIndex: 'contractAddress',
        width: 340,
        hideInSearch: true,
        render: (v: any) => {
          const s = String(v || '').trim();
          if (!s) return '-';
          return (
            <Space size={8}>
              <Text code>{s}</Text>
              <Button
                type="text"
                size="small"
                onClick={() => {
                  navigator.clipboard.writeText(s);
                  message.success('已复制');
                }}
              >
                复制
              </Button>
            </Space>
          );
        },
      },
      {
        title: '优先级',
        dataIndex: 'priority',
        width: 110,
        hideInSearch: true,
        render: (v: any) => (v === null || v === undefined ? '-' : String(v)),
      },
      {
        title: '操作',
        valueType: 'option',
        width: 110,
        fixed: 'right',
        render: (_, row: ProjectConfigItem) => {
          const enabled = !!row?.projectEnabled;
          const enabledId = String(row?.enabledId || '').trim();
          return (
            <a
              onClick={() => {
                if (!canDisableProjectConfig) return;
                if (!enabled || !enabledId) return;

                Modal.confirm({
                  title: '确认为项目禁用该配置？',
                  content: (
                    <div style={{ paddingTop: 8 }}>
                      <div style={{ marginBottom: 6 }}>
                        项目：<Text strong>{row.projectName || cfgProjectName || '-'}</Text>
                      </div>
                      <div style={{ marginBottom: 6 }}>
                        Provider：<Text code>{row.provider || '-'}</Text>
                      </div>
                      <div>
                        Token：<Text code>{row.tokenSymbol || '-'}</Text>
                      </div>
                    </div>
                  ),
                  okText: '禁用',
                  cancelText: '取消',
                  okButtonProps: { danger: true },
                  onOk: async () => {
                    const res: any = await adminDisableSwapConfigForProject({ id: enabledId } as any, {} as any, { skipErrorHandler: true });
                    if (!res?.success) throw new Error(res?.message || '禁用失败');
                    message.success('已禁用');
                    projectCfgActionRef.current?.reload();
                  },
                });
              }}
              style={{ color: canDisableProjectConfig && enabled && enabledId ? undefined : 'rgba(0,0,0,0.25)' }}
            >
              禁用
            </a>
          );
        },
      },
    ] as ProColumns<ProjectConfigItem>[];
  }, [canDisableProjectConfig, cfgProjectName, message]);

  return (
    <PageContainer>
      <ProTable<ApiKeyItem>
        actionRef={actionRef}
        rowKey={(row) => row.id || row.tokenDisplay || `${row.projectName || ''}-${row.keyName || ''}`}
        columns={keysColumns}
        scroll={{ x: 1600 }}
        search={false}
        toolBarRender={() => [
          <Button key="create" type="primary" disabled={!canCreateKey} onClick={openCreate}>
            创建 API Key
          </Button>,
        ]}
        request={async (params) => {
          if (!canListKeys) return { success: true, data: [], total: 0 };
          try {
            const res: any = await adminListSwapApiKeys({ page: params.current, pageSize: params.pageSize } as any, { skipErrorHandler: true });
            if (!res?.success) throw new Error(res?.message || '加载失败');
            const data = res?.data || {};
            const items = Array.isArray(data?.items) ? data.items : [];
            const pg = data?.pagination || {};
            return { success: true, data: items.map(normalizeApiKeyItem), total: Number(pg?.total ?? 0) };
          } catch (e: any) {
            message.error(e?.message || '加载失败');
            return { success: false, data: [], total: 0 };
          }
        }}
      />

      <ModalForm<CreateFormValues>
        formRef={createFormRef}
        title="创建 Swap API Key"
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open);
          if (!open) createFormRef.current?.resetFields();
        }}
        modalProps={{ destroyOnClose: true, okText: '创建', cancelText: '取消' }}
        onFinish={onCreateFinish}
        initialValues={{ neverExpire: true }}
      >
        <ProFormText name="projectName" label="projectName" rules={[{ required: true, message: '请输入项目名称' }]} />
        <ProFormText name="keyName" label="keyName" />
        <ProFormText name="rateLimitTier" label="rateLimitTier" />
        <ProFormSwitch name="neverExpire" label="永不过期" />
        <ProFormDependency name={['neverExpire']}>
          {({ neverExpire }) =>
            neverExpire ? null : (
              <ProFormDateTimePicker name="expiresAt" label="过期时间" rules={[{ required: true, message: '请选择过期时间' }]} fieldProps={{ showNow: true }} />
            )
          }
        </ProFormDependency>
      </ModalForm>

      <Modal
        open={cfgOpen}
        title={`项目 Swap 配置关联：${cfgProjectName}`}
        width={1500}
        destroyOnClose
        footer={null}
        onCancel={() => setCfgOpen(false)}
      >
        <div style={{ display: 'flex', gap: 12 }}>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ marginBottom: 8, fontWeight: 600 }}>全局 Swap 配置</div>
            <ProTable<GlobalConfigItem>
              actionRef={globalCfgActionRef}
              rowKey={(row) => row.id || `${row.provider || ''}-${row.chain_id || ''}-${row.token_symbol || ''}`}
              columns={globalCfgColumns}
              scroll={{ x: 1200, y: 520 }}
              search={{ labelWidth: 90 }}
              options={false}
              pagination={{ pageSize: 10 }}
              request={async (params) => {
                if (!canListGlobalConfigs) return { success: true, data: [], total: 0 };
                try {
                  const q: any = {
                    page: params.current,
                    page_size: params.pageSize,
                    provider_id: (params as any)?.provider_id,
                    chain_id: (params as any)?.chain_id,
                    token_symbol: (params as any)?.token_symbol,
                    is_enabled:
                      (params as any)?.is_enabled === undefined
                        ? undefined
                        : String((params as any).is_enabled) === 'true'
                          ? true
                          : String((params as any).is_enabled) === 'false'
                            ? false
                            : undefined,
                  };

                  const res: any = await adminListSwapConfigs(q, { skipErrorHandler: true });
                  if (!res?.success) throw new Error(res?.message || '加载失败');

                  const data = res?.data || {};
                  const list = Array.isArray(data?.configs) ? data.configs : [];
                  const pg = data?.pagination || {};

                  return { success: true, data: list.map(normalizeGlobalConfigItem), total: Number(pg?.total ?? 0) };
                } catch (e: any) {
                  message.error(e?.message || '加载失败');
                  return { success: false, data: [], total: 0 };
                }
              }}
            />
          </div>

          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ marginBottom: 8, fontWeight: 600 }}>项目已关联配置</div>
            <ProTable<ProjectConfigItem>
              actionRef={projectCfgActionRef}
              rowKey={(row) => row.enabledId || row.configId || `${row.provider || ''}-${row.chainId || ''}-${row.tokenSymbol || ''}`}
              columns={projectCfgColumns}
              scroll={{ x: 1200, y: 520 }}
              search={false}
              options={false}
              pagination={{ pageSize: 10 }}
              request={async (params) => {
                if (!canListProjectConfigs) return { success: true, data: [], total: 0 };
                const projectName = String(cfgProjectName || '').trim();
                if (!projectName) return { success: true, data: [], total: 0 };

                try {
                  const q: any = { page: params.current, pageSize: params.pageSize, projectEnabled: true };
                  const res: any = await adminListProjectSwapConfigs({ projectName } as any, q, { skipErrorHandler: true });
                  if (!res?.success) throw new Error(res?.message || '加载失败');

                  const data = res?.data || {};
                  const list = Array.isArray(data?.configs) ? data.configs : Array.isArray(res?.configs) ? res.configs : [];
                  const pg = data?.pagination || res?.pagination || {};

                  return { success: true, data: list.map(normalizeProjectConfigItem), total: Number(pg?.total ?? 0) };
                } catch (e: any) {
                  message.error(e?.message || '加载失败');
                  return { success: false, data: [], total: 0 };
                }
              }}
            />
          </div>
        </div>  
      </Modal>
    </PageContainer>
  );
};

export default SwapProjectsPage;
