import React from 'react';
import {
  SwapOutlined,
  CheckCircleOutlined,
  SyncOutlined,
  DollarCircleOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { FundFlowStats } from '../types';

const useStyles = createStyles(() => ({
  container: {
    display: 'grid',
    gridTemplateColumns: 'repeat(4, 1fr)',
    gap: 16,
    marginBottom: 24,
    '@media (max-width: 1200px)': {
      gridTemplateColumns: 'repeat(2, 1fr)',
    },
    '@media (max-width: 768px)': {
      gridTemplateColumns: '1fr',
    },
  },
  card: {
    borderRadius: 12,
    padding: '20px 24px',
    background: '#fff',
    border: '1px solid #f0f0f0',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  cardLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 16,
  },
  iconWrapper: {
    width: 48,
    height: 48,
    borderRadius: 12,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 22,
  },
  iconTotal: {
    background: 'linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)',
    color: '#3b82f6',
  },
  iconSuccess: {
    background: 'linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%)',
    color: '#10b981',
  },
  iconPending: {
    background: 'linear-gradient(135deg, #fef3c7 0%, #fde68a 100%)',
    color: '#f59e0b',
  },
  iconToday: {
    background: 'linear-gradient(135deg, #faf5ff 0%, #e9d5ff 100%)',
    color: '#a855f7',
  },
  content: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
  },
  label: {
    fontSize: 14,
    color: '#6b7280',
  },
  sublabel: {
    fontSize: 12,
    color: '#9ca3af',
  },
  valueWrapper: {
    textAlign: 'right',
  },
  value: {
    fontSize: 28,
    fontWeight: 700,
    fontFamily: '"DIN Alternate", "Bebas Neue", sans-serif',
  },
  valueBlue: {
    color: '#3b82f6',
  },
  valueGreen: {
    color: '#10b981',
  },
  valueOrange: {
    color: '#f59e0b',
  },
  valuePurple: {
    color: '#a855f7',
  },
  unit: {
    fontSize: 14,
    color: '#6b7280',
    marginLeft: 4,
  },
  subvalue: {
    fontSize: 12,
    color: '#9ca3af',
    marginTop: 2,
  },
}));

interface StatsCardsProps {
  stats: FundFlowStats;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const StatsCards: React.FC<StatsCardsProps> = ({ stats, t }) => {
  const { styles } = useStyles();

  return (
    <div className={styles.container}>
      {/* 总交易数 */}
      <div className={styles.card}>
        <div className={styles.cardLeft}>
          <div className={`${styles.iconWrapper} ${styles.iconTotal}`}>
            <SwapOutlined />
          </div>
          <div className={styles.content}>
            <span className={styles.label}>{t('pages.fundFlow.stats.total', '总交易数')}</span>
            <span className={styles.sublabel}>{t('pages.fundFlow.stats.allRecords', '所有记录')}</span>
          </div>
        </div>
        <div className={styles.valueWrapper}>
          <span className={`${styles.value} ${styles.valueBlue}`}>{stats.total}</span>
          <span className={styles.unit}>{t('pages.fundFlow.unit.tx', '笔交易')}</span>
        </div>
      </div>

      {/* 成功交易 */}
      <div className={styles.card}>
        <div className={styles.cardLeft}>
          <div className={`${styles.iconWrapper} ${styles.iconSuccess}`}>
            <CheckCircleOutlined />
          </div>
          <div className={styles.content}>
            <span className={styles.label}>{t('pages.fundFlow.stats.success', '成功交易')}</span>
            <span className={styles.sublabel}>{t('pages.fundFlow.stats.completed', '已完成')}</span>
          </div>
        </div>
        <div className={styles.valueWrapper}>
          <span className={`${styles.value} ${styles.valueGreen}`}>{stats.success}</span>
          <div className={styles.subvalue}>
            {t('pages.fundFlow.stats.successRate', '成功率')} {stats.successRate}%
          </div>
        </div>
      </div>

      {/* 处理中 */}
      <div className={styles.card}>
        <div className={styles.cardLeft}>
          <div className={`${styles.iconWrapper} ${styles.iconPending}`}>
            <SyncOutlined />
          </div>
          <div className={styles.content}>
            <span className={styles.label}>{t('pages.fundFlow.stats.pending', '处理中')}</span>
            <span className={styles.sublabel}>{t('pages.fundFlow.stats.waitingConfirm', '待确认')}</span>
          </div>
        </div>
        <div className={styles.valueWrapper}>
          <span className={`${styles.value} ${styles.valueOrange}`}>{stats.pending}</span>
          <span className={styles.unit}>{t('pages.fundFlow.unit.tx', '笔交易')}</span>
        </div>
      </div>

      {/* 今日交易额 */}
      <div className={styles.card}>
        <div className={styles.cardLeft}>
          <div className={`${styles.iconWrapper} ${styles.iconToday}`}>
            <DollarCircleOutlined />
          </div>
          <div className={styles.content}>
            <span className={styles.label}>{t('pages.fundFlow.stats.todayAmount', '今日交易额')}</span>
            <span className={styles.sublabel}>{t('pages.fundFlow.stats.usdEquivalent', '折合USD')}</span>
          </div>
        </div>
        <div className={styles.valueWrapper}>
          <span className={`${styles.value} ${styles.valuePurple}`}>${stats.todayAmount.toLocaleString()}</span>
          <div className={styles.subvalue}>{stats.todayCount}{t('pages.fundFlow.unit.tx', '笔交易')}</div>
        </div>
      </div>
    </div>
  );
};

export default StatsCards;

