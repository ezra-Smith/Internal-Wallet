package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"google.golang.org/grpc/codes"
)

func verifyAdminTwoFA(svcCtx *svc.ServiceContext, ctx context.Context, adminID int64, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "AUTH_2FA_CODE_REQUIRED", "missing 2fa code", map[string]string{"two_fa_code": "required"})
	}
	if svcCtx == nil || svcCtx.AdminUserRepo == nil {
		return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	admin, err := svcCtx.AdminUserRepo.FindByID(ctx, adminID)
	if err != nil || admin == nil {
		return errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if !admin.TwoFactorEnabled || strings.TrimSpace(admin.TwoFactorSecret) == "" {
		return errx.New(codes.FailedPrecondition, 428, errx.CodeAuthTwoFABindReq, "TWO_FA_BIND_REQUIRED", "需要绑定2FA", nil)
	}

	keyMaterial := svcCtx.TwoFASecretKeyMaterial()
	secretPlain, decErr := security.DecryptSecretFromStorage(admin.TwoFactorSecret, keyMaterial)
	if decErr != nil {
		return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	ok, verifyErr := security.VerifyTOTP(secretPlain, code, time.Now(), 1)
	if verifyErr != nil {
		return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa verify failed", nil)
	}
	if !ok {
		return errx.New(codes.Unauthenticated, 401, errx.CodeAuth2FACodeInvalid, "AUTH_2FA_CODE_INVALID", "invalid 2fa code", nil)
	}
	return nil
}
