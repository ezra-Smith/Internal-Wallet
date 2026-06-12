package logic

const (
	// TradePasswordAttemptsKeyPrefix 交易密码错误次数 Redis key 前缀
	// 保持与 business 服务一致
	TradePasswordAttemptsKeyPrefix = "trade_pwd_attempts:"
)
