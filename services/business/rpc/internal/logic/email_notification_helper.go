package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/pkg/notify"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// EmailNotificationData 邮件通知准备的数据
type EmailNotificationData struct {
	Email      string
	Username   string
	UID        string
	IPAddress  string
	Location   string
	ChangeTime string
}

// PrepareEmailNotificationData 准备发送邮件通知所需的数据
// 这是一个通用方法，封装了获取用户信息、IP地址、时间等逻辑
func PrepareEmailNotificationData(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, logger logx.Logger, ipAddress string) (*EmailNotificationData, error) {
	// 获取用户信息
	user, err := svcCtx.UserAccountRepository.GetByID(context.Background(), userID)
	if err != nil || user == nil {
		logger.Errorf("Failed to get user info for email notification: %v", err)
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// 检查用户是否有邮箱
	email := strings.TrimSpace(user.Email)
	if email == "" {
		logger.Infof("User %d has no email, skip email notification", userID)
		return nil, fmt.Errorf("user has no email")
	}

	// 准备用户名（优先使用昵称，否则使用邮箱）
	username := strings.TrimSpace(user.Nickname)
	if username == "" {
		username = email
	}

	// 用户ID
	uid := fmt.Sprintf("%d", userID)

	if ipAddress == "" {
		ipAddress = "未知"
	}

	// 位置信息（可选，如果有IP定位服务可以添加）
	location := "未知"
	if ipAddress != "未知" {
		location = getIPLocation(ctx, ipAddress)
	}
	// 格式化时间（UTC）
	changeTime := time.Now().UTC().Format("2006-01-02 15:04:05")

	return &EmailNotificationData{
		Email:      email,
		Username:   username,
		UID:        uid,
		IPAddress:  ipAddress,
		Location:   location,
		ChangeTime: changeTime,
	}, nil
}

// PrepareEmailNotificationDataWithUser 基于已有的用户对象准备邮件通知数据
// 当调用方已经获取了用户对象时使用此方法，避免重复查询数据库
func PrepareEmailNotificationDataWithUser(ctx context.Context, user *model.UserModel, userID int64, logger logx.Logger) (*EmailNotificationData, error) {
	if user == nil {
		logger.Errorf("User is nil for email notification")
		return nil, fmt.Errorf("user is nil")
	}

	// 检查用户是否有邮箱
	email := strings.TrimSpace(user.Email)
	if email == "" {
		logger.Infof("User %d has no email, skip email notification", userID)
		return nil, fmt.Errorf("user has no email")
	}

	// 准备用户名（优先使用昵称，否则使用邮箱）
	username := strings.TrimSpace(user.Nickname)
	if username == "" {
		username = email
	}

	// 用户ID
	uid := fmt.Sprintf("%d", userID)

	// 获取客户端IP地址
	ipAddress := middleware.GetClientIP(ctx)
	if ipAddress == "" {
		ipAddress = "未知"
	}
	location := "未知"
	if ipAddress != "未知" {
		location = getIPLocation(ctx, ipAddress)
	}
	// 格式化时间（UTC）
	changeTime := time.Now().UTC().Format("2006-01-02 15:04:05")

	return &EmailNotificationData{
		Email:      email,
		Username:   username,
		UID:        uid,
		IPAddress:  ipAddress,
		Location:   location,
		ChangeTime: changeTime,
	}, nil
}

// Send2FAEnabledEmailNotification 发送2FA启用成功邮件通知（公共方法）
// 供 bindGoogleAuth 和 enableGoogleAuth2FA 使用
func Send2FAEnabledEmailNotification(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, ipAddress string, logger logx.Logger) {
	// 准备邮件通知数据
	data, err := PrepareEmailNotificationData(ctx, svcCtx, userID, logger, ipAddress)
	if err != nil {
		// 已在 PrepareEmailNotificationData 中记录日志，这里直接返回
		return
	}

	logx.Infof("Sending 2FA enabled email notification for user %d, ip=%s", userID, ipAddress)
	// 异步发送邮件
	err = notify.Send2FAEnabledEmailAsync(
		data.Email,
		data.Username,
		data.UID,
		data.ChangeTime,
		data.IPAddress,
		data.Location,
	)

	if err != nil {
		logger.Errorf("Failed to send 2FA enabled email to %s: %v", data.Email, err)
	} else {
		logger.Infof("2FA enabled email sent successfully to %s", data.Email)
	}
}

// Send2FADisabledEmailNotification 发送2FA禁用成功邮件通知（公共方法）
// 供 unbindGoogleAuth 和 disableGoogleAuth2FA 使用
// verifyMethod: 验证方式 (totp/email/sms/biometric)
func Send2FADisabledEmailNotification(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, ipAddress, verifyMethod string, logger logx.Logger) {
	// 准备邮件通知数据
	data, err := PrepareEmailNotificationData(ctx, svcCtx, userID, logger, ipAddress)
	if err != nil {
		// 已在 PrepareEmailNotificationData 中记录日志，这里直接返回
		return
	}

	logx.Infof("Sending 2FA disabled email notification for user %d, verify_method=%s, ip=%s", userID, verifyMethod, ipAddress)

	// 禁用方式的描述
	disabledMethodMap := map[string]string{
		"totp":      "谷歌验证器",
		"email":     "邮箱验证",
		"sms":       "短信验证",
		"biometric": "生物识别验证",
	}
	disabledMethod := disabledMethodMap[verifyMethod]
	if disabledMethod == "" {
		disabledMethod = verifyMethod
	}

	// 异步发送邮件
	err = notify.Send2FADisabledEmailAsync(
		data.Email,
		data.Username,
		data.UID,
		disabledMethod,
		data.ChangeTime,
		data.IPAddress,
		data.Location,
	)

	if err != nil {
		logger.Errorf("Failed to send 2FA disabled email to %s: %v", data.Email, err)
	} else {
		logger.Infof("2FA disabled email sent successfully to %s", data.Email)
	}
}
