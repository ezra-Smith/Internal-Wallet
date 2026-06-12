package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/code"
	"internalwallet/common/middleware"
	"internalwallet/common/security"
	"internalwallet/common/utils"
	"internalwallet/pkg/notify"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetTradePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetTradePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetTradePasswordLogic {
	return &ResetTradePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetTradePasswordLogic) ResetTradePassword(in *pb.ResetTradePasswordReq) (*pb.ResetTradePasswordResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}

	// Validate new trade password
	newTradePassword := strings.TrimSpace(in.NewTradePassword)
	if newTradePassword == "" {
		return nil, errx.NewTradePasswordRequired()
	}
	if len(newTradePassword) < 6 {
		return nil, errx.TradePasswordTooShort(6)
	}

	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	id, _ := strconv.ParseInt(uidStr, 10, 64)
	if id <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	// 验证方式：邮箱验证码 / 短信验证码 / 生物识别 三选一
	hasBio := in.Biometric != nil
	hasCode := strings.TrimSpace(in.Code) != ""
	if hasBio == hasCode {
		// 两者都提供或都未提供都不允许（email/sms 共享 code 字段）
		return nil, errx.InvalidParam("email/sms code or biometric required")
	}

	if hasBio {
		payloadHash, err := calcResetTradePasswordPayloadHash(id, newTradePassword)
		if err != nil {
			return nil, errx.InvalidParam("invalid params")
		}
		if err := verifyBiometricProof(l.ctx, l.svcCtx, id, biometricSceneResetTradePassword, payloadHash, in.Biometric); err != nil {
			return nil, err
		}
	} else {
		// 验证码校验
		length := l.svcCtx.Config.Code.Length
		if length <= 0 {
			length = 6
		}
		expire := l.svcCtx.Config.Code.ExpireSeconds
		if expire <= 0 {
			expire = 600
		}
		gen := code.NewGenerator(l.svcCtx.RedisClient, int(length), time.Duration(expire)*time.Second)
		scene := "reset_trade_password"
		bypass := l.svcCtx.Config.Code.BypassCode

		switch in.Method {
		case pb.LoginMethod_LOGIN_METHOD_PHONE_CODE:
			countryCode, err := normalizeAndValidateCountryCode(l.svcCtx, in.CountryCode)
			if err != nil {
				return nil, err
			}
			phone := strings.TrimSpace(in.Phone)
			if !utils.ValidatePhone(phone) {
				return nil, errx.InvalidParam("invalid phone")
			}
			recipient := countryCode + phone
			if bypass != "" && strings.TrimSpace(in.Code) == bypass {
				// ok
			} else {
				ok, err := gen.Verify(l.ctx, recipient, scene, "sms", strings.TrimSpace(in.Code))
				if err != nil {
					return nil, errx.VerifyFailed("verify failed")
				}
				if !ok {
					return nil, errx.InvalidCode("sms")
				}
			}
			_ = gen.Delete(l.ctx, recipient, scene, "sms")
		default:
			email := strings.ToLower(strings.TrimSpace(in.Email))
			if !utils.ValidateEmail(email) {
				return nil, errx.InvalidParam("invalid email")
			}
			if bypass != "" && strings.TrimSpace(in.Code) == bypass {
				// ok
			} else {
				ok, err := gen.Verify(l.ctx, email, scene, "email", strings.TrimSpace(in.Code))
				if err != nil {
					return nil, errx.VerifyFailed("verify failed")
				}
				if !ok {
					return nil, errx.InvalidCode("email")
				}
			}
			_ = gen.Delete(l.ctx, email, scene, "email")
		}
	}

	// Hash and store the new trade password
	hash, err := security.HashPassword(newTradePassword)
	if err != nil {
		l.Logger.Errorf("Failed to hash trade password: %v", err)
		return nil, errx.Internal("internal error")
	}

	if err := l.svcCtx.UserAccountRepository.SetTradePassword(l.ctx, id, hash); err != nil {
		l.Logger.Errorf("Failed to set trade password: %v", err)
		return nil, errx.DBError()
	}

	// Also update security settings table
	if l.svcCtx.UserSecuritySettingsRepository != nil {
		_ = l.svcCtx.UserSecuritySettingsRepository.SetSecurityHasTradePassword(l.ctx, id, true)
	}

	// 设置安全冷却期（24小时内禁止转账等敏感操作）
	if err := SetSecurityCooldown(l.ctx, l.svcCtx, id, CooldownReasonTradePasswordReset); err != nil {
		l.Logger.Errorf("Failed to set security cooldown for user %d: %v", id, err)
		// 不阻塞主流程，只记录日志
	}

	l.Logger.Infof("Trade password reset successfully for user %d, security cooldown activated", id)

	// 异步发送邮件通知（不阻塞响应）
	ip := middleware.GetClientIP(l.ctx)
	go l.sendEmailNotification(id, ip)

	return &pb.ResetTradePasswordResp{Success: true, Message: "ok"}, nil
}

// sendEmailNotification 发送交易密码重置成功邮件通知
func (l *ResetTradePasswordLogic) sendEmailNotification(userID int64, ip string) {
	// 准备邮件通知数据
	data, err := PrepareEmailNotificationData(l.ctx, l.svcCtx, userID, l.Logger, ip)
	if err != nil {
		// 已在 PrepareEmailNotificationData 中记录日志，这里直接返回
		return
	}

	// 异步发送邮件
	err = notify.SendTradingPasswordChangeEmailAsync(
		data.Email,
		data.Username,
		data.UID,
		data.ChangeTime,
		data.IPAddress,
		data.Location,
	)

	if err != nil {
		l.Logger.Errorf("Failed to send trading password reset email to %s: %v", data.Email, err)
	} else {
		l.Logger.Infof("Trading password reset email sent successfully to %s", data.Email)
	}
}
