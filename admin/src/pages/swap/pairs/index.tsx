import type { ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App } from 'antd';
import React, { useRef } from 'react';
import type { ActionType } from '@ant-design/pro-components';
import { adminListSwapTransactions } from '@/api/generated/swap';
import type { AdminAdminSwapTransactionItem } from '@/api/generated/schemas';
import { useRbac } from '@/hooks/useRbac';

const PERM = {
  list: 'ListSwapTransactions',
};

const SwapPairsPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string, values?: Record<string, any>) =>
    intl.formatMessage({ id, defaultMessage }, values);
  const actionRef = useRef<ActionType | null>(null);

  const columns: ProColumns<AdminAdminSwapTransactionItem>[] = [
    { title: t('pages.swap.pairs.columns.id', 'ID'), dataIndex: 'id', copyable: true, width: 160 },
    { title: t('pages.swap.pairs.columns.project', 'Project'), dataIndex: 'project_name', width: 140 },
    { title: t('pages.swap.pairs.columns.wallet', 'Wallet'), dataIndex: 'wallet_address', copyable: true, width: 220, ellipsis: true },
    { title: t('pages.swap.pairs.columns.chain', 'Chain'), dataIndex: 'chain_name', width: 120 },
    { title: t('pages.swap.pairs.columns.provider', 'Provider'), dataIndex: 'provider', width: 120 },
    {
      title: t('pages.swap.pairs.columns.from', 'From'),
      dataIndex: 'from_token_symbol',
      width: 140,
      render: (_, r) => `${r.from_amount || '-'} ${r.from_token_symbol || ''}`.trim(),
    },
    {
      title: t('pages.swap.pairs.columns.to', 'To'),
      dataIndex: 'to_token_symbol',
      width: 140,
      render: (_, r) => `${r.to_amount || '-'} ${r.to_token_symbol || ''}`.trim(),
    },
    { title: t('pages.swap.pairs.columns.status', 'Status'), dataIndex: 'status', width: 120 },
    { title: t('pages.swap.pairs.columns.txHash', 'Tx Hash'), dataIndex: 'tx_hash', copyable: true, width: 240, ellipsis: true },
    { title: t('pages.swap.pairs.columns.feeUsd', 'Fee USD'), dataIndex: 'fee_usd', width: 120 },
    { title: t('pages.swap.pairs.columns.createdAt', 'Created At'), dataIndex: 'created_at', width: 180 },

    {
      title: t('pages.swap.pairs.search.wallet', 'Wallet Address'),
      dataIndex: 'wallet_address',
      hideInTable: true,
      search: { transform: (v) => ({ wallet_address: v }) },
    },
    {
      title: t('pages.swap.pairs.search.chainId', 'Chain ID'),
      dataIndex: 'chain_id',
      hideInTable: true,
      search: { transform: (v) => ({ chain_id: v }) },
    },
    {
      title: t('pages.swap.pairs.search.provider', 'Provider'),
      dataIndex: 'provider',
      hideInTable: true,
      search: { transform: (v) => ({ provider: v }) },
    },
    {
      title: t('pages.swap.pairs.search.status', 'Status'),
      dataIndex: 'status',
      hideInTable: true,
      search: { transform: (v) => ({ status: v }) },
    },
  ];

  return (
    <PageContainer>
      <ProTable<AdminAdminSwapTransactionItem>
        actionRef={actionRef}
        rowKey={(row) => row.id || row.tx_hash || JSON.stringify(row)}
        columns={columns}
        request={async (params) => {
          if (!canRpc(PERM.list)) return { success: true, data: [], total: 0 };
          try {
            const res = await adminListSwapTransactions(
              {
                page: params.current,
                page_size: params.pageSize,
                wallet_address: (params as any).wallet_address,
                chain_id: (params as any).chain_id,
                provider: (params as any).provider,
                status: (params as any).status,
              },
              { skipErrorHandler: true },
            );
            return {
              success: !!res?.success,
              data: res.data?.items ?? [],
              total: Number(res.data?.pagination?.total ?? 0),
            };
          } catch (e: any) {
            message.error(e?.message || t('pages.swap.pairs.messages.loadFailed', 'Failed to load'));
            return { success: false, data: [], total: 0 };
          }
        }}
        search={{ labelWidth: 110 }}
      />
    </PageContainer>
  );
};

export default SwapPairsPage;
