package errcode

// 签名服务错误码定义
// 范围: 2000-2999
const (
	// ========== 通用错误 2000-2099 ==========
	SignerSuccess        = 0    // 成功
	SignerInvalidParams  = 2001 // 参数错误
	SignerInternalError  = 2002 // 内部错误
	SignerDatabaseError  = 2003 // 数据库错误
	SignerUnauthorized   = 2004 // 未授权
	SignerRateLimitError = 2005 // 请求限流
	SignerServiceUnavail = 2006 // 服务不可用

	// ========== Seed错误 2100-2199 ==========
	SignerSeedNotFound        = 2101 // Seed不存在
	SignerSeedAlreadyExist    = 2102 // Seed已存在
	SignerSeedDecryptFail     = 2103 // Seed解密失败
	SignerSeedInactive        = 2104 // Seed未激活
	SignerSeedEncryptFail     = 2105 // Seed加密失败
	SignerSeedHashMismatch    = 2106 // Seed哈希校验失败
	SignerSeedInvalidPassword = 2107 // Seed密码错误
	SignerSeedDeprecated      = 2108 // Seed已废弃

	// ========== 公钥导出错误 2200-2299 ==========
	SignerInvalidPath      = 2201 // 派生路径无效
	SignerPubkeyExportFail = 2202 // 公钥导出失败
	SignerChainNotSupport  = 2203 // 链不支持
	SignerInvalidIndex     = 2204 // 索引无效
	SignerBatchTooLarge    = 2205 // 批量数量过大
	SignerDerivationFail   = 2206 // HD派生失败

	// ========== 签名错误 2300-2399 ==========
	SignerSignatureFailed    = 2301 // 签名失败
	SignerDuplicateRequest   = 2302 // 重复请求
	SignerInvalidTxData      = 2303 // 交易数据无效
	SignerAmountExceedLimit  = 2304 // 金额超过限额
	SignerApprovalRequired   = 2305 // 需要审批
	SignerInvalidSignature   = 2306 // 签名验证失败
	SignerPrivateKeyNotFound = 2307 // 私钥不存在
	SignerInvalidAddress     = 2308 // 地址无效

	// ========== 助记词验证挑战错误 2400-2499 ==========
	SignerMnemonicNotFound         = 2401 // 助记词不存在
	SignerRateLimitExceeded        = 2402 // 频率限制超出
	SignerChallengeExpired         = 2403 // 挑战会话已过期 (50010)
	SignerChallengeVerifyFailed    = 2404 // 助记词验证失败 (50011)
	SignerChallengeTooManyAttempts = 2405 // 尝试次数过多 (50012)
	SignerChallengeNotFound        = 2406 // 挑战会话不存在 (50013)

	// ========== 钱包初始化/解锁 2500-2599 ==========
	SignerWalletLocked               = 2501 // signer 未解锁
	SignerWalletUninitialized        = 2502 // 热钱包未初始化
	SignerWalletPendingBackup        = 2503 // 热钱包初始化未完成（待备份确认）
	SignerWalletInitSessionGone      = 2504 // 初始化会话不存在/已过期
	SignerWalletChallengeGone        = 2505 // 校验会话不存在/已过期
	SignerWalletUnlockFailed         = 2506 // 解锁失败
	SignerCompanyWalletMisconfigured = 2507 // 公司热钱包出款地址未配置/配置冲突
)

// SignerErrMsg 错误码消息映射
var SignerErrMsg = map[int]string{
	// 通用错误
	SignerSuccess:        "success",
	SignerInvalidParams:  "invalid parameters",
	SignerInternalError:  "internal error",
	SignerDatabaseError:  "database error",
	SignerUnauthorized:   "unauthorized",
	SignerRateLimitError: "rate limit exceeded",
	SignerServiceUnavail: "service unavailable",

	// Seed错误
	SignerSeedNotFound:        "seed not found",
	SignerSeedAlreadyExist:    "seed already exists",
	SignerSeedDecryptFail:     "seed decryption failed",
	SignerSeedInactive:        "seed is inactive",
	SignerSeedEncryptFail:     "seed encryption failed",
	SignerSeedHashMismatch:    "seed hash verification failed",
	SignerSeedInvalidPassword: "invalid seed password",
	SignerSeedDeprecated:      "seed is deprecated",

	// 公钥导出错误
	SignerInvalidPath:      "invalid derivation path",
	SignerPubkeyExportFail: "public key export failed",
	SignerChainNotSupport:  "chain not supported",
	SignerInvalidIndex:     "invalid address index",
	SignerBatchTooLarge:    "batch size too large",
	SignerDerivationFail:   "hd derivation failed",

	// 签名错误
	SignerSignatureFailed:    "signature failed",
	SignerDuplicateRequest:   "duplicate request",
	SignerInvalidTxData:      "invalid transaction data",
	SignerAmountExceedLimit:  "amount exceeds daily limit",
	SignerApprovalRequired:   "manual approval required",
	SignerInvalidSignature:   "signature verification failed",
	SignerPrivateKeyNotFound: "private key not found",
	SignerInvalidAddress:     "invalid address",

	// 助记词验证挑战错误
	SignerMnemonicNotFound:         "mnemonic not found",
	SignerRateLimitExceeded:        "rate limit exceeded",
	SignerChallengeExpired:         "challenge session has expired",
	SignerChallengeVerifyFailed:    "mnemonic verification failed",
	SignerChallengeTooManyAttempts: "too many attempts",
	SignerChallengeNotFound:        "challenge session not found",

	// 钱包初始化/解锁
	SignerWalletLocked:               "wallet is locked",
	SignerWalletUninitialized:        "wallet is not initialized",
	SignerWalletPendingBackup:        "wallet init pending backup confirmation",
	SignerWalletInitSessionGone:      "wallet init session not found",
	SignerWalletChallengeGone:        "wallet challenge session not found",
	SignerWalletUnlockFailed:         "wallet unlock failed",
	SignerCompanyWalletMisconfigured: "company wallet misconfigured",
}

// GetSignerErrMsg 获取错误消息
func GetSignerErrMsg(code int) string {
	if msg, ok := SignerErrMsg[code]; ok {
		return msg
	}
	return "unknown error"
}
