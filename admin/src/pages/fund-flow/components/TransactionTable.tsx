import React from 'react';
import { Tag, Button } from 'antd';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  SwapOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { TransactionRecord, TransactionType, TransactionStatus, NetworkType } from '../types';
import { TRANSACTION_TYPE_CONFIG, TRANSACTION_STATUS_CONFIG, NETWORK_CONFIG } from '../constants';

const useStyles = createStyles(() => ({
  container: {
    background: '#fff',
    borderRadius: 12,
    border: '1px solid #f0f0f0',
    overflow: 'hidden',
  },
  header: {
    display: 'grid',
    gridTemplateColumns: '120px 150px 1fr 120px 100px 150px 100px',
    gap: 16,
    padding: '16px 24px',
    background: '#fafbfc',
    borderBottom: '1px solid #f0f0f0',
    '@media (max-width: 1200px)': {
      display: 'none',
    },
  },
  headerCell: {
    fontSize: 14,
    fontWeight: 600,
    color: '#4b5563',
  },
  row: {
    display: 'grid',
    gridTemplateColumns: '120px 150px 1fr 120px 100px 150px 100px',
    gap: 16,
    padding: '16px 24px',
    borderBottom: '1px solid #f5f5f5',
    alignItems: 'center',
    transition: 'background 0.2s ease',
    '&:last-child': {
      borderBottom: 'none',
    },
    '&:hover': {
      background: '#fafbfc',
    },
    '@media (max-width: 1200px)': {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'flex-start',
      gap: 12,
    },
  },
  cell: {
    fontSize: 14,
    color: '#1f2937',
  },
  typeTag: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 13,
    fontWeight: 500,
    padding: '6px 12px',
    borderRadius: 6,
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
  amountCell: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
  },
  amount: {
    fontSize: 15,
    fontWeight: 600,
    color: '#1f2937',
  },
  fee: {
    fontSize: 12,
    color: '#9ca3af',
  },
  addressCell: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 13,
    fontFamily: 'monospace',
    color: '#4b5563',
  },
  addressArrow: {
    color: '#9ca3af',
    fontSize: 12,
  },
  networkTag: {
    fontSize: 12,
    padding: '4px 10px',
    borderRadius: 6,
    border: 'none',
    fontWeight: 500,
  },
  statusTag: {
    fontSize: 12,
    padding: '4px 10px',
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
  timeCell: {
    fontSize: 13,
    color: '#6b7280',
  },
  actionBtn: {
    fontSize: 13,
    color: '#3b82f6',
    padding: 0,
    height: 'auto',
  },
  emptyState: {
    padding: '60px 24px',
    textAlign: 'center',
    color: '#9ca3af',
  },
}));

interface TransactionTableProps {
  transactions: TransactionRecord[];
  onViewDetail: (record: TransactionRecord) => void;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const TransactionTable: React.FC<TransactionTableProps> = ({
  transactions,
  onViewDetail,
  t,
}) => {
  const { styles } = useStyles();

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

  const getNetworkStyle = (network: NetworkType) => {
    const config = NETWORK_CONFIG[network];
    return {
      background: `${config.color}15`,
      color: config.color,
    };
  };

  if (transactions.length === 0) {
    return (
      <div className={styles.container}>
        <div className={styles.emptyState}>{t('common.noData', '暂无数据')}</div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      {/* 表头 */}
      <div className={styles.header}>
        <div className={styles.headerCell}>{t('pages.fundFlow.table.type', '交易类型')}</div>
        <div className={styles.headerCell}>{t('pages.fundFlow.table.amount', '金额')}</div>
        <div className={styles.headerCell}>{t('pages.fundFlow.table.address', '发起方 → 接收方')}</div>
        <div className={styles.headerCell}>{t('pages.fundFlow.table.network', '网络')}</div>
        <div className={styles.headerCell}>{t('pages.fundFlow.table.status', '状态')}</div>
        <div className={styles.headerCell}>{t('pages.fundFlow.table.time', '时间')}</div>
        <div className={styles.headerCell}>{t('pages.fundFlow.table.action', '操作')}</div>
      </div>

      {/* 数据行 */}
      {transactions.map((tx) => (
        <div key={tx.id} className={styles.row}>
          {/* 交易类型 */}
          <div className={styles.cell}>
            <span className={`${styles.typeTag} ${getTypeClass(tx.type)}`}>
              {getTypeIcon(tx.type)}
              {t(`pages.fundFlow.type.${tx.type}`, TRANSACTION_TYPE_CONFIG[tx.type].label)}
            </span>
          </div>

          {/* 金额 */}
          <div className={styles.amountCell}>
            <span className={styles.amount}>
              {tx.amount.toLocaleString()} {tx.currency}
            </span>
            <span className={styles.fee}>
              {t('pages.fundFlow.table.fee', '手续费')}: {tx.fee} {tx.feeCurrency}
            </span>
          </div>

          {/* 地址 */}
          <div className={styles.addressCell}>
            <span>{tx.fromAddress}</span>
            <span className={styles.addressArrow}>⇄</span>
            <span>{tx.toAddress}</span>
          </div>

          {/* 网络 */}
          <div className={styles.cell}>
            <Tag className={styles.networkTag} style={getNetworkStyle(tx.network)}>
              {NETWORK_CONFIG[tx.network].label}
            </Tag>
          </div>

          {/* 状态 */}
          <div className={styles.cell}>
            <Tag className={`${styles.statusTag} ${getStatusClass(tx.status)}`}>
              {t(`pages.fundFlow.status.${tx.status}`, TRANSACTION_STATUS_CONFIG[tx.status].label)}
            </Tag>
          </div>

          {/* 时间 */}
          <div className={styles.timeCell}>{tx.timestamp}</div>

          {/* 操作 */}
          <div className={styles.cell}>
            <Button type="link" className={styles.actionBtn} onClick={() => onViewDetail(tx)}>
              {t('pages.fundFlow.actions.viewDetail', '查看详情')}
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
};

export default TransactionTable;

