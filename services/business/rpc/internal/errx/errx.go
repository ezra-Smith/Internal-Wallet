package errx

import (
	"fmt"
	"internalwallet/common/errcode"

	"google.golang.org/grpc/codes"
)

// 错误码别名，使用 common/errcode 中的统一定义
const (
	// 通用
	CodeInvalidParam  = errcode.CodeInvalidParam
	CodeUnauthorized  = errcode.CodeUnauthorized
	CodeForbidden     = errcode.CodeForbidden
	CodeNotFound      = errcode.CodeNotFound
	CodeConflict      = errcode.CodeConflict
	CodeTooManyReq    = errcode.CodeTooManyRequests
	CodeInternalError = errcode.CodeInternalError

	// Auth
	CodeAuthInvalidCredentials = errcode.CodeAuthInvalidCredentials
	CodeAuthAccountLocked      = errcode.CodeAuthAccountLocked
	CodeAuthCaptchaRequired    = errcode.CodeAuthCaptchaRequired
	CodeAuthCaptchaInvalid     = errcode.CodeAuthCaptchaInvalid
	CodeAuth2FATokenExpired    = errcode.CodeAuth2FATokenExpired
	CodeAuth2FACodeInvalid     = errcode.CodeAuth2FACodeInvalid
	CodeAuth2FATooManyAttempts = errcode.CodeAuth2FATooManyAttempts
	CodeAuthTokenExpired       = errcode.CodeAuthTokenExpired
	CodeAuthTokenInvalid       = errcode.CodeAuthTokenInvalid

	// User
	CodeUserNotFound     = errcode.CodeUserNotFound
	CodeUserDisabled     = errcode.CodeUserDisabled
	CodePasswordTooWeak  = errcode.CodePasswordTooWeak
	CodePasswordMismatch = errcode.CodePasswordMismatch

	// Asset
	CodeAssetNotFound        = errcode.CodeAssetNotFound
	CodeAssetDisabled        = errcode.CodeAssetDisabled
	CodeAssetWithdrawDisable = errcode.CodeAssetWithdrawDisable
	CodeAssetDepositDisable  = errcode.CodeAssetDepositDisable

	// Account/Balance
	CodeAccountNotFound     = errcode.CodeAccountNotFound
	CodeInsufficientBalance = errcode.CodeInsufficientBalance
	CodeInvalidAmount       = errcode.CodeInvalidAmount
	CodeAmountTooSmall      = errcode.CodeAmountTooSmall

	// Deposit/Withdraw
	CodeDepositNotFound    = errcode.CodeDepositNotFound
	CodeWithdrawNotFound   = errcode.CodeWithdrawNotFound
	CodeWithdrawDisabled   = errcode.CodeWithdrawDisabled
	CodeDepositDisabled    = errcode.CodeDepositDisabled
	CodeInvalidAddress     = errcode.CodeInvalidAddress
	CodeWithdrawNeedsAudit = errcode.CodeWithdrawNeedsAudit
	CodeInvalidNetwork     = errcode.CodeInvalidNetwork

	// Risk
	CodeRiskControlTriggered = errcode.CodeRiskControlTriggered
	CodeRiskBlacklisted      = errcode.CodeRiskBlacklisted
)

// ========================
// 快捷错误创建函数
// ========================
//
// 设计说明：
// - 业务错误统一返回 HTTP 200，前端通过响应体中的 code 字段判断是否成功
// - 只有真正的服务器内部错误（panic、服务不可达等）才返回 HTTP 5xx

// InvalidParam 参数错误
func InvalidParam(message string) error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_PARAM", message, nil)
}

// InvalidParamWithFields 参数错误（带字段信息）
func InvalidParamWithFields(message string, fields map[string]string) error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_PARAM", message, fields)
}

// InvalidPhoneFormat 手机号格式无效
func InvalidPhoneFormat() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_PHONE_FORMAT", "invalid phone format", nil)
}

// PhoneRequired 手机号必填
func PhoneRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "PHONE_REQUIRED", "phone is required", nil)
}

// InvalidEmailFormat 邮箱格式无效
func InvalidEmailFormat() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_EMAIL_FORMAT", "invalid email format", nil)
}

// EmailRequired 邮箱必填
func EmailRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "EMAIL_REQUIRED", "email is required", nil)
}

// Unauthorized 未授权
func Unauthorized(message string) error {
	return errcode.New(codes.Unauthenticated, 200, errcode.CodeUnauthorized, "UNAUTHORIZED", message, nil)
}

// Forbidden 禁止访问
func Forbidden(message string) error {
	return errcode.New(codes.PermissionDenied, 200, errcode.CodeForbidden, "FORBIDDEN", message, nil)
}

// AccountFrozen 账户已被冻结
func AccountFrozen() error {
	return errcode.New(codes.PermissionDenied, 200, errcode.CodeForbidden, "ACCOUNT_FROZEN", "account frozen", nil)
}

// AccountTerminated 账户已被终止
func AccountTerminated() error {
	return errcode.New(codes.PermissionDenied, 200, errcode.CodeForbidden, "ACCOUNT_TERMINATED", "account terminated", nil)
}

// NotFound 资源不存在
func NotFound(message string) error {
	return errcode.New(codes.NotFound, 200, errcode.CodeNotFound, "NOT_FOUND", message, nil)
}

// Conflict 资源冲突
func Conflict(message string) error {
	return errcode.New(codes.AlreadyExists, 200, errcode.CodeConflict, "CONFLICT", message, nil)
}

// Internal 内部错误（保持 500，因为这是真正的服务器异常）
func Internal(message string) error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "INTERNAL", message, nil)
}

// TooManyRequests 请求频率限制
func TooManyRequests(message string) error {
	return errcode.New(codes.ResourceExhausted, 200, errcode.CodeTooManyRequests, "TOO_MANY_REQUESTS", message, nil)
}

// ========================
// 业务特定错误（全部返回 HTTP 200）
// ========================

// UserNotFound 用户不存在
func UserNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeUserNotFound, "USER_NOT_FOUND", "user not found", nil)
}

// InvalidUser 无效用户
func InvalidUser() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_USER", "invalid user", nil)
}

// InvalidCode 验证码无效
func InvalidCode(codeType string) error {
	reason := "INVALID_CODE"
	message := "verification code is incorrect"
	if codeType == "sms" {
		reason = "INVALID_SMS_CODE"
		message = "SMS verification code is incorrect"
	} else if codeType == "email" {
		reason = "INVALID_EMAIL_CODE"
		message = "Email verification code is incorrect"
	}
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeAuthCaptchaInvalid, reason, message, nil)
}

// CaptchaRequired 需要验证码
func CaptchaRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeAuthCaptchaRequired, "AUTH_CAPTCHA_REQUIRED", "captcha required", nil)
}

// CaptchaInvalid 验证码无效
func CaptchaInvalid() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeAuthCaptchaInvalid, "AUTH_CAPTCHA_INVALID", "captcha invalid", nil)
}

// CaptchaCancelled 验证码已取消（用户主动关闭）
func CaptchaCancelled() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeAuthCaptchaInvalid, "AUTH_CAPTCHA_CANCELLED", "captcha cancelled", nil)
}

// VerifyFailed 验证失败
func VerifyFailed(message string) error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeValidationFailed, "VERIFY_FAILED", message, nil)
}

// PhoneAlreadyBound 手机号已绑定
func PhoneAlreadyBound() error {
	return errcode.New(codes.AlreadyExists, 200, errcode.CodeConflict, "PHONE_ALREADY_BOUND", "phone already bound, please use change phone", nil)
}

// PhoneAlreadyInUse 手机号已被使用
func PhoneAlreadyInUse() error {
	return errcode.New(codes.AlreadyExists, 200, errcode.CodePhoneAlreadyInUse, "PHONE_ALREADY_IN_USE", errcode.CodePhoneAlreadyInUse.Message(), nil)
}

// EmailAlreadyBound 邮箱已绑定
func EmailAlreadyBound() error {
	return errcode.New(codes.AlreadyExists, 200, errcode.CodeConflict, "EMAIL_ALREADY_BOUND", "email already bound, please use change email", nil)
}

// EmailAlreadyInUse 邮箱已被使用
func EmailAlreadyInUse() error {
	return errcode.New(codes.AlreadyExists, 200, errcode.CodeEmailAlreadyExists, "EMAIL_ALREADY_IN_USE", errcode.CodeEmailAlreadyExists.Message(), nil)
}

// NoPhoneBound 未绑定手机号
func NoPhoneBound() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeNotFound, "NO_PHONE_BOUND", "no phone bound", nil)
}

// NoEmailBound 未绑定邮箱
func NoEmailBound() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeNotFound, "NO_EMAIL_BOUND", "no email bound", nil)
}

// GoogleAuthNotBound 未绑定谷歌验证器
func GoogleAuthNotBound() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeNotFound, "GOOGLE_AUTH_NOT_BOUND", "google auth not bound", nil)
}

// InvalidPassword 密码错误
func InvalidPassword() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodePasswordMismatch, "INVALID_PASSWORD", "invalid password", nil)
}

// InvalidGoogleAuthCode 谷歌验证码错误
func InvalidGoogleAuthCode() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeAuth2FACodeInvalid, "INVALID_GOOGLE_AUTH_CODE", "invalid google auth code", nil)
}

// InvalidPhone 手机号格式无效
func InvalidPhone() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_PHONE", "invalid phone number format", nil)
}

// PhoneSameAsOld 新旧手机号相同
func PhoneSameAsOld() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "PHONE_SAME_AS_OLD", "new phone cannot be the same as old phone", nil)
}

// InvalidCountryCode 国家代码无效
func InvalidCountryCode() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_COUNTRY_CODE", "invalid country code", nil)
}

// UnsupportedCountryCode 不支持的国家代码
func UnsupportedCountryCode() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "UNSUPPORTED_COUNTRY_CODE", "unsupported country code", nil)
}

// VerifyOldPhoneCodeFailed 验证原手机验证码失败
func VerifyOldPhoneCodeFailed() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeValidationFailed, "VERIFY_OLD_PHONE_CODE_FAILED", "verify old phone code failed", nil)
}

// VerifyNewPhoneCodeFailed 验证新手机验证码失败
func VerifyNewPhoneCodeFailed() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeValidationFailed, "VERIFY_NEW_PHONE_CODE_FAILED", "verify new phone code failed", nil)
}

// InvalidOldPhoneCode 原手机验证码错误
func InvalidOldPhoneCode() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeOldPhoneCodeInvalid, "INVALID_OLD_PHONE_CODE", errcode.CodeOldPhoneCodeInvalid.Message(), nil)
}

// InvalidNewPhoneCode 新手机验证码错误
func InvalidNewPhoneCode() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeNewPhoneCodeInvalid, "INVALID_NEW_PHONE_CODE", errcode.CodeNewPhoneCodeInvalid.Message(), nil)
}

// InvalidBindPhoneCode 绑定手机验证码错误
func InvalidBindPhoneCode() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeBindPhoneCodeInvalid, "INVALID_BIND_PHONE_CODE", errcode.CodeBindPhoneCodeInvalid.Message(), nil)
}

// InvalidOldEmailCode 原邮箱验证码错误
func InvalidOldEmailCode() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeOldEmailCodeInvalid, "INVALID_OLD_EMAIL_CODE", errcode.CodeOldEmailCodeInvalid.Message(), nil)
}

// InvalidNewEmailCode 新邮箱验证码错误
func InvalidNewEmailCode() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeNewEmailCodeInvalid, "INVALID_NEW_EMAIL_CODE", errcode.CodeNewEmailCodeInvalid.Message(), nil)
}

// AssetNotAvailable 资产不可用
func AssetNotAvailable() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeAssetDisabled, "ASSET_NOT_AVAILABLE", "asset not available", nil)
}

// NetworkNotAvailable 网络不可用
func NetworkNotAvailable() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeInvalidNetwork, "NETWORK_NOT_AVAILABLE", "network not available", nil)
}

// WithdrawDisabled 提现已禁用
func WithdrawDisabled() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeWithdrawDisabled, "WITHDRAW_DISABLED", "withdraw disabled", nil)
}

// DepositDisabled 充值已禁用
func DepositDisabled() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeDepositDisabled, "DEPOSIT_DISABLED", "deposit disabled", nil)
}

// InsufficientBalance 余额不足
func InsufficientBalance() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeInsufficientBalance, "INSUFFICIENT_BALANCE", "余额不足", nil)
}

// AmountBelowMinimum 金额低于最小值
func AmountBelowMinimum() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeAmountTooSmall, "AMOUNT_BELOW_MINIMUM", "amount below minimum", nil)
}

// MinWithdrawAmountNotConfigured 提现最小金额未配置（服务端配置缺失）
func MinWithdrawAmountNotConfigured(assetCode, chainCode string) error {
	meta := map[string]string{}
	if assetCode != "" {
		meta["asset_code"] = assetCode
	}
	if chainCode != "" {
		meta["chain_code"] = chainCode
	}
	return errcode.NewWithMetadata(codes.FailedPrecondition, 200, errcode.CodeInvalidOperation, "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED", "min withdraw amount not configured", nil, meta)
}

// AmountMustExceedFee 提现金额必须大于手续费（内扣模式）
func AmountMustExceedFee(amount, fee string) error {
	meta := map[string]string{}
	if amount != "" {
		meta["amount"] = amount
	}
	if fee != "" {
		meta["fee"] = fee
	}
	return errcode.NewWithMetadata(codes.InvalidArgument, 200, errcode.CodeAmountTooSmall, "AMOUNT_MUST_EXCEED_FEE", "amount must exceed fee", nil, meta)
}

// AmountBelowMinPlusFee 金额低于（最小提现金额 + 手续费）（内扣模式）
func AmountBelowMinPlusFee(amount, fee, minWithdraw, requiredAmount, netAmount string) error {
	meta := map[string]string{}
	if amount != "" {
		meta["amount"] = amount
	}
	if fee != "" {
		meta["fee"] = fee
	}
	if minWithdraw != "" {
		meta["min_withdraw_amount"] = minWithdraw
	}
	if requiredAmount != "" {
		meta["required_amount"] = requiredAmount
	}
	if netAmount != "" {
		meta["net_amount"] = netAmount
	}
	return errcode.NewWithMetadata(codes.InvalidArgument, 200, errcode.CodeAmountTooSmall, "AMOUNT_BELOW_MIN_PLUS_FEE", "amount below minimum withdraw amount plus fee", nil, meta)
}

// RiskCheckFailed 风控检查失败
func RiskCheckFailed(message string) error {
	if message == "" {
		message = "risk check failed"
	}
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeRiskControlTriggered, "RISK_CHECK_FAILED", message, nil)
}

// Blacklisted 已被加入黑名单
func Blacklisted(message string) error {
	if message == "" {
		message = "blocked"
	}
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeRiskBlacklisted, "BLACKLISTED", message, nil)
}

// TokenError token错误
func TokenError() error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "TOKEN_ERROR", "token error", nil)
}

// DBError 数据库错误
func DBError() error {
	return errcode.New(codes.Internal, 500, errcode.CodeDatabaseError, "DB_ERROR", "db error", nil)
}

// DBSchemaNotMigrated 数据库结构未迁移（用于快速定位线上注册/登录失败）
// e.g. users.id 仍是非 AUTO_INCREMENT 导致插入失败（见 deploy/docker/init-db/63-users-auto-increment-id.sql）
func DBSchemaNotMigrated(migration string) error {
	meta := map[string]string{}
	if migration != "" {
		meta["migration"] = migration
	}
	return errcode.NewWithMetadata(codes.Internal, 500, errcode.CodeDatabaseError, "DB_SCHEMA_NOT_MIGRATED", "db schema not migrated", nil, meta)
}

// ServiceNotAvailable 服务不可用
func ServiceNotAvailable(service string) error {
	return errcode.New(codes.Unavailable, 503, errcode.CodeServiceUnavail, "SERVICE_NOT_AVAILABLE", service+" not available", nil)
}

// SendFailed 发送失败
func SendFailed(message string) error {
	if message == "" {
		message = "send failed"
	}
	return errcode.New(codes.Internal, 200, errcode.CodeInternalError, "SEND_FAILED", message, nil)
}

// SMSSendFailed 短信发送失败（带国际化）
func SMSSendFailed() error {
	return errcode.New(codes.Internal, 200, errcode.CodeInternalError, "SMS_SEND_FAILED", "SMS send failed", nil)
}

// SMSInvalidPhone 手机号格式错误，无法发送短信
func SMSInvalidPhone() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "SMS_INVALID_PHONE", "Invalid phone number format", nil)
}

// GenerateFailed 生成失败
func GenerateFailed() error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "GENERATE_FAILED", "generate failed", nil)
}

// RegistrationFailed 注册失败
func RegistrationFailed() error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "REGISTRATION_FAILED", "registration failed", nil)
}

// ========================
// Token/Session 相关错误（业务错误返回 HTTP 200）
// ========================

// RefreshTokenInvalid 刷新令牌无效
func RefreshTokenInvalid() error {
	return errcode.New(codes.Unauthenticated, 200, errcode.CodeAuthTokenInvalid, "REFRESH_TOKEN_INVALID", "invalid or expired refresh token", nil)
}

// RefreshTokenExpired 刷新令牌已过期
func RefreshTokenExpired() error {
	return errcode.New(codes.Unauthenticated, 200, errcode.CodeAuthTokenExpired, "REFRESH_TOKEN_EXPIRED", "refresh token expired", nil)
}

// SessionTerminated 会话已终止
func SessionTerminated() error {
	return errcode.New(codes.Unauthenticated, 200, errcode.CodeAuthSessionExpired, "SESSION_TERMINATED", "session has been terminated", nil)
}

// TokenGenerateFailed 令牌生成失败
func TokenGenerateFailed() error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "TOKEN_GENERATE_FAILED", "failed to generate token", nil)
}

// SessionUpdateFailed 会话更新失败
func SessionUpdateFailed() error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "SESSION_UPDATE_FAILED", "failed to update session", nil)
}

// ========================
// Swap 服务相关错误
// ========================

// SwapServiceNotAvailable Swap服务不可用（保持 503，服务级别错误）
func SwapServiceNotAvailable() error {
	return errcode.New(codes.Unavailable, 503, errcode.CodeServiceUnavail, "SWAP_SERVICE_NOT_AVAILABLE", "swap service not available", nil)
}

// SwapServiceError Swap服务错误（保持 500，服务级别错误）
func SwapServiceError() error {
	return errcode.New(codes.Internal, 500, errcode.CodeSwapProviderError, "SWAP_SERVICE_ERROR", "swap service error", nil)
}

// InvalidChainId 无效的链ID
func InvalidChainId() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_CHAIN_ID", "invalid chain_id", nil)
}

// InvalidTokenAddress 无效的代币地址
func InvalidTokenAddress() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_TOKEN_ADDRESS", "invalid token address", nil)
}

// InvalidAmount 无效金额
func InvalidAmount() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidAmount, "INVALID_AMOUNT", "invalid amount", nil)
}

// InvalidWalletAddress 无效钱包地址
func InvalidWalletAddress() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidAddress, "INVALID_WALLET_ADDRESS", "invalid wallet_address", nil)
}

// InvalidSwapId 无效的SwapID
func InvalidSwapId() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_SWAP_ID", "swap_id is required", nil)
}

// SwapAmountTooSmall 兑换金额低于最小限制
func SwapAmountTooSmall(minAmount string) error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidAmount, "SWAP_AMOUNT_TOO_SMALL",
		"swap amount is below minimum limit $"+minAmount, nil)
}

// SwapAmountTooLarge 兑换金额超过最大限制
func SwapAmountTooLarge(maxAmount string) error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidAmount, "SWAP_AMOUNT_TOO_LARGE",
		"swap amount exceeds maximum limit $"+maxAmount, nil)
}

// SwapInsufficientLiquidity 流动性不足
func SwapInsufficientLiquidity() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeSwapProviderError, "SWAP_INSUFFICIENT_LIQUIDITY",
		"insufficient liquidity for this swap, try a different amount or token pair", nil)
}

// SwapTokenNotSupported 代币不支持
func SwapTokenNotSupported() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "SWAP_TOKEN_NOT_SUPPORTED",
		"token not supported for swap on this chain", nil)
}

// SwapRateLimited 请求频率过高
func SwapRateLimited() error {
	return errcode.New(codes.ResourceExhausted, 200, errcode.CodeTooManyRequests, "SWAP_RATE_LIMITED",
		"too many requests, please try again later", nil)
}

// ========================
// 通知相关错误
// ========================

// NotificationNotFound 通知不存在
func NotificationNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeNotFound, "NOTIFICATION_NOT_FOUND", "notification not found", nil)
}

// ========================
// 钱包地址相关错误
// ========================

// AddressNotFound 地址不存在
func AddressNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeNotFound, "ADDRESS_NOT_FOUND", "address not found", nil)
}

// AddressValidationFailed 地址验证失败
func AddressValidationFailed(message string) error {
	if message == "" {
		message = "address validation failed"
	}
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidAddress, "ADDRESS_VALIDATION_FAILED", message, nil)
}

// ========================
// 资产相关错误
// ========================

// AssetNotFound 资产不存在
func AssetNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeAssetNotFound, "ASSET_NOT_FOUND", "asset not found", nil)
}

// DepositNotFound 充值记录不存在
func DepositNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeDepositNotFound, "DEPOSIT_NOT_FOUND", "deposit not found", nil)
}

// WithdrawNotFound 提现记录不存在
func WithdrawNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeWithdrawNotFound, "WITHDRAW_NOT_FOUND", "withdraw not found", nil)
}

// RecordNotFound 记录不存在
func RecordNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeNotFound, "RECORD_NOT_FOUND", "record not found", nil)
}

// ========================
// 钱包关联相关错误
// ========================

// ProviderNotFound 提供方不存在
func ProviderNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeNotFound, "PROVIDER_NOT_FOUND", "provider not found", nil)
}

// BindingNotFound 绑定关系不存在
func BindingNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeNotFound, "BINDING_NOT_FOUND", "binding not found", nil)
}

// PasswordTooWeak 密码强度不足
func PasswordTooWeak(message string) error {
	if message == "" {
		message = "password too weak"
	}
	return errcode.New(codes.InvalidArgument, 200, errcode.CodePasswordTooWeak, "PASSWORD_TOO_WEAK", message, nil)
}

// ========================
// 交易密码相关错误
// ========================

// TradePasswordNotSet 未设置交易密码
func TradePasswordNotSet() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodePasswordMismatch, "TRADE_PASSWORD_NOT_SET", "trade password not set", nil)
}

// InvalidTradePassword 交易密码错误
func InvalidTradePassword() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodePasswordMismatch, "INVALID_TRADE_PASSWORD", "invalid trade password", nil)
}

// InvalidOldTradePassword 旧交易密码错误
func InvalidOldTradePassword() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodePasswordMismatch, "INVALID_OLD_TRADE_PASSWORD", "old trade password is incorrect", nil)
}

// OldTradePasswordRequired 旧交易密码必填
func OldTradePasswordRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "OLD_TRADE_PASSWORD_REQUIRED", "old trade password required", nil)
}

// NewTradePasswordRequired 新交易密码必填
func NewTradePasswordRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "NEW_TRADE_PASSWORD_REQUIRED", "new trade password required", nil)
}

// TradePasswordTooShort 交易密码长度不足
func TradePasswordTooShort(minLength int) error {
	metadata := map[string]string{
		"min_length": fmt.Sprintf("%d", minLength),
	}
	return errcode.NewWithMetadata(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "TRADE_PASSWORD_MIN_LENGTH", "trade password too short", nil, metadata)
}

// TradePasswordSameAsOld 新旧交易密码相同
func TradePasswordSameAsOld() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "TRADE_PASSWORD_SAME_AS_OLD", "new password must be different from old password", nil)
}

// TradePasswordRequired 交易密码必填
func TradePasswordRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "TRADE_PASSWORD_REQUIRED", "trade password required", nil)
}

// TradePasswordLocked 交易密码已锁定（错误次数过多）
func TradePasswordLocked() error {
	return errcode.New(codes.PermissionDenied, 200, errcode.CodeAuthAccountLocked, "TRADE_PASSWORD_LOCKED", "trade password locked due to too many failed attempts", nil)
}

// TradePasswordWrongWithRemaining 交易密码错误（带剩余次数）
func TradePasswordWrongWithRemaining(remaining int) error {
	metadata := map[string]string{
		"remaining_attempts": fmt.Sprintf("%d", remaining),
	}
	return errcode.NewWithMetadata(codes.InvalidArgument, 200, errcode.CodePasswordMismatch, "INVALID_TRADE_PASSWORD", "invalid trade password", nil, metadata)
}

// ========================
// 安全冷却期相关错误
// ========================

// SecurityCooldownActiveWithMessage 安全冷却期内禁止操作（使用本地化消息）
// 当用户修改交易密码或解绑GA后24小时内，禁止进行转账等敏感操作
func SecurityCooldownActiveWithMessage(message string) error {
	// 使用 NewWithMetadata 将已本地化的消息放入 error_detail，
	// 防止 API Gateway 使用不完整的参数重新渲染模板导致 <no value>
	return errcode.NewWithMetadata(codes.PermissionDenied, 200, errcode.CodeRiskControlTriggered, "SECURITY_COOLDOWN_ACTIVE", message, nil, map[string]string{
		"error_detail": message,
	})
}

// ========================
// Web3 相关错误
// ========================

// Web3DeviceIDRequired 设备ID不能为空
func Web3DeviceIDRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_DEVICE_ID_REQUIRED", "device_id required", nil)
}

// Web3UserExists Web3用户已存在
func Web3UserExists() error {
	return errcode.New(codes.AlreadyExists, 200, errcode.CodeConflict, "WEB3_USER_EXISTS", "web3 user already exists", nil)
}

// Web3UserCreateFailed 创建用户失败
func Web3UserCreateFailed() error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "WEB3_USER_CREATE_FAILED", "failed to create user", nil)
}

// Web3AddressRequired 钱包地址不能为空
func Web3AddressRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_ADDRESS_REQUIRED", "wallet address required", nil)
}

// Web3AddressQueryFailed 查询地址余额失败
func Web3AddressQueryFailed() error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "WEB3_ADDRESS_QUERY_FAILED", "failed to query address balance", nil)
}

// Web3AddressBlacklisted 地址已被列入黑名单
func Web3AddressBlacklisted() error {
	return errcode.New(codes.PermissionDenied, 200, errcode.CodeRiskBlacklisted, "WEB3_ADDRESS_BLACKLISTED", "address is blacklisted", nil)
}

// Web3AddressDisabled 地址已禁用
func Web3AddressDisabled() error {
	return errcode.New(codes.PermissionDenied, 200, errcode.CodeForbidden, "WEB3_ADDRESS_DISABLED", "address is disabled", nil)
}

// Web3SignedTxRequired 已签名交易不能为空
func Web3SignedTxRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_SIGNED_TX_REQUIRED", "signed transaction required", nil)
}

// Web3NetworkRequired 网络名称不能为空
func Web3NetworkRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_NETWORK_REQUIRED", "network name required", nil)
}

// Web3FromAddressRequired 发送方地址不能为空
func Web3FromAddressRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_FROM_ADDRESS_REQUIRED", "from address required", nil)
}

// Web3ParseTxFailed 解析交易失败
func Web3ParseTxFailed(message string) error {
	if message == "" {
		message = "failed to parse transaction"
	}
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_PARSE_TX_FAILED", message, nil)
}

// Web3FromAddressMismatch 交易中的发送方地址与请求不一致
func Web3FromAddressMismatch() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_FROM_ADDRESS_MISMATCH", "from address in transaction does not match request", nil)
}

// Web3BroadcastFailed 广播交易失败
func Web3BroadcastFailed(message string) error {
	if message == "" {
		message = "failed to broadcast transaction"
	}
	// 将具体错误信息放到 metadata 中，避免被国际化覆盖
	metadata := map[string]string{
		"error_detail": message,
	}
	return errcode.NewWithMetadata(codes.Internal, 500, errcode.CodeInternalError, "WEB3_BROADCAST_FAILED", message, nil, metadata)
}

// Web3ChainServiceUnavailable 链服务不可用
func Web3ChainServiceUnavailable() error {
	return errcode.New(codes.Unavailable, 503, errcode.CodeServiceUnavail, "WEB3_CHAIN_SERVICE_UNAVAILABLE", "chain service unavailable", nil)
}

// Web3UnsupportedNetwork 不支持的网络
func Web3UnsupportedNetwork(network string) error {
	message := "unsupported network"
	if network != "" {
		message = "unsupported network: " + network
	}
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_UNSUPPORTED_NETWORK", message, nil)
}

// Web3RecordTxFailed 记录交易失败
func Web3RecordTxFailed(message string) error {
	if message == "" {
		message = "failed to record transaction"
	}
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "WEB3_RECORD_TX_FAILED", message, nil)
}

// Web3TxExists 交易已存在
func Web3TxExists() error {
	return errcode.New(codes.AlreadyExists, 200, errcode.CodeConflict, "WEB3_TX_EXISTS", "transaction already exists", nil)
}

// Web3UserNotFound Web3 用户不存在
func Web3UserNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeNotFound, "WEB3_USER_NOT_FOUND", "Web3 user not found", nil)
}

// Web3AddressCreateFailed 创建地址失败
func Web3AddressCreateFailed() error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "WEB3_ADDRESS_CREATE_FAILED", "failed to create address", nil)
}

// Web3AddressTransferFailed 转移地址失败
func Web3AddressTransferFailed() error {
	return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "WEB3_ADDRESS_TRANSFER_FAILED", "failed to transfer address", nil)
}

// Web3AddressListRequired 地址列表不能为空
func Web3AddressListRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_ADDRESS_LIST_REQUIRED", "address list is required", nil)
}

// Web3AddressBatchLimitExceeded 批量添加地址数量超限
func Web3AddressBatchLimitExceeded(limit int) error {
	message := "too many addresses in batch"
	if limit > 0 {
		message = fmt.Sprintf("maximum %d addresses allowed per batch", limit)
	}
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_BATCH_ADDRESS_LIMIT_EXCEEDED", message, nil)
}

// Web3TxHashRequired 交易哈希不能为空
func Web3TxHashRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_TX_HASH_REQUIRED", "transaction hash required", nil)
}

// Web3TxIDRequired 交易记录ID不能为空
func Web3TxIDRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_TX_ID_REQUIRED", "transaction id required", nil)
}

// Web3TransactionNotFound 交易不存在
func Web3TransactionNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeNotFound, "WEB3_TRANSACTION_NOT_FOUND", "transaction not found", nil)
}

// Web3ChainCodeRequired 链代码不能为空
func Web3ChainCodeRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_CHAIN_CODE_REQUIRED", "chain code required", nil)
}

// Web3AssetCodeRequired 资产代码不能为空
func Web3AssetCodeRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WEB3_ASSET_CODE_REQUIRED", "asset code required", nil)
}

// Web3AccountNotActivated 账户未激活（TRON 网络需要先收到 TRX 激活账户）
// 支持国际化：使用 errcode.CodeAccountNotActivated，前端可根据 reason 字段查找对应语言的消息
func Web3AccountNotActivated() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeAccountNotActivated, "WEB3_ACCOUNT_NOT_ACTIVATED", errcode.CodeAccountNotActivated.Message(), nil)
}

// Web3InsufficientGas Gas 不足
// 支持国际化：使用 errcode.CodeInsufficientGas，前端可根据 reason 字段查找对应语言的消息
// 额外的详细信息通过 metadata 传递
func Web3InsufficientGas(network string, required string, available string) error {
	message := errcode.CodeInsufficientGas.Message()
	metadata := map[string]string{}
	if network != "" {
		metadata["network"] = network
	}
	if required != "" {
		metadata["required"] = required
	}
	if available != "" {
		metadata["available"] = available
	}
	return errcode.NewWithMetadata(codes.FailedPrecondition, 200, errcode.CodeInsufficientGas, "WEB3_INSUFFICIENT_GAS", message, nil, metadata)
}

// Web3AccountResourceInsufficient 账户资源不足（TRON 网络的带宽或能量不足）
// 支持国际化：使用 WEB3_ACCOUNT_RESOURCE_INSUFFICIENT，前端可根据错误码查找对应语言的消息
func Web3AccountResourceInsufficient() error {
	return errcode.New(codes.FailedPrecondition, 200, errcode.CodeInsufficientBalance, "WEB3_ACCOUNT_RESOURCE_INSUFFICIENT", "account resource insufficient", nil)
}

// ========================
// 内部地址相关错误
// ========================

// InternalAddressTargetUserNotFound 目标用户不存在
func InternalAddressTargetUserNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeUserNotFound, "INTERNAL_ADDRESS_TARGET_USER_NOT_FOUND", "target user not found", nil)
}

// InternalAddressCannotAddSelf 不能添加自己为内部地址
func InternalAddressCannotAddSelf() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INTERNAL_ADDRESS_CANNOT_ADD_SELF", "cannot add yourself as internal address", nil)
}

// InternalAddressAlreadyExists 内部地址已存在
func InternalAddressAlreadyExists() error {
	return errcode.New(codes.AlreadyExists, 200, errcode.CodeConflict, "INTERNAL_ADDRESS_ALREADY_EXISTS", "internal address already exists", nil)
}

// AddressAlreadyExists 地址簿中地址已存在
func AddressAlreadyExists() error {
	return errcode.New(codes.AlreadyExists, 200, errcode.CodeAddressAlreadyExists, "ADDRESS_ALREADY_EXISTS", errcode.CodeAddressAlreadyExists.Message(), nil)
}

// InternalAddressNotFound 内部地址不存在
func InternalAddressNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeNotFound, "INTERNAL_ADDRESS_NOT_FOUND", "internal address not found", nil)
}

// InternalAddressLookupTypeRequired 查找类型和值不能为空
func InternalAddressLookupTypeRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INTERNAL_ADDRESS_LOOKUP_REQUIRED", "lookup_type and lookup_value are required", nil)
}

// InternalAddressLabelRequired 标签不能为空
func InternalAddressLabelRequired() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INTERNAL_ADDRESS_LABEL_REQUIRED", "label is required", nil)
}

// InternalAddressInvalidLookupType 无效的查找类型
func InternalAddressInvalidLookupType() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INTERNAL_ADDRESS_INVALID_LOOKUP_TYPE", "lookup_type must be 'uid' or 'email'", nil)
}

// ========================
// 内部转账相关错误
// ========================

// InternalTransferTargetUserNotFound 内部转账目标用户不存在
func InternalTransferTargetUserNotFound() error {
	return errcode.New(codes.NotFound, 200, errcode.CodeUserNotFound, "INTERNAL_TRANSFER_TARGET_USER_NOT_FOUND", "目标用户不存在", nil)
}

// InternalTransferInvalidTargetUserFormat 内部转账目标用户格式无效
func InternalTransferInvalidTargetUserFormat() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INTERNAL_TRANSFER_INVALID_TARGET_USER_FORMAT", "目标用户格式无效，请输入UID或邮箱", nil)
}

// InternalTransferAmountOutOfRange 内部转账金额超出允许范围
func InternalTransferAmountOutOfRange() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INTERNAL_TRANSFER_AMOUNT_OUT_OF_RANGE", "转账金额超出允许范围", nil)
}

// WithdrawAmountOutOfRange 提现金额超出允许范围
func WithdrawAmountOutOfRange() error {
	return errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "WITHDRAW_AMOUNT_OUT_OF_RANGE", "提现金额超出允许范围", nil)
}
