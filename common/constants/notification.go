package constants

// ====================================================================================
// 消息通知相关常量
// ====================================================================================

// 消息接收者类型
const (
	// NotificationRecipientWeb2 Web2 中心化用户（users 表）
	NotificationRecipientWeb2 = "web2_user"

	// NotificationRecipientWeb3 Web3 去中心化用户（web3_users 表）
	NotificationRecipientWeb3 = "web3_user"
)

// 消息类型
const (
	// NotificationTypeTransfer 转账相关（转账成功、转账失败、收到转账等）
	NotificationTypeTransfer = "transfer"

	// NotificationTypeDeposit 充值相关（充值到账、充值确认中等）
	NotificationTypeDeposit = "deposit"

	// NotificationTypeWithdraw 提现相关（提现审核、提现成功、提现失败等）
	NotificationTypeWithdraw = "withdraw"

	// NotificationTypeSystem 系统公告（Admin 后台发送的系统通知）
	NotificationTypeSystem = "system"

	// NotificationTypeSecurity 安全提醒（登录异常、密码修改、2FA 变更等）
	NotificationTypeSecurity = "security"

	// NotificationTypeSwap Swap 交易相关（兑换成功、兑换失败等）
	NotificationTypeSwap = "swap"

	// NotificationTypeOnChain 链上事件（Web3 用户的链上交易通知）
	NotificationTypeOnChain = "onchain"
)

// 消息类型描述（用于日志记录或调试）
var NotificationTypeDescriptions = map[string]string{
	NotificationTypeTransfer: "转账通知",
	NotificationTypeDeposit:  "充值通知",
	NotificationTypeWithdraw: "提现通知",
	NotificationTypeSystem:   "系统公告",
	NotificationTypeSecurity: "安全提醒",
	NotificationTypeSwap:     "Swap交易",
	NotificationTypeOnChain:  "链上事件",
}

// IsValidRecipientType 验证接收者类型是否有效
func IsValidRecipientType(recipientType string) bool {
	return recipientType == NotificationRecipientWeb2 || recipientType == NotificationRecipientWeb3
}

// IsValidNotificationType 验证消息类型是否有效
func IsValidNotificationType(notificationType string) bool {
	_, ok := NotificationTypeDescriptions[notificationType]
	return ok
}
