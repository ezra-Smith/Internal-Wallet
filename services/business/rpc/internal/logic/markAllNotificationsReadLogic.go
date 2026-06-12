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

type MarkAllNotificationsReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkAllNotificationsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkAllNotificationsReadLogic {
	return &MarkAllNotificationsReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *MarkAllNotificationsReadLogic) MarkAllNotificationsRead(in *pb.MarkAllNotificationsReadReq) (*pb.MarkAllNotificationsReadResp, error) {
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
	err := l.svcCtx.UserNotificationRepository.MarkAllReadByUser(l.ctx, uid)
	if err != nil {
		l.Errorf("mark all notifications read failed: %v", err)
		return nil, errx.DBError()
	}
	return &pb.MarkAllNotificationsReadResp{Success: true}, nil
}
