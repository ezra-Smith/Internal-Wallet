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

type DeleteNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteNotificationLogic {
	return &DeleteNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteNotificationLogic) DeleteNotification(in *pb.DeleteNotificationReq) (*pb.DeleteNotificationResp, error) {
	if in == nil || in.Id == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	id, err := strconv.ParseInt(in.Id, 10, 64)
	if err != nil {
		return nil, errx.InvalidParam("invalid notification id")
	}

	// 获取并验证用户身份 🔒 安全关键！
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

	// 使用 DeleteByUser 确保只能删除自己的消息
	err = l.svcCtx.UserNotificationRepository.DeleteByUser(l.ctx, id, uid)
	if err != nil {
		l.Errorf("delete notification failed: %v", err)
		return nil, errx.DBError()
	}
	return &pb.DeleteNotificationResp{Success: true}, nil
}
