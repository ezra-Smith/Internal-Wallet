import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App, Button, Divider, Form, Input, Modal, Popconfirm, Radio, Select, Space, Tag, Typography } from 'antd';
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useRbac } from '@/hooks/useRbac';
import { adminListCurrencies } from '@/api/generated/assets';
import {
  adminAccountingCreateAccountType,
  adminAccountingDeleteAccountType,
  adminAccountingListAccountTypes,
  adminAccountingSetAccountTypeAssets,
  adminAccountingUpdateAccountType,
} from '@/api/generated/accounting';
import type { AdminAccountingAccountTypeItem as AccountingAccountTypeItem, AdminCurrencyItem as CurrencyItem } from '@/api/generated/schemas';

const PERM = {
  list: 'AccountingListAccountTypes',
  create: 'AccountingCreateAccountType',
  update: 'AccountingUpdateAccountType',
  delete: 'AccountingDeleteAccountType',
  setAssets: 'AccountingSetAccountTypeAssets',
} as const;

type AccountTypeFormValues = {
  code: string;
  name: string;
  description?: string;
  normal_side: 'debit' | 'credit';
};

const AccountTypesPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const actionRef = useRef<ActionType | null>(null);

  const [assetOptions, setAssetOptions] = useState<{ label: string; value: string }[]>([]);

  const [editOpen, setEditOpen] = useState(false);
  const [editLoading, setEditLoading] = useState(false);
  const [editMode, setEditMode] = useState<'create' | 'update'>('create');
  const [current, setCurrent] = useState<AccountingAccountTypeItem | null>(null);
  const [editForm] = Form.useForm<AccountTypeFormValues>();

  const [assetsOpen, setAssetsOpen] = useState(false);
  const [assetsLoading, setAssetsLoading] = useState(false);
  const [assetsForm] = Form.useForm<{ asset_codes: string[] }>();

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

  const openCreate = () => {
    if (!canRpc(PERM.create)) return;
    setEditMode('create');
    setCurrent(null);
    editForm.resetFields();
    editForm.setFieldsValue({ normal_side: 'credit', description: '' } as any);
    setEditOpen(true);
  };

  const openUpdate = (row: AccountingAccountTypeItem) => {
    if (!canRpc(PERM.update)) return;
    setEditMode('update');
    setCurrent(row);
    editForm.resetFields();
    editForm.setFieldsValue({
      code: row.code,
      name: row.name,
      description: row.description || '',
      normal_side: (row.normal_side === 'debit' ? 'debit' : 'credit') as any,
    });
    setEditOpen(true);
  };

  const submitEdit = async () => {
    const values = await editForm.validateFields();
    const code = (values.code || '').trim().toUpperCase();
    const name = (values.name || '').trim();
    const description = (values.description || '').trim();
    const normal_side =
      editMode === 'update'
        ? (((current?.normal_side || '').toLowerCase() === 'debit' ? 'debit' : 'credit') as AccountTypeFormValues['normal_side'])
        : values.normal_side;

    setEditLoading(true);
    try {
      if (editMode === 'create') {
        const res = await adminAccountingCreateAccountType(
          { code, name, description, normal_side },
          { skipErrorHandler: true },
        );
        if (!res?.success) throw new Error(res?.message || '');
        message.success(t('pages.accounting.accountTypes.messages.created', 'Created'));
      } else {
        const res = await adminAccountingUpdateAccountType(
          { code },
          { name, description, normal_side },
          { skipErrorHandler: true },
        );
        if (!res?.success) throw new Error(res?.message || '');
        message.success(t('pages.accounting.accountTypes.messages.updated', 'Updated'));
      }
      setEditOpen(false);
      actionRef.current?.reload();
    } catch (e: any) {
      message.error(e?.message || t('pages.accounting.accountTypes.messages.saveFailed', 'Save failed'));
    } finally {
      setEditLoading(false);
    }
  };

  const openAssets = (row: AccountingAccountTypeItem) => {
    if (!canRpc(PERM.setAssets)) return;
    setCurrent(row);
    assetsForm.resetFields();
    assetsForm.setFieldsValue({ asset_codes: row.asset_codes || [] });
    setAssetsOpen(true);
  };

  const submitAssets = async () => {
    if (!current?.code) return;
    const values = await assetsForm.validateFields();
    const asset_codes = (values.asset_codes || []).map((x) => (x || '').trim().toUpperCase()).filter(Boolean);
    setAssetsLoading(true);
    try {
      const res = await adminAccountingSetAccountTypeAssets({ code: current.code }, { asset_codes }, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || '');
      message.success(t('pages.accounting.accountTypes.messages.assetsSaved', 'Assets saved'));
      setAssetsOpen(false);
      actionRef.current?.reload();
    } catch (e: any) {
      message.error(e?.message || t('pages.accounting.accountTypes.messages.saveFailed', 'Save failed'));
    } finally {
      setAssetsLoading(false);
    }
  };

  const deleteAccountType = async (code: string) => {
    if (!canRpc(PERM.delete)) return;
    try {
      const res = await adminAccountingDeleteAccountType({ code }, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || '');
      message.success(t('pages.accounting.accountTypes.messages.deleted', 'Deleted'));
      actionRef.current?.reload();
    } catch (e: any) {
      message.error(e?.message || t('pages.accounting.accountTypes.messages.deleteFailed', 'Delete failed'));
    }
  };

  const columns: ProColumns<AccountingAccountTypeItem>[] = useMemo(() => {
    return [
      {
        title: t('pages.accounting.accountTypes.columns.code', 'Code'),
        dataIndex: 'code',
        width: 180,
        render: (_, row) => <Typography.Text code>{row.code}</Typography.Text>,
      },
      {
        title: t('pages.accounting.accountTypes.columns.name', 'Name'),
        dataIndex: 'name',
        width: 240,
      },
      {
        title: t('pages.accounting.accountTypes.columns.description', 'Description'),
        dataIndex: 'description',
        width: 320,
        render: (_, row) => {
          const desc = (row.description || '').trim();
          if (!desc) return <Typography.Text type="secondary">-</Typography.Text>;
          return (
            <Typography.Text ellipsis={{ tooltip: desc }} style={{ maxWidth: 320, display: 'inline-block' }}>
              {desc}
            </Typography.Text>
          );
        },
      },
      {
        title: t('pages.accounting.accountTypes.columns.normalSide', 'Normal Side'),
        dataIndex: 'normal_side',
        width: 140,
        render: (_, row) => {
          const v = (row.normal_side || '').toLowerCase();
          const color = v === 'debit' ? 'processing' : v === 'credit' ? 'success' : 'default';
          return <Tag color={color}>{(v || '').toUpperCase() || '-'}</Tag>;
        },
      },
      {
        title: t('pages.accounting.accountTypes.columns.assets', 'Assets'),
        dataIndex: 'asset_codes',
        render: (_, row) => {
          const list = row.asset_codes || [];
          if (!list.length) return <Typography.Text type="secondary">-</Typography.Text>;
          return (
            <Space size={[4, 4]} wrap>
              {list.map((a) => (
                <Tag key={a}>{a}</Tag>
              ))}
            </Space>
          );
        },
      },
      {
        title: t('pages.accounting.accountTypes.columns.actions', 'Actions'),
        valueType: 'option',
        width: 220,
        render: (_, row) => (
          <Space split={<Divider type="vertical" />}>
            <Button type="link" size="small" disabled={!canRpc(PERM.update)} onClick={() => openUpdate(row)}>
              {t('pages.accounting.accountTypes.actions.edit', 'Edit')}
            </Button>
            <Button type="link" size="small" disabled={!canRpc(PERM.setAssets)} onClick={() => openAssets(row)}>
              {t('pages.accounting.accountTypes.actions.setAssets', 'Set Assets')}
            </Button>
            <Popconfirm
              title={t('pages.accounting.accountTypes.actions.confirmDelete', 'Delete this account type?')}
              onConfirm={() => row.code && deleteAccountType(row.code)}
              okButtonProps={{ danger: true }}
            >
              <Button type="link" size="small" danger disabled={!canRpc(PERM.delete)}>
                {t('pages.accounting.accountTypes.actions.delete', 'Delete')}
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ];
  }, [canRpc, intl]);

  return (
    <PageContainer>
      <ProTable<AccountingAccountTypeItem>
        actionRef={actionRef}
        rowKey="code"
        search={false}
        toolBarRender={() => [
          <Button key="create" type="primary" disabled={!canRpc(PERM.create)} onClick={openCreate}>
            {t('pages.accounting.accountTypes.actions.create', 'New')}
          </Button>,
        ]}
        request={async () => {
          if (!canRpc(PERM.list)) return { data: [], success: true };
          const res = await adminAccountingListAccountTypes({ skipErrorHandler: true });
          if (!res?.success) throw new Error(res?.message || '');
          return { data: res?.data?.items || [], success: true };
        }}
        columns={columns}
      />

      <Modal
        title={editMode === 'create' ? t('pages.accounting.accountTypes.modal.create', 'New Account Type') : t('pages.accounting.accountTypes.modal.edit', 'Edit Account Type')}
        open={editOpen}
        confirmLoading={editLoading}
        onCancel={() => setEditOpen(false)}
        onOk={submitEdit}
        destroyOnClose
      >
        <Form form={editForm} layout="vertical" preserve={false}>
          <Form.Item
            name="code"
            label={t('pages.accounting.accountTypes.form.code', 'Code')}
            rules={[{ required: true, message: t('pages.accounting.accountTypes.form.codeRequired', 'Code is required') }]}
          >
            <Input
              placeholder={t('pages.accounting.accountTypes.form.codePlaceholder', 'e.g. USER_LIABILITY')}
              disabled={editMode === 'update'}
              onBlur={(e) => editForm.setFieldValue('code', (e.target.value || '').trim().toUpperCase())}
            />
          </Form.Item>
          <Form.Item
            name="name"
            label={t('pages.accounting.accountTypes.form.name', 'Name')}
            rules={[{ required: true, message: t('pages.accounting.accountTypes.form.nameRequired', 'Name is required') }]}
          >
            <Input placeholder={t('pages.accounting.accountTypes.form.namePlaceholder', 'e.g. User Liability')} />
          </Form.Item>
          <Form.Item name="description" label={t('pages.accounting.accountTypes.form.description', 'Description')}>
            <Input.TextArea
              placeholder={t('pages.accounting.accountTypes.form.descriptionPlaceholder', 'Short description')}
              maxLength={255}
              showCount
              autoSize={{ minRows: 2, maxRows: 4 }}
            />
          </Form.Item>
          <Form.Item
            name="normal_side"
            label={t('pages.accounting.accountTypes.form.normalSide', 'Normal Side')}
            rules={[{ required: true, message: t('pages.accounting.accountTypes.form.normalSideRequired', 'Normal side is required') }]}
            extra={
              editMode === 'update'
                ? t('pages.accounting.accountTypes.form.normalSideLocked', 'Normal side cannot be changed after creation.')
                : undefined
            }
          >
            <Radio.Group disabled={editMode === 'update'}>
              <Radio value="debit">{t('pages.accounting.accountTypes.form.debit', 'Debit')}</Radio>
              <Radio value="credit">{t('pages.accounting.accountTypes.form.credit', 'Credit')}</Radio>
            </Radio.Group>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={t('pages.accounting.accountTypes.modal.assets', 'Set Supported Assets')}
        open={assetsOpen}
        confirmLoading={assetsLoading}
        onCancel={() => setAssetsOpen(false)}
        onOk={submitAssets}
        destroyOnClose
      >
        <Form form={assetsForm} layout="vertical" preserve={false}>
          <Form.Item name="asset_codes" label={t('pages.accounting.accountTypes.form.assetCodes', 'Assets')}>
            <Select
              mode="multiple"
              allowClear
              options={assetOptions}
              placeholder={t('pages.accounting.accountTypes.form.assetCodesPlaceholder', 'Select assets')}
            />
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {t('pages.accounting.accountTypes.hints.assets', 'Tip: account types without assets will not have balances created for new assets.')}
          </Typography.Paragraph>
        </Form>
      </Modal>
    </PageContainer>
  );
};

export default AccountTypesPage;
