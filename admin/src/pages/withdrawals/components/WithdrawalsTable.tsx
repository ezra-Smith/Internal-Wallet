import React from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import { ProTable } from '@ant-design/pro-components';
import { Button, Space, Tag, Popconfirm, Tooltip } from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { ApprovalRecord, ApprovalStatus, ApprovalType } from '../types';
import { CURRENCY_COLORS } from '../constants';

const useStyles = createStyles(() => ({
  tableCard: {
    borderRadius: 12,
    background: '#fff',
    border: '1px solid #f0f0f0',
    overflow: 'hidden',
  },
  currencyTag: {
    fontWeight: 600,
    borderRadius: 4,
  },
  statusPending: {
    color: '#f59e0b',
    background: '#fef3c7',
    border: 'none',
  },
  statusApproved: {
    color: '#10b981',
    background: '#d1fae5',
    border: 'none',
  },
  statusRejected: {
    color: '#ef4444',
    background: '#fee2e2',
    border: 'none',
  },
  whitelistTag: {
    color: '#3b82f6',
    background: '#eff6ff',
    border: '1px dashed #93c5fd',
  },
  placeholder: {
    color: '#9ca3af',
    fontStyle: 'italic',
  },
  totalCount: {
    fontSize: 14,
    color: '#6b7280',
    padding: '12px 16px',
    borderTop: '1px solid #f0f0f0',
    textAlign: 'right' as const,
  },
  countHighlight: {
    color: '#3b82f6',
    fontWeight: 600,
  },
  ruleInfo: {
    padding: '16px 20px',
    background: 'linear-gradient(135deg, #eff6ff 0%, #fff 100%)',
    borderTop: '1px solid #f0f0f0',
    borderRadius: '0 0 12px 12px',
    display: 'flex',
    alignItems: 'flex-start',
    gap: 12,
  },
  ruleIcon: {
    color: '#3b82f6',
    fontSize: 16,
    marginTop: 2,
  },
  ruleContent: {
    flex: 1,
  },
  ruleTitle: {
    fontSize: 14,
    fontWeight: 600,
    color: '#1f2937',
    marginBottom: 4,
  },
  ruleDesc: {
    fontSize: 13,
    color: '#6b7280',
  },
  ruleHighlight: {
    color: '#3b82f6',
    fontWeight: 600,
  },
}));

interface WithdrawalsTableProps {
  type: ApprovalType;
  data: ApprovalRecord[];
  loading?: boolean;
  total: number;
  actionRef?: React.MutableRefObject<ActionType | undefined>;
  onApprove?: (record: ApprovalRecord) => void;
  onReject?: (record: ApprovalRecord) => void;
  onWhitelist?: (record: ApprovalRecord) => void;
  onRequest?: (params: any) => Promise<{ data: ApprovalRecord[]; total: number }>;
  threshold: number;
  canApprove: boolean;
  canReject: boolean;
  canWhitelist: boolean;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const WithdrawalsTable: React.FC<WithdrawalsTableProps> = ({
  type,
  data,
  loading,
  total,
  actionRef,
  onApprove,
  onReject,
  onWhitelist,
  onRequest,
  threshold,
  canApprove,
  canReject,
  canWhitelist,
  t,
}) => {
  const { styles } = useStyles();

  const getStatusTag = (status: ApprovalStatus) => {
    const statusConfig: Record<ApprovalStatus, { text: string; className: string }> = {
      pending: {
        text: t('pages.approval.status.pending', '待审核'),
        className: styles.statusPending,
      },
      approved: {
        text: t('pages.approval.status.approved', '通过'),
        className: styles.statusApproved,
      },
      rejected: {
        text: t('pages.approval.status.rejected', '拒绝'),
        className: styles.statusRejected,
      },
    };
    const config = statusConfig[status];
    return <Tag className={config.className}>{config.text}</Tag>;
  };

  const columns: ProColumns<ApprovalRecord>[] = [
    {
      title: 'UID',
      dataIndex: 'uid',
      width: 100,
      copyable: true,
    },
    {
      title: t('pages.approval.columns.name', '姓名'),
      dataIndex: 'name',
      width: 100,
    },
    {
      title: t('pages.approval.columns.phone', '手机号'),
      dataIndex: 'phone',
      width: 130,
      render: (_, record) =>
        record.phone || (
          <span className={styles.placeholder}>
            {t('pages.approval.notSet', '未设置')}
          </span>
        ),
    },
    {
      title: t('pages.approval.columns.email', '邮箱'),
      dataIndex: 'email',
      width: 180,
      ellipsis: true,
      render: (_, record) =>
        record.email || (
          <span className={styles.placeholder}>
            {t('pages.approval.notSet', '未设置')}
          </span>
        ),
    },
    {
      title: t('pages.approval.columns.currency', '币种'),
      dataIndex: 'currency',
      width: 100,
      render: (_, record) => (
        <Tag
          className={styles.currencyTag}
          style={{
            background: CURRENCY_COLORS[record.currency] || '#6b7280',
            color: '#fff',
            border: 'none',
          }}
        >
          {record.currency}
        </Tag>
      ),
    },
    {
      title: t('pages.approval.columns.amount', '金额'),
      dataIndex: 'amount',
      width: 120,
      render: (_, record) => (
        <span style={{ fontWeight: 600, fontFamily: '"DIN Alternate", sans-serif' }}>
          {record.amount.toLocaleString()}
        </span>
      ),
    },
    {
      title: t('pages.approval.columns.applyTime', '申请时间'),
      dataIndex: 'applyTime',
      width: 160,
    },
    {
      title: t('pages.approval.columns.status', '状态'),
      dataIndex: 'status',
      width: 100,
      render: (_, record) => getStatusTag(record.status),
    },
    {
      title: t('pages.approval.columns.actions', '操作'),
      valueType: 'option',
      width: 200,
      render: (_, record) => {
        if (record.status !== 'pending') {
          // 已处理的记录显示白名单操作
          if (record.status === 'approved' && !record.isWhitelist) {
            return (
              <Tooltip title={t('pages.approval.actions.whitelistTip', '加入白名单后，该用户后续提现将免审')}>
                <Button
                  type="link"
                  size="small"
                  icon={<SafetyCertificateOutlined />}
                  disabled={!canWhitelist}
                  onClick={() => onWhitelist?.(record)}
                >
                  {t('pages.approval.actions.whitelist', '白名单免审')}
                </Button>
              </Tooltip>
            );
          }
          if (record.isWhitelist) {
            return (
              <Tag className={styles.whitelistTag} icon={<SafetyCertificateOutlined />}>
                {t('pages.approval.whitelistUser', '白名单免审')}
              </Tag>
            );
          }
          return null;
        }

        return (
          <Space>
            <Button
              type="link"
              size="small"
              icon={<CheckCircleOutlined />}
              disabled={!canApprove}
              onClick={() => onApprove?.(record)}
              style={{ color: '#10b981' }}
            >
              {t('pages.approval.actions.approve', '通过')}
            </Button>
            <Popconfirm
              title={t('pages.approval.actions.rejectConfirm', '确定拒绝此申请？')}
              okText={t('common.confirm', '确定')}
              cancelText={t('common.cancel', '取消')}
              onConfirm={() => onReject?.(record)}
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<CloseCircleOutlined />}
                disabled={!canReject}
              >
                {t('pages.approval.actions.reject', '拒绝')}
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  const ruleTitle =
    type === 'withdrawal'
      ? t('pages.approval.rule.withdrawal.title', '提现审核规则')
      : t('pages.approval.rule.transfer.title', '转账审核规则');

  const ruleDesc =
    type === 'withdrawal'
      ? t(
          'pages.approval.rule.withdrawal.desc',
          '当提现金额（折合USDT）≥ {threshold} 时，需要管理员审核通过后才能完成提现。',
          { threshold: threshold.toLocaleString() },
        )
      : t(
          'pages.approval.rule.transfer.desc',
          '当转账金额（折合USDT）≥ {threshold} 时，需要管理员审核通过后才能完成转账。',
          { threshold: threshold.toLocaleString() },
        );

  return (
    <div className={styles.tableCard}>
      <ProTable<ApprovalRecord>
        actionRef={actionRef}
        rowKey="id"
        columns={columns}
        dataSource={data}
        loading={loading}
        request={onRequest}
        search={false}
        options={false}
        pagination={{
          pageSize: 10,
          showSizeChanger: true,
          showTotal: (totalCount) => `${totalCount} 条记录`,
        }}
        cardProps={{ bodyStyle: { padding: 0 } }}
        tableStyle={{ padding: '0 16px' }}
      />
      <div className={styles.totalCount}>
        {t('pages.approval.total.label', '共')}{' '}
        <span className={styles.countHighlight}>{total}</span>{' '}
        {t('pages.approval.total.records', '条记录')}
      </div>
      <div className={styles.ruleInfo}>
        <SafetyCertificateOutlined className={styles.ruleIcon} />
        <div className={styles.ruleContent}>
          <div className={styles.ruleTitle}>{ruleTitle}</div>
          <div className={styles.ruleDesc}>{ruleDesc}</div>
        </div>
      </div>
    </div>
  );
};

export default WithdrawalsTable;

