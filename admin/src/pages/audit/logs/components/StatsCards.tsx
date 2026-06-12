import React from 'react';
import {
  LineChartOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { AuditLogStats } from '../types';

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
  iconFailed: {
    background: 'linear-gradient(135deg, #fef2f2 0%, #fecaca 100%)',
    color: '#ef4444',
  },
  iconToday: {
    background: 'linear-gradient(135deg, #faf5ff 0%, #e9d5ff 100%)',
    color: '#a855f7',
  },
  content: {
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
  },
  label: {
    fontSize: 14,
    color: '#6b7280',
  },
  value: {
    fontSize: 24,
    fontWeight: 700,
    color: '#1f2937',
  },
}));

interface StatsCardsProps {
  stats: AuditLogStats;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const StatsCards: React.FC<StatsCardsProps> = ({ stats, t }) => {
  const { styles } = useStyles();

  const cards = [
    {
      icon: <LineChartOutlined />,
      iconClass: styles.iconTotal,
      label: t('pages.audit.logs.stats.total', '总日志数'),
      value: stats.total,
    },
    {
      icon: <CheckCircleOutlined />,
      iconClass: styles.iconSuccess,
      label: t('pages.audit.logs.stats.success', '成功操作'),
      value: stats.success,
    },
    {
      icon: <ExclamationCircleOutlined />,
      iconClass: styles.iconFailed,
      label: t('pages.audit.logs.stats.failed', '失败操作'),
      value: stats.failed,
    },
    {
      icon: <ClockCircleOutlined />,
      iconClass: styles.iconToday,
      label: t('pages.audit.logs.stats.today', '今日操作'),
      value: stats.today,
    },
  ];

  return (
    <div className={styles.container}>
      {cards.map((card) => (
        <div key={card.label} className={styles.card}>
          <div className={`${styles.iconWrapper} ${card.iconClass}`}>{card.icon}</div>
          <div className={styles.content}>
            <span className={styles.label}>{card.label}</span>
            <span className={styles.value}>{card.value}</span>
          </div>
        </div>
      ))}
    </div>
  );
};

export default StatsCards;
