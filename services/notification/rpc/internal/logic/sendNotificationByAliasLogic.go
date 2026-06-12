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
)

type SendNotificationByAliasLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendNotificationByAliasLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendNotificationByAliasLogic {
	return &SendNotificationByAliasLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SendNotificationByAlias 通过别名批量发送推送
// 说明：客户端在用户注册时会绑定别名到极光推送
//   - Web2用户：使用user_id作为别名
//   - Web3用户：使用设备ID作为别名
//
// 优点：无需查询数据库获取registration_id，直接使用业务相关的标识进行推送
// 限制：
//   - 最多支持1000个别名
//   - 每个别名长度限制为40字节（UTF-8编码）
func (l *SendNotificationByAliasLogic) SendNotificationByAlias(in *pb.SendNotificationByAliasRequest) (*pb.SendNotificationByAliasResponse, error) {
	l.Infof("SendNotificationByAlias request: aliases_count=%d, user_id=%d, device_id=%s, type=%s, title=%s",
		len(in.Aliases), in.UserId, in.DeviceId, in.Type, in.Title)

	// 1. 参数验证
	if len(in.Aliases) == 0 {
		return &pb.SendNotificationByAliasResponse{
			Success: false,
			Message: "aliases cannot be empty",
		}, nil
	}

	// 验证别名数量（极光推送限制最多1000个）
	if len(in.Aliases) > 1000 {
		return &pb.SendNotificationByAliasResponse{
			Success: false,
			Message: "too many aliases, maximum 1000 allowed",
		}, nil
	}

	// 验证每个别名的长度和有效性（极光推送限制每个别名最大40字节）
	for i, alias := range in.Aliases {
		if alias == "" {
			return &pb.SendNotificationByAliasResponse{
				Success: false,
				Message: fmt.Sprintf("alias at index %d is empty", i),
			}, nil
		}
		// 检查字节长度（UTF-8编码）
		if len([]byte(alias)) > 40 {
			return &pb.SendNotificationByAliasResponse{
				Success: false,
				Message: fmt.Sprintf("alias at index %d exceeds 40 bytes (UTF-8): %s", i, alias),
			}, nil
		}
	}

	if in.Title == "" || in.Content == "" {
		return &pb.SendNotificationByAliasResponse{
			Success: false,
			Message: "title and content are required",
		}, nil
	}

	// 转换通知类型
	notificationType := l.convertNotificationType(in.Type)

	// 2. 检查用户推送设置
	// 优先使用user_id（Web2用户），如果没有则使用device_id（Web3用户）
	if in.UserId > 0 {
		// Web2用户：通过user_id查询设置
		settings, err := l.svcCtx.SettingsRepo.FindOrCreateByUserID(l.ctx, in.UserId)
		if err != nil {
			l.Errorf("Failed to get user settings by user_id: %v", err)
			// 继续执行，使用默认设置
		}

		// 3. 检查用户是否启用了该类型的推送
		if settings != nil && !settings.IsNotificationEnabled(notificationType) {
			l.Infof("User %d has disabled %s notifications", in.UserId, notificationType)
			return &pb.SendNotificationByAliasResponse{
				Success: true,
				Message: "User has disabled this notification type",
			}, nil
		}

		// 4. 检查是否在免打扰时段
		if settings != nil && settings.IsInQuietTime(time.Now()) {
			l.Infof("User %d is in quiet time, skipping push", in.UserId)
			// 仍然记录到历史，但不推送
			return &pb.SendNotificationByAliasResponse{
				Success: true,
				Message: "User is in quiet time",
			}, nil
		}
	} else if in.DeviceId != "" {
		// Web3用户：目前来说web3的配置是保存在客户端上的
		l.Infof("Web3 user push: device_id=%s (settings check not implemented yet)", in.DeviceId)
	}

	// 5. 准备推送内容
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

	// 6. 调用极光SDK通过别名批量推送
	var result *jpush.PushResult
	var err error

	//使用PushToAliases
	result, err = l.svcCtx.JPushClient.PushToAliases(in.Aliases, notification, options)

	// 7. 创建推送记录
	// 将 map[string]string 转换为 map[string]interface{}
	dataMap := make(model.JSONMap)
	for k, v := range in.Data {
		dataMap[k] = v
	}

	// 记录别名信息
	aliasesStr := fmt.Sprintf("%v", in.Aliases)
	if len(aliasesStr) > 255 {
		// 如果别名列表太长，截断保存
		aliasesStr = aliasesStr[:252] + "..."
	}

	record := &model.NotificationRecord{
		UserID:           in.UserId, // 可能为0，如果是批量推送且未指定user_id
		NotificationType: notificationType,
		TemplateCode:     in.TemplateCode,
		Title:            in.Title,
		Content:          in.Content,
		Data:             dataMap,
		RegistrationID:   aliasesStr, // 这里存储别名列表信息，方便后续追踪
	}

	if err != nil {
		// 推送失败
		l.Errorf("Failed to push to aliases %v: %v", in.Aliases, err)
		record.PushStatus = "failed"
		record.ErrorMessage = err.Error()

		// 保存推送记录
		if saveErr := l.svcCtx.RecordRepo.Create(l.ctx, record); saveErr != nil {
			l.Errorf("Failed to create notification record: %v", saveErr)
		}

		return &pb.SendNotificationByAliasResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to push: %v", err),
		}, nil
	}

	// 推送成功
	if result != nil && result.Success {
		l.Infof("Push success to %d aliases: msg_id=%s", len(in.Aliases), result.MsgID)
		record.PushStatus = "sent"
		record.JPushMsgID = result.MsgID
		now := time.Now()
		record.SentAt = &now

		// 保存推送记录
		if saveErr := l.svcCtx.RecordRepo.Create(l.ctx, record); saveErr != nil {
			l.Errorf("Failed to create notification record: %v", saveErr)
		}

		return &pb.SendNotificationByAliasResponse{
			Success:    true,
			Message:    fmt.Sprintf("Push sent successfully to %d aliases", len(in.Aliases)),
			JpushMsgId: result.MsgID,
		}, nil
	}

	// 未知情况
	record.PushStatus = "failed"
	record.ErrorMessage = "Unknown push result"
	if saveErr := l.svcCtx.RecordRepo.Create(l.ctx, record); saveErr != nil {
		l.Errorf("Failed to create notification record: %v", saveErr)
	}

	return &pb.SendNotificationByAliasResponse{
		Success: false,
		Message: "Push failed with unknown result",
	}, nil
}

// convertNotificationType 转换通知类型枚举
func (l *SendNotificationByAliasLogic) convertNotificationType(pbType pb.NotificationType) string {
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
