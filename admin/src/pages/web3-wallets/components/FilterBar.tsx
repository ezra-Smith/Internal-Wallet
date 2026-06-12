import React from 'react';
import { Input, Select } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { Web3WalletFilters } from '../types';

const useStyles = createStyles(() => ({
  container: {
    borderRadius: 12,
    padding: '20px 24px',
    background: '#fff',
    border: '1px solid #f0f0f0',
    marginBottom: 16,
  },
  searchRow: {
    marginBottom: 16,
  },
  searchInput: {
    borderRadius: 8,
    height: 42,
  },
  filtersRow: {
    display: 'grid',
    gridTemplateColumns: 'repeat(3, 1fr)',
    gap: 16,
    '@media (max-width: 768px)': {
      gridTemplateColumns: '1fr',
    },
  },
  filterItem: {
    display: 'flex',
    flexDirection: 'column',
    gap: 6,
  },
  filterLabel: {
    fontSize: 13,
    color: '#6b7280',
  },
  resultCount: {
    fontSize: 14,
    color: '#6b7280',
    marginTop: 16,
  },
  resultHighlight: {
    color: '#3b82f6',
    fontWeight: 600,
  },
}));

interface FilterBarProps {
  filters: Web3WalletFilters;
  onFiltersChange: (filters: Web3WalletFilters) => void;
  deviceCount: number;
  recordCount: number;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const FilterBar: React.FC<FilterBarProps> = ({
  filters,
  onFiltersChange,
  deviceCount,
  recordCount,
  t,
}) => {
  const { styles } = useStyles();

  const statusOptions = [
    { value: 'all', label: t('common.all', '全部') },
    { value: 'active', label: t('pages.web3Wallets.status.active', '正常') },
    { value: 'blacklisted', label: t('pages.web3Wallets.status.blacklisted', '黑名单') },
    { value: 'disabled', label: t('pages.web3Wallets.status.disabled', '已禁用') },
  ];

  const twoFAOptions = [
    { value: 'all', label: t('common.all', '全部') },
    { value: 'enabled', label: t('pages.web3Wallets.twoFA.enabled', '已开启') },
    { value: 'disabled', label: t('pages.web3Wallets.twoFA.disabled', '未开启') },
  ];

  const biometricOptions = [
    { value: 'all', label: t('common.all', '全部') },
    { value: 'enabled', label: t('pages.web3Wallets.biometric.enabled', '已开启') },
    { value: 'disabled', label: t('pages.web3Wallets.biometric.disabled', '未开启') },
  ];

  return (
    <div className={styles.container}>
      <div className={styles.searchRow}>
        <Input
          className={styles.searchInput}
          placeholder={t('pages.web3Wallets.search.placeholder', '搜索设备ID或钱包地址...')}
          prefix={<SearchOutlined style={{ color: '#9ca3af' }} />}
          value={filters.keyword}
          onChange={(e) => onFiltersChange({ ...filters, keyword: e.target.value })}
          allowClear
        />
      </div>

      <div className={styles.filtersRow}>
        <div className={styles.filterItem}>
          <span className={styles.filterLabel}>
            {t('pages.web3Wallets.filters.addressStatus', '地址状态')}
          </span>
          <Select
            value={filters.addressStatus || 'all'}
            options={statusOptions}
            onChange={(value) => onFiltersChange({ ...filters, addressStatus: value })}
            style={{ width: '100%' }}
          />
        </div>

        <div className={styles.filterItem}>
          <span className={styles.filterLabel}>
            {t('pages.web3Wallets.filters.twoFA', '2FA认证')}
          </span>
          <Select
            value={filters.twoFAStatus || 'all'}
            options={twoFAOptions}
            onChange={(value) => onFiltersChange({ ...filters, twoFAStatus: value })}
            style={{ width: '100%' }}
          />
        </div>

        <div className={styles.filterItem}>
          <span className={styles.filterLabel}>
            {t('pages.web3Wallets.filters.biometric', '生物识别')}
          </span>
          <Select
            value={filters.biometricStatus || 'all'}
            options={biometricOptions}
            onChange={(value) => onFiltersChange({ ...filters, biometricStatus: value })}
            style={{ width: '100%' }}
          />
        </div>
      </div>

      <div className={styles.resultCount}>
        {t('pages.web3Wallets.results', '共找到 {deviceCount} 个设备，{recordCount} 条记录', {
          deviceCount: <span className={styles.resultHighlight}>{deviceCount}</span>,
          recordCount: <span className={styles.resultHighlight}>{recordCount}</span>,
        })}
      </div>
    </div>
  );
};

export default FilterBar;

