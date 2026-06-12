package logic

import (
	"context"
	"strconv"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListLoginRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListLoginRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLoginRecordsLogic {
	return &ListLoginRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListLoginRecordsLogic) ListLoginRecords(in *pb.ListLoginRecordsReq) (*pb.ListLoginRecordsResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	if in == nil {
		in = &pb.ListLoginRecordsReq{}
	}
	page := in.Page
	if page <= 0 {
		page = 1
	}
	size := in.PageSize
	if size <= 0 {
		size = 20
	}
	var total int64
	var rows []model.UserDeviceModel
	if uidStr != "" {
		id, _ := strconv.ParseInt(uidStr, 10, 64)
		deviceRepo := l.svcCtx.UserDeviceRepository
		rows, total, _ = deviceRepo.ListDevicesByUser(l.ctx, id, page, size)
	}
	items := make([]*pb.LoginRecordItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, &pb.LoginRecordItem{DeviceName: r.DeviceName, Platform: pb.PlatformType(r.Platform), IpAddress: r.LastLoginIp, Location: r.LastLoginLocation, LoginTime: r.LastLoginTime.Unix(), Result: "success", IsCurrent: r.IsCurrent, IsTrusted: r.IsTrusted})
	}
	return &pb.ListLoginRecordsResp{Success: true, Total: int32(total), Items: items}, nil
}
