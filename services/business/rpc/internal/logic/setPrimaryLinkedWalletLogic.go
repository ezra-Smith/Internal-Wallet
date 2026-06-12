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

type SetPrimaryLinkedWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetPrimaryLinkedWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetPrimaryLinkedWalletLogic {
	return &SetPrimaryLinkedWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetPrimaryLinkedWalletLogic) SetPrimaryLinkedWallet(in *pb.SetPrimaryLinkedWalletReq) (*pb.SetPrimaryLinkedWalletResp, error) {
	if in == nil || strings.TrimSpace(in.BindingId) == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	if l.svcCtx.PlatformBindingRepository == nil {
		return nil, errx.Internal("db unavailable")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	if err := l.svcCtx.PlatformBindingRepository.SetPrimaryBinding(l.ctx, uid, strings.TrimSpace(in.BindingId)); err != nil {
		return nil, errx.DBError()
	}
	return &pb.SetPrimaryLinkedWalletResp{Success: true, Message: "ok"}, nil
}
