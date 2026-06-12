package constants

// ==================== 公司钱包地址索引常量 ====================
// 注意：address_index 0-100 保留给公司/系统使用，不分配给普通用户

const (
	// MinUserAddressIndex 用户地址起始索引（从100001开始）
	MinUserAddressIndex = 100001

	// ==================== 热钱包地址索引 ====================

	// HotWalletPrimaryIndex 热钱包主地址索引
	// 用途：日常充提业务，小额资金池
	// 路径示例：m/44'/60'/0'/0/1 (ETH/BSC)
	HotWalletPrimaryIndex = 1

	// HotWalletBackupIndex 热钱包备用地址索引
	// 用途：主地址故障时的备用地址
	HotWalletBackupIndex = 2

	// ==================== 冷钱包地址索引 ====================

	// ColdWalletPrimaryIndex 冷钱包主地址索引
	// 用途：大额资金存储，离线签名
	ColdWalletPrimaryIndex = 3

	// ColdWalletBackup1Index 冷钱包备份地址1
	ColdWalletBackup1Index = 4

	// ColdWalletBackup2Index 冷钱包备份地址2（第二备份）
	ColdWalletBackup2Index = 5

	// ==================== 归集中转地址索引 ====================

	// CollectionAddress1Index 归集中转地址1
	// 用途：批量归集时的中转地址，降低gas成本
	CollectionAddress1Index = 11

	// CollectionAddress2Index 归集中转地址2
	CollectionAddress2Index = 12

	// ==================== 特殊用途地址索引 ====================

	// FeeCollectionIndex 手续费归集地址
	// 用途：收集各链的手续费
	FeeCollectionIndex = 20

	// TestAddressIndex 测试地址索引
	// 用途：测试环境使用
	TestAddressIndex = 99
)

// ==================== 钱包温度类型 ====================

const (
	// WalletTypeHot 热钱包
	WalletTypeHot = 1

	// WalletTypeCold 冷钱包
	WalletTypeCold = 2
)

// ==================== 金额阈值配置 ====================

const (
	// HotWalletMaxAmount 热钱包最大持有金额（USDT）
	// 超过此金额需转入冷钱包
	HotWalletMaxAmount = 50000.0

	// ColdTransferThreshold 冷钱包转账阈值（USDT）
	// 大于此金额需要冷钱包签名
	ColdTransferThreshold = 10000.0

	// AutoApproveThreshold 自动审批阈值（USDT）
	// 小于此金额自动审批，大于需人工审核
	AutoApproveThreshold = 1000.0
)

// IsUserAddress 判断是否为用户地址索引
func IsUserAddress(index int64) bool {
	return index >= MinUserAddressIndex
}

// IsCompanyAddress 判断是否为公司地址索引
func IsCompanyAddress(index int64) bool {
	return index > 0 && index < MinUserAddressIndex
}

// IsHotWalletAddress 判断是否为热钱包地址
func IsHotWalletAddress(index int64) bool {
	return index == HotWalletPrimaryIndex || index == HotWalletBackupIndex
}

// IsColdWalletAddress 判断是否为冷钱包地址
func IsColdWalletAddress(index int64) bool {
	return index >= ColdWalletPrimaryIndex && index <= ColdWalletBackup2Index
}

// GetWalletTypeName 获取钱包类型名称
func GetWalletTypeName(walletType int) string {
	switch walletType {
	case WalletTypeHot:
		return "热钱包"
	case WalletTypeCold:
		return "冷钱包"
	default:
		return "未知"
	}
}
