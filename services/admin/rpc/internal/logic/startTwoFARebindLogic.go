package logic

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/skip2/go-qrcode"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type StartTwoFARebindLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStartTwoFARebindLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartTwoFARebindLogic {
	return &StartTwoFARebindLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StartTwoFARebindLogic) StartTwoFARebind(in *pb.StartTwoFARebindRequest) (*pb.StartTwoFARebindResponse, error) {
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

	plain, genErr := security.GenerateTOTPSecret(20)
	if genErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa secret generate failed", nil)
	}
	enc, encErr := security.EncryptSecretForStorage(plain, keyMaterial)
	if encErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	if err := l.svcCtx.AdminUserRepo.UpdateTwoFactorPendingSecret(l.ctx, admin.ID, enc, admin.ID); err != nil {
		l.Logger.Errorf("update pending 2fa secret failed: %v", err)
		if col, ok := errx.MySQLColumnNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING",
				"security schema not initialized (missing table: admin_users missing column: "+col+")", nil)
		}
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	issuer := "Zink Wallet Admin"
	otpauthURL := security.BuildTOTPAuthURL(issuer, admin.Username, plain)
	if otpauthURL == "" {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa url build failed", nil)
	}
	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 256)
	if err != nil {
		l.Logger.Errorf("qrcode encode failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "qrcode generate failed", nil)
	}
	qr := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)

	return &pb.StartTwoFARebindResponse{
		Success: true,
		Message: "ok",
		Data: &pb.TwoFASetupPayload{
			QrCode:     qr,
			Secret:     plain,
			OtpauthUrl: otpauthURL,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
