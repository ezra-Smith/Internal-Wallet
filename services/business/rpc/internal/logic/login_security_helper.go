package logic

import (
	"context"
	"fmt"
	"internalwallet/common/middleware"
	"strings"
	"time"

	"internalwallet/common/ipgeo"
	"internalwallet/pkg/notify"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// CheckAndNotifyAbnormalLogin 检测并通知异常登录
// 规则：IP地址变化 且 地理位置不同 → 发送邮件通知
// lastLoginInfo: 上次登录信息
// userAgent: 用户设备UA（从原始请求中获取，用于解析设备信息）
func CheckAndNotifyAbnormalLogin(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, lastLoginInfo *LastLoginInfo) {
	// 查询当前IP的地理位置
	currentIP := middleware.GetClientIP(ctx)
	currentLocation := getIPLocationForSecurity(ctx, currentIP)
	userAgent := middleware.GetUserAgent(ctx)
	// 如果没有传入上次登录信息，则从数据库查询
	var err error
	if lastLoginInfo == nil {
		// 如果是首次登录，不发送通知
		logx.WithContext(ctx).Infof("无法获取上次登录信息，跳过异常登录检测: user_id=%d, error=%v", userID, err)
		return
	}
	// 检测异常规则：IP变化 且 地理位置不同
	ipChanged := currentIP != lastLoginInfo.LastIP
	locationChanged := currentLocation != lastLoginInfo.LastLocation

	if ipChanged && locationChanged && currentLocation != "" {
		// 检测到异常登录，异步发送邮件通知
		logx.WithContext(ctx).Infof("检测到异常登录: user_id=%d, last_ip=%s, last_location=%s, current_ip=%s, current_location=%s",
			userID, lastLoginInfo.LastIP, lastLoginInfo.LastLocation, currentIP, currentLocation)

		// 发送邮件，传入userAgent用于解析设备信息
		sendAbnormalLoginEmail(ctx, svcCtx, userID, currentIP, currentLocation, userAgent)
	} else {
		logx.WithContext(ctx).Infof("登录检测正常: user_id=%d, ip_changed=%v, location_changed=%v",
			userID, ipChanged, locationChanged)
	}
}

// LastLoginInfo 上次登录信息
type LastLoginInfo struct {
	LastIP       string
	LastLocation string
	LastTime     time.Time
}

// GetLastLoginInfo 获取用户上次登录信息（导出供登录逻辑使用）
func GetLastLoginInfo(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) (*LastLoginInfo, error) {
	// 从设备表中查询最近一次登录记录（按登录时间倒序）
	var device model.UserDeviceModel
	err := svcCtx.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_login_time DESC").
		First(&device).Error

	if err != nil {
		return nil, err
	}

	return &LastLoginInfo{
		LastIP:       strings.TrimSpace(device.LastLoginIp),
		LastLocation: strings.TrimSpace(device.LastLoginLocation),
		LastTime:     device.LastLoginTime,
	}, nil
}

// getIPLocationForSecurity 获取IP地理位置（用于安全检测）
func getIPLocationForSecurity(ctx context.Context, ip string) string {
	if ip == "" {
		return ""
	}

	// 设置较短超时，避免影响登录性能
	geoCtx, geoCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer geoCancel()

	geo, err := ipgeo.GetGeoLocation(geoCtx, ip)
	if err != nil {
		logx.WithContext(ctx).Infof("IP地理位置查询失败: ip=%s, error=%v", ip, err)
		return ""
	}

	// 格式化地理位置：国家 省份 城市
	parts := make([]string, 0, 3)
	if geo.Country != "" {
		parts = append(parts, geo.Country)
	}
	if geo.Province != "" {
		parts = append(parts, geo.Province)
	}
	if geo.City != "" {
		parts = append(parts, geo.City)
	}

	location := strings.Join(parts, " ")
	logx.WithContext(ctx).Infof("IP地理位置查询成功: ip=%s, location=%s", ip, location)
	return strings.TrimSpace(location)
}

// sendAbnormalLoginEmail 发送异常登录邮件通知
func sendAbnormalLoginEmail(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, currentIP, currentLocation, userAgent string) {
	// 获取用户信息
	user, err := svcCtx.UserAccountRepository.GetByID(context.Background(), userID)
	if err != nil || user == nil {
		logx.WithContext(ctx).Errorf("获取用户信息失败，无法发送异常登录邮件: user_id=%d, error=%v", userID, err)
		return
	}

	// 准备邮件数据
	username := strings.TrimSpace(user.Nickname)
	if username == "" {
		username = user.Email
	}
	uid := fmt.Sprintf("%d", userID)

	// 获取设备信息（使用传入的userAgent而不是从context获取）
	device := parseDeviceFromUserAgent(userAgent)

	// 格式化登录时间（UTC）
	loginTime := time.Now().UTC().Format("2006-01-02 15:04:05")

	// 发送邮件
	err = notify.SendSecurityAlertAbnormalLoginEmailAsync(
		user.Email,
		username,
		uid,
		loginTime,
		device,
		currentIP,
		currentLocation,
	)

	if err != nil {
		logx.WithContext(ctx).Errorf("发送异常登录邮件失败: user_id=%d, email=%s, error=%v", userID, user.Email, err)
	} else {
		logx.WithContext(ctx).Infof("异常登录邮件发送成功: user_id=%d, email=%s, ip=%s, location=%s",
			userID, user.Email, currentIP, currentLocation)
	}
}

// parseDeviceFromUserAgent 从User-Agent中解析设备信息
func parseDeviceFromUserAgent(userAgent string) string {
	if userAgent == "" {
		return "未知设备"
	}

	// 简单解析，提取关键信息
	ua := strings.TrimSpace(userAgent)

	// 检测移动设备
	if strings.Contains(ua, "iPhone") {
		return "iPhone"
	}
	if strings.Contains(ua, "iPad") {
		return "iPad"
	}
	if strings.Contains(ua, "Android") {
		if strings.Contains(ua, "Mobile") {
			return "Android手机"
		}
		return "Android设备"
	}

	// 检测桌面浏览器
	if strings.Contains(ua, "Windows") {
		if strings.Contains(ua, "Chrome") {
			return "Windows Chrome"
		}
		if strings.Contains(ua, "Firefox") {
			return "Windows Firefox"
		}
		if strings.Contains(ua, "Edge") {
			return "Windows Edge"
		}
		return "Windows"
	}

	if strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS X") {
		if strings.Contains(ua, "Chrome") {
			return "Mac Chrome"
		}
		if strings.Contains(ua, "Safari") && !strings.Contains(ua, "Chrome") {
			return "Mac Safari"
		}
		if strings.Contains(ua, "Firefox") {
			return "Mac Firefox"
		}
		return "Mac"
	}

	if strings.Contains(ua, "Linux") {
		return "Linux"
	}

	// 截断过长的User-Agent
	if len(ua) > 50 {
		return ua[:50] + "..."
	}

	return ua
}
