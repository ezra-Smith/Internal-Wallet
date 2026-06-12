package errcode

import (
	"strconv"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorCode 统一业务错误码类型
// 错误码范围规划：
//   - 0:           成功
//   - 10000~10999: 通用错误
//   - 11000~11999: 认证/授权相关
//   - 12000~12999: 用户/RBAC/权限管理
//   - 13000~13999: 资产/币种管理
//   - 14000~14999: 账户/余额相关
//   - 15000~15999: 充值/提现相关
//   - 16000~16999: 交易/订单相关
//   - 17000~17999: 风控相关
//   - 20000~29999: 签名服务相关
//   - 30000~39999: 链同步服务相关
//   - 40000~49999: 兑换服务相关
type ErrorCode int

const (
	// ============================================
	// 成功 (0)
	// ============================================
	CodeSuccess ErrorCode = 0

	// ============================================
	// 通用错误 (10000~10999)
	// ============================================
	CodeInvalidParam      ErrorCode = 10001 // 参数错误
	CodeUnauthorized      ErrorCode = 10002 // 未授权
	CodeForbidden         ErrorCode = 10003 // 禁止访问
	CodeNotFound          ErrorCode = 10004 // 资源不存在
	CodeConflict          ErrorCode = 10005 // 资源冲突（已存在）
	CodeTooManyRequests   ErrorCode = 10006 // 请求过于频繁
	CodeInternalError     ErrorCode = 10007 // 内部错误
	CodeServiceUnavail    ErrorCode = 10008 // 服务不可用
	CodeDatabaseError     ErrorCode = 10009 // 数据库错误
	CodeCacheError        ErrorCode = 10010 // 缓存错误
	CodeTimeout           ErrorCode = 10011 // 请求超时
	CodeInvalidOperation  ErrorCode = 10012 // 无效操作
	CodeDataParseError    ErrorCode = 10013 // 数据解析错误
	CodeValidationFailed  ErrorCode = 10014 // 验证失败
	CodeResourceExhausted ErrorCode = 10015 // 资源耗尽

	// ============================================
	// 认证/授权相关 (11000~11999)
	// ============================================
	CodeAuthInvalidCredentials ErrorCode = 11001 // 用户名或密码错误
	CodeAuthAccountLocked      ErrorCode = 11002 // 账户已锁定
	CodeAuthCaptchaRequired    ErrorCode = 11003 // 需要验证码
	CodeAuthCaptchaInvalid     ErrorCode = 11004 // 验证码无效
	CodeAuth2FATokenExpired    ErrorCode = 11005 // 2FA 令牌已过期
	CodeAuth2FACodeInvalid     ErrorCode = 11006 // 2FA 验证码无效
	CodeAuth2FATooManyAttempts ErrorCode = 11007 // 2FA 尝试次数过多
	CodeAuthRefreshNotAllowed  ErrorCode = 11008 // 不允许刷新令牌
	CodeAuthTokenRevoked       ErrorCode = 11009 // 令牌已吊销
	CodeAuthPasswordChangeReq  ErrorCode = 11010 // 需要修改密码
	CodeAuthTwoFABindReq       ErrorCode = 11011 // 需要绑定2FA
	CodeAuthTokenExpired       ErrorCode = 11012 // 令牌已过期
	CodeAuthTokenInvalid       ErrorCode = 11013 // 令牌无效
	CodeAuthSessionExpired     ErrorCode = 11014 // 会话已过期
	CodeOldEmailCodeInvalid    ErrorCode = 11015 // 原邮箱验证码错误
	CodeNewEmailCodeInvalid    ErrorCode = 11016 // 新邮箱验证码错误
	CodeOldPhoneCodeInvalid    ErrorCode = 11017 // 原手机号验证码错误
	CodeNewPhoneCodeInvalid    ErrorCode = 11018 // 新手机号验证码错误
	CodePhoneAlreadyInUse      ErrorCode = 11019 // 手机号已被使用
	CodeBindPhoneCodeInvalid   ErrorCode = 11020 // 绑定手机验证码错误
	CodeAuthWalletInitReq      ErrorCode = 11021 // 需要初始化系统钱包

	// ============================================
	// 用户/RBAC/权限管理 (12000~12999)
	// ============================================
	CodeUserNotFound          ErrorCode = 12001 // 用户不存在
	CodeUserDisabled          ErrorCode = 12002 // 用户已禁用
	CodeCannotDisableSelf     ErrorCode = 12003 // 不能禁用自己
	CodeLastSuperAdmin        ErrorCode = 12004 // 最后一个超级管理员不能删除
	CodePasswordTooWeak       ErrorCode = 12005 // 密码强度不足
	CodePasswordMismatch      ErrorCode = 12006 // 密码不匹配
	CodePasswordRecentlyUsed  ErrorCode = 12007 // 密码最近使用过
	CodeRoleNotFound          ErrorCode = 12008 // 角色不存在
	CodePermissionDenied      ErrorCode = 12009 // 权限不足
	CodeMenuNotFound          ErrorCode = 12010 // 菜单不存在
	CodeEmailAlreadyExists    ErrorCode = 12011 // 邮箱已存在
	CodeUsernameAlreadyExists ErrorCode = 12012 // 用户名已存在
	CodeRoleCodeAlreadyExists ErrorCode = 12013 // 角色编码已存在
	CodeUserFrozen            ErrorCode = 12014 // 用户账户已冻结

	// ============================================
	// 资产/币种管理 (13000~13999)
	// ============================================
	CodeAssetNotFound        ErrorCode = 13001 // 资产不存在
	CodeAssetAlreadyExists   ErrorCode = 13002 // 资产已存在
	CodeAssetDisabled        ErrorCode = 13003 // 资产已禁用
	CodeInvalidAssetCode     ErrorCode = 13004 // 无效的资产编码
	CodeInvalidPrecision     ErrorCode = 13005 // 无效的精度
	CodeChainNotSupported    ErrorCode = 13006 // 链不支持
	CodeContractNotFound     ErrorCode = 13007 // 合约不存在
	CodeInvalidContractAddr  ErrorCode = 13008 // 无效的合约地址
	CodeAssetWithdrawDisable ErrorCode = 13009 // 资产禁止提现
	CodeAssetDepositDisable  ErrorCode = 13010 // 资产禁止充值
	CodeAssetSwapDisable     ErrorCode = 13011 // 资产禁止兑换

	// ============================================
	// 账户/余额相关 (14000~14999)
	// ============================================
	CodeAccountNotFound       ErrorCode = 14001 // 账户不存在
	CodeAccountDisabled       ErrorCode = 14002 // 账户已禁用
	CodeInsufficientBalance   ErrorCode = 14003 // 余额不足
	CodeInsufficientFrozen    ErrorCode = 14004 // 冻结余额不足
	CodeBalanceUpdateFailed   ErrorCode = 14005 // 余额更新失败
	CodeAccountAlreadyExists  ErrorCode = 14006 // 账户已存在
	CodeInvalidAmount         ErrorCode = 14007 // 无效的金额
	CodeAmountTooSmall        ErrorCode = 14008 // 金额过小
	CodeAmountTooLarge        ErrorCode = 14009 // 金额过大
	CodeAccountTypeMismatch   ErrorCode = 14010 // 账户类型不匹配
	CodeAccountFreezeFailed   ErrorCode = 14011 // 账户冻结失败
	CodeAccountUnfreezeFailed ErrorCode = 14012 // 账户解冻失败

	// ============================================
	// 充值/提现相关 (15000~15999)
	// ============================================
	CodeDepositNotFound       ErrorCode = 15001 // 充值记录不存在
	CodeDepositAlreadyExist   ErrorCode = 15002 // 充值记录已存在
	CodeDepositStatusInvalid  ErrorCode = 15003 // 充值状态无效
	CodeWithdrawNotFound      ErrorCode = 15004 // 提现记录不存在
	CodeWithdrawStatusInvalid ErrorCode = 15005 // 提现状态无效
	CodeWithdrawAmountLimit   ErrorCode = 15006 // 提现金额超限
	CodeWithdrawDailyLimit    ErrorCode = 15007 // 超过每日提现限额
	CodeWithdrawDisabled      ErrorCode = 15008 // 提现功能已禁用
	CodeDepositDisabled       ErrorCode = 15009 // 充值功能已禁用
	CodeInvalidAddress        ErrorCode = 15010 // 无效的地址
	CodeAddressNotWhitelisted ErrorCode = 15011 // 地址不在白名单
	CodeWithdrawNeedsAudit    ErrorCode = 15012 // 提现需要审核
	CodeWithdrawRejected      ErrorCode = 15013 // 提现已拒绝
	CodeWithdrawPending       ErrorCode = 15014 // 提现处理中
	CodeDepositPending        ErrorCode = 15015 // 充值确认中
	CodeInvalidNetwork        ErrorCode = 15016 // 无效的网络
	CodeNetworkFeeInsuffcient ErrorCode = 15017 // 网络费用不足
	CodeAddressAlreadyExists  ErrorCode = 15018 // 地址已存在

	// ============================================
	// 交易/订单相关 (16000~16999)
	// ============================================
	CodeOrderNotFound      ErrorCode = 16001 // 订单不存在
	CodeOrderStatusInvalid ErrorCode = 16002 // 订单状态无效
	CodeOrderAlreadyExist  ErrorCode = 16003 // 订单已存在
	CodeInvalidPrice       ErrorCode = 16004 // 无效的价格
	CodeInvalidQuantity    ErrorCode = 16005 // 无效的数量
	CodeOrderExpired       ErrorCode = 16006 // 订单已过期
	CodeOrderCancelled     ErrorCode = 16007 // 订单已取消
	CodeTradePairNotFound  ErrorCode = 16008 // 交易对不存在
	CodeTradePairDisabled  ErrorCode = 16009 // 交易对已禁用

	// ============================================
	// 风控相关 (17000~17999)
	// ============================================
	CodeRiskControlTriggered ErrorCode = 17001 // 触发风控规则
	CodeRiskLimitExceeded    ErrorCode = 17002 // 超过风控限额
	CodeRiskBlacklisted      ErrorCode = 17003 // 地址已列入黑名单
	CodeRiskSuspicious       ErrorCode = 17004 // 可疑交易
	CodeRiskManualReview     ErrorCode = 17005 // 需要人工审核

	// ============================================
	// 签名服务相关 (20000~20999)
	// ============================================
	CodeSignerInternalError    ErrorCode = 20001 // 签名服务内部错误
	CodeSignerSeedNotFound     ErrorCode = 20002 // Seed不存在
	CodeSignerSeedAlreadyExist ErrorCode = 20003 // Seed已存在
	CodeSignerSeedDecryptFail  ErrorCode = 20004 // Seed解密失败
	CodeSignerSeedInactive     ErrorCode = 20005 // Seed未激活
	CodeSignerInvalidPath      ErrorCode = 20006 // 派生路径无效
	CodeSignerPubkeyExportFail ErrorCode = 20007 // 公钥导出失败
	CodeSignerSignatureFailed  ErrorCode = 20008 // 签名失败
	CodeSignerDuplicateRequest ErrorCode = 20009 // 重复请求
	CodeSignerInvalidTxData    ErrorCode = 20010 // 交易数据无效
	CodeSignerAmountExceed     ErrorCode = 20011 // 金额超过限额
	CodeSignerApprovalRequired ErrorCode = 20012 // 需要审批
	CodeSignerChallengeExpired ErrorCode = 20013 // 挑战会话已过期
	CodeSignerVerifyFailed     ErrorCode = 20014 // 验证失败

	// ============================================
	// 链同步服务相关 (30000~30999)
	// ============================================
	CodeChainSyncNotReady    ErrorCode = 30001 // 链同步未就绪
	CodeChainBlockNotFound   ErrorCode = 30002 // 区块不存在
	CodeChainTxNotFound      ErrorCode = 30003 // 交易不存在
	CodeChainRPCError        ErrorCode = 30004 // 链RPC错误
	CodeChainNetworkError    ErrorCode = 30005 // 链网络错误
	CodeChainConfirmPending  ErrorCode = 30006 // 等待区块确认
	CodeChainBroadcastFailed ErrorCode = 30007 // 广播失败
	CodeAccountNotActivated  ErrorCode = 30008 // 账户未激活
	CodeInsufficientGas      ErrorCode = 30009 // Gas 不足

	// ============================================
	// 兑换服务相关 (40000~40999)
	// ============================================
	CodeSwapQuoteFailed       ErrorCode = 40001 // 获取报价失败
	CodeSwapPairNotSupported  ErrorCode = 40002 // 兑换对不支持
	CodeSwapSlippageExceeded  ErrorCode = 40003 // 滑点超限
	CodeSwapAmountInvalid     ErrorCode = 40004 // 兑换金额无效
	CodeSwapRouteNotFound     ErrorCode = 40005 // 兑换路由不存在
	CodeSwapExecutionFailed   ErrorCode = 40006 // 兑换执行失败
	CodeSwapProviderError     ErrorCode = 40007 // 兑换提供商错误
	CodeSwapMinAmountNotMet   ErrorCode = 40008 // 未达到最小兑换金额
	CodeSwapMaxAmountExceeded ErrorCode = 40009 // 超过最大兑换金额
)

// ErrorMessage 错误码对应的默认消息（英文）
var ErrorMessage = map[ErrorCode]string{
	CodeSuccess: "success",

	// 通用错误
	CodeInvalidParam:      "invalid parameter",
	CodeUnauthorized:      "unauthorized",
	CodeForbidden:         "forbidden",
	CodeNotFound:          "resource not found",
	CodeConflict:          "resource already exists",
	CodeTooManyRequests:   "too many requests",
	CodeInternalError:     "internal error",
	CodeServiceUnavail:    "service unavailable",
	CodeDatabaseError:     "database error",
	CodeCacheError:        "cache error",
	CodeTimeout:           "request timeout",
	CodeInvalidOperation:  "invalid operation",
	CodeDataParseError:    "data parse error",
	CodeValidationFailed:  "validation failed",
	CodeResourceExhausted: "resource exhausted",

	// 认证/授权
	CodeAuthInvalidCredentials: "invalid username or password",
	CodeAuthAccountLocked:      "account is locked",
	CodeAuthCaptchaRequired:    "captcha required",
	CodeAuthCaptchaInvalid:     "invalid captcha",
	CodeAuth2FATokenExpired:    "2FA token expired",
	CodeAuth2FACodeInvalid:     "invalid 2FA code",
	CodeAuth2FATooManyAttempts: "too many 2FA attempts",
	CodeAuthRefreshNotAllowed:  "token refresh not allowed",
	CodeAuthTokenRevoked:       "token has been revoked",
	CodeAuthPasswordChangeReq:  "password change required",
	CodeAuthTwoFABindReq:       "2FA binding required",
	CodeAuthTokenExpired:       "token expired",
	CodeAuthTokenInvalid:       "invalid token",
	CodeAuthSessionExpired:     "session expired",
	CodeOldEmailCodeInvalid:    "invalid old email verification code",
	CodeNewEmailCodeInvalid:    "invalid new email verification code",
	CodeOldPhoneCodeInvalid:    "invalid old phone verification code",
	CodeNewPhoneCodeInvalid:    "invalid new phone verification code",
	CodePhoneAlreadyInUse:      "phone number already in use",
	CodeBindPhoneCodeInvalid:   "invalid phone verification code",

	// 用户/RBAC
	CodeUserNotFound:          "user not found",
	CodeUserDisabled:          "user is disabled",
	CodeCannotDisableSelf:     "cannot disable yourself",
	CodeLastSuperAdmin:        "cannot remove last super admin",
	CodePasswordTooWeak:       "password is too weak",
	CodePasswordMismatch:      "password mismatch",
	CodePasswordRecentlyUsed:  "password recently used",
	CodeUserFrozen:            "account frozen",
	CodeRoleNotFound:          "role not found",
	CodePermissionDenied:      "permission denied",
	CodeMenuNotFound:          "menu not found",
	CodeEmailAlreadyExists:    "email already exists",
	CodeUsernameAlreadyExists: "username already exists",
	CodeRoleCodeAlreadyExists: "role code already exists",

	// 资产/币种
	CodeAssetNotFound:        "asset not found",
	CodeAssetAlreadyExists:   "asset already exists",
	CodeAssetDisabled:        "asset is disabled",
	CodeInvalidAssetCode:     "invalid asset code",
	CodeInvalidPrecision:     "invalid precision",
	CodeChainNotSupported:    "chain not supported",
	CodeContractNotFound:     "contract not found",
	CodeInvalidContractAddr:  "invalid contract address",
	CodeAssetWithdrawDisable: "asset withdrawal disabled",
	CodeAssetDepositDisable:  "asset deposit disabled",
	CodeAssetSwapDisable:     "asset swap disabled",

	// 账户/余额
	CodeAccountNotFound:       "account not found",
	CodeAccountDisabled:       "account is disabled",
	CodeInsufficientBalance:   "insufficient balance",
	CodeInsufficientFrozen:    "insufficient frozen balance",
	CodeBalanceUpdateFailed:   "balance update failed",
	CodeAccountAlreadyExists:  "account already exists",
	CodeInvalidAmount:         "invalid amount",
	CodeAmountTooSmall:        "amount too small",
	CodeAmountTooLarge:        "amount too large",
	CodeAccountTypeMismatch:   "account type mismatch",
	CodeAccountFreezeFailed:   "account freeze failed",
	CodeAccountUnfreezeFailed: "account unfreeze failed",

	// 充值/提现
	CodeDepositNotFound:       "deposit not found",
	CodeDepositAlreadyExist:   "deposit already exists",
	CodeDepositStatusInvalid:  "invalid deposit status",
	CodeWithdrawNotFound:      "withdrawal not found",
	CodeWithdrawStatusInvalid: "invalid withdrawal status",
	CodeWithdrawAmountLimit:   "withdrawal amount limit exceeded",
	CodeWithdrawDailyLimit:    "daily withdrawal limit exceeded",
	CodeWithdrawDisabled:      "withdrawal is disabled",
	CodeDepositDisabled:       "deposit is disabled",
	CodeInvalidAddress:        "invalid address",
	CodeAddressNotWhitelisted: "address not whitelisted",
	CodeWithdrawNeedsAudit:    "withdrawal requires audit",
	CodeWithdrawRejected:      "withdrawal rejected",
	CodeWithdrawPending:       "withdrawal pending",
	CodeDepositPending:        "deposit pending confirmation",
	CodeInvalidNetwork:        "invalid network",
	CodeNetworkFeeInsuffcient: "insufficient network fee",
	CodeAddressAlreadyExists:  "address already exists in your address book",

	// 交易/订单
	CodeOrderNotFound:      "order not found",
	CodeOrderStatusInvalid: "invalid order status",
	CodeOrderAlreadyExist:  "order already exists",
	CodeInvalidPrice:       "invalid price",
	CodeInvalidQuantity:    "invalid quantity",
	CodeOrderExpired:       "order expired",
	CodeOrderCancelled:     "order cancelled",
	CodeTradePairNotFound:  "trade pair not found",
	CodeTradePairDisabled:  "trade pair disabled",

	// 风控
	CodeRiskControlTriggered: "risk control triggered",
	CodeRiskLimitExceeded:    "risk limit exceeded",
	CodeRiskBlacklisted:      "address is blacklisted",
	CodeRiskSuspicious:       "suspicious transaction",
	CodeRiskManualReview:     "manual review required",

	// 签名服务
	CodeSignerInternalError:    "signer internal error",
	CodeSignerSeedNotFound:     "seed not found",
	CodeSignerSeedAlreadyExist: "seed already exists",
	CodeSignerSeedDecryptFail:  "seed decryption failed",
	CodeSignerSeedInactive:     "seed is inactive",
	CodeSignerInvalidPath:      "invalid derivation path",
	CodeSignerPubkeyExportFail: "public key export failed",
	CodeSignerSignatureFailed:  "signature failed",
	CodeSignerDuplicateRequest: "duplicate request",
	CodeSignerInvalidTxData:    "invalid transaction data",
	CodeSignerAmountExceed:     "amount exceeds limit",
	CodeSignerApprovalRequired: "approval required",
	CodeSignerChallengeExpired: "challenge session expired",
	CodeSignerVerifyFailed:     "verification failed",

	// 链同步服务
	CodeChainSyncNotReady:    "chain sync not ready",
	CodeChainBlockNotFound:   "block not found",
	CodeChainTxNotFound:      "transaction not found",
	CodeChainRPCError:        "chain RPC error",
	CodeChainNetworkError:    "chain network error",
	CodeChainConfirmPending:  "waiting for block confirmation",
	CodeChainBroadcastFailed: "broadcast failed",
	CodeAccountNotActivated:  "account not activated, please receive native tokens (TRX/BNB/ETH) first",
	CodeInsufficientGas:      "insufficient gas fee, please top up native tokens first",

	// 兑换服务
	CodeSwapQuoteFailed:       "quote failed",
	CodeSwapPairNotSupported:  "swap pair not supported",
	CodeSwapSlippageExceeded:  "slippage exceeded",
	CodeSwapAmountInvalid:     "invalid swap amount",
	CodeSwapRouteNotFound:     "swap route not found",
	CodeSwapExecutionFailed:   "swap execution failed",
	CodeSwapProviderError:     "swap provider error",
	CodeSwapMinAmountNotMet:   "minimum amount not met",
	CodeSwapMaxAmountExceeded: "maximum amount exceeded",
}

// ErrorMessageZH 错误码对应的默认消息（中文）
var ErrorMessageZH = map[ErrorCode]string{
	CodeSuccess: "成功",

	// 通用错误
	CodeInvalidParam:      "参数错误",
	CodeUnauthorized:      "未授权",
	CodeForbidden:         "禁止访问",
	CodeNotFound:          "资源不存在",
	CodeConflict:          "资源已存在",
	CodeTooManyRequests:   "请求过于频繁",
	CodeInternalError:     "内部错误",
	CodeServiceUnavail:    "服务不可用",
	CodeDatabaseError:     "数据库错误",
	CodeCacheError:        "缓存错误",
	CodeTimeout:           "请求超时",
	CodeInvalidOperation:  "无效操作",
	CodeDataParseError:    "数据解析错误",
	CodeValidationFailed:  "验证失败",
	CodeResourceExhausted: "资源耗尽",

	// 认证/授权
	CodeAuthInvalidCredentials: "用户名或密码错误",
	CodeAuthAccountLocked:      "账户已锁定",
	CodeAuthCaptchaRequired:    "需要验证码",
	CodeAuthCaptchaInvalid:     "验证码无效",
	CodeAuth2FATokenExpired:    "2FA令牌已过期",
	CodeAuth2FACodeInvalid:     "2FA验证码无效",
	CodeAuth2FATooManyAttempts: "2FA尝试次数过多",
	CodeAuthRefreshNotAllowed:  "不允许刷新令牌",
	CodeAuthTokenRevoked:       "令牌已吊销",
	CodeAuthPasswordChangeReq:  "需要修改密码",
	CodeAuthTwoFABindReq:       "需要绑定2FA",
	CodeAuthTokenExpired:       "令牌已过期",
	CodeAuthTokenInvalid:       "令牌无效",
	CodeAuthSessionExpired:     "会话已过期",
	CodeOldEmailCodeInvalid:    "原邮箱验证码错误",
	CodeNewEmailCodeInvalid:    "新邮箱验证码错误",
	CodeOldPhoneCodeInvalid:    "原手机号验证码错误",
	CodeNewPhoneCodeInvalid:    "新手机号验证码错误",
	CodePhoneAlreadyInUse:      "该手机号已被使用",
	CodeBindPhoneCodeInvalid:   "手机验证码错误",

	// 用户/RBAC
	CodeUserNotFound:          "用户不存在",
	CodeUserDisabled:          "用户已禁用",
	CodeCannotDisableSelf:     "不能禁用自己",
	CodeLastSuperAdmin:        "不能删除最后一个超级管理员",
	CodeUserFrozen:            "账户已冻结",
	CodePasswordTooWeak:       "密码强度不足",
	CodePasswordMismatch:      "密码不匹配",
	CodePasswordRecentlyUsed:  "密码最近使用过",
	CodeRoleNotFound:          "角色不存在",
	CodePermissionDenied:      "权限不足",
	CodeMenuNotFound:          "菜单不存在",
	CodeEmailAlreadyExists:    "邮箱已存在",
	CodeUsernameAlreadyExists: "用户名已存在",
	CodeRoleCodeAlreadyExists: "角色编码已存在",

	// 资产/币种
	CodeAssetNotFound:        "资产不存在",
	CodeAssetAlreadyExists:   "资产已存在",
	CodeAssetDisabled:        "资产已禁用",
	CodeInvalidAssetCode:     "无效的资产编码",
	CodeInvalidPrecision:     "无效的精度",
	CodeChainNotSupported:    "链不支持",
	CodeContractNotFound:     "合约不存在",
	CodeInvalidContractAddr:  "无效的合约地址",
	CodeAssetWithdrawDisable: "资产禁止提现",
	CodeAssetDepositDisable:  "资产禁止充值",
	CodeAssetSwapDisable:     "资产禁止兑换",

	// 账户/余额
	CodeAccountNotFound:       "账户不存在",
	CodeAccountDisabled:       "账户已禁用",
	CodeInsufficientBalance:   "余额不足",
	CodeInsufficientFrozen:    "冻结余额不足",
	CodeBalanceUpdateFailed:   "余额更新失败",
	CodeAccountAlreadyExists:  "账户已存在",
	CodeInvalidAmount:         "无效的金额",
	CodeAmountTooSmall:        "金额过小",
	CodeAmountTooLarge:        "金额过大",
	CodeAccountTypeMismatch:   "账户类型不匹配",
	CodeAccountFreezeFailed:   "账户冻结失败",
	CodeAccountUnfreezeFailed: "账户解冻失败",

	// 充值/提现
	CodeDepositNotFound:       "充值记录不存在",
	CodeDepositAlreadyExist:   "充值记录已存在",
	CodeDepositStatusInvalid:  "充值状态无效",
	CodeWithdrawNotFound:      "提现记录不存在",
	CodeWithdrawStatusInvalid: "提现状态无效",
	CodeWithdrawAmountLimit:   "提现金额超限",
	CodeWithdrawDailyLimit:    "超过每日提现限额",
	CodeWithdrawDisabled:      "提现功能已禁用",
	CodeDepositDisabled:       "充值功能已禁用",
	CodeInvalidAddress:        "无效的地址",
	CodeAddressNotWhitelisted: "地址不在白名单",
	CodeWithdrawNeedsAudit:    "提现需要审核",
	CodeWithdrawRejected:      "提现已拒绝",
	CodeWithdrawPending:       "提现处理中",
	CodeDepositPending:        "充值确认中",
	CodeInvalidNetwork:        "无效的网络",
	CodeNetworkFeeInsuffcient: "网络费用不足",
	CodeAddressAlreadyExists:  "该地址已存在于您的地址簿中",

	// 交易/订单
	CodeOrderNotFound:      "订单不存在",
	CodeOrderStatusInvalid: "订单状态无效",
	CodeOrderAlreadyExist:  "订单已存在",
	CodeInvalidPrice:       "无效的价格",
	CodeInvalidQuantity:    "无效的数量",
	CodeOrderExpired:       "订单已过期",
	CodeOrderCancelled:     "订单已取消",
	CodeTradePairNotFound:  "交易对不存在",
	CodeTradePairDisabled:  "交易对已禁用",

	// 风控
	CodeRiskControlTriggered: "触发风控规则",
	CodeRiskLimitExceeded:    "超过风控限额",
	CodeRiskBlacklisted:      "地址已列入黑名单",
	CodeRiskSuspicious:       "可疑交易",
	CodeRiskManualReview:     "需要人工审核",

	// 签名服务
	CodeSignerInternalError:    "签名服务内部错误",
	CodeSignerSeedNotFound:     "Seed不存在",
	CodeSignerSeedAlreadyExist: "Seed已存在",
	CodeSignerSeedDecryptFail:  "Seed解密失败",
	CodeSignerSeedInactive:     "Seed未激活",
	CodeSignerInvalidPath:      "派生路径无效",
	CodeSignerPubkeyExportFail: "公钥导出失败",
	CodeSignerSignatureFailed:  "签名失败",
	CodeSignerDuplicateRequest: "重复请求",
	CodeSignerInvalidTxData:    "交易数据无效",
	CodeSignerAmountExceed:     "金额超过限额",
	CodeSignerApprovalRequired: "需要审批",
	CodeSignerChallengeExpired: "挑战会话已过期",
	CodeSignerVerifyFailed:     "验证失败",

	// 链同步服务
	CodeChainSyncNotReady:    "链同步未就绪",
	CodeChainBlockNotFound:   "区块不存在",
	CodeChainTxNotFound:      "交易不存在",
	CodeChainRPCError:        "链RPC错误",
	CodeChainNetworkError:    "链网络错误",
	CodeChainConfirmPending:  "等待区块确认",
	CodeChainBroadcastFailed: "广播失败",
	CodeAccountNotActivated:  "账户未激活，请先接收原生代币（TRX/BNB/ETH）激活账户",
	CodeInsufficientGas:      "Gas费不足，请先充值原生代币作为手续费",

	// 兑换服务
	CodeSwapQuoteFailed:       "获取报价失败",
	CodeSwapPairNotSupported:  "兑换对不支持",
	CodeSwapSlippageExceeded:  "滑点超限",
	CodeSwapAmountInvalid:     "兑换金额无效",
	CodeSwapRouteNotFound:     "兑换路由不存在",
	CodeSwapExecutionFailed:   "兑换执行失败",
	CodeSwapProviderError:     "兑换提供商错误",
	CodeSwapMinAmountNotMet:   "未达到最小兑换金额",
	CodeSwapMaxAmountExceeded: "超过最大兑换金额",
}

// Message 获取错误码对应的英文消息
func (c ErrorCode) Message() string {
	if msg, ok := ErrorMessage[c]; ok {
		return msg
	}
	return "unknown error"
}

// MessageZH 获取错误码对应的中文消息
func (c ErrorCode) MessageZH() string {
	if msg, ok := ErrorMessageZH[c]; ok {
		return msg
	}
	return "未知错误"
}

// Int 转换为int
func (c ErrorCode) Int() int {
	return int(c)
}

// IsSuccess 判断是否成功
func (c ErrorCode) IsSuccess() bool {
	return c == CodeSuccess
}

// ========================
// gRPC 错误创建辅助函数
// ========================

// New 创建带 ErrorInfo/BadRequest details 的 gRPC error
// grpcCode: gRPC 状态码
// httpStatus: 目标 HTTP 状态码（由 api-gateway 根据 metadata 覆盖）
// bizCode: 业务错误码
// reason: 错误原因标识（用于 i18n key）
// message: 错误消息
// fieldViolations: 字段校验错误
func New(grpcCode codes.Code, httpStatus int, bizCode ErrorCode, reason string, message string, fieldViolations map[string]string) error {
	st := status.New(grpcCode, message)

	meta := map[string]string{
		"code":        strconv.Itoa(int(bizCode)),
		"http_status": strconv.Itoa(httpStatus),
	}

	info := &errdetails.ErrorInfo{
		Reason:   reason,
		Metadata: meta,
	}
	withDetails, err := st.WithDetails(info)
	if err != nil {
		return st.Err()
	}

	if len(fieldViolations) > 0 {
		br := &errdetails.BadRequest{}
		for field, desc := range fieldViolations {
			br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field:       field,
				Description: desc,
			})
		}
		withDetails, _ = withDetails.WithDetails(br)
	}

	return withDetails.Err()
}

// NewWithMetadata 创建带额外元数据的 gRPC error
func NewWithMetadata(grpcCode codes.Code, httpStatus int, bizCode ErrorCode, reason string, message string, fieldViolations map[string]string, extraMetadata map[string]string) error {
	st := status.New(grpcCode, message)

	meta := map[string]string{
		"code":        strconv.Itoa(int(bizCode)),
		"http_status": strconv.Itoa(httpStatus),
	}
	for k, v := range extraMetadata {
		if k == "code" || k == "http_status" {
			continue
		}
		if k != "" && v != "" {
			meta[k] = v
		}
	}

	info := &errdetails.ErrorInfo{
		Reason:   reason,
		Metadata: meta,
	}
	withDetails, err := st.WithDetails(info)
	if err != nil {
		return st.Err()
	}

	if len(fieldViolations) > 0 {
		br := &errdetails.BadRequest{}
		for field, desc := range fieldViolations {
			br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field:       field,
				Description: desc,
			})
		}
		withDetails, _ = withDetails.WithDetails(br)
	}

	return withDetails.Err()
}

// ========================
// 快捷错误创建函数
// ========================
//
// 设计说明：
// - 业务错误统一返回 HTTP 200，前端通过 code 字段判断是否成功
// - 只有真正的服务器内部错误（panic、服务不可达等）才返回 HTTP 5xx
// - 这样前端处理更简单，避免在 HTTP 层和业务层分别处理错误

// InvalidParam 创建参数错误
func InvalidParam(reason string, message string, fields map[string]string) error {
	return New(codes.InvalidArgument, 200, CodeInvalidParam, reason, message, fields)
}

// Unauthorized 创建未授权错误
func Unauthorized(reason string, message string) error {
	return New(codes.Unauthenticated, 200, CodeUnauthorized, reason, message, nil)
}

// Forbidden 创建禁止访问错误
func Forbidden(reason string, message string) error {
	return New(codes.PermissionDenied, 200, CodeForbidden, reason, message, nil)
}

// NotFound 创建资源不存在错误
func NotFound(reason string, message string) error {
	return New(codes.NotFound, 200, CodeNotFound, reason, message, nil)
}

// Conflict 创建资源冲突错误
func Conflict(reason string, message string) error {
	return New(codes.AlreadyExists, 200, CodeConflict, reason, message, nil)
}

// Internal 创建内部错误（保持 500，因为这是真正的服务器异常）
func Internal(reason string, message string) error {
	return New(codes.Internal, 500, CodeInternalError, reason, message, nil)
}

// TooManyRequests 创建请求频率限制错误
func TooManyRequests(reason string, message string) error {
	return New(codes.ResourceExhausted, 200, CodeTooManyRequests, reason, message, nil)
}

// ServiceUnavailable 创建服务不可用错误（保持 503，因为这是服务层面的异常）
func ServiceUnavailable(reason string, message string) error {
	return New(codes.Unavailable, 503, CodeServiceUnavail, reason, message, nil)
}
