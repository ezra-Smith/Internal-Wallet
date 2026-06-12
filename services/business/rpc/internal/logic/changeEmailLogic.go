package logic

import (
	"context"
	"fmt"
	"internalwallet/pkg/notify"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/code"
	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeEmailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeEmailLogic {
	return &ChangeEmailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChangeEmailLogic) ChangeEmail(in *pb.ChangeEmailReq) (*pb.ChangeEmailResp, error) {
	if !utils.ValidateEmail(in.NewEmail) || in.Code1 == "" || in.Code2 == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return nil, errx.InvalidUser()
	}

	accountRepo := l.svcCtx.UserAccountRepository
	securityRepo := l.svcCtx.UserSecuritySettingsRepository

	// 获取当前用户的原邮箱
	profile, err := accountRepo.GetByID(l.ctx, uid)
	if err != nil || profile == nil {
		return nil, errx.UserNotFound()
	}
	oldEmail := profile.Email

	// 检查用户是否已绑定邮箱
	if oldEmail == "" {
		return nil, errx.NoEmailBound()
	}

	// 检查新邮箱与旧邮箱是否相同
	if oldEmail == in.NewEmail {
		return nil, errx.InvalidParam("new email cannot be the same as old email")
	}

	// 检查新邮箱是否已被其他用户使用
	existingUser, err := accountRepo.GetByEmail(l.ctx, in.NewEmail)
	if err == nil && existingUser != nil && existingUser.ID != uid {
		return nil, errx.EmailAlreadyInUse()
	}

	// 验证码校验配置
	length := l.svcCtx.Config.Code.Length
	if length <= 0 {
		length = 6
	}
	expire := l.svcCtx.Config.Code.ExpireSeconds
	if expire <= 0 {
		expire = 600
	}
	gen := code.NewGenerator(l.svcCtx.RedisClient, int(length), time.Duration(expire)*time.Second)
	bypass := l.svcCtx.Config.Code.BypassCode

	// 验证原邮箱验证码 (code1)
	if oldEmail != "" {
		scene1 := "bind_email"
		codeType1 := "email"
		var ok1 bool
		if bypass != "" && in.Code1 == bypass {
			ok1 = true
		} else {
			ok1, err = gen.Verify(l.ctx, oldEmail, scene1, codeType1, in.Code1)
			if err != nil {
				l.Errorf("验证原邮箱验证码失败: %v", err)
				return nil, errx.InvalidOldEmailCode()
			}
		}
		if !ok1 {
			return nil, errx.InvalidOldEmailCode()
		}
		// 删除已使用的验证码
		_ = gen.Delete(l.ctx, oldEmail, scene1, codeType1)
	}

	// 验证新邮箱验证码 (code2)
	scene2 := "bind_email"
	codeType2 := "email"
	var ok2 bool
	if bypass != "" && in.Code2 == bypass {
		ok2 = true
	} else {
		ok2, err = gen.Verify(l.ctx, in.NewEmail, scene2, codeType2, in.Code2)
		if err != nil {
			l.Errorf("验证新邮箱验证码失败: %v", err)
			return nil, errx.InvalidNewEmailCode()
		}
	}
	if !ok2 {
		return nil, errx.InvalidNewEmailCode()
	}

	if err := accountRepo.UpsertEmailByID(l.ctx, uid, in.NewEmail); err != nil {
		return nil, errx.DBError()
	}
	_ = securityRepo.UpdateEmailBoundAndMask(l.ctx, uid, in.NewEmail)

	// 删除已使用的验证码
	_ = gen.Delete(l.ctx, in.NewEmail, scene2, codeType2)

	// 异步发送邮件通知（不阻塞响应）
	// 同时向新旧邮箱发送通知
	go l.sendEmailNotification(uid, oldEmail, in.NewEmail)

	return &pb.ChangeEmailResp{Success: true, Message: "ok"}, nil
}

// sendEmailNotification 发送邮箱修改成功邮件通知
func (l *ChangeEmailLogic) sendEmailNotification(userID int64, oldEmail, newEmail string) {
	// 获取用户信息（用于获取用户名等基础信息）
	user, err := l.svcCtx.UserAccountRepository.GetByID(l.ctx, userID)
	if err != nil || user == nil {
		l.Logger.Errorf("Failed to get user info for email notification: %v", err)
		return
	}

	// 准备基础数据
	data, err := PrepareEmailNotificationDataWithUser(l.ctx, user, userID, l.Logger)
	if err != nil {
		// 邮箱修改的情况比较特殊，即使没有邮箱也要继续处理
		// 因为我们需要向旧邮箱和新邮箱发送通知
		username := strings.TrimSpace(user.Nickname)
		if username == "" {
			username = newEmail
		}

		// 手动准备数据
		data = &EmailNotificationData{
			Username:   username,
			UID:        fmt.Sprintf("%d", userID),
			IPAddress:  middleware.GetClientIP(l.ctx),
			Location:   "未知",
			ChangeTime: time.Now().UTC().Format("2006-01-02 15:04:05"),
		}
		if data.IPAddress == "" {
			data.IPAddress = "未知"
		}
	}

	// 向新邮箱发送通知
	if strings.TrimSpace(newEmail) != "" {
		err = notify.SendEmailChangeEmailAsync(
			newEmail,
			data.Username,
			data.UID,
			oldEmail,
			newEmail,
			data.ChangeTime,
			data.IPAddress,
			data.Location,
		)

		if err != nil {
			l.Logger.Errorf("Failed to send email change notification to new email %s: %v", newEmail, err)
		} else {
			l.Logger.Infof("Email change notification sent successfully to new email %s", newEmail)
		}
	}

	// 向旧邮箱也发送通知（重要安全提醒）
	if strings.TrimSpace(oldEmail) != "" && oldEmail != newEmail {
		err = notify.SendEmailChangeEmailAsync(
			oldEmail,
			data.Username,
			data.UID,
			oldEmail,
			newEmail,
			data.ChangeTime,
			data.IPAddress,
			data.Location,
		)

		if err != nil {
			l.Logger.Errorf("Failed to send email change notification to old email %s: %v", oldEmail, err)
		} else {
			l.Logger.Infof("Email change notification sent successfully to old email %s", oldEmail)
		}
	}
}
