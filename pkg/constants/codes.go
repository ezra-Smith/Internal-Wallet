package constants

// HTTP/RPC 响应码
const (
	CodeSuccess      = 0   // 成功
	CodePartial      = 1   // 部分成功
	CodeBadRequest   = 400 // 参数错误
	CodeUnauthorized = 401 // 未授权
	CodeForbidden    = 402 // 余额不足/权限不足
	CodeNotFound     = 404 // 资源不存在
	CodeConflict     = 409 // 冲突（已存在）
	CodeInternal     = 500 // 内部错误
)

// 结算状态
const (
	SettlementStatusPending    = 0 // 待处理
	SettlementStatusProcessing = 1 // 处理中
	SettlementStatusSuccess    = 2 // 成功
	SettlementStatusFailed     = 3 // 失败
)

// 最大重试次数
const MaxRetryCount = 3

// 交易方向
const (
	SideBuy  = 1 // 买入
	SideSell = 2 // 卖出
)

// 交易角色
const (
	RoleMaker = 1 // Maker（挂单方）
	RoleTaker = 2 // Taker（吃单方）
)

// 余额变更类型（Wallet 服务使用）
const (
	ChangeTypeDeductFrozen = 1 // 从冻结扣除
	ChangeTypeAddAvailable = 2 // 增加可用
	ChangeTypeFee          = 3 // 手续费扣除
)

// 流水类型
const (
	TxTypeTradeIncome  = 1 // 交易收入
	TxTypeTradeOutcome = 2 // 交易支出
	TxTypeFee          = 3 // 手续费
	TxTypeFreeze       = 4 // 冻结
	TxTypeUnfreeze     = 5 // 解冻
)

// 返佣状态
const (
	RebateStatusPending = 0 // 待返佣
	RebateStatusDone    = 1 // 已返佣
)

// 默认费率
const (
	DefaultMakerFeeRate = 0.001 // 0.1%
	DefaultTakerFeeRate = 0.002 // 0.2%
)
