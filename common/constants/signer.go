package constants

// ========== Temperature 温度类型 ==========
const (
	TemperatureHot  = 1 // 热钱包
	TemperatureCold = 2 // 冷钱包
)

// ========== SeedStatus Seed状态 ==========
const (
	SeedStatusActive     = 1 // 激活
	SeedStatusInactive   = 2 // 停用
	SeedStatusDeprecated = 3 // 废弃
)

// ========== OperationType 操作类型 ==========
const (
	OpTypeWithdraw     = 1 // 提币
	OpTypeCollect      = 2 // 归集
	OpTypeColdTransfer = 3 // 冷转
	OpTypeTest         = 4 // 测试
	OpTypeOther        = 5 // 其他
)

// ========== SignatureStatus 签名状态 ==========
const (
	SignStatusPending         = 1 // 待处理
	SignStatusSuccess         = 2 // 成功
	SignStatusFailed          = 3 // 失败
	SignStatusRejected        = 4 // 拒绝
	SignStatusPendingApproval = 5 // 待审批（冷钱包/大额）
)

// ========== ApprovalStatus 审批状态 ==========
const (
	ApprovalStatusPending  = 1 // 待审批
	ApprovalStatusApproved = 2 // 已同意
	ApprovalStatusRejected = 3 // 已拒绝
	ApprovalStatusCanceled = 4 // 已取消
)

// ========== Chain 链名称 ==========
const (
	ChainETH      = "ETH"
	ChainBTC      = "BTC"
	ChainBSC      = "BSC"
	ChainTRN      = "TRON" // TRON链
	ChainSOL      = "SOL"
	ChainARBITRUM = "ARBITRUM"
	ChainPOLYGON  = "POLYGON"
)

// ========== BIP44 coin_type ==========
// BIP44派生路径: m/purpose'/coin_type'/account'/change/address_index
// purpose: 44 (BIP44)
// account: 通常为0
// change: 0=外部链(接收地址), 1=内部链(找零地址)
var CoinTypes = map[string]uint32{
	ChainBTC:      0,   // Bitcoin
	ChainETH:      60,  // Ethereum
	ChainBSC:      60,  // BSC使用ETH的coin_type
	ChainTRN:      195, // Tron
	ChainSOL:      501, // Solana
	ChainARBITRUM: 60,  // Arbitrum使用ETH的coin_type
	ChainPOLYGON:  60,  // Polygon使用ETH的coin_type
}

// ========== Seed ID 预定义 ==========
const (
	SeedEVMHot = "seed_evm_hot" // EVM系列热钱包
	SeedBTCHot = "seed_btc_hot" // BTC热钱包
	SeedTRXHot = "seed_trx_hot" // TRX热钱包
	SeedSOLHot = "seed_sol_hot" // SOL热钱包
)

// ========== 配置限制 ==========
const (
	MaxBatchExportSize = 1000  // 最大批量导出公钥数量
	MaxDailyAmount     = 10000 // 默认每日签名金额限制（单位根据链而定）
	MinPasswordLength  = 16    // 最小密码长度
	MaxPasswordLength  = 128   // 最大密码长度
	MinSeedLength      = 16    // 最小Seed长度（字节）
	MaxSeedLength      = 64    // 最大Seed长度（字节）
)

// ========== BIP44路径常量 ==========
const (
	BIP44Purpose = 44 // BIP44 purpose
)

// GetCoinType 获取链的coin_type
func GetCoinType(chain string) (uint32, bool) {
	coinType, ok := CoinTypes[chain]
	return coinType, ok
}

// IsChainSupported 检查链是否支持
func IsChainSupported(chain string) bool {
	_, ok := CoinTypes[chain]
	return ok
}

// GetSeedIDByChain 根据链获取默认的seed_id
func GetSeedIDByChain(chain string) string {
	switch chain {
	case ChainETH, ChainBSC, ChainARBITRUM, ChainPOLYGON:
		return SeedEVMHot
	case ChainBTC:
		return SeedBTCHot
	case ChainTRN:
		return SeedTRXHot
	case ChainSOL:
		return SeedSOLHot
	default:
		return ""
	}
}

// OperationTypeName 操作类型名称映射
var OperationTypeName = map[int]string{
	OpTypeWithdraw:     "withdraw",
	OpTypeCollect:      "collect",
	OpTypeColdTransfer: "cold_transfer",
	OpTypeTest:         "test",
	OpTypeOther:        "other",
}

// GetOperationTypeName 获取操作类型名称
func GetOperationTypeName(opType int) string {
	if name, ok := OperationTypeName[opType]; ok {
		return name
	}
	return "unknown"
}

// SignatureStatusName 签名状态名称映射
var SignatureStatusName = map[int]string{
	SignStatusPending:         "pending",
	SignStatusSuccess:         "success",
	SignStatusFailed:          "failed",
	SignStatusRejected:        "rejected",
	SignStatusPendingApproval: "pending_approval",
}

// GetSignatureStatusName 获取签名状态名称
func GetSignatureStatusName(status int) string {
	if name, ok := SignatureStatusName[status]; ok {
		return name
	}
	return "unknown"
}

// SeedStatusName Seed状态名称映射
var SeedStatusName = map[int]string{
	SeedStatusActive:     "active",
	SeedStatusInactive:   "inactive",
	SeedStatusDeprecated: "deprecated",
}

// GetSeedStatusName 获取Seed状态名称
func GetSeedStatusName(status int) string {
	if name, ok := SeedStatusName[status]; ok {
		return name
	}
	return "unknown"
}

// ApprovalStatusName 审批状态名称映射
var ApprovalStatusName = map[int]string{
	ApprovalStatusPending:  "pending",
	ApprovalStatusApproved: "approved",
	ApprovalStatusRejected: "rejected",
	ApprovalStatusCanceled: "canceled",
}

// GetApprovalStatusName 获取审批状态名称
func GetApprovalStatusName(status int) string {
	if name, ok := ApprovalStatusName[status]; ok {
		return name
	}
	return "unknown"
}
