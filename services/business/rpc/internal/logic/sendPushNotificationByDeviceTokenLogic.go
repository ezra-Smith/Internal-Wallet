package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendPushNotificationByDeviceTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendPushNotificationByDeviceTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendPushNotificationByDeviceTokenLogic {
	return &SendPushNotificationByDeviceTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SendPushNotificationByDeviceToken 通过设备ID发送推送（Web3用户专用）
// 说明：调用 Notification RPC 服务，通过设备ID发送推送
func (l *SendPushNotificationByDeviceTokenLogic) SendPushNotificationByDeviceToken(in *pb.SendPushNotificationByDeviceTokenReq) (*pb.SendPushNotificationByDeviceTokenResp, error) {
	l.Infof("SendPushNotificationByDeviceToken request: device_token=%s, type=%s, title=%s",
		in.DeviceToken, in.NotificationType, in.Title)

	// 1. 参数验证
	if in.DeviceToken == "" {
		return &pb.SendPushNotificationByDeviceTokenResp{
			Success: false,
			Message: "device_token is required",
		}, nil
	}
	if in.Title == "" || in.Content == "" {
		return &pb.SendPushNotificationByDeviceTokenResp{
			Success: false,
			Message: "title and content are required",
		}, nil
	}

	// 2. 转换通知类型
	var notificationType pb.NotificationType
	switch in.NotificationType {
	case "transaction":
		notificationType = pb.NotificationType_TRANSACTION
	case "security":
		notificationType = pb.NotificationType_SECURITY
	case "system":
		notificationType = pb.NotificationType_SYSTEM
	case "price_alert":
		notificationType = pb.NotificationType_PRICE_ALERT
	default:
		notificationType = pb.NotificationType_SYSTEM
	}

	// 3. 调用 Notification RPC 服务发送推送
	resp, err := l.svcCtx.NotificationRpc.SendNotificationByDeviceToken(l.ctx, &pb.SendNotificationByDeviceTokenRequest{
		DeviceToken: in.DeviceToken,
		Type:        notificationType,
		Title:       in.Title,
		Content:     in.Content,
		Data:        in.Data,
	})

	if err != nil {
		l.Errorf("Failed to send push notification: %v", err)
		return &pb.SendPushNotificationByDeviceTokenResp{
			Success: false,
			Message: "Failed to send push notification: " + err.Error(),
		}, nil
	}

	l.Infof("Push notification sent: success=%v, msg_id=%s", resp.Success, resp.JpushMsgId)

	return &pb.SendPushNotificationByDeviceTokenResp{
		Success:    resp.Success,
		Message:    resp.Message,
		JpushMsgId: resp.JpushMsgId,
	}, nil
}
