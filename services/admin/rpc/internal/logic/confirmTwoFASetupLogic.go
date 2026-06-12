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

type ConfirmTwoFASetupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmTwoFASetupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmTwoFASetupLogic {
	return &ConfirmTwoFASetupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfirmTwoFASetupLogic) ConfirmTwoFASetup(in *pb.ConfirmTwoFASetupRequest) (*pb.ConfirmTwoFASetupResponse, error) {
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

	if !security.VerifyPassword(admin.PasswordHash, strings.TrimSpace(in.CurrentPassword)) {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuthInvalidCredentials, "AUTH_INVALID_CREDENTIALS", "当前密码错误", nil)
	}

	pendingStored := strings.TrimSpace(admin.TwoFactorPendingSecret)
	if pendingStored == "" {
		return nil, errx.New(codes.FailedPrecondition, 400, errx.CodeInvalidParam, "TWO_FA_SETUP_REQUIRED", "请先获取 2FA 绑定信息", nil)
	}

	keyMaterial := l.svcCtx.TwoFASecretKeyMaterial()
	pendingPlain, decErr := security.DecryptSecretFromStorage(pendingStored, keyMaterial)
	if decErr != nil {
		l.Logger.Errorf("decrypt pending 2fa secret failed: %v", decErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	ok2, verifyErr := security.VerifyTOTP(strings.TrimSpace(pendingPlain), strings.TrimSpace(in.TotpCode), time.Now(), 1)
	if verifyErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa verify failed", nil)
	}
	if !ok2 {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuth2FACodeInvalid, "AUTH_2FA_CODE_INVALID", "invalid 2fa code", nil)
	}

	now := time.Now()
	if err := l.svcCtx.AdminUserRepo.PromoteTwoFactorPending(l.ctx, admin.ID, pendingStored, &now, admin.ID); err != nil {
		l.Logger.Errorf("promote 2fa pending secret failed: %v", err)
		if col, ok := errx.MySQLColumnNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING",
				"security schema not initialized (missing table: admin_users missing column: "+col+")", nil)
		}
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Audit log (do not record secrets).
	if l.svcCtx.AdminAuditLogRepo != nil {
		action := "admin.2fa.enable"
		description := "管理员启用 2FA"
		if admin.TwoFactorEnabled && strings.TrimSpace(admin.TwoFactorSecret) != "" {
			action = "admin.2fa.rebind"
			description = "管理员重绑 2FA"
		}

		ip := middleware.GetClientIP(l.ctx)
		ua := middleware.GetUserAgent(l.ctx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{})
		_ = l.svcCtx.AdminAuditLogRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     admin.ID,
			Action:      action,
			TargetType:  "admin",
			TargetID:    resp.AdminIDString(admin.ID),
			Description: description,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
	}

	backupCodes := make([]string, 0)
	if l.svcCtx.BackupCodeRepo != nil {
		keyMaterial := l.svcCtx.TwoFASecretKeyMaterial()
		codesPlain, hashes, genErr := generateBackupCodes(10, keyMaterial, admin.ID)
		if genErr != nil {
			// Backup codes are optional; don't block binding.
			l.Logger.Errorf("generate backup codes failed (skipped): %v", genErr)
		} else if err := l.svcCtx.BackupCodeRepo.ReplaceAll(l.ctx, admin.ID, hashes, admin.ID); err != nil {
			// The DB table may not exist if migrations are not applied; skip safely.
			l.Logger.Errorf("store backup codes failed (skipped): %v", err)
		} else {
			backupCodes = codesPlain
		}
	}

	return &pb.ConfirmTwoFASetupResponse{
		Success:     true,
		Message:     resp.Msg(l.ctx, "TWO_FA_BOUND"),
		BackupCodes: backupCodes,
		Data: &pb.ConfirmTwoFASetupData{
			Enabled: true,
			BoundAt: formatTime(now),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
