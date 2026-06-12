package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"internalwallet/common/ipgeo"
	"internalwallet/common/middleware"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// parsePlatformFromUserAgent 从 User-Agent 解析平台类型
// 当客户端没有发送 X-Platform 请求头时使用
func parsePlatformFromUserAgent(ua string) int32 {
	uaLower := strings.ToLower(ua)

	// iOS 特征
	if strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad") || strings.Contains(uaLower, "ios") {
		return 2 // ios
	}

	// Android 特征
	if strings.Contains(uaLower, "android") {
		return 3 // android
	}

	// 默认当作 web 处理
	return 1 // web
}

// getIPLocation 查询IP地理位置（独立函数，方便复用）
func getIPLocation(ctx context.Context, ip string) string {
	location := ""
	if ip != "" {
		// 设置2秒超时，避免影响登录性能
		geoCtx, geoCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer geoCancel()

		if geo, err := ipgeo.GetGeoLocation(geoCtx, ip); err == nil {
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
			location = strings.Join(parts, " ")

			// 截断位置信息防止超过数据库字段长度 (varchar 100)
			if len(location) > 95 {
				location = location[:95]
			}

			logx.WithContext(ctx).Infof("IP地理位置查询成功: ip=%s, location=%s, source=%s", ip, location, geo.Source)
		} else {
			// 查询失败不影响主流程，记录日志即可
			logx.WithContext(ctx).Infof("IP地理位置查询失败: ip=%s, error=%v", ip, err)
		}
	}
	return location
}

// createUserDeviceWithTx 创建用户设备记录（仅用于新用户注册）
func createUserDeviceWithTx(ctx context.Context, tx *gorm.DB, userID int64) error {
	deviceRepo := repository.NewUserDeviceRepository(tx)

	platformStr := middleware.GetPlatform(ctx)
	var platform int32
	switch strings.ToLower(platformStr) {
	case "web":
		platform = 1
	case "ios":
		platform = 2
	case "android":
		platform = 3
	case "api":
		platform = 4
	default:
		// 当 X-Platform 为空时，尝试从 User-Agent 解析
		platform = parsePlatformFromUserAgent(middleware.GetUserAgent(ctx))

		// 记录日志用于调试
		if platformStr == "" {
			logx.WithContext(ctx).Infof("createUserDeviceWithTx  Platform not in header, parsed from User-Agent (new user): user_id=%d, platform=%d, ua=%s",
				userID, platform, middleware.GetUserAgent(ctx))
		}
	}

	ua := middleware.GetUserAgent(ctx)
	// 截断 User-Agent 防止超过数据库字段长度 (varchar 512)
	if len(ua) > 500 {
		ua = ua[:500]
	}
	ip := middleware.GetClientIP(ctx)
	now := time.Now()

	// 查询IP地理位置
	location := getIPLocation(ctx, ip)

	dev := &model.UserDeviceModel{
		UserId:            userID,
		DeviceName:        ua,
		Platform:          platform,
		LastLoginIp:       ip,
		LastLoginLocation: location,
		LastLoginTime:     now,
		IsCurrent:         true,
		IsTrusted:         false,
		LoginCount:        1,
		FirstLoginTime:    now,
	}
	return deviceRepo.CreateDevice(ctx, dev)
}

// UpdateUserDeviceOnLogin 更新或创建用户设备记录（每次登录都应调用）
func UpdateUserDeviceOnLogin(ctx context.Context, db *gorm.DB, userID int64) error {
	platformStr := middleware.GetPlatform(ctx)
	// 添加调试日志：记录原始 platform 值
	logx.WithContext(ctx).Infof("UpdateUserDeviceOnLogin Start: user_id=%d, platform_str=%q", userID, platformStr)

	var platform int32
	switch strings.ToLower(platformStr) {
	case "web":
		platform = 1
	case "ios":
		platform = 2
	case "android":
		platform = 3
	case "api":
		platform = 4
	default:
		// 当 X-Platform 为空时，尝试从 User-Agent 解析
		platform = parsePlatformFromUserAgent(middleware.GetUserAgent(ctx))

		// 记录日志用于调试
		logx.WithContext(ctx).Infof("UpdateUserDeviceOnLogin  Platform not in header or invalid, parsed from User-Agent: user_id=%d, platform=%d, ua=%s",
			userID, platform, middleware.GetUserAgent(ctx))
	}

	ua := middleware.GetUserAgent(ctx)
	// 截断 User-Agent 防止超过数据库字段长度 (varchar 512)
	if len(ua) > 500 {
		ua = ua[:500]
	}
	ip := middleware.GetClientIP(ctx)
	now := time.Now()

	// 查询IP地理位置
	location := getIPLocation(ctx, ip)

	// 查找现有设备记录（根据 user_id 和 platform）
	var existingDevice model.UserDeviceModel
	err := db.WithContext(ctx).
		Where("user_id = ? AND platform = ?", userID, platform).
		First(&existingDevice).Error

	if err == nil {
		// 设备记录已存在，更新登录信息
		updates := map[string]interface{}{
			"device_name":         ua,
			"last_login_ip":       ip,
			"last_login_location": location,
			"last_login_time":     now,
			"is_current":          true,
			"login_count":         gorm.Expr("login_count + 1"),
		}
		if err := db.WithContext(ctx).Model(&existingDevice).Updates(updates).Error; err != nil {
			logx.WithContext(ctx).Errorf("更新设备记录失败: user_id=%d, platform=%d, error=%v", userID, platform, err)
			return err
		}
		logx.WithContext(ctx).Infof("设备记录已更新: user_id=%d, platform=%d, ip=%s, location=%s", userID, platform, ip, location)
		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		// 数据库查询错误
		logx.WithContext(ctx).Errorf("查询设备记录失败: user_id=%d, platform=%d, error=%v", userID, platform, err)
		return err
	}

	// 记录不存在，尝试查找 platform=0 的旧记录进行迁移
	// 这是为了处理之前 platform 默认为 0 的情况
	if platform != 0 {
		var oldDevice model.UserDeviceModel
		err := db.WithContext(ctx).
			Where("user_id = ? AND platform = ?", userID, 0).
			First(&oldDevice).Error
		if err == nil {
			// 找到 platform=0 的旧记录，更新 platform 和登录信息
			updates := map[string]interface{}{
				"platform":            platform,
				"device_name":         ua,
				"last_login_ip":       ip,
				"last_login_location": location,
				"last_login_time":     now,
				"is_current":          true,
				"login_count":         gorm.Expr("login_count + 1"),
			}
			if err := db.WithContext(ctx).Model(&oldDevice).Updates(updates).Error; err != nil {
				logx.WithContext(ctx).Errorf("迁移旧设备记录失败: user_id=%d, platform=%d, error=%v", userID, platform, err)
				return err
			}
			logx.WithContext(ctx).Infof("旧设备记录已迁移: user_id=%d, old_platform=0, new_platform=%d", userID, platform)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logx.WithContext(ctx).Errorf("查询旧设备记录失败: user_id=%d, error=%v", userID, err)
			return err
		}
	}

	// 设备记录不存在，创建新记录（可能是新设备）
	dev := &model.UserDeviceModel{
		UserId:            userID,
		DeviceName:        ua,
		Platform:          platform,
		LastLoginIp:       ip,
		LastLoginLocation: location,
		LastLoginTime:     now,
		IsCurrent:         true,
		IsTrusted:         false,
		LoginCount:        1,
		FirstLoginTime:    now,
	}
	if err := db.WithContext(ctx).Create(dev).Error; err != nil {
		logx.WithContext(ctx).Errorf("创建设备记录失败: user_id=%d, platform=%d, error=%v", userID, platform, err)
		return err
	}
	logx.WithContext(ctx).Infof("设备记录已创建: user_id=%d, platform=%d, ip=%s, location=%s", userID, platform, ip, location)

	return nil
}

func setupNewUserWithTx(ctx context.Context, svcCtx *svc.ServiceContext, tx *gorm.DB, profile *model.UserModel) error {
	securityRepo := repository.NewUserSecuritySettingsRepository(tx)
	if err := securityRepo.CreateDefaultSecurity(ctx, profile.ID, profile.Email != "", profile.Phone != ""); err != nil {
		return err
	}

	// NOTE: 移除了 Signer 直接调用逻辑
	// 现在改为在事务外调用 Admin.InitializeUserWallet
	// 详见各个登录 logic 的 autoRegister 方法

	if err := createUserDeviceWithTx(ctx, tx, profile.ID); err != nil {
		return err
	}
	return nil
}
