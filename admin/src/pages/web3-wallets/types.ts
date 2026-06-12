/**
 * Web3钱包模块类型定义
 */

/** 支持的链类型 */
export type ChainType = 'ethereum' | 'bsc' | 'polygon' | 'tron';

/** 地址状态 */
export type AddressStatus = 'active' | 'blacklisted' | 'disabled';

/** 2FA状态 */
export type TwoFAStatus = 'enabled' | 'disabled';

/** 生物识别状态 */
export type BiometricStatus = 'enabled' | 'disabled';

/** 钱包地址 */
export interface WalletAddress {
  id: string;
  chain: ChainType;
  address: string;
  status: AddressStatus;
  /** 是否启用 */
  enabled: boolean;
  /** 备注 */
  note?: string;
  /** 来源 */
  source?: string;
  /** 创建时间 */
  createdAt: string;
}

/** 设备信息 */
export interface DeviceInfo {
  id: string;
  deviceId: string;
  /** 钱包地址列表 */
  addresses: WalletAddress[];
  /** 地址数量 */
  addressCount: number;
  /** 黑名单地址数量 */
  blacklistCount: number;
  /** 注册时间 */
  registeredAt: string;
  /** 2FA状态 */
  twoFAEnabled: boolean;
  /** 生物识别状态 */
  biometricEnabled: boolean;
}

/** 统计数据 */
export interface Web3WalletStats {
  /** 设备总数 */
  deviceCount: number;
  /** 记录数 */
  recordCount: number;
  /** 钱包地址总数 */
  addressCount: number;
  /** 网络数 */
  networkCount: number;
  /** 黑名单地址数 */
  blacklistCount: number;
  /** 黑名单占比 */
  blacklistPercent: number;
  /** 支持网络数 */
  supportedNetworks: number;
}

/** 筛选参数 */
export interface Web3WalletFilters {
  keyword?: string;
  addressStatus?: AddressStatus | 'all';
  twoFAStatus?: TwoFAStatus | 'all';
  biometricStatus?: BiometricStatus | 'all';
}

/** 链配置 */
export interface ChainConfig {
  key: ChainType;
  name: string;
  color: string;
  shortName: string;
}

