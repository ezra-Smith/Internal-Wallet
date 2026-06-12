package logic

import (
	"context"
	"fmt"
	"internalwallet/common/middleware"
	"strconv"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendPushNotificationByAliasLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendPushNotificationByAliasLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendPushNotificationByAliasLogic {
	return &SendPushNotificationByAliasLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SendPushNotificationByAlias 通过别名发送推送通知
// 说明：业务服务调用推送服务，通过别名（user_id或设备ID）发送推送
func (l *SendPushNotificationByAliasLogic) SendPushNotificationByAlias(in *pb.SendPushNotificationByAliasReq) (*pb.SendPushNotificationByAliasResp, error) {
	l.Infof("SendPushNotificationByAlias request: aliases_count=%d, type=%s, title=%s",
		len(in.Aliases), in.NotificationType, in.Title)

	// 1. 检查NotificationRpc客户端是否可用
	if l.svcCtx.NotificationRpc == nil {
		l.Error("NotificationRpc client is not configured")
		return &pb.SendPushNotificationByAliasResp{
			Success: false,
			Message: "Push notification service is not available",
		}, nil
	}

	// 2. 参数验证
	if len(in.Aliases) == 0 {
		return &pb.SendPushNotificationByAliasResp{
			Success: false,
			Message: "aliases cannot be empty",
		}, nil
	}

	if in.Title == "" || in.Content == "" {
		return &pb.SendPushNotificationByAliasResp{
			Success: false,
			Message: "title and content are required",
		}, nil
	}

	// 3. 从context获取user_id（用于单用户推送和查询Web2用户设置）
	var userId int64
	if userIdStr := middleware.GetUserID(l.ctx); userIdStr != "" {
		if id, err := strconv.ParseInt(userIdStr, 10, 64); err == nil {
			userId = id
		}
	}

	// 4. 从context或请求参数获取device_id（用于查询Web3用户设置）
	deviceId := in.DeviceId
	if deviceId == "" {
		// 如果请求未提供device_id，尝试从context获取
		deviceId = middleware.GetDeviceID(l.ctx)
	}

	l.Infof("Context info: user_id=%d, device_id=%s", userId, deviceId)

	// 5. 转换通知类型
	notificationType := l.convertNotificationType(in.NotificationType)

	// 6. 调用推送服务
	pushReq := &pb.SendNotificationByAliasRequest{
		Aliases:  in.Aliases,
		UserId:   userId,   // 从context获取的user_id（Web2用户）
		DeviceId: deviceId, // 从context或请求获取的device_id（Web3用户）
		Type:     notificationType,
		Title:    in.Title,
		Content:  in.Content,
		Data:     in.Data,
	}

	pushResp, err := l.svcCtx.NotificationRpc.SendNotificationByAlias(l.ctx, pushReq)
	if err != nil {
		l.Errorf("Failed to call NotificationRpc.SendNotificationByAlias: %v", err)
		return &pb.SendPushNotificationByAliasResp{
			Success: false,
			Message: fmt.Sprintf("Failed to send push notification: %v", err),
		}, nil
	}

	// 7. 返回结果
	return &pb.SendPushNotificationByAliasResp{
		Success:    pushResp.Success,
		Message:    pushResp.Message,
		JpushMsgId: pushResp.JpushMsgId,
	}, nil
}

// convertNotificationType 转换通知类型字符串到枚举
func (l *SendPushNotificationByAliasLogic) convertNotificationType(typeStr string) pb.NotificationType {
	switch typeStr {
	case "transaction":
		return pb.NotificationType_TRANSACTION
	case "security":
		return pb.NotificationType_SECURITY
	case "system":
		return pb.NotificationType_SYSTEM
	case "price_alert":
		return pb.NotificationType_PRICE_ALERT
	default:
		return pb.NotificationType_SYSTEM
	}
}
