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

type UpdateWalletAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateWalletAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateWalletAddressLogic {
	return &UpdateWalletAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateWalletAddressLogic) UpdateWalletAddress(in *pb.UpdateWalletAddressReq) (*pb.UpdateWalletAddressResp, error) {
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
	fields := map[string]interface{}{}
	if in.Label != "" {
		fields["label"] = in.Label
	}
	if in.SetDefault {
		_ = repo.SetAllDefaultFalseByUser(l.ctx, uid)
		fields["is_default"] = true
	}
	if len(fields) == 0 {
		return &pb.UpdateWalletAddressResp{Success: true, Message: "ok"}, nil
	}
	if err := repo.UpdateFields(l.ctx, id, fields); err != nil {
		return nil, errx.DBError()
	}
	return &pb.UpdateWalletAddressResp{Success: true, Message: "ok"}, nil
}
