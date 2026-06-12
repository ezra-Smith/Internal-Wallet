package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserInfoLogic {
	return &GetUserInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserInfoLogic) GetUserInfo(in *pb.GetUserInfoReq) (*pb.GetUserInfoResp, error) {
	uid := middleware.GetUserID(l.ctx)
	uid = strings.TrimSpace(uid)
	if uid == "" {
		l.Logger.Error("GetUserInfo: missing user id in context")
		return nil, errx.Unauthorized("unauthorized")
	}
	id, err := strconv.ParseInt(uid, 10, 64)
	if err != nil || id <= 0 {
		l.Logger.Errorf("GetUserInfo: invalid user id format: %q, err=%v", uid, err)
		return nil, errx.InvalidParam("invalid user id")
	}

	var realName string
	var idNumber string
	var birthday string
	var gender int32
	var address string

	if l.svcCtx.UserKycRepository != nil {
		pi, kycErr := l.svcCtx.UserKycRepository.GetKycByUserID(l.ctx, id)
		if kycErr != nil {
			l.Logger.Errorf("GetUserInfo: get kyc failed (uid=%d): %v", id, kycErr)
		} else if pi != nil {
			realName = pi.RealName
			idNumber = pi.IdNumber
			birthday = pi.Birthday
			gender = pi.Gender
			address = pi.Address
		}
	}

	if l.svcCtx.UserAccountRepository == nil {
		l.Logger.Error("GetUserInfo: UserAccountRepository not initialized (likely DB/GORM config missing)")
		return nil, errx.ServiceNotAvailable("db")
	}
	profile, err := l.svcCtx.UserAccountRepository.GetByID(l.ctx, id)
	if err != nil || profile == nil {
		l.Logger.Errorf("GetUserInfo: get personal info failed (uid=%d): %v", id, err)
		return nil, errx.UserNotFound()
	}
	return &pb.GetUserInfoResp{
		Success:   true,
		RealName:  realName,
		IdNumber:  idNumber,
		Birthday:  birthday,
		Gender:    pb.Gender(gender),
		Address:   address,
		Nickname:  profile.Nickname,
		Email:     profile.Email,
		Phone:     profile.Phone,
		AvatarUrl: profile.Avatar,
	}, nil
}
