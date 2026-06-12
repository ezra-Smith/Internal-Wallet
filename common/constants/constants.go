package constants

// 用户状态
const (
	UserStatusNormal  = 1 // 正常
	UserStatusFrozen  = 2 // 冻结
	UserStatusDisable = 3 // 禁用
)

// 账户类型
const (
	AccountTypeSpot     = 1 // 现货账户
	AccountTypeContract = 2 // 合约账户
)

// 订单方向
const (
	OrderSideBuy  = 1 // 买入
	OrderSideSell = 2 // 卖出
)

// 订单类型
const (
	OrderTypeLimit  = 1 // 限价单
	OrderTypeMarket = 2 // 市价单
)

// 订单状态
const (
	OrderStatusPending       = 1 // 待成交
	OrderStatusPartialFilled = 2 // 部分成交
	OrderStatusFilled        = 3 // 完全成交
	OrderStatusCancelled     = 4 // 已撤销
)

// 充值状态
const (
	DepositStatusPending   = 1 // 待确认
	DepositStatusConfirmed = 2 // 已确认
	DepositStatusFailed    = 3 // 失败
)

// 提现状态
const (
	WithdrawStatusPending    = 1 // 待审核
	WithdrawStatusProcessing = 2 // 处理中
	WithdrawStatusCompleted  = 3 // 已完成
	WithdrawStatusRejected   = 4 // 已拒绝
)

// 转账状态
const (
	TransferStatusSuccess = 1 // 成功
	TransferStatusFailed  = 2 // 失败
)

// 风险等级
const (
	RiskLevelLow      = 1 // 低风险
	RiskLevelMedium   = 2 // 中等风险
	RiskLevelHigh     = 3 // 高风险
	RiskLevelCritical = 4 // 严重风险
)

// Kafka Topic
const (
	TopicTradeEvents      = "trade-events"
	TopicMatchingEvents   = "matching-events"
	TopicWalletEvents     = "wallet-events"
	TopicMarketEvents     = "market-events"
	TopicSettlementEvents = "settlement-events"
	TopicRiskEvents       = "risk-events"
)

// 错误码
const (
	ErrCodeSuccess             = 0
	ErrCodeInvalidParam        = 10001
	ErrCodeUnauthorized        = 10002
	ErrCodeForbidden           = 10003
	ErrCodeNotFound            = 10004
	ErrCodeInternalError       = 10005
	ErrCodeDatabaseError       = 10006
	ErrCodeCacheError          = 10007
	ErrCodeInsufficientBalance = 20001
	ErrCodeOrderNotFound       = 20002
	ErrCodeInvalidPrice        = 20003
	ErrCodeInvalidAmount       = 20004
	ErrCodeRiskControl         = 30001
	ErrCodeExceedLimit         = 30002
)

// 数据分类
const (
	CategoryKLine = iota + 1
	CategoryTicker
	CategoryTimes
	CategoryDepth
)

const (
	ActionSubscribeTickerReq  = 1
	ActionSubscribeTickerResp = 2
	ActionSubscribeKlineReq   = 3
	ActionSubscribeKlineResp  = 4
	ActionSubscribeTimeReq    = 5
	ActionSubscribeTimeResp   = 6
	ActionSubscribeDepthReq   = 7
	ActionSubscribeDepthResp  = 8
	ActionSubscribeCancelReq  = 9
	ActionSubscribeCancelResp = 10
)
