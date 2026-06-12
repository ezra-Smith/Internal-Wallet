import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import {
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { App, Button, Modal, Spin, Switch } from 'antd';
import React, { useMemo, useRef, useState } from 'react';
import {
  adminCreateSwapProvider,
  adminDeleteSwapProvider,
  adminDisableSwapProvider,
  adminEnableSwapProvider,
  adminGetSwapProviderDetail,
  adminListSwapProviders,
  adminUpdateSwapProvider,
} from '@/api/generated/swap';
import type {
  AdminAdminCreateSwapProviderRequest,
  AdminAdminSwapProviderItem,
  AdminUpdateSwapProviderBody,
} from '@/api/generated/schemas';
import { useRbac } from '@/hooks/useRbac';

const PERM = {
  list: 'ListSwapProviders',
  create: 'CreateSwapProvider',
  update: 'UpdateSwapProvider',
  delete: 'DeleteSwapProvider',
  enable: 'EnableSwapProvider',
  disable: 'DisableSwapProvider',
};

const BODY_STYLE: React.CSSProperties = {
  maxHeight: 560,
  overflowY: 'auto',
  paddingRight: 8,
};

const n2s = (v: any) => {
  if (v === null || v === undefined || v === '') return undefined;
  return typeof v === 'number' ? String(v) : String(v);
};

type ModalMode = 'create' | 'edit';

type CreateForm = Omit<
  AdminAdminCreateSwapProviderRequest,
  'min_swap_amount_usd' | 'max_swap_amount_usd' | 'default_slippage' | 'max_slippage' | 'fee_rate'
> & {
  min_swap_amount_usd?: number;
  max_swap_amount_usd?: number;
  default_slippage?: number;
  max_slippage?: number;
  fee_rate?: number;
};

type EditForm = Omit<
  AdminUpdateSwapProviderBody,
  'min_swap_amount_usd' | 'max_swap_amount_usd' | 'default_slippage' | 'max_slippage' | 'fee_rate'
> & {
  min_swap_amount_usd?: number;
  max_swap_amount_usd?: number;
  default_slippage?: number;
  max_slippage?: number;
  fee_rate?: number;
};

const SwapProvidersPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();

  const actionRef = useRef<ActionType | null>(null);
  const formRef = useRef<ProFormInstance<CreateForm & EditForm> | undefined>(undefined);

  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<ModalMode>('create');
  const [currentId, setCurrentId] = useState<string | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);

  const canList = canRpc(PERM.list);
  const canCreate = canRpc(PERM.create);
  const canUpdate = canRpc(PERM.update);
  const canDelete = canRpc(PERM.delete);
  const canEnable = canRpc(PERM.enable);
  const canDisable = canRpc(PERM.disable);

  const openCreate = () => {
    if (!canCreate) return;
    setModalMode('create');
    setCurrentId(null);
    setModalOpen(true);
    setTimeout(() => formRef.current?.resetFields(), 0);
  };

  const loadDetail = async (id: string) => {
    setDetailLoading(true);
    formRef.current?.resetFields();
    try {
      const res = await adminGetSwapProviderDetail({ id }, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || '加载详情失败');
      const p = res.data?.provider;
      if (!p) throw new Error('加载详情失败');
      formRef.current?.setFieldsValue({
        provider_name: p.provider_name,
        description: p.description,
        logo_url: p.logo_url,
        min_swap_amount_usd: p.min_swap_amount_usd ? Number(p.min_swap_amount_usd) : undefined,
        max_swap_amount_usd: p.max_swap_amount_usd ? Number(p.max_swap_amount_usd) : undefined,
        default_slippage: p.default_slippage ? Number(p.default_slippage) : undefined,
        max_slippage: p.max_slippage ? Number(p.max_slippage) : undefined,
        fee_rate: p.fee_rate ? Number(p.fee_rate) : undefined,
        config_json: p.config_json,
      } as any);
    } catch (e: any) {
      message.error(e?.message || '加载详情失败');
    } finally {
      setDetailLoading(false);
    }
  };

  const openEdit = async (row: AdminAdminSwapProviderItem) => {
    if (!canUpdate) return;
    if (!row.id) return;
    setModalMode('edit');
    setCurrentId(row.id);
    setModalOpen(true);
    await loadDetail(row.id);
  };

  const toggleProvider = (row: AdminAdminSwapProviderItem, next: boolean) => {
    if (!row.id) return;
    if (next && !canEnable) return;
    if (!next && !canDisable) return;

    Modal.confirm({
      title: next ? '确认启用' : '确认停用',
      content: next ? '确定要启用该服务商吗？' : '确定要停用该服务商吗？',
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          setTogglingId(row.id as string);
          const id = row.id as string;
          const res = next
            ? await adminEnableSwapProvider({ id }, {}, { skipErrorHandler: true })
            : await adminDisableSwapProvider({ id }, {}, { skipErrorHandler: true });
          if (!res?.success) throw new Error(res?.message || '操作失败');
          message.success('操作成功');
          actionRef.current?.reload();
        } catch (e: any) {
          message.error(e?.message || '操作失败');
        } finally {
          setTogglingId(null);
        }
      },
    });
  };

  const columns = useMemo(() => {
    return [
      { title: '服务商代码', dataIndex: 'provider_code', hideInTable: true },
      {
        title: '启用',
        dataIndex: 'is_enabled',
        hideInTable: true,
        valueType: 'select',
        valueEnum: { true: { text: '是' }, false: { text: '否' } },
      },

      { title: 'ID', dataIndex: 'id', copyable: true, width: 160, fixed: 'left', hideInSearch: true },
      { title: '服务商代码', dataIndex: 'provider_code', width: 140, hideInSearch: true },
      { title: '服务商名称', dataIndex: 'provider_name', width: 180, hideInSearch: true },
      { title: '描述', dataIndex: 'description', ellipsis: true, hideInSearch: true },
      { title: 'Logo URL', dataIndex: 'logo_url', ellipsis: true, hideInSearch: true },
      {
        title: '启用',
        dataIndex: 'is_enabled',
        width: 120,
        hideInSearch: true,
        render: (_, r) => (
          <Switch
            checked={!!r.is_enabled}
            loading={togglingId === r.id}
            disabled={(!!r.is_enabled && !canDisable) || (!r.is_enabled && !canEnable)}
            onChange={(checked) => toggleProvider(r, checked)}
          />
        ),
      },
      { title: '最小金额(USD)', dataIndex: 'min_swap_amount_usd', width: 140, hideInSearch: true },
      { title: '最大金额(USD)', dataIndex: 'max_swap_amount_usd', width: 140, hideInSearch: true },
      { title: '费率', dataIndex: 'fee_rate', width: 120, hideInSearch: true },
      { title: '默认滑点', dataIndex: 'default_slippage', width: 140, hideInSearch: true },
      { title: '最大滑点', dataIndex: 'max_slippage', width: 140, hideInSearch: true },
      { title: '更新时间', dataIndex: 'updated_at', width: 180, hideInSearch: true },
      {
        title: '操作',
        valueType: 'option',
        width: 150,
        fixed: 'right',
        render: (_, row) => {
          const nodes: React.ReactNode[] = [];
          nodes.push(
            <a key="edit" onClick={() => void openEdit(row)} style={{ color: canUpdate ? undefined : 'rgba(0,0,0,0.25)' }}>
              编辑
            </a>,
          );
          nodes.push(
            <a
              key="delete"
              style={{ color: canDelete ? undefined : 'rgba(0,0,0,0.25)' }}
              onClick={() => {
                if (!canDelete) return;
                if (!row.id) return;
                Modal.confirm({
                  title: '确认删除',
                  content: '确定要删除该服务商吗？',
                  okText: '删除',
                  cancelText: '取消',
                  okButtonProps: { danger: true },
                  onOk: async () => {
                    try {
                      const res = await adminDeleteSwapProvider({ id: row.id as string }, { skipErrorHandler: true });
                      if (!res?.success) throw new Error(res?.message || '删除失败');
                      message.success('删除成功');
                      actionRef.current?.reload();
                    } catch (e: any) {
                      message.error(e?.message || '删除失败');
                    }
                  },
                });
              }}
            >
              删除
            </a>,
          );
          return nodes;
        },
      },
    ] as ProColumns<AdminAdminSwapProviderItem>[];
  }, [canUpdate, canDelete, canEnable, canDisable, togglingId]);

  const onFinish = async (values: CreateForm & EditForm) => {
    try {
      if (modalMode === 'create') {
        if (!canCreate) return false;

        const body: AdminAdminCreateSwapProviderRequest = {
          provider_code: values.provider_code as string,
          provider_name: values.provider_name as string,
          description: values.description,
          logo_url: values.logo_url,
          min_swap_amount_usd: n2s(values.min_swap_amount_usd),
          max_swap_amount_usd: n2s(values.max_swap_amount_usd),
          default_slippage: n2s(values.default_slippage),
          max_slippage: n2s(values.max_slippage),
          fee_rate: n2s(values.fee_rate),
          config_json: values.config_json,
        };

        const res = await adminCreateSwapProvider(body, { skipErrorHandler: true });
        if (!res?.success) throw new Error(res?.message || '创建失败');
        message.success('创建成功');
      } else {
        if (!canUpdate) return false;
        const id = currentId;
        if (!id) return false;

        const body: AdminUpdateSwapProviderBody = {
          provider_name: values.provider_name,
          description: values.description,
          logo_url: values.logo_url,
          min_swap_amount_usd: values.min_swap_amount_usd !== undefined ? n2s(values.min_swap_amount_usd) : undefined,
          max_swap_amount_usd: values.max_swap_amount_usd !== undefined ? n2s(values.max_swap_amount_usd) : undefined,
          default_slippage: values.default_slippage !== undefined ? n2s(values.default_slippage) : undefined,
          max_slippage: values.max_slippage !== undefined ? n2s(values.max_slippage) : undefined,
          fee_rate: values.fee_rate !== undefined ? n2s(values.fee_rate) : undefined,
          config_json: values.config_json,
        };

        const res = await adminUpdateSwapProvider({ id }, body, { skipErrorHandler: true });
        if (!res?.success) throw new Error(res?.message || '更新失败');
        message.success('更新成功');
      }

      setModalOpen(false);
      setCurrentId(null);
      formRef.current?.resetFields();
      actionRef.current?.reload();
      return true;
    } catch (e: any) {
      message.error(e?.message || (modalMode === 'create' ? '创建失败' : '更新失败'));
      return false;
    }
  };

  return (
    <PageContainer>
      <ProTable<AdminAdminSwapProviderItem>
        actionRef={actionRef}
        rowKey={(row) => row.id || row.provider_code || JSON.stringify(row)}
        columns={columns}
        scroll={{ x: 1500 }}
        search={{ labelWidth: 110 }}
        toolBarRender={() => [
          <Button key="create" type="primary" disabled={!canCreate} onClick={openCreate}>
            新增服务商
          </Button>,
        ]}
        request={async (params) => {
          if (!canList) return { success: true, data: [], total: 0 };
          try {
            const res = await adminListSwapProviders(
              {
                page: params.current,
                page_size: params.pageSize,
                provider_code: (params as any).provider_code,
                is_enabled:
                  (params as any).is_enabled === undefined
                    ? undefined
                    : String((params as any).is_enabled) === 'true'
                      ? true
                      : String((params as any).is_enabled) === 'false'
                        ? false
                        : undefined,
              },
              { skipErrorHandler: true },
            );
            if (!res?.success) throw new Error(res?.message || '加载失败');
            return {
              success: true,
              data: res.data?.providers ?? [],
              total: Number(res.data?.pagination?.total ?? 0),
            };
          } catch (e: any) {
            message.error(e?.message || '加载失败');
            return { success: false, data: [], total: 0 };
          }
        }}
      />

      <ModalForm<CreateForm & EditForm>
        formRef={formRef}
        title={modalMode === 'create' ? '新增服务商' : '编辑服务商'}
        open={modalOpen}
        onOpenChange={(open) => {
          setModalOpen(open);
          if (!open) {
            setCurrentId(null);
            formRef.current?.resetFields();
          }
        }}
        modalProps={{
          destroyOnClose: true,
          okText: modalMode === 'create' ? '创建' : '保存',
          cancelText: '取消',
          bodyStyle: BODY_STYLE,
        }}
        onFinish={onFinish}
      >
        <Spin spinning={detailLoading} tip="加载中...">
          {modalMode === 'create' ? (
            <ProFormText
              name="provider_code"
              label="服务商代码"
              rules={[{ required: true, message: '请输入服务商代码' }]}
            />
          ) : null}

          <ProFormText
            name="provider_name"
            label="服务商名称"
            rules={[{ required: true, message: '请输入服务商名称' }]}
          />
          <ProFormTextArea name="description" label="描述" fieldProps={{ rows: 3 }} />
          <ProFormText name="logo_url" label="Logo URL" />

          <ProFormDigit name="min_swap_amount_usd" label="最小金额(USD)" min={0} fieldProps={{ precision: 8 }} />
          <ProFormDigit name="max_swap_amount_usd" label="最大金额(USD)" min={0} fieldProps={{ precision: 8 }} />
          <ProFormDigit name="fee_rate" label="费率" min={0} fieldProps={{ precision: 8 }} />
          <ProFormDigit name="default_slippage" label="默认滑点" min={0} fieldProps={{ precision: 8 }} />
          <ProFormDigit name="max_slippage" label="最大滑点" min={0} fieldProps={{ precision: 8 }} />
          <ProFormTextArea name="config_json" label="配置JSON" fieldProps={{ rows: 6 }} />
        </Spin>
      </ModalForm>
    </PageContainer>
  );
};

export default SwapProvidersPage;
