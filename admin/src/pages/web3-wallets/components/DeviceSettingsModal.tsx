import React from 'react';
import { Modal, Tag } from 'antd';
import {
  SettingOutlined,
  SafetyCertificateOutlined,
  KeyOutlined,
  ScanOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import { createStyles } from 'antd-style';
import type { DeviceInfo } from '../types';

const useStyles = createStyles(() => ({
  modalTitle: {
    display: 'flex',
    flexDirection: 'column',
  },
  titleRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 18,
    fontWeight: 600,
    color: '#1f2937',
  },
  titleIcon: {
    color: '#3b82f6',
  },
  subtitle: {
    fontSize: 14,
    color: '#6b7280',
    fontWeight: 400,
    marginTop: 4,
  },
  section: {
    borderRadius: 12,
    padding: '20px',
    background: '#f9fafb',
    border: '1px solid #f0f0f0',
    marginBottom: 20,
  },
  sectionTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 15,
    fontWeight: 600,
    color: '#1f2937',
    marginBottom: 16,
  },
  sectionIcon: {
    color: '#6b7280',
  },
  infoRow: {
    marginBottom: 16,
    '&:last-child': {
      marginBottom: 0,
    },
  },
  infoLabel: {
    fontSize: 13,
    color: '#6b7280',
    marginBottom: 6,
  },
  deviceIdTag: {
    fontSize: 14,
    fontWeight: 600,
    padding: '6px 12px',
    borderRadius: 6,
    background: '#eff6ff',
    color: '#3b82f6',
    border: '1px solid #bfdbfe',
    fontFamily: 'monospace',
  },
  infoValue: {
    fontSize: 16,
    fontWeight: 600,
    color: '#1f2937',
  },
  statusRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 16,
    flexWrap: 'wrap',
  },
  statusTag: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 14,
    padding: '8px 16px',
    borderRadius: 8,
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
  noteText: {
    fontSize: 13,
    color: '#6b7280',
    marginBottom: 16,
  },
  infoBox: {
    borderRadius: 10,
    background: '#eff6ff',
    border: '1px solid #bfdbfe',
    padding: '16px',
  },
  infoBoxTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 14,
    fontWeight: 600,
    color: '#3b82f6',
    marginBottom: 8,
  },
  infoBoxContent: {
    fontSize: 13,
    color: '#3b82f6',
    lineHeight: 1.6,
  },
}));

interface DeviceSettingsModalProps {
  open: boolean;
  device: DeviceInfo | null;
  onClose: () => void;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const DeviceSettingsModal: React.FC<DeviceSettingsModalProps> = ({
  open,
  device,
  onClose,
  t,
}) => {
  const { styles } = useStyles();

  if (!device) return null;

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      width={520}
      title={
        <div className={styles.modalTitle}>
          <div className={styles.titleRow}>
            <SettingOutlined className={styles.titleIcon} />
            {t('pages.web3Wallets.settings.title', 'Web3钱包设置')}
          </div>
          <div className={styles.subtitle}>
            {t('pages.web3Wallets.settings.subtitle', '查看用户安全设置状态')}
          </div>
        </div>
      }
      closeIcon={<span style={{ fontSize: 16, color: '#9ca3af' }}>×</span>}
    >
      {/* 设备信息 */}
      <div className={styles.section}>
        <div className={styles.infoRow}>
          <div className={styles.infoLabel}>
            {t('pages.web3Wallets.settings.deviceId', '设备ID')}
          </div>
          <Tag className={styles.deviceIdTag}>{device.deviceId}</Tag>
        </div>
        <div className={styles.infoRow}>
          <div className={styles.infoLabel}>
            {t('pages.web3Wallets.settings.addressCount', '钱包地址数量')}
          </div>
          <div className={styles.infoValue}>
            {device.addressCount} {t('pages.web3Wallets.settings.addresses', '个地址')}
          </div>
        </div>
      </div>

      {/* 安全设置状态 */}
      <div className={styles.sectionTitle}>
        <SafetyCertificateOutlined className={styles.sectionIcon} />
        {t('pages.web3Wallets.settings.securityStatus', '安全设置状态')}
      </div>
      <div className={styles.section}>
        <div className={styles.statusRow}>
          <Tag
            className={`${styles.statusTag} ${device.twoFAEnabled ? styles.statusTagEnabled : styles.statusTagDisabled}`}
          >
            <KeyOutlined />
            {device.twoFAEnabled
              ? t('pages.web3Wallets.settings.twoFAEnabled', '2FA已开启')
              : t('pages.web3Wallets.settings.twoFADisabled', '2FA未开启')}
          </Tag>
          <Tag
            className={`${styles.statusTag} ${device.biometricEnabled ? styles.statusTagEnabled : styles.statusTagDisabled}`}
          >
            <ScanOutlined />
            {device.biometricEnabled
              ? t('pages.web3Wallets.settings.biometricEnabled', '生物识别已开启')
              : t('pages.web3Wallets.settings.biometricDisabled', '生物识别未开启')}
          </Tag>
        </div>
      </div>

      <div className={styles.noteText}>
        {t(
          'pages.web3Wallets.settings.note',
          '用户需在App端自行设置2FA和生物识别，后台仅显示状态',
        )}
      </div>

      {/* 说明信息 */}
      <div className={styles.infoBox}>
        <div className={styles.infoBoxTitle}>
          <InfoCircleOutlined />
          {t('pages.web3Wallets.settings.infoTitle', 'Web3钱包说明')}
        </div>
        <div className={styles.infoBoxContent}>
          {t(
            'pages.web3Wallets.settings.infoContent',
            'Web3用户通过区块链钱包登录，一个设备可以绑定多个网络的钱包地址。黑名单功能针对单个地址进行标记。',
          )}
        </div>
      </div>
    </Modal>
  );
};

export default DeviceSettingsModal;

