package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/common/code"
	"internalwallet/common/security"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ForgotPasswordByPhoneLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewForgotPasswordByPhoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForgotPasswordByPhoneLogic {
	return &ForgotPasswordByPhoneLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 忘记密码（手机+验证码）
func (l *ForgotPasswordByPhoneLogic) ForgotPasswordByPhone(in *pb.ForgotPasswordByPhoneRequest) (*pb.ForgotPasswordByPhoneResponse, error) {
	if in == nil || in.Code == "" || in.NewPassword == "" {
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
	if !utils.ValidatePassword(in.NewPassword) {
		return nil, errx.InvalidParam("invalid password")
	}
	if err := security.ValidatePasswordStrength(in.NewPassword, 8, false); err != nil {
		return nil, errx.PasswordTooWeak("password too weak: " + err.Error())
	}

	fullPhone := fmt.Sprintf("%s%s", countryCode, phone)

	length := l.svcCtx.Config.Code.Length
	if length <= 0 {
		length = 6
	}
	expire := l.svcCtx.Config.Code.ExpireSeconds
	if expire <= 0 {
		expire = 600
	}
	gen := code.NewGenerator(l.svcCtx.RedisClient, int(length), time.Duration(expire)*time.Second)

	scene := "reset_password"
	codeType := "sms"
	recipient := fullPhone
	bypass := l.svcCtx.Config.Code.BypassCode

	var ok bool
	if bypass != "" && in.Code == bypass {
		ok = true
	} else {
		ok, err = gen.Verify(l.ctx, recipient, scene, codeType, in.Code)
		if err != nil {
			return nil, errx.VerifyFailed("verify failed")
		}
	}
	if !ok {
		return nil, errx.InvalidCode("sms")
	}

	accountRepo := l.svcCtx.UserAccountRepository
	profile, err := accountRepo.GetByPhone(l.ctx, phone, countryCode)
	if err != nil || profile == nil {
		return nil, errx.UserNotFound()
	}

	newHash, err := security.HashPassword(in.NewPassword)
	if err != nil {
		l.Logger.Errorf("failed to hash password: %v", err)
		return nil, errx.Internal("internal error")
	}
	if err := accountRepo.UpdatePassword(l.ctx, profile.ID, newHash); err != nil {
		l.Logger.Errorf("failed to update password: %v", err)
		return nil, errx.DBError()
	}

	_ = gen.Delete(l.ctx, recipient, scene, codeType)
	return &pb.ForgotPasswordByPhoneResponse{Success: true, Message: "ok"}, nil
}
