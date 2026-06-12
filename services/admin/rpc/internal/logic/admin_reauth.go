package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"google.golang.org/grpc/codes"
)

func requireAdminReauth(ctx context.Context, svcCtx *svc.ServiceContext, reauth *pb.AdminReauth) (*model.AdminUserModel, error) {
	if reauth == nil || strings.TrimSpace(reauth.AdminPassword) == "" || strings.TrimSpace(reauth.TotpCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "reauth required", map[string]string{
			"reauth.admin_password": "required",
			"reauth.totp_code":      "required",
		})
	}
	if svcCtx == nil || svcCtx.AdminUserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	admin, err := svcCtx.AdminUserRepo.FindByID(ctx, current.ID)
	if err != nil || admin == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if !security.VerifyPassword(admin.PasswordHash, strings.TrimSpace(reauth.AdminPassword)) {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuthInvalidCredentials, "AUTH_INVALID_CREDENTIALS", "invalid credentials", nil)
	}
	if !admin.TwoFactorEnabled || strings.TrimSpace(admin.TwoFactorSecret) == "" {
		return nil, errx.New(codes.FailedPrecondition, 428, errx.CodeForbidden, "TWO_FA_REQUIRED", "2fa required", nil)
	}

	keyMaterial := svcCtx.TwoFASecretKeyMaterial()
	secretPlain, decErr := security.DecryptSecretFromStorage(admin.TwoFactorSecret, keyMaterial)
	if decErr != nil || strings.TrimSpace(secretPlain) == "" {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa secret invalid", nil)
	}

	ok2, verifyErr := security.VerifyTOTP(strings.TrimSpace(secretPlain), strings.TrimSpace(reauth.TotpCode), time.Now(), 1)
	if verifyErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa verify failed", nil)
	}
	if !ok2 {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuth2FACodeInvalid, "AUTH_2FA_CODE_INVALID", "invalid 2fa code", nil)
	}
	return admin, nil
}
