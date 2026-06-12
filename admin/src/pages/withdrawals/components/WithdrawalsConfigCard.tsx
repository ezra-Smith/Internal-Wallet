import React from 'react';
import { Tag } from 'antd';
import { DownloadOutlined, SwapOutlined } from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { ApprovalConfig, ApprovalType } from '../types';

const useStyles = createStyles(() => ({
  container: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, 1fr)',
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
    '&:hover': {
      boxShadow: '0 4px 12px rgba(0, 0, 0, 0.08)',
    },
  },
  cardWithdrawal: {
    borderLeft: '4px solid #3b82f6',
  },
  cardTransfer: {
    borderLeft: '4px solid #8b5cf6',
  },
  iconWrap: {
    width: 48,
    height: 48,
    borderRadius: 12,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 22,
    flexShrink: 0,
  },
  iconWithdrawal: {
    background: 'linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)',
    color: '#3b82f6',
  },
  iconTransfer: {
    background: 'linear-gradient(135deg, #faf5ff 0%, #f3e8ff 100%)',
    color: '#8b5cf6',
  },
  content: {
    flex: 1,
  },
  title: {
    fontSize: 15,
    fontWeight: 600,
    color: '#1f2937',
    marginBottom: 4,
  },
  threshold: {
    fontSize: 13,
    color: '#6b7280',
  },
  thresholdValue: {
    color: '#3b82f6',
    fontWeight: 600,
  },
  statusTag: {
    marginLeft: 'auto',
    flexShrink: 0,
  },
}));

interface WithdrawalsConfigCardProps {
  configs: ApprovalConfig[];
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const WithdrawalsConfigCard: React.FC<WithdrawalsConfigCardProps> = ({ configs, t }) => {
  const { styles } = useStyles();

  const getConfig = (type: ApprovalType) => {
    return configs.find((c) => c.type === type) || { type, enabled: false, threshold: 0 };
  };

  const withdrawalConfig = getConfig('withdrawal');
  const transferConfig = getConfig('transfer');

  return (
    <div className={styles.container}>
      {/* 提现审核卡片 */}
      <div className={`${styles.card} ${styles.cardWithdrawal}`}>
        <div className={`${styles.iconWrap} ${styles.iconWithdrawal}`}>
          <DownloadOutlined />
        </div>
        <div className={styles.content}>
          <div className={styles.title}>
            {t('pages.approval.config.withdrawal.title', '提现审核')}
          </div>
          <div className={styles.threshold}>
            {t('pages.approval.config.threshold', '阈值')}：
            <span className={styles.thresholdValue}>
              ≥ {withdrawalConfig.threshold.toLocaleString()} USDT
            </span>
          </div>
        </div>
        <Tag
          color={withdrawalConfig.enabled ? 'blue' : 'default'}
          className={styles.statusTag}
        >
          {withdrawalConfig.enabled
            ? t('pages.approval.config.enabled', '已启用')
            : t('pages.approval.config.disabled', '已禁用')}
        </Tag>
      </div>

      {/* 转账审核卡片 */}
      <div className={`${styles.card} ${styles.cardTransfer}`}>
        <div className={`${styles.iconWrap} ${styles.iconTransfer}`}>
          <SwapOutlined />
        </div>
        <div className={styles.content}>
          <div className={styles.title}>
            {t('pages.approval.config.transfer.title', '转账审核')}
          </div>
          <div className={styles.threshold}>
            {t('pages.approval.config.threshold', '阈值')}：
            <span className={styles.thresholdValue}>
              ≥ {transferConfig.threshold.toLocaleString()} USDT
            </span>
          </div>
        </div>
        <Tag
          color={transferConfig.enabled ? 'blue' : 'default'}
          className={styles.statusTag}
        >
          {transferConfig.enabled
            ? t('pages.approval.config.enabled', '已启用')
            : t('pages.approval.config.disabled', '已禁用')}
        </Tag>
      </div>
    </div>
  );
};

export default WithdrawalsConfigCard;

