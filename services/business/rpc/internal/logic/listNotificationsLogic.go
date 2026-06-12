package logic

import (
	"context"
	"fmt"
	"strconv"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListNotificationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListNotificationsLogic {
	return &ListNotificationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListNotificationsLogic) ListNotifications(in *pb.ListNotificationsReq) (*pb.ListNotificationsResp, error) {
	// 获取并验证用户身份
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		l.Error("unauthorized: missing user_id in context")
		return nil, fmt.Errorf("unauthorized: user not authenticated")
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || uid <= 0 {
		l.Errorf("invalid user_id: %s", uidStr)
		return nil, fmt.Errorf("unauthorized: invalid user_id")
	}

	page := in.Page
	if page <= 0 {
		page = 1
	}
	size := in.PageSize
	if size <= 0 {
		size = 20
	}
	var rows []model.UserNotificationModel
	var total int64
	nRepo := l.svcCtx.UserNotificationRepository
	rows, total, _ = nRepo.ListNotifications(l.ctx, uid, in.OnlyUnread, page, size)
	items := make([]*pb.Notification, 0, len(rows))
	for _, r := range rows {
		items = append(items, &pb.Notification{Id: strconv.FormatInt(r.ID, 10), Type: r.Type, Title: r.Title, Content: r.Content, Timestamp: r.Timestamp, Read: r.Read})
	}
	unread, _ := nRepo.CountUnreadByUser(l.ctx, uid)
	return &pb.ListNotificationsResp{Success: true, Total: int32(total), UnreadCount: int32(unread), Items: items}, nil
}
