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

type EnableGoogleAuth2FALogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnableGoogleAuth2FALogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnableGoogleAuth2FALogic {
	return &EnableGoogleAuth2FALogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// EnableGoogleAuth2FA 开启双重认证（已绑定GA，传验证码开启）
func (l *EnableGoogleAuth2FALogic) EnableGoogleAuth2FA(in *pb.EnableGoogleAuth2FAReq) (*pb.EnableGoogleAuth2FAResp, error) {
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

	// Check if 2FA is already enabled
	if user.Is2faEnabled {
		return &pb.EnableGoogleAuth2FAResp{Success: true, Message: "2FA is already enabled"}, nil
	}

	// Verify Google Auth code
	if !totp.Validate(googleCode, googleSecret) {
		return nil, errx.InvalidGoogleAuthCode()
	}

	// Enable 2FA (set is_2fa_enabled to true)
	if err := l.svcCtx.UserAccountRepository.GetDB().WithContext(l.ctx).Model(&model.UserModel{}).
		Where("id = ?", uid).
		Updates(map[string]interface{}{
			"is_2fa_enabled": true,
		}).Error; err != nil {
		l.Logger.Errorf("enable 2fa failed: %v", err)
		return nil, errx.DBError()
	}

	// Update member_security_setting.google_auth_enabled
	if l.svcCtx.UserSecuritySettingsRepository != nil {
		if err := l.svcCtx.UserSecuritySettingsRepository.SetGoogleAuthEnabled(l.ctx, uid, true); err != nil {
			return nil, errx.DBError()
		}
	}

	// Best-effort: record 2FA history
	if l.svcCtx.User2FAHistoryRepository != nil {
		metaBytes, _ := json.Marshal(map[string]interface{}{
			"verify_method": "totp",
			"action":        "enable_2fa",
		})
		_ = l.svcCtx.User2FAHistoryRepository.Create(l.ctx, &model.User2FAHistoryModel{
			UserID:       uid,
			Event:        "enable",
			Factor:       "totp",
			OperatorType: "user",
			OperatorID:   uid,
			Reason:       "verified_by_totp",
			IP:           middleware.GetClientIP(l.ctx),
			UserAgent:    middleware.GetUserAgent(l.ctx),
			Meta:         metaBytes,
		})
	}

	// 异步发送邮件通知（不阻塞响应）
	go Send2FAEnabledEmailNotification(context.Background(), l.svcCtx, uid, middleware.GetClientIP(l.ctx), l.Logger)

	return &pb.EnableGoogleAuth2FAResp{Success: true, Message: "ok"}, nil
}
