package svc

import "time"

// ========== Redis Key 前缀 ==========

const (
	// RedisKeyPrefix Redis 键前缀
	RedisKeyPrefix = "chainsync"

	// TransactionProcessedKeyFmt 已处理交易键格式: chainsync:tx:{chain}:{txHash}
	TransactionProcessedKeyFmt = RedisKeyPrefix + ":tx:%s:%s"

	// BlockProgressKeyFmt 区块进度键格式: chainsync:block_progress:{chain}
	BlockProgressKeyFmt = RedisKeyPrefix + ":block_progress:%s"

	// BalanceCacheKeyFmt 余额缓存键格式: chainsync:balance:{chain}:{address}
	BalanceCacheKeyFmt = RedisKeyPrefix + ":balance:%s:%s"

	// ChainLatestBlockKeyFmt 链最新区块键格式: chainsync:block:{chain}
	ChainLatestBlockKeyFmt = RedisKeyPrefix + ":block:%s"

	// TokenInfoCacheKeyFmt 代币信息缓存键格式: chainsync:token_info:{chain}:{address}
	TokenInfoCacheKeyFmt = RedisKeyPrefix + ":token_info:%s:%s"

	// TRC10TokenInfoCacheKeyFmt TRC10代币信息缓存键格式: chainsync:trc10_token_info:{tokenId}
	TRC10TokenInfoCacheKeyFmt = RedisKeyPrefix + ":trc10_token_info:%s"
)

// ========== TTL (过期时间) ==========

const (
	// TransactionProcessedTTL 已处理交易记录 TTL
	TransactionProcessedTTL = 7 * 24 * time.Hour // 7天

	// BlockProgressTTL 区块进度 TTL
	BlockProgressTTL = 7 * 24 * time.Hour // 7天

	// BalanceCacheTTL 余额缓存 TTL
	BalanceCacheTTL = 24 * time.Hour // 24小时

	// TokenInfoCacheTTL 代币信息缓存 TTL
	TokenInfoCacheTTL = 24 * time.Hour // 24小时

	// LastProcessedBlockTTL 最后处理区块缓存 TTL
	LastProcessedBlockTTL = 7 * 24 * time.Hour // 7天

	// BlockProgressTTLSeconds 区块进度 TTL (秒)
	BlockProgressTTLSeconds = 7 * 24 * 60 * 60 // 7天 (秒)
)

// ========== 超时设置 ==========

const (
	// DefaultTimeout 默认超时
	DefaultTimeout = 30 * time.Second

	// BlockScanTimeout 单个区块扫描超时
	BlockScanTimeout = 30 * time.Second

	// RedisOperationTimeout Redis 操作超时
	RedisOperationTimeout = 3 * time.Second

	// DBOperationTimeout 数据库操作超时
	DBOperationTimeout = 5 * time.Second

	// KafkaSendTimeout Kafka 消息发送超时
	KafkaSendTimeout = 10 * time.Second

	// HealthCheckTimeout 健康检查超时
	HealthCheckTimeout = 10 * time.Second

	// ProviderRequestTimeout Provider 请求超时
	ProviderRequestTimeout = 15 * time.Second
)

// ========== 重试策略 ==========

const (
	// DefaultMaxRetries 默认最大重试次数
	DefaultMaxRetries = 3

	// DefaultRetryInitialDelay 默认重试初始延迟
	DefaultRetryInitialDelay = 100 * time.Millisecond

	// DefaultRetryMaxDelay 默认重试最大延迟
	DefaultRetryMaxDelay = 5 * time.Second

	// DefaultRetryMultiplier 默认重试指数因子
	DefaultRetryMultiplier = 2.0
)

// ========== 并发和批处理 ==========

const (
	// DefaultBatchSize 默认批处理大小
	DefaultBatchSize = 10

	// DefaultMaxConcurrency 默认最大并发数
	DefaultMaxConcurrency = 5

	// DefaultWorkerPoolSize 默认工作池大小
	DefaultWorkerPoolSize = 10

	// DefaultProgressLogStep 默认进度日志间隔
	DefaultProgressLogStep = 100

	// DefaultBlockLookback 默认区块回溯数量
	DefaultBlockLookback = 20
)

// ========== 健康检查和故障转移 ==========

const (
	// DefaultHealthCheckInterval 默认健康检查间隔
	DefaultHealthCheckInterval = 30 * time.Second

	// DefaultFailoverThreshold 默认故障转移阈值
	DefaultFailoverThreshold = 3

	// DefaultRecoveryCheckInterval 默认恢复检查间隔
	DefaultRecoveryCheckInterval = 5 * time.Minute
)

// ========== 链相关常量 ==========

const (
	// DefaultEthereumConfirmations 以太坊默认确认数
	DefaultEthereumConfirmations = 12

	// DefaultBSCConfirmations BSC 默认确认数
	DefaultBSCConfirmations = 6

	// DefaultTronConfirmations TRON 默认确认数
	DefaultTronConfirmations = 19

	// DefaultTokenDecimals 默认代币小数位
	DefaultTokenDecimals = 18

	// DefaultTRC10Decimals TRC-10 默认小数位
	DefaultTRC10Decimals = 6
)

// ========== ERC20/TRC20 方法签名 ==========

const (
	// ERC20TransferMethodID ERC20 transfer 方法ID
	ERC20TransferMethodID = "a9059cbb"

	// ERC20BalanceOfMethodID ERC20 balanceOf 方法ID
	ERC20BalanceOfMethodID = "70a08231"

	// ERC20DecimalsMethodID ERC20 decimals 方法ID
	ERC20DecimalsMethodID = "313ce567"

	// ERC20SymbolMethodID ERC20 symbol 方法ID
	ERC20SymbolMethodID = "95d89b41"

	// ERC20NameMethodID ERC20 name 方法ID
	ERC20NameMethodID = "06fdde03"
)

// ========== TRON 相关常量 ==========

const (
	// TronAddressPrefix TRON 地址前缀 (十六进制)
	TronAddressPrefix = "41"

	// TronAddressPrefixBase58 TRON 地址前缀 (Base58)
	TronAddressPrefixBase58 = "T"

	// TronContractTypeTransfer TRX 转账合约类型
	TronContractTypeTransfer = "TransferContract"

	// TronContractTypeTransferAsset TRC-10 转账合约类型
	TronContractTypeTransferAsset = "TransferAssetContract"

	// TronContractTypeTriggerSmart 智能合约调用类型
	TronContractTypeTriggerSmart = "TriggerSmartContract"

	// TronContractTypeTriggerSmartContract 别名
	TronContractTypeTriggerSmartContract = "TriggerSmartContract"

	// TronContractTypeUnDelegateResource 取消资源代理合约类型
	TronContractTypeUnDelegateResource = "UnDelegateResourceContract"

	// TronContractTypeDelegateResource 资源代理合约类型
	TronContractTypeDelegateResource = "DelegateResourceContract"

	// TronContractTypeFreezeBalanceV2 冻结余额V2合约类型
	TronContractTypeFreezeBalanceV2 = "FreezeBalanceV2Contract"

	// TronContractTypeUnfreezeBalanceV2 解冻余额V2合约类型
	TronContractTypeUnfreezeBalanceV2 = "UnfreezeBalanceV2Contract"

	// TronContractTypeWithdrawExpireUnfreeze 提取过期解冻合约类型
	TronContractTypeWithdrawExpireUnfreeze = "WithdrawExpireUnfreezeContract"

	// TronContractTypeVoteWitness 投票合约类型
	TronContractTypeVoteWitness = "VoteWitnessContract"

	// TronContractTypeAccountCreate 创建账户合约类型
	TronContractTypeAccountCreate = "AccountCreateContract"

	// TronContractTypeAccountUpdate 更新账户合约类型
	TronContractTypeAccountUpdate = "AccountUpdateContract"
)

// ========== TRON API 路径 ==========

const (
	// TronAPIGetNowBlock 获取最新区块
	TronAPIGetNowBlock = "/wallet/getnowblock"

	// TronAPIGetBlockByNum 按区块号获取区块
	TronAPIGetBlockByNum = "/wallet/getblockbynum"

	// TronAPIGetAssetIssueByID 获取 TRC-10 代币信息
	TronAPIGetAssetIssueByID = "/wallet/getassetissuebyid"

	// TronAPIGetAccount 获取账户信息
	TronAPIGetAccount = "/wallet/getaccount"

	// TronAPIGetTransactionByID 获取交易信息
	TronAPIGetTransactionByID = "/wallet/gettransactionbyid"
)

// ========== 监控间隔 ==========

const (
	// DefaultAddressMonitorCheckInterval 默认地址监控检查间隔
	DefaultAddressMonitorCheckInterval = 20 * time.Second

	// DefaultTransactionConfirmCheckInterval 默认交易确认检查间隔
	DefaultTransactionConfirmCheckInterval = 15 * time.Second

	// DefaultBalanceValidationCheckInterval 默认余额校验检查间隔
	DefaultBalanceValidationCheckInterval = 5 * time.Minute
)

// ========== 区块扫描相关 ==========

const (
	// DefaultBlockScanBatchSize 默认区块扫描批量大小
	DefaultBlockScanBatchSize = 10

	// DefaultBlockScanConcurrency 默认区块扫描并发数
	DefaultBlockScanConcurrency = 5

	// DefaultBlockScanCheckInterval 默认区块扫描检查间隔(秒)
	DefaultBlockScanCheckInterval = 20

	// DefaultInitialBlockLookback 首次扫描时的回溯区块数
	DefaultInitialBlockLookback = 10

	// DefaultAddressQueryLookback 首次监控地址时查询的回溯区块数
	DefaultAddressQueryLookback = 20

	// DefaultBlockScanTimeout 单个区块扫描超时
	DefaultBlockScanTimeoutSeconds = 10

	// DefaultLatestBlockTimeout 获取最新区块超时
	DefaultLatestBlockTimeoutSeconds = 10

	// DefaultAddressTransactionQueryTimeout 地址交易查询超时(秒)
	DefaultAddressTransactionQueryTimeout = 15
)

// ========== 分页和批处理 ==========

const (
	// DefaultTransactionPageSize 默认交易分页大小
	DefaultTransactionPageSize = 100

	// DefaultMaxPaginationPages 最大分页数限制
	DefaultMaxPaginationPages = 1000

	// DefaultTransactionConfirmBatchSize 交易确认批处理大小
	DefaultTransactionConfirmBatchSize = 100

	// DefaultBalanceValidationBatchSize 余额校验批处理大小
	DefaultBalanceValidationBatchSize = 20
)

// ========== 交易确认相关 ==========

const (
	// DefaultTransactionConfirmCheckIntervalSeconds 默认交易确认检查间隔(秒)
	DefaultTransactionConfirmCheckIntervalSeconds = 15

	// DefaultTransactionCleanupIntervalHours 默认交易清理间隔(小时)
	DefaultTransactionCleanupIntervalHours = 24

	// DefaultTransactionExpirationDays 默认交易过期天数
	DefaultTransactionExpirationDays = 7

	// DefaultTransactionConfirmContextTimeout 交易确认上下文超时(秒)
	DefaultTransactionConfirmContextTimeout = 30
)

// ========== 区块重组检测 ==========

const (
	// DefaultReorgCheckDepth 区块重组检测深度
	DefaultReorgCheckDepth = 6

	// ReorgStatusOrphaned 交易被重组状态: 孤块
	ReorgStatusOrphaned uint8 = 3

	// ReorgStatusReorged 交易被重组状态: 重组
	ReorgStatusReorged uint8 = 4

	// TransactionStatusPending 交易状态: 待确认
	TransactionStatusPending uint8 = 0

	// TransactionStatusSent 交易状态: 已发送
	TransactionStatusSent uint8 = 1

	// TransactionStatusFailed 交易状态: 失败
	TransactionStatusFailed uint8 = 2
)

// ========== Kafka 重试队列 ==========

const (
	// DefaultKafkaRetryMaxAttempts 默认Kafka重试最大次数
	DefaultKafkaRetryMaxAttempts = 5

	// DefaultKafkaRetryInitialDelay 默认Kafka重试初始延迟
	DefaultKafkaRetryInitialDelay = 1 * time.Second

	// DefaultKafkaRetryMaxDelay 默认Kafka重试最大延迟
	DefaultKafkaRetryMaxDelay = 5 * time.Minute

	// DefaultKafkaRetryBackoffMultiplier 默认Kafka重试退避倍数
	DefaultKafkaRetryBackoffMultiplier = 2.0

	// DefaultKafkaRetryQueueCheckInterval 默认Kafka重试队列检查间隔
	DefaultKafkaRetryQueueCheckInterval = 30 * time.Second

	// DefaultKafkaRetryQueueBatchSize 默认Kafka重试队列批处理大小
	DefaultKafkaRetryQueueBatchSize = 50

	// KafkaRetryStatusPending Kafka重试状态: 待重试
	KafkaRetryStatusPending uint8 = 0

	// KafkaRetryStatusSuccess Kafka重试状态: 成功
	KafkaRetryStatusSuccess uint8 = 1

	// KafkaRetryStatusFailed Kafka重试状态: 失败(超过最大重试次数)
	KafkaRetryStatusFailed uint8 = 2
)

// ========== 缓存TTL ==========

const (
	// DefaultBlockProgressCacheTTLSeconds 区块进度缓存TTL(秒) - 24小时
	DefaultBlockProgressCacheTTLSeconds = 24 * 60 * 60

	// DefaultAddressBlockCacheTTLSeconds 地址区块缓存TTL(秒) - 7天
	DefaultAddressBlockCacheTTLSeconds = 7 * 24 * 60 * 60
)

// ========== 服务启动延迟 ==========

const (
	// DefaultServiceInitialDelaySeconds 默认服务启动延迟(秒)
	DefaultServiceInitialDelaySeconds = 3

	// DefaultBalanceValidationStartDelay 余额校验启动延迟
	DefaultBalanceValidationStartDelay = 5 * time.Second
)
