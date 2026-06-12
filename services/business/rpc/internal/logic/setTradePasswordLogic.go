package logic

import (
	"context"
	"internalwallet/common/middleware"
	"internalwallet/pkg/notify"
	"strconv"
	"strings"

	"internalwallet/common/security"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetTradePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetTradePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetTradePasswordLogic {
	return &SetTradePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetTradePasswordLogic) SetTradePassword(in *pb.SetTradePasswordReq) (*pb.SetTradePasswordResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	id, _ := strconv.ParseInt(uidStr, 10, 64)

	// Validate login password first
	//if strings.TrimSpace(in.Password) == "" {
	//	return nil, errx.InvalidParam("password required")
	//}
	//user, err := l.svcCtx.UserAccountRepository.GetAuthByID(l.ctx, id)
	//if err != nil {
	//	return nil, errx.UserNotFound()
	//}
	//if user.PasswordHash == "" || !security.VerifyPassword(user.PasswordHash, in.Password) {
	//	return nil, errx.InvalidPassword()
	//}

	// Validate trade password
	tradePassword := strings.TrimSpace(in.TradePassword)
	if tradePassword == "" {
		return nil, errx.TradePasswordRequired()
	}
	if len(tradePassword) < 6 {
		return nil, errx.TradePasswordTooShort(6)
	}

	// 检查是否是首次设置（用于判断是否需要触发冷却期）
	_, hadTradePassword, _ := l.svcCtx.UserAccountRepository.GetTradePasswordHash(l.ctx, id)

	// Hash and store trade password
	hash, err := security.HashPassword(tradePassword)
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

	// 只有修改交易密码（非首次设置）才触发冷却期
	// 首次设置交易密码是正常的安全加固操作，不需要冷却期
	if hadTradePassword {
		if err := SetSecurityCooldown(l.ctx, l.svcCtx, id, CooldownReasonTradePasswordSet); err != nil {
			l.Logger.Errorf("Failed to set security cooldown for user %d: %v", id, err)
			// 不阻塞主流程，只记录日志
		}
		l.Logger.Infof("Trade password changed for user %d, security cooldown activated", id)
	} else {
		l.Logger.Infof("Trade password set for first time for user %d, no cooldown needed", id)
	}

	// 异步发送邮件通知（不阻塞响应）
	go l.sendEmailNotification(id, middleware.GetClientIP(l.ctx))

	return &pb.SetTradePasswordResp{Success: true, Message: "ok"}, nil
}

// sendEmailNotification 发送交易密码修改成功邮件通知
func (l *SetTradePasswordLogic) sendEmailNotification(userID int64, ip string) {
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
		l.Logger.Errorf("Failed to send trading password change email to %s: %v", data.Email, err)
	} else {
		l.Logger.Infof("Trading password change email sent successfully to %s", data.Email)
	}
}
