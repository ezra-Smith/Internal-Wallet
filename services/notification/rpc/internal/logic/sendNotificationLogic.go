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

type SendNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendNotificationLogic {
	return &SendNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SendNotification 发送单用户推送
func (l *SendNotificationLogic) SendNotification(in *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	l.Infof("SendNotification request: user_id=%d, type=%s, title=%s",
		in.UserId, in.Type, in.Title)

	// 1. 参数验证
	if in.UserId <= 0 {
		return &pb.SendNotificationResponse{
			Success: false,
			Message: "invalid user_id",
		}, nil
	}
	if in.Title == "" || in.Content == "" {
		return &pb.SendNotificationResponse{
			Success: false,
			Message: "title and content are required",
		}, nil
	}

	// 转换通知类型
	notificationType := l.convertNotificationType(in.Type)

	// 2. 查询用户推送设置
	settings, err := l.svcCtx.SettingsRepo.FindOrCreateByUserID(l.ctx, in.UserId)
	if err != nil {
		l.Errorf("Failed to get user settings: %v", err)
		// 继续执行，使用默认设置
	}

	// 3. 检查用户是否启用了该类型的推送
	if settings != nil && !settings.IsNotificationEnabled(notificationType) {
		l.Infof("User %d has disabled %s notifications", in.UserId, notificationType)
		return &pb.SendNotificationResponse{
			Success: true,
			Message: "User has disabled this notification type",
		}, nil
	}

	// 4. 检查是否在免打扰时段
	if settings != nil && settings.IsInQuietTime(time.Now()) {
		l.Infof("User %d is in quiet time, skipping push", in.UserId)
		// 仍然记录到历史，但不推送
		// 可以在后续实现延迟推送功能
	}

	// 5. 获取用户的活跃设备
	var devices []*model.NotificationDevice
	if in.SendToAllDevices {
		devices, err = l.svcCtx.DeviceRepo.FindActiveDevicesByUserID(l.ctx, in.UserId)
	} else {
		// 只获取第一个活跃设备
		allDevices, err := l.svcCtx.DeviceRepo.FindActiveDevicesByUserID(l.ctx, in.UserId)
		if err == nil && len(allDevices) > 0 {
			devices = []*model.NotificationDevice{allDevices[0]}
		}
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		l.Errorf("Failed to get user devices: %v", err)
		return &pb.SendNotificationResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to get user devices: %v", err),
		}, nil
	}

	if len(devices) == 0 {
		l.Infof("User %d has no active devices", in.UserId)
		return &pb.SendNotificationResponse{
			Success:     true,
			Message:     "User has no active devices",
			DeviceCount: 0,
		}, nil
	}

	l.Infof("Found %d active devices for user %d", len(devices), in.UserId)

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

	// 7. 推送到每个设备并记录
	var successCount int
	var jpushMsgIDs []string

	for _, device := range devices {
		// 7a. 调用极光SDK推送
		result, err := l.svcCtx.JPushClient.PushToDevice(device.RegistrationID, notification, options)

		// 7b. 创建推送记录
		// 将 map[string]string 转换为 map[string]interface{}
		dataMap := make(model.JSONMap)
		for k, v := range in.Data {
			dataMap[k] = v
		}

		record := &model.NotificationRecord{
			UserID:           in.UserId,
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
		} else if result != nil && result.Success {
			// 推送成功
			l.Infof("Push success to device %d: msg_id=%s", device.ID, result.MsgID)
			record.PushStatus = "sent"
			record.JPushMsgID = result.MsgID
			now := time.Now()
			record.SentAt = &now
			jpushMsgIDs = append(jpushMsgIDs, result.MsgID)
			successCount++
		}

		// 保存推送记录
		if err := l.svcCtx.RecordRepo.Create(l.ctx, record); err != nil {
			l.Errorf("Failed to create notification record: %v", err)
		}
	}

	l.Infof("Push completed: user_id=%d, total=%d, success=%d", in.UserId, len(devices), successCount)

	return &pb.SendNotificationResponse{
		Success:     successCount > 0,
		Message:     fmt.Sprintf("Pushed to %d/%d devices", successCount, len(devices)),
		JpushMsgIds: jpushMsgIDs,
		DeviceCount: int32(len(devices)),
	}, nil
}

// convertNotificationType 转换通知类型枚举
func (l *SendNotificationLogic) convertNotificationType(pbType pb.NotificationType) string {
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
