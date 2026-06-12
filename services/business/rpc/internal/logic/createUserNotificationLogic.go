package logic

import (
	"context"
	"fmt"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUserNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateUserNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserNotificationLogic {
	return &CreateUserNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateUserNotification 创建用户消息通知（内部接口，供业务逻辑调用）
// 支持批量创建（仅 Web2 用户）
func (l *CreateUserNotificationLogic) CreateUserNotification(in *pb.CreateUserNotificationReq) (*pb.CreateUserNotificationResp, error) {
	// 1. 参数验证
	if in == nil {
		l.Error("invalid params: request is nil")
		return &pb.CreateUserNotificationResp{Success: false, CreatedCount: 0}, fmt.Errorf("request is nil")
	}

	// 验证用户ID列表
	if len(in.UserIds) == 0 {
		l.Error("invalid params: user_ids is empty")
		return &pb.CreateUserNotificationResp{Success: false, CreatedCount: 0}, fmt.Errorf("user_ids is empty")
	}

	// 验证标题和内容
	if in.Title == "" || in.Content == "" {
		l.Error("invalid params: title or content is empty")
		return &pb.CreateUserNotificationResp{Success: false, CreatedCount: 0}, fmt.Errorf("title or content is empty")
	}

	// 2. 转换 data 为 map[string]interface{}
	data := make(map[string]interface{})
	if in.Data != nil {
		for k, v := range in.Data {
			data[k] = v
		}
	}

	// 3. 批量创建消息
	nRepo := l.svcCtx.UserNotificationRepository
	err := nRepo.BatchCreateNotifications(l.ctx, in.UserIds, in.Type, in.Title, in.Content, data)

	if err != nil {
		l.Errorf("create notifications failed: %v", err)
		return &pb.CreateUserNotificationResp{Success: false, CreatedCount: 0}, err
	}

	l.Infof("created %d notifications, type=%s", len(in.UserIds), in.Type)

	return &pb.CreateUserNotificationResp{
		Success:      true,
		CreatedCount: int32(len(in.UserIds)),
	}, nil
}
