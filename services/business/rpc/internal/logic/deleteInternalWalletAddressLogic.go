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

type DeleteInternalWalletAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteInternalWalletAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteInternalWalletAddressLogic {
	return &DeleteInternalWalletAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteInternalWalletAddressLogic) DeleteInternalWalletAddress(in *pb.DeleteInternalWalletAddressReq) (*pb.DeleteInternalWalletAddressResp, error) {
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

	repo := l.svcCtx.MemberInternalAddressRepository
	if repo == nil {
		return nil, errx.ServiceNotAvailable("internal address service")
	}

	row, err := repo.FindByID(l.ctx, id)
	if err != nil || row == nil {
		return nil, errx.AddressNotFound()
	}

	// 验证所有权
	if row.UserID != uid {
		return nil, errx.Forbidden("forbidden")
	}

	// 软删除
	if err := repo.Delete(l.ctx, id); err != nil {
		l.Logger.Errorf("Delete failed: %v", err)
		return nil, errx.DBError()
	}

	return &pb.DeleteInternalWalletAddressResp{Success: true, Message: "ok"}, nil
}
