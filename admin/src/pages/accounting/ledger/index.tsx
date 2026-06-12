import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App, Button, Descriptions, Drawer, Space, Table, Tag, Typography } from 'antd';
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useRbac } from '@/hooks/useRbac';
import { adminListCurrencies } from '@/api/generated/assets';
import {
  adminAccountingGetLedgerTxDetail,
  adminAccountingListLedgerTx,
} from '@/api/generated/accounting';
import type {
  AdminAccountingLedgerPostingItem as AccountingLedgerPostingItem,
  AdminAccountingLedgerTxItem as AccountingLedgerTxItem,
  AdminCurrencyItem as CurrencyItem,
} from '@/api/generated/schemas';

const PERM = {
  list: 'AccountingListLedgerTx',
  detail: 'AccountingGetLedgerTxDetail',
} as const;

const LedgerPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const actionRef = useRef<ActionType | null>(null);
  const [assetOptions, setAssetOptions] = useState<{ label: string; value: string }[]>([]);

  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailTx, setDetailTx] = useState<AccountingLedgerTxItem | null>(null);
  const [detailPostings, setDetailPostings] = useState<AccountingLedgerPostingItem[]>([]);

  const loadAssets = async () => {
    try {
      const res = await adminListCurrencies({ page: 1, page_size: 500 }, { skipErrorHandler: true });
      const list: CurrencyItem[] = res?.data?.currencies || [];
      const opts = list
        .map((x) => {
          const code = (x.asset_code || '').trim().toUpperCase();
          if (!code) return null;
          return {
            value: code,
            label: `${code}${x.asset_name ? ` (${x.asset_name})` : ''}`,
          };
        })
        .filter((x): x is { label: string; value: string } => !!x)
        .sort((a, b) => a.value.localeCompare(b.value));
      setAssetOptions(opts);
    } catch {
      // ignore
    }
  };

  useEffect(() => {
    loadAssets();
  }, []);

  const openDetail = async (row: AccountingLedgerTxItem) => {
    if (!canRpc(PERM.detail)) return;
    const txId = row.tx_id;
    if (!txId) return;
    setDetailLoading(true);
    try {
      const res = await adminAccountingGetLedgerTxDetail({ txId }, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || '');
      setDetailTx(res?.data?.tx || row);
      setDetailPostings(res?.data?.postings || []);
      setDetailOpen(true);
    } catch (e: any) {
      message.error(e?.message || t('pages.accounting.ledger.messages.detailFailed', 'Failed to load detail'));
    } finally {
      setDetailLoading(false);
    }
  };

  const opTypeValueEnum = {
    FreezeWithdraw: { text: 'FreezeWithdraw' },
    UnfreezeWithdraw: { text: 'UnfreezeWithdraw' },
    SettleWithdraw: { text: 'SettleWithdraw' },
    ConfirmDeposit: { text: 'ConfirmDeposit' },
    AdminAdjust: { text: 'AdminAdjust' },
    Transfer: { text: 'Transfer' },
    FundSystemWallet: { text: 'FundSystemWallet' },
  } as const;

  const columns: ProColumns<AccountingLedgerTxItem>[] = useMemo(() => {
    return [
      {
        title: t('pages.accounting.ledger.columns.txId', 'Tx ID'),
        dataIndex: 'tx_id',
        width: 160,
        copyable: true,
        render: (_, row) => <Typography.Text code>{row.tx_id}</Typography.Text>,
      },
      {
        title: t('pages.accounting.ledger.columns.createdTime', 'Created Time'),
        dataIndex: 'created_at',
        valueType: 'dateTime',
        width: 180,
      },
      {
        title: t('pages.accounting.ledger.columns.opType', 'Op Type'),
        dataIndex: 'op_type',
        width: 160,
        render: (_, row) => {
          const v = (row.op_type || '').trim();
          if (!v) return <Typography.Text type="secondary">-</Typography.Text>;
          return <Tag>{v}</Tag>;
        },
      },
      {
        title: t('pages.accounting.ledger.columns.bizRef', 'Biz Ref'),
        dataIndex: 'biz_ref',
        width: 220,
        ellipsis: true,
      },
      {
        title: t('pages.accounting.ledger.columns.idempotencyKey', 'Idempotency Key'),
        dataIndex: 'idempotency_key',
        ellipsis: true,
      },
      {
        title: t('pages.accounting.ledger.columns.requestHash', 'Request Hash'),
        dataIndex: 'request_hash',
        ellipsis: true,
      },
      {
        title: t('pages.accounting.ledger.columns.actions', 'Actions'),
        valueType: 'option',
        width: 120,
        render: (_, row) => (
          <Button type="link" size="small" disabled={!canRpc(PERM.detail)} onClick={() => openDetail(row)}>
            {t('pages.accounting.ledger.actions.detail', 'Detail')}
          </Button>
        ),
      },

      // -------- Search fields --------
      {
        title: t('pages.accounting.ledger.search.txId', 'Tx ID'),
        key: 'search_tx_id',
        dataIndex: 'tx_id',
        hideInTable: true,
        search: { transform: (v) => ({ tx_id: v }) },
      },
      {
        title: t('pages.accounting.ledger.search.userId', 'User ID'),
        key: 'search_user_id',
        dataIndex: 'user_id',
        hideInTable: true,
        search: { transform: (v) => ({ user_id: v }) },
      },
      {
        title: t('pages.accounting.ledger.search.asset', 'Asset'),
        key: 'search_asset_code',
        dataIndex: 'asset_code',
        hideInTable: true,
        valueType: 'select',
        fieldProps: { options: assetOptions, showSearch: true, allowClear: true },
        search: { transform: (v) => ({ asset_code: v }) },
      },
      {
        title: t('pages.accounting.ledger.search.opType', 'Op Type'),
        key: 'search_op_type',
        dataIndex: 'op_type',
        hideInTable: true,
        valueEnum: opTypeValueEnum as any,
        search: { transform: (v) => ({ op_type: v }) },
      },
      {
        title: t('pages.accounting.ledger.search.bizRef', 'Biz Ref'),
        key: 'search_biz_ref',
        dataIndex: 'biz_ref',
        hideInTable: true,
        search: { transform: (v) => ({ biz_ref: v }) },
      },
      {
        title: t('pages.accounting.ledger.search.idempotencyKey', 'Idempotency Key'),
        key: 'search_idempotency_key',
        dataIndex: 'idempotency_key',
        hideInTable: true,
        search: { transform: (v) => ({ idempotency_key: v }) },
      },
      {
        title: t('pages.accounting.ledger.search.createdFrom', 'Created From'),
        key: 'search_created_from',
        dataIndex: 'created_from',
        hideInTable: true,
        search: { transform: (v) => ({ created_from: v }) },
      },
      {
        title: t('pages.accounting.ledger.search.createdTo', 'Created To'),
        key: 'search_created_to',
        dataIndex: 'created_to',
        hideInTable: true,
        search: { transform: (v) => ({ created_to: v }) },
      },
    ];
  }, [assetOptions, canRpc, intl]);

  return (
    <PageContainer>
      <ProTable<AccountingLedgerTxItem>
        actionRef={actionRef}
        rowKey={(row) => row.tx_id || JSON.stringify(row)}
        columns={columns}
        request={async (params) => {
          if (!canRpc(PERM.list)) return { success: true, data: [], total: 0 };
          try {
            const res = await adminAccountingListLedgerTx(
              {
                page: params.current,
                page_size: params.pageSize,
                tx_id: (params as any).tx_id,
                user_id: (params as any).user_id,
                asset_code: (params as any).asset_code,
                op_type: (params as any).op_type,
                biz_ref: (params as any).biz_ref,
                idempotency_key: (params as any).idempotency_key,
                created_from: (params as any).created_from,
                created_to: (params as any).created_to,
              },
              { skipErrorHandler: true },
            );
            if (!res?.success) throw new Error(res?.message || '');
            return {
              success: true,
              data: res?.data?.items ?? [],
              total: Number(res?.data?.pagination?.total ?? 0),
            };
          } catch (e: any) {
            message.error(e?.message || t('pages.accounting.ledger.messages.loadFailed', 'Failed to load'));
            return { success: false, data: [], total: 0 };
          }
        }}
        search={{ labelWidth: 120 }}
      />

      <Drawer
        title={t('pages.accounting.ledger.detail.title', 'Ledger Detail')}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        width={880}
        destroyOnClose
      >
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Descriptions
            size="small"
            bordered
            column={2}
            items={[
              { key: 'tx_id', label: t('pages.accounting.ledger.detail.txId', 'Tx ID'), children: <Typography.Text code>{detailTx?.tx_id || '-'}</Typography.Text> },
              { key: 'created_at', label: t('pages.accounting.ledger.detail.createdTime', 'Created Time'), children: detailTx?.created_at || '-' },
              { key: 'op_type', label: t('pages.accounting.ledger.detail.opType', 'Op Type'), children: detailTx?.op_type || '-' },
              { key: 'biz_ref', label: t('pages.accounting.ledger.detail.bizRef', 'Biz Ref'), children: detailTx?.biz_ref || '-' },
              { key: 'idempotency_key', label: t('pages.accounting.ledger.detail.idempotencyKey', 'Idempotency Key'), children: <Typography.Text copyable>{detailTx?.idempotency_key || '-'}</Typography.Text> },
              { key: 'request_hash', label: t('pages.accounting.ledger.detail.requestHash', 'Request Hash'), children: <Typography.Text copyable>{detailTx?.request_hash || '-'}</Typography.Text> },
            ]}
          />

          <Table<AccountingLedgerPostingItem>
            rowKey={(row) => `${row.seq}-${row.account_id}-${row.asset_code}-${row.bucket}`}
            loading={detailLoading}
            pagination={false}
            size="small"
            dataSource={detailPostings}
            scroll={{ x: 1200 }}
            columns={[
              { title: t('pages.accounting.ledger.postings.seq', 'Seq'), dataIndex: 'seq', width: 60 },
              { title: t('pages.accounting.ledger.postings.asset', 'Asset'), dataIndex: 'asset_code', width: 100, render: (v) => <Typography.Text code>{v}</Typography.Text> },
              { title: t('pages.accounting.ledger.postings.bucket', 'Bucket'), dataIndex: 'bucket', width: 110, render: (v) => <Tag>{String(v || '').toUpperCase()}</Tag> },
              {
                title: t('pages.accounting.ledger.postings.account', 'Account'),
                dataIndex: 'account_id',
                width: 280,
                render: (_, row) => {
                  const owner = row.owner_type === 'user' ? `user:${row.owner_id}` : `system`;
                  const type = row.account_type_code || '';
                  const chain = row.chain_scope ? `/${row.chain_scope}` : '';
                  return (
                    <Space direction="vertical" size={0}>
                      <Typography.Text code>{row.account_id}</Typography.Text>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {owner} · {type}{chain}
                      </Typography.Text>
                    </Space>
                  );
                },
              },
              {
                title: t('pages.accounting.ledger.postings.debit', 'Debit'),
                dataIndex: 'debit_decimal',
                width: 140,
                render: (_, row) => <Typography.Text>{row.debit_decimal || '-'}</Typography.Text>,
              },
              {
                title: t('pages.accounting.ledger.postings.credit', 'Credit'),
                dataIndex: 'credit_decimal',
                width: 140,
                render: (_, row) => <Typography.Text>{row.credit_decimal || '-'}</Typography.Text>,
              },
              {
                title: t('pages.accounting.ledger.postings.raw', 'Raw'),
                dataIndex: 'debit_raw',
                width: 360,
                render: (_, row) => (
                  <Space direction="vertical" size={0}>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      debit_raw:{' '}
                      <Typography.Text
                        copyable
                        ellipsis={{ tooltip: true }}
                        style={{ fontSize: 12, maxWidth: 320, display: 'inline-block', verticalAlign: 'bottom' }}
                      >
                        {row.debit_raw}
                      </Typography.Text>
                    </Typography.Text>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      credit_raw:{' '}
                      <Typography.Text
                        copyable
                        ellipsis={{ tooltip: true }}
                        style={{ fontSize: 12, maxWidth: 320, display: 'inline-block', verticalAlign: 'bottom' }}
                      >
                        {row.credit_raw}
                      </Typography.Text>
                    </Typography.Text>
                  </Space>
                ),
              },
            ]}
          />
        </Space>
      </Drawer>
    </PageContainer>
  );
};

export default LedgerPage;
