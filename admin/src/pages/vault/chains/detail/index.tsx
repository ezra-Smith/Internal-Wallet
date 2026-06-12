import type { ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { App, Button, Descriptions, Space, Typography } from 'antd';
import { history, useIntl, useParams } from '@umijs/max';
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { adminGetVaultDetail, adminSyncVault } from '@/api/generated/vault';
import type {
  AdminVaultBalanceDetailItem,
  AdminVaultRecentTransactionItem,
} from '@/api/generated/schemas';
import { useRbac } from '@/hooks/useRbac';

const PERM = {
  detail: 'GetVaultDetail',
  sync: 'SyncVault',
};

const VaultChainDetailPage: React.FC = () => {
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string, values?: Record<string, any>) =>
    intl.formatMessage({ id, defaultMessage }, values);
  const { chainId = '' } = useParams<{ chainId: string }>();

  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<any>(null);

  const reload = useCallback(async () => {
    if (!chainId) return;
    if (!canRpc(PERM.detail)) return;
    setLoading(true);
    try {
      const res = await adminGetVaultDetail({ chainId }, { skipErrorHandler: true });
      if (!res?.success)
        throw new Error(res?.message || t('pages.vault.chainDetail.messages.loadFailed', 'Failed to load vault detail'));
      setData(res.data ?? null);
    } catch (e: any) {
      message.error(e?.message || t('pages.vault.chainDetail.messages.loadFailed', 'Failed to load vault detail'));
    } finally {
      setLoading(false);
    }
  }, [canRpc, chainId, message]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const balances: AdminVaultBalanceDetailItem[] = data?.balances ?? [];
  const txs: AdminVaultRecentTransactionItem[] = data?.recent_transactions ?? [];

  const balanceColumns: ProColumns<AdminVaultBalanceDetailItem>[] = [
    { title: t('pages.vault.chainDetail.balance.columns.currency', 'Currency'), dataIndex: 'currency', width: 120 },
    {
      title: t('pages.vault.chainDetail.balance.columns.contract', 'Contract'),
      dataIndex: 'contract_address',
      copyable: true,
      ellipsis: true,
    },
    { title: t('pages.vault.chainDetail.balance.columns.balance', 'Balance'), dataIndex: 'balance', width: 140 },
    { title: t('pages.vault.chainDetail.balance.columns.usd', 'USD'), dataIndex: 'balance_usd', width: 120 },
    { title: t('pages.vault.chainDetail.balance.columns.locked', 'Locked'), dataIndex: 'locked_balance', width: 120 },
    { title: t('pages.vault.chainDetail.balance.columns.available', 'Available'), dataIndex: 'available_balance', width: 120 },
    { title: t('pages.vault.chainDetail.balance.columns.pendingWds', 'Pending WDs'), dataIndex: 'pending_withdrawals', width: 120 },
    { title: t('pages.vault.chainDetail.balance.columns.pendingAmount', 'Pending Amount'), dataIndex: 'pending_withdrawals_amount', width: 140 },
    { title: t('pages.vault.chainDetail.balance.columns.thresholdLow', 'Threshold Low'), dataIndex: ['threshold_config', 'low'], width: 140 },
    { title: t('pages.vault.chainDetail.balance.columns.thresholdCritical', 'Threshold Critical'), dataIndex: ['threshold_config', 'critical'], width: 160 },
    { title: t('pages.vault.chainDetail.balance.columns.status', 'Status'), dataIndex: 'status', width: 120 },
  ];

  const txColumns: ProColumns<AdminVaultRecentTransactionItem>[] = [
    { title: t('pages.vault.chainDetail.txs.columns.id', 'ID'), dataIndex: 'id', copyable: true, width: 160 },
    { title: t('pages.vault.chainDetail.txs.columns.type', 'Type'), dataIndex: 'type', width: 120 },
    { title: t('pages.vault.chainDetail.txs.columns.currency', 'Currency'), dataIndex: 'currency', width: 120 },
    { title: t('pages.vault.chainDetail.txs.columns.amount', 'Amount'), dataIndex: 'amount', width: 120 },
    { title: t('pages.vault.chainDetail.txs.columns.to', 'To'), dataIndex: 'to_address', copyable: true, ellipsis: true },
    { title: t('pages.vault.chainDetail.txs.columns.txHash', 'Tx Hash'), dataIndex: 'tx_hash', copyable: true, ellipsis: true },
    { title: t('pages.vault.chainDetail.txs.columns.status', 'Status'), dataIndex: 'status', width: 120 },
    { title: t('pages.vault.chainDetail.txs.columns.createdAt', 'Created At'), dataIndex: 'created_at', valueType: 'dateTime', width: 170 },
  ];

  const info = useMemo(() => {
    return {
      network: data?.network,
      vault: data?.vault_address,
      status: data?.status,
      lastSync: data?.sync_info?.last_sync_at,
      blocksBehind: data?.sync_info?.blocks_behind,
      syncStatus: data?.sync_info?.sync_status,
      gas: data?.gas_info?.balance,
      gasUsd: data?.gas_info?.balance_usd,
    };
  }, [data]);

  return (
    <PageContainer
      title={t('pages.vault.chainDetail.title', 'Vault Chain: {id}', { id: chainId || '-' })}
      extra={[
        <Button key="back" onClick={() => history.push('/vault/funds')}>
          {t('pages.vault.chainDetail.actions.back', 'Back')}
        </Button>,
        <Button key="reload" onClick={() => void reload()}>
          {t('pages.vault.chainDetail.actions.reload', 'Reload')}
        </Button>,
        <Button
          key="sync"
          disabled={!canRpc(PERM.sync) || !chainId}
          onClick={() => {
            if (!chainId) return;
            modal.confirm({
              title: t('pages.vault.chainDetail.sync.title', 'Sync vault (chain {chainId})?', { chainId }),
              onOk: async () => {
                try {
                  const res = await adminSyncVault({ chainId }, {}, { skipErrorHandler: true });
                  if (!res?.success)
                    throw new Error(res?.message || t('pages.vault.chainDetail.messages.syncFailed', 'Sync failed'));
                  message.success(t('pages.vault.chainDetail.messages.syncStarted', 'Sync started'));
                  await reload();
                } catch (e: any) {
                  message.error(e?.message || t('pages.vault.chainDetail.messages.syncFailed', 'Sync failed'));
                }
              },
            });
          }}
        >
          {t('pages.vault.chainDetail.actions.sync', 'Sync')}
        </Button>,
      ]}
    >
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <Descriptions size="small" column={2} bordered>
          <Descriptions.Item label={t('pages.vault.chainDetail.info.network', 'Network')}>{info.network || '-'}</Descriptions.Item>
          <Descriptions.Item label={t('pages.vault.chainDetail.info.status', 'Status')}>{info.status || '-'}</Descriptions.Item>
          <Descriptions.Item label={t('pages.vault.chainDetail.info.vaultAddress', 'Vault Address')}>{info.vault || '-'}</Descriptions.Item>
          <Descriptions.Item label={t('pages.vault.chainDetail.info.syncStatus', 'Sync Status')}>{info.syncStatus || '-'}</Descriptions.Item>
          <Descriptions.Item label={t('pages.vault.chainDetail.info.lastSync', 'Last Sync')}>{info.lastSync || '-'}</Descriptions.Item>
          <Descriptions.Item label={t('pages.vault.chainDetail.info.blocksBehind', 'Blocks Behind')}>{info.blocksBehind || '-'}</Descriptions.Item>
          <Descriptions.Item label={t('pages.vault.chainDetail.info.gasBalance', 'Gas Balance')}>{info.gas || '-'}</Descriptions.Item>
          <Descriptions.Item label={t('pages.vault.chainDetail.info.gasUsd', 'Gas USD')}>{info.gasUsd || '-'}</Descriptions.Item>
        </Descriptions>

        <ProTable<AdminVaultBalanceDetailItem>
          rowKey={(r) => r.currency || r.contract_address || JSON.stringify(r)}
          loading={loading}
          columns={balanceColumns}
          dataSource={balances}
          search={false}
          options={{ reload: false, density: false, fullScreen: false, setting: false }}
          pagination={false}
          headerTitle={t('pages.vault.chainDetail.sections.balances', 'Balances')}
        />

        <ProTable<AdminVaultRecentTransactionItem>
          rowKey={(r) => r.id || r.tx_hash || JSON.stringify(r)}
          loading={loading}
          columns={txColumns}
          dataSource={txs}
          search={false}
          options={{ reload: false, density: false, fullScreen: false, setting: false }}
          pagination={{ pageSize: 10 }}
          headerTitle={t('pages.vault.chainDetail.sections.recentTransactions', 'Recent Transactions')}
        />

        {loading ? null : !data ? (
          <Typography.Text type="secondary">{t('pages.vault.chainDetail.empty', 'No data')}</Typography.Text>
        ) : null}
      </Space>
    </PageContainer>
  );
};

export default VaultChainDetailPage;
