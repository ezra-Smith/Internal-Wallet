package interceptor

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/rbac"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type ctxKey string

const currentAdminKey ctxKey = "current-admin"

// GetCurrentAdmin 从 context 获取当前管理员（由拦截器注入）
func GetCurrentAdmin(ctx context.Context) (*model.AdminUserModel, bool) {
	v := ctx.Value(currentAdminKey)
	if v == nil {
		return nil, false
	}
	m, ok := v.(*model.AdminUserModel)
	return m, ok
}

// AdminAuthUnaryInterceptor Admin 服务鉴权 + RBAC + 幂等控制
func AdminAuthUnaryInterceptor(svcCtx *svc.ServiceContext) grpc.UnaryServerInterceptor {
	publicMethods := map[string]bool{
		// Admin MFA flow (public endpoints)
		"AdminMFACaptchaGenerate": true,
		"AdminMFACaptchaValidate": true,
		"AdminMFALogin":           true,
		"AdminMFAGetTwoFAQrCode":  true,
		"AdminMFABindTwoFA":       true,
		"AdminMFAVerifyTwoFA":     true,
	}

	// 内部服务调用方法（business/chainsync → admin）
	// 注意：这些方法应该只允许内部服务调用，后续可增加服务间认证
	internalServiceMethods := map[string]bool{
		"InitializeUserWallet":           true, // business 调用：创建用户充值地址
		// chainsync 调用：拉取需要监控的充值地址（用于地址注册表/扫块）
		"ListDepositAddresses": true,
		// chainsync 调用：拉取 vault / web3 地址（用于地址注册表/扫块）
		"ListVaultAddresses":    true,
		"ListWeb3UserAddresses": true,
		"ListDepositAddressBalances":     true, // chainsync/business 调用：查询余额
		"GetCurrencyConfig":              true, // business 调用：获取币种配置（包括转账审核规则）
		"GetCurrencyGlobalTransferAudit": true, // business 调用：获取全局转账审核规则
		"CheckHotWalletBalance":          true, // business 调用：检查热钱包余额和 Gas（用于提现前校验）
		"GetClientConfig":                true, // business 调用：获取客户端配置
	}

	// 仅要求已登录（不做权限校验）的接口：账号自助/会话维护类，避免 RBAC 配置错误导致无法登出/改密/绑定 2FA。
	loginOnlyMethods := map[string]bool{
		"RefreshToken":          true,
		"Logout":                true,
		"GetMe":                 true,
		"ChangeMyPassword":      true,
		"GetMySecuritySettings": true,
		"StartTwoFASetup":       true,
		"ConfirmTwoFASetup":     true,
		"StartTwoFARebind":      true,
		"DisableTwoFA":          true,
		"GetMyRBAC":             true,
		"GetRbacPermissionMap":  true,
	}

	// 需要修改密码时允许访问的方法（其余一律拦截）
	passwordChangeBypass := map[string]bool{
		"ChangeMyPassword":      true,
		"Logout":                true,
		"RefreshToken":          true,
		"GetMe":                 true,
		"GetMySecuritySettings": true,
	}

	// 未绑定 2FA 时允许访问的方法（其余一律拦截）
	twoFABindBypass := map[string]bool{
		"GetMe":                 true,
		"GetMySecuritySettings": true,
		"StartTwoFASetup":       true,
		"ConfirmTwoFASetup":     true,
		"ChangeMyPassword":      true,
		"Logout":                true,
		"RefreshToken":          true,
	}

	// 系统钱包未初始化/未解锁时允许访问的方法（其余一律拦截）
	// 目标：在 hot wallet 解锁前，禁止进入系统页面与大部分业务接口。
	walletInitBypass := map[string]bool{
		// Session maintenance / status
		"GetMe":                 true,
		"Logout":                true,
		"RefreshToken":          true,
		"ChangeMyPassword":      true,
		"GetMySecuritySettings": true,
		"StartTwoFASetup":       true,
		"ConfirmTwoFASetup":     true,

		// Wallet init / signer unlock
		"GetHotWalletRuntimeStatus":        true,
		"StartHotWalletInit":               true,
		"GetHotWalletMnemonicPage":         true,
		"GetHotWalletMnemonicChallenge":    true,
		"VerifyHotWalletMnemonicChallenge": true,
		"UnlockSignerHotWallet":            true,
	}

	// method -> require Idempotency-Key
	idempotencyRequired := map[string]bool{
		"UpdateSystemConfig":               true,
		"DisableAdmin":                     true,
		"ResetAdminPassword":               true,
		"ResetAdminTwoFA":                  true,
		"ResetUserPassword":                true,
		"ResetUserTradePassword":           true,
		"UpdateUserMemberLevel":            true,
		"UpdateUserWithdrawAuditWhitelist": true,
		"UnbindUserTotp":                   true,
		"UpdateSwapConfig":                 true,
		"UpdateSwapProvider":               true,
		"ManageSwapTokens":                 true,
		"CancelDeposit":                    true,
		"CreateCurrencyWithdrawal":         true,
		"ApproveCurrencyWithdrawal":        true,
		"RejectCurrencyWithdrawal":         true,
		"CancelCurrencyWithdrawal":         true,
		"UpdateCurrencyWithdrawal":         true,
		"ApproveTransferBatch":             true,
		"ExecuteTransferBatch":             true,
		"RetryTransferBatch":               true,
		"CreateVaultAdjustment":            true,
		"ApproveVaultAdjustment":           true,

		// Blacklist (sensitive addresses)
		"CreateBlacklistAddress":              true,
		"BatchCreateBlacklistAddresses":       true,
		"UpdateBlacklistAddressMonitorStatus": true,
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		methodName := extractMethodName(info.FullMethod)

		// 1. 公开方法：直接放行
		if publicMethods[methodName] {
			return handler(ctx, req)
		}

		// 2. 内部服务调用方法：放行（TODO: 后续增加服务间认证）
		// 注意：当前直接放行，建议后续通过特殊 header 或 mutual TLS 验证来源
		if internalServiceMethods[methodName] {
			return handler(ctx, req)
		}

		// 基础鉴权：从 metadata 注入的 context 获取 admin_id
		adminIDStr := middleware.GetUserID(ctx)
		if adminIDStr == "" {
			return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
		}
		adminID, parseErr := strconv.ParseInt(adminIDStr, 10, 64)
		if parseErr != nil {
			return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", map[string]string{"user_id": "invalid"})
		}

		jti := middleware.GetJWTID(ctx)
		ver := middleware.GetJWTTokenVersion(ctx)

		// 服务端会话校验（Redis）：不存在则视为已失效
		if jti != "" {
			ok, err := svcCtx.SessionManager.Exists(ctx, adminID, jti)
			if err != nil {
				logx.WithContext(ctx).Errorw("session check failed", logx.Field("error", err))
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
			}
			if !ok {
				return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuthTokenRevoked, "AUTH_TOKEN_REVOKED", "token revoked", nil)
			}
		}

		// 黑名单校验（logout/refresh）
		if jti != "" {
			black, err := svcCtx.TokenBlacklist.IsBlacklisted(ctx, jti)
			if err != nil {
				logx.WithContext(ctx).Errorw("blacklist check failed", logx.Field("error", err))
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
			}
			if black {
				return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuthTokenRevoked, "AUTH_TOKEN_REVOKED", "token revoked", nil)
			}
		}

		// DB 校验：账号状态/版本
		admin, err := svcCtx.AdminUserRepo.FindByID(ctx, adminID)
		if err != nil || admin == nil {
			return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
		}
		if admin.Status != "active" {
			return nil, errx.New(codes.PermissionDenied, 403, errx.CodeForbidden, "AUTH_ACCOUNT_DISABLED", "account disabled", nil)
		}
		if ver > 0 && admin.TokenVersion > 0 && ver != admin.TokenVersion {
			return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuthTokenRevoked, "AUTH_TOKEN_REVOKED", "token revoked", nil)
		}

		// 首次登录/强制改密：除改密/刷新/登出外，阻断其他接口
		if admin.RequirePasswordChange && !passwordChangeBypass[methodName] {
			return nil, errx.New(codes.FailedPrecondition, 428, errx.CodeAuthPasswordChangeReq, "PASSWORD_CHANGE_REQUIRED", "需要修改密码", nil)
		}

		// 强制绑定 2FA：改密完成后，若要求绑定但未绑定，则仅允许进入绑定流程
		twoFARequired := svcCtx.EffectiveRequire2FA(ctx) || admin.TwoFactorRequired
		twoFABound := admin.TwoFactorEnabled && admin.TwoFactorSecret != ""
		if !admin.RequirePasswordChange && twoFARequired && !twoFABound && !twoFABindBypass[methodName] {
			return nil, errx.New(codes.FailedPrecondition, 428, errx.CodeAuthTwoFABindReq, "TWO_FA_BIND_REQUIRED", "需要绑定2FA", nil)
		}

		// 系统钱包初始化/解锁：在钱包未解锁前，仅允许进入钱包初始化流程
		// 说明：Signer 不可用/查询失败时，按“未解锁”保守处理（避免绕过）。
		walletState := ""
		if svcCtx != nil {
			if st, wErr := svcCtx.GetCachedHotWalletState(ctx); wErr == nil {
				walletState = strings.TrimSpace(st)
			}
		}
		if walletState != "unlocked" && !walletInitBypass[methodName] {
			return nil, errx.New(codes.FailedPrecondition, 428, errx.CodeAuthWalletInitReq, "WALLET_INIT_REQUIRED", "需要初始化系统钱包", nil)
		}

		// RBAC v2（按 permission code）
		// 约定：
		// - legacy: rpc:<MethodName>
		// - scheme B (canonical): <domain>.<resource>.<action> (e.g. rbac.role.create)
		if !loginOnlyMethods[methodName] {
			if svcCtx.AdminRBACRepo == nil {
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "rbac not ready", nil)
			}
			required := rbac.RequiredPermissionForMethod(methodName)
			list, qErr := svcCtx.AdminRBACRepo.GetUserPermissions(ctx, adminID)
			if qErr != nil {
				logx.WithContext(ctx).Errorw("query user permissions failed", logx.Field("error", qErr))
				if table, ok := errx.MySQLTableNotFound(qErr); ok {
					return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "rbac schema not initialized (missing table: "+table+")", nil)
				}
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query user permissions failed", nil)
			}
			granted := make([]string, 0, len(list))
			seen := map[string]struct{}{}
			for _, p := range list {
				if p == nil {
					continue
				}
				code := strings.TrimSpace(p.Code)
				if code == "" {
					continue
				}
				if _, ok := seen[code]; ok {
					continue
				}
				seen[code] = struct{}{}
				granted = append(granted, code)
			}
			granted = rbac.CanonicalizePermissionCodes(granted)
			if !rbac.HasPermission(granted, required) {
				return nil, errx.New(codes.PermissionDenied, 403, errx.CodeForbidden, "FORBIDDEN", "forbidden", map[string]string{"permission": required})
			}
		}

		// 幂等键（部分敏感接口）
		if idempotencyRequired[methodName] {
			key := middleware.GetIdempotencyKey(ctx)
			if key == "" {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "IDEMPOTENCY_KEY_REQUIRED", "missing Idempotency-Key", nil)
			}
			ok, acquireErr := svcCtx.IdempotencyManager.Acquire(ctx, adminID, methodName, key, 24*time.Hour)
			if acquireErr != nil {
				logx.WithContext(ctx).Errorw("idempotency acquire failed", logx.Field("error", acquireErr))
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
			}
			if !ok {
				return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "DUPLICATE_REQUEST", "duplicate request", nil)
			}
		}

		// 注入当前管理员到 context，供业务逻辑复用
		ctx = context.WithValue(ctx, currentAdminKey, admin)
		return handler(ctx, req)
	}
}

func extractMethodName(fullMethod string) string {
	// fullMethod: /{package}.{service}/{method}
	if fullMethod == "" {
		return ""
	}
	parts := strings.Split(fullMethod, "/")
	if len(parts) == 0 {
		return fullMethod
	}
	return parts[len(parts)-1]
}
