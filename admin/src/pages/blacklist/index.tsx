import React, { useCallback, useEffect, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { App, Modal } from 'antd';
import { useIntl } from '@umijs/max';
import { createStyles } from 'antd-style';
import { useRbac } from '@/hooks/useRbac';
import { adminListChains } from '@/api/generated/assets';
import {
  adminBatchCreateBlacklistAddresses,
  adminCreateBlacklistAddress,
  adminDeleteBlacklistAddress,
  adminExportBlacklistAddresses,
  adminListBlacklistAddresses,
  adminUpdateBlacklistAddressMonitorStatus,
} from '@/api/generated/blacklist';
import StatsCards from './components/StatsCards';
import FilterSection from './components/FilterSection';
import BlacklistTable from './components/BlacklistTable';
import AddBlacklistModal from './components/AddBlacklistModal';
import ImportBlacklistModal from './components/ImportBlacklistModal';
import type { AddBlacklistFormValues } from './components/AddBlacklistModal';
import type { AdminBatchCreateBlacklistAddressItem as BatchCreateBlacklistAddressItem } from '@/api/generated/schemas';
import type { BlacklistFilters, BlacklistAddress, BlacklistStats } from './types';
import { NETWORK_CONFIG, PERM } from './constants';

const useStyles = createStyles(() => ({
  container: {
    minHeight: '100vh',
    padding: 24,
    background: '#f5f7fa',
  },
  description: {
    fontSize: 14,
    color: '#6b7280',
    marginTop: -8,
    marginBottom: 24,
  },
}));

function toNumber(v: string | undefined): number {
  if (!v) return 0;
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

const DEFAULT_FILTERS: BlacklistFilters = {
  keyword: '',
  riskLevel: 'all',
  network: 'all',
  monitorStatus: 'all',
};

const BlacklistPage: React.FC = () => {
  const { styles } = useStyles();
  const intl = useIntl();
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();

  const t = useCallback(
    (id: string, defaultMessage: string, values?: Record<string, any>) =>
      intl.formatMessage({ id, defaultMessage }, values),
    [intl],
  );

  // 筛选状态（draft vs applied；仅点击“搜索”才触发请求）
  const [draftFilters, setDraftFilters] = useState<BlacklistFilters>(DEFAULT_FILTERS);
  const [appliedFilters, setAppliedFilters] = useState<BlacklistFilters>(DEFAULT_FILTERS);

  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);

  // 添加弹窗状态
  const [addModalOpen, setAddModalOpen] = useState(false);

  // 导入弹窗状态
  const [importModalOpen, setImportModalOpen] = useState(false);

  const [stats, setStats] = useState<BlacklistStats>({
    total: 0,
    highRisk: 0,
    highRiskRatio: 0,
    monitoring: 0,
    totalHits: 0,
  });
  const [dataSource, setDataSource] = useState<BlacklistAddress[]>([]);
  const [pagination, setPagination] = useState<{ current: number; pageSize: number; total: number }>({
    current: 1,
    pageSize: 20,
    total: 0,
  });

  const [networksLoading, setNetworksLoading] = useState(false);
  const [networks, setNetworks] = useState<{ value: string; label: string }[]>([]);
  const [networkLabelByValue, setNetworkLabelByValue] = useState<Record<string, string>>({});
  const [networkAliasToValue, setNetworkAliasToValue] = useState<Record<string, string>>({});

  const canList = canRpc(PERM.list);
  const canCreate = canRpc(PERM.create);
  const canBatchCreate = canRpc(PERM.batchCreate);
  const canUpdateMonitor = canRpc(PERM.updateMonitor);
  const canDelete = canRpc(PERM.delete);
  const canExport = canRpc(PERM.export);

  const loadNetworks = useCallback(async () => {
    setNetworksLoading(true);
    try {
      const res = await adminListChains({ include_disabled: false }, { skipErrorHandler: true });
      if (!res?.success) return;
      const list = res.data?.chains ?? [];

      const dedup = new Map<string, { value: string; label: string }>();
      const alias: Record<string, string> = {};

      for (const c of list) {
        const chainCode = (c.chain_code || '').trim();
        const network = (c.network || '').trim();
        const label = network || chainCode;
        const value = label.trim().toLowerCase();
        if (!value) continue;

        if (!dedup.has(value)) {
          dedup.set(value, { value, label });
        }

        alias[value] = value;
        if (network) alias[network.toLowerCase()] = value;
        if (chainCode) alias[chainCode.toLowerCase()] = value;
      }

      const opts = Array.from(dedup.values()).sort((a, b) => a.label.localeCompare(b.label));
      if (!opts.length) return;

      setNetworks(opts);
      setNetworkLabelByValue(Object.fromEntries(opts.map((o) => [o.value, o.label])));
      setNetworkAliasToValue(alias);
    } catch {
      // ignore
    } finally {
      setNetworksLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadNetworks();
  }, [loadNetworks]);

  const load = useCallback(async () => {
    if (!canList) {
      setDataSource([]);
      setStats({ total: 0, highRisk: 0, highRiskRatio: 0, monitoring: 0, totalHits: 0 });
      setPagination((p) => ({ ...p, total: 0 }));
      return;
    }

    setLoading(true);
    try {
      const res = await adminListBlacklistAddresses(
        {
          page: pagination.current,
          page_size: pagination.pageSize,
          keyword: appliedFilters.keyword || undefined,
          risk_level: appliedFilters.riskLevel,
          network: appliedFilters.network,
          monitor_status: appliedFilters.monitorStatus,
        },
        { skipErrorHandler: true },
      );
      if (!res?.success) throw new Error(res?.message || 'load failed');

      const list = res.data?.addresses || [];
      const mapped: BlacklistAddress[] = list.map((x) => ({
        id: x.id || '',
        address: x.address || '',
        network: (x.network || '') as any,
        riskLevel: (x.risk_level || 'low') as any,
        source: (x.source || 'user_report') as any,
        hitCount: toNumber(x.hit_count),
        lastHitAt: x.last_hit_at || undefined,
        monitorStatus: (x.monitor_status || 'active') as any,
        addedAt: x.added_at || '',
        addedBy: x.added_by || '',
        reason: x.reason || undefined,
      }));
      setDataSource(mapped);

      const s = res.data?.stats;
      setStats({
        total: toNumber(s?.total),
        highRisk: toNumber(s?.high_risk),
        highRiskRatio: Number(s?.high_risk_ratio || 0),
        monitoring: toNumber(s?.monitoring),
        totalHits: toNumber(s?.total_hits),
      });

      setPagination((p) => ({ ...p, total: toNumber(res.data?.pagination?.total) }));
    } catch (e: any) {
      message.error(e?.message || t('pages.blacklist.message.loadFailed', '加载失败'));
      setDataSource([]);
      setPagination((p) => ({ ...p, total: 0 }));
    } finally {
      setLoading(false);
    }
  }, [appliedFilters, canList, message, pagination.current, pagination.pageSize, t]);

  useEffect(() => {
    void load();
  }, [load]);

  // 处理添加地址
  const handleAdd = () => {
    setAddModalOpen(true);
  };

  // 处理添加地址提交
  const handleAddSubmit = async (values: AddBlacklistFormValues) => {
    if (!canCreate) return false;
    try {
      const res = await adminCreateBlacklistAddress(
        {
          address: values.address,
          network: values.network,
          risk_level: values.riskLevel,
          source: values.source,
          reason: values.reason,
          monitor_status: 'active',
        },
        { skipErrorHandler: true },
      );
      if (!res?.success) throw new Error(res?.message || 'add failed');
      message.success(t('pages.blacklist.message.addSuccess', '添加成功'));
      setPagination((p) => ({ ...p, current: 1 }));
      return true;
    } catch (e: any) {
      message.error(e?.message || t('pages.blacklist.message.addFailed', '添加失败'));
      return false;
    }
  };

  // 处理批量导入
  const handleImport = () => {
    setImportModalOpen(true);
  };

  // 处理导入提交
  const handleImportSubmit = async (items: BatchCreateBlacklistAddressItem[], opts?: { ignoreExisting?: boolean }) => {
    if (!canBatchCreate) return null;
    try {
      const res = await adminBatchCreateBlacklistAddresses(
        { addresses: items, ignore_existing: !!opts?.ignoreExisting },
        { skipErrorHandler: true },
      );
      return res || null;
    } catch (_e: any) {
      return null;
    }
  };

  // 处理导出
  const handleExport = () => {
    if (!canExport || exporting) return;
    setExporting(true);
    void (async () => {
      try {
        const res = await adminExportBlacklistAddresses(
          {
            format: 'csv',
            keyword: appliedFilters.keyword || undefined,
            risk_level: appliedFilters.riskLevel,
            network: appliedFilters.network,
            monitor_status: appliedFilters.monitorStatus,
          },
          { skipErrorHandler: true },
        );
        if (!res?.success) throw new Error(res?.message || 'export failed');
        const fileName = res.data?.file_name || 'blacklist_addresses.csv';
        const content = res.data?.content || '';

        const blob = new Blob([content], { type: 'text/csv;charset=utf-8' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = fileName;
        a.click();
        window.URL.revokeObjectURL(url);

        message.success(t('pages.blacklist.message.exportSuccess', '导出列表成功'));
      } catch (e: any) {
        message.error(e?.message || t('pages.blacklist.message.exportFailed', '导出失败'));
      } finally {
        setExporting(false);
      }
    })();
  };

  // 处理查看详情
  const handleDetail = (record: BlacklistAddress) => {
    Modal.info({
      title: t('pages.blacklist.modal.detailTitle', '地址详情'),
      width: 600,
      content: (
        <div style={{ marginTop: 16 }}>
          <p><strong>{t('pages.blacklist.table.address', '地址')}:</strong> {record.address}</p>
          <p><strong>{t('pages.blacklist.table.network', '网络')}:</strong> {record.network.toUpperCase()}</p>
          <p><strong>{t('pages.blacklist.table.riskLevel', '风险等级')}:</strong> {record.riskLevel}</p>
          <p><strong>{t('pages.blacklist.table.source', '来源')}:</strong> {record.source}</p>
          <p><strong>{t('pages.blacklist.modal.reason', '原因')}:</strong> {record.reason || '-'}</p>
          <p><strong>{t('pages.blacklist.table.hitCount', '命中次数')}:</strong> {record.hitCount}</p>
          <p><strong>{t('pages.blacklist.table.addedAt', '添加时间')}:</strong> {record.addedAt}</p>
          <p><strong>{t('pages.blacklist.modal.addedBy', '添加人')}:</strong> {record.addedBy}</p>
        </div>
      ),
    });
  };

  // 处理切换监测状态
  const handleToggleStatus = (record: BlacklistAddress) => {
    if (!canUpdateMonitor) return;
    const action = record.monitorStatus === 'active' ? '停止' : '启用';
    const nextStatus = record.monitorStatus === 'active' ? 'stopped' : 'active';
    modal.confirm({
      title: t('pages.blacklist.modal.confirmTitle', '确认操作'),
      content: t('pages.blacklist.modal.toggleStatusConfirm', `确定要${action}该地址的监测吗?`, { action }),
      onOk: async () => {
        try {
          const res = await adminUpdateBlacklistAddressMonitorStatus(
            { id: record.id },
            { monitor_status: nextStatus },
            { skipErrorHandler: true },
          );
          if (!res?.success) throw new Error(res?.message || 'update failed');
          message.success(t('pages.blacklist.message.toggleSuccess', '操作成功'));
          void load();
        } catch (e: any) {
          message.error(e?.message || t('pages.blacklist.message.toggleFailed', '操作失败'));
        }
      },
    });
  };

  // 处理删除
  const handleDelete = (record: BlacklistAddress) => {
    if (!canDelete) return;
    modal.confirm({
      title: t('pages.blacklist.modal.deleteTitle', '确认删除'),
      content: t('pages.blacklist.modal.deleteConfirm', '确定要删除该黑名单地址吗？删除后无法恢复。'),
      okType: 'danger',
      onOk: async () => {
        try {
          const res = await adminDeleteBlacklistAddress({ id: record.id }, { skipErrorHandler: true });
          if (!res?.success) throw new Error(res?.message || 'delete failed');
          message.success(t('pages.blacklist.message.deleteSuccess', '删除成功'));
          setPagination((p) => ({ ...p, current: 1 }));
        } catch (e: any) {
          message.error(e?.message || t('pages.blacklist.message.deleteFailed', '删除失败'));
        }
      },
    });
  };

  // 处理复制地址
  const handleCopyAddress = (address: string) => {
    navigator.clipboard.writeText(address);
    message.success(t('common.message.copySuccess', '复制成功'));
  };

  return (
    <PageContainer
      header={{
        title: t('pages.blacklist.title', '黑名单地址管理'),
      }}
    >
      <div className={styles.container}>
        <div className={styles.description}>
          {t('pages.blacklist.description', '监测和管理链上敏感地址，防范风险交易')}
        </div>

        {/* 统计卡片 */}
        <StatsCards stats={stats} t={t} />

      {/* 筛选区域 */}
      <FilterSection
          filters={draftFilters}
          onFiltersChange={setDraftFilters}
          onSearch={() => {
            setAppliedFilters({ ...draftFilters });
            setPagination((p) => ({ ...p, current: 1 }));
          }}
          onReset={() => {
            setDraftFilters({ ...DEFAULT_FILTERS });
            setAppliedFilters({ ...DEFAULT_FILTERS });
            setPagination((p) => ({ ...p, current: 1 }));
          }}
          networks={networks.length ? networks : Object.entries(NETWORK_CONFIG).map(([value, c]) => ({ value, label: c.label }))}
          networksLoading={networksLoading}
          resultCount={pagination.total}
          onAdd={handleAdd}
          onImport={handleImport}
          onExport={handleExport}
          canAdd={canCreate}
          canImport={canBatchCreate}
          canExport={canExport && !exporting}
          t={t}
        />

        {/* 表格 */}
        <BlacklistTable
          dataSource={dataSource}
          loading={loading}
          networkLabelByValue={networkLabelByValue}
          pagination={{
            current: pagination.current,
            pageSize: pagination.pageSize,
            total: pagination.total,
            onChange: (page, pageSize) => {
              setPagination((p) => ({
                ...p,
                current: pageSize !== p.pageSize ? 1 : page,
                pageSize,
              }));
            },
          }}
          onDetail={handleDetail}
          onToggleStatus={handleToggleStatus}
          onDelete={handleDelete}
          onCopyAddress={handleCopyAddress}
          canDetail={canList}
          canToggleStatus={canUpdateMonitor}
          canDelete={canDelete}
          t={t}
        />
      </div>

      {/* 添加黑名单弹窗 */}
      <AddBlacklistModal
        open={addModalOpen}
        onOpenChange={setAddModalOpen}
        onSubmit={handleAddSubmit}
        networks={networks.length ? networks : Object.entries(NETWORK_CONFIG).map(([value, c]) => ({ value, label: c.label }))}
        defaultNetwork={networks[0]?.value}
        t={t}
      />

      {/* 批量导入弹窗 */}
      <ImportBlacklistModal
        open={importModalOpen}
        onOpenChange={setImportModalOpen}
        onSubmit={handleImportSubmit}
        networks={networks.length ? networks : Object.entries(NETWORK_CONFIG).map(([value, c]) => ({ value, label: c.label }))}
        networkAliasToValue={networkAliasToValue}
        defaultNetwork={networks[0]?.value}
        onSuccess={() => setPagination((p) => ({ ...p, current: 1 }))}
        t={t}
      />
    </PageContainer>
  );
};

export default BlacklistPage;
