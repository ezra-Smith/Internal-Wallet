import { ProCard } from '@ant-design/pro-components';
import { Typography } from 'antd';
import React from 'react';
import type { I18nT } from '../types';

export type CurrenciesOverview = { total?: number; enabled?: number; disabled?: number; base?: string };

type Props = {
  overview: CurrenciesOverview;
  t: I18nT;
};

const OverviewCards: React.FC<Props> = ({ overview, t }) => {
  // Derive disabled from total - enabled to avoid incorrect/negative backend values
  const hasTotal = typeof overview.total === 'number';
  const hasEnabled = typeof overview.enabled === 'number';
  const derivedDisabled = hasTotal && hasEnabled ? Math.max((overview.total as number) - (overview.enabled as number), 0) : undefined;
  // Prefer derived value; fall back to provided value if numbers are incomplete
  const disabledDisplay = typeof derivedDisabled === 'number' ? derivedDisabled : typeof overview.disabled === 'number' ? Math.max(overview.disabled, 0) : '-';
  const enabledDisplay = hasEnabled ? (overview.enabled as number) : '-';

  return (
    <ProCard gutter={16} wrap style={{ marginBottom: 16 }}>
      <ProCard colSpan={{ xs: 24, md: 8 }} bordered>
        <Typography.Text type="secondary">{t('pages.assets.currencies.cards.total', 'Total currencies')}</Typography.Text>
        <div style={{ fontSize: 24, fontWeight: 600 }}>{overview.total ?? '-'}</div>
      </ProCard>
      <ProCard colSpan={{ xs: 24, md: 8 }} bordered>
        <Typography.Text type="secondary">
          {t('pages.assets.currencies.cards.enabled', 'Enabled / Disabled')}
        </Typography.Text>
        <div style={{ fontSize: 24, fontWeight: 600 }}>
          {`${enabledDisplay} / ${disabledDisplay}`}
        </div>
      </ProCard>
      <ProCard colSpan={{ xs: 24, md: 8 }} bordered>
        <Typography.Text type="secondary">{t('pages.assets.currencies.cards.base', 'Base currency')}</Typography.Text>
        <div style={{ fontSize: 24, fontWeight: 600 }}>{overview.base ?? 'USDT'}</div>
      </ProCard>
    </ProCard>
  );
};

export default OverviewCards;
