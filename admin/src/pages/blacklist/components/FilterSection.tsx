import React from 'react';
import { Input, Select, Button } from 'antd';
import { SearchOutlined, ReloadOutlined, PlusOutlined, UploadOutlined, DownloadOutlined } from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { BlacklistFilters } from '../types';
import { RISK_LEVEL_CONFIG, MONITOR_STATUS_CONFIG } from '../constants';

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
    '@media (max-width: 992px)': {
      flexDirection: 'column',
    },
  },
  searchInput: {
    flex: 1,
    height: 42,
    borderRadius: 8,
  },
  actionBtns: {
    display: 'flex',
    gap: 12,
    flexShrink: 0,
    '@media (max-width: 992px)': {
      flexWrap: 'wrap',
    },
  },
  searchBtn: {
    height: 42,
    borderRadius: 8,
    fontWeight: 500,
  },
  resetBtn: {
    height: 42,
    borderRadius: 8,
    fontWeight: 500,
  },
  addBtn: {
    height: 42,
    borderRadius: 8,
    fontWeight: 500,
  },
  importBtn: {
    height: 42,
    borderRadius: 8,
    fontWeight: 500,
    borderColor: '#f59e0b',
    color: '#f59e0b',
    '&:hover': {
      borderColor: '#d97706',
      color: '#d97706',
    },
  },
  exportBtn: {
    height: 42,
    borderRadius: 8,
    fontWeight: 500,
  },
  filterCard: {
    padding: '20px 24px',
    background: '#fff',
    borderRadius: 12,
    border: '1px solid #f0f0f0',
  },
  filterGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(3, 1fr)',
    gap: 16,
    '@media (max-width: 992px)': {
      gridTemplateColumns: 'repeat(2, 1fr)',
    },
    '@media (max-width: 576px)': {
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
  filters: BlacklistFilters;
  onFiltersChange: (filters: BlacklistFilters) => void;
  onSearch: () => void;
  onReset: () => void;
  networks: { value: string; label: string }[];
  networksLoading?: boolean;
  resultCount: number;
  onAdd: () => void;
  onImport: () => void;
  onExport: () => void;
  canAdd?: boolean;
  canImport?: boolean;
  canExport?: boolean;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const FilterSection: React.FC<FilterSectionProps> = ({
  filters,
  onFiltersChange,
  onSearch,
  onReset,
  networks,
  networksLoading = false,
  resultCount,
  onAdd,
  onImport,
  onExport,
  canAdd = true,
  canImport = true,
  canExport = true,
  t,
}) => {
  const { styles } = useStyles();

  const riskLevelOptions = [
    { value: 'all', label: t('common.all', '全部') },
    ...Object.entries(RISK_LEVEL_CONFIG).map(([key, config]) => ({
      value: key,
      label: t(`pages.blacklist.riskLevel.${key}`, config.label),
    })),
  ];

  const networkOptions = [
    { value: 'all', label: t('common.all', '全部') },
    ...networks,
  ];

  const statusOptions = [
    { value: 'all', label: t('common.all', '全部') },
    ...Object.entries(MONITOR_STATUS_CONFIG).map(([key, config]) => ({
      value: key,
      label: t(`pages.blacklist.monitorStatus.${key}`, config.label),
    })),
  ];

  return (
    <div className={styles.container}>
      {/* 搜索和操作按钮 */}
      <div className={styles.searchRow}>
        <Input
          className={styles.searchInput}
          placeholder={t('pages.blacklist.search.placeholder', '搜索地址、原因或来源...')}
          prefix={<SearchOutlined style={{ color: '#9ca3af' }} />}
          value={filters.keyword}
          onChange={(e) => onFiltersChange({ ...filters, keyword: e.target.value })}
          onPressEnter={onSearch}
          allowClear
        />
        <div className={styles.actionBtns}>
          <Button
            type="primary"
            icon={<SearchOutlined />}
            className={styles.searchBtn}
            onClick={onSearch}
          >
            {t('common.search', '搜索')}
          </Button>
          <Button
            icon={<ReloadOutlined />}
            className={styles.resetBtn}
            onClick={onReset}
          >
            {t('common.reset', '重置')}
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            className={styles.addBtn}
            onClick={onAdd}
            disabled={!canAdd}
          >
            {t('pages.blacklist.actions.add', '添加地址')}
          </Button>
          <Button
            icon={<UploadOutlined />}
            className={styles.importBtn}
            onClick={onImport}
            disabled={!canImport}
          >
            {t('pages.blacklist.actions.import', '批量导入')}
          </Button>
          <Button
            type="primary"
            icon={<DownloadOutlined />}
            className={styles.exportBtn}
            onClick={onExport}
            disabled={!canExport}
          >
            {t('pages.blacklist.actions.export', '导出列表')}
          </Button>
        </div>
      </div>

      {/* 筛选条件 */}
      <div className={styles.filterCard}>
        <div className={styles.filterGrid}>
          <div className={styles.filterItem}>
            <span className={styles.filterLabel}>
              {t('pages.blacklist.filter.riskLevel', '风险等级')}
            </span>
            <Select
              value={filters.riskLevel}
              options={riskLevelOptions}
              onChange={(value) => onFiltersChange({ ...filters, riskLevel: value })}
              style={{ width: '100%' }}
            />
          </div>

          <div className={styles.filterItem}>
            <span className={styles.filterLabel}>
              {t('pages.blacklist.filter.network', '区块链网络')}
            </span>
            <Select
              value={filters.network}
              options={networkOptions}
              onChange={(value) => onFiltersChange({ ...filters, network: value })}
              loading={networksLoading}
              style={{ width: '100%' }}
            />
          </div>

          <div className={styles.filterItem}>
            <span className={styles.filterLabel}>
              {t('pages.blacklist.filter.status', '监测状态')}
            </span>
            <Select
              value={filters.monitorStatus}
              options={statusOptions}
              onChange={(value) => onFiltersChange({ ...filters, monitorStatus: value })}
              style={{ width: '100%' }}
            />
          </div>
        </div>

        <div className={styles.resultCount}>
          {t('pages.blacklist.results', '共找到')}{' '}
          <span className={styles.resultHighlight}>{resultCount}</span>{' '}
          {t('pages.blacklist.resultsUnit', '个黑名单地址')}
        </div>
      </div>
    </div>
  );
};

export default FilterSection;
