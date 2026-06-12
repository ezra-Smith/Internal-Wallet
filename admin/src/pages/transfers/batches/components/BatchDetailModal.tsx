import React from 'react';
import { Modal, Tag, Table } from 'antd';
import {
  EyeOutlined,
  SwapOutlined,
  GiftOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';

const useStyles = createStyles(() => ({
  modalContent: {
    paddingTop: 8,
  },
  batchId: {
    fontSize: 14,
    color: '#6b7280',
    marginBottom: 20,
  },
  infoCard: {
    background: '#f9fafb',
    borderRadius: 12,
    padding: 20,
    marginBottom: 20,
  },
  infoGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(3, 1fr)',
    gap: '16px 24px',
    '@media (max-width: 576px)': {
      gridTemplateColumns: 'repeat(2, 1fr)',
    },
  },
  infoItem: {
    display: 'flex',
    flexDirection: 'column',
    gap: 6,
  },
  infoLabel: {
    fontSize: 12,
    color: '#9ca3af',
  },
  infoValue: {
    fontSize: 15,
    color: '#1f2937',
    fontWeight: 500,
  },
  infoValueGreen: {
    color: '#10b981',
  },
  infoValueRed: {
    color: '#ef4444',
  },
  typeTag: {
    borderRadius: 6,
    fontWeight: 500,
    border: 'none',
    height: 28,
    display: 'inline-flex',
    alignItems: 'center',
    gap: 4,
  },
  statusTag: {
    borderRadius: 6,
    fontWeight: 500,
    border: 'none',
  },
  progressCard: {
    background: '#f0fdf4',
    borderRadius: 12,
    padding: 20,
    marginBottom: 20,
    border: '1px solid #bbf7d0',
  },
  progressHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 16,
  },
  progressTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 15,
    fontWeight: 600,
    color: '#166534',
  },
  progressCount: {
    fontSize: 14,
    color: '#6b7280',
    fontWeight: 500,
  },
  progressBarContainer: {
    marginBottom: 16,
    height: 8,
    borderRadius: 4,
    background: '#e5e7eb',
    overflow: 'hidden',
    display: 'flex',
  },
  progressBarSuccess: {
    height: '100%',
    background: 'linear-gradient(90deg, #10b981 0%, #34d399 100%)',
    transition: 'width 0.3s ease',
  },
  progressBarPending: {
    height: '100%',
    background: 'linear-gradient(90deg, #3b82f6 0%, #60a5fa 100%)',
    transition: 'width 0.3s ease',
  },
  progressBarFailed: {
    height: '100%',
    background: 'linear-gradient(90deg, #ef4444 0%, #f87171 100%)',
    transition: 'width 0.3s ease',
  },
  progressStats: {
    display: 'grid',
    gridTemplateColumns: 'repeat(3, 1fr)',
    gap: 16,
    borderTop: '1px solid #bbf7d0',
    paddingTop: 16,
  },
  progressStat: {
    textAlign: 'center',
  },
  progressStatLabel: {
    fontSize: 12,
    color: '#6b7280',
    marginBottom: 4,
  },
  progressStatValue: {
    fontSize: 20,
    fontWeight: 700,
  },
  progressStatValueGreen: {
    color: '#10b981',
  },
  progressStatValueBlue: {
    color: '#3b82f6',
  },
  progressStatValueRed: {
    color: '#ef4444',
  },
  detailSection: {
    marginTop: 20,
  },
  detailHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 12,
  },
  detailTitle: {
    fontSize: 15,
    fontWeight: 600,
    color: '#1f2937',
  },
  legend: {
    display: 'flex',
    gap: 16,
    fontSize: 12,
  },
  legendItem: {
    display: 'flex',
    alignItems: 'center',
    gap: 4,
  },
  legendDot: {
    width: 8,
    height: 8,
    borderRadius: '50%',
  },
  legendDotGreen: {
    background: '#10b981',
  },
  legendDotBlue: {
    background: '#3b82f6',
  },
  legendDotRed: {
    background: '#ef4444',
  },
  legendDotGray: {
    background: '#9ca3af',
  },
  currencyTag: {
    borderRadius: 4,
    fontWeight: 500,
    fontSize: 12,
  },
  statusCell: {
    borderRadius: 6,
    padding: '4px 8px',
    fontSize: 12,
    fontWeight: 500,
    display: 'inline-flex',
    alignItems: 'center',
    gap: 4,
  },
  statusSuccess: {
    background: '#ecfdf5',
    color: '#10b981',
  },
  statusPending: {
    background: '#eff6ff',
    color: '#3b82f6',
  },
  statusFailed: {
    background: '#fef2f2',
    color: '#ef4444',
  },
  statusWaiting: {
    background: '#f3f4f6',
    color: '#6b7280',
  },
  contactInfo: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
  },
  phone: {
    fontSize: 14,
    color: '#1f2937',
  },
  email: {
    fontSize: 12,
    color: '#9ca3af',
  },
  transferInfo: {
    fontSize: 12,
    color: '#6b7280',
  },
  transferInfoError: {
    color: '#ef4444',
    display: 'flex',
    alignItems: 'center',
    gap: 4,
  },
}));

interface TransferItem {
  id: string;
  uid: string;
  phone: string;
  email: string;
  amount: string;
  currency: string;
  status: 'success' | 'pending' | 'failed' | 'waiting';
  transferTime?: string;
  errorMessage?: string;
}

interface BatchDetailData {
  batchId: string;
  name: string;
  type: 'normal' | 'airdrop';
  status: string;
  createdAt: string;
  executedAt: string;
  totalRecipients: number;
  totalAmount: string;
  currency: string;
  successCount: number;
  failedCount: number;
  pendingCount?: number;
  transfers: TransferItem[];
}

interface BatchDetailModalProps {
  open: boolean;
  onClose: () => void;
  data: BatchDetailData | null;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const BatchDetailModal: React.FC<BatchDetailModalProps> = ({
  open,
  onClose,
  data,
  t,
}) => {
  const { styles } = useStyles();

  if (!data) return null;

  const successRate = data.totalRecipients > 0
    ? ((data.successCount / data.totalRecipients) * 100).toFixed(1)
    : '0.0';

  // 计算进度条各部分的百分比
  const total = data.totalRecipients || 1;
  const successPercent = (data.successCount / total) * 100;
  const pendingPercent = ((data.pendingCount || 0) / total) * 100;
  const failedPercent = (data.failedCount / total) * 100;

  const getStatusStyle = (status: string) => {
    switch (status) {
      case 'success':
        return styles.statusSuccess;
      case 'pending':
        return styles.statusPending;
      case 'failed':
        return styles.statusFailed;
      case 'waiting':
        return styles.statusWaiting;
      default:
        return '';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success':
        return <CheckCircleOutlined />;
      case 'pending':
        return <ClockCircleOutlined />;
      case 'failed':
        return <CloseCircleOutlined />;
      case 'waiting':
        return <ClockCircleOutlined />;
      default:
        return null;
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'success':
        return t('pages.transfers.batches.detail.status.success', '已发放');
      case 'pending':
        return t('pages.transfers.batches.detail.status.pending', '发放中');
      case 'failed':
        return t('pages.transfers.batches.detail.status.failed', '失败');
      case 'waiting':
        return t('pages.transfers.batches.detail.status.waiting', '待发放');
      default:
        return status;
    }
  };

  const columns = [
    {
      title: 'UID',
      dataIndex: 'uid',
      key: 'uid',
      width: 80,
    },
    {
      title: t('pages.transfers.batches.detail.columns.contact', '联系方式'),
      key: 'contact',
      width: 160,
      render: (_: any, record: TransferItem) => (
        <div className={styles.contactInfo}>
          <span className={styles.phone}>{record.phone}</span>
          <span className={styles.email}>{record.email}</span>
        </div>
      ),
    },
    {
      title: t('pages.transfers.batches.detail.columns.amount', '金额'),
      dataIndex: 'amount',
      key: 'amount',
      width: 80,
      render: (amount: string) => (
        <span style={{ fontWeight: 500 }}>{amount}</span>
      ),
    },
    {
      title: t('pages.transfers.batches.detail.columns.currency', '币种'),
      dataIndex: 'currency',
      key: 'currency',
      width: 70,
      render: (currency: string) => (
        <Tag
          className={styles.currencyTag}
          color={currency === 'USDT' ? 'green' : 'blue'}
        >
          {currency}
        </Tag>
      ),
    },
    {
      title: t('pages.transfers.batches.detail.columns.status', '状态'),
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status: string) => (
        <span className={`${styles.statusCell} ${getStatusStyle(status)}`}>
          {getStatusIcon(status)}
          {getStatusText(status)}
        </span>
      ),
    },
    {
      title: t('pages.transfers.batches.detail.columns.info', '发放信息'),
      key: 'info',
      width: 140,
      render: (_: any, record: TransferItem) => {
        if (record.status === 'failed' && record.errorMessage) {
          return (
            <span className={styles.transferInfoError}>
              <ExclamationCircleOutlined />
              {record.errorMessage}
            </span>
          );
        }
        if (record.status === 'success' && record.transferTime) {
          return (
            <span className={styles.transferInfo}>
              {record.transferTime}
            </span>
          );
        }
        return '-';
      },
    },
  ];

  return (
    <Modal
      title={
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <EyeOutlined style={{ color: '#6b7280' }} />
          {t('pages.transfers.batches.detail.title', '批次详情')}
        </span>
      }
      open={open}
      onCancel={onClose}
      width={780}
      footer={null}
      destroyOnClose
    >
      <div className={styles.modalContent}>
        <div className={styles.batchId}>
          {t('pages.transfers.batches.detail.batchId', '批次ID')}: {data.batchId}
        </div>

        {/* 基本信息 */}
        <div className={styles.infoCard}>
          <div className={styles.infoGrid}>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.name', '批次名称')}
              </span>
              <span className={styles.infoValue}>{data.name}</span>
            </div>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.batchIdLabel', '批次ID')}
              </span>
              <span className={styles.infoValue}>{data.batchId}</span>
            </div>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.type', '转账类型')}
              </span>
              <Tag
                className={styles.typeTag}
                style={{
                  color: data.type === 'airdrop' ? '#a855f7' : '#3b82f6',
                  background: data.type === 'airdrop' ? '#faf5ff' : '#eff6ff',
                }}
              >
                {data.type === 'airdrop' ? <GiftOutlined /> : <SwapOutlined />}
                {data.type === 'airdrop'
                  ? t('pages.transfers.batches.type.airdrop', '平台空投')
                  : t('pages.transfers.batches.type.normal', '一般转账')}
              </Tag>
            </div>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.statusLabel', '状态')}
              </span>
              <Tag
                className={styles.statusTag}
                style={{ background: '#ecfdf5', color: '#10b981' }}
              >
                {t('pages.transfers.batches.status.completed', '已完成')}
              </Tag>
            </div>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.createdAt', '创建时间')}
              </span>
              <span className={styles.infoValue}>{data.createdAt}</span>
            </div>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.executedAt', '执行时间')}
              </span>
              <span className={styles.infoValue}>{data.executedAt}</span>
            </div>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.recipients', '转账人数')}
              </span>
              <span className={styles.infoValue}>
                {data.totalRecipients} {t('pages.transfers.batches.unit.person', '人')}
              </span>
            </div>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.totalAmount', '总金额')}
              </span>
              <span className={styles.infoValue}>
                {data.totalAmount} {data.currency}
              </span>
            </div>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.successCount', '成功数量')}
              </span>
              <span className={`${styles.infoValue} ${styles.infoValueGreen}`}>
                {data.successCount} {t('pages.transfers.batches.unit.person', '人')}
              </span>
            </div>
            <div className={styles.infoItem}>
              <span className={styles.infoLabel}>
                {t('pages.transfers.batches.detail.failedCount', '失败数量')}
              </span>
              <span className={`${styles.infoValue} ${styles.infoValueRed}`}>
                {data.failedCount} {t('pages.transfers.batches.unit.person', '人')}
              </span>
            </div>
          </div>
        </div>

        {/* 发放进度统计 */}
        <div className={styles.progressCard}>
          <div className={styles.progressHeader}>
            <span className={styles.progressTitle}>
              <CheckCircleOutlined />
              {t('pages.transfers.batches.detail.progressTitle', '发放进度统计')}
            </span>
            <span className={styles.progressCount}>
              {data.successCount} / {data.totalRecipients}
            </span>
          </div>
          <div className={styles.progressBarContainer}>
            <div
              className={styles.progressBarSuccess}
              style={{ width: `${successPercent}%` }}
            />
            <div
              className={styles.progressBarPending}
              style={{ width: `${pendingPercent}%` }}
            />
            <div
              className={styles.progressBarFailed}
              style={{ width: `${failedPercent}%` }}
            />
          </div>
          <div className={styles.progressStats}>
            <div className={styles.progressStat}>
              <div className={styles.progressStatLabel}>
                {t('pages.transfers.batches.detail.successRate', '成功率')}
              </div>
              <div className={`${styles.progressStatValue} ${styles.progressStatValueGreen}`}>
                {successRate}%
              </div>
            </div>
            <div className={styles.progressStat}>
              <div className={styles.progressStatLabel}>
                {t('pages.transfers.batches.detail.success', '成功')}
              </div>
              <div className={`${styles.progressStatValue} ${styles.progressStatValueBlue}`}>
                {data.successCount}
              </div>
            </div>
            <div className={styles.progressStat}>
              <div className={styles.progressStatLabel}>
                {t('pages.transfers.batches.detail.failed', '失败')}
              </div>
              <div className={`${styles.progressStatValue} ${styles.progressStatValueRed}`}>
                {data.failedCount}
              </div>
            </div>
          </div>
        </div>

        {/* 发放详情 */}
        <div className={styles.detailSection}>
          <div className={styles.detailHeader}>
            <span className={styles.detailTitle}>
              {t('pages.transfers.batches.detail.detailTitle', '发放详情')}
            </span>
            <div className={styles.legend}>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotGreen}`} />
                {t('pages.transfers.batches.detail.status.success', '已发放')}
              </span>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotBlue}`} />
                {t('pages.transfers.batches.detail.status.pending', '发放中')}
              </span>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotRed}`} />
                {t('pages.transfers.batches.detail.status.failed', '失败')}
              </span>
              <span className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.legendDotGray}`} />
                {t('pages.transfers.batches.detail.status.waiting', '待发放')}
              </span>
            </div>
          </div>
          <Table
            dataSource={data.transfers}
            columns={columns}
            rowKey="id"
            size="small"
            pagination={false}
            scroll={{ y: 240 }}
          />
        </div>
      </div>
    </Modal>
  );
};

export default BatchDetailModal;
