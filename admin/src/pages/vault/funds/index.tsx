import { PageContainer } from '@ant-design/pro-components';
import { FundsOverview } from '@/components';
import { createStyles } from 'antd-style';
import { useIntl } from '@umijs/max';
import React, { useState, useCallback, useEffect, useMemo } from 'react';
import {
  WalletOutlined,
  SyncOutlined,
  WarningOutlined,
  CheckCircleOutlined,
  PlusOutlined,
  SettingOutlined,
  RightOutlined,
  DownOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import {
  Button,
  Tag,
  Typography,
  Space,
  App,
  Pagination,
  Modal,
  Form,
  Input,
  Select,
  Alert,
  InputNumber,
  Spin,
} from 'antd';
import { SaveOutlined } from '@ant-design/icons';
import {
  adminGetVaultOverview,
  adminListVaultAddresses,
  adminGetVaultAddressBalances,
  adminAddVaultAddress,
  adminUpdateVaultAddressStatus,
  adminDeleteVaultAddress,
  adminUpdateVaultThresholds,
  adminSyncVault,
} from '@/api/generated/vault';
import type {
  AdminVaultNetworkOverviewItem,
  AdminVaultAddressItem,
  AdminVaultAddressBalanceItem,
  AdminAddVaultAddressRequest,
  AdminVaultBalanceOverviewItem,
} from '@/api/generated/schemas';
import { useRbac } from '@/hooks/useRbac';

const { Text } = Typography;

const PERM = {
  overview: 'GetVaultOverview',
  list: 'ListVaultAddresses',
  create: 'AddVaultAddress',
  updateStatus: 'UpdateVaultAddressStatus',
  remove: 'DeleteVaultAddress',
  balances: 'GetVaultAddressBalances',
  thresholds: 'UpdateVaultThresholds',
  sync: 'SyncVault',
};

// Wallet type options
const walletTypeOptions = [
  { value: 'active', label: '活跃钱包' },
  { value: 'hot', label: '热钱包' },
  { value: 'cold', label: '冷钱包' },
];

// Type for UI display - transformed from API data
interface UIWallet {
  id: string;
  type: string;
  label: string;
  address: string;
  addedAt: string;
  isActive: boolean;
  currencies: UICurrency[];
  networkId: string;
}

interface UICurrency {
  symbol: string;
  balance: number;
  balanceUsdt: number;
  threshold?: number;
  thresholdCritical?: number;
  status: 'normal' | 'warning' | 'critical';
  contractAddress?: string;
}

interface UINetwork {
  id: string;
  networkId: string;
  chainId: string;
  name: string;
  currencyCount: number;
  totalValueUsdt: number;
  hasAlert: boolean;
  alertCount: number;
  color: string;
  wallets: UIWallet[];
  inactiveWallets: { id: string; address: string; addedAt: string }[];
  lastSyncAt?: string;
  syncStatus?: string;
  // Threshold data from overview
  thresholds: Map<string, { low: string; critical: string }>;
}

// Type for editing state
interface EditingCurrency {
  networkId: string;
  walletId: string;
  symbol: string;
  low: number | null;
  critical: number | null;
}

const useStyles = createStyles(({ token }) => ({
  page: {
    position: 'relative',
  },
  // Management Section
  managementSection: {
    borderRadius: 16,
    background: '#fff',
    border: '1px solid #f0f0f0',
    boxShadow: '0 4px 20px rgba(0, 0, 0, 0.04)',
    overflow: 'hidden',
  },
  managementHeader: {
    padding: '24px 32px',
    borderBottom: '1px solid #f0f0f0',
  },
  managementTitle: {
    fontSize: 20,
    fontWeight: 700,
    color: '#1f2937',
    marginBottom: 4,
  },
  managementSubtitle: {
    fontSize: 14,
    color: '#9ca3af',
  },
  managementStats: {
    display: 'flex',
    alignItems: 'center',
    gap: 32,
    marginTop: 20,
    padding: 20,
    borderRadius: 12,
    background: 'linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%)',
  },
  managementStatItem: {
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
    paddingRight: 32,
    borderRight: '1px solid rgba(16, 185, 129, 0.2)',
    '&:last-child': {
      borderRight: 'none',
      paddingRight: 0,
    },
  },
  managementStatLabel: {
    fontSize: 13,
    color: '#059669',
  },
  managementStatValue: {
    fontSize: 24,
    fontWeight: 700,
    fontFamily: '"DIN Alternate", "Bebas Neue", "Teko", sans-serif',
    color: '#10b981',
  },
  managementActions: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    marginTop: 20,
  },
  managementMeta: {
    marginTop: 10,
    display: 'flex',
    alignItems: 'center',
    gap: 16,
    flexWrap: 'wrap',
    color: '#6b7280',
    fontSize: 12,
  },
  managementMetaItem: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  },
  // Table Section
  tableSection: {
    padding: '0 32px 32px',
  },
  tableHeader: {
    display: 'grid',
    gridTemplateColumns: '180px 200px 120px 100px 100px',
    padding: '16px 20px',
    background: '#f9fafb',
    borderRadius: '12px 12px 0 0',
    borderBottom: '1px solid #e5e7eb',
    gap: 16,
  },
  tableHeaderCell: {
    fontSize: 13,
    fontWeight: 600,
    color: '#6b7280',
  },
  paginationBar: {
    display: 'flex',
    justifyContent: 'flex-end',
    padding: '12px 0 16px',
  },
  networkGroup: {
    marginBottom: 8,
    borderRadius: 12,
    border: '1px solid #e5e7eb',
    overflow: 'hidden',
    '&:last-child': {
      marginBottom: 0,
    },
  },
  networkGroupHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '12px 20px',
    background: '#fafafa',
    borderBottom: '1px solid #e5e7eb',
  },
  networkGroupLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  },
  networkGroupTag: {
    borderRadius: 6,
    fontWeight: 600,
    fontSize: 12,
    padding: '2px 10px',
    border: 'none',
  },
  networkGroupCount: {
    fontSize: 13,
    color: '#9ca3af',
  },
  networkGroupTotal: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 13,
    color: '#10b981',
    fontWeight: 500,
  },
  addAddressBtn: {
    borderRadius: 8,
    fontWeight: 500,
  },
  walletSection: {
    borderBottom: '1px solid #f0f0f0',
    '&:last-child': {
      borderBottom: 'none',
    },
  },
  walletHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '14px 20px',
    background: '#fff',
    cursor: 'pointer',
    '&:hover': {
      background: '#fafafa',
    },
  },
  walletLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  },
  walletIcon: {
    width: 28,
    height: 28,
    borderRadius: 8,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 14,
  },
  walletIconHot: {
    background: '#eff6ff',
    color: '#3b82f6',
  },
  walletIconCold: {
    background: '#f0fdf4',
    color: '#22c55e',
  },
  walletLabel: {
    fontSize: 14,
    fontWeight: 500,
    color: '#4b5563',
  },
  walletAddress: {
    fontSize: 13,
    color: '#9ca3af',
    fontFamily: 'monospace',
  },
  walletDate: {
    fontSize: 12,
    color: '#9ca3af',
    marginLeft: 16,
  },
  walletRight: {
    display: 'flex',
    alignItems: 'center',
    gap: 16,
  },
  walletActiveTag: {
    background: '#dcfce7',
    color: '#16a34a',
    border: 'none',
    fontWeight: 500,
    borderRadius: 6,
  },
  currencyRow: {
    display: 'grid',
    gridTemplateColumns: '180px 200px 120px 100px 100px',
    padding: '14px 20px',
    alignItems: 'center',
    gap: 16,
    borderTop: '1px solid #f3f4f6',
    background: '#fafafa',
    '&:hover': {
      background: '#f3f4f6',
    },
  },
  currencyCell: {
    fontSize: 14,
  },
  currencyTag: {
    borderRadius: 6,
    fontWeight: 500,
    fontSize: 12,
    padding: '2px 10px',
  },
  currencyBalance: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
  },
  currencyBalanceValue: {
    fontSize: 15,
    fontWeight: 600,
    color: '#1f2937',
  },
  currencyBalanceValueWarning: {
    color: '#ef4444',
  },
  currencyBalanceUsdt: {
    fontSize: 12,
    color: '#9ca3af',
  },
  thresholdValue: {
    fontSize: 14,
    color: '#6b7280',
  },
  statusTag: {
    borderRadius: 6,
    border: 'none',
    fontWeight: 500,
    fontSize: 12,
  },
  statusWarning: {
    background: '#fef3c7',
    color: '#d97706',
  },
  statusCritical: {
    background: '#fee2e2',
    color: '#dc2626',
  },
  statusNormal: {
    background: '#dcfce7',
    color: '#16a34a',
  },
  configBtn: {
    fontSize: 13,
    color: '#6b7280',
    '&:hover': {
      color: '#3b82f6',
    },
  },
  expandIcon: {
    transition: 'transform 0.2s ease',
  },
  expandIconRotated: {
    transform: 'rotate(90deg)',
  },
  coldWalletCurrencies: {
    padding: '0 20px 16px',
  },
  coldCurrencyItem: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '12px 16px',
    background: '#f9fafb',
    borderRadius: 8,
    marginBottom: 8,
    '&:last-child': {
      marginBottom: 0,
    },
  },
  coldCurrencyLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  },
  coldCurrencyTag: {
    borderRadius: 6,
    border: '1px solid #e5e7eb',
    background: '#fff',
    color: '#4b5563',
    fontWeight: 500,
    fontSize: 12,
    padding: '2px 10px',
  },
  coldCurrencyBalance: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
  },
  coldCurrencyValue: {
    fontSize: 15,
    fontWeight: 600,
    color: '#1f2937',
  },
  coldCurrencyUsdt: {
    fontSize: 12,
    color: '#22c55e',
  },
  // Inactive Wallets Section
  inactiveWalletsSection: {
    borderTop: '1px dashed #e5e7eb',
    background: '#fafafa',
  },
  inactiveWalletsHeader: {
    padding: '12px 20px',
    fontSize: 13,
    color: '#6b7280',
    cursor: 'pointer',
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    '&:hover': {
      background: '#f3f4f6',
    },
  },
  inactiveWalletsList: {
    borderTop: '1px solid #e5e7eb',
  },
  inactiveWalletItem: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '14px 20px 14px 48px',
    borderBottom: '1px solid #f0f0f0',
    background: '#fff',
    '&:last-child': {
      borderBottom: 'none',
    },
    '&:hover': {
      background: '#fafafa',
    },
  },
  inactiveWalletLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 16,
  },
  inactiveWalletIcon: {
    width: 28,
    height: 28,
    borderRadius: 8,
    background: '#f3f4f6',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 14,
    color: '#9ca3af',
  },
  inactiveWalletAddress: {
    fontSize: 14,
    fontFamily: 'monospace',
    color: '#4b5563',
  },
  inactiveWalletDate: {
    fontSize: 13,
    color: '#9ca3af',
  },
  inactiveWalletStatus: {
    fontSize: 13,
    color: '#9ca3af',
  },
  inactiveWalletRight: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  enableBtn: {
    color: '#16a34a',
    fontWeight: 500,
    '&:hover': {
      color: '#15803d !important',
    },
  },
  deleteBtn: {
    color: '#ef4444',
    fontWeight: 500,
    '&:hover': {
      color: '#dc2626 !important',
    },
  },
  // Editable cell styles
  editableInput: {
    width: 100,
    '& input': {
      textAlign: 'right',
    },
  },
  saveBtn: {
    color: '#3b82f6',
    fontWeight: 500,
    '&:hover': {
      color: '#2563eb !important',
    },
  },
  // Loading
  loadingContainer: {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    padding: 60,
  },
  emptyContainer: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    padding: 60,
    color: '#9ca3af',
  },
}));

// Network color mapping
const getNetworkColor = (network: string): string => {
  const upper = (network || '').toUpperCase();
  if (upper.includes('ETH')) return '#627EEA';
  if (upper.includes('BSC') || upper.includes('BNB')) return '#F0B90B';
  if (upper.includes('TRON') || upper.includes('TRX')) return '#FF0013';
  if (upper.includes('POL') || upper.includes('MATIC')) return '#8247E5';
  return '#6b7280';
};

// Get wallet type label
const getWalletTypeLabel = (type: string): string => {
  const map: Record<string, string> = {
    active: '活跃钱包',
    hot: '热钱包',
    cold: '冷钱包',
    deposit: '充值地址',
  };
  return map[type?.toLowerCase()] || type || '钱包';
};

// Truncate address for display
const truncateAddress = (address: string): string => {
  if (!address || address.length <= 20) return address || '';
  return `${address.substring(0, 10)}...${address.substring(address.length - 8)}`;
};

const VaultFundsPage: React.FC = () => {
  const intl = useIntl();
  const { styles, cx } = useStyles();
  const { message } = App.useApp();
  const { canRpc } = useRbac();

  // Permissions
  const canOverview = canRpc(PERM.overview);
  const canList = canRpc(PERM.list);
  const canCreate = canRpc(PERM.create);
  const canUpdateStatus = canRpc(PERM.updateStatus);
  const canRemove = canRpc(PERM.remove);
  const canBalances = canRpc(PERM.balances);
  const canThresholds = canRpc(PERM.thresholds);
  const canSync = canRpc(PERM.sync);

  // Loading states
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  // Data states
  const [networks, setNetworks] = useState<UINetwork[]>([]);
  const [totalAssetsUsdt, setTotalAssetsUsdt] = useState(0);
  const [networkCount, setNetworkCount] = useState(0);
  const [lastSyncAt, setLastSyncAt] = useState<Date | null>(null);
  const [addressPage, setAddressPage] = useState(1);
  const [addressPageSize, setAddressPageSize] = useState(20);
  const [addressTotal, setAddressTotal] = useState(0);

  // UI states
  const [expandedWallets, setExpandedWallets] = useState<string[]>([]);
  const [expandedInactive, setExpandedInactive] = useState<string[]>([]);
  const [addWalletModalVisible, setAddWalletModalVisible] = useState(false);
  const [addWalletLoading, setAddWalletLoading] = useState(false);
  const [addWalletForm] = Form.useForm();
  const [editingCurrency, setEditingCurrency] = useState<EditingCurrency | null>(null);
  const [selectedNetworkId, setSelectedNetworkId] = useState<string | null>(null);
  const [walletBalancesLoading, setWalletBalancesLoading] = useState<Set<string>>(new Set());
  const [walletBalancesCache, setWalletBalancesCache] = useState<
    Map<string, AdminVaultAddressBalanceItem[]>
  >(new Map());

  const t = useCallback(
    (id: string, defaultMessage: string, values?: Record<string, any>) =>
      intl.formatMessage({ id, defaultMessage }, values),
    [intl],
  );

  const formatNumber = useCallback((value: number | string | undefined) => {
    const num = typeof value === 'string' ? parseFloat(value) : (value ?? 0);
    return new Intl.NumberFormat('zh-CN', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(num);
  }, []);

  const formatDateTime = useCallback((date: Date | string | null | undefined) => {
    if (!date) return '-';
    const d = typeof date === 'string' ? new Date(date) : date;
    return new Intl.DateTimeFormat('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).format(d);
  }, []);

  // Network options for modal
  const networkOptions = useMemo(() => {
    return networks.map((n) => ({
      value: n.networkId,
      label: `${n.name} (Chain ${n.chainId})`,
      color: n.color,
    }));
  }, [networks]);

  // Load data from APIs
  const loadData = useCallback(async () => {
    if (!canOverview && !canList) {
      setLoading(false);
      return;
    }

    try {
      // 1. Get vault overview for network list and thresholds
      let overviewNetworks: AdminVaultNetworkOverviewItem[] = [];
      if (canOverview) {
        const overviewRes = await adminGetVaultOverview({ skipErrorHandler: true });
        if (overviewRes?.success && overviewRes.data) {
          overviewNetworks = overviewRes.data.networks || [];
          const summary = overviewRes.data.summary;
          setTotalAssetsUsdt(parseFloat(summary?.total_balance_usd || '0'));
          setNetworkCount(summary?.total_networks ?? overviewNetworks.length);
        }
      }

      // 2. Get all addresses
      let allAddresses: AdminVaultAddressItem[] = [];
      if (canList) {
        setAddressTotal(0);
        const addressRes = await adminListVaultAddresses(
          { page: addressPage, page_size: addressPageSize },
          { skipErrorHandler: true },
        );
        if (addressRes?.success && addressRes.data) {
          allAddresses = addressRes.data.addresses || [];
          const totalStr = addressRes.data.pagination?.total;
          const total = parseInt(totalStr || String(allAddresses.length), 10);
          setAddressTotal(Number.isFinite(total) ? total : 0);
        }
      } else {
        setAddressTotal(0);
      }

      // 3. Transform data to UI structure
      const networkMap = new Map<string, UINetwork>();

      // Initialize networks from overview
      for (const net of overviewNetworks) {
        const networkId = net.network_id || net.chain_id || '';
        if (!networkId) continue;

        // Build threshold map from overview balances
        const thresholds = new Map<string, { low: string; critical: string }>();
        for (const bal of net.balances || []) {
          if (bal.currency) {
            thresholds.set(bal.currency, {
              low: bal.threshold_low || '0',
              critical: bal.threshold_critical || '0',
            });
          }
        }

        // Count alerts
        const alertCount = (net.balances || []).filter((b) => {
          const status = (b.status || '').toLowerCase();
          return status === 'low' || status === 'critical';
        }).length;

        networkMap.set(networkId, {
          id: networkId,
          networkId,
          chainId: net.chain_id || '',
          name: net.network || 'Unknown',
          currencyCount: (net.balances || []).length,
          totalValueUsdt: parseFloat(net.total_balance_usd || '0'),
          hasAlert: alertCount > 0 || net.status === 'warning' || net.status === 'critical',
          alertCount,
          color: getNetworkColor(net.network || ''),
          wallets: [],
          inactiveWallets: [],
          lastSyncAt: net.last_sync_at,
          syncStatus: net.sync_status,
          thresholds,
        });
      }

      // Group addresses by network
      for (const addr of allAddresses) {
        const networkId = addr.network_id || '';
        if (!networkId) continue;

        // Ensure network exists
        if (!networkMap.has(networkId)) {
          networkMap.set(networkId, {
            id: networkId,
            networkId,
            chainId: '',
            name: addr.network || 'Unknown',
            currencyCount: 0,
            totalValueUsdt: 0,
            hasAlert: false,
            alertCount: 0,
            color: getNetworkColor(addr.network || ''),
            wallets: [],
            inactiveWallets: [],
            thresholds: new Map(),
          });
        }

        const network = networkMap.get(networkId)!;
        const status = (addr.status || '').toLowerCase();

        if (status === 'active') {
          // Active wallet
          network.wallets.push({
            id: addr.id || '',
            type: addr.address_type || 'active',
            label: addr.label || getWalletTypeLabel(addr.address_type || ''),
            address: addr.address || '',
            addedAt: addr.created_at
              ? new Date(addr.created_at).toLocaleDateString('zh-CN')
              : '-',
            isActive: addr.is_active_wallet || false,
            currencies: [], // Will be loaded on demand
            networkId,
          });
        } else {
          // Inactive wallet
          network.inactiveWallets.push({
            id: addr.id || '',
            address: addr.address || '',
            addedAt: addr.created_at
              ? new Date(addr.created_at).toLocaleDateString('zh-CN')
              : '-',
          });
        }
      }

      // Update latest sync time
      let latestSync: Date | null = null;
      for (const net of networkMap.values()) {
        if (net.lastSyncAt) {
          const syncDate = new Date(net.lastSyncAt);
          if (!latestSync || syncDate > latestSync) {
            latestSync = syncDate;
          }
        }
      }
      setLastSyncAt(latestSync);

      setNetworks(Array.from(networkMap.values()));
    } catch (e) {
      console.error('Failed to load vault data:', e);
      message.error(t('pages.vault.funds.messages.loadFailed', '加载数据失败'));
    } finally {
      setLoading(false);
    }
  }, [addressPage, addressPageSize, canOverview, canList, message, t]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const resetAddressUIState = useCallback(() => {
    setExpandedWallets([]);
    setExpandedInactive([]);
    setEditingCurrency(null);
    setWalletBalancesCache(new Map());
    setWalletBalancesLoading(new Set());
  }, []);

  const handleAddressPageChange = useCallback(
    (page: number) => {
      resetAddressUIState();
      setAddressPage(page);
    },
    [resetAddressUIState],
  );

  const handleAddressPageSizeChange = useCallback(
    (_page: number, size: number) => {
      resetAddressUIState();
      setAddressPage(1);
      setAddressPageSize(size);
    },
    [resetAddressUIState],
  );

  // Load balances for a wallet (on demand)
  const loadWalletBalances = useCallback(
    async (walletId: string) => {
      if (!canBalances || walletBalancesCache.has(walletId)) return;

      setWalletBalancesLoading((prev) => new Set(prev).add(walletId));
      try {
        const res = await adminGetVaultAddressBalances(
          { addressId: walletId },
          { skipErrorHandler: true },
        );
        if (res?.success && res.data) {
          setWalletBalancesCache((prev) => new Map(prev).set(walletId, res.data?.balances || []));

          // Update the wallet's currencies in the network data
          setNetworks((prevNetworks) =>
            prevNetworks.map((network) => ({
              ...network,
              wallets: network.wallets.map((wallet) => {
                if (wallet.id !== walletId) return wallet;

                const balances = res.data?.balances || [];
                const thresholds = network.thresholds;

                return {
                  ...wallet,
                  currencies: balances.map((b) => {
                    const th = thresholds.get(b.currency || '');
                    const balance = parseFloat(b.balance || '0');
                    const thresholdLow = parseFloat(th?.low || '0');
                    const thresholdCritical = parseFloat(th?.critical || '0');

                    let status: 'normal' | 'warning' | 'critical' = 'normal';
                    if (thresholdCritical > 0 && balance < thresholdCritical) {
                      status = 'critical';
                    } else if (thresholdLow > 0 && balance < thresholdLow) {
                      status = 'warning';
                    }

                    return {
                      symbol: b.currency || 'Unknown',
                      balance,
                      balanceUsdt: parseFloat(b.balance_usd || '0'),
                      threshold: thresholdLow || undefined,
                      thresholdCritical: thresholdCritical || undefined,
                      status,
                      contractAddress: b.contract_address,
                    };
                  }),
                };
              }),
            })),
          );
        }
      } catch (e) {
        console.error('Failed to load wallet balances:', e);
      } finally {
        setWalletBalancesLoading((prev) => {
          const next = new Set(prev);
          next.delete(walletId);
          return next;
        });
      }
    },
    [canBalances, walletBalancesCache],
  );

  // Handle refresh
  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    // Clear balance cache to force reload
    setWalletBalancesCache(new Map());
    try {
      await loadData();
      message.success(t('pages.vault.funds.messages.refreshSuccess', '余额刷新成功'));
    } catch (e) {
      message.error(t('pages.vault.funds.messages.refreshFailed', '余额刷新失败'));
    } finally {
      setRefreshing(false);
    }
  }, [loadData, message, t]);

  // Handle sync for a specific network
  const handleSyncNetwork = useCallback(
    async (chainId: string) => {
      if (!canSync) return;
      try {
        const res = await adminSyncVault({ chainId }, {}, { skipErrorHandler: true });
        if (res?.success) {
          message.success(t('pages.vault.funds.messages.syncStarted', '同步已开始'));
          // Reload data after a delay to get updated sync status
          setTimeout(() => void loadData(), 2000);
        } else {
          throw new Error(res?.message || 'Sync failed');
        }
      } catch (e: any) {
        message.error(e?.message || t('pages.vault.funds.messages.syncFailed', '同步失败'));
      }
    },
    [canSync, message, t, loadData],
  );

  // Toggle wallet expansion
  const toggleWallet = useCallback(
    (walletId: string) => {
      setExpandedWallets((prev) => {
        const isExpanding = !prev.includes(walletId);
        if (isExpanding) {
          // Load balances when expanding
          void loadWalletBalances(walletId);
          return [...prev, walletId];
        }
        return prev.filter((id) => id !== walletId);
      });
    },
    [loadWalletBalances],
  );

  const toggleInactive = useCallback((networkId: string) => {
    setExpandedInactive((prev) =>
      prev.includes(networkId) ? prev.filter((id) => id !== networkId) : [...prev, networkId],
    );
  }, []);

  // Handle add wallet modal
  const handleOpenAddWalletModal = useCallback(
    (networkId?: string) => {
      addWalletForm.resetFields();
      if (networkId) {
        addWalletForm.setFieldsValue({ network_id: networkId });
        setSelectedNetworkId(networkId);
      } else {
        setSelectedNetworkId(null);
      }
      setAddWalletModalVisible(true);
    },
    [addWalletForm],
  );

  const handleCloseAddWalletModal = useCallback(() => {
    setAddWalletModalVisible(false);
    addWalletForm.resetFields();
    setSelectedNetworkId(null);
  }, [addWalletForm]);

  const handleAddWallet = useCallback(async () => {
    if (!canCreate) return;
    try {
      const values = await addWalletForm.validateFields();
      setAddWalletLoading(true);

      const payload: AdminAddVaultAddressRequest = {
        network_id: values.network_id,
        address: values.address?.trim(),
        address_type: values.address_type || 'active',
        label: values.label?.trim() || undefined,
      };

      const res = await adminAddVaultAddress(payload, { skipErrorHandler: true });
      if (!res?.success) {
        throw new Error(res?.message || 'Add failed');
      }

      message.success(t('pages.vault.funds.messages.addWalletSuccess', '钱包地址添加成功'));
      handleCloseAddWalletModal();
      void loadData();
    } catch (e: any) {
      if (e?.errorFields) return;
      message.error(
        e?.message || t('pages.vault.funds.messages.addWalletFailed', '钱包地址添加失败'),
      );
    } finally {
      setAddWalletLoading(false);
    }
  }, [canCreate, addWalletForm, message, t, handleCloseAddWalletModal, loadData]);

  // Handle enable/disable wallet
  const handleUpdateWalletStatus = useCallback(
    async (walletId: string, newStatus: string, setAsActive?: boolean) => {
      if (!canUpdateStatus) return;
      try {
        const res = await adminUpdateVaultAddressStatus(
          { id: walletId },
          { status: newStatus, set_as_active_wallet: setAsActive },
          { skipErrorHandler: true },
        );
        if (!res?.success) {
          throw new Error(res?.message || 'Update failed');
        }
        message.success(
          newStatus === 'active'
            ? t('pages.vault.funds.messages.enableSuccess', '启用成功')
            : t('pages.vault.funds.messages.disableSuccess', '禁用成功'),
        );
        void loadData();
      } catch (e: any) {
        message.error(e?.message || t('pages.vault.funds.messages.updateFailed', '更新失败'));
      }
    },
    [canUpdateStatus, message, t, loadData],
  );

  // Handle delete wallet
  const handleDeleteWallet = useCallback(
    async (walletId: string) => {
      if (!canRemove) return;
      Modal.confirm({
        title: t('pages.vault.funds.delete.title', '确认删除'),
        content: t('pages.vault.funds.delete.content', '确定要删除此钱包地址吗？'),
        okButtonProps: { danger: true },
        onOk: async () => {
          try {
            const res = await adminDeleteVaultAddress({ id: walletId }, { skipErrorHandler: true });
            if (!res?.success) {
              throw new Error(res?.message || 'Delete failed');
            }
            message.success(t('pages.vault.funds.messages.deleteSuccess', '删除成功'));
            void loadData();
          } catch (e: any) {
            message.error(e?.message || t('pages.vault.funds.messages.deleteFailed', '删除失败'));
          }
        },
      });
    },
    [canRemove, message, t, loadData],
  );

  // Handle threshold config
  const handleStartEditCurrency = useCallback(
    (networkId: string, walletId: string, currency: UICurrency) => {
      setEditingCurrency({
        networkId,
        walletId,
        symbol: currency.symbol,
        low: currency.threshold ?? null,
        critical: currency.thresholdCritical ?? null,
      });
    },
    [],
  );

  const handleCancelEditCurrency = useCallback(() => {
    setEditingCurrency(null);
  }, []);

  const handleSaveEditCurrency = useCallback(async () => {
    if (!editingCurrency || !canThresholds) return;

    const network = networks.find((n) => n.networkId === editingCurrency.networkId);
    if (!network) return;

    try {
      const low = editingCurrency.low ?? 0;
      const critical = editingCurrency.critical ?? 0;
      if (low < critical) {
        message.error(
          t(
            'pages.vault.funds.messages.thresholdInvalid',
            '低阈值必须大于等于严重阈值',
          ),
        );
        return;
      }

      const res = await adminUpdateVaultThresholds(
        {
          chain_id: network.chainId,
          currency: editingCurrency.symbol,
          thresholds: {
            low: String(low),
            critical: String(critical),
          },
        },
        { skipErrorHandler: true },
      );

      if (!res?.success) {
        throw new Error(res?.message || 'Update failed');
      }

      message.success(t('pages.vault.funds.messages.configSaveSuccess', '配置保存成功'));
      setEditingCurrency(null);
      void loadData();
    } catch (e: any) {
      message.error(
        e?.message || t('pages.vault.funds.messages.configSaveFailed', '配置保存失败'),
      );
    }
  }, [editingCurrency, canThresholds, networks, message, t, loadData]);

  const isEditingCurrency = useCallback(
    (networkId: string, walletId: string, symbol: string) => {
      return (
        editingCurrency?.networkId === networkId &&
        editingCurrency?.walletId === walletId &&
        editingCurrency?.symbol === symbol
      );
    },
    [editingCurrency],
  );

  // Render loading state
  if (loading) {
    return (
      <PageContainer className={styles.page} title={false} header={{ title: '', breadcrumb: {} }}>
        <div className={styles.loadingContainer}>
          <Spin size="large" />
        </div>
      </PageContainer>
    );
  }

  return (
    <PageContainer className={styles.page} title={false} header={{ title: '', breadcrumb: {} }}>
      {/* 资金总览（共享组件） */}
      <FundsOverview t={t} onRefresh={handleRefresh} />

      {/* Management Section */}
      <div className={styles.managementSection}>
        <div className={styles.managementHeader}>
          <div className={styles.managementTitle}>
            {t('pages.vault.funds.management.title', 'Vault资金管理')}
          </div>
          <div className={styles.managementSubtitle}>
            {t('pages.vault.funds.management.subtitle', '监控和管理公司各币种资金池，按网络地址统计')}
          </div>
          <div className={styles.managementStats}>
            <div className={styles.managementStatItem}>
              <div className={styles.managementStatLabel}>
                {t('pages.vault.funds.management.totalAssets', '所有钱包总资产')}
              </div>
              <div className={styles.managementStatValue}>{formatNumber(totalAssetsUsdt)} USDT</div>
            </div>
            <div className={styles.managementStatItem}>
              <div className={styles.managementStatLabel}>
                {t('pages.vault.funds.management.coverage', '覆盖网络')}
              </div>
              <div className={styles.managementStatValue}>
                {networkCount} {t('pages.vault.funds.management.networks', '个网络')}
              </div>
            </div>
          </div>
          <div className={styles.managementActions}>
            <Button
              icon={<PlusOutlined />}
              disabled={!canCreate}
              onClick={() => handleOpenAddWalletModal()}
            >
              {t('pages.vault.funds.actions.addWallet', '添加钱包地址')}
            </Button>
            <Button
              type="primary"
              icon={<SyncOutlined />}
              onClick={handleRefresh}
              loading={refreshing}
            >
              {t('pages.vault.funds.actions.refreshBalance', '刷新余额')}
            </Button>
          </div>
          <div className={styles.managementMeta}>
            <div className={styles.managementMetaItem}>
              <Text type="secondary">
                {t('pages.vault.funds.sync.lastSyncAt', '最后同步')}：{formatDateTime(lastSyncAt)}
              </Text>
            </div>
          </div>
        </div>

        {/* Table */}
        <div className={styles.tableSection}>
          {/* Table Header */}
          <div className={styles.tableHeader}>
            <div className={styles.tableHeaderCell}>
              {t('pages.vault.funds.table.networkCurrency', '网络/币种')}
            </div>
            <div className={styles.tableHeaderCell}>
              {t('pages.vault.funds.table.walletBalance', '钱包地址/余额')}
            </div>
            <div className={styles.tableHeaderCell}>
              {t('pages.vault.funds.table.threshold', '预警阈值（低/严重）')}
            </div>
            <div className={styles.tableHeaderCell}>
              {t('pages.vault.funds.table.status', '状态')}
            </div>
            <div className={styles.tableHeaderCell}>
              {t('pages.vault.funds.table.actions', '操作')}
            </div>
          </div>

          {canList && (
            <div className={styles.paginationBar}>
              <Pagination
                current={addressPage}
                pageSize={addressPageSize}
                total={addressTotal}
                showSizeChanger
                showQuickJumper
                pageSizeOptions={[10, 20, 50, 100]}
                onChange={handleAddressPageChange}
                onShowSizeChange={handleAddressPageSizeChange}
                showTotal={(total) =>
                  t('pages.vault.funds.pagination.total', '共 {total} 条', { total })
                }
              />
            </div>
          )}

          {/* Empty state */}
          {networks.length === 0 && (
            <div className={styles.emptyContainer}>
              <WalletOutlined style={{ fontSize: 48, marginBottom: 16 }} />
              <Text type="secondary">
                {t('pages.vault.funds.empty', '暂无钱包数据，请添加钱包地址')}
              </Text>
            </div>
          )}

          {/* Network Groups */}
          {networks.map((network) => (
            <div key={network.id} className={styles.networkGroup}>
              {/* Network Header */}
              <div className={styles.networkGroupHeader}>
                <div className={styles.networkGroupLeft}>
                  <Tag
                    className={styles.networkGroupTag}
                    style={{ background: network.color, color: '#fff' }}
                  >
                    {network.name}
                  </Tag>
                  <span className={styles.networkGroupCount}>
                    （{network.currencyCount} 个币种）
                  </span>
                  <span className={styles.networkGroupTotal}>
                    <span>📈</span>
                    {t('pages.vault.funds.table.walletTotal', '钱包总额')}:{' '}
                    {formatNumber(network.totalValueUsdt)} USDT
                  </span>
                </div>
                <Space>
                  {canSync && (
                    <Button
                      size="small"
                      icon={<SyncOutlined />}
                      onClick={() => handleSyncNetwork(network.chainId)}
                    >
                      {t('pages.vault.funds.actions.sync', '同步')}
                    </Button>
                  )}
                  <Button
                    type="primary"
                    size="small"
                    icon={<PlusOutlined />}
                    className={styles.addAddressBtn}
                    disabled={!canCreate}
                    onClick={() => handleOpenAddWalletModal(network.networkId)}
                  >
                    {t('pages.vault.funds.actions.addAddress', '添加地址')}
                  </Button>
                </Space>
              </div>

              {/* Wallets */}
              {network.wallets.map((wallet) => (
                <div key={wallet.id} className={styles.walletSection}>
                  {/* Wallet Header */}
                  <div className={styles.walletHeader} onClick={() => toggleWallet(wallet.id)}>
                    <div className={styles.walletLeft}>
                      <div
                        className={cx(
                          styles.walletIcon,
                          wallet.type === 'cold' ? styles.walletIconCold : styles.walletIconHot,
                        )}
                      >
                        <WalletOutlined />
                      </div>
                      <span className={styles.walletLabel}>{wallet.label}</span>
                      <span className={styles.walletAddress}>{truncateAddress(wallet.address)}</span>
                      <span className={styles.walletDate}>
                        {t('pages.vault.funds.table.addedAt', '添加于')} {wallet.addedAt}
                      </span>
                    </div>
                    <div className={styles.walletRight}>
                      {wallet.isActive && (
                        <Tag className={styles.walletActiveTag}>
                          <CheckCircleOutlined style={{ marginRight: 4 }} />
                          {t('pages.vault.funds.table.inUse', '当前使用中')}
                        </Tag>
                      )}
                      {walletBalancesLoading.has(wallet.id) ? (
                        <Spin size="small" />
                      ) : (
                        <RightOutlined
                          className={cx(
                            styles.expandIcon,
                            expandedWallets.includes(wallet.id) && styles.expandIconRotated,
                          )}
                        />
                      )}
                    </div>
                  </div>

                  {/* Currency Rows - shown when expanded */}
                  {expandedWallets.includes(wallet.id) && (
                    <>
                      {wallet.currencies.length === 0 && !walletBalancesLoading.has(wallet.id) && (
                        <div className={styles.currencyRow}>
                          <Text type="secondary">
                            {t('pages.vault.funds.noCurrencies', '暂无余额数据')}
                          </Text>
                        </div>
                      )}
                      {wallet.currencies.map((currency) => {
                        const isEditing = isEditingCurrency(
                          network.networkId,
                          wallet.id,
                          currency.symbol,
                        );
                        return (
                          <div key={currency.symbol} className={styles.currencyRow}>
                            <div className={styles.currencyCell}>
                              <Tag className={styles.currencyTag}>{currency.symbol}</Tag>
                            </div>
                            <div className={styles.currencyBalance}>
                              <span
                                className={cx(
                                  styles.currencyBalanceValue,
                                  currency.status !== 'normal' && styles.currencyBalanceValueWarning,
                                )}
                              >
                                {formatNumber(currency.balance)}
                              </span>
                              <span className={styles.currencyBalanceUsdt}>
                                ≈ {formatNumber(currency.balanceUsdt)} USDT
                              </span>
                            </div>
                            <div className={styles.currencyCell}>
                              {isEditing ? (
                                <Space direction="vertical" size={6}>
                                  <InputNumber
                                    size="small"
                                    className={styles.editableInput}
                                    value={editingCurrency?.low}
                                    onChange={(value) =>
                                      setEditingCurrency((prev) =>
                                        prev ? { ...prev, low: value } : null,
                                      )
                                    }
                                    min={0}
                                    placeholder={t(
                                      'pages.vault.funds.threshold.lowPlaceholder',
                                      '低阈值',
                                    )}
                                    controls={false}
                                  />
                                  <InputNumber
                                    size="small"
                                    className={styles.editableInput}
                                    value={editingCurrency?.critical}
                                    onChange={(value) =>
                                      setEditingCurrency((prev) =>
                                        prev ? { ...prev, critical: value } : null,
                                      )
                                    }
                                    min={0}
                                    placeholder={t(
                                      'pages.vault.funds.threshold.criticalPlaceholder',
                                      '严重阈值',
                                    )}
                                    controls={false}
                                  />
                                </Space>
                              ) : (
                                <div className={styles.thresholdValue}>
                                  <div>
                                    {t('pages.vault.funds.threshold.low', '低')}：
                                    {currency.threshold !== undefined
                                      ? formatNumber(currency.threshold)
                                      : '-'}
                                  </div>
                                  <div>
                                    {t('pages.vault.funds.threshold.critical', '严重')}：
                                    {currency.thresholdCritical !== undefined
                                      ? formatNumber(currency.thresholdCritical)
                                      : '-'}
                                  </div>
                                </div>
                              )}
                            </div>
                            <div className={styles.currencyCell}>
                              <Tag
                                className={cx(
                                  styles.statusTag,
                                  currency.status === 'warning' && styles.statusWarning,
                                  currency.status === 'critical' && styles.statusCritical,
                                  currency.status === 'normal' && styles.statusNormal,
                                )}
                              >
                                {currency.status === 'warning' && (
                                  <>
                                    <WarningOutlined style={{ marginRight: 4 }} />
                                    {t('pages.vault.funds.status.low', '余额偏低')}
                                  </>
                                )}
                                {currency.status === 'critical' && (
                                  <>
                                    <WarningOutlined style={{ marginRight: 4 }} />
                                    {t('pages.vault.funds.status.insufficientBalance', '余额不足')}
                                  </>
                                )}
                                {currency.status === 'normal' &&
                                  t('pages.vault.funds.status.normal', '正常')}
                              </Tag>
                            </div>
                            <div className={styles.currencyCell}>
                              {isEditing ? (
                                <Space>
                                  <Button
                                    type="link"
                                    size="small"
                                    className={styles.saveBtn}
                                    icon={<SaveOutlined />}
                                    onClick={handleSaveEditCurrency}
                                  >
                                    {t('pages.vault.funds.actions.save', '保存')}
                                  </Button>
                                  <Button
                                    type="link"
                                    size="small"
                                    onClick={handleCancelEditCurrency}
                                  >
                                    {t('pages.vault.funds.actions.cancel', '取消')}
                                  </Button>
                                </Space>
                              ) : (
                                <Button
                                  type="link"
                                  size="small"
                                  icon={<SettingOutlined />}
                                  className={styles.configBtn}
                                  disabled={!canThresholds}
                                  onClick={() =>
                                    handleStartEditCurrency(network.networkId, wallet.id, currency)
                                  }
                                >
                                  {t('pages.vault.funds.actions.config', '配置')}
                                </Button>
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </>
                  )}
                </div>
              ))}

              {/* Inactive Wallets Section */}
              {network.inactiveWallets.length > 0 && (
                <div className={styles.inactiveWalletsSection}>
                  <div
                    className={styles.inactiveWalletsHeader}
                    onClick={() => toggleInactive(network.id)}
                  >
                    {expandedInactive.includes(network.id) ? (
                      <DownOutlined className={styles.expandIcon} />
                    ) : (
                      <RightOutlined className={styles.expandIcon} />
                    )}
                    {t('pages.vault.funds.table.inactiveWallets', '未启用钱包地址')} (
                    {network.inactiveWallets.length})
                  </div>
                  {expandedInactive.includes(network.id) && (
                    <div className={styles.inactiveWalletsList}>
                      {network.inactiveWallets.map((inactiveWallet) => (
                        <div key={inactiveWallet.id} className={styles.inactiveWalletItem}>
                          <div className={styles.inactiveWalletLeft}>
                            <div className={styles.inactiveWalletIcon}>
                              <WalletOutlined />
                            </div>
                            <span className={styles.inactiveWalletAddress}>
                              {truncateAddress(inactiveWallet.address)}
                            </span>
                            <span className={styles.inactiveWalletDate}>
                              {t('pages.vault.funds.table.addedAt', '添加于')} {inactiveWallet.addedAt}
                            </span>
                            <span className={styles.inactiveWalletStatus}>
                              {t('pages.vault.funds.status.inactive', '未启用')}
                            </span>
                          </div>
                          <div className={styles.inactiveWalletRight}>
                            <Button
                              type="link"
                              size="small"
                              icon={<CheckCircleOutlined />}
                              className={styles.enableBtn}
                              disabled={!canUpdateStatus}
                              onClick={(e) => {
                                e.stopPropagation();
                                void handleUpdateWalletStatus(inactiveWallet.id, 'active');
                              }}
                            >
                              {t('pages.vault.funds.actions.enable', '启用')}
                            </Button>
                            <Button
                              type="link"
                              size="small"
                              danger
                              icon={<DeleteOutlined />}
                              className={styles.deleteBtn}
                              disabled={!canRemove}
                              onClick={(e) => {
                                e.stopPropagation();
                                void handleDeleteWallet(inactiveWallet.id);
                              }}
                            >
                              {t('pages.vault.funds.actions.delete', '删除')}
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Add Wallet Modal */}
      <Modal
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 10,
                background: 'linear-gradient(135deg, #3b82f6 0%, #60a5fa 100%)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 18,
                color: '#fff',
              }}
            >
              <WalletOutlined />
            </div>
            <span style={{ fontSize: 18, fontWeight: 600 }}>
              {t('pages.vault.funds.modal.addWallet.title', '添加钱包地址')}
            </span>
          </div>
        }
        open={addWalletModalVisible}
        onCancel={handleCloseAddWalletModal}
        footer={[
          <Button key="cancel" onClick={handleCloseAddWalletModal}>
            {t('pages.vault.funds.modal.addWallet.cancel', '取消')}
          </Button>,
          <Button
            key="submit"
            type="primary"
            loading={addWalletLoading}
            disabled={!canCreate}
            onClick={handleAddWallet}
          >
            {t('pages.vault.funds.modal.addWallet.confirm', '确认添加')}
          </Button>,
        ]}
        width={560}
        destroyOnClose
      >
        <div style={{ color: '#6b7280', marginBottom: 24, marginTop: -8 }}>
          {t(
            'pages.vault.funds.modal.addWallet.subtitle',
            '选择网络并输入钱包地址，新添加的地址默认不启用',
          )}
        </div>

        <Form
          form={addWalletForm}
          layout="vertical"
          initialValues={{
            address_type: 'active',
            network_id: selectedNetworkId,
          }}
        >
          <Form.Item
            name="address_type"
            label={t('pages.vault.funds.modal.addWallet.walletType', '钱包类型')}
            rules={[
              {
                required: true,
                message: t(
                  'pages.vault.funds.modal.addWallet.walletTypeRequired',
                  '请选择钱包类型',
                ),
              },
            ]}
          >
            <Select
              options={walletTypeOptions}
              placeholder={t('pages.vault.funds.modal.addWallet.selectWalletType', '选择钱包类型')}
              size="large"
            />
          </Form.Item>

          <Form.Item
            name="network_id"
            label={t('pages.vault.funds.modal.addWallet.network', '选择网络')}
            rules={[
              {
                required: true,
                message: t('pages.vault.funds.modal.addWallet.networkRequired', '请选择网络'),
              },
            ]}
          >
            <Select
              options={networkOptions}
              placeholder={t('pages.vault.funds.modal.addWallet.selectNetwork', '选择网络')}
              size="large"
            />
          </Form.Item>

          <Form.Item
            name="address"
            label={t('pages.vault.funds.modal.addWallet.address', '钱包地址')}
            rules={[
              {
                required: true,
                message: t('pages.vault.funds.modal.addWallet.addressRequired', '请输入钱包地址'),
              },
              {
                pattern: /^(0x[a-fA-F0-9]{40}|T[a-zA-Z0-9]{33})$/,
                message: t(
                  'pages.vault.funds.modal.addWallet.addressInvalid',
                  '请输入有效的钱包地址',
                ),
              },
            ]}
          >
            <Input
              placeholder={t(
                'pages.vault.funds.modal.addWallet.addressPlaceholder',
                '输入完整的钱包地址...',
              )}
              size="large"
              style={{ background: '#f3f4f6', border: 'none' }}
            />
          </Form.Item>

          <Form.Item
            name="label"
            label={t('pages.vault.funds.modal.addWallet.label', '标签（可选）')}
          >
            <Input
              placeholder={t('pages.vault.funds.modal.addWallet.labelPlaceholder', '例如：主钱包')}
              size="large"
            />
          </Form.Item>

          <Alert
            type="info"
            showIcon
            message={t('pages.vault.funds.modal.addWallet.tipTitle', '提示')}
            description={t(
              'pages.vault.funds.modal.addWallet.tipContent',
              '新添加的钱包地址默认处于未启用状态，需要手动启用后才能使用。同一网络只能有一个活跃钱包地址。',
            )}
            style={{
              background: '#eff6ff',
              border: '1px solid #bfdbfe',
              borderRadius: 10,
            }}
          />
        </Form>
      </Modal>
    </PageContainer>
  );
};

export default VaultFundsPage;
