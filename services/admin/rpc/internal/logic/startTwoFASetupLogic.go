package logic

import (
	"context"
	"encoding/base64"
	"strings"

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

type StartTwoFASetupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStartTwoFASetupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartTwoFASetupLogic {
	return &StartTwoFASetupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StartTwoFASetupLogic) StartTwoFASetup(in *pb.StartTwoFASetupRequest) (*pb.StartTwoFASetupResponse, error) {
	_ = in
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
	if admin.TwoFactorEnabled && strings.TrimSpace(admin.TwoFactorSecret) != "" {
		return nil, errx.New(codes.FailedPrecondition, 400, errx.CodeInvalidParam, "TWO_FA_ALREADY_ENABLED", "2FA 已启用", nil)
	}

	keyMaterial := l.svcCtx.TwoFASecretKeyMaterial()
	pendingStored := strings.TrimSpace(admin.TwoFactorPendingSecret)
	pendingPlain := ""
	if pendingStored != "" {
		plain, decErr := security.DecryptSecretFromStorage(pendingStored, keyMaterial)
		if decErr != nil {
			l.Logger.Errorf("decrypt pending 2fa secret failed: %v", decErr)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		pendingPlain = strings.TrimSpace(plain)
	}
	if pendingPlain == "" {
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
		pendingPlain = plain
		pendingStored = enc
	}

	issuer := "Zink Wallet Admin"
	otpauthURL := security.BuildTOTPAuthURL(issuer, admin.Username, pendingPlain)
	if otpauthURL == "" {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa url build failed", nil)
	}

	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 256)
	if err != nil {
		l.Logger.Errorf("qrcode encode failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "qrcode generate failed", nil)
	}
	qr := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)

	return &pb.StartTwoFASetupResponse{
		Success: true,
		Message: "ok",
		Data: &pb.TwoFASetupPayload{
			QrCode:     qr,
			Secret:     pendingPlain,
			OtpauthUrl: otpauthURL,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
