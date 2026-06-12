import React from 'react';
import { Modal, Tag, Button, message } from 'antd';
import {
  SwapOutlined,
  ArrowDownOutlined,
  ArrowUpOutlined,
  CopyOutlined,
  ExportOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { TransactionRecord, TransactionType, TransactionStatus } from '../types';
import { TRANSACTION_TYPE_CONFIG, TRANSACTION_STATUS_CONFIG, NETWORK_CONFIG } from '../constants';

const useStyles = createStyles(() => ({
  modalTitle: {
    display: 'flex',
    flexDirection: 'column',
  },
  titleRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 18,
    fontWeight: 600,
    color: '#1f2937',
  },
  titleIcon: {
    color: '#3b82f6',
  },
  subtitle: {
    fontSize: 14,
    color: '#6b7280',
    fontWeight: 400,
    marginTop: 4,
  },
  // 顶部摘要区域
  summaryCard: {
    borderRadius: 12,
    padding: '20px',
    background: '#f9fafb',
    border: '1px solid #f0f0f0',
    marginBottom: 20,
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  summaryLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  },
  typeTag: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 14,
    fontWeight: 500,
    padding: '8px 16px',
    borderRadius: 8,
    border: '1px solid',
  },
  typeDeposit: {
    background: '#eff6ff',
    color: '#3b82f6',
    borderColor: '#bfdbfe',
  },
  typeWithdraw: {
    background: '#fef3c7',
    color: '#d97706',
    borderColor: '#fcd34d',
  },
  typeSwap: {
    background: '#f3e8ff',
    color: '#7c3aed',
    borderColor: '#c4b5fd',
  },
  typeInternal: {
    background: '#f3f4f6',
    color: '#6b7280',
    borderColor: '#d1d5db',
  },
  statusTag: {
    fontSize: 14,
    padding: '6px 12px',
    borderRadius: 6,
    border: 'none',
    fontWeight: 500,
  },
  statusSuccess: {
    background: '#d1fae5',
    color: '#059669',
  },
  statusPending: {
    background: '#fef3c7',
    color: '#d97706',
  },
  statusFailed: {
    background: '#fee2e2',
    color: '#dc2626',
  },
  summaryRight: {
    textAlign: 'right',
  },
  amount: {
    fontSize: 24,
    fontWeight: 700,
    color: '#1f2937',
  },
  fee: {
    fontSize: 13,
    color: '#6b7280',
    marginTop: 4,
  },
  // 字段行
  fieldRow: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, 1fr)',
    gap: 16,
    marginBottom: 16,
    '@media (max-width: 576px)': {
      gridTemplateColumns: '1fr',
    },
  },
  fieldRowFull: {
    marginBottom: 16,
  },
  fieldItem: {
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
  },
  fieldLabel: {
    fontSize: 13,
    color: '#6b7280',
    fontWeight: 500,
  },
  fieldValue: {
    padding: '12px 16px',
    background: '#f9fafb',
    borderRadius: 8,
    border: '1px solid #f0f0f0',
    fontSize: 14,
    color: '#1f2937',
    fontFamily: 'monospace',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    wordBreak: 'break-all',
  },
  fieldValueText: {
    flex: 1,
    wordBreak: 'break-all',
  },
  fieldActions: {
    display: 'flex',
    gap: 8,
    flexShrink: 0,
  },
  actionBtn: {
    color: '#9ca3af',
    cursor: 'pointer',
    fontSize: 16,
    '&:hover': {
      color: '#3b82f6',
    },
  },
  // 提示信息
  infoBox: {
    borderRadius: 10,
    background: '#eff6ff',
    border: '1px solid #bfdbfe',
    padding: '16px',
    marginTop: 20,
  },
  infoBoxTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 14,
    fontWeight: 600,
    color: '#3b82f6',
    marginBottom: 8,
  },
  infoBoxContent: {
    fontSize: 13,
    color: '#3b82f6',
    lineHeight: 1.6,
  },
}));

interface TransactionDetailModalProps {
  open: boolean;
  transaction: TransactionRecord | null;
  onClose: () => void;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const TransactionDetailModal: React.FC<TransactionDetailModalProps> = ({
  open,
  transaction,
  onClose,
  t,
}) => {
  const { styles } = useStyles();

  if (!transaction) return null;

  const getTypeIcon = (type: TransactionType) => {
    switch (type) {
      case 'deposit':
        return <ArrowDownOutlined />;
      case 'withdraw':
        return <ArrowUpOutlined />;
      case 'swap':
      case 'internal':
        return <SwapOutlined />;
      default:
        return null;
    }
  };

  const getTypeClass = (type: TransactionType) => {
    switch (type) {
      case 'deposit':
        return styles.typeDeposit;
      case 'withdraw':
        return styles.typeWithdraw;
      case 'swap':
        return styles.typeSwap;
      case 'internal':
        return styles.typeInternal;
      default:
        return '';
    }
  };

  const getStatusClass = (status: TransactionStatus) => {
    switch (status) {
      case 'success':
        return styles.statusSuccess;
      case 'pending':
        return styles.statusPending;
      case 'failed':
        return styles.statusFailed;
      default:
        return '';
    }
  };

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    message.success(t('common.copied', '已复制') + ': ' + label);
  };

  const openExplorer = (txHash: string) => {
    // 根据网络打开对应的区块链浏览器
    const explorerUrls: Record<string, string> = {
      ethereum: 'https://etherscan.io/tx/',
      bsc: 'https://bscscan.com/tx/',
      polygon: 'https://polygonscan.com/tx/',
      arbitrum: 'https://arbiscan.io/tx/',
      optimism: 'https://optimistic.etherscan.io/tx/',
    };
    const baseUrl = explorerUrls[transaction.network] || explorerUrls.ethereum;
    window.open(baseUrl + txHash, '_blank');
  };

  // 模拟完整地址（实际应该从API获取）
  const fullFromAddress = '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb';
  const fullToAddress = '0x8B3192f2C7f5e9b7bf6b05d3E5B1e7D8C9A4F3E2';
  const fullTxHash = '0xabc123def456789...';

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={[
        <Button key="close" onClick={onClose}>
          {t('common.close', '关闭')}
        </Button>,
      ]}
      width={560}
      title={
        <div className={styles.modalTitle}>
          <div className={styles.titleRow}>
            <SwapOutlined className={styles.titleIcon} />
            {t('pages.fundFlow.detail.title', '交易详情')}
          </div>
          <div className={styles.subtitle}>
            {t('pages.fundFlow.detail.subtitle', '完整的交易信息和区块链记录')}
          </div>
        </div>
      }
    >
      {/* 摘要信息 */}
      <div className={styles.summaryCard}>
        <div className={styles.summaryLeft}>
          <span className={`${styles.typeTag} ${getTypeClass(transaction.type)}`}>
            {getTypeIcon(transaction.type)}
            {t(`pages.fundFlow.type.${transaction.type}`, TRANSACTION_TYPE_CONFIG[transaction.type].label)}
            </span>
          <Tag className={`${styles.statusTag} ${getStatusClass(transaction.status)}`}>
            {t(`pages.fundFlow.status.${transaction.status}`, TRANSACTION_STATUS_CONFIG[transaction.status].label)}
            </Tag>
          </div>
        <div className={styles.summaryRight}>
            <div className={styles.amount}>
            {transaction.amount.toLocaleString()} {transaction.currency}
            </div>
            <div className={styles.fee}>
            {t('pages.fundFlow.table.fee', '手续费')}: {transaction.fee} {transaction.feeCurrency}
            </div>
          </div>
        </div>

      {/* 地址信息 */}
      <div className={styles.fieldRow}>
        <div className={styles.fieldItem}>
          <span className={styles.fieldLabel}>
                {t('pages.fundFlow.detail.fromAddress', '发起方地址')}
          </span>
          <div className={styles.fieldValue}>
            <span className={styles.fieldValueText}>{fullFromAddress}</span>
            <div className={styles.fieldActions}>
                <CopyOutlined
                className={styles.actionBtn}
                  onClick={() => copyToClipboard(fullFromAddress, t('pages.fundFlow.detail.fromAddress', '发起方地址'))}
                />
              </div>
            </div>
        </div>
        <div className={styles.fieldItem}>
          <span className={styles.fieldLabel}>
                {t('pages.fundFlow.detail.toAddress', '接收方地址')}
          </span>
          <div className={styles.fieldValue}>
            <span className={styles.fieldValueText}>{fullToAddress}</span>
            <div className={styles.fieldActions}>
                <CopyOutlined
                className={styles.actionBtn}
                  onClick={() => copyToClipboard(fullToAddress, t('pages.fundFlow.detail.toAddress', '接收方地址'))}
                />
              </div>
            </div>
          </div>
        </div>

        {/* 交易哈希 */}
      {transaction.txHash && (
        <div className={styles.fieldRowFull}>
          <div className={styles.fieldItem}>
            <span className={styles.fieldLabel}>
              {t('pages.fundFlow.detail.txHash', '交易哈希 (TxHash)')}
            </span>
            <div className={styles.fieldValue}>
              <span className={styles.fieldValueText}>{fullTxHash}</span>
              <div className={styles.fieldActions}>
                <CopyOutlined
                  className={styles.actionBtn}
                  onClick={() => copyToClipboard(transaction.txHash || '', t('pages.fundFlow.detail.txHash', '交易哈希'))}
                />
                <ExportOutlined
                  className={styles.actionBtn}
                  onClick={() => openExplorer(transaction.txHash || '')}
                />
              </div>
              </div>
            </div>
          </div>
        )}

        {/* 网络和时间 */}
      <div className={styles.fieldRow}>
        <div className={styles.fieldItem}>
          <span className={styles.fieldLabel}>
                {t('pages.fundFlow.detail.network', '区块链网络')}
          </span>
          <div className={styles.fieldValue}>
            <span>{NETWORK_CONFIG[transaction.network].label}</span>
              </div>
            </div>
        <div className={styles.fieldItem}>
          <span className={styles.fieldLabel}>
                {t('pages.fundFlow.detail.time', '交易时间')}
          </span>
          <div className={styles.fieldValue}>
            <span>{transaction.timestamp}</span>
            </div>
          </div>
        </div>

        {/* 备注 */}
      <div className={styles.fieldRowFull}>
        <div className={styles.fieldItem}>
          <span className={styles.fieldLabel}>
            {t('pages.fundFlow.detail.remark', '备注')}
            </span>
          <div className={styles.fieldValue}>
            <span>{t('pages.fundFlow.detail.userDeposit', '用户充值')}</span>
          </div>
          </div>
        </div>

        {/* 提示信息 */}
      <div className={styles.infoBox}>
        <div className={styles.infoBoxTitle}>
            <InfoCircleOutlined />
            {t('pages.fundFlow.detail.blockchainRecord', '区块链记录')}
          </div>
        <div className={styles.infoBoxContent}>
            {t(
            'pages.fundFlow.detail.blockchainInfo',
              '所有交易都已记录在区块链上，可以通过交易哈希在区块链浏览器中验证真实性。',
            )}
        </div>
      </div>
    </Modal>
  );
};

export default TransactionDetailModal;
