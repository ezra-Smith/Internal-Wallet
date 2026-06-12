package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetMySecuritySettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMySecuritySettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMySecuritySettingsLogic {
	return &GetMySecuritySettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Security (My Account) ====================
func (l *GetMySecuritySettingsLogic) GetMySecuritySettings(in *pb.GetMySecuritySettingsRequest) (*pb.GetMySecuritySettingsResponse, error) {
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

	minLen := int32(l.svcCtx.Config.Security.PasswordMinLength)
	if minLen <= 0 {
		minLen = 10
	}

	expDays := int32(90)
	if l.svcCtx.Config.Security.PasswordExpiryDays > 0 {
		expDays = l.svcCtx.Config.Security.PasswordExpiryDays
	}

	passwordChangedAt := formatTimePtrFriendly(admin.PasswordChangedAt)
	nextPasswordExpiry := ""
	if admin.PasswordChangedAt != nil {
		nextPasswordExpiry = formatTimeFriendly(admin.PasswordChangedAt.Add(time.Duration(expDays) * 24 * time.Hour))
	}

	twoFARequired := l.svcCtx.EffectiveRequire2FA(l.ctx) || admin.TwoFactorRequired
	twoFAEnabled := admin.TwoFactorEnabled && strings.TrimSpace(admin.TwoFactorSecret) != ""
	twoFABoundAt := formatTimePtrFriendly(admin.TwoFactorBoundAt)
	hasPending := strings.TrimSpace(admin.TwoFactorPendingSecret) != ""

	return &pb.GetMySecuritySettingsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetMySecuritySettingsData{
			PasswordPolicy: &pb.PasswordPolicy{
				MinLength:      minLen,
				RequireUpper:   true,
				RequireLower:   true,
				RequireDigit:   true,
				RequireSpecial: true,
				ExpiryDays:     expDays,
			},
			PasswordChangedAt:  passwordChangedAt,
			NextPasswordExpiry: nextPasswordExpiry,
			TwoFa: &pb.TwoFAStatus{
				Enabled:         twoFAEnabled,
				Required:        twoFARequired,
				BoundAt:         twoFABoundAt,
				HasPendingSetup: hasPending,
			},
			LastLoginAt: formatTimePtrFriendly(admin.LastLoginAt),
			LastLoginIp: admin.LastLoginIP,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil

}
