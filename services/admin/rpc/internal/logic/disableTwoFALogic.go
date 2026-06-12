package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type DisableTwoFALogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDisableTwoFALogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableTwoFALogic {
	return &DisableTwoFALogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DisableTwoFALogic) DisableTwoFA(in *pb.DisableTwoFARequest) (*pb.DisableTwoFAResponse, error) {
	if in == nil || strings.TrimSpace(in.CurrentPassword) == "" || strings.TrimSpace(in.TotpCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"current_password": "required",
			"totp_code":        "required",
		})
	}
	if l.svcCtx == nil || l.svcCtx.AdminUserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	admin, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, current.ID)
	if err != nil || admin == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if !admin.TwoFactorEnabled || strings.TrimSpace(admin.TwoFactorSecret) == "" {
		return nil, errx.New(codes.FailedPrecondition, 400, errx.CodeInvalidParam, "TWO_FA_NOT_ENABLED", "2FA 未启用", nil)
	}

	twoFARequired := l.svcCtx.EffectiveRequire2FA(l.ctx) || admin.TwoFactorRequired
	if twoFARequired {
		return nil, errx.New(codes.FailedPrecondition, 428, errx.CodeForbidden, "TWO_FA_REQUIRED", "2FA 已被策略强制启用，无法关闭", nil)
	}

	if !security.VerifyPassword(admin.PasswordHash, strings.TrimSpace(in.CurrentPassword)) {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuthInvalidCredentials, "AUTH_INVALID_CREDENTIALS", "当前密码错误", nil)
	}

	keyMaterial := l.svcCtx.TwoFASecretKeyMaterial()
	activePlain, decErr := security.DecryptSecretFromStorage(admin.TwoFactorSecret, keyMaterial)
	if decErr != nil {
		l.Logger.Errorf("decrypt active 2fa secret failed: %v", decErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	ok2, verifyErr := security.VerifyTOTP(strings.TrimSpace(activePlain), strings.TrimSpace(in.TotpCode), time.Now(), 1)
	if verifyErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa verify failed", nil)
	}
	if !ok2 {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuth2FACodeInvalid, "AUTH_2FA_CODE_INVALID", "invalid 2fa code", nil)
	}

	now := time.Now()
	if err := l.svcCtx.AdminUserRepo.DisableTwoFactor(l.ctx, admin.ID, admin.ID); err != nil {
		l.Logger.Errorf("disable 2fa failed: %v", err)
		if col, ok := errx.MySQLColumnNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING",
				"security schema not initialized (missing table: admin_users missing column: "+col+")", nil)
		}
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Audit log (do not record secrets).
	if l.svcCtx.AdminAuditLogRepo != nil {
		ip := middleware.GetClientIP(l.ctx)
		ua := middleware.GetUserAgent(l.ctx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{})
		_ = l.svcCtx.AdminAuditLogRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     admin.ID,
			Action:      "admin.2fa.disable",
			TargetType:  "admin",
			TargetID:    resp.AdminIDString(admin.ID),
			Description: "管理员关闭 2FA",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
	}

	return &pb.DisableTwoFAResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "TWO_FA_DISABLED"),
		Data: &pb.DisableTwoFAData{
			Enabled:    false,
			DisabledAt: formatTime(now),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
