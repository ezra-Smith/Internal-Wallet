package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/pquerna/otp/totp"
	"github.com/zeromicro/go-zero/core/logx"
)

type DisableGoogleAuth2FAWithCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDisableGoogleAuth2FAWithCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableGoogleAuth2FAWithCodeLogic {
	return &DisableGoogleAuth2FAWithCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DisableGoogleAuth2FAWithCode 关闭双重认证（已绑定GA，传验证码关闭）
func (l *DisableGoogleAuth2FAWithCodeLogic) DisableGoogleAuth2FAWithCode(in *pb.DisableGoogleAuth2FAWithCodeReq) (*pb.DisableGoogleAuth2FAWithCodeResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}

	googleCode := strings.TrimSpace(in.GoogleCode)
	if googleCode == "" {
		return nil, errx.InvalidParam("google auth code is required")
	}

	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return nil, errx.InvalidUser()
	}

	// Get user info to check if GA is bound
	user, err := l.svcCtx.UserAccountRepository.GetAuthByID(l.ctx, uid)
	if err != nil || user == nil {
		return nil, errx.UserNotFound()
	}

	// Check if Google Auth is bound
	googleSecret := strings.TrimSpace(user.GoogleAuthSecret)
	if googleSecret == "" {
		return nil, errx.GoogleAuthNotBound()
	}

	// Check if 2FA is already disabled
	if !user.Is2faEnabled {
		return &pb.DisableGoogleAuth2FAWithCodeResp{Success: true, Message: "2FA is already disabled"}, nil
	}

	// Verify Google Auth code
	if !totp.Validate(googleCode, googleSecret) {
		return nil, errx.InvalidGoogleAuthCode()
	}

	// Disable 2FA (set is_2fa_enabled to false, but keep google_auth_secret)
	if err := l.svcCtx.UserAccountRepository.GetDB().WithContext(l.ctx).Model(&model.UserModel{}).
		Where("id = ?", uid).
		Updates(map[string]interface{}{
			"is_2fa_enabled": false,
		}).Error; err != nil {
		l.Logger.Errorf("disable 2fa failed: %v", err)
		return nil, errx.DBError()
	}

	// Update member_security_setting.google_auth_enabled
	if l.svcCtx.UserSecuritySettingsRepository != nil {
		if err := l.svcCtx.UserSecuritySettingsRepository.SetGoogleAuthEnabled(l.ctx, uid, false); err != nil {
			return nil, errx.DBError()
		}
	}

	ip := middleware.GetClientIP(l.ctx)
	// Best-effort: record 2FA history
	if l.svcCtx.User2FAHistoryRepository != nil {
		metaBytes, _ := json.Marshal(map[string]interface{}{
			"verify_method": "totp",
			"action":        "disable_2fa",
		})
		_ = l.svcCtx.User2FAHistoryRepository.Create(l.ctx, &model.User2FAHistoryModel{
			UserID:       uid,
			Event:        "disable",
			Factor:       "totp",
			OperatorType: "user",
			OperatorID:   uid,
			Reason:       "verified_by_totp",
			IP:           ip,
			UserAgent:    middleware.GetUserAgent(l.ctx),
			Meta:         metaBytes,
		})
	}
	// 异步发送邮件通知（不阻塞响应）
	go Send2FADisabledEmailNotification(context.Background(), l.svcCtx, uid, ip, "totp", l.Logger)
	return &pb.DisableGoogleAuth2FAWithCodeResp{Success: true, Message: "ok"}, nil
}
