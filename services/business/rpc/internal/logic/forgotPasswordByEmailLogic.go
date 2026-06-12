package logic

import (
	"context"
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

type ForgotPasswordByEmailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewForgotPasswordByEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForgotPasswordByEmailLogic {
	return &ForgotPasswordByEmailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 忘记密码（邮箱+验证码）
func (l *ForgotPasswordByEmailLogic) ForgotPasswordByEmail(in *pb.ForgotPasswordByEmailRequest) (*pb.ForgotPasswordByEmailResponse, error) {
	if in == nil || in.Code == "" || in.NewPassword == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	email := strings.TrimSpace(in.Email)
	if !utils.ValidateEmail(email) {
		return nil, errx.InvalidParam("invalid email")
	}
	if !utils.ValidatePassword(in.NewPassword) {
		return nil, errx.InvalidParam("invalid password")
	}
	if err := security.ValidatePasswordStrength(in.NewPassword, 8, false); err != nil {
		return nil, errx.PasswordTooWeak("password too weak: " + err.Error())
	}

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
	codeType := "email"
	bypass := l.svcCtx.Config.Code.BypassCode

	var ok bool
	var err error
	if bypass != "" && in.Code == bypass {
		ok = true
	} else {
		ok, err = gen.Verify(l.ctx, email, scene, codeType, in.Code)
	}
	if err != nil {
		return nil, errx.VerifyFailed("verify failed")
	}
	if !ok {
		return nil, errx.InvalidCode("email")
	}

	accountRepo := l.svcCtx.UserAccountRepository
	profile, err := accountRepo.GetByEmail(l.ctx, email)
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

	_ = gen.Delete(l.ctx, email, scene, codeType)
	return &pb.ForgotPasswordByEmailResponse{Success: true, Message: "ok"}, nil
}
