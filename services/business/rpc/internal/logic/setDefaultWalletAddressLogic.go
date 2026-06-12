package logic

import (
	"context"
	"strconv"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetDefaultWalletAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetDefaultWalletAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetDefaultWalletAddressLogic {
	return &SetDefaultWalletAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetDefaultWalletAddressLogic) SetDefaultWalletAddress(in *pb.SetDefaultWalletAddressReq) (*pb.SetDefaultWalletAddressResp, error) {
	if in == nil || in.Id == "" {
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
	id, err := strconv.ParseInt(in.Id, 10, 64)
	if err != nil {
		return nil, errx.InvalidParam("invalid id")
	}
	repo := l.svcCtx.UserWalletAddressRepository
	row, err := repo.FindByID(l.ctx, id)
	if err != nil || row == nil {
		return nil, errx.AddressNotFound()
	}
	if row.UserId != uid {
		return nil, errx.Forbidden("forbidden")
	}
	_ = repo.SetAllDefaultFalseByUser(l.ctx, uid)
	if err := repo.UpdateFields(l.ctx, id, map[string]interface{}{"is_default": true}); err != nil {
		return nil, errx.DBError()
	}
	return &pb.SetDefaultWalletAddressResp{Success: true, Message: "ok"}, nil
}
