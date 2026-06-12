package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/rbac"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AdminMFALoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminMFALoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMFALoginLogic {
	return &AdminMFALoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminMFALoginLogic) AdminMFALogin(in *pb.AdminMFALoginRequest) (*pb.AdminMFALoginResponse, error) {
	if in == nil || strings.TrimSpace(in.TempKey) == "" || strings.TrimSpace(in.Email) == "" || in.Password == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"temp_key": "required",
			"email":    "required",
			"password": "required",
		})
	}
	if l.svcCtx.AdminUserRepo == nil || l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if l.svcCtx.MFAFlow == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "mfa flow not configured", nil)
	}

	clientIP := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	ok, err := l.svcCtx.MFAFlow.ConsumeTempKey(l.ctx, in.TempKey, clientIP, ua)
	if err != nil {
		l.Logger.Errorf("consume temp_key failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	if !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TEMP_KEY_INVALID", "unauthorized", nil)
	}

	username := strings.TrimSpace(in.Email)
	now := time.Now()

	admin, err := l.svcCtx.AdminUserRepo.GetByUsername(l.ctx, username)
	if err != nil || admin == nil {
		// Avoid account enumeration.
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuthInvalidCredentials, "AUTH_INVALID_CREDENTIALS", "账号或密码错误", nil)
	}
	if admin.Status != "active" {
		return nil, errx.New(codes.PermissionDenied, 403, errx.CodeForbidden, "AUTH_ACCOUNT_DISABLED", "account disabled", nil)
	}
	if admin.LockUntil != nil && admin.LockUntil.After(now) {
		return nil, errx.New(codes.FailedPrecondition, 423, errx.CodeAuthAccountLocked, "AUTH_ACCOUNT_LOCKED", "account locked", nil)
	}

	if !security.VerifyPassword(admin.PasswordHash, in.Password) {
		maxAttempts := int32(5)
		if l.svcCtx.Config.Security.MaxLoginAttempts > 0 {
			maxAttempts = l.svcCtx.Config.Security.MaxLoginAttempts
		}
		var lockUntil *time.Time
		if int32(admin.FailedLoginCount)+1 >= maxAttempts {
			lockMinutes := int32(30)
			if l.svcCtx.Config.Security.LockoutDurationMinutes > 0 {
				lockMinutes = l.svcCtx.Config.Security.LockoutDurationMinutes
			}
			t := now.Add(time.Duration(lockMinutes) * time.Minute)
			lockUntil = &t
		}
		newCount, incErr := l.svcCtx.AdminUserRepo.IncrementFailedLogin(l.ctx, admin.ID, lockUntil)
		if incErr != nil {
			l.Logger.Errorf("increment failed login: %v", incErr)
		}

		// Log failed attempt.
		_ = l.svcCtx.AdminLoginLogRepo.CreateLog(l.ctx, &model.AdminLoginLogModel{
			AdminID:   admin.ID,
			Action:    "login_failed",
			IP:        clientIP,
			UserAgent: ua,
			Result:    "failed",
			Reason:    "invalid_credentials",
		})

		if int32(newCount) >= maxAttempts {
			return nil, errx.New(codes.FailedPrecondition, 423, errx.CodeAuthAccountLocked, "AUTH_ACCOUNT_LOCKED", "account locked", nil)
		}
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuthInvalidCredentials, "AUTH_INVALID_CREDENTIALS", "账号或密码错误", nil)
	}

	// Password ok: reset counters.
	_ = l.svcCtx.AdminUserRepo.ResetFailedLogin(l.ctx, admin.ID)

	twoFARequired := l.svcCtx.EffectiveRequire2FA(l.ctx) || admin.TwoFactorRequired
	twoFABound := admin.TwoFactorEnabled && strings.TrimSpace(admin.TwoFactorSecret) != ""

	// Case 4.1: 2FA already bound, require 2FA verification.
	if twoFABound {
		sess, sErr := l.svcCtx.MFAFlow.CreateTwoFASession(l.ctx, admin.ID, clientIP, ua)
		if sErr != nil {
			l.Logger.Errorf("create 2fa session failed: %v", sErr)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		return &pb.AdminMFALoginResponse{
			Success:           true,
			Status:            "2fa_required",
			SessionIdentifier: sess,
			Message:           resp.Msg(l.ctx, "MFA_CODE_REQUIRED"),
			RequestId:         resp.RequestID(l.ctx),
			Timestamp:         resp.Timestamp(),
		}, nil
	}

	// Case 4.2: 2FA not bound but required (only after password change is completed).
	// For first-login flows where password change is mandatory, allow issuing a limited token so user can change password first.
	if twoFARequired && !twoFABound && !admin.RequirePasswordChange {
		authKey, kErr := l.svcCtx.MFAFlow.CreateAuthKey(l.ctx, admin.ID, clientIP, ua)
		if kErr != nil {
			l.Logger.Errorf("create auth_key failed: %v", kErr)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		return &pb.AdminMFALoginResponse{
			Success:   true,
			Status:    "2fa_not_bound",
			AuthKey:   authKey,
			Message:   resp.Msg(l.ctx, "MFA_SETUP_REQUIRED"),
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	// No 2FA: issue final auth token.
	expireSeconds := int64(28800)
	if l.svcCtx.Config.JWT.AccessExpire > 0 {
		expireSeconds = l.svcCtx.Config.JWT.AccessExpire
	}
	accessToken, meta, genErr := security.GenerateAdminAccessToken(admin.ID, admin.Username, admin.TokenVersion, l.svcCtx.Config.JWT.AccessSecret, expireSeconds)
	if genErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "token generate failed", nil)
	}
	_ = l.svcCtx.SessionManager.Add(l.ctx, admin.ID, meta.JTI, meta.Exp)
	_ = l.svcCtx.AdminUserRepo.UpdateLoginSuccess(l.ctx, admin.ID, clientIP)
	_ = l.svcCtx.AdminLoginLogRepo.CreateLog(l.ctx, &model.AdminLoginLogModel{
		AdminID:   admin.ID,
		Action:    "login",
		IP:        clientIP,
		UserAgent: ua,
		Result:    "success",
	})

	// RBAC v2: return permission codes derived from admin_user_role/admin_role_permission.
	roleCode := admin.Role
	perms := []string{}
	if l.svcCtx.AdminRBACRepo != nil {
		if roles, err := l.svcCtx.AdminRBACRepo.GetUserRoles(l.ctx, admin.ID); err == nil && len(roles) > 0 {
			if v := strings.TrimSpace(roles[0].Code); v != "" {
				roleCode = v
			}
		}
		if list, err := l.svcCtx.AdminRBACRepo.GetUserPermissions(l.ctx, admin.ID); err == nil {
			seen := map[string]struct{}{}
			for _, p := range list {
				if p == nil {
					continue
				}
				code := strings.TrimSpace(p.Code)
				if code == "" {
					continue
				}
				seen[code] = struct{}{}
			}
			perms = make([]string, 0, len(seen))
			for code := range seen {
				perms = append(perms, code)
			}
			perms = rbac.CanonicalizePermissionCodes(perms)
		}
	}

	// Hot wallet runtime state (best-effort; do not fail login on signer issues)
	walletState := ""
	if l.svcCtx != nil && l.svcCtx.SignerRpc != nil {
		if st, err := l.svcCtx.SignerRpc.GetRuntimeStatus(l.ctx, &pb.GetRuntimeStatusRequest{}); err == nil && st != nil {
			walletState = st.GetWalletState()
		}
	}
	return &pb.AdminMFALoginResponse{
		Success:   true,
		Status:    "login_successful",
		AuthToken: accessToken,
		ExpiresIn: expireSeconds,
		UserInfo: &pb.AdminUserInfo{
			AdminId:               resp.AdminIDString(admin.ID),
			Username:              admin.Username,
			Name:                  admin.Name,
			Role:                  roleCode,
			Permissions:           perms,
			RequirePasswordChange: admin.RequirePasswordChange,
		},
		WalletState: walletState,
		Message:     resp.Msg(l.ctx, "LOGIN_SUCCESSFUL"),
		RequestId:   resp.RequestID(l.ctx),
		Timestamp:   resp.Timestamp(),
	}, nil
}
