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

type ChangePhoneLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangePhoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePhoneLogic {
	return &ChangePhoneLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChangePhoneLogic) ChangePhone(in *pb.ChangePhoneReq) (*pb.ChangePhoneResp, error) {
	if in == nil || in.Code1 == "" || in.Code2 == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	countryCode, err := normalizeAndValidateCountryCode(l.svcCtx, in.NewCountryCode)
	if err != nil {
		return nil, err
	}
	phone := strings.TrimSpace(in.NewPhone)
	if !utils.ValidatePhone(phone) {
		return nil, errx.InvalidPhone()
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

	// 获取当前用户的原手机号
	profile, err := accountRepo.GetByID(l.ctx, uid)
	if err != nil || profile == nil {
		return nil, errx.UserNotFound()
	}
	oldPhone := profile.Phone
	oldCountryCode := profile.CountryCode

	// 检查用户是否已绑定手机
	if oldPhone == "" {
		return nil, errx.NoPhoneBound()
	}

	// 检查新手机与旧手机是否相同
	if oldPhone == phone && oldCountryCode == countryCode {
		return nil, errx.PhoneSameAsOld()
	}

	// 检查新手机是否已被其他用户使用
	existingUser, err := accountRepo.GetByPhone(l.ctx, phone, countryCode)
	if err == nil && existingUser != nil && existingUser.ID != uid {
		return nil, errx.PhoneAlreadyInUse()
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

	// 验证原手机验证码 (code1)
	if oldPhone != "" {
		scene1 := "bind_phone"
		codeType1 := "sms"
		recipient1 := fmt.Sprintf("%s%s", oldCountryCode, oldPhone)
		var ok1 bool
		if bypass != "" && in.Code1 == bypass {
			ok1 = true
		} else {
			ok1, err = gen.Verify(l.ctx, recipient1, scene1, codeType1, in.Code1)
			if err != nil {
				return nil, errx.VerifyOldPhoneCodeFailed()
			}
		}
		if !ok1 {
			return nil, errx.InvalidOldPhoneCode()
		}
		// 删除已使用的验证码
		_ = gen.Delete(l.ctx, recipient1, scene1, codeType1)
	}

	// 验证新手机验证码 (code2)
	scene2 := "bind_phone"
	codeType2 := "sms"
	recipient2 := fmt.Sprintf("%s%s", countryCode, phone)
	var ok2 bool
	if bypass != "" && in.Code2 == bypass {
		ok2 = true
	} else {
		ok2, err = gen.Verify(l.ctx, recipient2, scene2, codeType2, in.Code2)
		if err != nil {
			return nil, errx.VerifyNewPhoneCodeFailed()
		}
	}
	if !ok2 {
		return nil, errx.InvalidNewPhoneCode()
	}

	if err := accountRepo.UpsertPhoneByID(l.ctx, uid, phone, countryCode); err != nil {
		return nil, errx.DBError()
	}
	_ = securityRepo.UpdatePhoneBoundAndMask(l.ctx, uid, phone)

	// 删除已使用的验证码
	_ = gen.Delete(l.ctx, recipient2, scene2, codeType2)

	// 异步发送邮件通知（不阻塞响应）
	ip := middleware.GetClientIP(l.ctx)
	go l.sendEmailNotification(uid, oldPhone, phone, oldCountryCode, countryCode, ip)

	return &pb.ChangePhoneResp{Success: true, Message: "ok"}, nil
}

// sendEmailNotification 发送手机号修改成功邮件通知
func (l *ChangePhoneLogic) sendEmailNotification(userID int64, oldPhone, newPhone, oldCountryCode, newCountryCode string, ip string) {
	// 准备邮件通知数据
	data, err := PrepareEmailNotificationData(l.ctx, l.svcCtx, userID, l.Logger, ip)
	if err != nil {
		// 已在 PrepareEmailNotificationData 中记录日志，这里直接返回
		return
	}

	// 格式化手机号显示（隐藏中间部分）
	oldPhoneDisplay := maskPhone(oldPhone)
	newPhoneDisplay := maskPhone(newPhone)

	// 异步发送邮件
	err = notify.SendPhoneChangeEmailAsync(
		data.Email,
		data.Username,
		data.UID,
		oldPhoneDisplay,
		newPhoneDisplay,
		newCountryCode,
		data.ChangeTime,
		data.IPAddress,
		data.Location,
	)

	if err != nil {
		l.Logger.Errorf("Failed to send phone change email to %s: %v", data.Email, err)
	} else {
		l.Logger.Infof("Phone change email sent successfully to %s", data.Email)
	}
}

// maskPhone 隐藏手机号中间部分
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	// 显示前3位和后4位，中间用*代替
	if len(phone) <= 7 {
		return phone[:2] + "***" + phone[len(phone)-2:]
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}
