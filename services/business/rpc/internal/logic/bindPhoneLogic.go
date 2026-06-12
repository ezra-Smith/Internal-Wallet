package logic

import (
	"context"
	"fmt"
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

type BindPhoneLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindPhoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindPhoneLogic {
	return &BindPhoneLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BindPhoneLogic) BindPhone(in *pb.BindPhoneReq) (*pb.BindPhoneResp, error) {
	if in == nil || in.Code == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	countryCode, err := normalizeAndValidateCountryCode(l.svcCtx, in.CountryCode)
	if err != nil {
		return nil, err
	}
	phone := strings.TrimSpace(in.Phone)
	if !utils.ValidatePhone(phone) {
		return nil, errx.InvalidParam("invalid phone")
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

	// 检查当前用户是否已绑定手机
	profile, err := accountRepo.GetByID(l.ctx, uid)
	if err != nil || profile == nil {
		return nil, errx.UserNotFound()
	}
	if profile.Phone != "" {
		return nil, errx.PhoneAlreadyBound()
	}

	// 检查新手机是否已被其他用户使用
	existingUser, err := accountRepo.GetByPhone(l.ctx, phone, countryCode)
	if err == nil && existingUser != nil && existingUser.ID != uid {
		return nil, errx.PhoneAlreadyInUse()
	}

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

	scene := "bind_phone"
	codeType := "sms"
	recipient := fmt.Sprintf("%s%s", countryCode, phone)
	bypass := l.svcCtx.Config.Code.BypassCode

	var ok bool
	if bypass != "" && in.Code == bypass {
		ok = true
	} else {
		ok, err = gen.Verify(l.ctx, recipient, scene, codeType, in.Code)
		if err != nil {
			l.Errorf("verify phone code failed: %v", err)
			return nil, errx.InvalidBindPhoneCode()
		}
	}
	if !ok {
		l.Errorf("invalid phone code for recipient: %s", recipient)
		return nil, errx.InvalidBindPhoneCode()
	}

	securityRepo := l.svcCtx.UserSecuritySettingsRepository
	if err := accountRepo.UpsertPhoneByID(l.ctx, uid, phone, countryCode); err != nil {
		return nil, errx.DBError()
	}
	_ = securityRepo.UpdatePhoneBoundAndMask(l.ctx, uid, phone)

	// 删除已使用的验证码
	_ = gen.Delete(l.ctx, recipient, scene, codeType)

	return &pb.BindPhoneResp{Success: true, Message: "ok"}, nil
}
