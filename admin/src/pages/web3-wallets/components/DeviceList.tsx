import React, { useState } from 'react';
import { Button, Tag, Tooltip, message } from 'antd';
import {
  MobileOutlined,
  CopyOutlined,
  SafetyCertificateOutlined,
  ScanOutlined,
  SettingOutlined,
  UpOutlined,
  DownOutlined,
  WarningOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { DeviceInfo, WalletAddress, ChainType } from '../types';
import { CHAIN_CONFIG } from '../constants';

const useStyles = createStyles(() => ({
  container: {
    display: 'flex',
    flexDirection: 'column',
    gap: 16,
  },
  deviceCard: {
    borderRadius: 12,
    background: '#fff',
    border: '1px solid #f0f0f0',
    overflow: 'hidden',
    transition: 'all 0.2s ease',
  },
  deviceCardExpanded: {
    boxShadow: '0 4px 16px rgba(0, 0, 0, 0.08)',
  },
  deviceHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '16px 20px',
    background: '#fafbfc',
    borderBottom: '1px solid #f0f0f0',
    cursor: 'pointer',
    '&:hover': {
      background: '#f5f6f8',
    },
  },
  deviceHeaderLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 14,
  },
  deviceIcon: {
    width: 40,
    height: 40,
    borderRadius: 10,
    background: 'linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 18,
    color: '#3b82f6',
  },
  deviceInfo: {
    display: 'flex',
    flexDirection: 'column',
  },
  deviceIdRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  deviceIdLabel: {
    fontSize: 13,
    color: '#6b7280',
  },
  deviceId: {
    fontSize: 14,
    fontWeight: 600,
    color: '#3b82f6',
  },
  copyBtn: {
    fontSize: 12,
    color: '#9ca3af',
    cursor: 'pointer',
    '&:hover': {
      color: '#3b82f6',
    },
  },
  deviceMeta: {
    fontSize: 12,
    color: '#9ca3af',
    marginTop: 2,
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  blacklistBadge: {
    color: '#ef4444',
    fontWeight: 500,
  },
  deviceHeaderRight: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  },
  statusTag: {
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    fontSize: 12,
    padding: '4px 10px',
    borderRadius: 6,
    border: 'none',
  },
  statusTagEnabled: {
    background: '#d1fae5',
    color: '#10b981',
  },
  statusTagDisabled: {
    background: '#f3f4f6',
    color: '#9ca3af',
  },
  actionBtn: {
    fontSize: 13,
    color: '#6b7280',
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    '&:hover': {
      color: '#3b82f6',
    },
  },
  expandBtn: {
    fontSize: 13,
    color: '#3b82f6',
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    padding: '4px 12px',
    borderRadius: 6,
    border: '1px solid #3b82f6',
    background: '#fff',
    cursor: 'pointer',
    '&:hover': {
      background: '#eff6ff',
    },
  },
  addressList: {
    padding: 0,
  },
  addressItem: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '14px 20px',
    borderBottom: '1px solid #f5f5f5',
    '&:last-child': {
      borderBottom: 'none',
    },
    '&:hover': {
      background: '#fafbfc',
    },
  },
  addressItemBlacklist: {
    background: 'linear-gradient(135deg, #fef2f2 0%, #fff 100%)',
  },
  addressLeft: {
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
  },
  addressHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  chainTag: {
    fontSize: 11,
    fontWeight: 600,
    padding: '2px 8px',
    borderRadius: 4,
    border: 'none',
  },
  blacklistTag: {
    fontSize: 11,
    fontWeight: 500,
    padding: '2px 8px',
    borderRadius: 4,
    background: '#fef2f2',
    color: '#ef4444',
    border: '1px solid #fecaca',
    display: 'flex',
    alignItems: 'center',
    gap: 4,
  },
  disabledTag: {
    fontSize: 11,
    fontWeight: 500,
    padding: '2px 6px',
    borderRadius: 4,
    background: '#f3f4f6',
    color: '#9ca3af',
  },
  addressRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  address: {
    fontSize: 14,
    color: '#1f2937',
    fontFamily: 'monospace',
  },
  sourceNote: {
    fontSize: 12,
    color: '#9ca3af',
    marginTop: 2,
  },
  sourceHighlight: {
    color: '#f59e0b',
  },
  addressRight: {
    flexShrink: 0,
  },
  blacklistBtn: {
    fontSize: 12,
    color: '#ef4444',
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    padding: '6px 12px',
    borderRadius: 6,
    border: '1px solid #fecaca',
    background: '#fff',
    cursor: 'pointer',
    '&:hover': {
      background: '#fef2f2',
    },
  },
  removeBlacklistBtn: {
    fontSize: 12,
    color: '#10b981',
    display: 'flex',
    alignItems: 'center',
    gap: 4,
    padding: '6px 12px',
    borderRadius: 6,
    border: '1px solid #a7f3d0',
    background: '#fff',
    cursor: 'pointer',
    '&:hover': {
      background: '#ecfdf5',
    },
  },
}));

interface DeviceListProps {
  devices: DeviceInfo[];
  onBlacklistToggle?: (address: WalletAddress, isBlacklist: boolean) => void;
  onSettings?: (device: DeviceInfo) => void;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const DeviceList: React.FC<DeviceListProps> = ({
  devices,
  onBlacklistToggle,
  onSettings,
  t,
}) => {
  const { styles } = useStyles();
  const [expandedDevices, setExpandedDevices] = useState<Set<string>>(new Set());

  const toggleExpand = (deviceId: string) => {
    setExpandedDevices((prev) => {
      const next = new Set(prev);
      if (next.has(deviceId)) {
        next.delete(deviceId);
      } else {
        next.add(deviceId);
      }
      return next;
    });
  };

  const copyAddress = (address: string) => {
    navigator.clipboard.writeText(address);
    message.success(t('common.copied', '已复制'));
  };

  const getChainConfig = (chain: ChainType) => {
    return CHAIN_CONFIG[chain] || CHAIN_CONFIG.ethereum;
  };

  return (
    <div className={styles.container}>
      {devices.map((device) => {
        const isExpanded = expandedDevices.has(device.id);
        const chainConfig = getChainConfig;

        return (
          <div
            key={device.id}
            className={`${styles.deviceCard} ${isExpanded ? styles.deviceCardExpanded : ''}`}
          >
            {/* Device Header */}
            <div className={styles.deviceHeader} onClick={() => toggleExpand(device.id)}>
              <div className={styles.deviceHeaderLeft}>
                <div className={styles.deviceIcon}>
                  <MobileOutlined />
                </div>
                <div className={styles.deviceInfo}>
                  <div className={styles.deviceIdRow}>
                    <span className={styles.deviceIdLabel}>
                      {t('pages.web3Wallets.device.id', '设备 ID')}:
                    </span>
                    <span className={styles.deviceId}>{device.deviceId}</span>
                    <CopyOutlined
                      className={styles.copyBtn}
                      onClick={(e) => {
                        e.stopPropagation();
                        copyAddress(device.deviceId);
                      }}
                    />
                  </div>
                  <div className={styles.deviceMeta}>
                    <span>
                      {device.addressCount}{' '}
                      {t('pages.web3Wallets.device.addresses', '个钱包地址')}
                    </span>
                    {device.blacklistCount > 0 && (
                      <span className={styles.blacklistBadge}>
                        {device.blacklistCount}{' '}
                        {t('pages.web3Wallets.device.blacklistAddresses', '个黑名单地址')}
                      </span>
                    )}
                    <span>
                      {t('pages.web3Wallets.device.registeredAt', '注册时间')}: {device.registeredAt}
                    </span>
                  </div>
                </div>
              </div>

              <div className={styles.deviceHeaderRight}>
                {/* 2FA Status */}
                <Tag
                  className={`${styles.statusTag} ${device.twoFAEnabled ? styles.statusTagEnabled : styles.statusTagDisabled}`}
                >
                  <SafetyCertificateOutlined />
                  2FA
                </Tag>

                {/* Biometric Status */}
                <Tag
                  className={`${styles.statusTag} ${device.biometricEnabled ? styles.statusTagEnabled : styles.statusTagDisabled}`}
                >
                  <ScanOutlined />
                  {t('pages.web3Wallets.biometric.label', '生物')}
                </Tag>

                {/* Settings Button */}
                <Button
                  type="link"
                  className={styles.actionBtn}
                  onClick={(e) => {
                    e.stopPropagation();
                    onSettings?.(device);
                  }}
                >
                  <SettingOutlined />
                  {t('pages.web3Wallets.actions.settings', '设置')}
                </Button>

                {/* Expand/Collapse Button */}
                <button
                  type="button"
                  className={styles.expandBtn}
                  onClick={(e) => {
                    e.stopPropagation();
                    toggleExpand(device.id);
                  }}
                >
                  {isExpanded ? <UpOutlined /> : <DownOutlined />}
                  {isExpanded
                    ? t('pages.web3Wallets.actions.collapse', '收起')
                    : t('pages.web3Wallets.actions.expand', '展开')}
                </button>
              </div>
            </div>

            {/* Address List (expanded) */}
            {isExpanded && (
              <div className={styles.addressList}>
                {device.addresses.map((addr) => {
                  const chain = getChainConfig(addr.chain);
                  const isBlacklisted = addr.status === 'blacklisted';
                  const isDisabled = addr.status === 'disabled';

                  return (
                    <div
                      key={addr.id}
                      className={`${styles.addressItem} ${isBlacklisted ? styles.addressItemBlacklist : ''}`}
                    >
                      <div className={styles.addressLeft}>
                        <div className={styles.addressHeader}>
                          <Tag
                            className={styles.chainTag}
                            style={{ background: chain.color, color: '#fff' }}
                          >
                            {chain.name}
                          </Tag>
                          {isBlacklisted && (
                            <span className={styles.blacklistTag}>
                              <WarningOutlined />
                              {t('pages.web3Wallets.status.blacklisted', '黑名单')}
                            </span>
                          )}
                          {isDisabled && (
                            <span className={styles.disabledTag}>
                              {t('pages.web3Wallets.status.disabled', '未启用')}
                            </span>
                          )}
                        </div>
                        <div className={styles.addressRow}>
                          <span className={styles.address}>{addr.address}</span>
                          <CopyOutlined
                            className={styles.copyBtn}
                            onClick={() => copyAddress(addr.address)}
                          />
                        </div>
                        {addr.source && (
                          <div className={styles.sourceNote}>
                            {t('pages.web3Wallets.address.source', '来源')}:{' '}
                            <span className={styles.sourceHighlight}>{addr.source}</span>
                          </div>
                        )}
                      </div>
                      <div className={styles.addressRight}>
                        {isBlacklisted ? (
                          <button
                            type="button"
                            className={styles.removeBlacklistBtn}
                            onClick={() => onBlacklistToggle?.(addr, false)}
                          >
                            <CheckCircleOutlined />
                            {t('pages.web3Wallets.actions.removeBlacklist', '移出黑名单')}
                          </button>
                        ) : (
                          <button
                            type="button"
                            className={styles.blacklistBtn}
                            onClick={() => onBlacklistToggle?.(addr, true)}
                          >
                            <WarningOutlined />
                            {t('pages.web3Wallets.actions.addBlacklist', '标记黑名单')}
                          </button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
};

export default DeviceList;
