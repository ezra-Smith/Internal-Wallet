import React from 'react';
import { AutoComplete, Button, Collapse, DatePicker, Input, InputNumber, Select } from 'antd';
import { FilterOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { createStyles } from 'antd-style';
import dayjs from 'dayjs';
import type { AuditLogFilters, ModuleType } from '../types';
import { MODULE_CONFIG } from '../constants';

const { RangePicker } = DatePicker;

const useStyles = createStyles(() => ({
  container: {
    display: 'flex',
    flexDirection: 'column',
    gap: 16,
  },
  // 模块标签区域
  moduleTabs: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 12,
    padding: '20px 24px',
    background: '#fff',
    borderRadius: 12,
    border: '1px solid #f0f0f0',
  },
  moduleTab: {
    padding: '8px 16px',
    fontSize: 14,
    borderRadius: 8,
    border: '1px solid #e5e7eb',
    background: '#fff',
    color: '#4b5563',
    cursor: 'pointer',
    transition: 'all 0.2s ease',
    '&:hover': {
      borderColor: '#3b82f6',
      color: '#3b82f6',
    },
  },
  moduleTabActive: {
    background: '#eff6ff',
    borderColor: '#3b82f6',
    color: '#3b82f6',
    fontWeight: 500,
  },
  // 筛选条件区域
  filterCard: {
    padding: '20px 24px',
    background: '#fff',
    borderRadius: 12,
    border: '1px solid #f0f0f0',
    overflow: 'hidden',
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
    gap: 16,
    alignItems: 'end',
  },
  fieldLabel: {
    fontSize: 12,
    color: '#6b7280',
    marginBottom: 6,
    userSelect: 'none',
  },
  input: {
    height: 42,
    borderRadius: 8,
    width: '100%',
  },
  autoComplete: {
    width: '100%',
    height: 42,
    '& .ant-select-selector': {
      height: '42px !important',
      borderRadius: '8px !important',
      alignItems: 'center',
    },
    '& .ant-select-selection-search-input': {
      height: '42px !important',
    },
  },
  select: {
    width: '100%',
    '& .ant-select-selector': {
      height: '42px !important',
      borderRadius: '8px !important',
      alignItems: 'center',
    },
    '& .ant-select-selection-search-input': {
      height: '40px !important',
    },
  },
  dateRangePicker: {
    width: '100%',
    height: 42,
    borderRadius: 8,
  },
  statusFilter: {
    display: 'inline-flex',
    flexShrink: 0,
    borderRadius: 8,
    overflow: 'hidden',
    height: 42, // Fix height
    border: '1px solid #e5e7eb',
    '@media (max-width: 1024px)': {
      width: '100%',
    },
  },
  statusBtn: {
    padding: '0 20px', // Use simplified padding
    height: '100%', // Fill the container
    display: 'flex', // Use flex for centering
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 14,
    fontWeight: 500,
    border: 'none',
    borderRight: '1px solid #e5e7eb',
    background: '#fff',
    color: '#4b5563',
    cursor: 'pointer',
    transition: 'all 0.2s ease',
    whiteSpace: 'nowrap',
    '&:last-child': {
      borderRight: 'none',
    },
    '&:hover': {
      background: '#f9fafb',
      color: '#3b82f6',
    },
    '@media (max-width: 1024px)': {
      flex: 1,
      textAlign: 'center',
    },
  },
  statusBtnActive: {
    background: '#3b82f6',
    borderColor: '#3b82f6',
    color: '#fff',
    '&:hover': {
      background: '#2563eb',
      color: '#fff',
    },
  },
  statusBtnSuccess: {},
  statusBtnSuccessActive: {
    background: '#10b981',
    borderColor: '#10b981',
    color: '#fff',
    '&:hover': {
      background: '#059669',
      color: '#fff',
    },
  },
  statusBtnFailed: {},
  statusBtnFailedActive: {
    background: '#ef4444',
    borderColor: '#ef4444',
    color: '#fff',
    '&:hover': {
      background: '#dc2626',
      color: '#fff',
    },
  },
  actionBar: {
    display: 'flex',
    gap: 12,
    justifyContent: 'flex-end',
    flexWrap: 'wrap',
    '& .ant-btn': {
      height: 42,
      borderRadius: 8,
    },
  },
  advanced: {
    background: '#fff',
    borderRadius: 12,
    border: '1px solid #f0f0f0',
    overflow: 'hidden',
  },
  collapse: {
    background: 'transparent',
    border: 'none',
    '& .ant-collapse-item': {
      borderBottom: 'none',
    },
    '& .ant-collapse-header': {
      padding: '14px 20px',
      fontWeight: 600,
      color: '#374151',
    },
    '& .ant-collapse-content': {
      borderTop: '1px solid #f0f0f0',
    },
    '& .ant-collapse-content-box': {
      padding: '16px 20px 20px 20px',
    },
  },
  splitRow: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: 12,
    '@media (max-width: 768px)': {
      gridTemplateColumns: '1fr',
    },
  },
}));

interface FilterSectionProps {
  draftFilters: AuditLogFilters;
  onDraftFiltersChange: (filters: AuditLogFilters) => void;
  onSearch: () => void;
  onReset: () => void;
  roleOptions?: { value: string; label: string }[];
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const FilterSection: React.FC<FilterSectionProps> = ({
  draftFilters,
  onDraftFiltersChange,
  onSearch,
  onReset,
  roleOptions,
  t,
}) => {
  const { styles } = useStyles();

  const modules: ModuleType[] = [
    'all',
    'account',
    'vault',
    'transfer',
    'user',
    'currency',
    'swap',
    'approval',
    'role',
    'blacklist',
    'system',
    'audit',
  ];

  const getModuleLabel = (module: ModuleType) => {
    const config = MODULE_CONFIG[module];
    if (module === 'all') {
      return t('pages.audit.logs.modules.all', '全部模块');
    }
    return t(`pages.audit.logs.modules.${module}`, config.label);
  };

  const handleModuleChange = (module: ModuleType) => {
    onDraftFiltersChange({ ...draftFilters, module });
  };

  const handleStatusChange = (status: 'all' | 'success' | 'failed') => {
    onDraftFiltersChange({ ...draftFilters, status });
  };

  const handleDateChange = (dates: any) => {
    if (dates) {
      onDraftFiltersChange({
        ...draftFilters,
        dateRange: [
          dates[0]?.startOf('day')?.format('YYYY-MM-DDTHH:mm:ssZ') || null,
          dates[1]?.endOf('day')?.format('YYYY-MM-DDTHH:mm:ssZ') || null,
        ],
      });
    } else {
      onDraftFiltersChange({ ...draftFilters, dateRange: [null, null] });
    }
  };

  const dateRangeValue = (() => {
    const [from, to] = draftFilters.dateRange;
    return [from ? dayjs(from) : null, to ? dayjs(to) : null];
  })();

  const update = (patch: Partial<AuditLogFilters>) => {
    onDraftFiltersChange({ ...draftFilters, ...patch });
  };

  return (
    <div className={styles.container}>
      {/* 模块标签 */}
      {/* <div className={styles.moduleTabs}>
        {modules.map((module) => (
          <button
            key={module}
            type="button"
            className={`${styles.moduleTab} ${draftFilters.module === module ? styles.moduleTabActive : ''}`}
            onClick={() => handleModuleChange(module)}
          >
            {getModuleLabel(module)}
          </button>
        ))}
      </div> */}

      {/* 筛选条件 */}
      <div className={styles.filterCard}>
        <div className={styles.grid}>
          <div>
            <div className={styles.fieldLabel}>{t('pages.audit.logs.filter.dateRange', '时间范围')}</div>
            <RangePicker
              className={styles.dateRangePicker}
              value={dateRangeValue as any}
              placeholder={[
                t('pages.audit.logs.filter.startDate', '开始日期'),
                t('pages.audit.logs.filter.endDate', '结束日期'),
              ]}
              onChange={handleDateChange}
              allowClear
            />
          </div>

          <div>
            <div className={styles.fieldLabel}>{t('pages.audit.logs.filter.status', '状态')}</div>
            <div className={styles.statusFilter}>
              <button
                type="button"
                className={`${styles.statusBtn} ${draftFilters.status === 'all' ? styles.statusBtnActive : ''}`}
                onClick={() => handleStatusChange('all')}
              >
                {t('common.all', '全部')}
              </button>
              <button
                type="button"
                className={`${styles.statusBtn} ${styles.statusBtnSuccess} ${draftFilters.status === 'success' ? styles.statusBtnSuccessActive : ''}`}
                onClick={() => handleStatusChange('success')}
              >
                {t('pages.audit.logs.status.success', '成功')}
              </button>
              <button
                type="button"
                className={`${styles.statusBtn} ${styles.statusBtnFailed} ${draftFilters.status === 'failed' ? styles.statusBtnFailedActive : ''}`}
                onClick={() => handleStatusChange('failed')}
              >
                {t('pages.audit.logs.status.failed', '失败')}
              </button>
            </div>
          </div>

          <div>
            <div className={styles.fieldLabel}>{t('pages.audit.logs.filter.operator', '操作人')}</div>
            <Input
              className={styles.input}
              placeholder={t('pages.audit.logs.filter.operator.placeholder', '管理员ID / 用户名')}
              value={draftFilters.operator}
              onChange={(e) => update({ operator: e.target.value })}
              onPressEnter={onSearch}
              allowClear
            />
          </div>

          <div>
            <div className={styles.fieldLabel}>{t('pages.audit.logs.filter.role', '角色')}</div>
            <AutoComplete
              className={styles.autoComplete}
              options={roleOptions}
              value={draftFilters.role}
              onChange={(v) => update({ role: v })}
              allowClear
            >
              <Input
                className={styles.input}
                placeholder={t('pages.audit.logs.filter.role.placeholder', '角色代码')}
                onPressEnter={onSearch}
              />
            </AutoComplete>
          </div>

          <div>
            <div className={styles.fieldLabel}>{t('pages.audit.logs.filter.action', '操作')}</div>
            <Input
              className={styles.input}
              placeholder={t('pages.audit.logs.filter.action.placeholder', '操作')}
              value={draftFilters.action}
              onChange={(e) => update({ action: e.target.value })}
              onPressEnter={onSearch}
              allowClear
            />
          </div>

          <div>
            <div className={styles.fieldLabel}>{t('pages.audit.logs.filter.target', '目标')}</div>
            <div className={styles.splitRow}>
              <Input
                className={styles.input}
                placeholder={t('pages.audit.logs.filter.targetType.placeholder', '目标类型')}
                value={draftFilters.targetType}
                onChange={(e) => update({ targetType: e.target.value })}
                onPressEnter={onSearch}
                allowClear
              />
              <Input
                className={styles.input}
                placeholder={t('pages.audit.logs.filter.targetId.placeholder', '目标ID')}
                value={draftFilters.targetId}
                onChange={(e) => update({ targetId: e.target.value })}
                onPressEnter={onSearch}
                allowClear
              />
            </div>
          </div>

          <div>
            <div className={styles.fieldLabel}>{t('pages.audit.logs.filter.keyword', '详情关键字')}</div>
            <Input
              className={styles.input}
              placeholder={t('pages.audit.logs.search.placeholder', '搜索详情 / 错误 / HTTP Path / RPC / Request ID ...')}
              prefix={<SearchOutlined style={{ color: '#9ca3af' }} />}
              value={draftFilters.keyword}
              onChange={(e) => update({ keyword: e.target.value })}
              onPressEnter={onSearch}
              allowClear
            />
          </div>

          <div className={styles.actionBar}>
            <Button type="primary" icon={<SearchOutlined />} onClick={onSearch}>
              {t('common.search', '查询')}
            </Button>
            <Button icon={<ReloadOutlined />} onClick={onReset}>
              {t('common.reset', '重置')}
            </Button>
          </div>
        </div>
      </div>

      {/* 高级筛选 */}
      <div className={styles.advanced}>
        <Collapse
          className={styles.collapse}
          ghost
          items={[
            {
              key: 'advanced',
              label: (
                <span>
                  <FilterOutlined style={{ marginRight: 8, color: '#6b7280' }} />
                  {t('pages.audit.logs.filter.advanced', '高级筛选')}
                </span>
              ),
              children: (
                <div className={styles.grid}>
                  <div>
                    <div className={styles.fieldLabel}>IP</div>
                    <Input
                      className={styles.input}
                      placeholder={t('pages.audit.logs.filter.ip.placeholder', 'IP地址')}
                      value={draftFilters.ip}
                      onChange={(e) => update({ ip: e.target.value })}
                      onPressEnter={onSearch}
                      allowClear
                    />
                  </div>

                  <div>
                    <div className={styles.fieldLabel}>Request ID</div>
                    <Input
                      className={styles.input}
                      placeholder={t('pages.audit.logs.filter.requestId.placeholder', '请求ID')}
                      value={draftFilters.requestId}
                      onChange={(e) => update({ requestId: e.target.value })}
                      onPressEnter={onSearch}
                      allowClear
                    />
                  </div>

                  <div>
                    <div className={styles.fieldLabel}>HTTP</div>
                    <div className={styles.splitRow}>
                      <Select
                        className={styles.select}
                        placeholder={t('pages.audit.logs.filter.httpMethod.placeholder', '请求方法')}
                        value={draftFilters.httpMethod || undefined}
                        onChange={(v) => update({ httpMethod: v || '' })}
                        allowClear
                        options={[
                          { value: 'GET', label: 'GET' },
                          { value: 'POST', label: 'POST' },
                          { value: 'PUT', label: 'PUT' },
                          { value: 'PATCH', label: 'PATCH' },
                          { value: 'DELETE', label: 'DELETE' },
                        ]}
                      />
                      <Input
                        className={styles.input}
                        placeholder={t('pages.audit.logs.filter.httpPath.placeholder', '路径包含')}
                        value={draftFilters.httpPath}
                        onChange={(e) => update({ httpPath: e.target.value })}
                        onPressEnter={onSearch}
                        allowClear
                      />
                    </div>
                  </div>

                  <div>
                    <div className={styles.fieldLabel}>RPC</div>
                    <Input
                      className={styles.input}
                      placeholder={t('pages.audit.logs.filter.rpcMethod.placeholder', 'RPC包含')}
                      value={draftFilters.rpcMethod}
                      onChange={(e) => update({ rpcMethod: e.target.value })}
                      onPressEnter={onSearch}
                      allowClear
                    />
                  </div>

                  <div>
                    <div className={styles.fieldLabel}>{t('pages.audit.logs.filter.duration', '耗时(ms)')}</div>
                    <div className={styles.splitRow}>
                      <InputNumber
                        className={styles.select}
                        min={0}
                        placeholder={t('pages.audit.logs.filter.durationFrom.placeholder', '最小值')}
                        value={draftFilters.durationMsFrom ?? undefined}
                        onChange={(v) => update({ durationMsFrom: typeof v === 'number' ? v : null })}
                      />
                      <InputNumber
                        className={styles.select}
                        min={0}
                        placeholder={t('pages.audit.logs.filter.durationTo.placeholder', '最大值')}
                        value={draftFilters.durationMsTo ?? undefined}
                        onChange={(v) => update({ durationMsTo: typeof v === 'number' ? v : null })}
                      />
                    </div>
                  </div>
                </div>
              ),
            },
          ]}
        />
      </div>
    </div>
  );
};

export default FilterSection;
