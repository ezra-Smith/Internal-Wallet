package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type ChangeMyPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeMyPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeMyPasswordLogic {
	return &ChangeMyPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChangeMyPasswordLogic) ChangeMyPassword(in *pb.ChangeMyPasswordRequest) (*pb.ChangeMyPasswordResponse, error) {
	if in == nil || in.CurrentPassword == "" || in.NewPassword == "" || in.ConfirmPassword == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", nil)
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminUserRepo == nil || l.svcCtx.PasswordHistoryRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	if !security.VerifyPassword(current.PasswordHash, in.CurrentPassword) {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeAuthInvalidCredentials, "AUTH_INVALID_CREDENTIALS", "当前密码错误", nil)
	}
	if in.NewPassword != in.ConfirmPassword {
		return nil, errx.New(codes.InvalidArgument, 422, errx.CodePasswordMismatch, "PASSWORD_MISMATCH", "密码不一致", map[string]string{"confirm_password": "not match"})
	}
	if in.NewPassword == in.CurrentPassword {
		return nil, errx.New(codes.InvalidArgument, 422, errx.CodePasswordTooWeak, "PASSWORD_SAME_AS_OLD", "新密码不能与旧密码相同", map[string]string{"new_password": "same"})
	}

	minLen := int(l.svcCtx.Config.Security.PasswordMinLength)
	if minLen <= 0 {
		minLen = 10
	}
	if err := security.ValidatePasswordPolicy(in.NewPassword, minLen); err != nil {
		return nil, errx.NewWithMetadata(codes.InvalidArgument, 422, errx.CodePasswordTooWeak, "PASSWORD_TOO_WEAK", err.Error(), map[string]string{"new_password": "too weak"}, map[string]string{
			"min_length": strconv.Itoa(minLen),
		})
	}
	passLower := strings.ToLower(in.NewPassword)
	userLower := strings.ToLower(strings.TrimSpace(current.Username))
	if userLower != "" && strings.Contains(passLower, userLower) {
		return nil, errx.New(codes.InvalidArgument, 422, errx.CodePasswordTooWeak, "PASSWORD_CONTAINS_USERNAME", "password must not contain username", map[string]string{"new_password": "contains username"})
	}
	if at := strings.Index(userLower, "@"); at > 0 {
		local := userLower[:at]
		if local != "" && strings.Contains(passLower, local) {
			return nil, errx.New(codes.InvalidArgument, 422, errx.CodePasswordTooWeak, "PASSWORD_CONTAINS_USERNAME", "password must not contain username", map[string]string{"new_password": "contains username"})
		}
	}

	// 禁止最近5次重复
	histories, hErr := l.svcCtx.PasswordHistoryRepo.ListRecent(l.ctx, current.ID, 5)
	if hErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	for _, h := range histories {
		if h == nil {
			continue
		}
		if security.VerifyPassword(h.PasswordHash, in.NewPassword) {
			return nil, errx.New(codes.InvalidArgument, 422, errx.CodePasswordRecentlyUsed, "PASSWORD_RECENTLY_USED", "新密码不能与最近 5 次相同", nil)
		}
	}

	newHash, hashErr := security.HashPassword(in.NewPassword)
	if hashErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "password hash failed", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"admin_id": resp.AdminIDString(current.ID),
	})

	newTokenVersion := int64(0)
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		adminRepo := repository.NewAdminUserRepository(tx)
		historyRepo := repository.NewAdminPasswordHistoryRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := adminRepo.UpdatePassword(l.ctx, current.ID, newHash, false, current.ID); err != nil {
			return err
		}
		if err := historyRepo.Add(l.ctx, current.ID, newHash); err != nil {
			return err
		}
		// Revoke all tokens by bumping token_version (even if Redis is unavailable).
		v, err := adminRepo.IncrementTokenVersion(l.ctx, current.ID, current.ID)
		if err != nil {
			return err
		}
		newTokenVersion = v
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "admin.change_password",
			TargetType:  "admin",
			TargetID:    resp.AdminIDString(current.ID),
			Description: "管理员修改密码",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("change password failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Revoke all server-side sessions (best-effort; token_version++ already ensures revocation).
	_ = l.svcCtx.SessionManager.RemoveAll(l.ctx, current.ID)

	// Issue a fresh token so the client can continue with post-password-change flows (e.g., bind 2FA).
	expireSeconds := int64(28800)
	if l.svcCtx.Config.JWT.AccessExpire > 0 {
		expireSeconds = l.svcCtx.Config.JWT.AccessExpire
	}
	accessToken := ""
	tokenExpiresIn := int64(0)
	if newTokenVersion > 0 {
		if t, meta, genErr := security.GenerateAdminAccessToken(current.ID, current.Username, newTokenVersion, l.svcCtx.Config.JWT.AccessSecret, expireSeconds); genErr != nil {
			l.Logger.Errorf("generate new token after password change failed (skipped): %v", genErr)
		} else {
			accessToken = t
			tokenExpiresIn = expireSeconds
			_ = l.svcCtx.SessionManager.Add(l.ctx, current.ID, meta.JTI, meta.Exp)
		}
	}

	expDays := int32(90)
	if l.svcCtx.Config.Security.PasswordExpiryDays > 0 {
		expDays = l.svcCtx.Config.Security.PasswordExpiryDays
	}
	nextExpiry := now.Add(time.Duration(expDays) * 24 * time.Hour)

	return &pb.ChangeMyPasswordResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "PASSWORD_CHANGED"),
		Data: &pb.ChangeMyPasswordData{
			PasswordChangedAt: formatTime(now),
			NextExpiry:        formatTime(nextExpiry),
		},
		AuthToken: accessToken,
		ExpiresIn: tokenExpiresIn,
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
