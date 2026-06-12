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

type LinkWalletAccountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLinkWalletAccountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LinkWalletAccountLogic {
	return &LinkWalletAccountLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LinkWalletAccountLogic) LinkWalletAccount(in *pb.LinkWalletAccountReq) (*pb.LinkWalletAccountResp, error) {
	if in == nil || strings.TrimSpace(in.ProviderId) == "" || strings.TrimSpace(in.AccountId) == "" {
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
	row, err := l.svcCtx.PlatformBindingRepository.CreateBinding(l.ctx, uid, strings.TrimSpace(in.ProviderId), strings.TrimSpace(in.AccountId))
	if err != nil {
		return nil, errx.DBError()
	}
	return &pb.LinkWalletAccountResp{Success: true, Message: "ok", BindingId: row.BindingId}, nil
}
