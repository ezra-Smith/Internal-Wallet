package logic

import (
	"context"
	"strconv"
	"time"

	"internalwallet/common/code"
	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindEmailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindEmailLogic {
	return &BindEmailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BindEmailLogic) BindEmail(in *pb.BindEmailReq) (*pb.BindEmailResp, error) {
	if in == nil || in.Email == "" || !utils.ValidateEmail(in.Email) || in.Code == "" {
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

	// 检查当前用户是否已绑定邮箱
	profile, err := accountRepo.GetByID(l.ctx, uid)
	if err != nil || profile == nil {
		return nil, errx.UserNotFound()
	}
	if profile.Email != "" {
		return nil, errx.EmailAlreadyBound()
	}

	// 检查新邮箱是否已被其他用户使用
	existingUser, err := accountRepo.GetByEmail(l.ctx, in.Email)
	if err == nil && existingUser != nil && existingUser.ID != uid {
		return nil, errx.EmailAlreadyInUse()
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

	scene := "bind_email"
	codeType := "email"
	recipient := in.Email
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
		return nil, errx.InvalidCode("email")
	}

	securityRepo := l.svcCtx.UserSecuritySettingsRepository
	if err := accountRepo.UpsertEmailByID(l.ctx, uid, in.Email); err != nil {
		return nil, errx.DBError()
	}
	_ = securityRepo.UpdateEmailBoundAndMask(l.ctx, uid, in.Email)

	// 删除已使用的验证码
	_ = gen.Delete(l.ctx, recipient, scene, codeType)

	return &pb.BindEmailResp{Success: true, Message: "ok"}, nil
}
