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

type MarkNotificationReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkNotificationReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkNotificationReadLogic {
	return &MarkNotificationReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *MarkNotificationReadLogic) MarkNotificationRead(in *pb.MarkNotificationReadReq) (*pb.MarkNotificationReadResp, error) {
	if in == nil || in.Id == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	id, err := strconv.ParseInt(in.Id, 10, 64)
	if err != nil {
		l.Errorf("invalid notification id: %v", err)
		return nil, errx.InvalidParam("invalid notification id")
	}

	// 获取并验证用户身份
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		l.Error("unauthorized: missing user_id in context")
		return nil, errx.Unauthorized("user not authenticated")
	}
	uid, parseErr := strconv.ParseInt(uidStr, 10, 64)
	if parseErr != nil || uid <= 0 {
		l.Errorf("invalid user_id: %s", uidStr)
		return nil, errx.Unauthorized("invalid user_id")
	}
	if err := l.svcCtx.UserNotificationRepository.MarkRead(l.ctx, id, uid); err != nil {
		l.Errorf("mark notification read failed: %v", err)
		return nil, errx.DBError()
	}
	return &pb.MarkNotificationReadResp{Success: true}, nil
}
