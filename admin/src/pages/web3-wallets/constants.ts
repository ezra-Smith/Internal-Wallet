import type { ChainConfig, ChainType, DeviceInfo, Web3WalletStats } from './types';

/**
 * Web3钱包模块常量和模拟数据
 */

/** 链配置 */
export const CHAIN_CONFIG: Record<ChainType, ChainConfig> = {
  ethereum: {
    key: 'ethereum',
    name: 'Ethereum',
    color: '#627eea',
    shortName: 'ETH',
  },
  bsc: {
    key: 'bsc',
    name: 'BSC',
    color: '#f3ba2f',
    shortName: 'BSC',
  },
  polygon: {
    key: 'polygon',
    name: 'Polygon',
    color: '#8247e5',
    shortName: 'MATIC',
  },
  tron: {
    key: 'tron',
    name: 'Tron',
    color: '#eb0029',
    shortName: 'TRX',
  },
};

/** 模拟统计数据 */
export const MOCK_STATS: Web3WalletStats = {
  deviceCount: 3,
  recordCount: 4,
  addressCount: 10,
  networkCount: 4,
  blacklistCount: 1,
  blacklistPercent: 10.0,
  supportedNetworks: 4,
};

/** 模拟设备数据 */
export const MOCK_DEVICES: DeviceInfo[] = [
  {
    id: '1',
    deviceId: 'DEVICE-A1B2C3D4',
    addressCount: 4,
    blacklistCount: 0,
    registeredAt: '2024-01-15',
    twoFAEnabled: true,
    biometricEnabled: true,
    addresses: [
      {
        id: '1-1',
        chain: 'ethereum',
        address: '0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb',
        status: 'active',
        enabled: true,
        createdAt: '2024-01-15',
      },
      {
        id: '1-2',
        chain: 'bsc',
        address: '0x8B3192f2C7f5e9b7bf6b05d3E5B1e7D8C9A4F3E2',
        status: 'active',
        enabled: true,
        createdAt: '2024-01-15',
      },
      {
        id: '1-3',
        chain: 'polygon',
        address: '0xC4F8A7B902E1F3C5A6B8E907C2F1A4B5E8D9C7A6',
        status: 'disabled',
        enabled: false,
        note: '未启用',
        createdAt: '2024-01-16',
      },
      {
        id: '1-4',
        chain: 'ethereum',
        address: '0x9876543210FEDCBA9876543210FEDCBA98765432',
        status: 'active',
        enabled: true,
        createdAt: '2024-01-17',
      },
    ],
  },
  {
    id: '2',
    deviceId: 'DEVICE-E5F6G7H8',
    addressCount: 2,
    blacklistCount: 0,
    registeredAt: '2024-02-20',
    twoFAEnabled: true,
    biometricEnabled: false,
    addresses: [
      {
        id: '2-1',
        chain: 'ethereum',
        address: '0xABCDEF1234567890ABCDEF1234567890ABCDEF12',
        status: 'active',
        enabled: true,
        createdAt: '2024-02-20',
      },
      {
        id: '2-2',
        chain: 'bsc',
        address: '0x1234567890ABCDEF1234567890ABCDEF12345678',
        status: 'active',
        enabled: true,
        createdAt: '2024-02-20',
      },
    ],
  },
  {
    id: '3',
    deviceId: 'DEVICE-I9J0K1L2',
    addressCount: 4,
    blacklistCount: 1,
    registeredAt: '2024-03-10',
    twoFAEnabled: false,
    biometricEnabled: true,
    addresses: [
      {
        id: '3-1',
        chain: 'ethereum',
        address: '0xDEF1234567890ABCDEF1234567890ABCDEF12345',
        status: 'blacklisted',
        enabled: true,
        note: '黑名单',
        source: '可疑交易行为',
        createdAt: '2024-03-10',
      },
      {
        id: '3-2',
        chain: 'bsc',
        address: '0xBCA9876543210FEDCBA9876543210FEDCBA98765',
        status: 'active',
        enabled: true,
        createdAt: '2024-03-10',
      },
      {
        id: '3-3',
        chain: 'polygon',
        address: '0x567890ABCDEF1234567890ABCDEF1234567890AB',
        status: 'active',
        enabled: true,
        createdAt: '2024-03-11',
      },
      {
        id: '3-4',
        chain: 'tron',
        address: 'TN1234567890abcdefghijklmnopqrstuvw',
        status: 'disabled',
        enabled: false,
        note: '未启用',
        createdAt: '2024-03-12',
      },
    ],
  },
];

/** 权限常量 */
export const PERM = {
  list: 'ListWeb3Wallets',
  view: 'ViewWeb3Wallet',
  blacklist: 'ManageWeb3Blacklist',
  settings: 'ManageWeb3Settings',
};

