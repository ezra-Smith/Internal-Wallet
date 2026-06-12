import React, { useState, useEffect, useCallback } from 'react';
import { Tag } from 'antd';
import {
  WalletOutlined,
  SyncOutlined,
  WarningOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import { adminGetVaultOverview } from '@/api/generated/vault';
import type {
  AdminVaultOverviewSummary,
  AdminVaultNetworkOverviewItem,
  AdminVaultBalanceOverviewItem,
} from '@/api/generated/schemas';

const useStyles = createStyles(() => ({
  // Top Summary Card
  summaryCard: {
    borderRadius: 16,
    padding: '24px 32px',
    background: 'linear-gradient(135deg, #fff 0%, #f8fafc 100%)',
    border: '1px solid #e5e7eb',
    boxShadow: '0 4px 20px rgba(0, 0, 0, 0.04)',
    marginBottom: 20,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  summaryLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 20,
  },
  summaryIcon: {
    width: 56,
    height: 56,
    borderRadius: 14,
    background: 'linear-gradient(135deg, #3b82f6 0%, #60a5fa 100%)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 26,
    color: '#fff',
    boxShadow: '0 4px 12px rgba(59, 130, 246, 0.3)',
  },
  summaryContent: {
    display: 'flex',
    flexDirection: 'column',
  },
  summaryLabel: {
    fontSize: 14,
    color: '#6b7280',
    marginBottom: 4,
  },
  summaryValue: {
    fontSize: 36,
    fontWeight: 700,
    color: '#1f2937',
    fontFamily: '"DIN Alternate", "Bebas Neue", sans-serif',
    lineHeight: 1.2,
  },
  summaryUnit: {
    fontSize: 18,
    fontWeight: 500,
    color: '#6b7280',
    marginLeft: 8,
  },
  summaryRight: {
    display: 'flex',
    alignItems: 'center',
    gap: 32,
  },
  summaryStatItem: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    padding: '0 24px',
    borderRight: '1px solid #e5e7eb',
  },
  summaryStatLabel: {
    fontSize: 13,
    color: '#6b7280',
    marginBottom: 4,
  },
  summaryStatValue: {
    fontSize: 28,
    fontWeight: 700,
    color: '#1f2937',
    fontFamily: '"DIN Alternate", "Bebas Neue", sans-serif',
  },
  summaryStatValueWarning: {
    color: '#ef4444',
  },
  refreshBtn: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    padding: '10px 20px',
    borderRadius: 10,
    background: '#fff',
    border: '1px solid #e5e7eb',
    fontSize: 14,
    color: '#374151',
    cursor: 'pointer',
    transition: 'all 0.2s ease',
    '&:hover': {
      borderColor: '#3b82f6',
      color: '#3b82f6',
    },
  },

  // Warning Section
  warningSection: {
    borderRadius: 16,
    padding: '20px 24px',
    background: 'linear-gradient(135deg, #fef2f2 0%, #fff 100%)',
    border: '1px solid #fecaca',
    marginBottom: 20,
  },
  warningHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 16,
  },
  warningTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    fontSize: 15,
    fontWeight: 600,
    color: '#dc2626',
  },
  warningIcon: {
    fontSize: 18,
  },
  viewAllBtn: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    padding: '6px 14px',
    borderRadius: 8,
    background: '#fff',
    border: '1px solid #fecaca',
    fontSize: 13,
    color: '#dc2626',
    cursor: 'pointer',
    transition: 'all 0.2s ease',
    '&:hover': {
      background: '#fef2f2',
    },
  },
  warningGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, 1fr)',
    gap: 16,
  },
  warningCard: {
    borderRadius: 12,
    padding: '16px 20px',
    background: '#fff',
    border: '1px solid #f3f4f6',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  warningCardLeft: {
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
  },
  warningCardHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  chainTag: {
    padding: '2px 8px',
    borderRadius: 4,
    fontSize: 12,
    fontWeight: 600,
    border: 'none',
  },
  chainTagETH: {
    background: '#e0f2fe',
    color: '#0284c7',
  },
  chainTagBSC: {
    background: '#fef3c7',
    color: '#d97706',
  },
  chainTagTRON: {
    background: '#fee2e2',
    color: '#dc2626',
  },
  tokenName: {
    fontSize: 15,
    fontWeight: 600,
    color: '#1f2937',
  },
  thresholdInfo: {
    fontSize: 13,
    color: '#6b7280',
  },
  thresholdValue: {
    color: '#dc2626',
    fontWeight: 600,
    fontFamily: '"DIN Alternate", sans-serif',
  },
  thresholdTotal: {
    color: '#1f2937',
    fontWeight: 500,
  },
  warningCardRight: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'flex-end',
    gap: 4,
  },
  roleTag: {
    fontSize: 12,
    color: '#6b7280',
    background: '#f3f4f6',
    padding: '2px 8px',
    borderRadius: 4,
  },
  belowThresholdLabel: {
    fontSize: 12,
    color: '#6b7280',
  },
  belowThresholdValue: {
    fontSize: 16,
    fontWeight: 700,
    color: '#dc2626',
    fontFamily: '"DIN Alternate", sans-serif',
  },

  // Chain Cards Grid
  chainCardsGrid: {
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
  chainCard: {
    borderRadius: 14,
    padding: '20px 24px',
    background: '#fff',
    border: '1px solid #f0f0f0',
    boxShadow: '0 2px 12px rgba(0, 0, 0, 0.03)',
    transition: 'all 0.2s ease',
    '&:hover': {
      boxShadow: '0 4px 16px rgba(0, 0, 0, 0.08)',
    },
  },
  chainCardWarning: {
    borderColor: '#fecaca',
    background: 'linear-gradient(135deg, #fff 0%, #fef2f2 100%)',
  },
  chainCardHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    marginBottom: 16,
  },
  chainCardInfo: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  },
  chainLogo: {
    width: 40,
    height: 40,
    borderRadius: 10,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 14,
    fontWeight: 700,
    color: '#fff',
  },
  chainLogoETH: {
    background: 'linear-gradient(135deg, #627eea 0%, #8b9ff5 100%)',
  },
  chainLogoBSC: {
    background: 'linear-gradient(135deg, #f3ba2f 0%, #f7d06f 100%)',
  },
  chainLogoTRON: {
    background: 'linear-gradient(135deg, #eb0029 0%, #ff4d6a 100%)',
  },
  chainLogoPOL: {
    background: 'linear-gradient(135deg, #8247e5 0%, #a879f5 100%)',
  },
  chainLogoDefault: {
    background: 'linear-gradient(135deg, #6b7280 0%, #9ca3af 100%)',
  },
  chainNameWrap: {
    display: 'flex',
    flexDirection: 'column',
  },
  chainName: {
    fontSize: 16,
    fontWeight: 600,
    color: '#1f2937',
  },
  chainTokenCount: {
    fontSize: 12,
    color: '#9ca3af',
  },
  chainStatusIcon: {
    fontSize: 18,
  },
  chainStatusWarning: {
    color: '#f59e0b',
  },
  chainStatusOk: {
    color: '#10b981',
  },
  chainCardBody: {
    marginBottom: 12,
  },
  chainValueLabel: {
    fontSize: 13,
    color: '#6b7280',
    marginBottom: 4,
  },
  chainValue: {
    fontSize: 24,
    fontWeight: 700,
    color: '#1f2937',
    fontFamily: '"DIN Alternate", "Bebas Neue", sans-serif',
  },
  chainCardFooter: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 13,
  },
  chainFooterWarning: {
    color: '#dc2626',
  },
  chainFooterOk: {
    color: '#10b981',
  },
  // Loading state
  loadingText: {
    color: '#9ca3af',
    fontSize: 14,
  },
}));

/** 从 API 数据中提取预警项 */
interface BalanceWarning {
  id: string;
  chain: string;
  token: string;
  current: number;
  threshold: number;
  belowPct: number;
}

function extractWarnings(networks: AdminVaultNetworkOverviewItem[]): BalanceWarning[] {
  const warnings: BalanceWarning[] = [];
  for (const net of networks) {
    const networkName = net.network || 'Unknown';
    const balances = net.balances || [];
    for (const bal of balances) {
      const status = (bal.status || '').toLowerCase();
      if (status === 'low' || status === 'critical') {
        const current = parseFloat(bal.balance || '0');
        const thresholdLow = parseFloat(bal.threshold_low || '0');
        const threshold = thresholdLow > 0 ? thresholdLow : parseFloat(bal.threshold_critical || '0');
        const belowPct = threshold > 0 ? ((threshold - current) / threshold) * 100 : 0;
        warnings.push({
          id: `${networkName}-${bal.currency}`,
          chain: networkName.toUpperCase().replace('ETHEREUM', 'ETH').substring(0, 4),
          token: bal.currency || 'Unknown',
          current,
          threshold,
          belowPct: Math.max(0, belowPct),
        });
      }
    }
  }
  return warnings;
}

interface FundsOverviewProps {
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
  onRefresh?: () => void;
}

const FundsOverview: React.FC<FundsOverviewProps> = ({ t, onRefresh }) => {
  const { styles } = useStyles();
  const [refreshing, setRefreshing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [summary, setSummary] = useState<AdminVaultOverviewSummary | null>(null);
  const [networks, setNetworks] = useState<AdminVaultNetworkOverviewItem[]>([]);
  const [warnings, setWarnings] = useState<BalanceWarning[]>([]);

  const loadData = useCallback(async () => {
    try {
      const res = await adminGetVaultOverview({ skipErrorHandler: true });
      if (res?.success && res.data) {
        setSummary(res.data.summary || null);
        const networkList = res.data.networks || [];
        setNetworks(networkList);
        setWarnings(extractWarnings(networkList));
      }
    } catch (e) {
      console.error('Failed to load vault overview:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await loadData();
      onRefresh?.();
    } finally {
      setRefreshing(false);
    }
  }, [loadData, onRefresh]);

  const getChainTagClass = (chain: string) => {
    const upper = chain.toUpperCase();
    if (upper.includes('ETH')) return styles.chainTagETH;
    if (upper.includes('BSC') || upper.includes('BNB')) return styles.chainTagBSC;
    if (upper.includes('TRON') || upper.includes('TRX')) return styles.chainTagTRON;
    return styles.chainTagETH;
  };

  const getChainLogoClass = (name: string) => {
    const upper = (name || '').toUpperCase();
    if (upper.includes('ETH')) return styles.chainLogoETH;
    if (upper.includes('BSC') || upper.includes('BNB')) return styles.chainLogoBSC;
    if (upper.includes('TRON') || upper.includes('TRX')) return styles.chainLogoTRON;
    if (upper.includes('POL') || upper.includes('MATIC')) return styles.chainLogoPOL;
    return styles.chainLogoDefault;
  };

  const getChainLogoText = (name: string) => {
    const upper = (name || '').toUpperCase();
    if (upper.includes('ETHEREUM')) return 'ETH';
    if (upper.includes('BSC') || upper.includes('BNB')) return 'BSC';
    if (upper.includes('TRON')) return 'TRO';
    if (upper.includes('POLYGON') || upper.includes('MATIC')) return 'POL';
    return (name || 'N/A').substring(0, 3).toUpperCase();
  };

  const formatNumber = (value: number | string | undefined) => {
    const num = typeof value === 'string' ? parseFloat(value) : (value ?? 0);
    return num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  };

  const getNetworkWarningCount = (net: AdminVaultNetworkOverviewItem) => {
    const balances = net.balances || [];
    return balances.filter((b) => {
      const status = (b.status || '').toLowerCase();
      return status === 'low' || status === 'critical';
    }).length;
  };

  const hasNetworkWarning = (net: AdminVaultNetworkOverviewItem) => {
    const status = (net.status || '').toLowerCase();
    return status === 'warning' || status === 'critical' || getNetworkWarningCount(net) > 0;
  };

  // Summary data
  const totalFunds = parseFloat(summary?.total_balance_usd || '0');
  const networkCount = summary?.total_networks ?? networks.length;
  const warningCount = summary?.alerts_count ?? warnings.length;

  if (loading) {
    return (
      <div className={styles.summaryCard}>
        <span className={styles.loadingText}>{t('pages.approval.funds.loading', '加载中...')}</span>
      </div>
    );
  }

  return (
    <>
      {/* 总资金卡片 */}
      <div className={styles.summaryCard}>
        <div className={styles.summaryLeft}>
          <div className={styles.summaryIcon}>
            <WalletOutlined />
          </div>
          <div className={styles.summaryContent}>
            <div className={styles.summaryLabel}>
              {t('pages.approval.funds.totalLabel', '总资金（USDT等值）')}
            </div>
            <div>
              <span className={styles.summaryValue}>{formatNumber(totalFunds)}</span>
              <span className={styles.summaryUnit}>USDT</span>
            </div>
          </div>
        </div>
        <div className={styles.summaryRight}>
          <div className={styles.summaryStatItem}>
            <div className={styles.summaryStatLabel}>
              {t('pages.approval.funds.networkCount', '网络数量')}
            </div>
            <div className={styles.summaryStatValue}>{networkCount}</div>
          </div>
          <div className={styles.summaryStatItem} style={{ borderRight: 'none' }}>
            <div className={styles.summaryStatLabel}>
              {t('pages.approval.funds.warningCount', '预警数量')}
            </div>
            <div className={`${styles.summaryStatValue} ${warningCount > 0 ? styles.summaryStatValueWarning : ''}`}>
              {warningCount}
            </div>
          </div>
          <button type="button" className={styles.refreshBtn} onClick={handleRefresh} disabled={refreshing}>
            <SyncOutlined spin={refreshing} />
            {t('pages.approval.funds.refresh', '刷新余额')}
          </button>
        </div>
      </div>

      {/* 余额预警区域 */}
      {warnings.length > 0 && (
        <div className={styles.warningSection}>
          <div className={styles.warningHeader}>
            <div className={styles.warningTitle}>
              <WarningOutlined className={styles.warningIcon} />
              {t(
                'pages.approval.funds.warningTitle',
                '余额预警：检测到 {count} 个币种余额低于预警阈值，请及时充值',
                { count: warnings.length },
              )}
            </div>
          </div>
          <div className={styles.warningGrid}>
            {warnings.slice(0, 4).map((item) => (
              <div key={item.id} className={styles.warningCard}>
                <div className={styles.warningCardLeft}>
                  <div className={styles.warningCardHeader}>
                    <Tag className={`${styles.chainTag} ${getChainTagClass(item.chain)}`}>
                      {item.chain}
                    </Tag>
                    <span className={styles.tokenName}>{item.token}</span>
                  </div>
                  <div className={styles.thresholdInfo}>
                    {t('pages.approval.funds.currentThreshold', '当前 / 阈值')}
                    <br />
                    <span className={styles.thresholdValue}>{formatNumber(item.current)}</span>
                    <span className={styles.thresholdTotal}> / {formatNumber(item.threshold)}</span>
                  </div>
                </div>
                <div className={styles.warningCardRight}>
                  <div className={styles.belowThresholdLabel}>
                    {t('pages.approval.funds.belowThreshold', '低于阈值')}
                  </div>
                  <div className={styles.belowThresholdValue}>{item.belowPct.toFixed(1)}%</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 链卡片 */}
      <div className={styles.chainCardsGrid}>
        {networks.map((net) => {
          const hasWarning = hasNetworkWarning(net);
          const warnCount = getNetworkWarningCount(net);
          const tokenCount = (net.balances || []).length;
          return (
            <div
              key={net.network_id || net.chain_id || net.network}
              className={`${styles.chainCard} ${hasWarning ? styles.chainCardWarning : ''}`}
            >
              <div className={styles.chainCardHeader}>
                <div className={styles.chainCardInfo}>
                  <div className={`${styles.chainLogo} ${getChainLogoClass(net.network || '')}`}>
                    {getChainLogoText(net.network || '')}
                  </div>
                  <div className={styles.chainNameWrap}>
                    <span className={styles.chainName}>{net.network || 'Unknown'}</span>
                    <span className={styles.chainTokenCount}>
                      {tokenCount} {t('pages.approval.funds.tokens', '币种')}
                    </span>
                  </div>
                </div>
                {hasWarning ? (
                  <WarningOutlined
                    className={`${styles.chainStatusIcon} ${styles.chainStatusWarning}`}
                  />
                ) : (
                  <CheckCircleOutlined
                    className={`${styles.chainStatusIcon} ${styles.chainStatusOk}`}
                  />
                )}
              </div>
              <div className={styles.chainCardBody}>
                <div className={styles.chainValueLabel}>
                  {t('pages.approval.funds.totalValue', '总价值（USDT）')}
                </div>
                <div className={styles.chainValue}>{formatNumber(net.total_balance_usd)}</div>
              </div>
              <div
                className={`${styles.chainCardFooter} ${hasWarning ? styles.chainFooterWarning : styles.chainFooterOk}`}
              >
                {hasWarning ? (
                  <>
                    <WarningOutlined />
                    {t('pages.approval.funds.tokenInsufficient', '{count} 个币种余额不足', {
                      count: warnCount,
                    })}
                  </>
                ) : (
                  <>
                    <CheckCircleOutlined />
                    {t('pages.approval.funds.allNormal', '所有币种余额正常')}
                  </>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </>
  );
};

export default FundsOverview;
