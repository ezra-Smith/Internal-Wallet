import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { App, Button, Drawer, Form, Input, Modal, Select, Space, Tag, Typography } from 'antd';
import { history, useIntl } from '@umijs/max';
import React, { useCallback, useMemo, useRef, useState } from 'react';
import {
  adminAddVaultAddress,
  adminDeleteVaultAddress,
  adminGetVaultAddressBalances,
  adminGetVaultOverview,
  adminListVaultAddresses,
  adminUpdateVaultAddressStatus,
} from '@/api/generated/vault';
import type {
  AdminAddVaultAddressRequest,
  AdminUpdateVaultAddressStatusBody,
  AdminVaultAddressBalanceItem,
  AdminVaultAddressItem,
  AdminVaultNetworkOverviewItem,
} from '@/api/generated/schemas';
import { useRbac } from '@/hooks/useRbac';

const PERM = {
  overview: 'GetVaultOverview',
  list: 'ListVaultAddresses',
  create: 'AddVaultAddress',
  updateStatus: 'UpdateVaultAddressStatus',
  remove: 'DeleteVaultAddress',
  balances: 'GetVaultAddressBalances',
};

const VaultAddressesPage: React.FC = () => {
  const { message } = App.useApp();
  const intl = useIntl();
  const { canRpc } = useRbac();

  const t = useCallback(
    (id: string, defaultMessage: string, values?: Record<string, any>) =>
      intl.formatMessage({ id, defaultMessage }, values),
    [intl],
  );

  const ADDRESS_TYPE_OPTIONS = useMemo(() => [
    { value: 'active', label: t('pages.vault.addresses.types.active', 'Active Wallet') },
    { value: 'hot', label: t('pages.vault.addresses.types.hot', 'Hot Wallet') },
    { value: 'cold', label: t('pages.vault.addresses.types.cold', 'Cold Wallet') },
    { value: 'deposit', label: t('pages.vault.addresses.types.deposit', 'Deposit') },
  ], [t]);

  const STATUS_OPTIONS = useMemo(() => [
    { value: 'active', label: t('pages.vault.addresses.status.active', 'Active') },
    { value: 'inactive', label: t('pages.vault.addresses.status.inactive', 'Inactive') },
    { value: 'disabled', label: t('pages.vault.addresses.status.disabled', 'Disabled') },
  ], [t]);

  const statusTag = useCallback((status: string | undefined) => {
    const s = (status || '').trim().toLowerCase();
    if (s === 'active') return <Tag color="green">{t('pages.vault.addresses.status.active', 'Active')}</Tag>;
    if (s === 'inactive') return <Tag color="default">{t('pages.vault.addresses.status.inactive', 'Inactive')}</Tag>;
    if (s === 'disabled') return <Tag color="red">{t('pages.vault.addresses.status.disabled', 'Disabled')}</Tag>;
    if (s) return <Tag>{s}</Tag>;
    return <Tag>{t('pages.vault.addresses.status.unknown', 'Unknown')}</Tag>;
  }, [t]);

  const typeTag = useCallback((type: string | undefined) => {
    const v = (type || '').trim().toLowerCase();
    if (v === 'active') return <Tag color="blue">{t('pages.vault.addresses.types.active', 'Active Wallet')}</Tag>;
    if (v === 'hot') return <Tag color="gold">{t('pages.vault.addresses.types.hot', 'Hot Wallet')}</Tag>;
    if (v === 'cold') return <Tag color="purple">{t('pages.vault.addresses.types.cold', 'Cold Wallet')}</Tag>;
    if (v === 'deposit') return <Tag>{t('pages.vault.addresses.types.deposit', 'Deposit')}</Tag>;
    if (v) return <Tag>{v}</Tag>;
    return <Tag>{t('pages.vault.addresses.status.unknown', 'Unknown')}</Tag>;
  }, [t]);

  const canOverview = canRpc(PERM.overview);
  const canList = canRpc(PERM.list);
  const canCreate = canRpc(PERM.create);
  const canUpdateStatus = canRpc(PERM.updateStatus);
  const canRemove = canRpc(PERM.remove);
  const canBalances = canRpc(PERM.balances);

  const actionRef = useRef<ActionType | undefined>(undefined);

  const [networkOptionsLoading, setNetworkOptionsLoading] = useState(false);
  const [networkOptions, setNetworkOptions] = useState<{ value: string; label: string }[]>([]);

  const loadNetworkOptions = useCallback(async () => {
    if (!canOverview) return;
    setNetworkOptionsLoading(true);
    try {
      const res = await adminGetVaultOverview({ skipErrorHandler: true });
      if (!res?.success) return;
      const list: AdminVaultNetworkOverviewItem[] = res.data?.networks ?? [];
      const opts = list
        .map((n) => {
          const id = (n.network_id || '').trim();
          const name = (n.network || '').trim();
          const chainId = (n.chain_id || '').trim();
          if (!id) return null;
          const label = chainId ? `${name || id} (chain ${chainId})` : name || id;
          return { value: id, label };
        })
        .filter((x): x is { value: string; label: string } => !!x);
      setNetworkOptions(opts);
    } catch {
      // ignore
    } finally {
      setNetworkOptionsLoading(false);
    }
  }, [canOverview]);

  const [addOpen, setAddOpen] = useState(false);
  const [addLoading, setAddLoading] = useState(false);
  const [addForm] = Form.useForm<AdminAddVaultAddressRequest>();

  const openAdd = useCallback(async () => {
    await loadNetworkOptions();
    addForm.resetFields();
    setAddOpen(true);
  }, [addForm, loadNetworkOptions]);

  const closeAdd = useCallback(() => {
    setAddOpen(false);
    addForm.resetFields();
  }, [addForm]);

  const submitAdd = useCallback(async () => {
    try {
      const values = await addForm.validateFields();
      setAddLoading(true);
      const payload: AdminAddVaultAddressRequest = {
        network_id: (values.network_id || '').trim(),
        address_type: (values.address_type || '').trim(),
        address: (values.address || '').trim(),
        label: (values.label || '').trim(),
      };
      const res = await adminAddVaultAddress(payload, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || t('pages.vault.addresses.messages.addFailed', 'Add failed'));
      message.success(t('pages.vault.addresses.messages.addSuccess', 'Added'));
      closeAdd();
      actionRef.current?.reload();
    } catch (e: any) {
      if (e?.errorFields) return;
      message.error(e?.message || t('pages.vault.addresses.messages.addFailed', 'Add failed'));
    } finally {
      setAddLoading(false);
    }
  }, [addForm, closeAdd, message, t]);

  const [balancesOpen, setBalancesOpen] = useState(false);
  const [balancesLoading, setBalancesLoading] = useState(false);
  const [balancesMeta, setBalancesMeta] = useState<{ address?: string; network?: string; totalUsd?: string } | null>(null);
  const [balances, setBalances] = useState<AdminVaultAddressBalanceItem[]>([]);

  const openBalances = useCallback(async (row: AdminVaultAddressItem) => {
    const addressId = (row.id || '').trim();
    if (!addressId) return;
    if (!canBalances) return;
    setBalancesOpen(true);
    setBalancesLoading(true);
    setBalancesMeta(null);
    setBalances([]);
    try {
      const res = await adminGetVaultAddressBalances({ addressId }, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || t('pages.vault.addresses.messages.loadBalancesFailed', 'Load balances failed'));
      setBalancesMeta({
        address: res.data?.address,
        network: res.data?.network,
        totalUsd: res.data?.total_balance_usd,
      });
      setBalances(res.data?.balances ?? []);
    } catch (e: any) {
      message.error(e?.message || t('pages.vault.addresses.messages.loadBalancesFailed', 'Load balances failed'));
    } finally {
      setBalancesLoading(false);
    }
  }, [canBalances, message, t]);

  const updateStatus = useCallback(
    async (row: AdminVaultAddressItem, body: AdminUpdateVaultAddressStatusBody) => {
      const id = (row.id || '').trim();
      if (!id) return;
      if (!canUpdateStatus) return;
      try {
        const res = await adminUpdateVaultAddressStatus({ id }, body, { skipErrorHandler: true });
        if (!res?.success) throw new Error(res?.message || t('pages.vault.addresses.messages.updateFailed', 'Update failed'));
        message.success(t('pages.vault.addresses.messages.updateSuccess', 'Updated'));
        actionRef.current?.reload();
      } catch (e: any) {
        message.error(e?.message || t('pages.vault.addresses.messages.updateFailed', 'Update failed'));
      }
    },
    [canUpdateStatus, message, t],
  );

  const removeAddress = useCallback(
    async (row: AdminVaultAddressItem) => {
      const id = (row.id || '').trim();
      if (!id) return;
      if (!canRemove) return;
      Modal.confirm({
        title: t('pages.vault.addresses.remove.title', 'Delete this address?'),
        content: t('pages.vault.addresses.remove.content', 'This will soft-delete the address from the vault monitoring list.'),
        okButtonProps: { danger: true },
        onOk: async () => {
          try {
            const res = await adminDeleteVaultAddress({ id }, { skipErrorHandler: true });
            if (!res?.success) throw new Error(res?.message || t('pages.vault.addresses.messages.deleteFailed', 'Delete failed'));
            message.success(t('pages.vault.addresses.messages.deleteSuccess', 'Deleted'));
            actionRef.current?.reload();
          } catch (e: any) {
            message.error(e?.message || t('pages.vault.addresses.messages.deleteFailed', 'Delete failed'));
          }
        },
      });
    },
    [canRemove, message, t],
  );

  const balanceColumns: ProColumns<AdminVaultAddressBalanceItem>[] = useMemo(() => {
    return [
      { title: t('pages.vault.addresses.balances.columns.currency', 'Currency'), dataIndex: 'currency', width: 120 },
      { title: t('pages.vault.addresses.balances.columns.contract', 'Contract'), dataIndex: 'contract_address', copyable: true, ellipsis: true },
      { title: t('pages.vault.addresses.balances.columns.balance', 'Balance'), dataIndex: 'balance', width: 140 },
      { title: t('pages.vault.addresses.balances.columns.usd', 'USD'), dataIndex: 'balance_usd', width: 140 },
      { title: t('pages.vault.addresses.balances.columns.lastSyncedAt', 'Last Synced At'), dataIndex: 'last_synced_at', width: 180 },
    ];
  }, [t]);

  const columns: ProColumns<AdminVaultAddressItem>[] = useMemo(() => {
    return [
      { title: t('pages.vault.addresses.columns.id', 'ID'), dataIndex: 'id', copyable: true, width: 140, search: false },
      {
        title: t('pages.vault.addresses.columns.network', 'Network'),
        dataIndex: 'network_id',
        width: 180,
        valueType: 'select',
        fieldProps: {
          loading: networkOptionsLoading,
          options: networkOptions,
          placeholder: t('pages.vault.addresses.filters.network', 'All'),
        },
        render: (_, r) => r.network || r.network_id || '-',
      },
      { title: t('pages.vault.addresses.columns.address', 'Address'), dataIndex: 'address', copyable: true, ellipsis: true },
      {
        title: t('pages.vault.addresses.columns.type', 'Type'),
        dataIndex: 'address_type',
        width: 120,
        valueType: 'select',
        fieldProps: { options: ADDRESS_TYPE_OPTIONS, placeholder: t('pages.vault.addresses.filters.type', 'All') },
        render: (_, r) => typeTag(r.address_type),
      },
      { title: t('pages.vault.addresses.columns.label', 'Label'), dataIndex: 'label', search: false, width: 180, ellipsis: true },
      {
        title: t('pages.vault.addresses.columns.status', 'Status'),
        dataIndex: 'status',
        width: 120,
        valueType: 'select',
        fieldProps: { options: STATUS_OPTIONS, placeholder: t('pages.vault.addresses.filters.status', 'All') },
        render: (_, r) => statusTag(r.status),
      },
      {
        title: t('pages.vault.addresses.columns.activeWallet', 'Active Wallet'),
        dataIndex: 'is_active_wallet',
        search: false,
        width: 120,
        render: (_, r) => (r.is_active_wallet ? <Tag color="blue">{t('pages.vault.addresses.values.yes', 'Yes')}</Tag> : '-'),
      },
      { title: t('pages.vault.addresses.columns.totalUsd', 'Total USD'), dataIndex: 'total_balance_usd', search: false, width: 140 },
      { title: t('pages.vault.addresses.columns.currencies', 'Currencies'), dataIndex: 'currencies_count', search: false, width: 120 },
      { title: t('pages.vault.addresses.columns.lastSyncedAt', 'Last Synced At'), dataIndex: 'last_synced_at', search: false, width: 180 },
      { title: t('pages.vault.addresses.columns.createdAt', 'Created At'), dataIndex: 'created_at', search: false, width: 180 },
      { title: t('pages.vault.addresses.columns.createdBy', 'Created By'), dataIndex: 'created_by_name', search: false, width: 140 },
      {
        title: t('pages.vault.addresses.columns.actions', 'Actions'),
        valueType: 'option',
        width: 240,
        fixed: 'right',
        render: (_, r) => {
          const status = (r.status || '').trim().toLowerCase();
          const addrType = (r.address_type || '').trim().toLowerCase();
          const canEnable = canUpdateStatus && status !== 'active';
          const canDisable = canUpdateStatus && status === 'active';
          const canSetActiveWallet = canUpdateStatus && status === 'active' && addrType === 'active' && !r.is_active_wallet;

          return [
            <Button
              key="balances"
              type="link"
              disabled={!canBalances}
              onClick={() => void openBalances(r)}
            >
              {t('pages.vault.addresses.actions.balances', 'Balances')}
            </Button>,
            <Button
              key="enable"
              type="link"
              disabled={!canEnable}
              onClick={() => void updateStatus(r, { status: 'active' })}
            >
              {t('pages.vault.addresses.actions.enable', 'Enable')}
            </Button>,
            <Button
              key="disable"
              type="link"
              disabled={!canDisable}
              onClick={() => void updateStatus(r, { status: 'inactive' })}
            >
              {t('pages.vault.addresses.actions.disable', 'Disable')}
            </Button>,
            <Button
              key="setActiveWallet"
              type="link"
              disabled={!canSetActiveWallet}
              onClick={() => void updateStatus(r, { status: 'active', set_as_active_wallet: true })}
            >
              {t('pages.vault.addresses.actions.setActiveWallet', 'Set Active')}
            </Button>,
            <Button key="delete" type="link" danger disabled={!canRemove} onClick={() => void removeAddress(r)}>
              {t('pages.vault.addresses.actions.delete', 'Delete')}
            </Button>,
          ];
        },
      },
    ];
  }, [canBalances, canRemove, canUpdateStatus, networkOptions, networkOptionsLoading, openBalances, removeAddress, t, updateStatus]);

  return (
    <PageContainer
      title={t('pages.vault.addresses.title', 'Vault Address Pool')}
      extra={[
        <Button key="back" onClick={() => history.push('/vault/funds')}>
          {t('pages.vault.addresses.actions.back', 'Back')}
        </Button>,
        <Button key="add" type="primary" disabled={!canCreate} onClick={() => void openAdd()}>
          {t('pages.vault.addresses.actions.add', 'Add Address')}
        </Button>,
      ]}
    >
      <Typography.Paragraph type="secondary" style={{ marginTop: -8 }}>
        {t('pages.vault.addresses.description', 'Manually maintain the vault monitoring address pool. New addresses are inactive by default.')}
      </Typography.Paragraph>

      <ProTable<AdminVaultAddressItem>
        actionRef={actionRef}
        rowKey={(r) => r.id || `${r.network_id || ''}-${r.address || ''}`}
        columns={columns}
        request={async (params) => {
          if (!canList) return { success: true, data: [], total: 0 };
          // Lazily load network options to power the filters.
          if (!networkOptions.length) {
            void loadNetworkOptions();
          }
          try {
            const res = await adminListVaultAddresses(
              {
                page: params.current,
                page_size: params.pageSize,
                network_id: (params as any).network_id,
                address_type: (params as any).address_type,
                status: (params as any).status,
              },
              { skipErrorHandler: true },
            );
            return {
              success: !!res?.success,
              data: res.data?.addresses ?? [],
              total: Number(res.data?.pagination?.total ?? 0),
            };
          } catch (e: any) {
            message.error(e?.message || t('pages.vault.addresses.messages.loadFailed', 'Load failed'));
            return { success: false, data: [], total: 0 };
          }
        }}
        search={{ labelWidth: 100 }}
        scroll={{ x: 1600 }}
        options={{ density: true, fullScreen: true, reload: true }}
        pagination={{ showSizeChanger: true, showQuickJumper: true }}
      />

      <Modal
        title={t('pages.vault.addresses.add.title', 'Add Vault Address')}
        open={addOpen}
        onCancel={closeAdd}
        onOk={() => void submitAdd()}
        okButtonProps={{ loading: addLoading, disabled: !canCreate }}
        destroyOnClose
      >
        <Form<AdminAddVaultAddressRequest>
          form={addForm}
          layout="vertical"
          initialValues={{ address_type: 'active' }}
        >
          <Form.Item
            name="network_id"
            label={t('pages.vault.addresses.add.network', 'Network')}
            rules={[{ required: true, message: t('pages.vault.addresses.add.networkRequired', 'Select a network') }]}
          >
            <Select
              showSearch
              loading={networkOptionsLoading}
              options={networkOptions}
              placeholder={t('pages.vault.addresses.add.networkPlaceholder', 'Select network')}
              filterOption={(input, opt) => (opt?.label as any)?.toString?.().toLowerCase?.().includes(input.toLowerCase())}
            />
          </Form.Item>

          <Form.Item
            name="address_type"
            label={t('pages.vault.addresses.add.type', 'Address Type')}
            rules={[{ required: true, message: t('pages.vault.addresses.add.typeRequired', 'Select an address type') }]}
          >
            <Select options={ADDRESS_TYPE_OPTIONS} />
          </Form.Item>

          <Form.Item
            name="address"
            label={t('pages.vault.addresses.add.address', 'Address')}
            rules={[{ required: true, message: t('pages.vault.addresses.add.addressRequired', 'Enter an address') }]}
          >
            <Input placeholder={t('pages.vault.addresses.add.addressPlaceholder', 'Full address')} />
          </Form.Item>

          <Form.Item name="label" label={t('pages.vault.addresses.add.label', 'Label (optional)')}>
            <Input placeholder={t('pages.vault.addresses.add.labelPlaceholder', 'e.g. Binance hot wallet')} />
          </Form.Item>

          <Typography.Text type="secondary">
            {t('pages.vault.addresses.add.tip', 'New addresses are created as inactive. Enable them to start monitoring.')}
          </Typography.Text>
        </Form>
      </Modal>

      <Drawer
        title={t('pages.vault.addresses.balances.title', 'Address Balances')}
        open={balancesOpen}
        width={860}
        onClose={() => {
          setBalancesOpen(false);
          setBalances([]);
          setBalancesMeta(null);
          setBalancesLoading(false);
        }}
      >
        {balancesLoading ? (
          <Typography.Text>{t('pages.vault.addresses.balances.loading', 'Loading...')}</Typography.Text>
        ) : (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <div style={{ background: '#f9fafb', borderRadius: 12, padding: 16 }}>
              <div style={{ display: 'grid', gridTemplateColumns: '120px 1fr', rowGap: 8, columnGap: 12 }}>
                <div style={{ color: '#6b7280' }}>{t('pages.vault.addresses.balances.network', 'Network')}</div>
                <div style={{ fontWeight: 500 }}>{balancesMeta?.network || '-'}</div>
                <div style={{ color: '#6b7280' }}>{t('pages.vault.addresses.balances.address', 'Address')}</div>
                <div style={{ fontWeight: 500, wordBreak: 'break-all' }}>{balancesMeta?.address || '-'}</div>
                <div style={{ color: '#6b7280' }}>{t('pages.vault.addresses.balances.totalUsd', 'Total USD')}</div>
                <div style={{ fontWeight: 500 }}>{balancesMeta?.totalUsd || '-'}</div>
              </div>
            </div>

            <ProTable<AdminVaultAddressBalanceItem>
              rowKey={(r) => `${r.currency || ''}-${r.contract_address || ''}`}
              columns={balanceColumns}
              dataSource={balances}
              search={false}
              options={{ reload: false, density: false, fullScreen: false, setting: false }}
              pagination={{ pageSize: 10 }}
            />
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
};

export default VaultAddressesPage;
