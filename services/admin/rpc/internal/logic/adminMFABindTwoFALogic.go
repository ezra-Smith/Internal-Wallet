package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AdminMFABindTwoFALogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminMFABindTwoFALogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMFABindTwoFALogic {
	return &AdminMFABindTwoFALogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminMFABindTwoFALogic) AdminMFABindTwoFA(in *pb.AdminMFABindTwoFARequest) (*pb.AdminMFABindTwoFAResponse, error) {
	if in == nil || strings.TrimSpace(in.AuthKey) == "" || strings.TrimSpace(in.TotpCode) == "" || strings.TrimSpace(in.SecretKey) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"auth_key":   "required",
			"totp_code":  "required",
			"secret_key": "required",
		})
	}
	if l.svcCtx.MFAFlow == nil || l.svcCtx.AdminUserRepo == nil || l.svcCtx.SessionManager == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	clientIP := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	session, ok, err := l.svcCtx.MFAFlow.GetAuthSession(l.ctx, in.AuthKey, clientIP, ua)
	if err != nil {
		l.Logger.Errorf("get auth session failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	if !ok || session == nil || session.AdminID <= 0 {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_KEY_INVALID", "unauthorized", nil)
	}

	admin, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, session.AdminID)
	if err != nil || admin == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_KEY_INVALID", "unauthorized", nil)
	}
	if admin.Status != "active" {
		return nil, errx.New(codes.PermissionDenied, 403, errx.CodeForbidden, "AUTH_ACCOUNT_DISABLED", "account disabled", nil)
	}
	if admin.TwoFactorEnabled {
		return nil, errx.New(codes.FailedPrecondition, 400, errx.CodeInvalidParam, "TWO_FA_ALREADY_ENABLED", "2FA already enabled", nil)
	}

	// Ensure the secret is bound to this auth_key session.
	if strings.TrimSpace(session.PendingSecret) == "" {
		return nil, errx.New(codes.FailedPrecondition, 400, errx.CodeInvalidParam, "TWO_FA_SETUP_REQUIRED", "Please request QR code first", nil)
	}
	if strings.TrimSpace(in.SecretKey) != strings.TrimSpace(session.PendingSecret) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SECRET_KEY", "invalid secret_key", nil)
	}

	ok2, verifyErr := security.VerifyTOTP(strings.TrimSpace(in.SecretKey), strings.TrimSpace(in.TotpCode), time.Now(), 1)
	if verifyErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa verify failed", nil)
	}
	if !ok2 {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuth2FACodeInvalid, "AUTH_2FA_CODE_INVALID", "invalid 2fa code", nil)
	}

	keyMaterial := l.svcCtx.TwoFASecretKeyMaterial()
	enc, encErr := security.EncryptSecretForStorage(strings.TrimSpace(in.SecretKey), keyMaterial)
	if encErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	now := time.Now()
	if err := l.svcCtx.AdminUserRepo.PromoteTwoFactorPending(l.ctx, admin.ID, enc, &now, admin.ID); err != nil {
		l.Logger.Errorf("enable 2fa failed: %v", err)
		if col, ok := errx.MySQLColumnNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING",
				"security schema not initialized (missing table: admin_users missing column: "+col+")", nil)
		}
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	backupCodes := make([]string, 0)
	if l.svcCtx.BackupCodeRepo != nil {
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

	// Issue final auth token.
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

	_ = l.svcCtx.MFAFlow.ConsumeAuthKey(l.ctx, in.AuthKey)

	return &pb.AdminMFABindTwoFAResponse{
		Success:     true,
		Status:      "2fa_bound_successfully",
		AuthToken:   accessToken,
		ExpiresIn:   expireSeconds,
		BackupCodes: backupCodes,
		Message:     "ok",
		RequestId:   resp.RequestID(l.ctx),
		Timestamp:   resp.Timestamp(),
	}, nil
}
