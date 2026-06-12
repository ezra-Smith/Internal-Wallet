package logic

import (
	"context"
	"fmt"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/jpush"
	"internalwallet/services/notification/rpc/internal/model"
	"internalwallet/services/notification/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type SendNotificationByDeviceTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendNotificationByDeviceTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendNotificationByDeviceTokenLogic {
	return &SendNotificationByDeviceTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SendNotificationByDeviceToken 通过设备ID发送推送（Web3用户专用）
// 流程：
//  1. 通过 device_token 查询设备记录，获取 registration_id
//  2. 查询用户推送设置（如果有 user_id）
//  3. 调用极光推送
//  4. 记录推送历史
func (l *SendNotificationByDeviceTokenLogic) SendNotificationByDeviceToken(in *pb.SendNotificationByDeviceTokenRequest) (*pb.SendNotificationByDeviceTokenResponse, error) {
	l.Infof("SendNotificationByDeviceToken request: device_token=%s, type=%s, title=%s",
		in.DeviceToken, in.Type, in.Title)

	// 1. 参数验证
	if in.DeviceToken == "" {
		return &pb.SendNotificationByDeviceTokenResponse{
			Success: false,
			Message: "device_token is required",
		}, nil
	}
	if in.Title == "" || in.Content == "" {
		return &pb.SendNotificationByDeviceTokenResponse{
			Success: false,
			Message: "title and content are required",
		}, nil
	}

	// 2. 通过 device_token 查询设备记录
	device, err := l.svcCtx.DeviceRepo.FindByDeviceToken(l.ctx, in.DeviceToken)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			l.Infof("Device not found: device_token=%s", in.DeviceToken)
			return &pb.SendNotificationByDeviceTokenResponse{
				Success: false,
				Message: "Device not found",
			}, nil
		}
		l.Errorf("Failed to find device: %v", err)
		return &pb.SendNotificationByDeviceTokenResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to find device: %v", err),
		}, nil
	}

	// 3. 检查设备是否活跃
	if !device.IsActive {
		l.Infof("Device is not active: device_id=%d", device.ID)
		return &pb.SendNotificationByDeviceTokenResponse{
			Success: false,
			Message: "Device is not active",
		}, nil
	}

	// 4. 检查 registration_id 是否存在
	if device.RegistrationID == "" {
		l.Infof("Device has no registration_id: device_id=%d", device.ID)
		return &pb.SendNotificationByDeviceTokenResponse{
			Success: false,
			Message: "Device has no registration_id",
		}, nil
	}

	l.Infof("Device found: device_id=%d, registration_id=%s, user_id=%d",
		device.ID, device.RegistrationID, device.UserID)

	// 转换通知类型
	notificationType := l.convertNotificationType(in.Type)

	// 5. 查询用户推送设置（如果有 user_id）
	if device.UserID > 0 {
		settings, err := l.svcCtx.SettingsRepo.FindOrCreateByUserID(l.ctx, device.UserID)
		if err != nil {
			l.Errorf("Failed to get user settings: %v", err)
			// 继续执行，使用默认设置
		}

		// 检查用户是否启用了该类型的推送
		if settings != nil && !settings.IsNotificationEnabled(notificationType) {
			l.Infof("User %d has disabled %s notifications", device.UserID, notificationType)
			return &pb.SendNotificationByDeviceTokenResponse{
				Success: true,
				Message: "User has disabled this notification type",
			}, nil
		}

		// 检查是否在免打扰时段
		if settings != nil && settings.IsInQuietTime(time.Now()) {
			l.Infof("User %d is in quiet time, skipping push", device.UserID)
			return &pb.SendNotificationByDeviceTokenResponse{
				Success: true,
				Message: "User is in quiet time",
			}, nil
		}
	}

	// 6. 准备推送内容
	notification := &jpush.NotificationPayload{
		Title:   in.Title,
		Content: in.Content,
		Extras:  in.Data,
	}

	options := &jpush.PushOptions{
		ApnsProduction: l.svcCtx.Config.JPush.ApnsProduction,
		TimeToLive:     jpush.DefaultTimeToLive,
		Priority:       jpush.PriorityNormal,
	}

	// 7. 调用极光SDK推送
	result, err := l.svcCtx.JPushClient.PushToDevice(device.RegistrationID, notification, options)

	// 8. 创建推送记录
	// 将 map[string]string 转换为 map[string]interface{}
	dataMap := make(model.JSONMap)
	for k, v := range in.Data {
		dataMap[k] = v
	}

	record := &model.NotificationRecord{
		UserID:           device.UserID, // Web3用户可能为0
		DeviceID:         device.ID,
		NotificationType: notificationType,
		TemplateCode:     in.TemplateCode,
		Title:            in.Title,
		Content:          in.Content,
		Data:             dataMap,
		RegistrationID:   device.RegistrationID,
	}

	if err != nil {
		// 推送失败
		l.Errorf("Failed to push to device %d: %v", device.ID, err)
		record.PushStatus = "failed"
		record.ErrorMessage = err.Error()

		// 保存推送记录
		if err := l.svcCtx.RecordRepo.Create(l.ctx, record); err != nil {
			l.Errorf("Failed to create notification record: %v", err)
		}

		return &pb.SendNotificationByDeviceTokenResponse{
			Success: false,
			Message: fmt.Sprintf("Push failed: %v", err),
		}, nil
	}

	// 推送成功
	l.Infof("Push success to device %d: msg_id=%s", device.ID, result.MsgID)
	record.PushStatus = "sent"
	record.JPushMsgID = result.MsgID
	now := time.Now()
	record.SentAt = &now

	// 保存推送记录
	if err := l.svcCtx.RecordRepo.Create(l.ctx, record); err != nil {
		l.Errorf("Failed to create notification record: %v", err)
	}

	return &pb.SendNotificationByDeviceTokenResponse{
		Success:    true,
		Message:    "Push sent successfully",
		JpushMsgId: result.MsgID,
	}, nil
}

// convertNotificationType 转换通知类型枚举
func (l *SendNotificationByDeviceTokenLogic) convertNotificationType(pbType pb.NotificationType) string {
	switch pbType {
	case pb.NotificationType_TRANSACTION:
		return "transaction"
	case pb.NotificationType_SECURITY:
		return "security"
	case pb.NotificationType_SYSTEM:
		return "system"
	case pb.NotificationType_PRICE_ALERT:
		return "price_alert"
	default:
		return "system"
	}
}
