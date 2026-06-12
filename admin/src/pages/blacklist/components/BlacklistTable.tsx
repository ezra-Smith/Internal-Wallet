import React from 'react';
import { Space, Tag, Tooltip, Typography, Button } from 'antd';
import { CopyOutlined, LinkOutlined, EyeOutlined, EyeInvisibleOutlined } from '@ant-design/icons';
import { ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import { createStyles } from 'antd-style';
import type { BlacklistAddress } from '../types';
import {
  RISK_LEVEL_CONFIG,
  NETWORK_CONFIG,
  SOURCE_CONFIG,
  MONITOR_STATUS_CONFIG,
} from '../constants';

const { Text } = Typography;

const useStyles = createStyles(() => ({
  container: {
    marginTop: 16,
    '.ant-pro-table-list-toolbar': {
      display: 'none',
    },
    '.ant-table-wrapper': {
      borderRadius: 12,
      overflow: 'hidden',
    },
  },
  addressCell: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  address: {
    fontFamily: 'monospace',
    fontSize: 13,
    maxWidth: 260,
  },
  iconBtn: {
    padding: 4,
    fontSize: 14,
    color: '#9ca3af',
    cursor: 'pointer',
    '&:hover': {
      color: '#3b82f6',
    },
  },
  riskTag: {
    borderRadius: 6,
    fontWeight: 500,
    border: 'none',
  },
  networkTag: {
    borderRadius: 6,
    fontWeight: 500,
  },
  statusActive: {
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    color: '#10b981',
  },
  statusStopped: {
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    color: '#9ca3af',
  },
  hitCount: {
    color: '#f59e0b',
    fontWeight: 600,
  },
  hitDate: {
    fontSize: 12,
    color: '#9ca3af',
  },
  addedTime: {
    fontSize: 13,
  },
  addedBy: {
    fontSize: 12,
    color: '#9ca3af',
  },
  actionBtn: {
    padding: '0 8px',
    fontSize: 13,
  },
  actionLink: {
    color: '#3b82f6',
    cursor: 'pointer',
    '&:hover': {
      textDecoration: 'underline',
    },
  },
  actionDanger: {
    color: '#ef4444',
    cursor: 'pointer',
    '&:hover': {
      textDecoration: 'underline',
    },
  },
  actionWarning: {
    color: '#f59e0b',
    cursor: 'pointer',
    '&:hover': {
      textDecoration: 'underline',
    },
  },
}));

interface BlacklistTableProps {
  dataSource: BlacklistAddress[];
  loading?: boolean;
  networkLabelByValue?: Record<string, string>;
  pagination: {
    current: number;
    pageSize: number;
    total: number;
    onChange: (page: number, pageSize: number) => void;
  };
  onDetail: (record: BlacklistAddress) => void;
  onToggleStatus: (record: BlacklistAddress) => void;
  onDelete: (record: BlacklistAddress) => void;
  onCopyAddress: (address: string) => void;
  canDetail?: boolean;
  canToggleStatus?: boolean;
  canDelete?: boolean;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const BlacklistTable: React.FC<BlacklistTableProps> = ({
  dataSource,
  loading,
  networkLabelByValue,
  pagination,
  onDetail,
  onToggleStatus,
  onDelete,
  onCopyAddress,
  canDetail = true,
  canToggleStatus = true,
  canDelete = true,
  t,
}) => {
  const { styles } = useStyles();

  const columns: ProColumns<BlacklistAddress>[] = [
    {
      title: t('pages.blacklist.table.address', '地址'),
      dataIndex: 'address',
      key: 'address',
      width: 320,
      render: (_, record) => (
        <div className={styles.addressCell}>
          <Text ellipsis className={styles.address} title={record.address}>
            {record.address}
          </Text>
          <Tooltip title={t('common.copy', '复制')}>
            <CopyOutlined
              className={styles.iconBtn}
              onClick={() => onCopyAddress(record.address)}
            />
          </Tooltip>
          <Tooltip title={t('pages.blacklist.table.viewOnChain', '在区块链浏览器中查看')}>
            <LinkOutlined className={styles.iconBtn} />
          </Tooltip>
        </div>
      ),
    },
    {
      title: t('pages.blacklist.table.network', '网络'),
      dataIndex: 'network',
      key: 'network',
      width: 120,
      render: (_, record) => {
        const raw = String(record.network || '').trim();
        const key = raw.toLowerCase();
        const config = NETWORK_CONFIG[key];
        const label = networkLabelByValue?.[key] || config?.label || (raw ? raw.toUpperCase() : '-');
        const color = config?.color || '#6b7280';
        return (
          <Tag
            className={styles.networkTag}
            style={{ 
              color, 
              background: `${color}15`,
              borderColor: `${color}30`,
            }}
          >
            {label}
          </Tag>
        );
      },
    },
    {
      title: t('pages.blacklist.table.riskLevel', '风险等级'),
      dataIndex: 'riskLevel',
      key: 'riskLevel',
      width: 100,
      render: (_, record) => {
        const config = RISK_LEVEL_CONFIG[record.riskLevel];
        return (
          <Tag
            className={styles.riskTag}
            style={{ background: config.color, color: '#fff' }}
          >
            {t(`pages.blacklist.riskLevel.${record.riskLevel}`, config.label)}
          </Tag>
        );
      },
    },
    {
      title: t('pages.blacklist.table.source', '来源'),
      dataIndex: 'source',
      key: 'source',
      width: 120,
      render: (_, record) => {
        const config = SOURCE_CONFIG[record.source];
        return <span>{t(`pages.blacklist.source.${record.source}`, config.label)}</span>;
      },
    },
    {
      title: t('pages.blacklist.table.hitCount', '命中次数'),
      dataIndex: 'hitCount',
      key: 'hitCount',
      width: 130,
      render: (_, record) => (
        <div>
          <span className={styles.hitCount}>
            {record.hitCount} {t('pages.blacklist.unit.hit', '次')}
          </span>
          {record.lastHitAt && (
            <div className={styles.hitDate}>
              {t('pages.blacklist.table.lastHit', '最后')}: {record.lastHitAt}
            </div>
          )}
        </div>
      ),
    },
    {
      title: t('pages.blacklist.table.monitorStatus', '监测状态'),
      dataIndex: 'monitorStatus',
      key: 'monitorStatus',
      width: 100,
      render: (_, record) => {
        const config = MONITOR_STATUS_CONFIG[record.monitorStatus];
        const isActive = record.monitorStatus === 'active';
        return (
          <span className={isActive ? styles.statusActive : styles.statusStopped}>
            {isActive ? <EyeOutlined /> : <EyeInvisibleOutlined />}
            {t(`pages.blacklist.monitorStatus.${record.monitorStatus}`, config.label)}
          </span>
        );
      },
    },
    {
      title: t('pages.blacklist.table.addedAt', '添加时间'),
      dataIndex: 'addedAt',
      key: 'addedAt',
      width: 160,
      render: (_, record) => (
        <div>
          <div className={styles.addedTime}>{record.addedAt}</div>
          <div className={styles.addedBy}>{record.addedBy}</div>
        </div>
      ),
    },
    {
      title: t('pages.blacklist.table.actions', '操作'),
      key: 'actions',
      width: 150,
      fixed: 'right',
      render: (_, record) => (
        <Space size={12}>
          <Button
            type="link"
            className={styles.actionBtn}
            style={{ color: '#3b82f6' }}
            onClick={() => onDetail(record)}
            disabled={!canDetail}
          >
            {t('pages.blacklist.actions.detail', '详情')}
          </Button>
          <Button
            type="link"
            className={styles.actionBtn}
            style={{ color: record.monitorStatus === 'active' ? '#f59e0b' : '#10b981' }}
            onClick={() => onToggleStatus(record)}
            disabled={!canToggleStatus}
          >
            {record.monitorStatus === 'active'
              ? t('pages.blacklist.actions.stop', '停止')
              : t('pages.blacklist.actions.start', '启用')}
          </Button>
          <Button
            type="link"
            className={styles.actionBtn}
            style={{ color: '#ef4444' }}
            onClick={() => onDelete(record)}
            disabled={!canDelete}
          >
            {t('pages.blacklist.actions.delete', '删除')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div className={styles.container}>
      <ProTable<BlacklistAddress>
        dataSource={dataSource}
        columns={columns}
        rowKey="id"
        loading={loading}
        search={false}
        options={false}
        scroll={{ x: 1200 }}
        pagination={{
          current: pagination.current,
          pageSize: pagination.pageSize,
          total: pagination.total,
          onChange: pagination.onChange,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) =>
            t('common.pagination.total', '共 {total} 条', { total }),
        }}
      />
    </div>
  );
};

export default BlacklistTable;
