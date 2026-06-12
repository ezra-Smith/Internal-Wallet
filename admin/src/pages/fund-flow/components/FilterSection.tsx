import React from 'react';
import { Input, Select, DatePicker, Button } from 'antd';
import { SearchOutlined, DownloadOutlined } from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { FundFlowFilters } from '../types';
import { TRANSACTION_TYPE_CONFIG, TRANSACTION_STATUS_CONFIG, NETWORK_CONFIG, CURRENCIES } from '../constants';

const useStyles = createStyles(() => ({
  container: {
    display: 'flex',
    flexDirection: 'column',
    gap: 16,
  },
  searchRow: {
    display: 'flex',
    gap: 16,
    padding: '20px 24px',
    background: '#fff',
    borderRadius: 12,
    border: '1px solid #f0f0f0',
    '@media (max-width: 768px)': {
      flexDirection: 'column',
    },
  },
  searchInput: {
    flex: 1,
    height: 42,
    borderRadius: 8,
  },
  exportBtn: {
    height: 42,
    borderRadius: 8,
    fontWeight: 500,
    flexShrink: 0,
  },
  filterCard: {
    padding: '20px 24px',
    background: '#fff',
    borderRadius: 12,
    border: '1px solid #f0f0f0',
  },
  filterGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(4, 1fr)',
    gap: 16,
    marginBottom: 16,
    '@media (max-width: 1200px)': {
      gridTemplateColumns: 'repeat(2, 1fr)',
    },
    '@media (max-width: 768px)': {
      gridTemplateColumns: '1fr',
    },
  },
  dateRow: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, 1fr)',
    gap: 16,
    '@media (max-width: 768px)': {
      gridTemplateColumns: '1fr',
    },
  },
  filterItem: {
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
  },
  filterLabel: {
    fontSize: 13,
    color: '#6b7280',
    fontWeight: 500,
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

interface FilterSectionProps {
  filters: FundFlowFilters;
  onFiltersChange: (filters: FundFlowFilters) => void;
  resultCount: number;
  onExport: () => void;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const FilterSection: React.FC<FilterSectionProps> = ({
  filters,
  onFiltersChange,
  resultCount,
  onExport,
  t,
}) => {
  const { styles } = useStyles();

  const typeOptions = [
    { value: 'all', label: t('common.all', '全部') },
    ...Object.entries(TRANSACTION_TYPE_CONFIG).map(([key, config]) => ({
      value: key,
      label: t(`pages.fundFlow.type.${key}`, config.label),
    })),
  ];

  const statusOptions = [
    { value: 'all', label: t('common.all', '全部') },
    ...Object.entries(TRANSACTION_STATUS_CONFIG).map(([key, config]) => ({
      value: key,
      label: t(`pages.fundFlow.status.${key}`, config.label),
    })),
  ];

  const networkOptions = [
    { value: 'all', label: t('common.all', '全部') },
    ...Object.entries(NETWORK_CONFIG).map(([key, config]) => ({
      value: key,
      label: config.label,
    })),
  ];

  const currencyOptions = [
    { value: '', label: t('common.all', '全部') },
    ...CURRENCIES.map((currency) => ({
      value: currency,
      label: currency,
    })),
  ];

  return (
    <div className={styles.container}>
      {/* 搜索和导出 */}
      <div className={styles.searchRow}>
        <Input
          className={styles.searchInput}
          placeholder={t('pages.fundFlow.search.placeholder', '搜索交易哈希、地址或备注...')}
          prefix={<SearchOutlined style={{ color: '#9ca3af' }} />}
          value={filters.keyword}
          onChange={(e) => onFiltersChange({ ...filters, keyword: e.target.value })}
          allowClear
        />
        <Button
          type="primary"
          icon={<DownloadOutlined />}
          className={styles.exportBtn}
          onClick={onExport}
        >
          {t('pages.fundFlow.actions.export', '导出记录')}
        </Button>
      </div>

      {/* 筛选条件 */}
      <div className={styles.filterCard}>
        <div className={styles.filterGrid}>
          <div className={styles.filterItem}>
            <span className={styles.filterLabel}>
              {t('pages.fundFlow.filter.type', '交易类型')}
            </span>
            <Select
              value={filters.type}
              options={typeOptions}
              onChange={(value) => onFiltersChange({ ...filters, type: value })}
              style={{ width: '100%' }}
            />
          </div>

          <div className={styles.filterItem}>
            <span className={styles.filterLabel}>
              {t('pages.fundFlow.filter.status', '交易状态')}
            </span>
            <Select
              value={filters.status}
              options={statusOptions}
              onChange={(value) => onFiltersChange({ ...filters, status: value })}
              style={{ width: '100%' }}
            />
          </div>

          <div className={styles.filterItem}>
            <span className={styles.filterLabel}>
              {t('pages.fundFlow.filter.network', '区块链网络')}
            </span>
            <Select
              value={filters.network}
              options={networkOptions}
              onChange={(value) => onFiltersChange({ ...filters, network: value })}
              style={{ width: '100%' }}
            />
          </div>

          <div className={styles.filterItem}>
            <span className={styles.filterLabel}>
              {t('pages.fundFlow.filter.currency', '币种')}
            </span>
            <Select
              value={filters.currency}
              options={currencyOptions}
              onChange={(value) => onFiltersChange({ ...filters, currency: value })}
              style={{ width: '100%' }}
            />
          </div>
        </div>

        <div className={styles.dateRow}>
          <div className={styles.filterItem}>
            <span className={styles.filterLabel}>
              {t('pages.fundFlow.filter.startDate', '开始日期')}
            </span>
            <DatePicker
              showTime
              style={{ width: '100%' }}
              placeholder={t('pages.fundFlow.filter.selectStartDate', '选择开始日期')}
              onChange={(date) =>
                onFiltersChange({
                  ...filters,
                  startDate: date?.format('YYYY-MM-DD HH:mm:ss') || null,
                })
              }
            />
          </div>

          <div className={styles.filterItem}>
            <span className={styles.filterLabel}>
              {t('pages.fundFlow.filter.endDate', '结束日期')}
            </span>
            <DatePicker
              showTime
              style={{ width: '100%' }}
              placeholder={t('pages.fundFlow.filter.selectEndDate', '选择结束日期')}
              onChange={(date) =>
                onFiltersChange({
                  ...filters,
                  endDate: date?.format('YYYY-MM-DD HH:mm:ss') || null,
                })
              }
            />
          </div>
        </div>

        <div className={styles.resultCount}>
          {t('pages.fundFlow.results', '共找到')}{' '}
          <span className={styles.resultHighlight}>{resultCount}</span>{' '}
          {t('pages.fundFlow.resultsUnit', '条交易记录')}
        </div>
      </div>
    </div>
  );
};

export default FilterSection;

