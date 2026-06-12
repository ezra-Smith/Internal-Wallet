import React from 'react';
import { ClockCircleOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { ApprovalStats } from '../types';

const useStyles = createStyles(() => ({
  container: {
    display: 'grid',
    gridTemplateColumns: 'repeat(3, 1fr)',
    gap: 16,
    marginBottom: 24,
  },
  card: {
    borderRadius: 12,
    padding: '20px 24px',
    background: '#fff',
    border: '1px solid #f0f0f0',
    display: 'flex',
    alignItems: 'center',
    gap: 16,
    transition: 'all 0.2s ease',
    cursor: 'pointer',
    '&:hover': {
      boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
    },
  },
  cardActive: {
    borderColor: '#3b82f6',
    background: 'linear-gradient(135deg, #eff6ff 0%, #fff 100%)',
  },
  iconWrap: {
    width: 44,
    height: 44,
    borderRadius: 10,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 20,
    flexShrink: 0,
  },
  iconPending: {
    background: 'linear-gradient(135deg, #fef3c7 0%, #fde68a 100%)',
    color: '#f59e0b',
  },
  iconApproved: {
    background: 'linear-gradient(135deg, #d1fae5 0%, #a7f3d0 100%)',
    color: '#10b981',
  },
  iconRejected: {
    background: 'linear-gradient(135deg, #fee2e2 0%, #fecaca 100%)',
    color: '#ef4444',
  },
  content: {
    flex: 1,
  },
  label: {
    fontSize: 14,
    color: '#6b7280',
    marginBottom: 4,
  },
  value: {
    fontSize: 28,
    fontWeight: 700,
    color: '#1f2937',
    fontFamily: '"DIN Alternate", "Bebas Neue", sans-serif',
  },
  unit: {
    fontSize: 14,
    fontWeight: 400,
    color: '#9ca3af',
    marginLeft: 4,
  },
}));

interface WithdrawalsStatsCardProps {
  stats: ApprovalStats;
  activeStatus?: 'pending' | 'approved' | 'rejected' | 'all';
  onStatusClick?: (status: 'pending' | 'approved' | 'rejected' | 'all') => void;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const WithdrawalsStatsCard: React.FC<WithdrawalsStatsCardProps> = ({
  stats,
  activeStatus,
  onStatusClick,
  t,
}) => {
  const { styles } = useStyles();

  const statItems = [
    {
      key: 'pending' as const,
      label: t('pages.approval.stats.pending', '待审核'),
      value: stats.pending,
      icon: <ClockCircleOutlined />,
      iconClass: styles.iconPending,
    },
    {
      key: 'approved' as const,
      label: t('pages.approval.stats.approved', '已通过'),
      value: stats.approved,
      icon: <CheckCircleOutlined />,
      iconClass: styles.iconApproved,
    },
    {
      key: 'rejected' as const,
      label: t('pages.approval.stats.rejected', '已拒绝'),
      value: stats.rejected,
      icon: <CloseCircleOutlined />,
      iconClass: styles.iconRejected,
    },
  ];

  return (
    <div className={styles.container}>
      {statItems.map((item) => (
        <div
          key={item.key}
          className={`${styles.card} ${activeStatus === item.key ? styles.cardActive : ''}`}
          onClick={() => onStatusClick?.(item.key)}
        >
          <div className={`${styles.iconWrap} ${item.iconClass}`}>{item.icon}</div>
          <div className={styles.content}>
            <div className={styles.label}>{item.label}</div>
            <div>
              <span className={styles.value}>{item.value}</span>
              <span className={styles.unit}>{t('pages.approval.unit.count', '笔')}</span>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
};

export default WithdrawalsStatsCard;

