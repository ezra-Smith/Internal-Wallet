package logic

import (
	"context"
	"encoding/base64"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/skip2/go-qrcode"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AdminMFAGetTwoFAQrCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminMFAGetTwoFAQrCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMFAGetTwoFAQrCodeLogic {
	return &AdminMFAGetTwoFAQrCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminMFAGetTwoFAQrCodeLogic) AdminMFAGetTwoFAQrCode(in *pb.AdminMFAGetTwoFAQrCodeRequest) (*pb.AdminMFAGetTwoFAQrCodeResponse, error) {
	if in == nil || strings.TrimSpace(in.AuthKey) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"auth_key": "required",
		})
	}
	if l.svcCtx.MFAFlow == nil || l.svcCtx.AdminUserRepo == nil {
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

	if strings.TrimSpace(session.PendingSecret) == "" {
		secret, genErr := security.GenerateTOTPSecret(20)
		if genErr != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa secret generate failed", nil)
		}
		session.PendingSecret = secret
		if err := l.svcCtx.MFAFlow.SetAuthSession(l.ctx, in.AuthKey, session); err != nil {
			l.Logger.Errorf("set auth session failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
	}

	issuer := "Admin Backend"
	accountName := admin.Username
	otpauthURL := security.BuildTOTPAuthURL(issuer, accountName, session.PendingSecret)
	if otpauthURL == "" {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "2fa url build failed", nil)
	}

	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 256)
	if err != nil {
		l.Logger.Errorf("qrcode encode failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "qrcode generate failed", nil)
	}
	qr := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)

	return &pb.AdminMFAGetTwoFAQrCodeResponse{
		Success:     true,
		QrCode:      qr,
		SecretKey:   session.PendingSecret,
		Issuer:      issuer,
		AccountName: accountName,
		Message:     "ok",
		RequestId:   resp.RequestID(l.ctx),
		Timestamp:   resp.Timestamp(),
	}, nil
}
