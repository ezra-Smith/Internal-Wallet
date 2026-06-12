import React from 'react';
import {
  StopOutlined,
  WarningOutlined,
  EyeOutlined,
  SecurityScanOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { BlacklistStats } from '../types';

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
  iconRed: {
    background: 'linear-gradient(135deg, #fef2f2 0%, #fecaca 100%)',
    color: '#ef4444',
  },
  iconOrange: {
    background: 'linear-gradient(135deg, #fef3c7 0%, #fde68a 100%)',
    color: '#f59e0b',
  },
  iconGreen: {
    background: 'linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%)',
    color: '#10b981',
  },
  iconPurple: {
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
  valueRed: {
    color: '#ef4444',
  },
  valueOrange: {
    color: '#f59e0b',
  },
  valueGreen: {
    color: '#10b981',
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
  stats: BlacklistStats;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const StatsCards: React.FC<StatsCardsProps> = ({ stats, t }) => {
  const { styles } = useStyles();

  return (
    <div className={styles.container}>
      {/* 黑名单地址 */}
      <div className={styles.card}>
        <div className={styles.cardLeft}>
          <div className={`${styles.iconWrapper} ${styles.iconRed}`}>
            <StopOutlined />
          </div>
          <div className={styles.content}>
            <span className={styles.label}>{t('pages.blacklist.stats.total', '黑名单地址')}</span>
            <span className={styles.sublabel}>{t('pages.blacklist.stats.totalCount', '总数量')}</span>
          </div>
        </div>
        <div className={styles.valueWrapper}>
          <span className={`${styles.value} ${styles.valueRed}`}>{stats.total}</span>
          <span className={styles.unit}>{t('pages.blacklist.unit.address', '个地址')}</span>
        </div>
      </div>

      {/* 高风险地址 */}
      <div className={styles.card}>
        <div className={styles.cardLeft}>
          <div className={`${styles.iconWrapper} ${styles.iconOrange}`}>
            <WarningOutlined />
          </div>
          <div className={styles.content}>
            <span className={styles.label}>{t('pages.blacklist.stats.highRisk', '高风险地址')}</span>
            <span className={styles.sublabel}>{t('pages.blacklist.stats.keyMonitor', '重点监测')}</span>
          </div>
        </div>
        <div className={styles.valueWrapper}>
          <span className={`${styles.value} ${styles.valueOrange}`}>{stats.highRisk}</span>
          <div className={styles.subvalue}>
            {t('pages.blacklist.stats.ratio', '占比')} {stats.highRiskRatio}%
          </div>
        </div>
      </div>

      {/* 监测中 */}
      <div className={styles.card}>
        <div className={styles.cardLeft}>
          <div className={`${styles.iconWrapper} ${styles.iconGreen}`}>
            <EyeOutlined />
          </div>
          <div className={styles.content}>
            <span className={styles.label}>{t('pages.blacklist.stats.monitoring', '监测中')}</span>
            <span className={styles.sublabel}>{t('pages.blacklist.stats.realtime', '实时监控')}</span>
          </div>
        </div>
        <div className={styles.valueWrapper}>
          <span className={`${styles.value} ${styles.valueGreen}`}>{stats.monitoring}</span>
          <span className={styles.unit}>{t('pages.blacklist.unit.address', '个地址')}</span>
        </div>
      </div>

      {/* 总命中次数 */}
      <div className={styles.card}>
        <div className={styles.cardLeft}>
          <div className={`${styles.iconWrapper} ${styles.iconPurple}`}>
            <SecurityScanOutlined />
          </div>
          <div className={styles.content}>
            <span className={styles.label}>{t('pages.blacklist.stats.totalHits', '总命中次数')}</span>
            <span className={styles.sublabel}>{t('pages.blacklist.stats.intercepted', '拦截记录')}</span>
          </div>
        </div>
        <div className={styles.valueWrapper}>
          <span className={`${styles.value} ${styles.valuePurple}`}>{stats.totalHits}</span>
          <span className={styles.unit}>{t('pages.blacklist.unit.times', '次拦截')}</span>
        </div>
      </div>
    </div>
  );
};

export default StatsCards;

