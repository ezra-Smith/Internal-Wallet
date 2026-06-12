import React, { useState, useMemo } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App } from 'antd';
import { createStyles } from 'antd-style';
import { useRbac } from '@/hooks/useRbac';

import StatsCards from './components/StatsCards';
import FilterBar from './components/FilterBar';
import DeviceList from './components/DeviceList';
import DeviceSettingsModal from './components/DeviceSettingsModal';
import type { Web3WalletFilters, DeviceInfo, WalletAddress } from './types';
import { MOCK_STATS, MOCK_DEVICES, PERM } from './constants';

const useStyles = createStyles(() => ({
  page: {
    background: '#f5f7fa',
    minHeight: '100vh',
  },
}));

const Web3WalletsPage: React.FC = () => {
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const { styles } = useStyles();

  const t = (id: string, defaultMessage: string, values?: Record<string, any>) =>
    intl.formatMessage({ id, defaultMessage }, values);

  // State
  const [filters, setFilters] = useState<Web3WalletFilters>({
    keyword: '',
    addressStatus: 'all',
    twoFAStatus: 'all',
    biometricStatus: 'all',
  });
  const [settingsDevice, setSettingsDevice] = useState<DeviceInfo | null>(null);

  // 使用模拟数据
  const [devices] = useState<DeviceInfo[]>(MOCK_DEVICES);
  const stats = MOCK_STATS;

  // 筛选设备
  const filteredDevices = useMemo(() => {
    return devices.filter((device) => {
      // 关键词筛选
      if (filters.keyword) {
        const keyword = filters.keyword.toLowerCase();
        const matchDeviceId = device.deviceId.toLowerCase().includes(keyword);
        const matchAddress = device.addresses.some((addr) =>
          addr.address.toLowerCase().includes(keyword),
        );
        if (!matchDeviceId && !matchAddress) {
          return false;
        }
      }

      // 2FA筛选
      if (filters.twoFAStatus && filters.twoFAStatus !== 'all') {
        if (filters.twoFAStatus === 'enabled' && !device.twoFAEnabled) return false;
        if (filters.twoFAStatus === 'disabled' && device.twoFAEnabled) return false;
      }

      // 生物识别筛选
      if (filters.biometricStatus && filters.biometricStatus !== 'all') {
        if (filters.biometricStatus === 'enabled' && !device.biometricEnabled) return false;
        if (filters.biometricStatus === 'disabled' && device.biometricEnabled) return false;
      }

      // 地址状态筛选
      if (filters.addressStatus && filters.addressStatus !== 'all') {
        const hasMatchingAddress = device.addresses.some(
          (addr) => addr.status === filters.addressStatus,
        );
        if (!hasMatchingAddress) return false;
      }

      return true;
    });
  }, [devices, filters]);

  // 计算记录数
  const recordCount = useMemo(() => {
    return filteredDevices.reduce((acc, device) => acc + device.addresses.length, 0);
  }, [filteredDevices]);

  // 处理黑名单切换
  const handleBlacklistToggle = (address: WalletAddress, isBlacklist: boolean) => {
    if (isBlacklist) {
      modal.confirm({
        title: t('pages.web3Wallets.modal.addBlacklist.title', '标记黑名单'),
        content: t(
          'pages.web3Wallets.modal.addBlacklist.content',
          '确定要将此地址标记为黑名单吗？标记后该地址将无法进行任何交易。',
        ),
        okText: t('common.confirm', '确定'),
        cancelText: t('common.cancel', '取消'),
        okButtonProps: { danger: true },
        onOk: () => {
          // TODO: 调用API
          message.success(t('pages.web3Wallets.messages.blacklistAdded', '已添加到黑名单'));
        },
      });
    } else {
      modal.confirm({
        title: t('pages.web3Wallets.modal.removeBlacklist.title', '移出黑名单'),
        content: t(
          'pages.web3Wallets.modal.removeBlacklist.content',
          '确定要将此地址移出黑名单吗？移出后该地址将恢复正常交易功能。',
        ),
        okText: t('common.confirm', '确定'),
        cancelText: t('common.cancel', '取消'),
        onOk: () => {
          // TODO: 调用API
          message.success(t('pages.web3Wallets.messages.blacklistRemoved', '已从黑名单移出'));
        },
      });
    }
  };

  // 处理设备设置
  const handleSettings = (device: DeviceInfo) => {
    setSettingsDevice(device);
  };

  return (
    <PageContainer
      className={styles.page}
      header={{
        title: t('pages.web3Wallets.title', 'Web3钱包'),
        subTitle: t('pages.web3Wallets.subtitle', '管理区块链钱包用户，按设备ID归类'),
      }}
    >
      {/* 统计卡片 */}
      <StatsCards stats={stats} t={t} />

      {/* 筛选栏 */}
      <FilterBar
        filters={filters}
        onFiltersChange={setFilters}
        deviceCount={filteredDevices.length}
        recordCount={recordCount}
        t={t}
      />

      {/* 设备列表 */}
      <DeviceList
        devices={filteredDevices}
        onBlacklistToggle={handleBlacklistToggle}
        onSettings={handleSettings}
        t={t}
      />

      {/* 设置弹窗 */}
      <DeviceSettingsModal
        open={!!settingsDevice}
        device={settingsDevice}
        onClose={() => setSettingsDevice(null)}
        t={t}
      />
    </PageContainer>
  );
};

export default Web3WalletsPage;

