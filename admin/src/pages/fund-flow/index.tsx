import React, { useState, useMemo } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App } from 'antd';
import { createStyles } from 'antd-style';
import { useRbac } from '@/hooks/useRbac';

import StatsCards from './components/StatsCards';
import FilterSection from './components/FilterSection';
import TransactionTable from './components/TransactionTable';
import TransactionDetailModal from './components/TransactionDetailModal';
import type { FundFlowFilters, TransactionRecord } from './types';
import { MOCK_STATS, MOCK_TRANSACTIONS } from './constants';

const useStyles = createStyles(() => ({
  page: {
    background: '#f5f7fa',
    minHeight: '100vh',
  },
  tableWrapper: {
    marginTop: 16,
  },
}));

const FundFlowPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const { styles } = useStyles();

  const t = (id: string, defaultMessage: string, values?: Record<string, any>) =>
    intl.formatMessage({ id, defaultMessage }, values);

  // State
  const [filters, setFilters] = useState<FundFlowFilters>({
    keyword: '',
    type: 'all',
    status: 'all',
    network: 'all',
    currency: '',
    startDate: null,
    endDate: null,
  });
  const [selectedTransaction, setSelectedTransaction] = useState<TransactionRecord | null>(null);

  // 使用模拟数据
  const stats = MOCK_STATS;

  // 筛选交易记录
  const filteredTransactions = useMemo(() => {
    return MOCK_TRANSACTIONS.filter((tx) => {
      // 关键词筛选
      if (filters.keyword) {
        const keyword = filters.keyword.toLowerCase();
        const matchHash = tx.txHash?.toLowerCase().includes(keyword);
        const matchFrom = tx.fromAddress.toLowerCase().includes(keyword);
        const matchTo = tx.toAddress.toLowerCase().includes(keyword);
        if (!matchHash && !matchFrom && !matchTo) {
          return false;
        }
      }

      // 类型筛选
      if (filters.type && filters.type !== 'all') {
        if (tx.type !== filters.type) {
          return false;
        }
      }

      // 状态筛选
      if (filters.status && filters.status !== 'all') {
        if (tx.status !== filters.status) {
          return false;
        }
      }

      // 网络筛选
      if (filters.network && filters.network !== 'all') {
        if (tx.network !== filters.network) {
          return false;
        }
      }

      // 币种筛选
      if (filters.currency) {
        if (tx.currency !== filters.currency) {
          return false;
        }
      }

      // 日期筛选
      if (filters.startDate && tx.timestamp < filters.startDate) {
        return false;
      }
      if (filters.endDate && tx.timestamp > filters.endDate) {
        return false;
      }

      return true;
    });
  }, [filters]);

  // 导出记录
  const handleExport = () => {
    // TODO: 实现导出功能
    message.success(t('pages.fundFlow.messages.exportStarted', '开始导出记录...'));
  };

  // 查看详情
  const handleViewDetail = (record: TransactionRecord) => {
    setSelectedTransaction(record);
  };

  return (
    <PageContainer
      className={styles.page}
      header={{
        title: t('pages.fundFlow.title', '资金流水记录'),
        subTitle: t('pages.fundFlow.subtitle', '查看所有资金流转记录和交易详情'),
      }}
    >
      {/* 统计卡片 */}
      <StatsCards stats={stats} t={t} />

      {/* 筛选区域 */}
      <FilterSection
        filters={filters}
        onFiltersChange={setFilters}
        resultCount={filteredTransactions.length}
        onExport={handleExport}
        t={t}
      />

      {/* 交易记录表格 */}
      <div className={styles.tableWrapper}>
        <TransactionTable
          transactions={filteredTransactions}
          onViewDetail={handleViewDetail}
          t={t}
        />
      </div>

      {/* 交易详情弹窗 */}
      <TransactionDetailModal
        open={!!selectedTransaction}
        transaction={selectedTransaction}
        onClose={() => setSelectedTransaction(null)}
        t={t}
      />
    </PageContainer>
  );
};

export default FundFlowPage;

