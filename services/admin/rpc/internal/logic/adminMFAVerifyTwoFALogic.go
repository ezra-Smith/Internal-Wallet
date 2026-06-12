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

type AdminMFAVerifyTwoFALogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminMFAVerifyTwoFALogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMFAVerifyTwoFALogic {
	return &AdminMFAVerifyTwoFALogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminMFAVerifyTwoFALogic) AdminMFAVerifyTwoFA(in *pb.AdminMFAVerifyTwoFARequest) (*pb.AdminMFAVerifyTwoFAResponse, error) {
	if in == nil || strings.TrimSpace(in.SessionIdentifier) == "" || strings.TrimSpace(in.TotpCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"session_identifier": "required",
			"totp_code":          "required",
		})
	}
	if l.svcCtx.MFAFlow == nil || l.svcCtx.AdminUserRepo == nil || l.svcCtx.SessionManager == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	clientIP := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	session, ok, err := l.svcCtx.MFAFlow.GetTwoFASession(l.ctx, in.SessionIdentifier, clientIP, ua)
	if err != nil {
		l.Logger.Errorf("get 2fa session failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	if !ok || session == nil || session.AdminID <= 0 {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuth2FATokenExpired, "AUTH_2FA_SESSION_EXPIRED", "2fa session expired", nil)
	}
	if session.Attempts >= 5 {
		_ = l.svcCtx.MFAFlow.ConsumeTwoFASession(l.ctx, in.SessionIdentifier)
		return nil, errx.New(codes.ResourceExhausted, 429, errx.CodeAuth2FATooManyAttempts, "AUTH_2FA_TOO_MANY_ATTEMPTS", "too many attempts", nil)
	}

	admin, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, session.AdminID)
	if err != nil || admin == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	now := time.Now()
	if admin.LockUntil != nil && admin.LockUntil.After(now) {
		return nil, errx.New(codes.FailedPrecondition, 423, errx.CodeAuthAccountLocked, "AUTH_ACCOUNT_LOCKED", "account locked", nil)
	}
	if admin.Status != "active" {
		return nil, errx.New(codes.PermissionDenied, 403, errx.CodeForbidden, "AUTH_ACCOUNT_DISABLED", "account disabled", nil)
	}
	if strings.TrimSpace(admin.TwoFactorSecret) == "" {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa secret not configured", nil)
	}

	keyMaterial := l.svcCtx.TwoFASecretKeyMaterial()
	secretPlain, decErr := security.DecryptSecretFromStorage(admin.TwoFactorSecret, keyMaterial)
	if decErr != nil {
		l.Logger.Errorf("decrypt 2fa secret failed: %v", decErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	if secretPlain == "" {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa secret not configured", nil)
	}

	ok2, verifyErr := security.VerifyTOTP(secretPlain, strings.TrimSpace(in.TotpCode), now, 1)
	if verifyErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa verify failed", nil)
	}
	if !ok2 {
		_ = l.svcCtx.AdminLoginLogRepo.CreateLog(l.ctx, &model.AdminLoginLogModel{
			AdminID:   admin.ID,
			Action:    "login_failed",
			IP:        clientIP,
			UserAgent: ua,
			Result:    "failed",
			Reason:    "invalid_2fa",
		})

		session.Attempts++
		_ = l.svcCtx.MFAFlow.UpdateTwoFASession(l.ctx, in.SessionIdentifier, session)

		if session.Attempts >= 5 {
			_ = l.svcCtx.MFAFlow.ConsumeTwoFASession(l.ctx, in.SessionIdentifier)
			lockMinutes := int32(30)
			if l.svcCtx.Config.Security.LockoutDurationMinutes > 0 {
				lockMinutes = l.svcCtx.Config.Security.LockoutDurationMinutes
			}
			t := now.Add(time.Duration(lockMinutes) * time.Minute)
			_ = l.svcCtx.AdminUserRepo.SetLockUntil(l.ctx, admin.ID, &t, admin.ID)
			return nil, errx.New(codes.ResourceExhausted, 429, errx.CodeAuth2FATooManyAttempts, "AUTH_2FA_TOO_MANY_ATTEMPTS", "too many attempts", nil)
		}
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuth2FACodeInvalid, "AUTH_2FA_CODE_INVALID", "invalid 2fa code", nil)
	}

	// Consume session identifier on success.
	_ = l.svcCtx.MFAFlow.ConsumeTwoFASession(l.ctx, in.SessionIdentifier)

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
	return &pb.AdminMFAVerifyTwoFAResponse{
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
		Message:   "ok",
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
