import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App, Button, Card, Form, Select, Space, Typography } from 'antd';
import React, { useEffect, useMemo, useState } from 'react';
import { useRbac } from '@/hooks/useRbac';
import { adminListChains } from '@/api/generated/assets';
import { adminAccountingEnsureSystemAccounts } from '@/api/generated/accounting';

const PERM = {
  ensure: 'AccountingEnsureSystemAccounts',
} as const;

type ChainOption = { label: string; value: string };

const SystemSetupPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const [form] = Form.useForm<{ chain_code?: string }>();
  const [chains, setChains] = useState<ChainOption[]>([]);
  const [loading, setLoading] = useState(false);

  const loadChains = async () => {
    try {
      const res = await adminListChains({ include_disabled: false }, { skipErrorHandler: true });
      if (!res?.success) return;
      const opts =
        res?.data?.chains
          ?.map((c) => (c?.chain_code ? { value: c.chain_code, label: `${c.chain_code}${c.network ? ` (${c.network})` : ''}` } : null))
          .filter((x): x is ChainOption => !!x) || [];
      opts.sort((a, b) => a.value.localeCompare(b.value));
      setChains(opts);
    } catch {
      // ignore
    }
  };

  useEffect(() => {
    loadChains();
  }, []);

  const chainOptions = useMemo(() => chains, [chains]);

  const ensure = async () => {
    if (!canRpc(PERM.ensure)) return;
    const values = await form.validateFields();
    const chain_code = (values.chain_code || '').trim().toUpperCase();
    setLoading(true);
    try {
      const res = await adminAccountingEnsureSystemAccounts({ chain_code }, { skipErrorHandler: true });
      if (!res?.success) throw new Error(res?.message || '');
      message.success(t('pages.accounting.systemSetup.messages.done', 'Done'));
    } catch (e: any) {
      message.error(e?.message || t('pages.accounting.systemSetup.messages.failed', 'Failed'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <PageContainer>
      <Card>
        <Form form={form} layout="vertical" initialValues={{ chain_code: '' }}>
          <Form.Item
            name="chain_code"
            label={t('pages.accounting.systemSetup.form.chainCode', 'Chain Code (optional)')}
            tooltip={t('pages.accounting.systemSetup.form.chainCodeTip', 'If provided, creates SYS_WALLET_HOT scoped by this chain')}
          >
            <Select
              allowClear
              showSearch
              options={chainOptions}
              placeholder={t('pages.accounting.systemSetup.form.chainCodePlaceholder', 'e.g. TRON / ETH / BTC')}
              filterOption={(input, option) => (option?.value || '').toLowerCase().includes(input.toLowerCase())}
            />
          </Form.Item>

          <Space>
            <Button type="primary" loading={loading} disabled={!canRpc(PERM.ensure)} onClick={ensure}>
              {t('pages.accounting.systemSetup.actions.ensure', 'Ensure System Accounts')}
            </Button>
          </Space>

          <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
            {t(
              'pages.accounting.systemSetup.hint',
              'This operation is idempotent. It will create missing system accounts, account-type assets, and zero balances.',
            )}
          </Typography.Paragraph>
        </Form>
      </Card>
    </PageContainer>
  );
};

export default SystemSetupPage;
