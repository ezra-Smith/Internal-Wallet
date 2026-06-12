import React from 'react';
import {
  MobileOutlined,
  WalletOutlined,
  StopOutlined,
  GlobalOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { Web3WalletStats } from '../types';

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
    transition: 'all 0.2s ease',
    '&:hover': {
      boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
    },
  },
  cardLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 14,
  },
  iconWrap: {
    width: 44,
    height: 44,
    borderRadius: 10,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 20,
  },
  iconBlue: {
    background: 'linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)',
    color: '#3b82f6',
  },
  iconGreen: {
    background: 'linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%)',
    color: '#10b981',
  },
  iconRed: {
    background: 'linear-gradient(135deg, #fef2f2 0%, #fee2e2 100%)',
    color: '#ef4444',
  },
  iconPurple: {
    background: 'linear-gradient(135deg, #f5f3ff 0%, #ede9fe 100%)',
    color: '#8b5cf6',
  },
  content: {
    display: 'flex',
    flexDirection: 'column',
  },
  title: {
    fontSize: 15,
    fontWeight: 600,
    color: '#1f2937',
    marginBottom: 2,
  },
  subtitle: {
    fontSize: 12,
    color: '#9ca3af',
  },
  cardRight: {
    textAlign: 'right' as const,
  },
  value: {
    fontSize: 28,
    fontWeight: 700,
    color: '#1f2937',
    fontFamily: '"DIN Alternate", "Bebas Neue", sans-serif',
    lineHeight: 1,
  },
  valueRed: {
    color: '#ef4444',
  },
  valueGreen: {
    color: '#10b981',
  },
  unit: {
    fontSize: 12,
    color: '#9ca3af',
    marginTop: 4,
  },
}));

interface StatsCardsProps {
  stats: Web3WalletStats;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const StatsCards: React.FC<StatsCardsProps> = ({ stats, t }) => {
  const { styles } = useStyles();

  const cards = [
    {
      key: 'deviceCount',
      icon: <MobileOutlined />,
      iconClass: styles.iconBlue,
      title: t('pages.web3Wallets.stats.deviceCount', '设备总数'),
      subtitle: t('pages.web3Wallets.stats.deviceSubtitle', '●●一设备ID'),
      value: stats.deviceCount,
      unit: t('pages.web3Wallets.stats.deviceUnit', '{count}个记录', { count: stats.recordCount }),
    },
    {
      key: 'addressCount',
      icon: <WalletOutlined />,
      iconClass: styles.iconGreen,
      title: t('pages.web3Wallets.stats.addressCount', '钱包地址'),
      subtitle: t('pages.web3Wallets.stats.addressSubtitle', '所有绑定地址'),
      value: stats.addressCount,
      unit: t('pages.web3Wallets.stats.addressUnit', '{count}个网络', { count: stats.networkCount }),
      valueClass: styles.valueGreen,
    },
    {
      key: 'blacklistCount',
      icon: <StopOutlined />,
      iconClass: styles.iconRed,
      title: t('pages.web3Wallets.stats.blacklistCount', '黑名单地址'),
      subtitle: t('pages.web3Wallets.stats.blacklistSubtitle', '受限制的地址'),
      value: stats.blacklistCount,
      unit: t('pages.web3Wallets.stats.blacklistUnit', '占比 {percent}%', { percent: stats.blacklistPercent.toFixed(1) }),
      valueClass: styles.valueRed,
    },
    {
      key: 'supportedNetworks',
      icon: <GlobalOutlined />,
      iconClass: styles.iconPurple,
      title: t('pages.web3Wallets.stats.supportedNetworks', '支持网络'),
      subtitle: t('pages.web3Wallets.stats.networksSubtitle', '已启用的区块链'),
      value: stats.supportedNetworks,
      unit: t('pages.web3Wallets.stats.networksUnit', '个网络'),
      valueClass: styles.valueGreen,
    },
  ];

  return (
    <div className={styles.container}>
      {cards.map((card) => (
        <div key={card.key} className={styles.card}>
          <div className={styles.cardLeft}>
            <div className={`${styles.iconWrap} ${card.iconClass}`}>{card.icon}</div>
            <div className={styles.content}>
              <div className={styles.title}>{card.title}</div>
              <div className={styles.subtitle}>{card.subtitle}</div>
            </div>
          </div>
          <div className={styles.cardRight}>
            <div className={`${styles.value} ${card.valueClass || ''}`}>{card.value}</div>
            <div className={styles.unit}>{card.unit}</div>
          </div>
        </div>
      ))}
    </div>
  );
};

export default StatsCards;
