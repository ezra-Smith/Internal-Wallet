package config

import (
	commonDB "internalwallet/common/db"
	"internalwallet/pkg/chainnode"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf

	// 雪花ID配置（必须配置，范围0-1023，每个服务唯一）
	NodeID int64 `json:",optional"`

	// Direct chain node configs (no ChainRpc dependency).
	Chains []chainnode.ChainConfig `json:",optional"`

	// GORM MySQL 配置（使用通用配置）
	MySQL commonDB.MySQLConfig `json:",optional"`

	// Redis 缓存配置（可选）
	CacheRedis cache.CacheConf `json:",optional"`

	// RPC clients
	SignerRpc     zrpc.RpcClientConf `json:",optional"`
	AccountingRpc zrpc.RpcClientConf `json:",optional"`

	Consolidation ConsolidationConfig `json:"Consolidation,optional"`

	// 数据库自动迁移配置（仅开发/测试环境建议启用）
	AutoMigrate bool `json:",optional,default=false"`
}

type ConsolidationConfig struct {
	Enabled bool `json:",optional,default=true"`

	// Polling intervals (seconds)
	DiscoveryInterval    int64 `json:",optional,default=300"`
	StatusCheckInterval  int64 `json:",optional,default=30"`
	EnergyCheckInterval  int64 `json:",optional,default=10"`
	ExecutorTickInterval int64 `json:",optional,default=2"`

	// EVM stuck-tx handling (legacy gas_price bump).
	// If a tx has no receipt for longer than this threshold, StatusTracker will rebuild+re-sign with the same nonce and higher gas_price.
	EvmBumpAfterSeconds  int64   `json:",optional,default=120"`
	EvmGasBumpMultiplier float64 `json:",optional,default=1.2"`
	EvmMaxBumps          int     `json:",optional,default=3"`

	// Concurrency limits
	MaxConcurrentTasks    int `json:",optional,default=20"`
	MaxConcurrentPerChain struct {
		TRON int `json:",optional,default=5"`
		ETH  int `json:",optional,default=10"`
		BSC  int `json:",optional,default=10"`
	} `json:"MaxConcurrentPerChain,optional"`

	// To-address selection for consolidation transfers:
	// - target_addresses: use Consolidation.TargetAddresses (per chain/asset)
	// - system_hot_wallet: use Signer.GetCompanyWallet (per chain)
	ToAddressMode string `json:",optional,default=target_addresses"`

	// System hot wallet selector (used when ToAddressMode=system_hot_wallet).
	SystemHotWalletAddressType string `json:",optional,default=hot_primary"`
	SystemHotWalletTemperature int32  `json:",optional,default=1"`

	// Target addresses (hot wallets): chain -> asset_symbol -> address
	TargetAddresses map[string]map[string]string `json:"TargetAddresses,optional"`

	// Minimum balance thresholds: chain -> asset_symbol -> amount(smallest unit as string)
	MinBalanceThresholds map[string]map[string]string `json:"MinBalanceThresholds,optional"`

	// Validate token configs via chain nodes on startup (fail-fast on mismatch)
	ValidateTokenInfo bool `json:",optional,default=true"`

	// Native reserve kept on deposit addresses (smallest unit): chain -> native_symbol -> amount
	NativeReserves map[string]map[string]string `json:"NativeReserves,optional"`

	// Minimum consolidation amount for native assets (smallest unit): chain -> native_symbol -> amount
	// Native consolidation triggers only if (balance - reserve) >= MinConsolidationAmount.
	MinConsolidationAmount map[string]map[string]string `json:"MinConsolidationAmount,optional"`

	// Gas/Fee settings
	// GasSafetyMultipliers applies a safety buffer to estimated fees, by chain:
	//   fee_with_buffer = ceil(fee * multiplier)
	// Example:
	//   TRON: 2.0
	//   ETH:  2.0
	//   BSC:  2.0
	//
	// This affects:
	// - EVM: EstimateGas -> estimated_fee (wei)
	// - TRON: EstimateTronFee -> estimated_fee_sun (SUN)
	// - TopUp: derived shortfall amount (native smallest unit)
	GasSafetyMultipliers map[string]float64 `json:"GasSafetyMultipliers,optional"`
	MaxGasPrice          map[string]string  `json:"MaxGasPrice,optional"` // chain -> max gas price (wei)

	// Risk controls (native smallest unit).
	Risk RiskConfig `json:"Risk,optional"`

	// FeeGuard blocks executor loops when the estimated USDT token-transfer fee exceeds risk limits.
	// This prevents mass transitions into NeedGas during high-fee periods.
	FeeGuard FeeGuardConfig `json:"FeeGuard,optional"`

	// TRON TRC20 fee mode when energy is insufficient:
	// - energy_rental: rent energy via provider (requires EnergyRental.Enabled=true for TRON tokens)
	// - trx_fee: pay shortage by burning TRX (requires enough TRX balance)
	TronTrc20FeeMode string `json:",optional,default=energy_rental"`

	// Confirmations
	RequiredConfirmations struct {
		TRON uint64 `json:",optional,default=19"`
		ETH  uint64 `json:",optional,default=12"`
		BSC  uint64 `json:",optional,default=15"`
	} `json:"RequiredConfirmations,optional"`

	// Retry & timeout
	MaxRetries          int     `json:",optional,default=3"`
	RetryBackoffSeconds []int64 `json:",optional"`
	MaxPendingDuration  int64   `json:",optional,default=1800"`
	// If a tx has no receipt beyond this duration, abandon the current tx_hash/signed_tx and retry the task.
	// This prevents tasks being stuck InProgress/Timeout forever due to broadcast rejection / missing tx / RPC inconsistencies.
	MaxNoReceiptDuration int64 `json:",optional,default=3600"`

	// Cooldown period (seconds)
	ConsolidationCooldown int64 `json:",optional,default=86400"`

	// Task claiming / leasing
	ClaimLeaseSeconds int64 `json:",optional,default=60"`

	// Strategy
	Strategy      string `json:",optional,default=immediate"` // immediate/scheduled
	ScheduledTime string `json:",optional,default=02:00"`     // UTC HH:MM, for scheduled strategy

	// TRON Energy rental
	EnergyRental EnergyRentalConfig `json:"EnergyRental,optional"`

	// Auto top-up (gas/activation) for stuck token consolidation tasks.
	TopUp TopUpConfig `json:"TopUp,optional"`
}

type TokenConfig struct {
	Contract string `json:"Contract"`
	Decimals uint32 `json:"Decimals"`
}

type RiskConfig struct {
	// MaxNativeSpendPerTask caps automatic spending per task (smallest unit).
	// It is used by:
	// - Executor: if required native shortfall exceeds cap => mark task PermanentFailed.
	// - TopUp: prevent and cap top-up spending.
	// - FeeGuard: pause execution when typical USDT fee exceeds cap.
	MaxNativeSpendPerTask map[string]string `json:"MaxNativeSpendPerTask,optional"` // chain -> max cost (smallest unit)
}

type FeeGuardConfig struct {
	Enabled bool `json:",optional,default=false"`

	// How often to refresh fee estimates per chain (seconds).
	CheckIntervalSeconds int64 `json:",optional,default=30"`

	// How long to block a chain executor loop after triggering (seconds).
	BlockSeconds int64 `json:",optional,default=60"`

	// Typical gas limit for an ERC20 USDT transfer, used with current gas_price to estimate fees.
	// This avoids relying on EstimateGas(transfer) which may fail (e.g. insufficient token balance causes revert).
	EvmUsdtTransferGasLimit uint64 `json:",optional,default=70000"`
}

type EnergyRentalConfig struct {
	Enabled  bool   `json:",optional,default=false"`
	Provider string `json:",optional,default=itrx"`

	ApiEndpoint string `json:",optional"`
	ApiKey      string `json:",optional"` // prefer env var override
	ApiSecret   string `json:",optional"` // prefer env var override

	DefaultRentalAmount   int `json:",optional,default=32000"`
	RentalDurationSeconds int `json:",optional,default=3600"`

	// Safety caps (fund protection).
	// Prevent runaway iTRX spending on a single stuck address/task.
	MaxOrdersPerTask       int   `json:",optional,default=2"`
	MaxEnergyPerOrder      int   `json:",optional,default=100000"`
	MaxTotalCostSunPerTask int64 `json:",optional,default=3000000"`

	MaxRetries       int `json:",optional,default=3"`
	TimeoutSeconds   int `json:",optional,default=300"`
	RetryBaseSeconds int `json:",optional,default=2"`
}

type TopUpConfig struct {
	Enabled bool `json:",optional,default=false"`

	// Only top-up token consolidation tasks (never native sweeps).
	OnlyForTokens bool `json:",optional,default=true"`

	// Hot wallet selector via Signer GetCompanyWallet.
	HotWalletAddressType string `json:",optional,default=hot_primary"`
	HotWalletTemperature int32  `json:",optional,default=1"`

	// Risk controls (per task).
	MaxTopUpAttemptsPerTask int `json:",optional,default=3"`

	// Verification confirmations (may be lower than consolidation confirmations).
	VerificationConfirmations struct {
		TRON uint64 `json:",optional,default=10"`
		ETH  uint64 `json:",optional,default=6"`
		BSC  uint64 `json:",optional,default=6"`
	} `json:"VerificationConfirmations,optional"`

	// Worker intervals (seconds).
	TopUpInterval  int64 `json:",optional,default=10"`
	VerifyInterval int64 `json:",optional,default=15"`

	// Backoff for top-up related retries (seconds).
	RetryBackoffSeconds []int64 `json:",optional"`

	// How soon a NeedGas/NeedEnergy task becomes eligible again after marking (seconds).
	TaskRetryDelaySeconds int64 `json:",optional,default=30"`

	// TRON TRC20: fixed TRX top-up (SUN) to cover bandwidth burn when bandwidth points are insufficient.
	// NOTE: top-up does not increase bandwidth points; it ensures the address has enough TRX to be burned as bandwidth fee.
	TronBandwidthTopUpSun string `json:",optional"`
}
